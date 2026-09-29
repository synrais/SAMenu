package gamesdb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
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
// A file that isn't in this format (damaged, or from an older SAMenu,
// which used Go's gob format) reads as an error, and the menu builds a
// new database.

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

	// First pass: every path into one block of text, noting where each
	// one ends and where its file name starts (numbers, not a working
	// record per game: loading is when memory is tightest).
	type named struct{ name, ext string }
	files := make([]FileInfo, count)
	ends := make([]int32, count)   // where each path ends in the block
	baseAt := make([]int32, count) // where its file name starts
	explicit := map[int]named{}    // the rare games with their own name
	all := make([]byte, 0, 2*len(data))
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
		all = append(all, dir...)
		baseAt[i] = int32(len(all))
		all = append(all, base...)
		ends[i] = int32(len(all))
		if pos >= len(data) {
			return nil, errBadDB
		}
		flag := data[pos]
		pos++
		if flag == 1 {
			explicit[i] = named{string(getBytes()), string(getBytes())}
		} else if flag != 0 {
			return nil, errBadDB
		}
		if bad || len(all) > math.MaxInt32 {
			return nil, errBadDB
		}
	}
	if pos != len(data) {
		return nil, errBadDB
	}

	// Second pass: paths and names point into the block.
	block := string(all)
	all = nil
	start := int32(0)
	for i := range files {
		f := &files[i]
		f.Path = block[start:ends[i]]
		if e, ok := explicit[i]; ok {
			f.Name, f.Ext = e.name, e.ext
		} else {
			f.Name, f.Ext = nameAndExt(block[baseAt[i]:ends[i]])
		}
		start = ends[i]
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
