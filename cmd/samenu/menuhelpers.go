package main

import (
	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/curses"
)

// -------------------------
// Menu screens
// -------------------------
//
// An options screen is a list of lines, each knowing what it is and what
// pressing it does. The button names what that is, so every screen reads
// the same way:
//
//	opens    "Mapping..."        Select   (another screen, or a choice)
//	setting  "Playback: Random"  Change
//	action   "Next track"        its own verb (Start, Play, Delete...)
//	info     "Plays: ..."        no button, pressing does nothing
//	restore  "Restore defaults"  Restore
//
// Lines are found by what they are, not their place in the list, so a line
// can be added anywhere without renumbering the others.

type menuLine struct {
	text  string
	label string      // the button: what pressing does ("" = nothing)
	press func() bool // true leaves the screen; nil for information
}

// opens is a line that opens another screen (or makes a choice).
func opens(text string, press func()) menuLine {
	return menuLine{text, "Select", stay(press)}
}

// setting is a line showing a setting ("Name: value"), changed by pressing.
func setting(text string, press func()) menuLine {
	return menuLine{text, "Change", stay(press)}
}

// action is a line that does something straight away, named by label.
func action(label, text string, press func()) menuLine {
	return menuLine{text, label, stay(press)}
}

// info is a line that only shows something: no button. It can still be
// highlighted, so a long one scrolls.
func info(text string) menuLine {
	return menuLine{text: text}
}

// restoreDefaults is a screen's "Restore defaults" line.
func restoreDefaults(press func()) menuLine {
	return menuLine{"Restore defaults", "Restore", stay(press)}
}

// leaves makes pressing a line leave the screen when press says so, e.g.
// once a playlist is deleted.
func (l menuLine) leaves(press func() bool) menuLine {
	l.press = press
	return l
}

func stay(press func()) func() bool {
	return func() bool {
		if press != nil {
			press()
		}
		return false
	}
}

// menuScreen is one options screen.
type menuScreen struct {
	title    string
	titleOf  func() string // instead of title, when it can change (a rename)
	wide     bool          // the wider options width, for long lines
	selected int           // the line highlighted: set it from a press to move on
	lines    func() []menuLine
}

// run shows the screen until Back, or until a line leaves it. The lines
// are made again after every press, so they show the current values.
// Backing out of a screen opened here lands on Back, to carry on backing
// out; moving to another line goes back to the line's button.
func (m *menuScreen) run(stdscr *gc.Window) {
	buttons := []string{"Select", "Back"}
	cameBack := false
	for {
		lines := m.lines()
		if len(lines) == 0 {
			return
		}
		items := make([]string, len(lines))
		for i, l := range lines {
			items[i] = l.text
		}
		if m.selected >= len(lines) {
			m.selected = len(lines) - 1
		}
		title := m.title
		if m.titleOf != nil {
			title = m.titleOf()
		}
		width := 60
		if m.wide {
			width = optionsWidth
		}
		clearScreen(stdscr)
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Shortcuts:     menuShortcuts(),
			Title:         title,
			Buttons:       buttons,
			DefaultButton: landingButton(buttons, 0, cameBack),
			ActionButton:  0,
			SnapToAction:  true,
			Width:         width,
			Height:        len(items) + 4,
			InitialIndex:  m.selected,
			DynamicActionLabel: func(i int) string {
				if i >= 0 && i < len(lines) {
					return lines[i].label
				}
				return ""
			},
		}, items)
		clearScreen(stdscr)
		if err != nil || button != 0 || sel < 0 || sel >= len(lines) {
			return
		}
		m.selected = sel
		if press := lines[sel].press; press != nil && press() {
			return
		}
		cameBack = curses.ClosedByBack()
	}
}

// -------------------------
// Shared helpers
// -------------------------

// landingButton is the button highlighted when a list shows again: its
// action button (Open, Pick...), or after backing out of something, its Back
// button, so Back can be pressed again to carry on climbing out. A list
// without one, like the main menu (Exit), always lands on its action.
// Backing out stays on Back, going forward stays on the action, throughout
// the menus.
func landingButton(buttons []string, action int, cameBack bool) int {
	if cameBack {
		for i, b := range buttons {
			if b == "Back" {
				return i
			}
		}
	}
	return action
}

// confirm asks a yes or no question, starting on no (the safe choice), and
// reports whether yes was chosen.
func confirm(stdscr *gc.Window, question, yes, no string) bool {
	c, ok := optionsList(stdscr, question, []string{yes, no}, 1)
	return ok && c == 0
}

// nextOf is the value after current in values, round to the first; a value
// that isn't in the list (e.g. typed into the ini) goes to the first.
func nextOf[T comparable](values []T, current T) T {
	for i, v := range values {
		if v == current {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

// onOffText shows a switch.
func onOffText(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}
