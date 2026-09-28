package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
	"github.com/synrais/SAMenu/pkg/mister"
	"github.com/synrais/SAMenu/pkg/music"
	"github.com/synrais/SAMenu/pkg/video"
)

// -------------------------
// Aliases
// -------------------------

type MenuFile = gamesdb.FileInfo

// errGameLaunched unwinds the menu so the program exits once a game starts.
var errGameLaunched = errors.New("game launched")

// errAttractStarted is Options -> Start attract mode: a kind of
// errGameLaunched (the menu closes), but the menu then exits the way Exit
// does, so attract mode starts from a clean MiSTer menu (see main).
var errAttractStarted = fmt.Errorf("attract mode started: %w", errGameLaunched)

// afterMenuEnv tells attract mode started from the menu which process to
// wait for (the menu), before it launches its first game.
const afterMenuEnv = "SAMENU_AFTER_MENU"

// -------------------------
// Load Gob Index
// -------------------------

// loadMenuDb loads the games database, shared with search (gamesdb keeps
// the one copy in memory).
func loadMenuDb() ([]MenuFile, error) { return gamesdb.Load() }

// -------------------------
// Index Generation
// -------------------------

func generateIndexWindow(cfg *config.Config, stdscr *gc.Window) ([]MenuFile, error) {
	clearScreen(stdscr)

	win, err := curses.NewWindow(stdscr, 4, 75, "", -1)
	if err != nil {
		return nil, err
	}
	defer win.Delete()

	_, width := win.MaxYX()

	drawProgressBar := func(current, total int) {
		if total == 0 {
			return
		}
		progressWidth := width - 4
		progressPct := int(float64(current) / float64(total) * float64(progressWidth))
		if progressPct < 1 {
			progressPct = 1
		}
		for i := 0; i < progressPct; i++ {
			win.MoveAddChar(2, 2+i, gc.ACS_BLOCK)
		}
		win.NoutRefresh()
	}

	clearText := func() { win.MovePrint(1, 2, strings.Repeat(" ", width-4)) }

	// Progress from the build, read by the screen while it runs.
	var mu sync.Mutex
	var progress gamesdb.IndexStatus

	err = withSpinner(func() error {
		err := gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			mu.Lock()
			progress = is
			mu.Unlock()
		})
		return err
	}, func(spin string) {
		mu.Lock()
		p := progress
		mu.Unlock()

		clearText()
		win.MovePrint(1, width-3, spin)

		countText := fmt.Sprintf("%6d files", p.Files)
		countCol := width - len(countText) - 6
		win.MovePrint(1, countCol, countText)

		sysText := fmt.Sprintf("Indexing %s...", games.DisplayName(p.SystemId))
		if maxSysWidth := countCol - 10; len(sysText) > maxSysWidth {
			sysText = sysText[:maxSysWidth]
		}
		win.MovePrint(1, 2, sysText)

		drawProgressBar(p.Step, p.Total)
		win.NoutRefresh()
	})
	clearScreen(stdscr)
	if err != nil {
		return nil, err
	}

	return loadingWindow(stdscr, loadMenuDb)
}

// -------------------------
// Options Menu
// -------------------------

func optionsMenu(cfg *config.Config, stdscr *gc.Window, files []MenuFile, sysIds []string) ([]MenuFile, error) {
	// Each group stays open after its screens, with the same entry
	// highlighted, until Back. Rebuilding the database and starting
	// attract mode leave the options straight away.
	var newFiles []MenuFile
	var err error
	leave := false // the options are finished: rebuilt, or attract started
	// group opens one group of options, leaving them all when it's done.
	group := func(title string, lines ...menuLine) menuLine {
		return opens(title, nil).leaves(func() bool {
			(&menuScreen{title: title, lines: func() []menuLine { return lines }}).run(stdscr)
			return leave
		})
	}
	rebuild := func() bool {
		newFiles, err = generateIndexWindow(cfg, stdscr)
		leave = true
		return true
	}
	options := &menuScreen{title: "Options", lines: func() []menuLine {
		return []menuLine{
			group("Game Database",
				opens("Rebuild games database...", nil).leaves(func() bool {
					// A quick double press on entering Options lands here, so
					// ask first, with Cancel highlighted.
					return confirm(stdscr, "Rebuild the games database?", "Rebuild (can take a few minutes)", "Cancel") && rebuild()
				}),
				opens("Database systems...", nil).leaves(func() bool {
					return databaseSystemsScreen(stdscr, cfg) && rebuild()
				}),
				opens("Cores...", func() { coresScreen(stdscr, cfg, sysIds) }),
			),
			group("Attract Mode",
				action("Start", "Start attract mode", nil).leaves(func() bool {
					if e := startAttractInBackground(); e != nil {
						_ = curses.InfoBox(stdscr, "Error",
							fmt.Sprintf("Failed to start attract mode: %v", e), false, true)
						return false
					}
					err, leave = errAttractStarted, true // the menu exits, attract mode carries on
					return true
				}),
				opens("Attract mode settings...", func() { attractSettingsScreen(stdscr, cfg, sysIds) }),
				opens("Playlists...", func() { playlistsScreen(stdscr, cfg, files) }),
				opens("Detector & list settings...", func() { detectorSettingsScreen(stdscr, cfg) }),
			),
			group("Display & Sorting",
				opens("Menu list options...", func() { menuListOptions(stdscr, sysIds, cfg) }),
				opens("Menu list sorting...", func() { systemsSortOptions(stdscr, sysIds, cfg) }),
				opens("Game list sorting...", func() { gameSortOptions(stdscr, cfg) }),
				opens("Virtual A-Z Folders...", func() { azFoldersScreen(stdscr, cfg, sysIds) }),
				opens("[Pick Random Game]...", func() { randomGameScreen(stdscr, cfg) }),
				opens("[Genre Collection]...", func() { genresScreen(stdscr, cfg, files) }),
				opens("[Favourites]...", func() { favouritesScreen(stdscr, cfg) }),
				opens("[History]...", func() { historyScreen(stdscr, cfg) }),
			),
			opens("Controls", func() { controlsScreen(stdscr, cfg, sysIds) }),
			opens("Music Player", func() { musicScreen(stdscr, cfg) }),
			opens("Video Player", func() { videoScreen(stdscr, cfg) }),
			opens("Startup", func() { startupScreen(stdscr, cfg) }),
		}
	}}
	options.run(stdscr)
	return newFiles, err
}

// optionsList shows a list of choices and returns the one chosen, or false
// for Back.
func optionsList(stdscr *gc.Window, title string, items []string, selected int) (int, bool) {
	clearScreen(stdscr)
	button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Shortcuts:     menuShortcuts(),
		Title:         title,
		Buttons:       []string{"Select", "Back"},
		DefaultButton: 0,
		ActionButton:  0,
		Width:         60,
		Height:        len(items) + 4,
		InitialIndex:  selected,
	}, items)
	clearScreen(stdscr)
	if err != nil || button != 0 || sel < 0 || sel >= len(items) {
		return 0, false
	}
	return sel, true
}

// -------------------------
// Tree Navigation
// -------------------------

// browseNode shows one folder of the games tree. It reports whether the user
// left it with Back, so the parent can keep Back highlighted and the user can
// hammer Back to climb out of deep folders (see landingButton).
type browseEntry = gamesdb.Entry

func browseNode(cfg *config.Config, stdscr *gc.Window, node *gamesdb.Node, depth int) (bool, error) {
	const actionButton, backButton = 2, 3
	currentIndex := 0
	cameBack := false // the last folder opened here was backed out of
	first := true
	pickedRandom := false // the random entry was the last one used
	for {
		clearScreen(stdscr)

		entries := node.Entries(gameOrder(), optFolders.value())

		// [Pick Random Game] on top (r rows before the entries).
		r := randomRows(cfg)
		items := make([]string, r, len(entries)+r)
		if r > 0 {
			items[0] = randomLabel
		}
		inFavourites := node == favouritesNode
		hasGames := false
		for _, e := range entries {
			if e.File == nil {
				items = append(items, e.Folder)
			} else {
				hasGames = true
				label := gameName(e.File.Name, e.File.Ext)
				// Favourites get a mark (after sorting, so it doesn't move them).
				if cfg.Menu.Favourites && !inFavourites && isFavourite(e.File.Path) {
					label = favouriteMark + label
				}
				items = append(items, label)
			}
		}
		if currentIndex < r && len(entries) > 0 && !pickedRandom {
			currentIndex = r // start on the first real entry
		}

		title := node.Name
		if title == "" {
			title = "Games"
		}

		// Walking back to a remembered position: start on its line here,
		// and open it straight away if the position goes deeper.
		autoOpen := false
		if first {
			first = false
			if idx, ok, found, open := restoreAt(depth, func(i int) string { return browseKey(entries[i]) }, len(entries)); ok {
				currentIndex = idx + r
				autoOpen = found && open && entries[idx].File == nil
				if !autoOpen {
					doneRestoring(depth)
				}
			}
		}

		buttons := []string{"PgUp", "PgDn", "", "Back"}
		// Favourites: Fav (Square) marks the highlighted game in a game list;
		// in [Favourites], Remove (Triangle, or Square) takes it off.
		shortcuts := map[gc.Key]string{}
		favButton := ""
		if cfg.Menu.Favourites && hasGames {
			favButton = "Fav"
			shortcuts = menuShortcutsFor("favourite")
			if inFavourites {
				// In [Favourites], Remove and Favourite both take it off.
				favButton = "Remove"
				shortcuts = menuShortcutsFor("favourite", "remove")
				for k := range shortcuts {
					shortcuts[k] = favButton
				}
			}
			buttons = append(buttons, favButton)
		}
		button, selected, err := actionButton, currentIndex, error(nil)
		if !autoOpen {
			button, selected, err = curses.ListPicker(stdscr, curses.ListPickerOpts{
				Shortcuts:     shortcuts,
				ItemButtons:   []string{"Fav", "Remove"}, // act on the highlighted game
				Title:         title,
				Buttons:       buttons,
				ActionButton:  actionButton,
				DefaultButton: landingButton(buttons, actionButton, cameBack),
				SnapToAction:  true,
				ShowTotal:     true,
				Width:         systemListWidth,
				Height:        listHeight,
				InitialIndex:  currentIndex,
				DynamicActionLabel: func(idx int) string {
					if idx < r {
						return "Pick"
					}
					if idx -= r; idx >= 0 && idx < len(entries) && entries[idx].File == nil {
						return "Open"
					}
					return "Launch"
				},
			}, items)
		}
		if err != nil {
			return false, err
		}

		// Keep our place in this list (Back reports no selection).
		if selected >= 0 {
			currentIndex = selected
			pickedRandom = selected < r
			if i := selected - r; i >= 0 && i < len(entries) {
				navHere(depth, browseKey(entries[i]), i)
			}
		}
		cameBack = false

		// Fav / Remove on the highlighted game.
		if favButton != "" && button == len(buttons)-1 {
			if i := selected - r; i >= 0 && i < len(entries) && entries[i].File != nil {
				f := originalGame(*entries[i].File)
				toggleFavourite(f)
				if inFavourites {
					// Rebuild the folder without it, in place.
					fresh := buildFavouritesNode(cfg)
					node.Files, node.SortKeys = fresh.Files, fresh.SortKeys
					favouritesNode = node
					if len(node.Files) == 0 {
						clearScreen(stdscr)
						return true, nil
					}
				}
			}
			continue
		}

		switch button {
		case actionButton:
			clearScreen(stdscr)
			if selected >= 0 && selected < r {
				if err := pickRandomGame(stdscr, cfg, nodeFiles(node)); err != nil {
					savePosition()
					return false, err
				}
				continue
			}
			selected -= r
			if selected < 0 || selected >= len(entries) {
				continue
			}
			if e := entries[selected]; e.File == nil {
				wentBack, err := browseNode(cfg, stdscr, folderNode(node, e), depth+1)
				if err != nil {
					return false, err
				}
				cameBack = wentBack
			} else {
				file := entries[selected].File
				sys, err := games.GetSystem(file.SystemId)
				if err == nil && mister.LaunchGame(cfg, *sys, file.Path) == nil {
					recordHistory(cfg, originalGame(*file))
					savePosition()
					return false, errGameLaunched
				}
				clearScreen(stdscr)
			}
		case backButton:
			clearScreen(stdscr)
			return true, nil
		}
	}
}

// -------------------------
// Main Menu
// -------------------------

func mainMenu(cfg *config.Config, stdscr *gc.Window, files []MenuFile) error {
	setupPosition(cfg)
	st := &menuState{}
	st.build(stdscr, files)
	if startInSearch {
		if err := searchWindow(cfg, stdscr); err != nil {
			return err
		}
	}
	return systemsScreen(cfg, stdscr, st, "Systems", nil, true)
}

// -------------------------
// Search Window
// -------------------------

func searchWindow(cfg *config.Config, stdscr *gc.Window) error {
	clearScreen(stdscr)

	text := ""
	startIndex := 0
	count := newMatchCounter(cfg)
	for {
		// Controller keys: Cross types (and presses Search), Circle backs
		// out, Square types a space, Triangle deletes, L/R move the cursor
		// (see KeyboardOpts). The live match count shows under the text.
		gc.Cursor(1)
		button, query, err := curses.OnScreenKeyboardWith(stdscr, "Search", []string{"Search", "Back"}, text,
			curses.KeyboardOpts{PadKeys: true, OnTextChange: count.changed, Status: count.status})
		gc.Cursor(0)

		// Only the Search button searches: Back and Esc (Circle) back out.
		if err != nil || button != 0 {
			clearScreen(stdscr)
			return nil
		}
		text = query

		// Nothing to search for: stay on the keyboard (searching for nothing
		// would list every game).
		if strings.TrimSpace(query) == "" {
			_ = curses.InfoBox(stdscr, "", "Type something to search for first.", false, false)
			gc.Nap(1200)
			clearScreen(stdscr)
			continue
		}
		startIndex = 0
		_ = curses.InfoBox(stdscr, "", "Searching...", false, false)

		var results []gamesdb.SearchResult
		err = withSpinner(func() (err error) {
			results, err = gamesdb.SearchNamesWords(query)
			return err
		}, func(spin string) {
			_ = curses.InfoBox(stdscr, "", "Searching... "+spin, false, false)
		})
		clearScreen(stdscr)
		if err != nil {
			return err
		}

		// Search results always read "[System] Title.ext": systems A-Z, then
		// titles A-Z, whatever the menu's sorting and extension settings.
		if cfg.Menu.SearchHidden && !menuHide.Empty() { // [Menu] HideTags
			kept := results[:0]
			for _, r := range results {
				if !menuHide.Matches(r.Name) {
					kept = append(kept, r)
				}
			}
			results = kept
		}
		gamesdb.SortResultsBySystem(results, games.DisplayName)
		if len(results) == 0 {
			_ = curses.InfoBox(stdscr, "", "No results found.", false, true)
			clearScreen(stdscr)
			continue
		}

		var items []string
		for _, r := range results {
			items = append(items, fmt.Sprintf("[%s] %s", games.DisplayName(r.SystemId), r.FileName()))
		}

		for {
			clearScreen(stdscr)
			button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
				Shortcuts:     menuShortcuts(),
				Title:         "Search Results",
				Buttons:       []string{"PgUp", "PgDn", "Launch", "Back"},
				ActionButton:  2,
				DefaultButton: 2,
				ShowTotal:     true,
				Width:         systemListWidth,
				Height:        listHeight,
				InitialIndex:  startIndex,
			}, items)
			if err != nil {
				return err
			}
			startIndex = selected
			if button == 2 {
				game := results[selected]
				sys, err := games.GetSystem(game.SystemId)
				if err == nil && mister.LaunchGame(cfg, *sys, game.Path) == nil {
					recordHistory(cfg, MenuFile{SystemId: game.SystemId, Name: game.Name, Ext: game.Ext, Path: game.Path})
					savePosition()
					return errGameLaunched
				}
				clearScreen(stdscr)
			} else if button == 3 {
				clearScreen(stdscr)
				break
			}
		}
	}
}

// -------------------------
// Loading Spinner
// -------------------------

func loadingWindow(stdscr *gc.Window, loadFn func() ([]MenuFile, error)) ([]MenuFile, error) {
	var files []MenuFile
	err := withSpinner(func() (err error) {
		files, err = loadFn()
		return err
	}, func(spin string) {
		_ = curses.InfoBox(stdscr, "", "Loading... "+spin, false, false)
	})
	clearScreen(stdscr)
	if err != nil {
		return nil, err
	}
	return files, nil
}

// -------------------------
// Attract (command line)
// -------------------------

// runAttract starts attract mode without the menu screens (SAMenu.sh
// -attract), printing its progress to the terminal. The games database is
// built first if it doesn't exist yet.
func runAttract(cfg *config.Config) {
	files, err := loadOrBuild(cfg)
	if err != nil {
		fmt.Println(err)
		flushLog()
		os.Exit(1)
	}
	err = attract.StartAttractMode(cfg, files)
	if err != nil {
		fmt.Println(err)
	}
	flushLog() // the last lines (why it stopped) reach the log
	if err != nil {
		os.Exit(1)
	}
}

// -------------------------
// Main Entry
// -------------------------

func main() {
	// Started from the Scripts menu, we inherit MiSTer's own core (core 1,
	// where its main program runs): use any core, like over SSH, and so
	// does everything we start.
	mister.UseAllCores()

	// -h shows helpText (help.go): add any new option there too.
	listPtr := flag.Bool("list", false, "Print every game in the database, one per line")
	genresPtr := flag.Bool("genres", false, "Print the genres found in the games database, with counts per system")
	printPtr := flag.Bool("print", false, "Same as -list")
	statusPtr := flag.Bool("status", false, "Show whether attract mode is running, and what it's playing")
	stopPtr := flag.Bool("stop", false, "Stop attract mode and go back to the MiSTer menu")
	nextPtr := flag.Bool("next", false, "Attract mode: next game")
	backPtr := flag.Bool("back", false, "Attract mode: previous game")
	playPtr := flag.Bool("play", false, "Attract mode: stop the show and keep playing this game")
	stayPtr := flag.Bool("stay", false, "Attract mode: stay on this game (again to resume)")
	blacklistPtr := flag.Bool("blacklist", false, "Attract mode: never play this game again, next game")
	rebuildPtr := flag.Bool("rebuild", false, "Rebuild the games database (no screens)")
	launchPtr := flag.String("launch", "", "Launch a game file, e.g. -launch /media/fat/games/NES/Tetris.nes")
	systemPtr := flag.String("system", "", "With -launch: launch as this system (ID or name) instead of guessing, e.g. -system Atari2600")
	corePtr := flag.String("core", "", "With -launch: use this core instead of the system's usual one, e.g. -core _Unstable/NES")
	randomPtr := flag.Bool("random", false, "Launch a random game, from the systems or groups given (e.g. -random Nintendo)")
	bgPtr := flag.Bool("bg", false, "With -attract: run in the background (log in /tmp/SAMenu_attract.log)")
	playlistPtr := flag.String("playlist", "", "With -attract: use this playlist for this run (Normal = the usual setup)")
	menuPtr := flag.Bool("menu", false, "Open SAMenu on the TV (closes the running game)")
	searchPtr := flag.Bool("search", false, "Open SAMenu's Search on the TV (closes the running game)")
	openMenuPtr := flag.String("openmenu", "", "Internal: open SAMenu on the TV (menu or search)")
	findPtr := flag.Bool("find", false, "Play the games matching the words first, e.g. -find mario 3 (starts attract mode if it isn't running)")
	herePtr := flag.Bool("here", false, "With -search or -menu: open it in this terminal (e.g. over SSH), not on the TV")
	musicPtr := flag.String("music", "", "Music player: start, stop, next, previous or status")
	musicdPtr := flag.Bool("musicd", false, "Internal: the music player process")
	videoPtr := flag.String("video", "", "Video player: play [file|folder|playlist|-], next, stop or status")
	videodPtr := flag.Bool("videod", false, "Internal: the video player process")
	bootPtr := flag.String("boot", "", "Internal: run at MiSTer startup (menu or attract)")
	idleWatchPtr := flag.Bool("idlewatch", false, "Internal: the idle watcher (starts attract mode when idle)")
	attractPtr := flag.Bool("attract", false, "Start attract mode straight away (no menu screens), optionally only some systems or groups, e.g. -attract SNES,Console")
	watchPtr := flag.Bool("watch", false, "Show the static detector's live status (leaves a running menu or attract mode alone)")
	inputsPtr := flag.Bool("inputs", false, "Print every key, mouse and controller press, to test the input detectors (leaves a running menu or attract mode alone)")
	pressPtr := flag.Bool("press", false, "Press buttons on SAMenu's virtual pad or keyboard, e.g. -press start (leaves a running menu or attract mode alone)")
	autoInputPtr := flag.String("autoinput", "", "Internal: run a BIOS skip sequence in the background")
	flag.Parse()

	// Options that only work with another one say so, instead of being
	// quietly ignored.
	for _, need := range []struct {
		used, with bool
		msg        string
	}{
		{*bgPtr, *attractPtr, "-bg only works with -attract, e.g. -attract -bg"},
		{*playlistPtr != "", *attractPtr, "-playlist only works with -attract, e.g. -attract -playlist \"Fighters\""},
		{*herePtr, *menuPtr || *searchPtr, "-here only works with -menu or -search"},
		{*systemPtr != "" || *corePtr != "", *launchPtr != "", "-system and -core only work with -launch, e.g. -launch <file> -system SNES"},
	} {
		if need.used && !need.with {
			fmt.Println(need.msg)
			os.Exit(1)
		}
	}
	timestampOutput() // background attract mode: times in its log

	if *listPtr || *printPtr {
		listGames()
		return
	}
	if *genresPtr {
		listGenres()
		return
	}
	if *statusPtr {
		fmt.Println(attract.Status())
		return
	}
	for cmd, on := range map[string]bool{"stop": *stopPtr, "next": *nextPtr, "back": *backPtr,
		"play": *playPtr, "stay": *stayPtr, "blacklist": *blacklistPtr} {
		if on {
			remoteCommand(cmd)
			return
		}
	}
	if *idleWatchPtr {
		runIdleWatcher()
		return
	}
	if *musicdPtr {
		music.Run(mustConfig())
		return
	}
	if *musicPtr != "" {
		musicCommand(*musicPtr)
		return
	}
	if *videodPtr {
		video.Run(mustConfig(), strings.Join(flag.Args(), " "))
		return
	}
	if *videoPtr != "" {
		videoCommand(*videoPtr, strings.Join(flag.Args(), " "))
		return
	}
	if *bootPtr != "" {
		// A mute left by attract mode before a restart (MiSTer keeps it on
		// the SD card) is undone.
		mister.UndoStaleMute(false)
		bootStart(*bootPtr)
		return
	}
	if *findPtr && !*attractPtr { // -attract -find: attract mode's own start (below)
		query := strings.TrimSpace(strings.Join(flag.Args(), " "))
		if query == "" {
			fmt.Println("Usage: SAMenu.sh -find <words>, e.g. -find mario 3")
			os.Exit(1)
		}
		// Attract mode running: it plays them next. Not running: start it,
		// playing them first, then carrying on as usual.
		if attract.SendCommand("find:"+query) == nil {
			fmt.Printf("Attract mode will play the games matching %q first.\n", query)
			return
		}
		if err := startAttractInBackground("-find", query); err != nil {
			fmt.Println("Couldn't start attract mode:", err)
			os.Exit(1)
		}
		fmt.Printf("Attract mode started, playing the games matching %q first. Log: %s\n", query, attractLog)
		return
	}
	if *openMenuPtr != "" {
		if err := mister.OpenGamesMenu(*openMenuPtr == "search"); err != nil {
			fmt.Println("Couldn't open SAMenu:", err)
			if f, e := os.OpenFile(mister.OpenMenuLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); e == nil {
				fmt.Fprintln(f, "FAILED:", err)
				f.Close()
			}
		}
		return
	}
	// On the TV these just open the menu (-search straight into Search);
	// from anywhere else they open it on the TV.
	if (*menuPtr || *searchPtr) && !mister.OnConsole() && !*herePtr {
		openOnTV(*searchPtr)
		return
	}
	startInSearch = *searchPtr

	if *rebuildPtr {
		rebuildDatabase()
		return
	}
	if *launchPtr != "" {
		launchFile(*launchPtr, *systemPtr, *corePtr)
		return
	}
	if *randomPtr {
		launchRandom(flag.Args())
		return
	}
	if *attractPtr && *playlistPtr != "" && !checkPlaylist(*playlistPtr) {
		return
	}
	if *attractPtr && *bgPtr {
		args := flag.Args()
		if *playlistPtr != "" {
			args = append([]string{"-playlist", *playlistPtr}, args...)
		}
		if err := startAttractInBackground(args...); err != nil {
			fmt.Println("Couldn't start attract mode:", err)
			os.Exit(1)
		}
		fmt.Println("Attract mode started in the background. Log: " + attractLog)
		return
	}

	// Watching only reads a status file, so it never takes the lock (which
	// would stop a running menu or attract mode).
	if *watchPtr {
		watchDetector()
		return
	}
	if *inputsPtr {
		testInputs()
		return
	}
	if *autoInputPtr != "" {
		mister.RunAutoInput(*autoInputPtr)
		return
	}
	if *pressPtr {
		testPress(flag.Args())
		return
	}

	lockFile := "/tmp/SAMenu.lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("failed to open lock file: %v", err)
	}
	defer f.Close()
	menuLock = f

	tryLock := func() error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
	if err := tryLock(); err != nil {
		buf := make([]byte, 32)
		n, _ := f.ReadAt(buf, 0)
		if n > 0 {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(buf[:n]))); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				gc.Nap(500)
			}
		}
		if err := tryLock(); err != nil {
			log.Fatal("failed to acquire lock even after killing old process")
		}
	}

	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprint(f, os.Getpid())
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	applyMenuConfig(cfg.Menu)
	applyAZConfig(cfg.Menu)
	menuControls = cfg.MenuControls
	if padBindingsInUse() {
		startInputs() // controller buttons mapped to menu actions
	}
	curses.SwapConfirmBack = cfg.MenuLayout == "Japanese"

	if *attractPtr {
		// Attract mode can run for hours on a MiSTer with ~500 MB shared
		// with Linux and MiSTer's own program: keep Go's memory in check.
		debug.SetMemoryLimit(128 * 1024 * 1024)
		waitForMenuExit()

		// Systems or groups after -attract replace [Attract] Include for
		// this run, e.g. -attract Nintendo,Console.
		if *playlistPtr != "" {
			cfg.Attract.Playlist = *playlistPtr // this run only; SAMenu.ini is left alone
		}
		if *findPtr { // started by -find: the words are the search, played first
			attract.StartWithFind = strings.TrimSpace(strings.Join(flag.Args(), " "))
		} else if args := flag.Args(); len(args) > 0 {
			cfg.Attract.Include = nil
			for _, a := range args {
				cfg.Attract.Include = append(cfg.Attract.Include, strings.Split(a, ",")...)
			}
		}
		runAttract(cfg)
		return
	}

	applyTextSize() // needs the menu settings applied above, before the screen is set up
	mister.UndoStaleMute(attract.Running())
	ensureIdleWatcher(cfg)

	stdscr, err := curses.Setup()
	if err != nil {
		log.Fatal(err)
	}
	defer gc.End()
	if err := fitToScreen(stdscr); err != nil {
		gc.End()
		fmt.Println(err)
		return
	}

	gc.Cursor(0)

	files, err := loadingWindow(stdscr, loadMenuDb)
	if err != nil {
		files, err = generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = mainMenu(cfg, stdscr, files)
	if err != nil && !errors.Is(err, errGameLaunched) {
		log.Fatal(err)
	}
	// Exit, and Start attract mode: leave the way Exit does. Attract mode
	// (started in the background) waits for this, so its first game loads
	// from a clean MiSTer menu, not the finished script screen with the
	// menu's text size still set, which some cores don't load from.
	if err == nil || errors.Is(err, errAttractStarted) {
		gc.End()
		if mister.OnConsole() {
			// On the MiSTer's own screen, go straight back to the MiSTer
			// menu. Otherwise MiSTer shows "Press any key to continue"
			// after the script ends. Reloading the menu also resets the
			// text size.
			if mister.LaunchMenu() == nil {
				return
			}
		}
		restoreTextSize()
	}
}

// menuLock is this process's lock file (see main).
var menuLock *os.File

// attractLog is where attract mode started from the menu writes its output.
const attractLog = "/tmp/SAMenu_attract.log"

// startAttractInBackground runs "SAMenu -attract" as its own process, in
// its own session, so it keeps running after the menu exits. The menu's
// lock is released first, so the new process takes it over instead of
// stopping the menu.
func startAttractInBackground(systems ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(attractLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	t0 := time.Now()
	fmt.Fprintf(logFile, "%sAsked to start attract mode (by %s)\n", logStamp(t0), startedBy())

	if menuLock != nil {
		_ = menuLock.Truncate(0)
		_ = syscall.Flock(int(menuLock.Fd()), syscall.LOCK_UN)
	}
	cmd := exec.Command(exe, append([]string{"-attract"}, systems...)...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%d", logT0Env, t0.UnixNano()))
	if menuLock != nil { // from the menu: wait for it to finish exiting
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%d", afterMenuEnv, os.Getpid()))
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// buildTree builds the games tree, with each system folder named from its
// current system name (so a renamed system never shows up as "Other").
func buildTree(files []MenuFile) *gamesdb.Node {
	tree := gamesdb.BuildTree(files)
	tree.RenameSystems(func(id string) string {
		if sys, err := games.GetSystem(id); err == nil {
			return sys.Name
		}
		return ""
	})
	addAZFolders(tree)
	return tree
}

// folderNode is the folder an entry opens: a real subfolder, or a disc set.
func folderNode(parent *gamesdb.Node, e gamesdb.Entry) *gamesdb.Node {
	if e.Node != nil {
		return e.Node
	}
	return parent.Children[e.Folder]
}

// waitForMenuExit is for attract mode started from the menu: it waits for
// the menu's process to end (it exits the way Exit does, reloading the
// MiSTer menu), then gives that reload time to finish, so the first game
// isn't loaded over the finished script screen, or wiped by the reload.
func waitForMenuExit() {
	pid := os.Getenv(afterMenuEnv)
	if pid == "" {
		return
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat("/proc/" + pid); err != nil {
			break // the menu has gone
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(2 * time.Second) // the MiSTer menu core reloading
	fmt.Println("[Attract] SAMenu has closed, starting")
}
