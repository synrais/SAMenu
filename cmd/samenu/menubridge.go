package main

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/input"
)

// -------------------------
// Controller buttons in the games menu
// -------------------------
//
// MiSTer passes only six controller buttons to scripts as keys (A enter,
// B esc, X space, Y tab, L/R page up/down). Others (Start, Select, L2...)
// can be mapped to menu actions as "pad:<name>" in [Controls.Menu]: SAMenu
// reads them itself with its input detectors and hands the menu the
// action (curses.Injected).
//
// One reader (the hub) takes the detectors' events. Normally it turns
// mapped pad buttons into menu actions; while the Input test or a "press a
// button" capture is open (takeInputs), it hands everything to that screen
// instead.
//
// The detectors only run while something needs them: a mapped pad button,
// or one of those screens. Otherwise they're paused (inputGate), reading
// nothing.

var (
	hubOnce     sync.Once
	hubStarted  atomic.Bool
	inputGate   = &input.Gate{}
	exclusive   atomic.Bool
	exclusiveCh = make(chan input.Event, 64)
)

// startInputs starts the input detectors and the hub, or resumes them.
func startInputs() {
	if inputGate.Resume() {
		time.Sleep(300 * time.Millisecond) // let the devices be found again
	}
	hubOnce.Do(func() {
		hubStarted.Store(true)
		stickRules()
		raw := input.Start(input.Options{Keyboard: true, Mouse: true, Joystick: true, Quiet: true, Gate: inputGate})
		time.Sleep(300 * time.Millisecond) // let the devices be found
		go func() {
			for ev := range raw {
				if exclusive.Load() {
					select {
					case exclusiveCh <- ev:
					default:
					}
					continue
				}
				padToMenu(ev)
			}
		}()
	})
}

// takeInputs hands every input to the caller (the Input test, a capture)
// until releaseInputs.
func takeInputs() <-chan input.Event {
	startInputs()
	for len(exclusiveCh) > 0 {
		<-exclusiveCh
	}
	exclusive.Store(true)
	return exclusiveCh
}

// releaseInputs ends takeInputs, pausing the detectors unless a mapped
// pad button still needs them.
func releaseInputs() {
	exclusive.Store(false)
	syncInputs()
}

// syncInputs runs the detectors if a menu action is mapped to a pad
// button, and pauses them if not (while a screen has taken them, they stay
// on).
func syncInputs() {
	switch {
	case padBindingsInUse():
		startInputs()
	case hubStarted.Load() && !exclusive.Load():
		inputGate.Pause()
	}
}

// actionKey is the key a mapped pad button hands the menu for an action
// (not a real key: only lists that know the action react to it).
func actionKey(act string) gc.Key {
	for i, a := range config.MenuActions {
		if a == act {
			return gc.Key(0x7000 + i)
		}
	}
	return 0
}

// padToMenu hands the menu the actions a controller button is mapped to.
func padToMenu(ev input.Event) {
	if ev.Up || ev.Ignored != "" || ev.Kind != "joystick" {
		return
	}
	name := "pad:" + strings.ToLower(ev.Name)
	for _, act := range config.MenuActions {
		for _, in := range menuControls[act] {
			if in == name {
				select {
				case curses.Injected <- actionKey(act):
				default:
				}
			}
		}
	}
}

// padBindingsInUse reports whether any menu action is mapped to a
// controller button (so the hub is needed).
func padBindingsInUse() bool {
	for _, ins := range menuControls {
		for _, in := range ins {
			if strings.HasPrefix(in, "pad:") {
				return true
			}
		}
	}
	return false
}

// keyName is the name of a key for [Controls.Menu], or "" for one that
// can't be mapped (Enter, Esc and the arrows move around the menu).
func keyName(k gc.Key) string {
	for name, keys := range namedKeys {
		for _, x := range keys {
			if x == k && name != "esc" {
				return name
			}
		}
	}
	if k > ' ' && k < 127 {
		return string(rune(k))
	}
	return ""
}

// captureMenuInput waits for a key or a controller button for a menu
// action: "tab", "p", or "pad:start". A button MiSTer turns into a key
// arrives both ways; the key wins, so it works without the hub. Back (or
// waiting) cancels.
func captureMenuInput(stdscr *gc.Window, prompt string) (string, bool) {
	_ = curses.InfoBox(stdscr, "", prompt, false, false)
	events := takeInputs()
	defer releaseInputs()
	input.SetDebug(true) // releases, for the gate
	defer input.SetDebug(false)
	accept := captureGate()
	stdscr.Timeout(50)
	defer stdscr.Timeout(-1)
	_ = gc.FlushInput()
	deadline := time.Now().Add(bindTimeout)
	pad := ""
	var padAt, keyAt time.Time
	for time.Now().Before(deadline) {
		if k := stdscr.GetChar(); k != 0 {
			keyAt = time.Now()
			if k == 27 {
				return "", false // Back
			}
			if name := keyName(k); name != "" {
				return name, true
			}
		}
		select {
		case ev := <-events:
			if accept(ev) && ev.Kind == "joystick" && pad == "" {
				pad, padAt = "pad:"+strings.ToLower(ev.Name), time.Now()
			}
		default:
		}
		// A button MiSTer also sent a key for (A, B, X, Y, L, R) is one of
		// its six, mapped by its key; only a button with no key is a pad one.
		if pad != "" && time.Since(padAt) > 300*time.Millisecond {
			if d := keyAt.Sub(padAt); d > -300*time.Millisecond && d < 300*time.Millisecond {
				pad = ""
				continue
			}
			return pad, true
		}
	}
	return "", false
}

// captureGate filters a capture's inputs so the button pressed to open it
// (A on "Change") isn't taken as the answer: the detectors can report it
// a moment after the capture has started listening. Anything pressed in
// the first moments, or still held from before, is ignored until it's let
// go. It needs the detectors' release reports (input.SetDebug).
func captureGate() func(ev input.Event) bool {
	start := time.Now()
	held := map[string]bool{}
	return func(ev input.Event) bool {
		key := ev.Kind + "|" + ev.Path + "|" + ev.Name
		switch {
		case ev.Up:
			delete(held, key)
			return false
		case ev.Ignored != "":
			return false
		case time.Since(start) < 500*time.Millisecond || held[key]:
			held[key] = true // the press that opened this: wait for its release
			return false
		}
		return true
	}
}
