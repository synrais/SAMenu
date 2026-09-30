package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input"
	"github.com/synrais/SAMenu/pkg/mister"
	"github.com/synrais/SAMenu/pkg/video"
)

// -------------------------
// Background watcher: Start When Idle, and the Back to Menu hotkey
// -------------------------
//
// The background watcher ("SAMenu -idlewatch", also called the idle
// watcher) runs from boot, and starts when one of its settings is
// switched on. One process with one set of input detectors does two jobs:
//
// [Startup] Attract mode "When idle": it starts attract mode once nothing
// has been pressed for that many minutes (IdleTime), like a screensaver,
// counting in the MiSTer menu, in games, or both (IdleWhere). After you
// choose to play one of attract mode's games, this is what starts it
// again, with the same history.
//
// [Controls.BackToMenu] WorksIn: it watches for the Back to Menu hotkey
// in every game, the MiSTer menu, or both (backtomenu.go), when it isn't
// only for games SAMenu started.
//
// It stays quiet while attract mode runs and while a video plays (and
// the hotkey while SAMenu is open), its input detectors paused when
// neither job needs them, and never acts over a script.

const idlePidFile = "/tmp/SAMenu_idle.pid"

func idleWatcherRunning() bool {
	b, err := os.ReadFile(idlePidFile)
	if err != nil {
		return false
	}
	c, err := os.ReadFile("/proc/" + strings.TrimSpace(string(b)) + "/cmdline")
	return err == nil && strings.Contains(string(c), "-idlewatch")
}

// ensureIdleWatcher starts or stops the idle watcher to match SAMenu.ini.
func ensureIdleWatcher(cfg *config.Config) {
	running := idleWatcherRunning()
	switch {
	case cfg.BackgroundWatch() && !running:
		exe, err := os.Executable()
		if err != nil {
			return
		}
		cmd := exec.Command(exe, "-idlewatch")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if cmd.Start() == nil {
			go func() { _ = cmd.Wait() }() // no zombie if it's stopped while this runs
		}
	case !cfg.BackgroundWatch() && running:
		if b, err := os.ReadFile(idlePidFile); err == nil {
			var pid int
			fmt.Sscan(strings.TrimSpace(string(b)), &pid)
			if pid > 1 {
				_ = syscall.Kill(pid, syscall.SIGTERM)
			}
		}
		_ = os.Remove(idlePidFile)
	}
}

// runIdleWatcher is the background watcher process: idle time, and the
// Back to Menu hotkey when it works beyond games started by SAMenu.
func runIdleWatcher() {
	if idleWatcherRunning() {
		return
	}
	_ = os.WriteFile(idlePidFile, []byte(fmt.Sprint(os.Getpid())), 0644)
	defer os.Remove(idlePidFile)

	stickRules()
	// The detectors sleep while nothing is being watched (below): attract
	// mode has its own, and anything they'd see then is ignored anyway.
	// Controllers are read every 100 ms, not 25: this only needs to know
	// someone is there, or that the hotkey's buttons are held (for half a
	// second or more), and it runs the whole time the MiSTer is on.
	gate := &input.Gate{}
	events := input.Start(input.Options{Keyboard: true, Mouse: true, Joystick: true, Quiet: true, Gate: gate,
		Releases: true, JoystickEvery: 100 * time.Millisecond})
	cfg := mustConfig()
	lastInput, lastLoad := time.Now(), time.Now()
	iniText, _ := os.ReadFile(cfg.Path)
	fmt.Println("Started: " + idleSettings(cfg))
	notCounting := "-" // why idle time isn't counted, as last logged ("" = it is)
	notWatching := "-" // why the hotkey isn't watched, as last logged ("" = it is)
	hk := newHotkey(cfg.BackToMenu)
	core, coreAt := "", time.Now() // the core loaded, and since when
	var quietUntil time.Time       // after the hotkey went somewhere

	// hotkeyUsed acts on the hotkey, if it's being watched and the core has
	// settled: asked for while one is still loading, MiSTer can miss the
	// menu core load, or load the game over it.
	hotkeyUsed := func() {
		hk.reset()
		c, _ := mister.GetActiveCoreName()
		switch {
		case notWatching != "" || time.Now().Before(quietUntil):
		case c != core || time.Since(coreAt) < 1500*time.Millisecond || !mister.MainRunning():
			fmt.Println("Hotkey, but a core is still loading: not now")
		case goBack(cfg.BackToMenu, c == config.MenuCore):
			quietUntil = time.Now().Add(10 * time.Second)
		}
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case ev := <-events:
			if !ev.Up {
				lastInput = time.Now()
			}
			if notWatching == "" && hk.event(ev) {
				hotkeyUsed()
			}
		case <-hk.holdDone:
			hotkeyUsed()
		case now := <-tick.C:
			// Pick up setting changes: looked at every 10 seconds, but only
			// worked through again when SAMenu.ini's text has changed (this
			// runs the whole time the MiSTer is on). The text, not the file
			// time: the SD card keeps times only to 2 seconds.
			if now.Sub(lastLoad) > 10*time.Second {
				lastLoad = now
				if text, err := os.ReadFile(cfg.Path); err == nil && !bytes.Equal(text, iniText) {
					if c, err := config.Load(); err == nil {
						cfg, iniText = c, text
						hk = newHotkey(cfg.BackToMenu)
						fmt.Println("SAMenu.ini changed: " + idleSettings(cfg))
					}
				}
			}
			if !cfg.BackgroundWatch() {
				fmt.Println("Attract when idle is off, and the hotkey isn't needed here: stopping")
				return
			}
			running := attract.Running()
			mister.UndoStaleMute(running)
			if c, err := mister.GetActiveCoreName(); err == nil && c != core {
				core, coreAt = c, now
			}
			inMenu := core == config.MenuCore
			playing := video.Playing()

			// Idle time.
			counts := cfg.Startup.IdleWhere == config.IdleBoth ||
				(cfg.Startup.IdleWhere == config.IdleMenu && inMenu) ||
				(cfg.Startup.IdleWhere == config.IdleGames && !inMenu)
			why := ""
			switch {
			case !cfg.IdleWatch():
				why = "attract when idle is off"
			case running:
				why = "attract mode is running"
			case playing:
				why = "a video is playing"
			case !counts && inMenu:
				why = "in the MiSTer menu (idle time counts in: " + cfg.Startup.IdleWhere + ")"
			case !counts:
				why = "in a game (idle time counts in: " + cfg.Startup.IdleWhere + ")"
			}
			if why != notCounting {
				if why == "" {
					fmt.Println("Counting idle time")
				} else {
					fmt.Println("Not counting idle time: " + why)
				}
				notCounting = why
			}

			// The hotkey: not over attract mode or a video (they have their
			// own controls), nor in SAMenu itself.
			hwhy := ""
			switch {
			case !cfg.HotkeyInBackground():
				hwhy = "it's off, or only for games started by SAMenu"
			case running:
				hwhy = "attract mode is running"
			case playing:
				hwhy = "a video is playing"
			case openMenuPid() > 0:
				hwhy = "SAMenu is open"
			case inMenu && !cfg.BackToMenu.InMenu():
				hwhy = "in the MiSTer menu (works in: " + cfg.BackToMenu.WorksIn + ")"
			case !inMenu && !cfg.BackToMenu.InGames():
				hwhy = "in a game (works in: " + cfg.BackToMenu.WorksIn + ")"
			}
			if hwhy != notWatching {
				if hwhy == "" {
					fmt.Println("Watching for the hotkey: " + hk.describe())
				} else {
					fmt.Println("Not watching for the hotkey: " + hwhy)
					hk.reset()
				}
				notWatching = hwhy
			}

			if why != "" && hwhy != "" {
				gate.Pause()
			} else {
				gate.Resume()
			}
			if why != "" {
				lastInput = now
				continue
			}
			if now.Sub(lastInput) >= time.Duration(cfg.Startup.IdleTime)*time.Minute {
				// Never over a script (e.g. update_all): the timer starts
				// again. Looked for only now, as it reads every process.
				if what, busy := mister.Busy(); busy {
					fmt.Printf("Nothing pressed for %d min, but something is running (%s): not now\n", cfg.Startup.IdleTime, what)
				} else {
					fmt.Printf("Nothing pressed for %d min: starting attract mode\n", cfg.Startup.IdleTime)
					if err := startAttractInBackground(); err != nil {
						fmt.Println("Couldn't start attract mode:", err)
					}
				}
				lastInput = now
			}
		}
	}
}

// idleSettings describes the background watcher's settings, for its log.
func idleSettings(cfg *config.Config) string {
	idle := "attract when idle off"
	if cfg.IdleWatch() {
		idle = fmt.Sprintf("attract mode after %d min with nothing pressed, counting in: %s",
			cfg.Startup.IdleTime, cfg.Startup.IdleWhere)
	}
	hotkey := "hotkey not watched here (works in: " + cfg.BackToMenu.WorksIn + ")"
	if !cfg.HotkeyWatched() {
		hotkey = "hotkey off"
	} else if cfg.HotkeyInBackground() {
		hotkey = fmt.Sprintf("hotkey in: %s, %s", cfg.BackToMenu.WorksIn, newHotkey(cfg.BackToMenu).describe())
	}
	return idle + "; " + hotkey
}

// applyBackgroundWatch starts or stops the background watcher to match
// SAMenu.ini, and updates user-startup.sh so it starts at boot or not.
func applyBackgroundWatch(cfg *config.Config) error {
	ensureIdleWatcher(cfg)
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return mister.UpdateStartup(cfg, exe)
}
