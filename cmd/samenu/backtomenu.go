package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Back to Menu hotkey
// -------------------------
//
// [Controls.BackToMenu]: one or two controller buttons held for HoldTime
// seconds, or a key pressed twice quickly, close the game and go to
// Target: the games menu (the way attract mode's SAMenu button opens it),
// the MiSTer menu, or a script run on the TV.
//
// Where it works is [Controls.BackToMenu] WorksIn. Games started by
// SAMenu: the games menu starts a small watcher ("SAMenu -backtomenu") for
// each game it launches, which waits for the game's core to load and ends
// as soon as that core is left some other way. Games, MiSTer menu or
// Anywhere: the background watcher (idle.go) watches for it, from boot.

const backToMenuPidFile = "/tmp/SAMenu_backtomenu.pid"

// hotkey follows the inputs for the Back to Menu hotkey. Feed it every
// input with event; holdDone fires once the buttons have been held long
// enough.
type hotkey struct {
	cfg      config.BackToMenuConfig
	held     map[string]bool  // device + button, for the hotkey's buttons only
	holdDone <-chan time.Time // nil while the buttons aren't all held
	keyAt    time.Time        // the key's last press
}

func newHotkey(cfg config.BackToMenuConfig) *hotkey {
	return &hotkey{cfg: cfg, held: map[string]bool{}}
}

// event takes one input, reporting whether it was the key's second press
// (twice quickly, as attract mode's Search key opens SAMenu).
func (h *hotkey) event(ev input.Event) bool {
	if ev.Kind == "keyboard" {
		if !ev.Up && h.cfg.Key != "" && strings.ToLower(ev.Name) == h.cfg.Key {
			if time.Since(h.keyAt) <= 400*time.Millisecond {
				h.keyAt = time.Time{}
				return true
			}
			h.keyAt = time.Now()
		}
		return false
	}
	name := padButtonName(ev)
	if name == "" || !contains(h.cfg.Buttons, name) {
		return false
	}
	h.held[ev.Path+"|"+name] = !ev.Up
	switch {
	case !hotkeyHeld(h.cfg.Buttons, h.held):
		h.holdDone = nil
	case h.holdDone == nil:
		h.holdDone = time.After(time.Duration(h.cfg.HoldTime * float64(time.Second)))
	}
	return false
}

// reset forgets what's held and pressed (while the hotkey isn't watched).
func (h *hotkey) reset() {
	h.held, h.holdDone, h.keyAt = map[string]bool{}, nil, time.Time{}
}

// describe says what the hotkey is and does, for the logs.
func (h *hotkey) describe() string {
	b := h.cfg
	var how []string
	if len(b.Buttons) > 0 {
		how = append(how, fmt.Sprintf("hold %s for %.1fs", strings.Join(b.Buttons, " + "), b.HoldTime))
	}
	if b.Key != "" {
		how = append(how, fmt.Sprintf("press %s twice", b.Key))
	}
	to := b.Target
	if to == config.BackToScript {
		to = b.Script
	}
	return strings.Join(how, ", or ") + " to go to " + to
}

// startBackToMenuWatcher starts the watcher for the game the menu has
// just launched, if the hotkey is on for games the menu launches only
// (otherwise the background watcher has it, or it's off in games).
func startBackToMenuWatcher(cfg *config.Config) {
	if !cfg.HotkeyWatched() || cfg.HotkeyInBackground() {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "-backtomenu")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		_ = cmd.Process.Release() // the menu exits straight after
	}
}

// stopOtherBackToMenuWatcher ends a watcher left from an earlier game (one
// at a time), and records this one.
func stopOtherBackToMenuWatcher() {
	if b, err := os.ReadFile(backToMenuPidFile); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if c, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); pid > 1 && pid != os.Getpid() &&
			err == nil && strings.Contains(string(c), "-backtomenu") {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
	_ = os.WriteFile(backToMenuPidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// runBackToMenuWatcher is the watcher process for one game.
func runBackToMenuWatcher() {
	stopOtherBackToMenuWatcher()
	defer func() {
		if b, err := os.ReadFile(backToMenuPidFile); err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(os.Getpid()) {
			_ = os.Remove(backToMenuPidFile)
		}
	}()
	cfg := mustConfig()
	b := cfg.BackToMenu
	if !cfg.HotkeyWatched() || cfg.HotkeyInBackground() {
		return
	}
	hk := newHotkey(b)
	fmt.Println("Started: " + hk.describe())

	// Only once the game's core is up: asked for while it's still loading,
	// MiSTer can miss the menu core load, or load the game over it.
	core, ok := waitForGameCore()
	if !ok {
		return
	}
	fmt.Printf("Game core %s is up: watching for the hotkey\n", core)

	events := input.Start(input.Options{Keyboard: b.Key != "", Joystick: len(b.Buttons) > 0, Quiet: true, Releases: true,
		JoystickEvery: 100 * time.Millisecond}) // plenty for a hold of half a second or more

	check := time.NewTicker(time.Second)
	defer check.Stop()
	for {
		select {
		case ev := <-events:
			if hk.event(ev) && goBack(b, false) {
				return
			}
		case <-hk.holdDone:
			hk.reset()
			if goBack(b, false) {
				return
			}
		case <-check.C:
			// Left the game some other way: this watcher's job is done.
			if c, err := mister.GetActiveCoreName(); err == nil && c != core {
				fmt.Printf("Core is now %s: stopping\n", c)
				return
			}
			if attract.Running() {
				fmt.Println("Attract mode started: stopping")
				return
			}
		}
	}
}

// waitForGameCore waits for the launched game's core to be loaded and
// settled (the same core for a second, with MiSTer's program running),
// and returns its name. It gives up if no game core loads within a minute.
func waitForGameCore() (string, bool) {
	deadline := time.Now().Add(time.Minute)
	last, steady := "", 0
	for time.Now().Before(deadline) {
		c, err := mister.GetActiveCoreName()
		switch {
		case err != nil || c == config.MenuCore || !mister.MainRunning():
			steady = 0
		case c == last:
			steady++
		default:
			steady = 1
		}
		last = c
		if steady >= 5 { // 5 reads, 250 ms apart
			return c, true
		}
		time.Sleep(250 * time.Millisecond)
	}
	fmt.Println("No game core loaded within a minute: stopping")
	return "", false
}

// padButtonName is a controller button as [Controls.BackToMenu] Buttons
// names it: BIOS skip's names (start, select, a = right...), or the raw
// name for one it has none for (btn9). Sticks and the d-pad can't be used
// (their releases aren't reported), nor anything but a controller.
func padButtonName(ev input.Event) string {
	if ev.Kind != "joystick" || ev.Axis || ev.Ignored != "" {
		return ""
	}
	name := strings.ToLower(ev.Name)
	if n, ok := sdlToBios[name]; ok {
		return n
	}
	return name
}

// hotkeyHeld reports whether every one of the hotkey's inputs is held (on
// any device).
func hotkeyHeld(buttons []string, held map[string]bool) bool {
	for _, b := range buttons {
		found := false
		for k, down := range held {
			if down && strings.HasSuffix(k, "|"+b) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// goBack closes the game and goes where the hotkey's Target says,
// reporting whether it did. Never over a script (e.g. update_all), and
// in the MiSTer menu (inMenu), going to the MiSTer menu does nothing. A
// script that's gone (or none chosen) goes to the MiSTer menu instead.
func goBack(hk config.BackToMenuConfig, inMenu bool) bool {
	if what, busy := mister.Busy(); busy {
		fmt.Printf("Hotkey, but something is running (%s): not now\n", what)
		return false
	}
	target := hk.Target
	if target == config.BackToScript {
		if _, err := os.Stat(hk.Script); hk.Script != "" && err == nil {
			fmt.Printf("Hotkey: running %s\n", hk.Script)
			startOnScreenHelper("script", hk.Script)
			return true
		}
		fmt.Printf("Hotkey, but the script %q isn't there: going to the MiSTer menu instead\n", hk.Script)
		target = config.BackToMiSTerMenu
	}
	switch target {
	case config.BackToMiSTerMenu:
		if inMenu {
			fmt.Println("Hotkey: already in the MiSTer menu, nothing to do")
			return false
		}
		fmt.Println("Hotkey: closing the game, going to the MiSTer menu")
		if err := mister.LaunchMenu(); err != nil {
			fmt.Println("Couldn't load the MiSTer menu:", err)
		}
	default:
		fmt.Println("Hotkey: opening the games menu")
		startOnScreenHelper("menu")
	}
	return true
}

// startOnScreenHelper opens the games menu, or runs a script, on the TV
// in a helper of its own ("SAMenu -openmenu"), which stays with it until
// it ends.
func startOnScreenHelper(args ...string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, append([]string{"-openmenu"}, args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err == nil {
		// Collected when it ends: the background watcher lives on, and
		// each one would otherwise stay in the process list as a zombie.
		go func() { _ = cmd.Wait() }()
	}
}
