package main

import (
	"strings"

	gc "github.com/rthornton128/goncurses"
)

// -------------------------
// Menu helpers
// -------------------------
//
// Small pieces shared by the options screens, so each works the same way
// everywhere: the button label for a line, asking yes or no, and moving a
// setting on to its next value.

// actionLabel is the button label for a menu line, from how it reads:
//
//	"Restore defaults"   Restore
//	"Mapping..."         Select (opens another screen)
//	"Playback:  Random"  Change (a setting)
//	anything else        Select (a choice)
//
// Lines that do something else (Start, Play, Delete...) are given their own
// label by the screen, see lineLabels.
func actionLabel(item string) string {
	switch {
	case item == "Restore defaults":
		return "Restore"
	case strings.HasSuffix(item, "..."):
		return "Select"
	case strings.Contains(item, ":"):
		return "Change"
	}
	return "Select"
}

// lineLabels gives each line its button label: the screen's own label for
// that line if it has one ("" shows no button: pressing it does nothing),
// otherwise actionLabel.
func lineLabels(items []string, labels map[int]string) func(int) string {
	return func(i int) string {
		if l, ok := labels[i]; ok {
			return l
		}
		if i >= 0 && i < len(items) {
			return actionLabel(items[i])
		}
		return ""
	}
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
