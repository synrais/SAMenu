package main

import (
	"strings"
	"testing"

	"github.com/synrais/SAMenu/pkg/mister"
)

func TestPictureChoices(t *testing.T) {
	// Nothing set: MiSTer's defaults, which is an HDMI setup.
	text := "[MiSTer]\r\nvsync_adjust=0\r\n"
	for s, want := range map[*pictureSetting]string{
		&myScreen: "HDMI TV or monitor", &analogSignal: "RGB", &syncSetting: "Separate (VGA)",
		&menuOnAnalog: "Native", &gamesOnAnalog: "Native, no lag", &resolution: "Auto, from the screen",
		&topBottomBorder: "Off", &hdmiRange: "Full, for monitors", &hdmiToVGA: "No",
	} {
		if got := choiceText(*s, text); got != want {
			t.Errorf("%s %q, want %q", s.label, got, want)
		}
	}

	// A preset sets the lines under it, and is then what My screen shows.
	for i, p := range myScreen.choices {
		out := p.apply(text)
		if got := myScreen.current(out); got != i {
			t.Errorf("preset %q shows as %d, want %d", p.text, got, i)
		}
		if !strings.Contains(out, "\r\n") || strings.Count(out, "\r\n") != strings.Count(out, "\n") {
			t.Errorf("preset %q broke the line endings", p.text)
		}
	}
	// HDMI leaves the analog settings alone: an HDMI setup with the menu
	// scaled on analog is still HDMI, and HDMI from SCART keeps SCART's.
	hdmiMenuScaled := menuOnAnalog.choices[1].apply(text)
	if got := choiceText(myScreen, hdmiMenuScaled); got != "HDMI TV or monitor" {
		t.Errorf("HDMI with the menu scaled shows %q", got)
	}
	scart := myScreen.choices[1].apply(text)
	for s, want := range map[*pictureSetting]string{
		&analogSignal: "RGB", &syncSetting: "Combined (SCART)", &menuOnAnalog: "Scaled, fits any screen", &gamesOnAnalog: "Native, no lag",
	} {
		if got := choiceText(*s, scart); got != want {
			t.Errorf("SCART preset: %s %q, want %q", s.label, got, want)
		}
	}
	if v, _ := mister.IniGet(scart, "Menu", "vga_scaler"); v != "1" {
		t.Errorf("SCART preset: [Menu] vga_scaler = %q", v)
	}
	if v, _ := mister.IniGet(scart, "MiSTer", "vga_scaler"); v != "0" {
		t.Errorf("SCART preset: [MiSTer] vga_scaler = %q (games would be scaled)", v)
	}

	// Colour system only for S-Video and composite; Sync only for RGB.
	cvbs := analogSignal.choices[3].apply(text)
	if colourSystem.shown(text) || !colourSystem.shown(cvbs) || syncSetting.shown(cvbs) {
		t.Error("Colour system / Sync shown in the wrong places")
	}

	// Cycling: Auto -> 640x480 (video_mode=6); back to Auto comments it out.
	res := nextChoice(resolution, text)
	if v, _ := mister.IniGet(res, "MiSTer", "video_mode"); v != "6" {
		t.Errorf("next resolution: video_mode = %q", v)
	}
	back := resolution.choices[0].apply(res)
	if _, set := mister.IniGet(back, "MiSTer", "video_mode"); set {
		t.Error("Auto left video_mode set")
	}

	// A value set by hand is Custom, and moves on to the first choice.
	custom := mister.IniSet(text, "MiSTer", "video_mode", "1280,110,40,220,720,5,5,20,74250", false)
	if got := choiceText(resolution, custom); got != "Custom" {
		t.Errorf("hand-made video_mode shows %q", got)
	}
	if got := choiceText(resolution, nextChoice(resolution, custom)); got != "Auto, from the screen" {
		t.Errorf("Custom moved on to %q", got)
	}

	// [Menu] follows [MiSTer] until it's set itself.
	games := gamesOnAnalog.choices[2].apply(text) // [MiSTer] vga_scaler=1
	if got := choiceText(menuOnAnalog, games); got != "Scaled, fits any screen" {
		t.Errorf("menu with scaled games shows %q", got)
	}

	// The summary fits the preview box.
	for _, l := range pictureSummary("MiSTer.ini", true, scart, text) {
		if len(l) > 68 {
			t.Errorf("summary line too long (%d): %q", len(l), l)
		}
	}
}
