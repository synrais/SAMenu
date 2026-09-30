package attract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveFromList(t *testing.T) {
	p := filepath.Join(t.TempDir(), "SNES_blacklist.txt")
	write := func(s string) {
		if err := os.WriteFile(p, []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	read := func() string { b, _ := os.ReadFile(p); return string(b) }

	write("# my list\nTetris (USA)\n\n  Mario Paint (Japan)  \nTetris (USA)\n")
	if err := RemoveFromList(p, "Tetris (USA)"); err != nil {
		t.Fatal(err)
	}
	// Only the first one; comments, blank lines and the rest stay.
	if got, want := read(), "# my list\n\n  Mario Paint (Japan)  \nTetris (USA)\n"; got != want {
		t.Errorf("after one:\n%q\nwant\n%q", got, want)
	}
	if err := RemoveFromList(p, "Zelda"); err == nil {
		t.Error("removing a game that isn't there: no error")
	}
	// Lines are matched as ListFiles gives them: trimmed.
	if err := RemoveFromList(p, "Mario Paint (Japan)"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveFromList(p, "Tetris (USA)"); err != nil {
		t.Fatal(err)
	}
	// No games left (only the comment): the list is gone, so a whitelist
	// doesn't stay behind playing nothing.
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("empty list kept: %q", read())
	}

	write("<42> Sonic (USA)\n")
	if err := DeleteList(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("DeleteList left the file")
	}
}
