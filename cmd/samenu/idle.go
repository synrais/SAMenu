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
// Start When Idle
// -------------------------
//
// [Startup] Attract mode "When idle": the idle watcher ("SAMenu -idlewatch") runs in
// the background (started at boot, and when the setting is switched on)
// and starts attract mode once nothing has been pressed for that many
// minutes (IdleTime), like a screensaver, counting in the MiSTer menu,
// in games, or both (IdleWhere). After you choose to play one of attract
// mode's games, this is what starts it again, with the same history.
// It stays quiet while attract mode runs and while a video plays, its
// input detectors paused, and never starts attract mode over a script.

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
	case cfg.IdleWatch() && !running:
		exe, err := os.Executable()
		if err != nil {
			return
		}
		cmd := exec.Command(exe, "-idlewatch")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if cmd.Start() == nil {
			go func() { _ = cmd.Wait() }() // no zombie if it's stopped while this runs
		}
	case !cfg.IdleWatch() && running:
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

// runIdleWatcher is the idle watcher process.
func runIdleWatcher() {
	if idleWatcherRunning() {
		return
	}
	_ = os.WriteFile(idlePidFile, []byte(fmt.Sprint(os.Getpid())), 0644)
	defer os.Remove(idlePidFile)

	stickRules()
	// The detectors sleep while nothing is being counted (below): attract
	// mode has its own, and anything they'd see then is ignored anyway.
	// Controllers are read every 100 ms, not 25: this only needs to know
	// someone is there, and it runs the whole time the MiSTer is on.
	gate := &input.Gate{}
	events := input.Start(input.Options{Keyboard: true, Mouse: true, Joystick: true, Quiet: true, Gate: gate,
		JoystickEvery: 100 * time.Millisecond})
	cfg := mustConfig()
	lastInput, lastLoad := time.Now(), time.Now()
	iniText, _ := os.ReadFile(cfg.Path)
	fmt.Println("Started: " + idleSettings(cfg))
	notCounting := "" // why idle time isn't counted, as last logged ("" = it is)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-events:
			lastInput = time.Now()
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
						fmt.Println("SAMenu.ini changed: " + idleSettings(cfg))
					}
				}
			}
			if !cfg.IdleWatch() {
				fmt.Println("Switched off in Startup: stopping")
				return
			}
			running := attract.Running()
			mister.UndoStaleMute(running)
			inMenu := mister.IsMenuRunning()
			counts := cfg.Startup.IdleWhere == config.IdleBoth ||
				(cfg.Startup.IdleWhere == config.IdleMenu && inMenu) ||
				(cfg.Startup.IdleWhere == config.IdleGames && !inMenu)
			why := ""
			switch {
			case running:
				why = "attract mode is running"
			case video.Playing():
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
					fmt.Println("Not counting: " + why)
				}
				notCounting = why
			}
			if why != "" {
				lastInput = now
				gate.Pause()
				continue
			}
			gate.Resume()
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

// idleSettings describes the idle watcher's settings, for its log.
func idleSettings(cfg *config.Config) string {
	return fmt.Sprintf("attract mode after %d min with nothing pressed, counting in: %s",
		cfg.Startup.IdleTime, cfg.Startup.IdleWhere)
}
