// Package favourites keeps the favourite games, on the SD card, for the
// games menu ([Favourites], the Fav button) and attract mode (its
// Favourite action).
package favourites

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/gamesdb"
)

// File is where the favourites are kept.
const File = config.SAMFolder + "/favourites.json"

var (
	mu    sync.Mutex
	paths map[string]bool // loaded once, kept in step with the file
)

// Load reads the favourites.
func Load() []gamesdb.FileInfo {
	var f []gamesdb.FileInfo
	if b, err := os.ReadFile(File); err == nil {
		_ = json.Unmarshal(b, &f)
	}
	return f
}

func save(f []gamesdb.FileInfo) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(File), 0755)
	tmp := File + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, File)
}

func loadPaths() {
	if paths == nil {
		paths = map[string]bool{}
		for _, f := range Load() {
			paths[f.Path] = true
		}
	}
}

// Is reports whether a game is a favourite.
func Is(path string) bool {
	mu.Lock()
	defer mu.Unlock()
	loadPaths()
	return paths[path]
}

// Toggle adds a game to the favourites, or takes it off; it reports
// whether it's a favourite now.
func Toggle(g gamesdb.FileInfo) bool {
	mu.Lock()
	defer mu.Unlock()
	loadPaths()
	favs := Load()
	if paths[g.Path] {
		kept := favs[:0]
		for _, x := range favs {
			if x.Path != g.Path {
				kept = append(kept, x)
			}
		}
		favs = kept
		delete(paths, g.Path)
	} else {
		favs = append(favs, g)
		paths[g.Path] = true
	}
	_ = save(favs)
	return paths[g.Path]
}
