package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/mister"
	"github.com/synrais/SAMenu/pkg/music"
)

// -------------------------
// Music Player and Startup
// -------------------------

// playlistName shows a playlist setting ("" is the music folder itself).
func playlistName(p string) string {
	if p == "" {
		return "Music folder"
	}
	return p
}

// musicSkipped is whether Next track was pressed since the music started:
// Previous track shows from then on (In order only: Random has no order to
// go back through).
var musicSkipped bool

// skipTrack sends the player "next" or "previous", and waits a moment for
// it to change track, so the screen shows the new one.
func skipTrack(cmd string) {
	before := music.Status()
	if music.Send(cmd) != nil {
		return
	}
	for i := 0; i < 20 && music.Status() == before; i++ {
		time.Sleep(50 * time.Millisecond)
	}
}

// musicScreen is Options -> Music Player: play or stop the player, skip a
// track (Next, then Previous) while it plays, and its settings ([Music],
// and [Startup] Music).
func musicScreen(stdscr *gc.Window, cfg *config.Config) {
	m := &cfg.Music
	save := func() {
		if err := config.SaveValues(cfg.Path, "Music", [][2]string{
			{"Playback", m.Playback}, {"Playlist", m.Playlist}, {"PauseInGames", strconv.FormatBool(m.PauseInGames)},
		}); err != nil {
			message(stdscr, fmt.Sprintf("Couldn't do that: %v", err))
		}
	}
	(&menuScreen{title: "Music Player", lines: func() []menuLine {
		var lines []menuLine
		if !music.Running() {
			musicSkipped = false
			lines = append(lines, action("Play", settingText("Play music", fmt.Sprintf("(%s)", music.Status())), func() {
				exe, err := os.Executable()
				if err == nil {
					err = music.Start(exe)
				}
				if err != nil {
					message(stdscr, fmt.Sprintf("Couldn't do that: %v", err))
				}
			}))
		} else {
			lines = append(lines,
				action("Stop", settingText("Stop music", fmt.Sprintf("(%s)", music.Status())), music.Stop),
				action("Next", "Next track", func() {
					skipTrack("next")
					musicSkipped = true
				}))
			if musicSkipped && strings.EqualFold(m.Playback, "In order") {
				lines = append(lines, action("Previous", "Previous track", func() { skipTrack("previous") }))
			}
		}
		return append(lines,
			setting(settingText("Playback:", m.Playback), func() {
				m.Playback = nextOf([]string{"Random", "In order"}, m.Playback)
				save()
			}),
			setting(settingText("Playlist:", playlistName(m.Playlist)), func() {
				m.Playlist = nextOf(music.Playlists(), m.Playlist)
				save()
			}),
			setting(settingText("Pause during games:", onOffText(m.PauseInGames)), func() {
				m.PauseInGames = !m.PauseInGames
				save()
			}),
		)
	}}).run(stdscr)
}

// startupScreen is Options -> Startup: what happens when the MiSTer boots,
// and (for attract mode "When idle") from then on.
func startupScreen(stdscr *gc.Window, cfg *config.Config) {
	st := &cfg.Startup
	start := labelOption{"On boot", []string{"Nothing", "SAMenu", "Attract mode"}, 0}
	start.set(st.Start)
	when := labelOption{"Attract starts", []string{"Instantly", "After a delay", "When idle"}, 0}
	when.set(st.AttractWhen)

	delays := []string{"30 sec", "1 min", "2 min", "5 min"}
	delaySecs := []int{30, 60, 120, 300}
	delay := labelOption{"Delay", delays, 1}
	for i, s := range delaySecs {
		if s == st.AttractDelay {
			delay.index = i
		}
	}
	press := labelOption{"A press during it", []string{"Restarts the countdown", "Cancels it", "Is ignored"}, 0}
	press.set(st.AttractPress)

	idleChoicesOn := idleChoices[1:] // no "Off": When idle needs a time
	idle := labelOption{"Idle time", append([]string(nil), idleChoicesOn...), 2}
	idle.set(fmt.Sprintf("%d min", st.IdleTime))
	where := labelOption{"Where", []string{config.IdleMenu, config.IdleGames, config.IdleBoth}, 0}
	where.set(st.IdleWhere)
	musicOn := onOffOption("Music on boot", st.Music)

	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "Startup",
		noPreview: true,
		options: func() []*labelOption {
			opts := []*labelOption{&start}
			if start.value() == "Attract mode" {
				opts = append(opts, &when)
				switch when.value() {
				case "After a delay":
					opts = append(opts, &delay, &press)
				case "When idle":
					opts = append(opts, &idle, &where)
				}
			}
			return append(opts, &musicOn)
		},
		save: func() error {
			st.Start, st.Music, st.AttractWhen = start.value(), musicOn.isOn(), when.value()
			st.AttractDelay, st.AttractPress = delaySecs[delay.index], press.value()
			st.IdleTime, st.IdleWhere = idleMinutes(idle), where.value()
			return saveStartup(cfg)
		},
	})
}

// saveStartup saves [Startup] and updates user-startup.sh to match.
func saveStartup(cfg *config.Config) error {
	if err := config.SaveValues(cfg.Path, "Startup", [][2]string{
		{"Start", cfg.Startup.Start}, {"Music", strconv.FormatBool(cfg.Startup.Music)},
		{"AttractWhen", cfg.Startup.AttractWhen},
		{"AttractDelay", fmt.Sprint(cfg.Startup.AttractDelay)}, {"AttractPress", cfg.Startup.AttractPress},
		{"IdleTime", fmt.Sprint(cfg.Startup.IdleTime)}, {"IdleWhere", cfg.Startup.IdleWhere},
	}); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	ensureIdleWatcher(cfg)
	if err := mister.UpdateStartup(cfg, exe); err != nil {
		return err
	}
	return nil
}
