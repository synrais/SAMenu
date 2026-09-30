package mister

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input/virtualinput"
)

// Picture settings: trying MiSTer.ini changes
//
// Options -> Screen -> Picture changes MiSTer's own settings file. A
// change is always tried first: the new file goes in a copy in RAM,
// mounted over the real one (a bind mount), and the menu core is loaded
// again so MiSTer reads it. If the picture's gone, nothing needs undoing
// on the SD card: dropping the mount (or a reboot) brings the old file
// back. Kept, the copy stays mounted until reboot, or is written into
// the real file for good (the file as it was first kept as a backup).
//
// MiSTer can have up to three other settings files, MiSTer_<name>.ini
// (Alt1 to Alt3), picked in its own menu, which keeps that choice where
// SAMenu can't read it. So SAMenu watches which one MiSTer opens each
// time it loads the menu core, and remembers it (ActiveIni).

const (
	tempIniDir    = "/tmp/SAMenu_ini" // the copies in RAM
	activeIniFile = tempIniDir + "/active"
)

// IniFiles lists MiSTer's settings files that are there: MiSTer.ini,
// then the MiSTer_<name>.ini ones, A-Z.
func IniFiles() []string {
	files := []string{"MiSTer.ini"}
	ents, _ := os.ReadDir(config.SdFolder)
	var alts []string
	for _, e := range ents {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, "MiSTer_") && strings.EqualFold(filepath.Ext(n), ".ini") {
			alts = append(alts, n)
		}
	}
	sort.Strings(alts)
	return append(files, alts...)
}

// ActiveIni is the settings file MiSTer read when SAMenu last saw it load
// the menu core. Not seen yet: MiSTer.ini, and known is false (unless
// it's the only one, so it can't be another).
func ActiveIni() (name string, known bool) {
	if b, err := os.ReadFile(activeIniFile); err == nil {
		if n := strings.TrimSpace(string(b)); n != "" {
			return n, true
		}
	}
	return "MiSTer.ini", len(IniFiles()) == 1
}

func iniPath(name string) string { return filepath.Join(config.SdFolder, name) }

// IniTemporary reports whether a settings file has SAMenu's copy in RAM
// mounted over it (until reboot).
func IniTemporary(name string) bool {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	defer f.Close()
	want := iniPath(name)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if fields := strings.Fields(sc.Text()); len(fields) > 4 && fields[4] == want {
			return true
		}
	}
	return false
}

// ReadIni reads a settings file as MiSTer sees it (the copy, if mounted).
func ReadIni(name string) (string, error) {
	b, err := os.ReadFile(iniPath(name))
	return string(b), err
}

// SetIniTemporary puts text in use for a settings file until reboot: in
// the copy in RAM, mounted over the file. The file itself isn't touched.
func SetIniTemporary(name, text string) error {
	if err := os.MkdirAll(tempIniDir, 0755); err != nil {
		return err
	}
	tmp := filepath.Join(tempIniDir, name)
	// Written in place, never replaced: the mount holds on to the file.
	if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
		return err
	}
	if IniTemporary(name) {
		return nil
	}
	if err := unix.Mount(tmp, iniPath(name), "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("couldn't put the new settings in use: %w", err)
	}
	return nil
}

// DropIniTemporary takes SAMenu's copy off a settings file, so the file
// on the SD card is in use again.
func DropIniTemporary(name string) error {
	if IniTemporary(name) {
		if err := unix.Unmount(iniPath(name), 0); err != nil {
			return fmt.Errorf("couldn't undo the temporary settings: %w", err)
		}
	}
	_ = os.Remove(filepath.Join(tempIniDir, name))
	return nil
}

// IniBackup is where a settings file is kept, as it was before SAMenu
// first changed it for good.
func IniBackup(name string) string { return filepath.Join(config.SAMFolder, name+".bak") }

// SaveIni writes text into a settings file for good, dropping SAMenu's
// copy first. The first time, the file as it was is kept (IniBackup).
func SaveIni(name, text string) error {
	if err := DropIniTemporary(name); err != nil {
		return err
	}
	path := iniPath(name)
	if _, err := os.Stat(IniBackup(name)); os.IsNotExist(err) {
		if err := copyFile(path, IniBackup(name)); err != nil {
			return fmt.Errorf("couldn't keep a backup: %w", err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0755); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	unix.Sync()
	return nil
}

// RestoreIni puts a settings file back as it was before SAMenu first
// changed it for good, and drops any temporary copy.
func RestoreIni(name string) error {
	if err := DropIniTemporary(name); err != nil {
		return err
	}
	if _, err := os.Stat(IniBackup(name)); err != nil {
		return fmt.Errorf("no backup of %s (SAMenu hasn't changed it for good)", name)
	}
	if err := copyFile(IniBackup(name), iniPath(name)); err != nil {
		return err
	}
	unix.Sync()
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ReloadMenuHere loads the MiSTer menu core again, so MiSTer reads its
// settings, and brings back the Linux console the caller is on (tty,
// e.g. "tty2") with F9. It reports the settings file MiSTer read, and
// remembers it for ActiveIni.
func ReloadMenuHere(tty string) (read string, err error) {
	// Made first, so MiSTer finds it as it starts.
	kbd, err := virtualinput.NewKeyboard(40 * time.Millisecond)
	if err != nil {
		return "", fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	defer kbd.Close()

	read, err = reloadMenuWatching()
	if err != nil {
		return "", err
	}
	if err := showConsole(kbd); err != nil {
		return read, err
	}
	if n := strings.TrimPrefix(tty, "tty"); n != "" && n != "1" {
		if out, err := exec.Command("chvt", n).CombinedOutput(); err != nil {
			return read, fmt.Errorf("chvt %s: %v %s", n, err, out)
		}
	}
	return read, nil
}

// reloadMenuWatching loads the menu core (again, even if it's loaded),
// watching which settings file MiSTer opens as it starts, and waits for
// it to be up. The command is sent again if MiSTer misses it.
func reloadMenuWatching() (string, error) {
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)
	_ = os.MkdirAll(tempIniDir, 0755)
	// A mounted copy is opened through its own folder, in RAM.
	for _, dir := range []string{config.SdFolder, tempIniDir} {
		_, _ = unix.InotifyAddWatch(fd, dir, unix.IN_OPEN)
	}

	read := ""
	buf := make([]byte, 16384)
	deadline := time.Now().Add(20 * time.Second)
	var sent time.Time
	for time.Now().Before(deadline) {
		if read == "" && time.Since(sent) >= 6*time.Second {
			if err := LaunchMenu(); err != nil {
				return "", err
			}
			sent = time.Now()
		}
		n, _ := unix.Read(fd, buf)
		for off := 0; off+unix.SizeofInotifyEvent <= n; {
			ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[off]))
			name := strings.TrimRight(string(buf[off+unix.SizeofInotifyEvent:off+unix.SizeofInotifyEvent+int(ev.Len)]), "\x00")
			if read == "" && strings.HasPrefix(name, "MiSTer") && strings.EqualFold(filepath.Ext(name), ".ini") {
				read = name
			}
			off += unix.SizeofInotifyEvent + int(ev.Len)
		}
		if read != "" && IsMenuRunning() && MainRunning() {
			_ = os.WriteFile(activeIniFile, []byte(read), 0644)
			return read, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if read == "" {
		return "", fmt.Errorf("the MiSTer menu didn't load")
	}
	return read, fmt.Errorf("the MiSTer menu didn't come back")
}
