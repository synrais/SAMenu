package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/favourites"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
)

// -------------------------
// [History] and [Favourites]
// -------------------------
//
// Two folders on the systems list ([Menu] HistoryEntry / Favourites):
//
//   - [History]: the games you launch from SAMenu (not attract mode's),
//     newest first, each with the day it was played ("Today",
//     "Yesterday", "26 Sep"). Kept on the SD card, so it survives a
//     restart. HistoryMoveToTop: a game played again moves to the top
//     instead of being listed twice. HistoryKeep: how many are kept.
//   - [Favourites]: games you mark with the Fav button (Square) in any game
//     list, where they show a "* " (it doesn't affect sorting). In
//     [Favourites], Remove (Triangle) takes one off.

const (
	historyLabel    = "[History]"
	favouritesLabel = "[Favourites]"
	historyFile     = config.SAMFolder + "/history.json"
)

type historyEntry struct {
	Game MenuFile
	At   time.Time
}

func loadJSON(path string, v interface{}) {
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, v)
	}
}

func saveJSON(path string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---- History ----

func loadHistory() []historyEntry {
	var h []historyEntry
	loadJSON(historyFile, &h)
	return h
}

// recordHistory notes a game launched from SAMenu (newest first).
func recordHistory(cfg *config.Config, f MenuFile) {
	if !cfg.Menu.HistoryEntry {
		return
	}
	h := loadHistory()
	if cfg.Menu.HistoryMoveToTop {
		kept := h[:0]
		for _, e := range h {
			if e.Game.Path != f.Path {
				kept = append(kept, e)
			}
		}
		h = kept
	}
	h = append([]historyEntry{{Game: f, At: time.Now()}}, h...)
	if keep := cfg.Menu.HistoryKeep; keep > 0 && len(h) > keep {
		h = h[:keep]
	}
	_ = saveJSON(historyFile, h)
}

// dayLabel is when a game was played: "Today", "Yesterday", "26 Sep", or
// with the year for an earlier year.
func dayLabel(t, now time.Time) string {
	y1, m1, d1 := t.Date()
	y2, m2, d2 := now.Date()
	today := time.Date(y2, m2, d2, 0, 0, 0, 0, now.Location())
	day := time.Date(y1, m1, d1, 0, 0, 0, 0, now.Location())
	switch {
	case day.Equal(today):
		return "Today"
	case day.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	case y1 == y2:
		return t.Format("2 Jan")
	}
	return t.Format("2 Jan 2006")
}

// withSystem puts a game's system in its name, the way a setting says:
// Before ("[SNES] Title"), After ("Title [SNES]") or Off.
func withSystem(style, sysName, title string) string {
	switch style {
	case "After":
		return title + " [" + sysName + "]"
	case "Off":
		return title
	}
	return "[" + sysName + "] " + title
}

func systemName(id string) string {
	if s, err := games.GetSystem(id); err == nil {
		return s.Name
	}
	return id
}

// buildHistoryNode is the [History] folder. With attract mode's history
// for this session (in /tmp), two folders: Attract and Games Menu (each
// only if it has games); without it, straight into the games menu's.
func buildHistoryNode(cfg *config.Config) *gamesdb.Node {
	menu := buildMenuHistoryNode(cfg)
	played := attract.SessionHistory()
	if len(played) == 0 {
		return menu
	}
	root := &gamesdb.Node{Name: "History", Children: map[string]*gamesdb.Node{}}
	att := &gamesdb.Node{Name: "Attract", Children: map[string]*gamesdb.Node{}}
	for i, f := range played {
		keepOriginal(f)
		f.Name = withSystem(cfg.Menu.HistorySystem, systemName(f.SystemId), f.Name)
		att.Files = append(att.Files, f)
		att.SortKeys = append(att.SortKeys, fmt.Sprintf("%06d", i)) // newest first
	}
	root.Children["Attract"] = att
	if len(menu.Files) > 0 {
		menu.Name = "Games Menu"
		root.Children["Games Menu"] = menu
	}
	return root
}

// hasHistory reports whether [History] has anything to show.
func hasHistory() bool { return len(loadHistory()) > 0 || len(attract.SessionHistory()) > 0 }

// buildMenuHistoryNode is the games menu's history: newest first, with the
// day.
func buildMenuHistoryNode(cfg *config.Config) *gamesdb.Node {
	node := &gamesdb.Node{Name: "History", Children: map[string]*gamesdb.Node{}}
	now := time.Now()
	for i, e := range loadHistory() {
		f := e.Game
		keepOriginal(f)
		f.Name = fmt.Sprintf("%-9s  %s", dayLabel(e.At, now), withSystem(cfg.Menu.HistorySystem, systemName(f.SystemId), f.Name))
		node.Files = append(node.Files, f)
		node.SortKeys = append(node.SortKeys, fmt.Sprintf("%06d", i)) // as stored: newest first
	}
	return node
}

// ---- Favourites ----

func loadFavourites() []MenuFile { return favourites.Load() }

func isFavourite(path string) bool { return favourites.Is(path) }

func toggleFavourite(f MenuFile) bool { return favourites.Toggle(f) }

// buildFavouritesNode is the [Favourites] folder, by title.
func buildFavouritesNode(cfg *config.Config) *gamesdb.Node {
	node := &gamesdb.Node{Name: "Favourites", Children: map[string]*gamesdb.Node{}}
	favs := loadFavourites()
	sort.Slice(favs, func(i, j int) bool { return strings.ToLower(favs[i].Name) < strings.ToLower(favs[j].Name) })
	for i, f := range favs {
		keepOriginal(f)
		f.Name = withSystem(cfg.Menu.FavouritesSystem, systemName(f.SystemId), f.Name)
		node.Files = append(node.Files, f)
		node.SortKeys = append(node.SortKeys, fmt.Sprintf("%06d", i))
	}
	favouritesNode = node
	return node
}

// favouriteMark is how a favourite's name starts in the game lists.
const favouriteMark = "* "

// ---- Settings screens (Display & Sorting) ----

var historyKeeps = []string{"25", "50", "100", "250"}

func historyScreen(stdscr *gc.Window, cfg *config.Config) {
	m := &cfg.Menu
	show := onOffOption("Show [History]", m.HistoryEntry)
	top := onOffOption("Played again: to the top", m.HistoryMoveToTop)
	keep := labelOption{"Keep", historyKeeps, 2}
	keep.set(strconv.Itoa(m.HistoryKeep))
	sys := labelOption{"System names", []string{"Before", "After", "Off"}, 0}
	sys.set(m.HistorySystem)
	clear := labelOption{"Clear history now", []string{"No", "Yes"}, 0}
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title: "[History]",
		options: func() []*labelOption {
			if !show.isOn() {
				return []*labelOption{&show}
			}
			return []*labelOption{&show, &top, &keep, &sys, &clear}
		},
		preview: func() []string {
			if !show.isOn() {
				return []string{"[History] isn't shown, and nothing is recorded."}
			}
			return []string{
				"[History]",
				"  Today      " + withSystem(sys.value(), "SNES", "Street Fighter II Turbo"),
				"  Yesterday  " + withSystem(sys.value(), "Playstation", "Tekken 3"),
				"  26 Sep     " + withSystem(sys.value(), "NES", "Duck Tales"),
			}
		},
		save: func() error {
			m.HistoryEntry, m.HistoryMoveToTop, m.HistorySystem = show.isOn(), top.isOn(), sys.value()
			m.HistoryKeep, _ = strconv.Atoi(keep.value())
			if clear.value() == "Yes" {
				_ = os.Remove(historyFile)
				clear.index = 0
			}
			return config.SaveValues(cfg.Path, "Menu", [][2]string{
				{"HistoryEntry", strconv.FormatBool(m.HistoryEntry)},
				{"HistoryMoveToTop", strconv.FormatBool(m.HistoryMoveToTop)},
				{"HistoryKeep", strconv.Itoa(m.HistoryKeep)},
				{"HistorySystem", m.HistorySystem},
			})
		},
	})
}

func favouritesScreen(stdscr *gc.Window, cfg *config.Config) {
	m := &cfg.Menu
	on := onOffOption("Favourites", m.Favourites)
	sys := labelOption{"System names", []string{"Before", "After", "Off"}, 0}
	sys.set(m.FavouritesSystem)
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title: "[Favourites]",
		options: func() []*labelOption {
			if !on.isOn() {
				return []*labelOption{&on}
			}
			return []*labelOption{&on, &sys}
		},
		preview: func() []string {
			if !on.isOn() {
				return []string{"Favourites are off: no Fav button, no * marks,", "no [Favourites] folder."}
			}
			return []string{
				"In a game list, Square (Fav) marks a game:",
				"  " + favouriteMark + "Street Fighter II Turbo",
				"[Favourites] lists them; Triangle removes one:",
				"  " + withSystem(sys.value(), "SNES", "Street Fighter II Turbo"),
			}
		},
		save: func() error {
			m.Favourites, m.FavouritesSystem = on.isOn(), sys.value()
			return config.SaveValues(cfg.Path, "Menu", [][2]string{
				{"Favourites", strconv.FormatBool(m.Favourites)},
				{"FavouritesSystem", m.FavouritesSystem},
			})
		},
	})
}

// Virtual folders ([History], [Favourites], [Genre Collection]) show
// games under changed names (a day, a system). originals keeps each one's
// real entry by path, so recording or marking it uses the real game.
var originals = map[string]MenuFile{}

func keepOriginal(f MenuFile) { originals[f.Path] = f }

func originalGame(f MenuFile) MenuFile {
	if o, ok := originals[f.Path]; ok {
		return o
	}
	return f
}

// favouritesNode is the [Favourites] folder being shown, if any (Remove
// works there).
var favouritesNode *gamesdb.Node
