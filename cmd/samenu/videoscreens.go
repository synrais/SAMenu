package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/utils"
	"github.com/synrais/SAMenu/pkg/video"
)

// -------------------------
// Video Player
// -------------------------
//
// Options -> Video Player: browse /media/fat/video and play a video (or a
// whole folder), and the settings for videos in attract mode ([Video]).

// videoAttractChoices are the "In attract mode" steps: 0 = never.
var videoAttractChoices = []int{0, 3, 5, 10, 20}

func videoAttractText(n int) string {
	if n <= 0 {
		return "Off"
	}
	return fmt.Sprintf("Every %d games", n)
}

// videoFolderName shows a playlist setting ("" is the video folder itself).
func videoFolderName(p string) string {
	if p == "" {
		return "Video folder"
	}
	return p
}

// videoScreen is Options -> Video Player.
func videoScreen(stdscr *gc.Window, cfg *config.Config) {
	v := &cfg.Video
	// change is a setting line: pressing moves it on and saves.
	change := func(text string, next func()) menuLine {
		return setting(text, func() {
			next()
			if err := config.SaveValues(cfg.Path, "Video", [][2]string{
				{"Playback", v.Playback}, {"Playlist", v.Playlist}, {"AttractEvery", fmt.Sprint(v.AttractEvery)},
				{"AutoSync", strconv.FormatBool(v.AutoSync)}, {"CorrectPts", strconv.FormatBool(v.CorrectPts)},
				{"Mp3Seek", strconv.FormatBool(v.Mp3Seek)}, {"AviIndex", strconv.FormatBool(v.AviIndex)},
			}); err != nil {
				message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			}
		})
	}
	// sync is one of the sync settings, mostly for sound drifting after
	// seeking.
	sync := func(name string, on *bool) menuLine {
		return change(fmt.Sprintf("%-32s %s", name, onOffText(*on)), func() { *on = !*on })
	}
	(&menuScreen{title: "Video Player", lines: func() []menuLine {
		return []menuLine{
			opens("Play videos...", func() { videoBrowser(stdscr, video.Folder) }),
			action("Play", fmt.Sprintf("Play playlist (%s, %s)", videoFolderName(v.Playlist), strings.ToLower(v.Playback)), func() {
				files := video.Videos(v.Playlist)
				if len(files) == 0 {
					message(stdscr, "No videos found in "+filepath.Join(video.Folder, v.Playlist))
					return
				}
				playVideos(stdscr, files, v.Playback == "Random")
			}),
			change(fmt.Sprintf("%-22s %s", "Playback:", v.Playback), func() {
				v.Playback = nextOf([]string{"Random", "In order"}, v.Playback)
			}),
			change(fmt.Sprintf("%-22s %s", "Playlist:", videoFolderName(v.Playlist)), func() {
				v.Playlist = nextOf(video.Playlists(), v.Playlist)
			}),
			change(fmt.Sprintf("%-22s %s", "In attract mode:", videoAttractText(v.AttractEvery)), func() {
				v.AttractEvery = nextOf(videoAttractChoices, v.AttractEvery)
			}),
			sync("Fast A/V resync (all files):", &v.AutoSync),
			sync("Timestamps (MP4, MKV):", &v.CorrectPts),
			sync("Accurate seek (MP3 audio):", &v.Mp3Seek),
			sync("Build index (AVI, no index):", &v.AviIndex),
		}
	}}).run(stdscr)
}

// videoBrowser lists a folder: "[Play all]" (when it has videos), its
// subfolders, then its videos. Picking a video plays just that one.
func videoBrowser(stdscr *gc.Window, dir string) {
	if _, err := os.Stat(dir); err != nil {
		message(stdscr, "No video folder yet: put videos in "+video.Folder+"\n(folders inside it are playlists).")
		return
	}
	selected := 0
	for {
		var folders []string
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() {
				folders = append(folders, e.Name())
			}
		}
		sort.Slice(folders, func(i, j int) bool { return utils.LessFold(folders[i], folders[j]) })
		files := video.InFolder(dir)

		var items []string
		playAll := len(files) > 1
		if playAll {
			items = append(items, "[Play all]")
		}
		for _, f := range folders {
			items = append(items, f+"/")
		}
		for _, f := range files {
			items = append(items, filepath.Base(f))
		}
		if len(items) == 0 {
			message(stdscr, "No videos here.")
			return
		}

		title := "Videos"
		if rel, err := filepath.Rel(video.Folder, dir); err == nil && rel != "." {
			title = rel
		}
		first := 0
		if playAll {
			first = 1
		}
		clearScreen(stdscr)
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         title,
			Buttons:       []string{"PgUp", "PgDn", "", "Back"},
			ActionButton:  2,
			DefaultButton: 2,
			SnapToAction:  true,
			ShowTotal:     true,
			Width:         systemListWidth,
			Height:        listHeight,
			InitialIndex:  selected,
			DynamicActionLabel: func(i int) string {
				if i >= first && i < first+len(folders) {
					return "Open"
				}
				return "Play"
			},
		}, items)
		if err != nil || button != 2 || sel < 0 {
			return
		}
		selected = sel
		switch {
		case playAll && sel == 0:
			playVideos(stdscr, files, false)
		case sel < first+len(folders):
			videoBrowser(stdscr, filepath.Join(dir, folders[sel-first]))
		default:
			playVideos(stdscr, []string{files[sel-first-len(folders)]}, false)
		}
	}
}

// playVideos hands the screen to the video player, then brings the menu
// back at its own text size.
func playVideos(stdscr *gc.Window, files []string, shuffle bool) {
	video.Stop() // a video started over SSH
	gc.End()
	err := video.PlayOnScreen(files, shuffle)

	// The video player changed the screen size: set the menu's again.
	textSize.current = 0
	stdscr.Refresh()
	applyTextSizeLive(stdscr)
	if err != nil {
		message(stdscr, fmt.Sprintf("Couldn't play: %v", err))
	}
}
