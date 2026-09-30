package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input/virtualinput"
)

func IsMenuRunning() bool {
	activeCore, err := GetActiveCoreName()
	if err != nil {
		return false
	}
	return activeCore == config.MenuCore
}

// OpenMenuLog records each step of opening SAMenu on the TV.
const OpenMenuLog = "/tmp/SAMenu_openmenu.log"

// stepLog prints a step and appends it to OpenMenuLog.
func stepLog(format string, args ...interface{}) {
	line := time.Now().Format("15:04:05.000") + " " + fmt.Sprintf(format, args...)
	fmt.Println(line)
	if f, err := os.OpenFile(OpenMenuLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		fmt.Fprintln(f, line)
		f.Close()
	}
}

func activeTty() string {
	b, err := os.ReadFile("/sys/devices/virtual/tty/tty0/active")
	if err != nil {
		return "unknown (" + err.Error() + ")"
	}
	return strings.TrimSpace(string(b))
}

// showConsole switches the MiSTer menu core's screen to the Linux console
// (as the Scripts menu does) by pressing F9, MiSTer's key for it, on a
// virtual keyboard. It keeps pressing until it shows, so it needs no fixed wait:
// the first presses can come before MiSTer has found the keyboard, or
// before its program is ready after loading the menu core.
func showConsole(kbd virtualinput.Keyboard) error {
	core, _ := GetActiveCoreName()
	stepLog("core loaded: %q, active console: %s", core, activeTty())
	if !IsMenuRunning() {
		return fmt.Errorf("the MiSTer menu core isn't loaded")
	}
	if out, err := exec.Command("chvt", "3").CombinedOutput(); err != nil {
		return fmt.Errorf("chvt 3: %v %s", err, out)
	}
	stepLog("switched to console 3, now: %s; pressing F9", activeTty())
	f9, ok := virtualinput.ToKeyboardCode("f9")
	if !ok {
		return fmt.Errorf("no F9 key in the virtual keyboard map")
	}
	for i := 0; i < 120; i++ { // up to about 6 seconds
		_ = kbd.Press(f9)
		time.Sleep(50 * time.Millisecond)
		if activeTty() == "tty1" {
			stepLog("F9 worked, console: %s", activeTty())
			return nil
		}
		if i%20 == 19 && !IsMenuRunning() {
			break // a game loaded after all (asked for just as one launched)
		}
	}
	return fmt.Errorf("couldn't show the console with F9 (console is %s, wanted tty1)", activeTty())
}

// runOnConsole runs a command on console 2 (the console F9 shows is 1),
// from folder dir, and waits for it to end. With pause, it then waits for
// a key, as MiSTer does after a script from its Scripts menu. It doesn't
// press F12 afterwards (SAMenu leaves the console itself).
func runOnConsole(name, dir string, command []string, pause bool) error {
	if out, err := exec.Command("chvt", "2").CombinedOutput(); err != nil {
		return fmt.Errorf("chvt 2: %v %s", err, out)
	}
	stepLog("switched to console 2 (%s), starting %s there", activeTty(), name)
	quoted := make([]string, 0, len(command))
	for _, a := range command {
		quoted = append(quoted, shellQuote(a))
	}
	launcher := "#!/bin/bash\nexport LC_ALL=en_US.UTF-8\nexport HOME=/root\nexport LESSKEY=/media/fat/linux/lesskey\ncd " +
		shellQuote(dir) + "\n" + strings.Join(quoted, " ") + "\n"
	if pause {
		launcher += "echo\nread -n 1 -s -r -p \"Press any key to continue\"\n"
	}
	if err := os.WriteFile("/tmp/script", []byte(launcher), 0755); err != nil {
		return err
	}
	out, err := exec.Command("/sbin/agetty", "-a", "root", "-l", "/tmp/script", "--nohostname", "-L", "tty2", "linux").CombinedOutput()
	stepLog("%s on console 2 ended: err=%v %s", name, err, strings.TrimSpace(string(out)))
	return err
}

// shellQuote quotes one word for bash.
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// OpenGamesMenu closes whatever game is running (loading the MiSTer menu
// core) and opens SAMenu on the TV, straight into its Search
// screen if search is true.
func OpenGamesMenu(search bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	command := []string{exe}
	if search {
		command = append(command, "-search")
	}
	return openOnScreen("SAMenu", filepath.Dir(exe), command, false)
}

// RunScriptOnScreen closes whatever game is running (loading the MiSTer
// menu core) and runs a script on the TV, the way the Scripts menu does:
// from its own folder, then "Press any key to continue", then back to the
// MiSTer menu.
func RunScriptOnScreen(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	err := openOnScreen(filepath.Base(path), filepath.Dir(path), []string{"/bin/bash", path}, true)
	if IsMenuRunning() {
		_ = LaunchMenu() // leave the console for the MiSTer menu
	}
	return err
}

// openOnScreen gets the MiSTer menu core's Linux console on the TV and
// runs a command there (see runOnConsole).
//
// There are no fixed waits: the virtual keyboard is made first, so MiSTer
// finds it while the menu core loads, and F9 is pressed until the console
// shows (about 3 seconds from a game to the menu, down from 6). Asked for
// just as a game launches, the game can still load over the menu core:
// so it tries up to three times.
func openOnScreen(name, dir string, command []string, pause bool) error {
	_ = os.Remove(OpenMenuLog)
	stepLog("opening %s on the TV", strings.Join(command, " "))
	kbd, err := virtualinput.NewKeyboard(40 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	defer kbd.Close()
	stepLog("created SAMenu's virtual keyboard")

	for try := 1; ; try++ {
		err = loadMenuCore()
		if err == nil {
			err = showConsole(kbd)
		}
		if err == nil {
			break
		}
		if try == 3 {
			return err
		}
		stepLog("%v: trying again", err)
	}
	return runOnConsole(name, dir, command, pause)
}

// loadMenuCore loads the MiSTer menu core, if it isn't loaded, and waits
// for it and MiSTer's program to be up. MiSTer can miss the command while
// it's still loading a game (mounting a CD image), so it's sent again
// every few seconds until the menu core is up.
func loadMenuCore() error {
	if IsMenuRunning() {
		return nil
	}
	deadline := time.Now().Add(20 * time.Second)
	var sent time.Time
	for !IsMenuRunning() || !MainRunning() {
		now := time.Now()
		if now.After(deadline) {
			return fmt.Errorf("the MiSTer menu didn't load")
		}
		if now.Sub(sent) >= 6*time.Second && !IsMenuRunning() {
			stepLog("loading the MiSTer menu core")
			if err := LaunchMenu(); err != nil {
				return err
			}
			sent = now
		}
		time.Sleep(100 * time.Millisecond)
	}
	stepLog("menu core loaded")
	return nil
}
