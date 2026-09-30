package mister

import (
	"os"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"
)

// Processor cores
//
// MiSTer's main program pins itself to core 1 (main.cpp) and serves CD
// games from there: every 13-16 ms it reads, decompresses and sends the
// next part of the disc image (CHD). Programs it starts inherit that
// pinning, so SAMenu started from the Scripts menu, and everything SAMenu
// started (attract mode, music, video, BIOS skip), all ran on core 1
// only, sharing MiSTer's own busy core while core 0 sat mostly idle. CD
// games (seen with the TurboGrafx CD) could then fail to load or crash on
// reset, but only when attract mode had been started from SAMenu, never
// over SSH, where nothing is pinned ("Cpus_allowed_list: 0-1").
//
// UseAllCores undoes the pinning, so SAMenu runs like a program started
// over SSH, and everything it starts inherits that.

// UseAllCores lets every thread of this process run on any core.
func UseAllCores() {
	var set unix.CPUSet
	for i := 0; i < runtime.NumCPU(); i++ {
		set.Set(i)
	}
	// Each thread has its own affinity (new threads copy the one that
	// makes them), so set them all.
	tasks, err := os.ReadDir("/proc/self/task")
	if err != nil {
		_ = unix.SchedSetaffinity(0, &set)
		return
	}
	for _, t := range tasks {
		if tid, err := strconv.Atoi(t.Name()); err == nil {
			_ = unix.SchedSetaffinity(tid, &set)
		}
	}
}
