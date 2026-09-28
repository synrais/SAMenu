package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/input"
)

// -------------------------
// Control Binding Screens
// -------------------------
//
// Options -> Controls: the menu's button layout, then attract mode's
// controls. Pick an attract action, then press what you want for it: a new
// key or button is added, one that is already bound to it is removed.
// Waiting bindTimeout cancels. The menu's own Search (Y) and Options (X)
// buttons are fixed and not listed.

const bindTimeout = 10 * time.Second

var layoutText = map[string]string{
	"Western":  "Western (A confirms, B backs out)",
	"Japanese": "Japanese (B confirms, A backs out)",
}

var attractActionNames = map[string]string{
	"next": "Next game", "back": "Previous game", "play": "Play this game",
	"stop": "Stop attract", "blacklist": "Blacklist game", "stay": "Stay on game",
	"menu": "Open SAMenu", "search": "Search (x2: menu)",
	"favourite": "Favourite", "mute": "Mute on/off", "screenshot": "Screenshot",
}

// message shows a short note for a moment.
func message(stdscr *gc.Window, text string) {
	_ = curses.InfoBox(stdscr, "", text, true, false)
	gc.Nap(1500)
}

// -------- Controls --------

// controlsScreen is Options -> Controls: the games menu's buttons, attract
// mode's, BIOS skip and the input test.
func controlsScreen(stdscr *gc.Window, cfg *config.Config, sysNames []string) {
	(&menuScreen{title: "Controls", lines: func() []menuLine {
		return []menuLine{
			opens("Games Menu...", func() { gamesMenuControls(stdscr, cfg) }),
			opens("Attract Mode...", func() { attractModeControls(stdscr, cfg) }),
			opens("BIOS Skip...", func() { biosSkipScreen(stdscr, cfg, sysNames) }),
			opens("Input test...", func() { inputTestScreen(stdscr, cfg) }),
			restoreDefaults(func() {
				// Everything on these screens.
				previous := copyBinds(cfg.AttractControls)
				cfg.AttractControls = config.DefaultAttractControlsCopy()
				cfg.MenuControls = config.DefaultMenuControlsCopy()
				menuControls = cfg.MenuControls
				cfg.MenuLayout = "Western"
				curses.SwapConfirmBack = false
				saveErr(stdscr, config.SaveMenuLayout(cfg), config.SaveMenuControls(cfg), config.SaveAttractControls(cfg, previous))
			}),
		}
	}}).run(stdscr)
}

// saveErr shows the first error from saving, if any.
func saveErr(stdscr *gc.Window, errs ...error) {
	for _, err := range errs {
		if err != nil {
			message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			return
		}
	}
}

// menuActionNames name the games menu's actions, with where they apply.
var menuActionNames = map[string]string{
	"search":    "Search (main menu)",
	"options":   "Options (main menu)",
	"favourite": "Favourite (game lists)",
	"remove":    "Remove ([Favourites])",
}

// gamesMenuControls is Controls -> Games Menu: the layout, and the menu's
// actions (keys, or controller buttons MiSTer doesn't pass on as keys).
func gamesMenuControls(stdscr *gc.Window, cfg *config.Config) {
	(&menuScreen{title: "Games Menu", lines: func() []menuLine {
		var lines []menuLine
		for _, act := range config.MenuActions {
			act := act
			lines = append(lines, setting(settingText(menuActionNames[act]+":", inputsText(cfg.MenuControls[act])), func() {
				in, ok := captureMenuInput(stdscr, fmt.Sprintf("Press a key or button for\n%s (a bound one removes it, Back or wait %ds to cancel)",
					menuActionNames[act], int(bindTimeout.Seconds())))
				if !ok {
					return
				}
				cfg.MenuControls[act] = toggleInput(cfg.MenuControls[act], in)
				menuControls = cfg.MenuControls
				saveErr(stdscr, config.SaveMenuControls(cfg))
				if padBindingsInUse() {
					startInputs() // controller buttons for the menu
				}
			}))
		}
		// Layout below the actions, so a quick double press on entering
		// can't swap confirm and back.
		return append(lines,
			setting(settingText("Menu Layout:", layoutText[cfg.MenuLayout]), func() {
				cfg.MenuLayout = nextOf([]string{"Japanese", "Western"}, cfg.MenuLayout)
				curses.SwapConfirmBack = cfg.MenuLayout == "Japanese"
				saveErr(stdscr, config.SaveMenuLayout(cfg))
			}),
			restoreDefaults(func() {
				cfg.MenuControls = config.DefaultMenuControlsCopy()
				menuControls = cfg.MenuControls
				cfg.MenuLayout = "Western"
				curses.SwapConfirmBack = false
				saveErr(stdscr, config.SaveMenuLayout(cfg), config.SaveMenuControls(cfg))
			}),
		)
	}}).run(stdscr)
}

// toggleInput adds an input to a list, or takes it off if it's there.
func toggleInput(list []string, in string) []string {
	var out []string
	found := false
	for _, x := range list {
		if x == in {
			found = true
			continue
		}
		out = append(out, x)
	}
	if !found {
		out = append(out, in)
	}
	return out
}

// inputsText shows a menu action's inputs: "key tab | pad start" (plain
// ASCII: the screens can't show other characters).
func inputsText(ins []string) string {
	var keys, pads []string
	for _, in := range ins {
		if p, ok := strings.CutPrefix(in, "pad:"); ok {
			pads = append(pads, p)
		} else {
			keys = append(keys, in)
		}
	}
	var parts []string
	if len(keys) > 0 {
		parts = append(parts, "key "+strings.Join(keys, ", "))
	}
	if len(pads) > 0 {
		parts = append(parts, "pad "+strings.Join(pads, ", "))
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " | ")
}

// attractModeControls is Controls -> Attract Mode: which inputs it
// watches, the mapping, and what unbound buttons do.
func attractModeControls(stdscr *gc.Window, cfg *config.Config) {
	d := &cfg.InputDetector
	saveDetector := func() { saveErr(stdscr, config.SaveInputDetector(cfg)) }
	toggle := func(name string, on *bool) menuLine {
		return setting(settingText(name+":", onOffText(*on)), func() {
			*on = !*on
			saveDetector()
		})
	}
	(&menuScreen{title: "Attract Mode", lines: func() []menuLine {
		lines := []menuLine{
			toggle("Mouse", &d.Mouse),
			toggle("Keyboard", &d.Keyboard),
			toggle("Controller", &d.Joystick),
			toggle("Sticks", &d.Sticks),
		}
		if d.Sticks {
			lines = append(lines, setting(settingText("Stick hold:", fmt.Sprintf("%d ms", d.StickHoldMs)), func() {
				d.StickHoldMs = nextStickHold(d.StickHoldMs)
				saveDetector()
			}))
		}
		return append(lines,
			opens("Mapping...", func() { attractMapping(stdscr, cfg) }),
			setting(settingText("Other buttons:", otherButtonsText(cfg)), func() {
				cfg.Attract.OtherInput = nextOtherButtons(cfg)
				saveErr(stdscr, config.SaveValues(cfg.Path, "Attract", [][2]string{{"OtherInput", cfg.Attract.OtherInput}}))
			}),
			restoreDefaults(func() {
				// Attract mode's mapping.
				previous := copyBinds(cfg.AttractControls)
				cfg.AttractControls = config.DefaultAttractControlsCopy()
				saveErr(stdscr, config.SaveAttractControls(cfg, previous))
			}),
		)
	}}).run(stdscr)
}

// attractMapping is Controls -> Attract Mode -> Mapping: attract mode's
// actions and the keys, buttons and mouse buttons bound to them.
func attractMapping(stdscr *gc.Window, cfg *config.Config) {
	(&menuScreen{title: "Mapping", lines: func() []menuLine {
		var lines []menuLine
		for _, act := range config.AttractActions {
			act := act
			lines = append(lines, setting(settingText(attractActionNames[act]+":", attractBindingText(cfg, act)), func() {
				ev, ok := waitForInput(stdscr, fmt.Sprintf("Press a key, button or mouse button for\n%s (a bound one removes it, wait %ds to cancel)",
					attractActionNames[act], int(bindTimeout.Seconds())))
				if !ok {
					return
				}
				previous := copyBinds(cfg.AttractControls)
				binds := cfg.AttractControls[ev.Kind]
				if binds == nil {
					binds = map[string]string{}
					cfg.AttractControls[ev.Kind] = binds
				}
				name := strings.ToLower(ev.Name)
				if binds[name] == act {
					delete(binds, name)
				} else {
					binds[name] = act // also takes it off any other action
				}
				saveErr(stdscr, config.SaveAttractControls(cfg, previous))
			}))
		}
		return append(lines, restoreDefaults(func() {
			previous := copyBinds(cfg.AttractControls)
			cfg.AttractControls = config.DefaultAttractControlsCopy()
			saveErr(stdscr, config.SaveAttractControls(cfg, previous))
		}))
	}}).run(stdscr)
}

// waitForInput waits for a press from the input detectors. Mouse movement
// and the wheel are ignored, so a nudged mouse can't bind by accident.
func waitForInput(stdscr *gc.Window, prompt string) (input.Event, bool) {
	menuInputs := takeInputs() // everything to this capture until it's done
	defer releaseInputs()
	input.SetDebug(true) // releases, for the gate
	defer input.SetDebug(false)
	accept := captureGate() // not the button pressed to open this
	_ = curses.InfoBox(stdscr, "", prompt, true, false)

	defer func() {
		// The same press usually also reaches the menu as a key (MiSTer
		// turns controller buttons into keys), so drop it.
		time.Sleep(150 * time.Millisecond)
		_ = gc.FlushInput()
	}()
	timeout := time.After(bindTimeout)
	for {
		select {
		case ev := <-menuInputs:
			if !accept(ev) {
				continue // the opening press, a release, or an input that didn't count
			}
			if ev.Kind == "mouse" && (strings.HasPrefix(ev.Name, "swipe") || strings.HasPrefix(ev.Name, "wheel")) {
				continue
			}
			return ev, true
		case <-timeout:
			return input.Event{}, false
		}
	}
}

// attractBindingText lists an action's inputs, e.g.
// "key right | pad dpright, leftx+ | mouse right".
func attractBindingText(cfg *config.Config, act string) string {
	labels := map[string]string{"keyboard": "key", "joystick": "pad", "mouse": "mouse"}
	var parts []string
	for _, kind := range []string{"keyboard", "joystick", "mouse"} {
		var ins []string
		for in, a := range cfg.AttractControls[kind] {
			if a == act {
				ins = append(ins, in)
			}
		}
		if len(ins) > 0 {
			sort.Strings(ins)
			parts = append(parts, labels[kind]+" "+strings.Join(ins, ", "))
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " | ")
}

func copyBinds(m map[string]map[string]string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for k, v := range m {
		out[k] = map[string]string{}
		for a, b := range v {
			out[k][a] = b
		}
	}
	return out
}

// BIOS skip is one choice on screen (Off, Attract, Menu, Both), kept as
// [BiosSkip] Attract and Menu in SAMenu.ini.
func biosSkipText(cfg *config.Config) string {
	switch a, m := cfg.AutoInput.Attract, cfg.AutoInput.Menu; {
	case a && m:
		return "Both"
	case a:
		return "Attract"
	case m:
		return "Menu"
	}
	return "Off"
}

// nextBiosSkip moves BIOS skip on: Off -> Attract -> Menu -> Both -> Off.
func nextBiosSkip(cfg *config.Config) {
	switch biosSkipText(cfg) {
	case "Off":
		cfg.AutoInput.Attract, cfg.AutoInput.Menu = true, false
	case "Attract":
		cfg.AutoInput.Attract, cfg.AutoInput.Menu = false, true
	case "Menu":
		cfg.AutoInput.Attract, cfg.AutoInput.Menu = true, true
	default:
		cfg.AutoInput.Attract, cfg.AutoInput.Menu = false, false
	}
}

// stickHolds are the Stick hold steps on the Controls screen (ms).
var stickHolds = []int{50, 75, 100, 150, 250}

// nextStickHold is the next step up from a hold time, round to the first;
// a value typed into the ini between steps goes to the next one up.
func nextStickHold(ms int) int {
	for _, h := range stickHolds {
		if h > ms {
			return h
		}
	}
	return stickHolds[0]
}

// Other buttons ([Attract] OtherInput): what attract mode does with an
// input that isn't bound to anything.
func otherButtonsIndex(cfg *config.Config) int {
	for i, ob := range otherButtons {
		if strings.EqualFold(ob.ini, strings.TrimSpace(cfg.Attract.OtherInput)) {
			return i
		}
	}
	return 0
}

func otherButtonsText(cfg *config.Config) string { return otherButtons[otherButtonsIndex(cfg)].label }

func nextOtherButtons(cfg *config.Config) string {
	return otherButtons[(otherButtonsIndex(cfg)+1)%len(otherButtons)].ini
}
