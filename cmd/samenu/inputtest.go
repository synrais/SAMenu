package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Input test
// -------------------------
//
// Options -> Controls -> Input test: a live view of what the input
// detectors see, the same way attract mode does. The devices being
// watched, bars showing the last-used controller's sticks (with the line a
// stick must pass to count), and every input as it happens, with the
// attract action it would trigger, or dimmed with why it didn't count (the
// stick hold, Sticks = false). Holding a controller's Back (Select) or a
// keyboard's Esc for two seconds leaves; tapping it just shows it, so any
// button can be tried.

const (
	exitHold    = 2 * time.Second // hold Back or Esc this long to leave
	releaseWait = 5 * time.Second // then wait at most this long for it to be let go
)

// testLine is one input, kept in parts so it can be laid out to fit the
// screen when drawn.
type testLine struct {
	at     time.Time
	port   string // js0, hidraw1: the Controllers list says which device
	name   string // as used in SAMenu.ini
	hint   string // "right stick up"
	result string // "attract: next", "ignored: too short", "-"
	dimmed bool
}

func inputTestScreen(stdscr *gc.Window, cfg *config.Config) {
	opened = time.Now()
	stickRules()
	input.SetDebug(true)
	defer input.SetDebug(false)
	menuInputs := takeInputs() // everything to this screen until it closes
	defer releaseInputs()
	// Keys are read without waiting (only what's already there), and the
	// screen is paced by a short sleep each frame: a held button's key
	// repeats come faster than any wait, and would otherwise keep the key
	// reading from ever finishing.
	stdscr.Timeout(0)
	defer func() {
		stdscr.Timeout(-1)
		_ = gc.FlushInput()
		clearScreen(stdscr)
	}()

	var lines []testLine
	lastPad := "" // the controller whose sticks are shown

	// Leaving: holding a controller's Back (Select) or a keyboard's Esc for
	// exitHold, as the detectors see them go down and up. Every other button
	// can be tried freely. Over SSH the keyboard isn't one the detectors
	// see, so there an Esc that keeps repeating for as long counts too (on
	// the TV it doesn't: MiSTer turns a controller's B into Esc).
	holdKey := "" // the button being held to leave
	var holdStart time.Time
	var repeatStart, repeatLast time.Time // Esc repeating in an SSH terminal
	sshEsc := !mister.OnConsole()
	// After the hold: wait for the button to be let go (or 5 seconds), so
	// its key repeats don't carry on into the next screen.
	releasing := false
	var releaseStart time.Time

	for {
		time.Sleep(50 * time.Millisecond)
		now := time.Now()
		// Menu keys (at most a screenful's worth each frame): only read, so
		// they don't pile up, apart from an SSH terminal's Esc.
		for n := 0; n < 64; n++ {
			k := stdscr.GetChar()
			if k == 0 || k == gc.KEY_RESIZE {
				break
			}
			if k == 27 {
				// Arrow keys (MiSTer sends them for sticks and the d-pad)
				// arrive as Esc followed by "[" or "O": not an Esc.
				next := stdscr.GetChar()
				if next == '[' || next == 'O' {
					for c := stdscr.GetChar(); c != 0 && !(c >= 'A' && c <= 'Z' || c == '~'); c = stdscr.GetChar() {
					}
					continue
				}
				if sshEsc {
					if now.Sub(repeatLast) > 300*time.Millisecond {
						repeatStart = now
					}
					repeatLast = now
				}
			}
		}
		// Detector events.
	drain:
		for {
			select {
			case ev := <-menuInputs:
				key := ev.Path + "|" + ev.Name
				if ev.Up {
					if key == holdKey {
						holdKey = ""
					}
					continue
				}
				if ev.Kind == "mouse" && strings.HasPrefix(ev.Name, "swipe") {
					continue // mouse movement would flood the list
				}
				if ev.Kind == "joystick" && ev.Path != "" {
					lastPad = ev.Path
				}
				// A controller's Back or a keyboard's Esc starts the hold.
				exitButton := (ev.Kind == "joystick" && ev.Name == "back") || (ev.Kind == "keyboard" && ev.Name == "esc")
				if exitButton && ev.Ignored == "" && holdKey == "" && !releasing {
					holdKey, holdStart = key, now
				}
				lines = append([]testLine{describeInput(cfg, ev, now)}, lines...)
				if len(lines) > 200 {
					lines = lines[:200]
				}
			default:
				break drain
			}
		}
		// Leave after a two-second hold (or a key repeating for as long).
		held := time.Duration(0)
		if holdKey != "" {
			held = now.Sub(holdStart)
		}
		if r := repeatLast.Sub(repeatStart); now.Sub(repeatLast) < 300*time.Millisecond && r > held {
			held = r
		}
		if !releasing && held >= exitHold {
			releasing, releaseStart = true, now
		}
		if releasing {
			// Let go: the held button's release came in, or (a key that
			// just repeats) its repeats have stopped. Or 5 seconds are up.
			letGo := holdKey == "" && now.Sub(repeatLast) > 300*time.Millisecond
			if letGo || now.Sub(releaseStart) >= releaseWait {
				flushKeys(stdscr)
				return
			}
		}
		drawInputTest(stdscr, lines, lastPad, held, releasing, releaseWait-now.Sub(releaseStart))
	}
}

// describeInput is one line of the input list.
func describeInput(cfg *config.Config, ev input.Event, at time.Time) testLine {
	result := "-"
	if ev.Ignored != "" {
		result = "ignored: " + ev.Ignored
	} else if a, ok := cfg.AttractControls[ev.Kind][ev.Name]; ok && a != "" {
		result = "attract: " + a
	}
	port := filepath.Base(ev.Path)
	if ev.Path == "" {
		port = ev.Kind
	}
	return testLine{at: at, port: port, name: ev.Name, hint: ev.Hint, result: result, dimmed: ev.Ignored != ""}
}

// resultWidth fits the longest result ("ignored: sticks off"), so the
// results line up in a column.
const resultWidth = 19

// layoutInput fits a line to the screen width. Whether the time and the
// plain-English hint show is decided by the width alone, so every line on
// the screen looks the same; the input's name and the result always show.
func layoutInput(l testLine, width int, t time.Duration) string {
	showTime := width >= 70
	stamp := ""
	if showTime {
		stamp = l.at.Format("04:05.0") + "  "
	}
	port := fmt.Sprintf("%-8s ", fitText(l.port, 8))
	room := width - 2 - len(stamp) - len(port) - 1 - resultWidth
	if room < 8 {
		room = 8
	}
	what := l.name
	if l.hint != "" && room >= 24 {
		what += " (" + l.hint + ")"
	}
	return fmt.Sprintf("  %s%s%s %s", stamp, port, scrollFit(what, room, t), l.result)
}

// deviceLine fits a Controllers line: port, name, then how it's named,
// sharing the width between the name and the note.
func deviceLine(d input.DeviceInfo, width int, t time.Duration) string {
	info := d.Info
	switch {
	case strings.HasPrefix(info, "named: "):
		info = "SDL: " + strings.TrimPrefix(info, "named: ")
		info = strings.Replace(info, "(exact match)", "(exact)", 1)
		info = strings.Replace(info, "(close match, ignoring bus type and version)", "(close)", 1)
		info = strings.Replace(info, "(close match, ignoring bus type)", "(close)", 1)
	case strings.HasPrefix(info, "raw names"):
		info = "raw names"
	case info == "":
		info = d.Kind
	}
	nameWidth := (width - 11) / 2
	if nameWidth > 26 {
		nameWidth = 26
	}
	infoWidth := width - 2 - 9 - nameWidth - 1
	if infoWidth < 4 {
		infoWidth = 4
	}
	return fmt.Sprintf("  %-8s %s %s", fitText(filepath.Base(d.Path), 8), scrollFit(d.Name, nameWidth, t), scrollFit(info, infoWidth, t))
}

func drawInputTest(stdscr *gc.Window, lines []testLine, pad string, held time.Duration, releasing bool, left time.Duration) {
	rows, cols := stdscr.MaxYX()
	stdscr.Erase()
	row := 0
	put := func(text string, attr gc.Char) {
		if row >= rows-1 {
			return
		}
		if attr != 0 {
			stdscr.AttrOn(attr)
		}
		stdscr.MovePrint(row, 0, fitText(text, cols-1))
		if attr != 0 {
			stdscr.AttrOff(attr)
		}
		row++
	}

	put("Input test", gc.A_BOLD)
	put("Controllers", gc.A_BOLD)
	devices := input.Devices()
	if len(devices) == 0 {
		put("  (none found)", gc.A_DIM)
	}
	for _, d := range devices {
		put(deviceLine(d, cols-1, time.Since(opened)), 0)
	}

	// The last-used controller's sticks (not its d-pad).
	var sticks []input.Stick
	for _, s := range input.Sticks(pad) {
		if !s.Hat {
			sticks = append(sticks, s)
		}
	}
	if len(sticks) > 0 {
		row++
		name := filepath.Base(pad)
		for _, d := range devices {
			if d.Path == pad {
				name = d.Name + " (" + name + ")"
			}
		}
		put("Sticks: "+name+"   (| = where a push counts)", gc.A_BOLD)
		width := cols - 24
		if width > 41 {
			width = 41
		}
		if width%2 == 0 {
			width--
		}
		for _, s := range sticks {
			if width < 11 {
				break
			}
			put(fmt.Sprintf("  %-8s [%s] %4d%%", fitText(s.Name, 8), stickBar(s.Value, width), int(s.Value)*100/32767), 0)
		}
	}

	row++
	put("Inputs (newest first)", gc.A_BOLD)
	if len(lines) == 0 {
		put("  Press anything...", gc.A_DIM)
	}
	for _, l := range lines {
		var attr gc.Char
		if l.dimmed {
			attr = gc.A_DIM
		}
		put(layoutInput(l, cols-1, time.Since(opened)), attr)
	}

	hint := "Hold Back/ESC for 2 seconds to exit"
	if releasing {
		secs := int((left + time.Second - 1) / time.Second)
		hint = fmt.Sprintf("Release to exit, else exiting in %d seconds...", secs)
	} else if held > 0 {
		hint = fmt.Sprintf("Exiting... %.1fs", (exitHold - held).Seconds())
	}
	stdscr.AttrOn(gc.A_REVERSE)
	stdscr.MovePrint(rows-1, 0, fitText(hint, cols-1))
	stdscr.AttrOff(gc.A_REVERSE)
	stdscr.Refresh()
}

// stickBar draws a stick's position: the middle is the centre, "#" fills
// towards where the stick is, and "|" marks the line a push must pass.
func stickBar(v int16, width int) string {
	half := width / 2
	b := []byte(strings.Repeat("-", width))
	b[half] = '+'
	line := input.StickLine() * half / 32767
	b[half-line], b[half+line] = '|', '|'
	pos := int(v) * half / 32767
	for i := 1; i <= abs(pos) && i <= half; i++ {
		if pos < 0 {
			b[half-i] = '#'
		} else {
			b[half+i] = '#'
		}
	}
	return string(b)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// flushKeys throws away every key waiting to be read (a held button's
// repeats), so none reach the next screen.
func flushKeys(stdscr *gc.Window) {
	stdscr.Timeout(0)
	for i := 0; i < 1000 && stdscr.GetChar() != 0; i++ {
	}
	_ = gc.FlushInput()
}

// opened is when the input test screen opened: long names scroll from then.
var opened = time.Now()

// scrollFit fits text to a width, padded, or, if it's too long, scrolls it
// the way the lists do: after a second on its start, one character every
// 200ms, looping with a three-space gap.
func scrollFit(text string, width int, t time.Duration) string {
	if width <= 0 {
		return ""
	}
	// Counted in characters, so a letter of two or more bytes (é) takes one
	// column.
	r := []rune(text)
	if len(r) <= width {
		return text + strings.Repeat(" ", width-len(r))
	}
	loop := append(r, []rune("   ")...)
	offset := 0
	if t > time.Second {
		offset = int((t-time.Second)/(200*time.Millisecond)) % len(loop)
	}
	return string(append(loop, loop...)[offset : offset+width])
}
