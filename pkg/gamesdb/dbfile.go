package gamesdb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

// The games database file
//
// games.db holds every game, written so it reads back fast and small on
// the MiSTer. Most of what a game carries is the same for many games (its
// system, its folder on the SD card, its menu folder, its genres), so the
// file has tables of those, each written once, and then one short record
// per game: numbers into the tables and its file name. Name and extension
// come from the file name, as the database build makes them.
//
// Reading it puts every game's full path into one block of text, which
// the games' paths and names point into: 100,000 games are then a few
// pieces of memory, not hundreds of thousands, which Go's memory clean-up
// would otherwise have to go through every time it runs.
//
// Layout (numbers are unsigned varints, text is a length then bytes):
//
//	dbMagic
//	text table: count, then each text
//	genre lists: count, then each: count, then text numbers
//	games: count, then each game:
//	  system, folder, menu folder, rotation (text numbers), genre list
//	  (0 = none, else list number + 1), file name, flag: 0 = name and
//	  extension from the file name, 1 = name and extension follow.
//
// The folder is the path up to and including its last "/"; the path is
// folder + file name.
//
// An older games.db (Go's gob format) is still read, and saved in this
// format straight away (see loadAll).

const dbMagic = "SAMenu games database v2\n"

var errBadDB = errors.New("games database is damaged")

// writeDB writes files in the games database format.
func writeDB(w io.Writer, files []FileInfo) error {
	bw := bufio.NewWriterSize(w, 64*1024)
	var num [binary.MaxVarintLen64]byte
	putNum := func(n int) { bw.Write(num[:binary.PutUvarint(num[:], uint64(n))]) }
	putText := func(s string) { putNum(len(s)); bw.WriteString(s) }

	// Tables. Text number 0 is always "".
	texts := []string{""}
	textNum := map[string]int{"": 0}
	text := func(s string) int {
		n, ok := textNum[s]
		if !ok {
			n = len(texts)
			texts = append(texts, s)
			textNum[s] = n
		}
		return n
	}
	var lists [][]int
	listNum := map[string]int{}
	var key []byte
	list := func(genres []string) int {
		if len(genres) == 0 {
			return 0
		}
		key = key[:0]
		for _, g := range genres {
			key = append(append(key, g...), 0)
		}
		n, ok := listNum[string(key)]
		if !ok {
			l := make([]int, len(genres))
			for i, g := range genres {
				l[i] = text(g)
			}
			n = len(lists)
			lists = append(lists, l)
			listNum[string(key)] = n
		}
		return n + 1
	}

	type record struct {
		sys, dir, menuDir, rot, genres int
		base, name, ext                string
		explicit                       bool
	}
	records := make([]record, len(files))
	for i, f := range files {
		dir, base := splitPath(f.Path)
		name, ext := nameAndExt(base)
		r := record{
			sys: text(f.SystemId), dir: text(dir), menuDir: text(f.MenuDir),
			rot: text(f.Rotation), genres: list(f.Genres), base: base,
		}
		if name != f.Name || ext != f.Ext {
			r.explicit, r.name, r.ext = true, f.Name, f.Ext
		}
		records[i] = r
	}

	bw.WriteString(dbMagic)
	putNum(len(texts))
	for _, s := range texts {
		putText(s)
	}
	putNum(len(lists))
	for _, l := range lists {
		putNum(len(l))
		for _, n := range l {
			putNum(n)
		}
	}
	putNum(len(records))
	for _, r := range records {
		putNum(r.sys)
		putNum(r.dir)
		putNum(r.menuDir)
		putNum(r.rot)
		putNum(r.genres)
		putText(r.base)
		if r.explicit {
			bw.WriteByte(1)
			putText(r.name)
			putText(r.ext)
		} else {
			bw.WriteByte(0)
		}
	}
	return bw.Flush()
}

// readDB reads the games database format. A damaged file is an error,
// never a crash.
func readDB(data []byte) ([]FileInfo, error) {
	if !bytes.HasPrefix(data, []byte(dbMagic)) {
		return nil, errBadDB
	}
	pos := len(dbMagic)
	bad := false
	getNum := func() int {
		n, size := binary.Uvarint(data[pos:])
		if size <= 0 || n > uint64(len(data)) {
			bad = true
			return 0
		}
		pos += size
		return int(n)
	}
	// getCount reads how many of something follow: each takes at least a
	// byte, so a count bigger than what's left means a damaged file (and
	// is never used to set aside memory).
	getCount := func() int {
		n := getNum()
		if n > len(data)-pos {
			bad = true
			return 0
		}
		return n
	}
	getBytes := func() []byte {
		n := getNum()
		if bad || n > len(data)-pos {
			bad = true
			return nil
		}
		b := data[pos : pos+n]
		pos += n
		return b
	}

	texts := make([]string, getCount())
	for i := range texts {
		texts[i] = string(getBytes())
	}
	textAt := func(n int) string {
		if n >= len(texts) {
			bad = true
			return ""
		}
		return texts[n]
	}
	lists := make([][]string, getCount())
	for i := range lists {
		l := make([]string, getCount())
		for j := range l {
			l[j] = textAt(getNum())
		}
		lists[i] = l
		if bad {
			return nil, errBadDB
		}
	}
	count := getCount()
	if bad {
		return nil, errBadDB
	}

	// First pass: the records, and every path into one block of text.
	type record struct {
		start, dirLen, end int
		explicit           bool
		name, ext          string
	}
	files := make([]FileInfo, count)
	records := make([]record, count)
	all := make([]byte, 0, len(data))
	for i := range files {
		f := &files[i]
		f.SystemId = textAt(getNum())
		dir := textAt(getNum())
		f.MenuDir = textAt(getNum())
		f.Rotation = textAt(getNum())
		if g := getNum(); g > 0 {
			if g > len(lists) {
				return nil, errBadDB
			}
			f.Genres = lists[g-1]
		}
		base := getBytes()
		r := record{start: len(all), dirLen: len(dir)}
		all = append(append(all, dir...), base...)
		r.end = len(all)
		if pos >= len(data) {
			return nil, errBadDB
		}
		flag := data[pos]
		pos++
		if flag == 1 {
			r.explicit = true
			r.name, r.ext = string(getBytes()), string(getBytes())
		} else if flag != 0 {
			return nil, errBadDB
		}
		if bad {
			return nil, errBadDB
		}
		records[i] = r
	}
	if pos != len(data) {
		return nil, errBadDB
	}

	// Second pass: paths and names point into the block.
	block := string(all)
	for i := range files {
		f, r := &files[i], records[i]
		f.Path = block[r.start:r.end]
		if r.explicit {
			f.Name, f.Ext = r.name, r.ext
		} else {
			f.Name, f.Ext = nameAndExt(f.Path[r.dirLen:])
		}
	}
	return files, nil
}

// splitPath splits a path after its last "/": folder (with the "/") and
// file name.
func splitPath(path string) (dir, base string) {
	i := strings.LastIndexByte(path, '/')
	return path[:i+1], path[i+1:]
}

// nameAndExt splits a file name as the database build does: "Game.sfc"
// -> "Game", "sfc".
func nameAndExt(base string) (name, ext string) {
	e := filepath.Ext(base)
	return base[:len(base)-len(e)], strings.TrimPrefix(e, ".")
}

// legacyFileInfo is a game as the older gob games.db has it.
type legacyFileInfo struct {
	SystemId string
	Name     string
	Ext      string
	Path     string
	MenuPath string
	Rotation string
	Genres   []string
}

// readLegacyDB reads an older gob games.db.
func readLegacyDB(data []byte) ([]FileInfo, error) {
	var old []legacyFileInfo
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&old); err != nil {
		return nil, err
	}
	files := make([]FileInfo, len(old))
	for i, o := range old {
		menuDir := ""
		if j := strings.LastIndexByte(o.MenuPath, '/'); j >= 0 {
			menuDir = o.MenuPath[:j]
		}
		files[i] = FileInfo{SystemId: o.SystemId, Name: o.Name, Ext: o.Ext, Path: o.Path,
			MenuDir: menuDir, Rotation: o.Rotation, Genres: o.Genres}
	}
	shareStrings(files)
	return files, nil
}

// KeepOwnText gives files (a few games kept from the whole database) their
// own block of path text. Read from games.db, games' paths and names point
// into one block for the whole database, which stays in memory as long as
// any of them does: attract mode, keeping a few thousand games for hours,
// would otherwise keep every game's path.
func KeepOwnText(files []FileInfo) {
	size := 0
	for i := range files {
		size += len(files[i].Path)
	}
	all := make([]byte, 0, size)
	for i := range files {
		all = append(all, files[i].Path...)
	}
	block := string(all)
	at := 0
	for i := range files {
		f := &files[i]
		path := block[at : at+len(f.Path)]
		at += len(f.Path)
		_, base := splitPath(path)
		name, ext := nameAndExt(base)
		if f.Name == name {
			f.Name = name
		}
		if f.Ext == ext {
			f.Ext = ext
		}
		f.Path = path
	}
}
