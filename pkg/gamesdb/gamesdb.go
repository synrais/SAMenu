package gamesdb

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/utils"
)

// -------------------------
// Types
// -------------------------

type FileInfo struct {
	SystemId string
	Name     string
	Ext      string
	Path     string
	// MenuDir is the menu folder the game is in, e.g. "SNES/RPG" (the
	// system's name, then its folders). Games in the same folder share
	// it; MenuPath adds the file name.
	MenuDir string
	// Rotation, for arcade MRAs: "horizontal", "vertical cw",
	// "vertical ccw", "vertical" (direction not stated) or "" (unknown).
	Rotation string
	// Genres, from the folders the game is in (see genres.go), e.g.
	// ["Sports", "Sports/Golf"].
	Genres []string
}

// MenuPath is the game's place in the menu, e.g.
// "SNES/RPG/Chrono Trigger.sfc": its menu folder, then its file name.
func (f FileInfo) MenuPath() string {
	if f.MenuDir == "" {
		return f.FileName()
	}
	return f.MenuDir + "/" + f.FileName()
}

// ListKey is the game's name as the game lists (Blacklist, Staticlist,
// Whitelist) compare it.
func (f FileInfo) ListKey() string { return utils.NormalizeTitle(f.Name) }

type IndexStatus struct {
	Total    int
	Step     int
	SystemId string // while Waiting: a message to show instead
	Files    int    // games found so far
	Waiting  bool   // another build is running; this one waits for it
	Doing    string // instead of "Indexing <system>", e.g. "Saving..."
	Skipped  bool   // SystemId is left out of the database: not scanned
}

type SearchResult struct {
	SystemId string
	Name     string
	Ext      string
	Path     string
}

// FileName is the game's file name: its name plus the extension, if any.
func (f FileInfo) FileName() string { return fileName(f.Name, f.Ext) }

// FileName is the game's file name: its name plus the extension, if any.
func (r SearchResult) FileName() string { return fileName(r.Name, r.Ext) }

func fileName(name, ext string) string {
	if ext == "" {
		return name
	}
	return name + "." + ext
}

// -------------------------
// Global in-memory cache
// -------------------------

var cachedFiles []FileInfo
var cacheLoaded bool

// Load returns the games database, reading it from the SD card only the
// first time: the menu, search and attract mode all share this one copy.
// It must not be changed in place.
func Load() ([]FileInfo, error) { return loadAll() }

// ForgetCache lets go of the kept copy, for a program that has taken what
// it needs from it (attract mode keeps only the games it can play). A
// later Load reads the SD card again.
func ForgetCache() {
	cachedFiles, cacheLoaded = nil, false
}

func loadAll() ([]FileInfo, error) {
	// Read once, then kept (see Load).
	if cacheLoaded {
		return cachedFiles, nil
	}

	data, err := os.ReadFile(config.MenuDb)
	if err != nil {
		return nil, err
	}
	var files []FileInfo
	if bytes.HasPrefix(data, []byte(dbMagic)) {
		if files, err = readDB(data); err != nil {
			return nil, err
		}
	} else {
		// An older games.db: read it, and save it in the new format so
		// it loads fast from now on (unless a build is running, which
		// writes one itself). Not saving it just means trying next time.
		if files, err = readLegacyDB(data); err != nil {
			return nil, err
		}
		if unlock, ok := tryLockBuild(); ok {
			_ = saveAll(files)
			unlock()
		}
	}

	cachedFiles = files
	cacheLoaded = true
	return cachedFiles, nil
}

// shareStrings makes games share the text they have in common, straight
// after the database is read. Reading gives every game its own copy of
// everything: its system ID, extension and genres (the same few words
// over and over), and its name (already there at the end of its path).
// Sharing them makes the database about 40% smaller in memory, and leaves
// far fewer pieces for Go's memory clean-up to go through every time it
// runs, for as long as the menu is open.
func shareStrings(files []FileInfo) {
	words := map[string]string{}
	share := func(s string) string {
		if s == "" {
			return s
		}
		if w, ok := words[s]; ok {
			return w
		}
		words[s] = s
		return s
	}
	lists := map[string][]string{} // genre lists, by their genres joined
	var key []byte
	for i := range files {
		f := &files[i]
		f.SystemId = share(f.SystemId)
		f.MenuDir = share(f.MenuDir)
		f.Ext = share(f.Ext)
		f.Rotation = share(f.Rotation)
		// The name is the end of the path: "<name>.<ext>" (or "<name>").
		tail := len(f.Name)
		if f.Ext != "" {
			tail += 1 + len(f.Ext)
		}
		if start := len(f.Path) - tail; start >= 0 && f.Path[start:start+len(f.Name)] == f.Name &&
			(f.Ext == "" || f.Path[start+len(f.Name)] == '.' && f.Path[start+len(f.Name)+1:] == f.Ext) {
			f.Name = f.Path[start : start+len(f.Name)]
		}
		if len(f.Genres) > 0 {
			key = key[:0]
			for _, g := range f.Genres {
				key = append(append(key, g...), 0)
			}
			if l, ok := lists[string(key)]; ok {
				f.Genres = l
			} else {
				for j, g := range f.Genres {
					f.Genres[j] = share(g)
				}
				lists[string(key)] = f.Genres
			}
		}
	}
}

// saveAll writes the database to a temporary file first and then swaps it
// in (an instant rename), so the old database stays usable until the new
// one is complete, an interrupted build changes nothing, and nothing ever
// reads a half-written file.
func saveAll(files []FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(config.MenuDb), 0755); err != nil {
		return err
	}
	tmp := config.MenuDb + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := writeDB(f, files); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, config.MenuDb)
}

// buildLockFile makes database builds take turns: only one runs at a time,
// whoever started it (the menu, attract mode, -rebuild, -random...). It's
// an flock, so it's let go automatically if a build is killed.
const buildLockFile = "/tmp/SAMenu_dbbuild.lock"

// lockBuild takes the build lock, calling waiting and waiting for it if
// another build has it. It reports whether it had to wait.
func lockBuild(waiting func()) (unlock func(), waited bool) {
	f, err := os.OpenFile(buildLockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return func() {}, false // no lock possible: build anyway
	}
	fd := int(f.Fd())
	if syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		waiting()
		waited = true
		if syscall.Flock(fd, syscall.LOCK_EX) != nil {
			f.Close()
			return func() {}, true
		}
	}
	return func() {
		_ = syscall.Flock(fd, syscall.LOCK_UN)
		f.Close()
	}, waited
}

// tryLockBuild takes the build lock only if nobody has it.
func tryLockBuild() (unlock func(), ok bool) {
	f, err := os.OpenFile(buildLockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, false
	}
	fd := int(f.Fd())
	if syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		f.Close()
		return nil, false
	}
	return func() {
		_ = syscall.Flock(fd, syscall.LOCK_UN)
		f.Close()
	}, true
}

// -------------------------
// Indexing
// -------------------------

func NewNamesIndex(cfg *config.Config, systems []games.System, update func(IndexStatus)) error {
	// One build at a time. If another was running, use what it built.
	unlock, waited := lockBuild(func() { update(IndexStatus{Waiting: true}) })
	defer unlock()
	if waited {
		cacheLoaded = false
		if _, err := loadAll(); err == nil {
			return nil
		}
	}

	status := IndexStatus{
		Total: len(systems) + 1,
		Step:  1,
	}
	update(status)

	var allFiles []FileInfo

	// [Database] Exclude drops whole systems; [Database.X] rules drop games.
	excluded, _ := games.ResolveSystems(cfg.Database.Exclude)
	rules := NewRuleSet(cfg.DatabaseRules)
	finder := games.NewSystemPathFinder(cfg) // each folder read once, for all systems
	genres := NewGenreFinder(cfg)            // each folder's genres worked out once

	for _, sys := range systems {
		status.SystemId = sys.Id
		status.Step++
		status.Skipped = excluded[strings.ToLower(sys.Id)]
		update(status)
		if status.Skipped {
			continue
		}
		files, err := scanSystem(finder, genres, sys, rules)
		if err != nil {
			return err
		}
		allFiles = append(allFiles, files...)
		status.Files = len(allFiles)
	}

	status.Step++
	status.Skipped = false
	status.Doing = "Saving the games database..."
	update(status)

	FillRotations(allFiles)
	if err := saveAll(allFiles); err != nil {
		return err
	}

	// Update in-memory cache immediately after building
	cachedFiles = allFiles
	cacheLoaded = true

	return nil
}

// scanSystem finds one system's games in all its folders, as the database
// holds them.
func scanSystem(finder *games.SystemPathFinder, genres *GenreFinder, sys games.System, rules RuleSet) ([]FileInfo, error) {
	var out []FileInfo
	menuDirs := map[string]string{} // one shared copy of each menu folder
	for _, sp := range finder.Paths(sys) {
		pathFiles, err := games.GetFiles(sys.Id, sp.Path)
		if err != nil {
			return nil, fmt.Errorf("error getting files: %v", err)
		}
		for _, fullPath := range pathFiles {
			base := filepath.Base(fullPath)
			ext := strings.TrimPrefix(filepath.Ext(base), ".")
			name := strings.TrimSuffix(base, filepath.Ext(base))
			menuPath := menuPathFor(sys, sp.Path, fullPath)
			menuDir := ""
			if i := strings.LastIndexByte(menuPath, '/'); i >= 0 {
				menuDir = menuPath[:i]
				if shared, ok := menuDirs[menuDir]; ok {
					menuDir = shared
				} else {
					menuDir = string([]byte(menuDir)) // not part of this game's path text
					menuDirs[menuDir] = menuDir
				}
			}
			file := FileInfo{
				SystemId: sys.Id,
				Name:     name,
				Ext:      ext,
				Path:     fullPath,
				MenuDir:  menuDir,
			}
			if rules.Excludes(file) {
				continue
			}
			if strings.EqualFold(ext, "mra") {
				file.Rotation = ReadRotation(fullPath)
			}
			file.Genres = genres.For(menuPath, base)
			out = append(out, file)
		}
	}
	return out, nil
}

// UpdateSystems changes the database for systems ticked on or off in
// [Database] Exclude, without a full rebuild: the removed systems' games are
// dropped (no scanning at all), and only the added systems are scanned. The
// other systems' games stay as they were (a full rebuild picks up games
// copied in or deleted since). Games stay in the systems list's order, as a
// full build has them.
func UpdateSystems(cfg *config.Config, added []games.System, removed map[string]bool, update func(IndexStatus)) error {
	unlock, waited := lockBuild(func() { update(IndexStatus{Waiting: true}) })
	defer unlock()

	status := IndexStatus{Total: len(added) + 2, Step: 1}
	// The database the menu already has, unless another build just ran (or
	// nothing's loaded yet): then it's read from the SD card again.
	if waited || !cacheLoaded {
		status.Doing = "Reading the games database..."
		update(status)
		cacheLoaded = false
	}
	current, err := loadAll()
	if err != nil {
		return err
	}
	if len(removed) > 0 {
		var names []string
		for id := range removed {
			names = append(names, games.DisplayName(id))
		}
		sort.Strings(names)
		status.Doing = "Removing " + strings.Join(names, ", ") + "..."
		update(status)
	}
	rules := NewRuleSet(cfg.DatabaseRules)
	finder := games.NewSystemPathFinder(cfg)
	genres := NewGenreFinder(cfg)

	bySystem := map[string][]FileInfo{} // lower-case system ID -> games
	for _, f := range current {
		id := strings.ToLower(f.SystemId)
		if !removed[id] {
			bySystem[id] = append(bySystem[id], f)
		}
	}
	status.Doing = ""
	for _, sys := range added {
		status.SystemId = sys.Id
		status.Step++
		update(status)
		files, err := scanSystem(finder, genres, sys, rules)
		if err != nil {
			return err
		}
		FillRotations(files)
		bySystem[strings.ToLower(sys.Id)] = files
		status.Files += len(files)
	}

	status.Step++
	status.Doing = "Saving the games database..."
	update(status)
	var all []FileInfo
	for _, sys := range games.AllSystems() {
		id := strings.ToLower(sys.Id)
		all = append(all, bySystem[id]...)
		delete(bySystem, id)
	}
	for _, f := range current { // any left: systems not in the list, kept as they were
		if files, ok := bySystem[strings.ToLower(f.SystemId)]; ok {
			all = append(all, files...)
			delete(bySystem, strings.ToLower(f.SystemId))
		}
	}
	if err := saveAll(all); err != nil {
		return err
	}
	cachedFiles, cacheLoaded = all, true
	return nil
}

// menuPathFor builds a game's menu path, e.g. "SNES/RPG/Chrono Trigger.sfc":
// the system's name, then the game's folders inside the system folder that
// was scanned (root), so any folder layout keeps its subfolders.
func menuPathFor(sys games.System, root, fullPath string) string {
	rel, err := filepath.Rel(root, fullPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filepath.Join(sys.Name, filepath.Base(fullPath)))
	}
	relParts := strings.Split(filepath.ToSlash(rel), "/")

	// Case 1: collapse a fake .zip folder (e.g. "N64.zip/...")
	if len(relParts) > 1 && strings.HasSuffix(strings.ToLower(relParts[0]), ".zip") {
		relParts = relParts[1:]
	}

	// Case 2: listings/*.txt → label
	if len(relParts) > 1 && relParts[0] == "listings" && strings.HasSuffix(relParts[1], ".txt") {
		label := strings.TrimSuffix(relParts[1], ".txt")
		if len(label) > 0 {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
		relParts = append([]string{label}, relParts[2:]...)
	}

	return filepath.ToSlash(filepath.Join(append([]string{sys.Name}, relParts...)...))
}

// -------------------------
// Searching
// -------------------------

// Search keeps a lowercase copy of every game's name and extension, so a
// search just compares: it doesn't lowercase 100,000-odd names every time
// (5x faster on a MiSTer). PrepareSearch makes it in the background once
// the menu is built (any earlier and, on the MiSTer's two cores, it slows
// the menu's start), so even the first search is quick. It's made again if
// the database changes.
var (
	lowerMu    sync.Mutex
	lowerFor   *FileInfo // the list the copy was made from (its first game)
	lowerNames []string
	lowerExts  []string
)

// PrepareSearch makes the lowercase copy for files in the background.
// files must be the database as Load returned it. A search that comes
// first makes the copy itself, and this then finds it done.
func PrepareSearch(files []FileInfo) {
	go lowered(files)
}

// lowered returns the lowercase names and extensions for files, making them
// first if they're not ready. A search that arrives while PrepareSearch is
// still working just waits for it.
func lowered(files []FileInfo) ([]string, []string) {
	lowerMu.Lock()
	defer lowerMu.Unlock()
	if len(files) == 0 {
		return nil, nil
	}
	if lowerFor != &files[0] || len(lowerNames) != len(files) {
		names := make([]string, len(files))
		exts := make([]string, len(files))
		for i, f := range files {
			names[i] = strings.ToLower(f.Name)
			exts[i] = strings.ToLower(f.Ext)
		}
		lowerFor, lowerNames, lowerExts = &files[0], names, exts
	}
	return lowerNames, lowerExts
}

// searchGeneric returns every game whose lowercase name and extension pass
// test, once per system (the same file on two drives only once).
func searchGeneric(test func(name, ext string) bool) ([]SearchResult, error) {
	files, err := loadAll()
	if err != nil {
		return nil, err
	}
	names, exts := lowered(files)

	type key struct{ sys, name, ext string }
	results := make([]SearchResult, 0, 128)
	seen := make(map[key]bool)

	for i, f := range files {
		if !test(names[i], exts[i]) {
			continue
		}
		k := key{strings.ToLower(f.SystemId), names[i], exts[i]}
		if seen[k] {
			continue
		}
		seen[k] = true
		results = append(results, SearchResult{
			SystemId: f.SystemId,
			Name:     f.Name,
			Ext:      f.Ext,
			Path:     f.Path,
		})
	}
	return results, nil
}

// wordsTest turns a search query into a test on lowercase names and
// extensions: every word must be in the name; words starting with a dot
// (".nes") filter by extension instead.
func wordsTest(query string) func(name, ext string) bool {
	var words, extFilters []string
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if strings.HasPrefix(w, ".") && len(w) > 1 {
			extFilters = append(extFilters, w[1:])
		} else {
			words = append(words, w)
		}
	}
	return func(name, ext string) bool {
		if len(extFilters) > 0 {
			match := false
			for _, e := range extFilters {
				if e == ext {
					match = true
					break
				}
			}
			if !match {
				return false
			}
		}
		for _, w := range words {
			if !strings.Contains(name, w) {
				return false
			}
		}
		return true
	}
}

// SearchNamesWords finds the games whose names contain every word of the
// query (ignoring case). Words starting with a dot (".nes") filter by
// extension instead.
func SearchNamesWords(query string) ([]SearchResult, error) {
	return searchGeneric(wordsTest(query))
}

// CountNamesWords counts the games SearchNamesWords would find, without
// building the results list (for a live count while typing). skip leaves
// out games the caller would hide from the results (nil = none), so the
// count matches exactly what the caller shows.
func CountNamesWords(query string, skip func(name string) bool) (int, error) {
	files, err := loadAll()
	if err != nil {
		return 0, err
	}
	names, exts := lowered(files)
	test := wordsTest(query)

	type key struct{ sys, name, ext string }
	seen := make(map[key]bool)
	for i, f := range files {
		if !test(names[i], exts[i]) || (skip != nil && skip(f.Name)) {
			continue
		}
		seen[key{strings.ToLower(f.SystemId), names[i], exts[i]}] = true
	}
	return len(seen), nil
}
