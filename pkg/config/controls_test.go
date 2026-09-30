package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/synrais/SAMenu/pkg/assets"
)

// Back to Menu settings are read back as saved, including keys that mean
// something in INI files, and the first version's pad:<SDL name> buttons.
func TestBackToMenuRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SAMenu.ini")
	if err := os.WriteFile(path, assets.DefaultSAMIni, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(UserConfigEnv, path)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := DefaultBackToMenu(); !reflect.DeepEqual(cfg.BackToMenu, want) {
		t.Fatalf("default ini: got %+v, want %+v", cfg.BackToMenu, want)
	}

	for _, key := range []string{"`", ";", "#", "=", "\"", "f12", ""} {
		cfg.BackToMenu = BackToMenuConfig{Enabled: true, Buttons: []string{"b", "select"}, HoldTime: 2.5,
			Key: key, WorksIn: WorksInAnywhere, Target: BackToScript, Script: "/media/fat/Scripts/update_all.sh"}
		if err := SaveBackToMenu(cfg); err != nil {
			t.Fatal(err)
		}
		got, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.BackToMenu, cfg.BackToMenu) {
			t.Errorf("key %q: got %+v, want %+v", key, got.BackToMenu, cfg.BackToMenu)
		}
	}

	if err := SaveValues(path, "Controls.BackToMenu", [][2]string{{"Buttons", "pad:start, pad:back"}}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"start", "select"}; !reflect.DeepEqual(got.BackToMenu.Buttons, want) {
		t.Errorf("old buttons: got %v, want %v", got.BackToMenu.Buttons, want)
	}
}

// Where the hotkey works decides whether the background watcher runs for
// it, and where it watches.
func TestBackToMenuWorksIn(t *testing.T) {
	for _, c := range []struct {
		worksIn                 string
		background, games, menu bool
	}{
		{WorksInSAMenuGames, false, false, false},
		{WorksInGames, true, true, false},
		{WorksInMiSTerMenu, true, false, true},
		{WorksInAnywhere, true, true, true},
	} {
		cfg := &Config{BackToMenu: DefaultBackToMenu()}
		cfg.BackToMenu.Enabled = true
		cfg.BackToMenu.WorksIn = c.worksIn
		if cfg.HotkeyInBackground() != c.background || cfg.BackgroundWatch() != c.background ||
			cfg.BackToMenu.InGames() != c.games || cfg.BackToMenu.InMenu() != c.menu {
			t.Errorf("%s: background %v, games %v, menu %v; want %v, %v, %v", c.worksIn,
				cfg.HotkeyInBackground(), cfg.BackToMenu.InGames(), cfg.BackToMenu.InMenu(), c.background, c.games, c.menu)
		}
		cfg.BackToMenu.Enabled = false
		if cfg.HotkeyInBackground() || cfg.BackgroundWatch() {
			t.Errorf("%s, hotkey off: background watcher still wanted", c.worksIn)
		}
	}
}
