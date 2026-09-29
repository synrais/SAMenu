package main

import (
	"fmt"
	"strconv"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"

	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
)

// -------------------------
// [Genres]
// -------------------------
//
// A virtual folder on the systems list ([Menu] GenresEntry, Options ->
// Display & Sorting -> [Genres]): every game by genre, across all
// systems, from the genres read into the games database (genres.go in
// gamesdb). [Genre Collection] > Fighting lists every fighting game, each marked
// with its system; a genre with sub-genres shows them as folders
// (Sports > Golf) beside its other games.

const genresLabel = "[Genre Collection]"

// How [Genres] shows games: the system Before, After or Off, and the order
// by "Game name" or "System" (the systems list's own order). Set from
// [Menu] by applyMenuConfig and the [Genres] screen.
var genresSystem, genresOrder = "Before", "Game name"

// genresLeftOut is [Menu] GenresExclude: systems left out of [Genres]
// (lower-case IDs). They still appear everywhere else in the menu.
var genresLeftOut = map[string]bool{}

// buildGenreTree builds the [Genres] folder from the games.
func buildGenreTree(files []MenuFile) *gamesdb.Node {
	if len(genresLeftOut) > 0 {
		kept := make([]MenuFile, 0, len(files))
		for i := range files {
			if !genresLeftOut[strings.ToLower(files[i].SystemId)] {
				kept = append(kept, files[i])
			}
		}
		files = kept
	}
	root := &gamesdb.Node{Name: "Genres", Children: map[string]*gamesdb.Node{}}
	nameOf := games.DisplayName
	// Systems in the systems list's order, for Order: System.
	var names []string
	seenSys := map[string]bool{}
	for _, f := range files {
		if len(f.Genres) > 0 && !seenSys[f.SystemId] {
			seenSys[f.SystemId] = true
			names = append(names, nameOf(f.SystemId))
		}
	}
	sortSystems(names)
	rank := map[string]int{}
	for i, n := range names {
		rank[n] = i
	}

	seen := map[*gamesdb.Node]map[string]bool{}
	folder := func(parent *gamesdb.Node, name string) *gamesdb.Node {
		n, ok := parent.Children[name]
		if !ok {
			n = &gamesdb.Node{Name: name, Children: map[string]*gamesdb.Node{}}
			parent.Children[name] = n
			seen[n] = map[string]bool{}
		}
		return n
	}
	add := func(n *gamesdb.Node, f MenuFile) {
		// Once per system per folder, however many genre folders the same
		// game sits in.
		key := strings.ToLower(f.SystemId + "|" + f.Name + "." + f.Ext)
		if seen[n][key] {
			return
		}
		seen[n][key] = true
		keepOriginal(f)
		sys := nameOf(f.SystemId)
		title := f.Name
		switch genresSystem {
		case "After":
			f.Name = title + " [" + sys + "]"
		case "Off":
		default: // Before
			f.Name = "[" + sys + "] " + title
		}
		// Sort by title (the system settles ties), or by system then title.
		sortKey := title + "\x00" + fmt.Sprintf("%04d", rank[sys])
		if genresOrder == "System" {
			sortKey = fmt.Sprintf("%04d", rank[sys]) + "\x00" + title
		}
		n.Files = append(n.Files, &f)
		n.SortKeys = append(n.SortKeys, sortKey)
	}
	for _, f := range files {
		if len(f.Genres) == 0 {
			continue
		}
		hasSub := map[string]bool{}
		for _, g := range f.Genres {
			if p := gamesdb.GenreParent(g); p != "" {
				hasSub[p] = true
			}
		}
		for _, g := range f.Genres {
			if p := gamesdb.GenreParent(g); p != "" {
				add(folder(folder(root, p), g[len(p)+1:]), f)
			} else if !hasSub[g] {
				add(folder(root, g), f)
			}
		}
	}
	return root
}

// genresScreen is Options -> Display & Sorting -> [Genres].
func genresScreen(stdscr *gc.Window, cfg *config.Config, files []MenuFile) {
	show := onOffOption("Show [Genre Collection]", cfg.Menu.GenresEntry)
	// Leave out systems: a tick list, its systems shown here.
	leaveOut := labelOption{"Leave out systems", []string{""}, 0}
	leaveOutText := func() {
		leaveOut.values[0] = "none"
		if len(cfg.Menu.GenresExclude) > 0 {
			names := make([]string, len(cfg.Menu.GenresExclude))
			for i, id := range cfg.Menu.GenresExclude {
				names[i] = games.DisplayName(id)
			}
			leaveOut.values[0] = strings.Join(names, ", ")
		}
	}
	leaveOutText()
	where := labelOption{"System names", []string{"Before", "After", "Off"}, 0}
	where.set(genresSystem)
	order := labelOption{"Order", []string{"Game name", "System"}, 0}
	order.set(genresOrder)
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title: "[Genre Collection]",
		options: func() []*labelOption {
			if !show.isOn() {
				return []*labelOption{&show}
			}
			return []*labelOption{&show, &where, &order, &leaveOut}
		},
		changed: func(o *labelOption) {
			if o != &leaveOut {
				return
			}
			ids := systemIDsIn(files)
			on := map[string]bool{}
			var kept []string // groups written in the ini: kept as they are
			for _, x := range cfg.Menu.GenresExclude {
				found := false
				for _, id := range ids {
					if strings.EqualFold(x, id) {
						on[id], found = true, true
					}
				}
				if !found {
					kept = append(kept, x)
				}
			}
			tickSystems(stdscr, "Leave out systems", ids, on, func(on map[string]bool) {
				out := append([]string(nil), kept...)
				for _, id := range ids {
					if on[id] {
						out = append(out, id)
					}
				}
				cfg.Menu.GenresExclude = out
				genresLeftOut, _ = games.ResolveSystems(out)
				treeDirty = true // rebuild [Genres] without them
			})
			leaveOutText()
		},
		preview: func() []string {
			if !show.isOn() {
				return []string{"[Genre Collection] isn't shown on the systems list."}
			}
			sample := func(sys, title string) string {
				switch where.value() {
				case "After":
					return title + " [" + sys + "]"
				case "Off":
					return title
				}
				return "[" + sys + "] " + title
			}
			var rows [][2]string
			if order.value() == "System" {
				rows = [][2]string{{"SNES", "Killer Instinct"}, {"SNES", "Street Fighter II"}, {"Saturn", "Darkstalkers"}, {"Saturn", "X-Men vs. Street Fighter"}}
			} else {
				rows = [][2]string{{"Saturn", "Darkstalkers"}, {"SNES", "Killer Instinct"}, {"SNES", "Street Fighter II"}, {"Saturn", "X-Men vs. Street Fighter"}}
			}
			out := []string{"[Genre Collection] > Fighting"}
			for _, r := range rows {
				out = append(out, "  "+gameName(sample(r[0], r[1]), "sfc"))
			}
			return out
		},
		save: func() error {
			cfg.Menu.GenresEntry = show.isOn()
			cfg.Menu.GenresSystem, cfg.Menu.GenresOrder = where.value(), order.value()
			genresSystem, genresOrder = where.value(), order.value()
			return config.SaveValues(cfg.Path, "Menu", [][2]string{
				{"GenresEntry", strconv.FormatBool(cfg.Menu.GenresEntry)},
				{"GenresSystem", cfg.Menu.GenresSystem},
				{"GenresOrder", cfg.Menu.GenresOrder},
				{"GenresExclude", strings.Join(cfg.Menu.GenresExclude, ", ")},
			})
		},
	})
}
