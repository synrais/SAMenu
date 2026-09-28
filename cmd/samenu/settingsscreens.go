package main

import (
	"fmt"
	"strconv"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
)

// -------------------------
// SAMenu.ini Settings Screens
// -------------------------
//
// Options -> Attract Mode (settings, Detector & list settings) and
// Options -> Game Database (Database
// systems). They edit [Attract], [StaticDetector], [List] and [Database]
// in SAMenu.ini; the rest of those sections stays as it is.

func onOffOption(name string, on bool) labelOption {
	o := labelOption{name, []string{"Off", "On"}, 0}
	o.setBool(on)
	return o
}

// -------- Attract mode settings --------

var playTimePresets = []string{"30", "45-60", "60-90", "90-120", "120-180"}

// otherButtons are the "Other buttons" choices, and the [Attract]
// OtherInput value each one saves.
var otherButtons = []struct{ label, ini string }{
	{"Do nothing", "Ignore"},
	{"Play the game", "Play"},
	{"Back to menu", "Stop"},
}

func attractSettingsScreen(stdscr *gc.Window, cfg *config.Config, sysNames []string) {
	a := &cfg.Attract

	playTime := labelOption{"Play time (seconds)", append([]string(nil), playTimePresets...), 1}
	if pt := strings.ReplaceAll(strings.TrimSpace(a.PlayTime), " ", ""); pt != "" {
		playTime.set(pt)
		if playTime.value() != pt { // a custom value from SAMenu.ini: keep it
			playTime.values = append([]string{pt}, playTimePresets...)
			playTime.index = 0
		}
	}

	ids := systemIDs(sysNames)
	systems := labelOption{"Systems", []string{""}, 0}
	systemsText := func() {
		on := attractTicks(cfg, ids)
		systems.values[0] = fmt.Sprintf("%d of %d  (select to choose)", countTicks(on, ids), len(ids))
	}
	systemsText()

	// Other buttons: presses with no action in Options -> Controls.

	mute := onOffOption("Mute during attract", a.Mute)

	// How games are picked (pkg/attract/picker.go).
	selection := labelOption{"Game selection", append([]string(nil), attract.SelectionModes...), 0}
	selection.set(a.Selection)
	noRepeats := onOffOption("No repeats", a.NoRepeats)
	mixSystems := onOffOption("Mix systems", a.MixSystems)
	oneVersion := onOffOption("One version per title", a.OneVersion)

	// Skip games tagged: Beta, Proto, Disc 2+ ... (a tick list).
	skipTags := labelOption{"Skip games tagged", []string{""}, 0}
	skipText := func() { skipTags.values[0] = tagsText(a.SkipTags) }
	skipText()

	// Arcade orientation (each MRA's <rotation>); unknown ones only play
	// with Both.
	orientation := labelOption{"Arcade orientation", []string{"Both", "Horizontal", "Vertical", "Vertical CW", "Vertical CCW"}, 0}
	orientation.set(a.Orientation)

	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "Attract Mode Settings",
		noPreview: true,
		options: func() []*labelOption {
			return []*labelOption{&playTime, &systems, &selection, &noRepeats, &mixSystems, &oneVersion, &skipTags, &orientation, &mute}
		},
		changed: func(o *labelOption) {
			if o == &systems {
				tickSystems(stdscr, "Attract Mode Systems", ids, attractTicks(cfg, ids), func(on map[string]bool) {
					a.Include = nil
					a.Exclude = unticked(ids, on)
				})
				systemsText()
			}
			if o == &skipTags {
				tickTags(stdscr, "Skip Games Tagged", &a.SkipTags)
				skipText()
			}
		},
		save: func() error {
			a.PlayTime = playTime.value()
			a.Mute = mute.isOn()
			a.Selection = selection.value()
			a.NoRepeats = noRepeats.isOn()
			a.MixSystems = mixSystems.isOn()
			a.OneVersion = oneVersion.isOn()
			a.Orientation = orientation.value()
			return config.SaveValues(cfg.Path, "Attract", [][2]string{
				{"Orientation", a.Orientation},
				{"Selection", a.Selection},
				{"NoRepeats", strconv.FormatBool(a.NoRepeats)},
				{"MixSystems", strconv.FormatBool(a.MixSystems)},
				{"OneVersion", strconv.FormatBool(a.OneVersion)},
				{"SkipTags", strings.Join(a.SkipTags, ", ")},
				{"PlayTime", a.PlayTime},
				{"Include", strings.Join(a.Include, ", ")},
				{"Exclude", strings.Join(a.Exclude, ", ")},
				{"Mute", strconv.FormatBool(a.Mute)},
			})
		},
	})
}

// attractTicks is which systems attract mode plays now, from Include and
// Exclude (which may use group names).
func attractTicks(cfg *config.Config, ids []string) map[string]bool {
	inc, _ := games.ResolveSystems(cfg.Attract.Include)
	exc, _ := games.ResolveSystems(cfg.Attract.Exclude)
	on := map[string]bool{}
	for _, id := range ids {
		l := strings.ToLower(id)
		on[id] = (len(inc) == 0 || inc[l]) && !exc[l]
	}
	return on
}

// -------- Detector & list settings --------

func detectorSettingsScreen(stdscr *gc.Window, cfg *config.Config) {
	d, l := &cfg.StaticDetector, &cfg.List
	opts := []struct {
		o   labelOption
		dst *bool
	}{
		{onOffOption("Static detector", cfg.Attract.UseStaticDetector), &cfg.Attract.UseStaticDetector},
		{onOffOption("Skip black screens", d.SkipBlack), &d.SkipBlack},
		{onOffOption("Blacklist black screens", d.WriteBlackList), &d.WriteBlackList},
		{onOffOption("Skip static screens", d.SkipStatic), &d.SkipStatic},
		{onOffOption("Staticlist static screens", d.WriteStaticList), &d.WriteStaticList},
		{onOffOption("Use blacklist", l.UseBlacklist), &l.UseBlacklist},
		{onOffOption("Use staticlist", l.UseStaticlist), &l.UseStaticlist},
		{onOffOption("Use whitelist", l.UseWhitelist), &l.UseWhitelist},
	}
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "Detector & List Settings",
		noPreview: true,
		options: func() []*labelOption {
			out := make([]*labelOption, len(opts))
			for i := range opts {
				out[i] = &opts[i].o
			}
			return out
		},
		save: func() error {
			for i := range opts {
				*opts[i].dst = opts[i].o.isOn()
			}
			if err := config.SaveValues(cfg.Path, "Attract", [][2]string{
				{"UseStaticDetector", strconv.FormatBool(cfg.Attract.UseStaticDetector)},
			}); err != nil {
				return err
			}
			if err := config.SaveValues(cfg.Path, "StaticDetector", [][2]string{
				{"SkipBlack", strconv.FormatBool(d.SkipBlack)}, {"WriteBlackList", strconv.FormatBool(d.WriteBlackList)},
				{"SkipStatic", strconv.FormatBool(d.SkipStatic)}, {"WriteStaticList", strconv.FormatBool(d.WriteStaticList)},
			}); err != nil {
				return err
			}
			return config.SaveValues(cfg.Path, "List", [][2]string{
				{"UseBlacklist", strconv.FormatBool(l.UseBlacklist)}, {"UseStaticlist", strconv.FormatBool(l.UseStaticlist)},
				{"UseWhitelist", strconv.FormatBool(l.UseWhitelist)},
			})
		},
	})
}

// -------- Database systems --------

// databaseSystemsScreen ticks which systems go in the games database. It
// reports whether the user asked to rebuild the database now.
func databaseSystemsScreen(stdscr *gc.Window, cfg *config.Config) bool {
	var names []string
	for _, s := range games.Systems {
		names = append(names, s.Name)
	}
	sortSystems(names)
	ids := systemIDs(names)

	exc, _ := games.ResolveSystems(cfg.Database.Exclude)
	on := map[string]bool{}
	for _, id := range ids {
		on[id] = !exc[strings.ToLower(id)]
	}

	changed := false
	tickSystems(stdscr, "Games Database Systems", ids, on, func(on map[string]bool) {
		cfg.Database.Exclude = unticked(ids, on)
		if err := config.SaveValues(cfg.Path, "Database", [][2]string{
			{"Exclude", strings.Join(cfg.Database.Exclude, ", ")},
		}); err != nil {
			message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			return
		}
		changed = true
	})
	if !changed {
		return false
	}

	// Starts on Rebuild now: the systems were just changed to rebuild them.
	c, ok := optionsList(stdscr, "Rebuild the games database now?", []string{"Rebuild now", "Later"}, 0)
	return ok && c == 0
}

// -------- System tick list --------

// tickSystems shows a tick list of systems. Changes are handed to save
// when leaving, only if anything changed.
// The systems are in groups, as the menu groups them (by manufacturer or
// category; by category when the menu doesn't group), each with a heading
// that turns the whole group on or off.
func tickSystems(stdscr *gc.Window, title string, ids []string, on map[string]bool, save func(map[string]bool)) {
	names := namesOf(ids)
	labels := systemLabels(names, optionsWidth-14)
	by := optGroup.value()
	if by == "None" {
		by = "Category"
	}
	groups := make([]string, len(ids))
	for i, n := range names {
		g := games.SystemGroup(n, by)
		if t, ok := categoryTitles[g]; ok && by == "Category" {
			g = t
		}
		groups[i] = g
	}
	tickListWith(stdscr, title, ids, labels, groups, on, save, nil)
}

// tickList is a list of tick boxes: ids are what's ticked (keys of on),
// labels what's shown for each. save gets the ticks if anything changed.
func tickList(stdscr *gc.Window, title string, ids, labels []string, on map[string]bool, save func(map[string]bool)) {
	tickListWith(stdscr, title, ids, labels, nil, on, save, nil)
}

// tickAction is an entry above a tick list's boxes (e.g. "Create custom
// genre..."): choosing it runs it, and it can add a new, ticked entry.
type tickAction struct {
	label string
	run   func() (id, label string, ok bool)
}

// tickRow is one line of a tick list: an entry (id >= 0), or a group's
// heading (group >= 0), or the action at the top (neither).
type tickRow struct{ id, group int }

// tickListWith is tickList with the entries in groups, and an optional
// action at the top (nil for none), which is also where the highlight
// starts. groups is each entry's group, shown as a heading above its
// entries (nil for no groups). A heading has its own box: [x] all its
// entries on, [ ] none, [-] some; choosing it turns them all on, or off
// if they already are.
func tickListWith(stdscr *gc.Window, title string, ids, labels, groups []string, on map[string]bool, save func(map[string]bool), top *tickAction) {
	changed := false
	selected := 0
	box := func(n, of int) string {
		switch {
		case n == 0:
			return "[ ]"
		case n == of:
			return "[x]"
		}
		return "[-]"
	}
	for {
		// The rows: the action, then each group's heading and entries, in
		// the order the groups first appear.
		var rows []tickRow
		if top != nil {
			rows = append(rows, tickRow{-1, -1})
		}
		var names []string  // the groups' names
		var members [][]int // each group's entries
		if groups == nil {
			for i := range ids {
				rows = append(rows, tickRow{i, -1})
			}
		} else {
			index := map[string]int{}
			for i := range ids {
				g, ok := index[groups[i]]
				if !ok {
					g = len(names)
					index[groups[i]] = g
					names = append(names, groups[i])
					members = append(members, nil)
				}
				members[g] = append(members[g], i)
			}
			for g := range names {
				rows = append(rows, tickRow{-1, g})
				for _, i := range members[g] {
					rows = append(rows, tickRow{i, -1})
				}
			}
		}
		ticked := func(g int) int {
			n := 0
			for _, i := range members[g] {
				if on[ids[i]] {
					n++
				}
			}
			return n
		}
		items := make([]string, len(rows))
		for r, row := range rows {
			switch {
			case row.id >= 0:
				mark := "[ ]"
				if on[ids[row.id]] {
					mark = "[x]"
				}
				indent := ""
				if groups != nil {
					indent = "    "
				}
				items[r] = indent + mark + " " + strings.TrimLeft(labels[row.id], " ")
			case row.group >= 0:
				items[r] = box(ticked(row.group), len(members[row.group])) + " " + names[row.group]
			default:
				items[r] = top.label
			}
		}
		clearScreen(stdscr)
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Shortcuts:     menuShortcuts(),
			Title:         fmt.Sprintf("%s (%d of %d)", title, countTicks(on, ids), len(ids)),
			ScrollKey:     "tick:" + title, // the title's count changes with every tick
			Buttons:       []string{"Toggle", "All", "None", "Back"},
			ActionButton:  0,
			DefaultButton: 0,
			ShowTotal:     true,
			Width:         optionsWidth,
			Height:        listHeight,
			InitialIndex:  selected,
			DynamicActionLabel: func(r int) string {
				if r < 0 || r >= len(rows) {
					return "Toggle"
				}
				switch row := rows[r]; {
				case row.id >= 0:
					return "Toggle"
				case row.group >= 0:
					if ticked(row.group) == len(members[row.group]) {
						return "All off"
					}
					return "All on"
				}
				return "Choose"
			},
		}, items)
		if err != nil {
			break
		}
		if sel >= 0 {
			selected = sel
		}
		switch button {
		case 0:
			if sel < 0 || sel >= len(rows) {
				continue
			}
			switch row := rows[sel]; {
			case row.id >= 0:
				on[ids[row.id]] = !on[ids[row.id]]
				changed = true
			case row.group >= 0:
				// Some or none on: all on. All on: all off.
				all := ticked(row.group) < len(members[row.group])
				for _, i := range members[row.group] {
					on[ids[i]] = all
				}
				changed = true
			default:
				if id, label, ok := top.run(); ok {
					known := false
					for _, existing := range ids {
						known = known || existing == id
					}
					if !known { // new entries go at the top, under the action
						ids = append([]string{id}, ids...)
						labels = append([]string{label}, labels...)
						if groups != nil {
							groups = append([]string{""}, groups...)
						}
					}
					on[id] = true
					changed = true
					for i, existing := range ids {
						if existing == id {
							selected = i + 1
						}
					}
				}
			}
			continue
		case 1, 2:
			for _, id := range ids {
				on[id] = button == 1
			}
			changed = true
			continue
		}
		break
	}
	clearScreen(stdscr)
	if changed {
		save(on)
	}
}

func countTicks(on map[string]bool, ids []string) int {
	n := 0
	for _, id := range ids {
		if on[id] {
			n++
		}
	}
	return n
}

// unticked lists the IDs that are off, for an Exclude line.
func unticked(ids []string, on map[string]bool) []string {
	var out []string
	for _, id := range ids {
		if !on[id] {
			out = append(out, id)
		}
	}
	return out
}

// systemIDs turns menu display names into system IDs, keeping the order.
func systemIDs(names []string) []string {
	byName := map[string]string{}
	for _, s := range games.Systems {
		byName[s.Name] = s.Id
	}
	var ids []string
	for _, n := range names {
		if id, ok := byName[n]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func namesOf(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = games.DisplayName(id)
	}
	return out
}

// idleChoices are the idle times offered in the menu.
var idleChoices = []string{"Off", "1 min", "2 min", "5 min", "10 min", "15 min", "30 min"}

// idleMinutes reads an idleChoice back (0 for Off).
func idleMinutes(o labelOption) int {
	n, _ := strconv.Atoi(strings.TrimSuffix(o.value(), " min"))
	return n
}

// -------- Name tags --------

// tagsText is a tag setting's value on an options screen.
func tagsText(tags []string) string {
	return fmt.Sprintf("%d of %d  (select to choose)", len(gamesdb.NewTagFilter(tags).Names()), len(gamesdb.Tags))
}

// tickTags edits a list of name tags (Beta, Proto...) with tick boxes.
func tickTags(stdscr *gc.Window, title string, tags *[]string) {
	names := gamesdb.TagNames()
	on := map[string]bool{}
	for _, n := range gamesdb.NewTagFilter(*tags).Names() {
		on[n] = true
	}
	tickList(stdscr, title, names, names, on, func(on map[string]bool) {
		var ticked []string
		for _, n := range names {
			if on[n] {
				ticked = append(ticked, n)
			}
		}
		*tags = ticked
	})
}
