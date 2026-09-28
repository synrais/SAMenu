package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Cores
// -------------------------
//
// Options -> Game Database -> Cores: pick which core each system uses, from
// the core files on the SD card, saved as [Systems] set_core lines.

func coresScreen(stdscr *gc.Window, cfg *config.Config, sysNames []string) {
	// Not Arcade: its MRA files name their own cores.
	var ids []string
	for _, id := range systemIDs(sysNames) {
		if !strings.EqualFold(id, "Arcade") {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		message(stdscr, "No systems in the games database yet.")
		return
	}
	(&menuScreen{title: "Cores", lines: func() []menuLine {
		lines := make([]menuLine, len(ids))
		for i, id := range ids {
			id := id
			core := mister.DefaultCore(id) + " (default)"
			if c, ok := mister.SetCoreFor(cfg, id); ok {
				core = c
			}
			lines[i] = setting(fmt.Sprintf("%-22s %s", games.DisplayName(id)+":", core), func() { chooseCore(stdscr, cfg, id) })
		}
		return lines
	}}).run(stdscr)
}

// chooseCore browses the core files on every drive, like the games menu:
// folders (merged across drives) first, then files with their drive, and
// "Default" at the top. The chosen file is saved as the system's core.
func chooseCore(stdscr *gc.Window, cfg *config.Config, id string) {
	def := mister.DefaultCore(id)
	current, _ := mister.SetCoreFor(cfg, id)
	name := games.DisplayName(id)
	found := mister.ScanCores()
	if len(found) == 0 {
		message(stdscr, "No core files found on any drive.")
		return
	}
	dir := "" // the folder being shown, from the drives' roots
	selected := 0
	backTo := "" // after going back up, the folder just left
	for {
		// This folder's subfolders (merged across drives) and files.
		subs := map[string]bool{}
		var files []mister.CoreFile
		for _, f := range found {
			switch {
			case f.Dir == dir:
				files = append(files, f)
			case dir == "" || strings.HasPrefix(f.Dir, dir+"/"):
				rest := strings.TrimPrefix(strings.TrimPrefix(f.Dir, dir), "/")
				subs[strings.SplitN(rest, "/", 2)[0]] = true
			}
		}
		folders := make([]string, 0, len(subs))
		for f := range subs {
			folders = append(folders, f)
		}
		sort.Slice(folders, func(i, j int) bool { return strings.ToLower(folders[i]) < strings.ToLower(folders[j]) })
		sort.Slice(files, func(i, j int) bool {
			if !strings.EqualFold(files[i].Name, files[j].Name) {
				return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
			}
			return files[i].Drive < files[j].Drive
		})

		var items []string
		if dir == "" {
			d := "Default: " + def
			if current == "" {
				d += " *"
			}
			items = append(items, d)
		}
		for _, f := range folders {
			items = append(items, f+"/")
		}
		for _, f := range files {
			label := fmt.Sprintf("%s (%s)", f.Name, f.Drive)
			if strings.EqualFold(f.Core, current) {
				label += " *"
			}
			items = append(items, label)
		}
		if backTo != "" {
			for i, it := range items {
				if it == backTo+"/" {
					selected = i
				}
			}
			backTo = ""
		}
		title := "Core for " + name
		if dir != "" {
			title = dir
		}
		sel, ok := optionsList(stdscr, title, items, selected)
		if !ok {
			if dir == "" {
				return
			}
			// Back up a folder, onto the folder just left.
			backTo = filepath.Base(dir)
			dir = filepath.Dir(dir)
			if dir == "." {
				dir = ""
			}
			selected = 0
			continue
		}
		off := 0
		if dir == "" {
			off = 1
			if sel == 0 {
				saveCore(stdscr, cfg, id, "")
				return
			}
		}
		if i := sel - off; i < len(folders) {
			dir = strings.TrimPrefix(dir+"/"+folders[i], "/")
			selected = 0
			continue
		}
		saveCore(stdscr, cfg, id, files[sel-off-len(folders)].Core)
		return
	}
}

// saveCore saves a system's core ("" = its own) with the others.
func saveCore(stdscr *gc.Window, cfg *config.Config, id, core string) {
	cores := map[string]string{}
	for _, s := range games.Systems {
		if c, ok := mister.SetCoreFor(cfg, s.Id); ok {
			cores[s.Id] = c
		}
	}
	if core == "" {
		delete(cores, id)
	} else {
		cores[id] = core
	}
	if err := config.SaveSetCores(cfg, cores); err != nil {
		message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
	}
}
