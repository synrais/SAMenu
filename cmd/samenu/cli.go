package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
	"github.com/synrais/SAMenu/pkg/input"
	"github.com/synrais/SAMenu/pkg/mister"
	"github.com/synrais/SAMenu/pkg/music"
	"github.com/synrais/SAMenu/pkg/video"
)

// -------------------------
// Command Line
// -------------------------
//
// Options for running SAMenu from outside (SSH, scripts, Zaparoo,
// Home Assistant...). They're listed at the top of SAMenu.ini as a cheat
// sheet. None of them take the menu's lock, so they never stop a running
// menu or attract mode (except -launch and -random, which end attract
// mode on purpose to start their game).

// remoteCommand sends a command to a running attract mode.
func remoteCommand(cmd string) {
	if err := attract.SendCommand(cmd); err != nil {
		fmt.Println("Attract mode isn't running.")
		os.Exit(1)
	}
	fmt.Printf("Sent %q to attract mode.\n", cmd)
}

// buildDatabase indexes the games, printing progress.
func buildDatabase(cfg *config.Config) ([]MenuFile, error) {
	prevSystem, prevTotal, done := "", 0, 0
	if err := gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(s gamesdb.IndexStatus) {
		if s.Waiting {
			fmt.Println("[DB] Another database build is running, waiting for it to finish...")
			return
		}
		if prevSystem != "" {
			done++
			fmt.Printf("[DB] %d/%d %s: %d games (total %d)\n", done, s.Total-1, prevSystem, s.Files-prevTotal, s.Files)
		}
		prevSystem, prevTotal = s.SystemId, s.Files
	}); err != nil {
		return nil, err
	}
	return loadMenuDb()
}

// loadOrBuild loads the games database, building it first if needed.
func loadOrBuild(cfg *config.Config) ([]MenuFile, error) {
	if files, err := loadMenuDb(); err == nil {
		return files, nil
	}
	fmt.Println("[Menu] No games database found, building...")
	return buildDatabase(cfg)
}

func mustConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Couldn't load SAMenu.ini:", err)
		os.Exit(1)
	}
	return cfg
}

// listGames prints every game in the database (the old -print).
func listGames() {
	files, err := loadOrBuild(mustConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, f := range files {
		name := f.FileName()
		fmt.Println(filepath.Join(f.MenuPath, name))
	}
}

// rebuildDatabase rebuilds the games database with no screens.
func rebuildDatabase() {
	files, err := buildDatabase(mustConfig())
	if err != nil {
		fmt.Println("Rebuild failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Done: %d games.\n", len(files))
}

// endAttract stops a running attract mode, leaving its game running, so
// -launch and -random can start theirs.
func endAttract() {
	if attract.SendCommand("quit") == nil {
		fmt.Println("Stopping attract mode...")
		for i := 0; i < 20 && attract.SendCommand("") == nil; i++ {
			time.Sleep(250 * time.Millisecond)
		}
	}
}

// launchFile launches one game file (with BIOS skip, as a menu launch).
func launchFile(path, systemName, core string) {
	cfg := mustConfig()
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	var sys games.System
	if systemName != "" {
		// -system: launch as this system instead of guessing from the path.
		s, ok := findSystem(systemName)
		if !ok {
			fmt.Printf("No system called %q (use a system ID like SNES or Atari2600, or its name).\n", systemName)
			os.Exit(1)
		}
		sys = s
		if !games.MatchSystemFile(sys, path) {
			var exts []string
			for _, sl := range sys.Slots {
				exts = append(exts, sl.Exts...)
			}
			fmt.Printf("%s doesn't take %s files (it takes %s).\n", sys.Name, filepath.Ext(path), strings.Join(exts, " "))
			os.Exit(1)
		}
	} else {
		sys, err = games.BestSystemMatch(cfg, path)
	}
	if core != "" {
		// -core: this core instead of the system's usual one, loading the
		// file the same way.
		if systemName == "" && err != nil {
			fmt.Println("With -core, add -system too: SAMenu couldn't tell which system this file is for.")
			os.Exit(1)
		}
		sys.Rbf = mister.CoreName(core)
		err = nil
	}
	if err != nil {
		// Not a game file of a known system: cores (.rbf), MGL files and
		// arcade files (.mra) can still be launched from anywhere.
		endAttract()
		if err := mister.LaunchGenericFile(cfg, path); err != nil {
			fmt.Println("Couldn't launch it:", err)
			os.Exit(1)
		}
		fmt.Printf("Launched %s.\n", filepath.Base(path))
		return
	}
	endAttract()
	if err := mister.LaunchGame(cfg, sys, path); err != nil {
		fmt.Println("Launch failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Launched %s (%s, core %s).\n", filepath.Base(path), sys.Name, sys.Rbf)
}

// findSystem finds a system by ID or display name, ignoring capitals.
func findSystem(name string) (games.System, bool) {
	for _, s := range games.Systems {
		if strings.EqualFold(s.Id, name) || strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return games.System{}, false
}

// launchRandom launches a random game, from the systems or groups given
// ("Console", "Nintendo", "SNES") or from everything.
func launchRandom(args []string) {
	cfg := mustConfig()
	files, err := loadOrBuild(cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var want map[string]bool
	if len(args) > 0 {
		var names []string
		for _, a := range args {
			names = append(names, strings.Split(a, ",")...)
		}
		want, _ = games.ResolveSystems(names)
	}
	var pool []MenuFile
	for _, f := range files {
		if want == nil || want[strings.ToLower(f.SystemId)] {
			pool = append(pool, f)
		}
	}
	if len(pool) == 0 {
		fmt.Println("No games found for:", strings.Join(args, " "))
		os.Exit(1)
	}
	// Picked like attract mode and [Pick Random Game] ([Attract] Selection).
	f, _ := attract.NewPicker(cfg, pool, true).Next()
	sys, err := games.GetSystem(f.SystemId)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	endAttract()
	if err := mister.LaunchGame(cfg, *sys, f.Path); err != nil {
		fmt.Println("Launch failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Launched %s (%s).\n", f.Name, sys.Name)
}

// startInSearch opens SAMenu straight into its Search screen.
var startInSearch bool

// openOnTV opens SAMenu (or its Search) on the TV from outside:
// through a running attract mode, or with a background helper.
func openOnTV(search bool) {
	mode := "menu"
	if search {
		mode = "search"
	}
	if attract.SendCommand(mode) == nil {
		fmt.Println("Asked attract mode to open SAMenu on the TV.")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cmd := exec.Command(exe, "-openmenu", mode)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Println("Couldn't open SAMenu:", err)
		os.Exit(1)
	}
	_ = cmd.Process.Release()
	fmt.Println("Opening SAMenu on the TV...")
}

// musicCommand controls the music player: start, stop, next, previous or
// status.
func musicCommand(cmd string) {
	switch strings.ToLower(cmd) {
	case "start":
		exe, err := os.Executable()
		if err == nil {
			err = music.Start(exe)
		}
		if err != nil {
			fmt.Println("Couldn't start the music player:", err)
			os.Exit(1)
		}
		fmt.Println("Music player:", music.Status())
	case "stop":
		music.Stop()
		fmt.Println("Music player stopped.")
	case "next":
		if err := music.Send("next"); err != nil {
			fmt.Println("The music player isn't running.")
			os.Exit(1)
		}
		fmt.Println("Next track.")
	case "previous":
		if err := music.Send("previous"); err != nil {
			fmt.Println("The music player isn't running.")
			os.Exit(1)
		}
		fmt.Println("Previous track.")
	case "status":
		fmt.Println("Music player:", music.Status())
	default:
		fmt.Println("Usage: SAMenu.sh -music start|stop|next|previous|status")
		os.Exit(1)
	}
}

// videoCommand is -video: play, next, stop or status. "play" takes an
// optional file, folder or playlist name ("" = the [Video] playlist), or
// "-" to play a video piped in over SSH.
func videoCommand(cmd, target string) {
	switch strings.ToLower(cmd) {
	case "play":
		endAttract()
		if target == "-" {
			video.Stop()
			fmt.Println("Playing the video coming in (Ctrl+C or -video stop to stop)...")
			if err := video.PlayStream(); err != nil {
				fmt.Println("Couldn't play it:", err)
				os.Exit(1)
			}
			fmt.Println("Video finished.")
			return
		}
		vids, _, err := video.Resolve(mustConfig(), target)
		if err != nil {
			fmt.Println("Can't play that:", err)
			os.Exit(1)
		}
		exe, err := os.Executable()
		if err == nil {
			err = video.Start(exe, target)
		}
		if err != nil {
			fmt.Println("Couldn't start the video player:", err)
			os.Exit(1)
		}
		fmt.Printf("Video player started: %d video(s).\n", len(vids))
	case "stop":
		video.Stop()
		fmt.Println("Video stopped.")
	case "next":
		if err := video.Send("next"); err != nil {
			fmt.Println("No video playlist is playing.")
			os.Exit(1)
		}
		fmt.Println("Next video.")
	case "status":
		fmt.Println("Video player:", video.Status())
	default:
		fmt.Println("Usage: SAMenu.sh -video play [file|folder|playlist|-] | next | stop | status")
		os.Exit(1)
	}
}

// bootStart runs from user-startup.sh at MiSTer startup: it waits for the
// MiSTer to finish starting, then opens SAMenu or attract mode.
func bootStart(what string) {
	if err := mister.WaitForMiSTer(); err != nil {
		fmt.Println(err)
		return
	}
	switch what {
	case "attract":
		if bootCountdown() {
			_ = startAttractInBackground()
		}
	case "menu":
		_ = mister.OpenGamesMenu(false)
	}
}

// bootCountdown waits [Startup] AttractDelay before attract mode starts at
// boot, reporting false if it shouldn't start after all: a press cancelled
// it (AttractPress "Cancels it"), or attract mode is already running
// (e.g. Start when idle got there first). With "Restarts the countdown"
// every press starts the wait again; with "Is ignored" presses don't
// matter.
func bootCountdown() bool {
	cfg := mustConfig()
	delay := time.Duration(cfg.Startup.AttractDelay) * time.Second
	if cfg.Startup.AttractWhen != "After a delay" || delay <= 0 {
		return !attract.Running()
	}
	var events <-chan input.Event
	if cfg.Startup.AttractPress != "Is ignored" {
		stickRules()
		events = input.Start(input.Options{Keyboard: true, Mouse: true, Joystick: true, Quiet: true})
	}
	return countdown(delay, cfg.Startup.AttractPress, events, attract.Running, time.Second)
}

// countdown waits delay, then reports true. A press on events restarts the
// wait, or ends it with false when press is "Cancels it"; running()
// becoming true (attract mode started some other way) also ends it false.
func countdown(delay time.Duration, press string, events <-chan input.Event, running func() bool, step time.Duration) bool {
	deadline := time.Now().Add(delay)
	tick := time.NewTicker(step)
	defer tick.Stop()
	for {
		select {
		case <-events:
			if press == "Cancels it" {
				return false
			}
			deadline = time.Now().Add(delay)
		case now := <-tick.C:
			if running() {
				return false
			}
			if _, busy := mister.Busy(); busy {
				deadline = now.Add(delay) // wait for the script, then the full delay
				continue
			}
			if now.After(deadline) {
				return true
			}
		}
	}
}

// listGenres prints the genres in the games database: each genre with its
// total, then the systems it's in (sub-genres indented under it).
func listGenres() {
	files, err := loadOrBuild(mustConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	type count struct {
		total int
		bySys map[string]int
	}
	counts := map[string]*count{}
	tagged := 0
	for _, f := range files {
		if len(f.Genres) > 0 {
			tagged++
		}
		for _, g := range f.Genres {
			c := counts[g]
			if c == nil {
				c = &count{bySys: map[string]int{}}
				counts[g] = c
			}
			c.total++
			c.bySys[f.SystemId]++
		}
	}
	if len(counts) == 0 {
		fmt.Println("No genres in the games database. Rebuild it (-rebuild) so genre folders are read.")
		return
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("%d of %d games have a genre.\n\n", tagged, len(files))
	for _, k := range keys {
		c := counts[k]
		indent, name := "", k
		if p := gamesdb.GenreParent(k); p != "" {
			indent, name = "    ", k[len(p)+1:]
		}
		type sc struct {
			id string
			n  int
		}
		var sys []sc
		for id, n := range c.bySys {
			sys = append(sys, sc{id, n})
		}
		sort.Slice(sys, func(i, j int) bool { return sys[i].n > sys[j].n || sys[i].n == sys[j].n && sys[i].id < sys[j].id })
		var parts []string
		for _, s := range sys {
			parts = append(parts, fmt.Sprintf("%s %d", s.id, s.n))
		}
		fmt.Printf("%s%-24s %5d   %s\n", indent, name, c.total, strings.Join(parts, ", "))
	}
}

// checkPlaylist reports whether a playlist exists (or is "Normal"), and
// lists the ones there are if not.
func checkPlaylist(name string) bool {
	cfg := mustConfig()
	if strings.EqualFold(name, "Normal") || cfg.Playlists[strings.ToLower(name)] != nil {
		return true
	}
	fmt.Printf("No playlist called %q. Playlists: Normal", name)
	for _, n := range cfg.PlaylistNames() {
		fmt.Printf(", %s", n)
	}
	fmt.Println()
	return false
}
