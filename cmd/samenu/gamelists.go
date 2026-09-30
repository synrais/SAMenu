package main

import (
	"fmt"
	"strconv"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/utils"
)

// -------------------------
// Game lists (Options -> Attract Mode -> Game lists)
// -------------------------
//
// The Blacklist, Staticlist and Whitelist, per system, to look through
// and take games off: the static detector adds to the first two by
// itself, and a game that shouldn't be there can be removed here instead
// of editing the files. Removing a system's last game deletes its list.

var listKinds = []attract.ListKind{attract.Blacklist, attract.Staticlist, attract.Whitelist}

// gameListsScreen shows the three kinds of list.
func gameListsScreen(stdscr *gc.Window) {
	(&menuScreen{title: "Game lists", lines: func() []menuLine {
		var lines []menuLine
		for _, k := range listKinds {
			k := k
			lists := attract.ListFiles(k)
			value := "empty"
			if len(lists) > 0 {
				value = fmt.Sprintf("%s in %s", countText(gamesIn(lists), "game"), countText(len(lists), "system"))
			}
			lines = append(lines, opens(settingText(string(k)+":", value), func() { listSystemsScreen(stdscr, k) }))
		}
		lines = append(lines, info(""),
			info("Blacklist:  games never played"),
			info("Staticlist: games left once their screen goes static"),
			info("Whitelist:  the only games played, for its system"))
		return lines
	}}).run(stdscr)
}

// listSystemsScreen shows the systems with a list of this kind.
func listSystemsScreen(stdscr *gc.Window, kind attract.ListKind) {
	// Remove all (a button): every system's list of this kind at once. It
	// asks first; once they're gone, the screen closes.
	removeAll := func() bool {
		lists := attract.ListFiles(kind)
		question := fmt.Sprintf("Remove the %s of all %s (%s)?", kind, countText(len(lists), "system"), countText(gamesIn(lists), "game"))
		if len(lists) == 1 {
			question = fmt.Sprintf("Remove the %s %s (%s)?", games.DisplayName(lists[0].SystemID), kind, countText(gamesIn(lists), "game"))
		}
		if !confirm(stdscr, question, "Remove all", "Cancel") {
			return false
		}
		for _, l := range lists {
			if err := attract.DeleteList(l.Path); err != nil {
				message(stdscr, "Couldn't remove the "+games.DisplayName(l.SystemID)+" list: "+err.Error())
				return false
			}
		}
		return true
	}
	(&menuScreen{title: string(kind), buttons: []screenButton{{"Remove all", removeAll}}, lines: func() []menuLine {
		lists := attract.ListFiles(kind)
		if len(lists) == 0 {
			return nil // the last one went: back to the kinds
		}
		// Under Arcade, Consoles, Handhelds, Computers and Other, A-Z in
		// each, like every options screen's list of systems.
		byID := map[string]attract.ListFile{}
		var ids []string
		for _, l := range lists {
			byID[l.SystemID] = l
			ids = append(ids, l.SystemID)
		}
		cats, byCat := categoryGroups(ids)
		var lines []menuLine
		for _, cat := range cats {
			lines = append(lines, heading(categoryTitles[cat]))
			for _, id := range byCat[cat] {
				l := byID[id]
				lines = append(lines, opens(settingIndented(games.DisplayName(id)+":", countText(len(l.Games), "game")),
					func() { listGamesScreen(stdscr, kind, l) }))
			}
		}
		return lines
	}}).run(stdscr)
}

// listGamesScreen shows one system's list, to take games off it.
func listGamesScreen(stdscr *gc.Window, kind attract.ListKind, l attract.ListFile) {
	const remove, removeAll, back = 2, 3, 4
	selected := len(l.Games) - 1 // the newest: lists are added to at the end
	for {
		items := make([]string, len(l.Games))
		for i, g := range l.Games {
			items[i] = listGameText(kind, g)
		}
		title := fmt.Sprintf("%s: %s", games.DisplayName(l.SystemID), kind)
		clearScreen(stdscr)
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Shortcuts:     menuShortcuts(),
			Title:         title,
			Buttons:       []string{"PgUp", "PgDn", "Remove", "Remove all", "Back"},
			ActionButton:  remove,
			DefaultButton: remove,
			ShowTotal:     true,
			Width:         systemListWidth,
			Height:        listHeight,
			InitialIndex:  selected,
		}, items)
		clearScreen(stdscr)
		if err != nil || button == back || button < 0 {
			return
		}
		switch button {
		case remove:
			if sel < 0 || sel >= len(l.Games) {
				continue
			}
			if err := attract.RemoveFromList(l.Path, l.Games[sel]); err != nil {
				message(stdscr, "Couldn't remove it: "+err.Error())
			}
			selected = sel
		case removeAll:
			if !confirm(stdscr, fmt.Sprintf("Remove all %s from the %s %s?", countText(len(l.Games), "game"),
				games.DisplayName(l.SystemID), kind), "Remove all", "Cancel") {
				continue
			}
			if err := attract.DeleteList(l.Path); err != nil {
				message(stdscr, "Couldn't remove them: "+err.Error())
			}
		}
		// Read the list again: what's on the card is what's shown.
		l.Games = nil
		for _, f := range attract.ListFiles(kind) {
			if f.Path == l.Path {
				l.Games = f.Games
			}
		}
		if len(l.Games) == 0 {
			return // none left: the list is gone
		}
		if selected >= len(l.Games) {
			selected = len(l.Games) - 1
		}
	}
}

// listGameText is how a list line is shown: a Staticlist's "<42> Title"
// as "Title  (static at 42 s)".
func listGameText(kind attract.ListKind, line string) string {
	if kind != attract.Staticlist {
		return line
	}
	secs, title := utils.ParseLine(line)
	if secs <= 0 {
		return title
	}
	return fmt.Sprintf("%s  (static at %s s)", title, strconv.FormatFloat(secs, 'f', 0, 64))
}

func gamesIn(lists []attract.ListFile) int {
	n := 0
	for _, l := range lists {
		n += len(l.Games)
	}
	return n
}

// countText is "1 game", "12 games".
func countText(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%s %ss", withCommas(n), what)
}
