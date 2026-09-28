package main

import (
	"fmt"
	"strings"

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
//	heading  "_Console"          can't be highlighted: it names the lines below
//
// Lines are found by what they are, not their place in the list, so a line
// can be added anywhere without renumbering the others.

type menuLine struct {
	text    string
	label   string      // the button: what pressing does ("" = nothing)
	press   func() bool // true leaves the screen; nil for information
	heading bool        // a heading: the highlight skips it
}

// opens is a line that opens another screen (or makes a choice).
func opens(text string, press func()) menuLine {
	return menuLine{text: text, label: "Select", press: stay(press)}
}

// setting is a line showing a setting ("Name: value"), changed by pressing.
func setting(text string, press func()) menuLine {
	return menuLine{text: text, label: "Change", press: stay(press)}
}

// action is a line that does something straight away, named by label.
func action(label, text string, press func()) menuLine {
	return menuLine{text: text, label: label, press: stay(press)}
}

// info is a line that only shows something: no button. It can still be
// highlighted, so a long one scrolls.
func info(text string) menuLine {
	return menuLine{text: text}
}

// heading names the group of lines below it; it can't be highlighted.
func heading(text string) menuLine {
	return menuLine{text: text, heading: true}
}

// restoreDefaults is a screen's "Restore defaults" line.
func restoreDefaults(press func()) menuLine {
	return menuLine{text: "Restore defaults", label: "Restore", press: stay(press)}
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

// labelColumn is where every setting's value starts ("Name:" padded to
// it), on every options screen: the longest label fits.
const labelColumn = 28

// settingText is a setting line, "Name:  value", its value in the shared
// column.
func settingText(label, value string) string {
	return fmt.Sprintf("%-*s %s", labelColumn, label, value)
}

// settingIndented is settingText for a line indented under a heading, its
// value in the same column.
func settingIndented(label, value string) string {
	return "  " + fmt.Sprintf("%-*s %s", labelColumn-2, label, value)
}

// menuScreen is one options screen. Every options screen is one, so they
// all look and work the same: one width, values in one column.
type menuScreen struct {
	title    string
	titleOf  func() string // instead of title, when it can change (a rename)
	selected int           // the line highlighted: set it from a press to move on
	lines    func() []menuLine
	preview  func() []string // a live preview in a box under the list, if any
	leave    func()          // run on leaving the screen (e.g. saving), if any
}

// run shows the screen until Back, or until a line leaves it. The lines
// are made again after every press, so they show the current values.
// It always lands on the line's button (Select, Change...), also after
// backing out of a screen opened here: B backs out from anywhere.
func (m *menuScreen) run(stdscr *gc.Window) {
	if m.leave != nil {
		defer m.leave()
	}
	buttons := []string{"Select", "Back"}
	pressed := "" // the name of the line last pressed
	for {
		lines := m.lines()
		if len(lines) == 0 {
			return
		}
		// Stay on the line pressed, if a line above it came or went: the
		// nearest line of that name.
		if pressed != "" {
			best := -1
			for i, l := range lines {
				if lineName(l.text) == pressed && (best < 0 || abs(i-m.selected) < abs(best-m.selected)) {
					best = i
				}
			}
			if best >= 0 {
				m.selected = best
			}
		}
		items := make([]string, len(lines))
		headers := map[int]bool{}
		for i, l := range lines {
			items[i] = l.text
			if l.heading {
				headers[i] = true
			}
		}
		if m.selected >= len(lines) {
			m.selected = len(lines) - 1
		}
		title := m.title
		if m.titleOf != nil {
			title = m.titleOf()
		}
		clearScreen(stdscr)
		// With a preview, the list and the preview's box are placed together
		// as one block, centred, the preview cut short if the screen is.
		height := len(items) + 4
		top := 0 // centred
		var previewWin *gc.Window
		if m.preview != nil {
			rows, _ := stdscr.MaxYX()
			shown := m.preview()
			if room := rows - height - 2; len(shown) > room {
				if room < 0 {
					room = 0
				}
				shown = shown[:room]
			}
			block := height
			if len(shown) > 0 {
				block += len(shown) + 2
			}
			if top = (rows - block) / 2; top < 1 {
				top = 1 // 0 would mean "centre" to the list
			}
			if len(shown) > 0 {
				previewWin = drawPreview(stdscr, shown, top+height)
			}
		}
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Shortcuts:     menuShortcuts(),
			Title:         title,
			Buttons:       buttons,
			DefaultButton: 0,
			ActionButton:  0,
			Width:         optionsWidth,
			Height:        height,
			Top:           top,
			InitialIndex:  m.selected,
			Headers:       headers,
			DynamicActionLabel: func(i int) string {
				if i >= 0 && i < len(lines) {
					return lines[i].label
				}
				return ""
			},
		}, items)
		if previewWin != nil {
			previewWin.Delete()
		}
		clearScreen(stdscr)
		if err != nil || button != 0 || sel < 0 || sel >= len(lines) {
			return
		}
		m.selected = sel
		pressed = lineName(lines[sel].text)
		if press := lines[sel].press; press != nil && press() {
			return
		}
	}
}

// lineName is a line's text without its value ("Playback:  Random" is
// "Playback"), to find it again once its value changes.
func lineName(text string) string {
	if i := strings.Index(text, ":"); i >= 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

// -------------------------
// Shared helpers
// -------------------------

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
