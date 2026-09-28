package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/synrais/SAMenu/pkg/config"
)

// Main program watchdog
//
// MiSTer's main program (the menu, the OSD, passing controllers to cores)
// can die on a bad file, e.g. the Atari 800 core crash. The core keeps
// running but nothing responds. MainRunning spots that, and RestartMain
// starts it again, the same as running this by hand:
//
//	cd /media/fat && ./MiSTer > /tmp/MiSTer.log 2>&1 &

// mainLog keeps the restarted program's messages (in RAM), so a crash can
// be looked into: tail -50 /tmp/MiSTer.log
const mainLog = "/tmp/MiSTer.log"

// MainRunning reports whether the MiSTer main program is running.
func MainRunning() bool {
	// The one found last time, if it's still there: one small read instead
	// of every process's.
	if pid := mainPid.Load(); pid > 0 {
		if isMain(fmt.Sprintf("/proc/%d/comm", pid)) {
			return true
		}
		mainPid.Store(0)
	}
	dirs, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, f := range dirs {
		if isMain(f) {
			if pid, err := strconv.ParseInt(filepath.Base(filepath.Dir(f)), 10, 64); err == nil {
				mainPid.Store(pid)
			}
			return true
		}
	}
	return false
}

// mainPid is the MiSTer main program's process, as last found (0: none).
var mainPid atomic.Int64

// isMain reports whether a /proc/<pid>/comm is the MiSTer main program's.
func isMain(comm string) bool {
	b, err := os.ReadFile(comm)
	return err == nil && strings.TrimSpace(string(b)) == "MiSTer"
}

// RestartMain starts the MiSTer main program in its own session and waits
// for it to be ready to take commands.
func RestartMain() error {
	exe := filepath.Join(config.SdFolder, "MiSTer")
	logFile, err := os.OpenFile(mainLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(exe)
	cmd.Dir = config.SdFolder
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Collect its exit if it ever stops: otherwise it would stay in the
	// process list as a zombie, and MainRunning would still find it.
	go func() { _ = cmd.Wait() }()

	// Give it time to start up and load the menu core before the next
	// launch command arrives.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if MainRunning() {
			if _, err := os.Stat(config.CmdInterface); err == nil {
				time.Sleep(3 * time.Second)
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("MiSTer main program didn't come back up")
}

// EnsureMain restarts the main program if it isn't running, and reports
// whether it had to.
func EnsureMain() (restarted bool, err error) {
	if MainRunning() {
		return false, nil
	}
	return true, RestartMain()
}
