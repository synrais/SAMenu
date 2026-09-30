package main

import (
	"fmt"
	"sort"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
)

// -------------------------
// Attract Playlists
// -------------------------
//
// Options -> Attract Mode -> Playlists: genre playlists for attract mode, a
// separate layer over its normal setup (see config/playlists.go). Choose
// the one in use (Normal = the usual setup), create, edit, rename, delete.

func playlistsScreen(stdscr *gc.Window, cfg *config.Config, files []MenuFile) {
	(&menuScreen{title: "Playlists", lines: func() []menuLine {
		names := cfg.PlaylistNames()
		active := "Normal"
		if p := cfg.ActivePlaylist(); p != nil {
			active = p.Name
		}
		lines := []menuLine{setting(settingText("Attract mode plays:", active), func() {
			// Cycle through Normal and the playlists.
			if err := config.SetActivePlaylist(cfg, nextOf(append([]string{"Normal"}, names...), active)); err != nil {
				message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			}
		})}
		for _, n := range names {
			n := n
			lines = append(lines, action("Edit", "Edit: "+n, func() {
				if p := cfg.Playlists[strings.ToLower(n)]; p != nil {
					editPlaylist(stdscr, cfg, files, p)
				}
			}))
		}
		return append(lines, opens("New playlist...", func() {
			name, ok := askPlaylistName(stdscr, cfg, "")
			if !ok {
				return
			}
			p := &config.Playlist{Name: name, Systems: map[string][]string{}, Others: config.OthersLeaveOut}
			if err := config.SavePlaylist(cfg, p); err != nil {
				message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
				return
			}
			editPlaylist(stdscr, cfg, files, p)
		}))
	}}).run(stdscr)
}

// askPlaylistName asks for a new playlist name on the on-screen keyboard.
func askPlaylistName(stdscr *gc.Window, cfg *config.Config, current string) (string, bool) {
	for {
		button, text, err := curses.OnScreenKeyboardWith(stdscr, "Playlist name", []string{"OK", "Cancel"}, current, curses.KeyboardOpts{PadKeys: true})
		if err != nil || button != 0 {
			return "", false
		}
		name := strings.Join(strings.Fields(text), " ")
		switch {
		case name == "":
			return "", false
		case strings.ContainsAny(name, "[]=;#"):
			message(stdscr, "A playlist name can't have [ ] = ; or # in it.")
		case strings.EqualFold(name, "Normal"):
			message(stdscr, `"Normal" is the usual setup: pick another name.`)
		case !strings.EqualFold(name, current) && cfg.Playlists[strings.ToLower(name)] != nil:
			message(stdscr, "There's already a playlist called "+name+".")
		default:
			return name, true
		}
		current = name
	}
}

// genreCounts counts the games in each genre, for one system ("" = all).
func genreCounts(files []MenuFile, systemID string) map[string]int {
	counts := map[string]int{}
	for i := range files {
		if systemID != "" && !strings.EqualFold(files[i].SystemId, systemID) {
			continue
		}
		for _, g := range files[i].Genres {
			counts[g]++
		}
	}
	return counts
}

// tickGenres is a tick list of genres (sub-genres indented under theirs,
// with game counts); it returns the ticked ones and whether they changed.
// tickGenres picks genres for a playlist: all systems (systemID "") or
// one. At the top, "Create custom genre..." makes a genre from a word or
// phrase instead (any game with it in its name or folder); custom genres
// from all playlists are listed first, so each is only made once.
func tickGenres(stdscr *gc.Window, cfg *config.Config, title string, files []MenuFile, systemID string, current []string) ([]string, bool) {
	var scope []MenuFile // the games this screen is about
	for i := range files {
		if systemID == "" || strings.EqualFold(files[i].SystemId, systemID) {
			scope = append(scope, files[i])
		}
	}
	counts := genreCounts(scope, "")
	customCount := func(g string) int {
		n := 0
		for i := range scope {
			if scope[i].InPick(g) {
				n++
			}
		}
		return n
	}
	// A custom genre named like a real one is labelled as the user's own.
	customLabel := func(g string) string {
		label := config.GenreLabel(g)
		for real := range counts {
			if strings.EqualFold(real, label) || strings.EqualFold(real[strings.LastIndex(real, "/")+1:], label) {
				return label + " - Made by you!"
			}
		}
		return label
	}

	// Custom genres: the ones picked here, and the ones in other playlists.
	var customs []string
	seen := map[string]bool{}
	addCustom := func(g string) {
		if config.IsCustomGenre(g) && !seen[strings.ToLower(g)] {
			seen[strings.ToLower(g)] = true
			customs = append(customs, g)
		}
	}
	for _, g := range current {
		addCustom(g)
	}
	for _, pl := range cfg.Playlists {
		for _, g := range pl.All {
			addCustom(g)
		}
		for _, list := range pl.Systems {
			for _, g := range list {
				addCustom(g)
			}
		}
	}
	sort.Slice(customs, func(i, j int) bool { return config.GenreLabel(customs[i]) < config.GenreLabel(customs[j]) })

	keys := map[string]bool{}
	for k := range counts {
		keys[k] = true
	}
	for _, k := range current { // keep picks that no longer have games, so they can be unticked
		if !config.IsCustomGenre(k) {
			keys[k] = true
		}
	}
	var parents []string
	subs := map[string][]string{}
	for k := range keys {
		if p := gamesdb.GenreParent(k); p != "" {
			subs[p] = append(subs[p], k)
			if !keys[p] {
				keys[p] = true
			}
		}
	}
	for k := range keys {
		if gamesdb.GenreParent(k) == "" {
			parents = append(parents, k)
		}
	}
	sort.Strings(parents)
	var ids, labels []string
	for _, g := range customs {
		ids = append(ids, g)
		labels = append(labels, fmt.Sprintf("%s (%d)", customLabel(g), customCount(g)))
	}
	for _, p := range parents {
		ids = append(ids, p)
		labels = append(labels, fmt.Sprintf("%s (%d)", p, counts[p]))
		sort.Strings(subs[p])
		for _, s := range subs[p] {
			ids = append(ids, s)
			labels = append(labels, fmt.Sprintf("    %s (%d)", s[len(p)+1:], counts[s]))
		}
	}
	on := map[string]bool{}
	for _, k := range current {
		for _, id := range ids {
			if strings.EqualFold(id, k) {
				on[id] = true
			}
		}
	}
	var picked []string
	changed := false
	create := &tickAction{label: "Create custom genre...", run: func() (string, string, bool) {
		count := newPatternCounter(scope, func(text string) []string { return []string{config.CustomGenre(text)} })
		gc.Cursor(1)
		button, text, err := curses.OnScreenKeyboardWith(stdscr, "Custom genre: games containing",
			[]string{"OK", "Cancel"}, "", curses.KeyboardOpts{PadKeys: true, OnTextChange: count.changed, Status: count.status})
		gc.Cursor(0)
		if err != nil || button != 0 || strings.TrimSpace(text) == "" {
			return "", "", false
		}
		g := config.CustomGenre(text)
		return g, fmt.Sprintf("%s (%d)", customLabel(g), customCount(g)), true
	}}
	tickListWith(stdscr, title, ids, labels, nil, on, func(m map[string]bool) {
		changed = true
		listed := map[string]bool{}
		for _, id := range ids {
			listed[id] = true
		}
		// Custom genres created on this screen first (they're not in ids),
		// then the rest in list order.
		var created []string
		for id, ticked := range m {
			if ticked && !listed[id] {
				created = append(created, id)
			}
		}
		sort.Strings(created)
		picked = append(picked, created...)
		for _, id := range ids {
			if m[id] {
				picked = append(picked, id)
			}
		}
	}, create)
	if !changed {
		return current, false
	}
	return picked, true
}

func editPlaylist(stdscr *gc.Window, cfg *config.Config, files []MenuFile, p *config.Playlist) {
	nameOf := games.DisplayName
	save := func() {
		if err := config.SavePlaylist(cfg, p); err != nil {
			message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
		}
	}
	(&menuScreen{titleOf: func() string { return "Playlist: " + p.Name }, selected: 1, lines: func() []menuLine {
		all := "none"
		if len(p.All) > 0 {
			all = config.GenreLabels(p.All)
		}
		return []menuLine{
			info("Plays: " + config.DescribePlaylist(p, nameOf)),
			setting(settingText("All systems:", all), func() {
				if picked, changed := tickGenres(stdscr, cfg, "Genres for all systems", files, "", p.All); changed {
					p.All = picked
					save()
				}
			}),
			setting(settingText("Per system:", fmt.Sprintf("%d picked", len(p.Systems))), func() {
				perSystemGenres(stdscr, cfg, files, p, nameOf, save)
			}),
			setting(othersLine(p), func() {
				p.Others = nextOf([]string{config.OthersAsNormal, config.OthersLeaveOut}, p.Others)
				save()
			}),
			setting(settingText("Skip:", skipText(p)), func() {
				words, kept := skipWords(p.Skip)
				title := "Skip games containing (words, space between)"
				if len(kept) > 0 {
					title += fmt.Sprintf(" +%d in ini", len(kept))
				}
				// The live count shows how many games the words would skip.
				count := newPatternCounter(files, skipPatterns)
				button, text, err := curses.OnScreenKeyboardWith(stdscr, title,
					[]string{"OK", "Cancel"}, words,
					curses.KeyboardOpts{PadKeys: true, OnTextChange: count.changed, Status: count.status})
				if err != nil || button != 0 {
					return
				}
				p.Skip = append(skipPatterns(text), kept...)
				save()
			}),
			setting(settingText("Leave out systems:", leftOutText(p, nameOf)), func() {
				leaveOutSystems(stdscr, files, p, save)
			}),
			opens("Rename...", func() {
				name, ok := askPlaylistName(stdscr, cfg, p.Name)
				if !ok || name == p.Name {
					return
				}
				old := p.Name
				wasActive := strings.EqualFold(cfg.Attract.Playlist, old)
				if err := config.RemoveSection(cfg.Path, "Playlist."+old); err != nil {
					message(stdscr, fmt.Sprintf("Couldn't rename: %v", err))
					return
				}
				delete(cfg.Playlists, strings.ToLower(old))
				p.Name = name
				save()
				if wasActive {
					_ = config.SetActivePlaylist(cfg, name)
				}
			}),
			action("Delete", "Delete this playlist", nil).leaves(func() bool {
				if !confirm(stdscr, "Delete "+p.Name+"?", "Delete it", "Keep it") {
					return false
				}
				if err := config.DeletePlaylist(cfg, p.Name); err != nil {
					message(stdscr, fmt.Sprintf("Couldn't delete: %v", err))
				}
				return true
			}),
		}
	}}).run(stdscr)
}

// perSystemGenres lists the systems that have genres (with their picks),
// and edits a chosen system's genres.
func perSystemGenres(stdscr *gc.Window, cfg *config.Config, files []MenuFile, p *config.Playlist, nameOf func(string) string, save func()) {
	withGenres := map[string]bool{}
	for i := range files {
		if len(files[i].Genres) > 0 {
			withGenres[strings.ToLower(files[i].SystemId)] = true
		}
	}
	for id := range p.Systems {
		withGenres[id] = true
	}
	if len(withGenres) == 0 {
		message(stdscr, "No games have genres yet: rebuild the games\ndatabase (Options -> Game Database).")
		return
	}
	ids := make([]string, 0, len(withGenres))
	for id := range withGenres {
		ids = append(ids, id)
	}
	// Always under Arcade, Consoles, Handhelds, Computers and Other, A-Z in
	// each, whatever the menu's sorting.
	cats, byCat := categoryGroups(ids)
	(&menuScreen{title: "Per system: " + p.Name, lines: func() []menuLine {
		var lines []menuLine
		for _, cat := range cats {
			lines = append(lines, heading(categoryTitles[cat]))
			for _, id := range byCat[cat] {
				lines = append(lines, perSystemLine(stdscr, cfg, files, p, nameOf, save, id))
			}
		}
		return lines
	}}).run(stdscr)
}

// perSystemLine is one system on the Per system screen: its genre picks,
// edited by choosing it.
func perSystemLine(stdscr *gc.Window, cfg *config.Config, files []MenuFile, p *config.Playlist, nameOf func(string) string, save func(), id string) menuLine {
	picks := "-"
	if g := p.Systems[id]; len(g) > 0 {
		picks = strings.Join(g, ", ")
	}
	return opens(settingIndented(nameOf(id), picks), func() {
		if picked, changed := tickGenres(stdscr, cfg, "Genres for "+nameOf(id), files, id, p.Systems[id]); changed {
			if len(picked) == 0 {
				delete(p.Systems, id)
			} else {
				p.Systems[id] = picked
			}
			save()
		}
	})
}

// othersLine shows what happens to systems with no genres picked. With
// All systems genres picked there are none: those apply to every system.
func othersLine(p *config.Playlist) string {
	if len(p.All) > 0 {
		return settingText("Systems with no picks:", "Set by All Systems")
	}
	return settingText("Systems with no picks:", p.Others)
}

// skipText shows the playlist's Skip patterns (per-system skips are only
// in the ini, so they're just mentioned).
func skipText(p *config.Playlist) string {
	t := "none"
	if len(p.Skip) > 0 {
		t = strings.Join(p.Skip, ", ")
	}
	if len(p.SystemSkip) > 0 {
		t += " (+ per-system in SAMenu.ini)"
	}
	return t
}

func init() {
	ids := map[string]string{}
	for _, s := range games.Systems {
		ids[strings.ToLower(s.Id)] = s.Id
	}
	config.SystemIDCase = func(id string) string { return ids[strings.ToLower(id)] }
}

// The Skip screen takes plain words, since the on-screen keyboard has no
// "*" or ",": "mahjong pachinko" skips anything with either word in its
// name or folder. They're saved as the ini's usual patterns ("*mahjong*,
// *pachinko*"), so the ini itself works exactly as always.

// skipWords turns the Skip patterns into the text to edit: "*word*" shows
// as the word, other one-word patterns (like "mah*") as they are. Patterns
// with a space ("*street fighter*", only possible by editing the ini) would
// be split apart by the screen, so they're kept aside and saved back as
// they were.
func skipWords(patterns []string) (words string, kept []string) {
	var shown []string
	for _, pat := range patterns {
		inner := strings.TrimSpace(pat)
		switch {
		case strings.ContainsAny(inner, " \t"):
			kept = append(kept, pat)
		case len(inner) > 2 && strings.HasPrefix(inner, "*") && strings.HasSuffix(inner, "*") && !strings.Contains(inner[1:len(inner)-1], "*"):
			shown = append(shown, inner[1:len(inner)-1])
		default:
			shown = append(shown, inner)
		}
	}
	return strings.Join(shown, " "), kept
}

// skipPatterns turns the typed words into Skip patterns: each plain word
// becomes "*word*" (anything containing it); a word that already has a "*"
// is kept as typed. Repeated words are only kept once.
func skipPatterns(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range strings.Fields(text) {
		if !strings.Contains(w, "*") {
			w = "*" + w + "*"
		}
		if !seen[strings.ToLower(w)] {
			seen[strings.ToLower(w)] = true
			out = append(out, w)
		}
	}
	return out
}

// leftOutText shows a playlist's left-out systems.
func leftOutText(p *config.Playlist, nameOf func(string) string) string {
	if len(p.Exclude) == 0 {
		return "none"
	}
	names := make([]string, len(p.Exclude))
	for i, id := range p.Exclude {
		names[i] = nameOf(id)
	}
	return strings.Join(names, ", ")
}

// leaveOutSystems ticks the systems a playlist never plays, whatever
// genres it picks (saved as Exclude; groups like Computer also work there,
// written by hand).
func leaveOutSystems(stdscr *gc.Window, files []MenuFile, p *config.Playlist, save func()) {
	ids := systemIDsIn(files)
	on := map[string]bool{}
	for _, id := range ids {
		for _, x := range p.Exclude {
			if strings.EqualFold(x, id) {
				on[id] = true
			}
		}
	}
	// Groups and unknown names written in the ini aren't in the list:
	// they're kept as they are.
	var kept []string
	for _, x := range p.Exclude {
		found := false
		for _, id := range ids {
			found = found || strings.EqualFold(x, id)
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
		p.Exclude = out
		save()
	})
}

// systemIDsIn lists the systems that have games in files, in the systems
// list's order.
func systemIDsIn(files []MenuFile) []string {
	seen := map[string]bool{}
	var names []string
	byName := map[string]string{}
	for i := range files {
		id := files[i].SystemId
		if !seen[id] {
			seen[id] = true
			n := games.DisplayName(id)
			names = append(names, n)
			byName[n] = id
		}
	}
	sortSystems(names)
	ids := make([]string, len(names))
	for i, n := range names {
		ids[i] = byName[n]
	}
	return ids
}
