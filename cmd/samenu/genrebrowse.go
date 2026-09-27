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
// gamesdb). [Genres] > Fighting lists every fighting game, each marked
// with its system; a genre with sub-genres shows them as folders
// (Sports > Golf) beside its other games.

const genresLabel = "[Genres]"

// How [Genres] shows games: the system Before, After or Off, and the order
// by "Game name" or "System" (the systems list's own order). Set from
// [Menu] by applyMenuConfig and the [Genres] screen.
var genresSystem, genresOrder = "Before", "Game name"

// buildGenreTree builds the [Genres] folder from the games.
func buildGenreTree(files []MenuFile) *gamesdb.Node {
	root := &gamesdb.Node{Name: "Genres", Children: map[string]*gamesdb.Node{}}
	sysName := map[string]string{}
	nameOf := func(id string) string {
		name, ok := sysName[id]
		if !ok {
			name = id
			if s, err := games.GetSystem(id); err == nil {
				name = s.Name
			}
			sysName[id] = name
		}
		return name
	}
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
		n.Files = append(n.Files, f)
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
func genresScreen(stdscr *gc.Window, cfg *config.Config) {
	show := onOffOption("Show [Genres]", cfg.Menu.GenresEntry)
	where := labelOption{"System names", []string{"Before", "After", "Off"}, 0}
	where.set(genresSystem)
	order := labelOption{"Order", []string{"Game name", "System"}, 0}
	order.set(genresOrder)
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title: "[Genres]",
		options: func() []*labelOption {
			if !show.isOn() {
				return []*labelOption{&show}
			}
			return []*labelOption{&show, &where, &order}
		},
		preview: func() []string {
			if !show.isOn() {
				return []string{"[Genres] isn't shown on the systems list."}
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
			rows := [][2]string{{"SNES", "Street Fighter II"}, {"Saturn", "Darkstalkers"}, {"SNES", "Killer Instinct"}, {"Saturn", "X-Men vs. Street Fighter"}}
			if order.value() == "System" {
				rows = [][2]string{{"SNES", "Killer Instinct"}, {"SNES", "Street Fighter II"}, {"Saturn", "Darkstalkers"}, {"Saturn", "X-Men vs. Street Fighter"}}
			} else {
				rows = [][2]string{{"Saturn", "Darkstalkers"}, {"SNES", "Killer Instinct"}, {"SNES", "Street Fighter II"}, {"Saturn", "X-Men vs. Street Fighter"}}
			}
			out := []string{"[Genres] > Fighting"}
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
			})
		},
	})
}
