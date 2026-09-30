package mister

import (
	"strings"
	"testing"
)

// A MiSTer.ini with the awkward parts of real ones: Windows line endings,
// inline comments, commented-out settings, a [Menu] section and a header
// over several lines.
var testIni = strings.ReplaceAll(`[MiSTer]
forced_scandoubler=0   ; set to 1 to run scandoubler on VGA output always (depends on core).
vga_mode=rgb           ; supported modes: rgb, ypbpr, svideo, cvbs. rgb is default.
;video_mode_ntsc=0
osd_rotate=0           ; Display OSD menu rotated,  0 - no rotation, 1 - rotate right (+90°)
video_mode=13

vsync_adjust=0
[Menu]
vga_scaler=1

[Amiga
+Amiga500
+Amiga600HD]
video_mode_ntsc=8 ; These two use the recommended setting
vga_mode=cvbs
`, "\n", "\r\n")

func TestIniGet(t *testing.T) {
	for _, c := range []struct {
		section, key, want string
		found              bool
	}{
		{"MiSTer", "vga_mode", "rgb", true},
		{"MiSTer", "forced_scandoubler", "0", true},
		{"MiSTer", "video_mode", "13", true},
		{"MiSTer", "video_mode_ntsc", "", false}, // commented out
		{"Menu", "vga_scaler", "1", true},
		{"Menu", "vga_mode", "", false},
		{"MiSTer", "vga_scaler", "", false},
	} {
		got, found := IniGet(testIni, c.section, c.key)
		if got != c.want || found != c.found {
			t.Errorf("%s %s: got %q %v, want %q %v", c.section, c.key, got, found, c.want, c.found)
		}
	}
}

func TestIniSet(t *testing.T) {
	// A value replaced in place: spacing and comment kept.
	out := IniSet(testIni, "MiSTer", "vga_mode", "ypbpr", false)
	if !strings.Contains(out, "\nvga_mode=ypbpr         ; supported modes") { // comment kept in its column
		t.Errorf("replace lost the layout:\n%s", out)
	}
	if v, _ := IniGet(out, "Amiga", "vga_mode"); v != "" {
		t.Errorf("the multi-line section's setting was read as Amiga: %q", v)
	}
	if !strings.Contains(out, "\r\nvga_mode=cvbs\r\n") {
		t.Error("the multi-line section's vga_mode changed")
	}

	// Only the one line changed, and every line still ends in \r\n.
	if a, b := strings.Split(testIni, "\n"), strings.Split(out, "\n"); len(a) != len(b) {
		t.Fatalf("line count %d -> %d", len(a), len(b))
	} else {
		changed := 0
		for i := range a {
			if a[i] != b[i] {
				changed++
			}
			if i < len(b)-1 && !strings.HasSuffix(b[i], "\r") {
				t.Errorf("line %d lost its \\r: %q", i, b[i])
			}
		}
		if changed != 1 {
			t.Errorf("%d lines changed, want 1", changed)
		}
	}

	// A new setting goes after the section's last setting.
	out = IniSet(testIni, "MiSTer", "composite_sync", "1", false)
	if !strings.Contains(out, "vsync_adjust=0\r\ncomposite_sync=1\r\n[Menu]") {
		t.Errorf("new setting in the wrong place:\n%s", out)
	}
	out = IniSet(testIni, "Menu", "video_mode", "8", false)
	if v, _ := IniGet(out, "Menu", "video_mode"); v != "8" {
		t.Errorf("[Menu] video_mode = %q", v)
	}
	if v, _ := IniGet(out, "MiSTer", "video_mode"); v != "13" {
		t.Errorf("[MiSTer] video_mode changed to %q", v)
	}

	// Back to MiSTer's default: commented out, not deleted.
	out = IniSet(testIni, "MiSTer", "video_mode", "", true)
	if _, found := IniGet(out, "MiSTer", "video_mode"); found {
		t.Error("video_mode still set")
	}
	if !strings.Contains(out, "\r\n;video_mode=13\r\n") {
		t.Errorf("video_mode not commented out:\n%s", out)
	}

	// A section that isn't there yet goes at the end.
	noMenu := strings.Replace(testIni, "[Menu]\r\nvga_scaler=1\r\n", "", 1)
	out = IniSet(noMenu, "Menu", "vga_scaler", "1", false)
	if !strings.HasSuffix(out, "\r\n\r\n[Menu]\r\nvga_scaler=1\r\n") {
		t.Errorf("new section:\n%q", out[len(out)-60:])
	}
	if v, _ := IniGet(out, "Menu", "vga_scaler"); v != "1" {
		t.Errorf("new [Menu] vga_scaler = %q", v)
	}

	// Unix line endings stay Unix.
	lf := strings.ReplaceAll(testIni, "\r\n", "\n")
	if out := IniSet(lf, "MiSTer", "composite_sync", "1", false); strings.Contains(out, "\r") {
		t.Error("\\r added to a file without them")
	}
}
