package curses

import "testing"

func TestLetterOf(t *testing.T) {
	for in, want := range map[string]byte{
		"Alien":              'A',
		"  * bomberman":      'B',
		"[SNES] Contra":      'C',
		"* [NES] Duck Hunt":  'D',
		"1942":               '#',
		"'Ecco'":             'E',
		"[Pick Random Game]": 'P',
		"---":                0,
	} {
		if got := letterOf(in); got != want {
			t.Errorf("letterOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLetterJump(t *testing.T) {
	items := []string{"[Pick Random Game]", "1942", "2020", "Alien", "Axelay", "HEAD", "Bomber", "Contra"}
	headers := map[int]bool{5: true}
	isHeader := func(i int) bool { return headers[i] }
	for _, c := range []struct{ from, dir, want int }{
		{0, 1, 1},  // from Random: the first title
		{0, -1, 0}, // nowhere to go
		{1, 1, 3},  // digits are one group
		{3, 1, 6},  // over the heading
		{7, 1, 7},  // the last letter: stays
		{4, -1, 3}, // the start of its letter
		{3, -1, 1}, // already there: the previous letter's start
		{6, -1, 3}, // over the heading
		{1, -1, 1}, // never onto Random
	} {
		if got := letterJump(items, c.from, c.dir, 1, isHeader); got != c.want {
			t.Errorf("letterJump(from %d, dir %d) = %d, want %d", c.from, c.dir, got, c.want)
		}
	}
}
