package curses

import (
	"strings"

	gc "github.com/rthornton128/goncurses"
)

// LetterKeys are the keys that jump a list by letter: -1 back to the start
// of the letter (or the previous letter, from its start), +1 on to the
// next letter. Every list reacts to them (the games menu's Previous and
// Next letter actions).
var LetterKeys = map[gc.Key]int{}

// letterOf is the letter a list line is filed under: its first letter or
// digit, past leading spaces, a favourite's "* " and a "[System] " before
// the name. Every digit is one group, '#'. 0 is a line with neither.
func letterOf(s string) byte {
	s = strings.TrimLeft(s, " ")
	s = strings.TrimPrefix(s, "* ")
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "] "); i > 0 {
			s = s[i+2:]
		}
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			return '#'
		case c >= 'a' && c <= 'z':
			return c - 'a' + 'A'
		case c >= 'A' && c <= 'Z':
			return c
		}
	}
	return 0
}

// letterJump is where a letter jump from line from goes, in direction dir.
// It never lands on a heading or a line above first (e.g. [Pick Random
// Game]), and stops at either end: from is returned when there's nowhere
// to go.
func letterJump(items []string, from, dir, first int, isHeader func(int) bool) int {
	if from < first {
		if dir > 0 {
			return nextLine(items, first-1, 1, first, isHeader)
		}
		return from
	}
	// start is the first line of the letter line i is in.
	start := func(i int) int {
		for {
			j := nextLine(items, i, -1, first, isHeader)
			if j < 0 || letterOf(items[j]) != letterOf(items[i]) {
				return i
			}
			i = j
		}
	}
	if dir > 0 {
		for i := nextLine(items, from, 1, first, isHeader); i >= 0; i = nextLine(items, i, 1, first, isHeader) {
			if letterOf(items[i]) != letterOf(items[from]) {
				return i
			}
		}
		return from
	}
	if s := start(from); s != from {
		return s
	}
	if p := nextLine(items, from, -1, first, isHeader); p >= 0 {
		return start(p)
	}
	return from
}

// nextLine is the line after i in direction dir that the highlight can
// land on, or -1.
func nextLine(items []string, i, dir, first int, isHeader func(int) bool) int {
	for i += dir; i >= first && i < len(items); i += dir {
		if !isHeader(i) {
			return i
		}
	}
	return -1
}
