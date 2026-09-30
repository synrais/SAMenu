package main

import (
	"fmt"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
)

// -------------------------
// Menu Controls
// -------------------------
//
// The games menu's actions ([Controls.Menu], Options -> Controls -> Games
// Menu): Search and Options on the systems list, Favourite in game lists,
// Remove in [Favourites]. A controller reaches the menu as keys (MiSTer
// sends A = enter, B = esc, X = space, Y = tab, L/R = page up/down); other
// buttons through SAMenu's own detectors (menubridge.go).

// menuControls is action -> key names (the fixed defaults).
var menuControls = config.DefaultMenuControlsCopy()

// namedKeys are the keys with names; any other printable character is its
// own name ("/", "o", "`").
var namedKeys = map[string][]gc.Key{
	"esc":       {27},
	"backspace": {gc.KEY_BACKSPACE, 127, 8},
	"tab":       {9},
	"space":     {' '},
	"delete":    {gc.KEY_DC},
	"insert":    {gc.KEY_IC},
	"home":      {gc.KEY_HOME},
	"end":       {gc.KEY_END},
}

func init() {
	for i := 1; i <= 12; i++ {
		namedKeys[fmt.Sprintf("f%d", i)] = []gc.Key{gc.KEY_F1 + gc.Key(i-1)}
	}
}

// keysFor returns the ncurses keys for a key name.
func keysFor(name string) []gc.Key {
	name = strings.ToLower(strings.TrimSpace(name))
	if k, ok := namedKeys[name]; ok {
		return k
	}
	if len(name) == 1 && name[0] > ' ' && name[0] < 127 {
		return []gc.Key{gc.Key(name[0])}
	}
	return nil
}

// buttonFor is the list button each menu action presses.
var buttonFor = map[string]string{"search": "Search", "options": "Options", "favourite": "Fav", "remove": "Remove"}

// menuShortcutsFor maps the inputs of some menu actions to their buttons:
// their keys, and the key a mapped controller button hands the menu. A
// list names the actions that apply in it, so the same button can do
// different things in different places.
func menuShortcutsFor(acts ...string) map[gc.Key]string {
	out := map[gc.Key]string{}
	for _, act := range acts {
		for _, n := range menuControls[act] {
			for _, k := range keysFor(n) {
				out[k] = buttonFor[act]
			}
		}
		out[actionKey(act)] = buttonFor[act]
	}
	return out
}

// setMenuControls puts the menu's actions in use: the letter jumps for
// every list, and the input detectors for pad buttons.
func setMenuControls(m map[string][]string) {
	menuControls = m
	curses.LetterKeys = map[gc.Key]int{}
	for act, dir := range map[string]int{"prevletter": -1, "nextletter": 1} {
		for _, n := range m[act] {
			for _, k := range keysFor(n) {
				curses.LetterKeys[k] = dir
			}
		}
		curses.LetterKeys[actionKey(act)] = dir
	}
	syncInputs() // pad buttons need the input detectors
}

// menuShortcuts are the systems list's shortcuts: Search and Options.
func menuShortcuts() map[gc.Key]string { return menuShortcutsFor("search", "options") }
