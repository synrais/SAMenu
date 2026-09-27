package gamesdb

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
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
	MenuPath string
	// Rotation, for arcade MRAs: "horizontal", "vertical cw",
	// "vertical ccw", "vertical" (direction not stated) or "" (unknown).
	Rotation string
	// Genres, from the folders the game is in (see genres.go), e.g.
	// ["Sports", "Sports/Golf"].
	Genres []string
}

// ListKey is the game's name as the game lists (Blacklist, Staticlist,
// Whitelist) compare it.
func (f FileInfo) ListKey() string { return utils.NormalizeTitle(f.Name) }

type IndexStatus struct {
	Total    int
	Step     int
	SystemId string // while Waiting: a message to show instead
	Files    int
	Waiting  bool // another build is running; this one waits for it
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

func loadAll() ([]FileInfo, error) {
	// If we've already loaded the Gob file once, return the cached version instantly.
	if cacheLoaded {
		return cachedFiles, nil
	}

	f, err := os.Open(config.MenuDb)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var files []FileInfo
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&files); err != nil {
		return nil, err
	}

	cachedFiles = files
	cacheLoaded = true
	return cachedFiles, nil
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
	if err := gob.NewEncoder(f).Encode(files); err != nil {
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

// -------------------------
// Indexing
// -------------------------

func NewNamesIndex(cfg *config.Config, systems []games.System, update func(IndexStatus)) (int, error) {
	// One build at a time. If another was running, use what it built.
	unlock, waited := lockBuild(func() {
		update(IndexStatus{Waiting: true, SystemId: "Waiting for another database build to finish..."})
	})
	defer unlock()
	if waited {
		cacheLoaded = false
		if files, err := loadAll(); err == nil {
			return len(files), nil
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

	for _, sys := range systems {
		status.SystemId = sys.Id
		status.Step++
		update(status)

		if excluded[strings.ToLower(sys.Id)] {
			continue
		}

		sysPaths := games.GetSystemPaths(cfg, []games.System{sys})
		for _, sp := range sysPaths {
			pathFiles, err := games.GetFiles(sys.Id, sp.Path)
			if err != nil {
				return len(allFiles), fmt.Errorf("error getting files: %v", err)
			}

			for _, fullPath := range pathFiles {
				base := filepath.Base(fullPath)
				ext := strings.TrimPrefix(filepath.Ext(base), ".")
				name := strings.TrimSuffix(base, filepath.Ext(base))

				menuPath := menuPathFor(sys, sp.Path, fullPath)

				file := FileInfo{
					SystemId: sys.Id,
					Name:     name,
					Ext:      ext,
					Path:     fullPath,
					MenuPath: menuPath,
				}
				if rules.Excludes(file) {
					continue
				}
				if strings.EqualFold(ext, "mra") {
					file.Rotation = ReadRotation(fullPath)
				}
				file.Genres = GenresFor(cfg, menuPath, base)
				allFiles = append(allFiles, file)
			}
		}
		status.Files = len(allFiles)
	}

	status.Step++
	update(status)

	FillRotations(allFiles)
	if err := saveAll(allFiles); err != nil {
		return len(allFiles), err
	}

	// Update in-memory cache immediately after building
	cachedFiles = allFiles
	cacheLoaded = true

	return len(allFiles), nil
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
func SearchNamesWords(_ []games.System, query string) ([]SearchResult, error) {
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
