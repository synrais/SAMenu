package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"time"
)

// -------------------------
// Timestamped background log
// -------------------------
//
// Attract mode started in the background (from the menu, -attract -bg, the
// idle watcher or boot) writes its log to /tmp/SAMenu_attract.log. Each
// line gets the time and the seconds since it was asked to start, e.g.
//
//	07:02:11.4  +2.3s  [Attract] Picking: Balanced
//
// so slow steps show up. The starter passes the start time in
// SAMENU_LOG_T0 (Unix nanoseconds).

const logT0Env = "SAMENU_LOG_T0"

// logStamp is how a log line starts: the time and the seconds since t0.
func logStamp(t0 time.Time) string {
	now := time.Now()
	return fmt.Sprintf("%s.%d  +%.1fs  ", now.Format("15:04:05"), now.Nanosecond()/1e8, now.Sub(t0).Seconds())
}

// flushLog writes out any log lines still on their way through the
// timestamping pipe. It must run before the program exits: otherwise the
// last lines (the ones saying why attract mode stopped) are lost.
var flushLog = func() {}

// timestampOutput puts the time on every line this process prints, when it
// was started with SAMENU_LOG_T0 set.
func timestampOutput() {
	ns, err := strconv.ParseInt(os.Getenv(logT0Env), 10, 64)
	if err != nil {
		return
	}
	t0 := time.Unix(0, ns)
	out := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	os.Stdout, os.Stderr = w, w
	done := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			fmt.Fprintf(out, "%s%s\n", logStamp(t0), sc.Text())
		}
		close(done)
	}()
	flushLog = func() {
		// Closing the pipe lets the helper finish the lines still in it.
		os.Stdout, os.Stderr = out, out
		_ = w.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

// startedBy names what's starting attract mode in the background, for the
// first log line.
func startedBy() string {
	switch {
	case menuLock != nil:
		return "SAMenu's Options menu"
	case len(os.Args) > 1 && os.Args[1] == "-idlewatch":
		return "the idle watcher"
	case len(os.Args) > 1 && os.Args[1] == "-boot":
		return "startup"
	default:
		return "-attract -bg"
	}
}

// Background processes' logs, in /tmp (in RAM: no SD card writes, gone at
// reboot). Attract mode has its own (attractLog).
const (
	idleLog       = "/tmp/SAMenu_idle.log"
	bootLog       = "/tmp/SAMenu_boot.log"
	musicLog      = "/tmp/SAMenu_music.log"
	videoLog      = "/tmp/SAMenu_video.log"
	biosSkipLog   = "/tmp/SAMenu_biosskip.log"
	backToMenuLog = "/tmp/SAMenu_backtomenu.log"
)

// logCap is the most a background log grows to: one that reaches it
// starts again (the idle watcher runs for as long as the MiSTer is on).
var logCap int64 = 512 * 1024

// logOutputTo sends what this process prints to a log file, each line
// with the time. The file is added to, and started again once it reaches
// logCap. flushLog must run before the process ends.
func logOutputTo(path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	size := int64(0)
	if info, err := f.Stat(); err == nil {
		size = info.Size()
	}
	r, w, err := os.Pipe()
	if err != nil {
		f.Close()
		return
	}
	out := os.Stdout
	os.Stdout, os.Stderr = w, w
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer f.Close()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if size >= logCap {
				_ = f.Truncate(0)
				size = 0
				n, _ := fmt.Fprintf(f, "%s(the log reached %d KB, so it started again)\n", clock(), logCap/1024)
				size += int64(n)
			}
			n, _ := fmt.Fprintf(f, "%s%s\n", clock(), sc.Text())
			size += int64(n)
		}
	}()
	flushLog = func() {
		os.Stdout, os.Stderr = out, out
		_ = w.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

// clock is how a background log line starts: the time.
func clock() string {
	now := time.Now()
	return fmt.Sprintf("%s.%d  ", now.Format("15:04:05"), now.Nanosecond()/1e8)
}
