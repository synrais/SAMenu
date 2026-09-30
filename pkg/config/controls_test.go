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
			Key: key, Target: BackToScript, Script: "/media/fat/Scripts/update_all.sh"}
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
