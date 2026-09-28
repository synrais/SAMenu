package games

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAMenu/pkg/config"
)

func GetGamesFolders(cfg *config.Config) []string {
	var folders []string
	for _, folder := range cfg.Systems.GamesFolder {
		folder = filepath.Clean(folder)
		if !strings.HasSuffix(folder, "/games") {
			folders = append(folders, filepath.Join(folder, "games"))
		}
		folders = append(folders, folder)
	}
	folders = append(folders, config.GamesFolders...)
	return folders
}

// SystemFolders returns the extra folders set for one system with
// "system_folder = ID:/path" lines in SAMenu.ini. Each is that system's own
// folder, scanned as-is on top of its normal folders.
func SystemFolders(cfg *config.Config, system System) []string {
	var folders []string
	for _, entry := range cfg.Systems.SystemFolder {
		id, folder, ok := strings.Cut(entry, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(id), system.Id) {
			continue
		}
		if folder = strings.TrimSpace(folder); folder != "" {
			folders = append(folders, filepath.Clean(folder))
		}
	}
	return folders
}

func FindFile(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	parent := filepath.Dir(path)
	name := filepath.Base(path)

	files, err := os.ReadDir(parent)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		target := file.Name()

		if len(target) != len(name) {
			continue
		} else if strings.EqualFold(target, name) {
			return filepath.Join(parent, target), nil
		}
	}

	return "", fmt.Errorf("file match not found: %s", path)
}

// FolderToSystems returns what systems a path could be for.
func FolderToSystems(cfg *config.Config, path string) []System {
	path = strings.ToLower(path)

	// A game inside a system's own extra folder (system_folder) is that
	// system's.
	var owners []System
	for _, system := range Systems {
		for _, folder := range SystemFolders(cfg, system) {
			f := strings.ToLower(folder)
			if path == f || strings.HasPrefix(path, f+"/") {
				owners = append(owners, system)
				break
			}
		}
	}
	if len(owners) > 0 {
		return owners
	}
	validGamesFolder := false
	gamesFolder := ""

	for _, folder := range GetGamesFolders(cfg) {
		if strings.HasPrefix(path, strings.ToLower(folder)) {
			validGamesFolder = true
			gamesFolder = folder
			break
		}
	}

	if !validGamesFolder {
		return nil
	}

	var validSystems []System
	for _, system := range Systems {
		for _, folder := range system.Folder {
			systemPath := strings.ToLower(filepath.Join(gamesFolder, folder))
			if strings.HasPrefix(path, systemPath) {
				validSystems = append(validSystems, system)
				break
			}
		}
	}

	if strings.HasSuffix(path, "/") {
		return validSystems
	}

	var matchedExtensions []System
	for _, system := range validSystems {
		if MatchSystemFile(system, path) {
			matchedExtensions = append(matchedExtensions, system)
		}
	}

	if len(matchedExtensions) == 0 {
		// fall back to just the folder match
		return validSystems
	}

	return matchedExtensions
}

func BestSystemMatch(cfg *config.Config, path string) (System, error) {
	systems := FolderToSystems(cfg, path)

	if len(systems) == 0 {
		return System{}, fmt.Errorf("no systems found for %s", path)
	}

	if len(systems) == 1 {
		return systems[0], nil
	}

	// check for system matches by file extension if possible
	if filepath.Ext(path) != "" {
		filtered := []System{}
		for _, system := range systems {
			if MatchSystemFile(system, path) {
				filtered = append(filtered, system)
			}
		}

		if len(filtered) > 0 {
			systems = filtered
		}
	}

	// prefer the system with a setname
	for _, system := range systems {
		if system.SetName != "" {
			return system, nil
		}
	}

	// otherwise just return the first one
	return systems[0], nil
}

type PathResult struct {
	Path string
}

// GetSystemPaths returns all possible paths for each system.
func GetSystemPaths(cfg *config.Config, systems []System) []PathResult {
	finder := NewSystemPathFinder(cfg)
	var matches []PathResult
	for _, system := range systems {
		matches = append(matches, finder.Paths(system)...)
	}
	return matches
}

// SystemPathFinder finds systems' folders, as GetSystemPaths does, reading
// each folder's listing only once: a database build looks up every system's
// folder names in the same games folders (on every drive), and each miss
// used to read the whole listing again, thousands of times a build (each a
// round trip on a network drive). Use one for a whole build.
type SystemPathFinder struct {
	cfg          *config.Config
	gamesFolders []string
	listings     map[string][]string // folder -> its entries' names (nil: unreadable)
}

func NewSystemPathFinder(cfg *config.Config) *SystemPathFinder {
	return &SystemPathFinder{cfg: cfg, gamesFolders: GetGamesFolders(cfg), listings: map[string][]string{}}
}

// Paths returns all of a system's folders.
func (f *SystemPathFinder) Paths(system System) []PathResult {
	var matches []PathResult
	seen := make(map[string]bool)
	for _, gamesFolder := range f.gamesFolders {
		gf, err := f.find(gamesFolder)
		if err != nil {
			continue
		}
		for _, folder := range system.Folder {
			path, err := f.find(filepath.Join(gf, folder))
			if err != nil {
				continue
			}
			seen[path] = true
			matches = append(matches, PathResult{Path: path})
		}
	}

	// Extra folders just for this system (system_folder in SAMenu.ini).
	for _, folder := range SystemFolders(f.cfg, system) {
		if info, err := os.Stat(folder); err != nil || !info.IsDir() || seen[folder] {
			continue
		}
		seen[folder] = true
		matches = append(matches, PathResult{Path: folder})
	}
	return matches
}

// find is FindFile from the folder listings read so far: the name as it
// is, or else the same name in different capitals.
func (f *SystemPathFinder) find(path string) (string, error) {
	parent, name := filepath.Dir(path), filepath.Base(path)
	names, read := f.listings[parent]
	if !read {
		if entries, err := os.ReadDir(parent); err == nil {
			names = make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}
		}
		f.listings[parent] = names
	}
	if names == nil {
		return "", fmt.Errorf("can't read %s", parent)
	}
	for _, n := range names {
		if n == name {
			return path, nil
		}
	}
	for _, n := range names {
		if len(n) == len(name) && strings.EqualFold(n, name) {
			return filepath.Join(parent, n), nil
		}
	}
	return "", fmt.Errorf("file match not found: %s", path)
}

// GetActiveSystemPaths returns the active path for each system.
func GetActiveSystemPaths(cfg *config.Config, systems []System) []PathResult {
	var matches []PathResult

	gamesFolders := GetGamesFolders(cfg)
	for _, system := range systems {
		for _, gamesFolder := range gamesFolders {
			gf, err := FindFile(gamesFolder)
			if err != nil {
				continue
			}

			found := false

			for _, folder := range system.Folder {
				systemFolder := filepath.Join(gf, folder)
				path, err := FindFile(systemFolder)
				if err != nil {
					continue
				}

				matches = append(matches, PathResult{Path: path})
				found = true
				break
			}

			if found {
				break
			}
		}

		if len(matches) == len(systems) {
			break
		}
	}

	return matches
}
