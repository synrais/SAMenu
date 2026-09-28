package mister

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
)

// Core choice
//
// [Systems] set_core = SystemID:core picks another core for a system (Options
// -> Game Database -> Cores in the menu, or -core for one launch). A core is
// written the way MGL files name them: a path under /media/fat without
// ".rbf" or the build date, e.g. "_Unstable/SNES".

// CoreName turns any way of writing a core into the form MGL files use: a
// path from the SD card's root without ".rbf". A build date is kept, so
// that exact build loads; without one, MiSTer loads the newest build of
// that name in the folder. Cores on other drives get a path from the SD
// card, e.g. "/media/usb0/_Cores/X.rbf" -> "../usb0/_Cores/X".
func CoreName(core string) string {
	c := strings.TrimSpace(strings.ReplaceAll(core, "\\", "/"))
	if strings.EqualFold(filepath.Ext(c), ".rbf") {
		c = c[:len(c)-len(filepath.Ext(c))]
	}
	if filepath.IsAbs(c) {
		if rel, err := filepath.Rel(config.SdFolder, c); err == nil {
			c = rel
		}
	}
	return filepath.ToSlash(c)
}

// SetCoreFor is the core [Systems] set_core picks for a system, if any.
func SetCoreFor(cfg *config.Config, systemID string) (string, bool) {
	for _, line := range cfg.Systems.SetCore {
		id, core, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(id), systemID) && strings.TrimSpace(core) != "" {
			return CoreName(core), true
		}
	}
	return "", false
}

// DefaultCore is a system's own core from the systems list.
func DefaultCore(systemID string) string {
	if s, err := games.GetSystem(systemID); err == nil {
		return s.Rbf
	}
	return ""
}

// CoreFile is a core file found on a drive.
type CoreFile struct {
	Drive string // "FAT", "USB0"...
	Dir   string // its folder from the drive's root ("" for the root)
	Name  string // file name, e.g. "SNES_20260901.rbf"
	Core  string // what set_core saves, e.g. "_Unstable/SNES_20260901"
}

// ScanCores lists the core files on every drive (/media/fat, /media/usb0...):
// those in a drive's root, and in its folders whose names start with "_"
// (MiSTer only shows those), at any depth, except _Arcade.
func ScanCores() []CoreFile {
	var out []CoreFile
	drives, _ := os.ReadDir("/media")
	for _, d := range drives {
		if !d.IsDir() {
			continue
		}
		root := filepath.Join("/media", d.Name())
		drive := strings.ToUpper(d.Name())
		var walk func(dir, rel string)
		walk = func(dir, rel string) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				return
			}
			for _, e := range entries {
				name := e.Name()
				full := filepath.Join(dir, name)
				isDir := e.IsDir()
				if e.Type()&os.ModeSymlink != 0 {
					if st, err := os.Stat(full); err == nil {
						isDir = st.IsDir()
					}
				}
				if isDir {
					if strings.HasPrefix(name, "_") && !(rel == "" && strings.EqualFold(name, "_Arcade")) {
						walk(full, filepath.Join(rel, name))
					}
					continue
				}
				if strings.EqualFold(filepath.Ext(name), ".rbf") {
					out = append(out, CoreFile{Drive: drive, Dir: filepath.ToSlash(rel), Name: name, Core: CoreName(full)})
				}
			}
		}
		walk(root, "")
	}
	return out
}
