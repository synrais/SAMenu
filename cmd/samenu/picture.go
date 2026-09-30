package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Screen -> Picture: MiSTer.ini's picture settings
// -------------------------
//
// The settings that decide whether a screen shows a picture at all, in
// plain words: what's plugged in (My screen sets the rest to suit it), the
// analog signal, sync, how the menu and games go out on the analog port,
// the resolution. Changes are made to a copy of the settings file in RAM
// first and tried (the menu core is loaded again), with a countdown back
// to the old ones: the new ones are then kept until reboot, or for good
// (mister/picture.go). Each choice is the MiSTer.ini lines it sets, in
// [MiSTer] (every core) or [Menu] (only the MiSTer menu, and SAMenu).

// iniSet is one MiSTer.ini setting a choice makes. An empty value is
// MiSTer's own default: the setting is commented out.
type iniSet struct{ section, key, value string }

// pictureChoice is one value of a picture setting.
type pictureChoice struct {
	text string
	sets []iniSet
}

// pictureSetting is one line of the Picture screen.
type pictureSetting struct {
	label   string
	choices []pictureChoice
	shown   func(text string) bool // nil: always
}

func mi(key, value string) iniSet   { return iniSet{"MiSTer", key, value} }
func menu(key, value string) iniSet { return iniSet{"Menu", key, value} }

// iniDefaults are MiSTer's own values for the settings used here, when
// MiSTer.ini doesn't set them.
var iniDefaults = map[string]string{
	"vga_mode": "rgb", "ntsc_mode": "0", "composite_sync": "0", "vga_sog": "0",
	"forced_scandoubler": "0", "vga_scaler": "0", "direct_video": "0",
	"hdmi_limited": "0", "vscale_border": "0", "video_mode": "",
}

// iniEffective is a setting's value as MiSTer uses it: [Menu] falls back
// to [MiSTer], and that to MiSTer's default.
func iniEffective(text, section, key string) string {
	if v, ok := mister.IniGet(text, section, key); ok {
		return strings.ToLower(strings.TrimSpace(v))
	}
	if section == "Menu" {
		return iniEffective(text, "MiSTer", key)
	}
	return iniDefaults[key]
}

// matches reports whether all of a choice's settings are in effect.
func (c pictureChoice) matches(text string) bool {
	for _, s := range c.sets {
		want := s.value
		if want == "" {
			want = iniDefaults[s.key]
		}
		if iniEffective(text, s.section, s.key) != want {
			return false
		}
	}
	return true
}

// apply makes a choice's settings in the text.
func (c pictureChoice) apply(text string) string {
	for _, s := range c.sets {
		text = mister.IniSet(text, s.section, s.key, s.value, s.value == "")
	}
	return text
}

// current is the choice in effect, or -1 for none of them (set by hand).
// When several match (My screen's HDMI one only sets HDMI things, so it
// matches most analog setups too), the one setting the most wins.
func (s pictureSetting) current(text string) int {
	best := -1
	for i, c := range s.choices {
		if c.matches(text) && (best < 0 || len(c.sets) > len(s.choices[best].sets)) {
			best = i
		}
	}
	return best
}

func analogIs(modes ...string) func(string) bool {
	return func(text string) bool {
		v := iniEffective(text, "MiSTer", "vga_mode")
		for _, m := range modes {
			if v == m {
				return true
			}
		}
		return false
	}
}

// The screens My screen knows. Each analog one sets everything about the
// analog output and the HDMI to VGA adapter; the resolution, range and
// border stay as they are.
func screenPreset(text string, vgaMode, csync, scandoubler, gamesScaler, menuScaler, direct string, more ...iniSet) pictureChoice {
	sets := []iniSet{mi("vga_mode", vgaMode), mi("composite_sync", csync), mi("vga_sog", "0"),
		mi("forced_scandoubler", scandoubler), mi("vga_scaler", gamesScaler), menu("vga_scaler", menuScaler),
		mi("direct_video", direct)}
	return pictureChoice{text, append(sets, more...)}
}

var myScreen = pictureSetting{"My screen:", []pictureChoice{
	// HDMI only: the analog output's settings don't matter to it, and
	// stay as they are.
	{"HDMI TV or monitor", []iniSet{mi("direct_video", "0")}},
	screenPreset("CRT TV, RGB (SCART)", "rgb", "1", "0", "0", "1", "0"),
	screenPreset("CRT TV, component", "ypbpr", "0", "0", "0", "1", "0"),
	screenPreset("CRT TV, S-Video", "svideo", "0", "0", "0", "1", "0"),
	screenPreset("CRT TV, composite", "cvbs", "0", "0", "0", "1", "0"),
	screenPreset("PC monitor, VGA", "rgb", "0", "1", "0", "1", "0"),
	screenPreset("HDMI to VGA adapter", "rgb", "0", "0", "0", "0", "1", mi("hdmi_limited", "2")),
}, nil}

var (
	analogSignal = pictureSetting{"Analog signal:", []pictureChoice{
		{"RGB", []iniSet{mi("vga_mode", "rgb")}},
		{"Component (YPbPr)", []iniSet{mi("vga_mode", "ypbpr")}},
		{"S-Video", []iniSet{mi("vga_mode", "svideo")}},
		{"Composite", []iniSet{mi("vga_mode", "cvbs")}},
	}, nil}
	colourSystem = pictureSetting{"Colour system:", []pictureChoice{
		{"NTSC", []iniSet{mi("ntsc_mode", "0")}},
		{"PAL-60", []iniSet{mi("ntsc_mode", "1")}},
		{"PAL-M", []iniSet{mi("ntsc_mode", "2")}},
	}, analogIs("svideo", "cvbs")}
	syncSetting = pictureSetting{"Sync:", []pictureChoice{
		{"Separate (VGA)", []iniSet{mi("composite_sync", "0"), mi("vga_sog", "0")}},
		{"Combined (SCART)", []iniSet{mi("composite_sync", "1"), mi("vga_sog", "0")}},
		{"On green", []iniSet{mi("composite_sync", "0"), mi("vga_sog", "1")}},
	}, analogIs("rgb")}
	menuOnAnalog = pictureSetting{"Menu on analog:", []pictureChoice{
		{"Native", []iniSet{menu("vga_scaler", "0")}},
		{"Scaled, fits any screen", []iniSet{menu("vga_scaler", "1")}},
	}, nil}
	gamesOnAnalog = pictureSetting{"Games on analog:", []pictureChoice{
		{"Native, no lag", []iniSet{mi("forced_scandoubler", "0"), mi("vga_scaler", "0")}},
		{"Doubled, for PC monitors", []iniSet{mi("forced_scandoubler", "1"), mi("vga_scaler", "0")}},
		{"Scaled, small lag", []iniSet{mi("forced_scandoubler", "0"), mi("vga_scaler", "1")}},
	}, nil}
	resolution = pictureSetting{"Resolution:", []pictureChoice{
		{"Auto, from the screen", []iniSet{mi("video_mode", "")}},
		{"640x480 60Hz", []iniSet{mi("video_mode", "6")}},
		{"720x480 60Hz", []iniSet{mi("video_mode", "2")}},
		{"720x576 50Hz", []iniSet{mi("video_mode", "3")}},
		{"800x600 60Hz", []iniSet{mi("video_mode", "5")}},
		{"1024x600 60Hz", []iniSet{mi("video_mode", "11")}},
		{"1024x768 60Hz", []iniSet{mi("video_mode", "1")}},
		{"1280x720 60Hz", []iniSet{mi("video_mode", "0")}},
		{"1280x720 50Hz", []iniSet{mi("video_mode", "7")}},
		{"1280x1024 60Hz", []iniSet{mi("video_mode", "4")}},
		{"1366x768 60Hz", []iniSet{mi("video_mode", "10")}},
		{"1920x1080 60Hz", []iniSet{mi("video_mode", "8")}},
		{"1920x1080 50Hz", []iniSet{mi("video_mode", "9")}},
		{"1920x1440 60Hz", []iniSet{mi("video_mode", "12")}},
		{"2048x1536 60Hz", []iniSet{mi("video_mode", "13")}},
		{"2560x1440 60Hz", []iniSet{mi("video_mode", "14")}},
	}, nil}
	topBottomBorder = pictureSetting{"Top and bottom cut off:", []pictureChoice{
		{"Off", []iniSet{mi("vscale_border", "0")}},
		{"Small border", []iniSet{mi("vscale_border", "16")}},
		{"Medium border", []iniSet{mi("vscale_border", "32")}},
		{"Large border", []iniSet{mi("vscale_border", "48")}},
	}, nil}
	hdmiRange = pictureSetting{"HDMI colour range:", []pictureChoice{
		{"Full, for monitors", []iniSet{mi("hdmi_limited", "0")}},
		{"Limited, for TVs", []iniSet{mi("hdmi_limited", "1")}},
		{"For HDMI to VGA adapters", []iniSet{mi("hdmi_limited", "2")}},
	}, nil}
	hdmiToVGA = pictureSetting{"HDMI to VGA adapter:", []pictureChoice{
		{"No", []iniSet{mi("direct_video", "0")}},
		{"Yes, direct video", []iniSet{mi("direct_video", "1")}},
	}, nil}
)

// pictureSettings are the Picture screen's lines after My screen, in order.
var pictureSettings = []pictureSetting{analogSignal, colourSystem, syncSetting, menuOnAnalog, gamesOnAnalog, resolution, topBottomBorder, hdmiRange, hdmiToVGA}

// choiceText shows a setting's value: its choice, or Custom when
// MiSTer.ini has something else (set by hand).
func choiceText(s pictureSetting, text string) string {
	if i := s.current(text); i >= 0 {
		return s.choices[i].text
	}
	return "Custom"
}

// nextChoice moves a setting on to its next choice (Custom to the first).
func nextChoice(s pictureSetting, text string) string {
	i := s.current(text) + 1
	return s.choices[i%len(s.choices)].apply(text)
}

// myTty is the console this program is on ("tty2"), or "".
func myTty() string {
	link, err := os.Readlink("/proc/self/fd/0")
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(link, "/dev/")
}

// pictureScreen is Options -> Screen -> Picture.
func pictureScreen(stdscr *gc.Window) {
	name, known := mister.ActiveIni()
	applied, err := mister.ReadIni(name)
	if err != nil {
		message(stdscr, fmt.Sprintf("Couldn't read %s: %v", name, err))
		return
	}
	pending := applied

	// try puts the new settings in use and asks whether the picture's
	// there, going back by itself if there's no answer.
	try := func() {
		if !mister.OnConsole() {
			message(stdscr, "Picture settings can only be tried on the MiSTer's own screen.")
			return
		}
		wasTemp := mister.IniTemporary(name)
		_ = curses.InfoBox(stdscr, "", "Trying the new picture settings.\nThe screen goes dark for a few seconds.", true, false)
		reload := func() (string, error) {
			read, err := mister.ReloadMenuHere(myTty())
			textSize.measured = false // the screen may have a new size
			applyTextSizeLive(stdscr)
			stdscr.Clear()
			return read, err
		}
		goBack := func() {
			if wasTemp {
				_ = mister.SetIniTemporary(name, applied)
			} else {
				_ = mister.DropIniTemporary(name)
			}
			_, _ = reload()
		}
		if err := mister.SetIniTemporary(name, pending); err != nil {
			message(stdscr, err.Error())
			return
		}
		read, err := reload()
		if err != nil {
			goBack()
			message(stdscr, fmt.Sprintf("Couldn't try them: %v. Back to the old settings.", err))
			return
		}
		if read != name {
			// MiSTer is using another of its settings files (picked in
			// its own menu): the change went to the wrong one.
			goBack()
			name, known = read, true
			applied, _ = mister.ReadIni(name)
			pending = applied
			message(stdscr, fmt.Sprintf("MiSTer is using %s, not the file just changed. The settings now show %s: change them again.", read, read))
			return
		}
		switch curses.CountdownChoice(stdscr, "New picture settings",
			"Can you see this? If not, wait: it goes back by itself. Games on the analog output can only be checked in a game.",
			[]string{"Go back", "Until reboot", "Keep"}, 0, 15) {
		case 1:
			applied = pending
		case 2:
			if err := mister.SaveIni(name, pending); err != nil {
				message(stdscr, fmt.Sprintf("Couldn't save them: %v. They stay until reboot.", err))
			}
			applied = pending
		default:
			goBack()
			pending = applied
			message(stdscr, "Back to the old picture settings.")
		}
		clearScreen(stdscr)
	}

	(&menuScreen{title: "Picture", lines: func() []menuLine {
		lines := []menuLine{setting(settingText(myScreen.label, choiceText(myScreen, pending)), func() {
			pending = nextChoice(myScreen, pending)
		})}
		for _, s := range pictureSettings {
			s := s
			if s.shown != nil && !s.shown(pending) {
				continue
			}
			lines = append(lines, setting(settingText(s.label, choiceText(s, pending)), func() {
				pending = nextChoice(s, pending)
			}))
		}
		if pending != applied {
			lines = append(lines, action("Try", "Try these settings", try))
		}
		if mister.IniTemporary(name) {
			lines = append(lines, action("Undo", "Undo the temporary settings", func() {
				saved, _ := mister.ReadIni(name) // as it was before (the copy's taken off)
				if !confirm(stdscr, "Undo the temporary picture settings?", "Undo them", "Keep them") {
					return
				}
				if err := mister.DropIniTemporary(name); err != nil {
					message(stdscr, err.Error())
					return
				}
				applied, _ = mister.ReadIni(name)
				pending = applied
				if applied != saved && mister.OnConsole() {
					_, _ = mister.ReloadMenuHere(myTty())
					textSize.measured = false
					applyTextSizeLive(stdscr)
					stdscr.Clear()
				}
			}))
		}
		return append(lines, restoreDefaults(func() {
			// MiSTer's own defaults for these settings (to try).
			for _, s := range pictureSettings {
				pending = s.choices[0].apply(pending)
			}
			pending = myScreen.choices[0].apply(pending)
		}))
	}, preview: func() []string {
		return pictureSummary(name, known, pending, applied)
	}, leave: func() {
		if pending != applied && confirm(stdscr, "Try the new picture settings?", "Try them", "Leave them") {
			try()
		}
	}}).run(stdscr)
}

// pictureSummary says in words what the settings do, for the preview.
func pictureSummary(name string, known bool, text, applied string) []string {
	res := choiceText(resolution, text)
	if strings.HasPrefix(res, "Auto") {
		res = "the screen's own resolution"
	}
	analog := choiceText(analogSignal, text)
	switch iniEffective(text, "MiSTer", "vga_mode") {
	case "rgb":
		analog += ", " + strings.ToLower(choiceText(syncSetting, text)) + " sync"
	case "svideo", "cvbs":
		analog += ", " + choiceText(colourSystem, text)
	}
	games := "native (15 kHz for most cores)"
	switch iniEffective(text, "MiSTer", "vga_scaler") + iniEffective(text, "MiSTer", "forced_scandoubler") {
	case "10", "11":
		games = "scaled to " + res
	case "01":
		games = "doubled to 31 kHz"
	}
	menuOut := "native"
	if iniEffective(text, "Menu", "vga_scaler") == "1" {
		menuOut = "scaled to " + res
	}
	hdmi := res + ", " + strings.ToLower(strings.SplitN(choiceText(hdmiRange, text), ",", 2)[0]) + " range"
	if iniEffective(text, "MiSTer", "direct_video") == "1" {
		hdmi = "direct video, for an HDMI to VGA adapter"
	}
	file := name
	switch {
	case text != applied:
		file += ", not tried yet"
	case mister.IniTemporary(name):
		file += ", temporary until reboot"
	case !known:
		file += " (if MiSTer uses another, trying finds it)"
	}
	return []string{
		"Analog: " + analog,
		"Games:  " + games,
		"Menu:   " + menuOut + " on analog",
		"HDMI:   " + hdmi,
		"File:   " + filepath.Base(file),
	}
}

// screenCommand is -screen: the picture settings from the command line,
// e.g. to undo them from a PC when the screen shows nothing.
func screenCommand(cmd string) {
	switch strings.ToLower(cmd) {
	case "status":
		name, known := mister.ActiveIni()
		text, err := mister.ReadIni(name)
		if err != nil {
			fmt.Println("Couldn't read", name+":", err)
			os.Exit(1)
		}
		if !known {
			fmt.Printf("Settings file: %s (probably: MiSTer hasn't been seen reading one yet)\n", name)
		} else {
			fmt.Println("Settings file:", name)
		}
		for _, f := range mister.IniFiles() {
			if mister.IniTemporary(f) {
				fmt.Printf("%s has temporary settings from SAMenu, until reboot (-screen undo takes them off)\n", f)
			}
			if _, err := os.Stat(mister.IniBackup(f)); err == nil {
				fmt.Printf("%s as it was before SAMenu changed it: %s (-screen restore puts it back)\n", f, mister.IniBackup(f))
			}
		}
		fmt.Println(settingText(myScreen.label, choiceText(myScreen, text)))
		for _, s := range pictureSettings {
			if s.shown == nil || s.shown(text) {
				fmt.Println(settingText(s.label, choiceText(s, text)))
			}
		}
	case "undo", "restore":
		changed := false
		for _, f := range mister.IniFiles() {
			var err error
			switch {
			case cmd == "undo" && mister.IniTemporary(f):
				err = mister.DropIniTemporary(f)
			case cmd == "restore":
				if _, e := os.Stat(mister.IniBackup(f)); e != nil && !mister.IniTemporary(f) {
					continue
				}
				if _, e := os.Stat(mister.IniBackup(f)); e != nil {
					err = mister.DropIniTemporary(f)
				} else {
					err = mister.RestoreIni(f)
				}
			default:
				continue
			}
			if err != nil {
				fmt.Printf("%s: %v\n", f, err)
				os.Exit(1)
			}
			fmt.Printf("%s: back as it was\n", f)
			changed = true
		}
		if !changed {
			fmt.Println("Nothing to undo.")
			return
		}
		// Load the MiSTer menu again so it takes effect (not over a game:
		// the next one loaded picks it up).
		if mister.IsMenuRunning() {
			if openMenuPid() > 0 {
				fmt.Println("SAMenu is open: it takes effect when the MiSTer menu next loads.")
			} else if err := mister.LaunchMenu(); err == nil {
				fmt.Println("MiSTer menu loaded again with the old settings.")
			}
		}
	default:
		fmt.Println("Usage: SAMenu.sh -screen status|undo|restore")
		fmt.Println("  status   the picture settings in use, and any temporary ones")
		fmt.Println("  undo     take off temporary settings (as a reboot would)")
		fmt.Println("  restore  also put MiSTer.ini back as it was before SAMenu changed it for good")
		os.Exit(1)
	}
}
