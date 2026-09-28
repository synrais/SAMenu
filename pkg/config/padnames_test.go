package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// An older SAMenu.ini (SDL's button names, no PadNames) is brought over to
// MiSTer's names once: each binding stays on the same button.
func TestMisterPadNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SAMenu.ini")
	old := `[InputDetector]
Joystick = true

[InputDetector.Joystick]
a = next
x = stay
b =
start = play

[Controls.Menu]
favourite = tab, pad:a
remove = pad:y
`
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(UserConfigEnv, path)

	for round := 1; round <= 2; round++ { // the second load must change nothing
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		wantPad := map[string]string{"b": "next", "y": "stay", "start": "play"}
		if got := cfg.AttractControls["joystick"]; !reflect.DeepEqual(got, wantPad) {
			t.Errorf("round %d: joystick %v, want %v", round, got, wantPad)
		}
		if got := cfg.MenuControls["favourite"]; !reflect.DeepEqual(got, []string{"tab", "pad:b"}) {
			t.Errorf("round %d: favourite %v", round, got)
		}
		if got := cfg.MenuControls["remove"]; !reflect.DeepEqual(got, []string{"pad:x"}) {
			t.Errorf("round %d: remove %v", round, got)
		}
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "PadNames = MiSTer") {
		t.Errorf("no PadNames marker written:\n%s", b)
	}
}

// A new SAMenu.ini (from the default) already has MiSTer's names: nothing
// is swapped.
func TestMisterPadNamesNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SAMenu.ini")
	t.Setenv(UserConfigEnv, path)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Created || cfg.InputDetector.PadNames != "MiSTer" {
		t.Fatalf("created %v, PadNames %q", cfg.Created, cfg.InputDetector.PadNames)
	}
	before, _ := os.ReadFile(path)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("loading a new file changed it")
	}
}
