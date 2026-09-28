package mister

import (
	"fmt"
	"os"
	"path/filepath"
	s "strings"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
)

func GenerateMgl(cfg *config.Config, system *games.System, path string, override string) (string, error) {
	// [Systems] set_core: the user's core for this system, unless this
	// launch already chose one (-core), which wins for that launch.
	if core, ok := SetCoreFor(cfg, system.Id); ok && system.Rbf == DefaultCore(system.Id) {
		system.Rbf = core
	}

	mgl := fmt.Sprintf("<mistergamedescription>\n\t<rbf>%s</rbf>\n", system.Rbf)

	if system.SetName != "" {
		sameDir := ""
		if system.SetNameSameDir {
			sameDir = " same_dir=\"1\""
		}

		mgl += fmt.Sprintf("\t<setname%s>%s</setname>\n", sameDir, system.SetName)
	}

	if path == "" {
		mgl += "</mistergamedescription>"
		return mgl, nil
	} else if override != "" {
		mgl += override
		mgl += "</mistergamedescription>"
		return mgl, nil
	}

	mglDef, err := games.PathToMglDef(*system, path)
	if err != nil {
		return "", err
	}

	mgl += fmt.Sprintf("<file delay=\"%d\" type=\"%s\" index=\"%d\" path=\"../../../../..%s\"/>\n", mglDef.Delay, mglDef.Method, mglDef.Index, path)
	mgl += "</mistergamedescription>"
	return mgl, nil
}

func writeTempFile(content string) (string, error) {
	if err := os.WriteFile(config.LastLaunchFile, []byte(content), 0644); err != nil {
		return "", err
	}
	return config.LastLaunchFile, nil
}

func launchFile(path string) error {
	_, err := os.Stat(config.CmdInterface)
	if err != nil {
		return fmt.Errorf("command interface not accessible: %s", err)
	}

	if !(s.HasSuffix(s.ToLower(path), ".mgl") || s.HasSuffix(s.ToLower(path), ".mra") || s.HasSuffix(s.ToLower(path), ".rbf")) {
		return fmt.Errorf("not a valid launch file: %s", path)
	}

	cmd, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer cmd.Close()

	// A failed write means the core didn't load: report it, so it's not
	// counted as a launch.
	_, err = fmt.Fprintf(cmd, "load_core %s\n", path)
	return err
}

func launchTempMgl(cfg *config.Config, system *games.System, path string) error {
	override, err := games.RunSystemHook(cfg, *system, path)
	if err != nil {
		return err
	}

	mgl, err := GenerateMgl(cfg, system, path, override)
	if err != nil {
		return err
	}

	tmpFile, err := writeTempFile(mgl)
	if err != nil {
		return err
	}

	return launchFile(tmpFile)
}

func LaunchGame(cfg *config.Config, system games.System, path string) error {
	// [BiosSkip]: create the virtual pad before the game loads.
	startAutoInput(cfg, system)

	// check if sidelaunchers wants to handle this system specially
	if handled, err := SideLaunchers(cfg, system, path); handled {
		return err
	}

	switch s.ToLower(filepath.Ext(path)) {
	case ".mra":
		// Arcade launchers are already valid files
		err := launchFile(path)
		if err != nil {
			return err
		}
	case ".mgl":
		// Pre-made .mgl file → just launch
		err := launchFile(path)
		if err != nil {
			return err
		}
		if ActiveGameEnabled() {
			SetActiveGame(path)
		}
	default:
		// Generic game file → build temporary MGL and launch
		err := launchTempMgl(cfg, &system, path)
		if err != nil {
			return err
		}
		if ActiveGameEnabled() {
			SetActiveGame(path)
		}
	}

	return nil
}

func LaunchMenu() error {
	if _, err := os.Stat(config.CmdInterface); err != nil {
		return fmt.Errorf("command interface not accessible: %s", err)
	}

	cmd, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer cmd.Close()

	// TODO: don't hardcode here
	_, err = fmt.Fprintf(cmd, "load_core %s\n", filepath.Join(config.SdFolder, "menu.rbf"))
	return err
}

// LaunchGenericFile Given a generic file path, launch it using the correct method, if possible.
func LaunchGenericFile(cfg *config.Config, path string) error {

	system, _ := games.BestSystemMatch(cfg, path)

	// check if sidelaunchers wants to handle this system specially
	if system.Id != "" {
		if handled, err := SideLaunchers(cfg, system, path); handled {
			return err
		}
	}

	var err error
	isGame := false
	ext := s.ToLower(filepath.Ext(path))

	switch ext {
	case ".mra":
		err = launchFile(path)
	case ".mgl":
		err = launchFile(path)
		isGame = true
	case ".rbf":
		err = launchFile(path)
	default:
		if system.Id == "" {
			return fmt.Errorf("unknown file type: %s", ext)
		}
		err = launchTempMgl(cfg, &system, path)
		isGame = true
	}
	if err != nil {
		return err
	}

	// Track active game if applicable
	if ActiveGameEnabled() && isGame {
		if err := SetActiveGame(path); err != nil {
			return err
		}
	}

	return nil
}

// SetMute mutes or unmutes MiSTer's sound (its "volume mute" command).
// A mute from attract mode is noted in muteMarker, so it can be undone
// even if attract mode is stopped without cleaning up, or the MiSTer
// restarts: MiSTer saves the mute to the SD card (config/Volume.dat), so
// the marker is kept there too, not in /tmp.
func SetMute(on bool) error {
	f, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd := "volume unmute\n"
	if on {
		cmd = "volume mute\n"
	}
	_, err = f.WriteString(cmd)
	if on {
		_ = os.MkdirAll(filepath.Dir(muteMarker), 0755)
		_ = os.WriteFile(muteMarker, nil, 0644)
	} else if _, statErr := os.Stat(muteMarker); statErr == nil {
		_ = os.Remove(muteMarker)
	}
	return err
}

// muteMarker: attract mode muted the sound (only written when it does).
const muteMarker = config.SAMFolder + "/attract_muted"

// IsMuted reports whether MiSTer's sound is muted, from the volume it
// saves (config/Volume.dat: one byte, 0x10 = muted).
func IsMuted() bool {
	b, err := os.ReadFile(filepath.Join(config.CoreConfigFolder, "Volume.dat"))
	return err == nil && len(b) > 0 && b[0]&0x10 != 0
}

// UndoStaleMute unmutes if attract mode muted the sound and is no longer
// running to unmute it (e.g. it was killed).
func UndoStaleMute(attractRunning bool) {
	if _, err := os.Stat(muteMarker); err == nil && !attractRunning {
		_ = SetMute(false)
	}
}

// Screenshot asks MiSTer to take a screenshot (its "screenshot" command);
// it goes in MiSTer's usual screenshots folder.
func Screenshot() error {
	f, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("screenshot\n")
	return err
}

// SendMute mutes or unmutes MiSTer's sound for you (a Mute binding), not
// for attract mode: no marker, so nothing undoes it later. Unmuting also
// clears attract mode's own marker.
func SendMute(on bool) error {
	if !on {
		return SetMute(false)
	}
	f, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("volume mute\n")
	return err
}
