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
// [Controls.BackToMenu]: when the games menu launches a game, it starts a
// small watcher ("SAMenu -backtomenu") that waits for the game's core to
// load, then for the hotkey: one or two controller buttons held for
// HoldTime seconds, or a key pressed twice quickly. Then it closes the game and goes to Target: the games
// menu (the way attract mode's SAMenu button opens it), the MiSTer menu, or
// a script run on the TV. It ends as soon as that game's
// core is left some other way (the MiSTer menu, another core, attract
// mode starting), so it only ever works in games the menu launched.

const backToMenuPidFile = "/tmp/SAMenu_backtomenu.pid"

// startBackToMenuWatcher starts the watcher for the game the menu has
// just launched, if the hotkey is on.
func startBackToMenuWatcher(cfg *config.Config) {
	if b := cfg.BackToMenu; !b.Enabled || (len(b.Buttons) == 0 && b.Key == "") {
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

// runBackToMenuWatcher is the watcher process.
func runBackToMenuWatcher() {
	stopOtherBackToMenuWatcher()
	defer func() {
		if b, err := os.ReadFile(backToMenuPidFile); err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(os.Getpid()) {
			_ = os.Remove(backToMenuPidFile)
		}
	}()
	cfg := mustConfig()
	hk := cfg.BackToMenu
	if !hk.Enabled || (len(hk.Buttons) == 0 && hk.Key == "") {
		return
	}
	hold := time.Duration(hk.HoldTime * float64(time.Second))
	to := hk.Target
	if to == config.BackToScript {
		to = hk.Script
	}
	var how []string
	if len(hk.Buttons) > 0 {
		how = append(how, fmt.Sprintf("hold %s for %.1fs", strings.Join(hk.Buttons, " + "), hk.HoldTime))
	}
	if hk.Key != "" {
		how = append(how, fmt.Sprintf("press %s twice", hk.Key))
	}
	fmt.Printf("Started: %s to go back to %s\n", strings.Join(how, ", or "), to)

	// Only once the game's core is up: asked for while it's still loading,
	// MiSTer can miss the menu core load, or load the game over it.
	core, ok := waitForGameCore()
	if !ok {
		return
	}
	fmt.Printf("Game core %s is up: watching for the hotkey\n", core)

	events := input.Start(input.Options{Keyboard: hk.Key != "", Joystick: len(hk.Buttons) > 0, Quiet: true, Releases: true,
		JoystickEvery: 100 * time.Millisecond}) // plenty for a hold of half a second or more

	held := map[string]bool{} // device + button, for the hotkey's buttons only
	var since time.Time       // when all the buttons were first held (zero = they aren't)
	var keyAt time.Time       // the key's last press
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	lastCheck := time.Now()
	for {
		select {
		case ev := <-events:
			if ev.Kind == "keyboard" && !ev.Up && strings.ToLower(ev.Name) == hk.Key {
				// Twice quickly, as attract mode's Search key opens SAMenu.
				if time.Since(keyAt) <= 400*time.Millisecond {
					goBack(hk)
					return
				}
				keyAt = time.Now()
				continue
			}
			name := padButtonName(ev)
			if name == "" || !contains(hk.Buttons, name) {
				continue
			}
			held[ev.Path+"|"+name] = !ev.Up
			if hotkeyHeld(hk.Buttons, held) {
				if since.IsZero() {
					since = time.Now()
				}
			} else {
				since = time.Time{}
			}
		case now := <-tick.C:
			if !since.IsZero() && now.Sub(since) >= hold {
				goBack(hk)
				return
			}
			// Left the game some other way: this watcher's job is done.
			if now.Sub(lastCheck) >= time.Second {
				lastCheck = now
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

// goBack closes the game and goes where the hotkey's Target says. A
// script that's gone (or none chosen) goes to the MiSTer menu instead.
func goBack(hk config.BackToMenuConfig) {
	switch hk.Target {
	case config.BackToScript:
		if _, err := os.Stat(hk.Script); hk.Script != "" && err == nil {
			fmt.Printf("Hotkey held: closing the game, running %s\n", hk.Script)
			startOnScreenHelper("script", hk.Script)
			return
		}
		fmt.Printf("Hotkey held, but the script %q isn't there: going to the MiSTer menu\n", hk.Script)
		fallthrough
	case config.BackToMiSTerMenu:
		fmt.Println("Hotkey held: closing the game, going to the MiSTer menu")
		if err := mister.LaunchMenu(); err != nil {
			fmt.Println("Couldn't load the MiSTer menu:", err)
		}
	default:
		fmt.Println("Hotkey held: closing the game, opening the games menu")
		startOnScreenHelper("menu")
	}
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
		_ = cmd.Process.Release()
	}
}
