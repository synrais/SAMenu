package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	gc "github.com/rthornton128/goncurses"
	"golang.org/x/sys/unix"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Logs (Options -> Logs)
// -------------------------
//
// The logs worth looking at when something didn't go as expected, in two
// groups: SAMenu's own (attract mode, opening SAMenu on the TV...) and
// the MiSTer's (Linux's kernel messages, MiSTer's program after a
// restart, the updaters' logs). Only the logs that exist are listed. A log
// opens at its end, the newest lines, and only its last part is read, so a
// big one can't fill the memory.

// logTail is how much of a log is read: its last part.
const logTail = 256 * 1024

// logSource is one log in the list.
type logSource struct {
	name string
	path string                 // its file ("" for kernel messages)
	read func() ([]byte, error) // how to read it, for one that isn't a file
}

// samenuLogs are SAMenu's own logs, in /tmp (in RAM: gone at reboot).
func samenuLogs() []logSource {
	return []logSource{
		{name: "Attract mode", path: attractLog},
		{name: "Idle watcher", path: idleLog},
		{name: "Startup", path: bootLog},
		{name: "BIOS skip", path: biosSkipLog},
		{name: "Back to Menu", path: backToMenuLog},
		{name: "Music player", path: musicLog},
		{name: "Video player", path: videoLog},
		{name: "Opening SAMenu on the TV", path: mister.OpenMenuLog},
	}
}

// misterConfigLogs is where the MiSTer's updaters (downloader, update_all
// and others) keep their logs: every .log file under it is listed.
const misterConfigLogs = config.ScriptsFolder + "/.config"

// misterLogs are the MiSTer's own logs.
func misterLogs() []logSource {
	logs := []logSource{
		{name: "Linux kernel messages", read: kernelMessages},
		{name: "MiSTer program (after a restart)", path: mister.MainLog},
		{name: "System log", path: "/var/log/messages"},
	}
	// The updaters' logs, found rather than named: they keep them in their
	// own folders, e.g. .config/downloader/downloader.log.
	var found []string
	_ = filepath.WalkDir(misterConfigLogs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.Count(strings.TrimPrefix(p, misterConfigLogs), "/") > 2 {
			return filepath.SkipDir // not deep in anyone's files
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".log") {
			found = append(found, p)
		}
		return nil
	})
	sort.Strings(found)
	for _, p := range found {
		logs = append(logs, logSource{name: strings.TrimPrefix(p, misterConfigLogs+"/"), path: p})
	}
	return logs
}

// kernelMessages reads Linux's kernel message buffer (what dmesg shows):
// devices plugged in, SD card and USB errors.
func kernelMessages() ([]byte, error) {
	size, err := unix.Klogctl(unix.SYSLOG_ACTION_SIZE_BUFFER, nil)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, size)
	n, err := unix.Klogctl(unix.SYSLOG_ACTION_READ_ALL, buf)
	if err != nil {
		return nil, err
	}
	// Each line starts with its level, "<6>": not worth showing.
	var out bytes.Buffer
	for _, line := range bytes.Split(buf[:n], []byte("\n")) {
		if len(line) > 2 && line[0] == '<' {
			if i := bytes.IndexByte(line, '>'); i > 0 && i < 5 {
				line = line[i+1:]
			}
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

// about describes a log for the list, e.g. "(12 min ago, 48 KB)", and
// reports whether it's there to show.
func (l logSource) about() (string, bool) {
	if l.read != nil {
		data, err := l.read()
		if err != nil || len(data) == 0 {
			return "", false
		}
		return fmt.Sprintf("(%s)", sizeText(int64(len(data)))), true
	}
	info, err := os.Stat(l.path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return "", false
	}
	return fmt.Sprintf("(%s, %s)", ago(time.Since(info.ModTime())), sizeText(info.Size())), true
}

// load reads a log's last part, as lines.
func (l logSource) load() ([]string, error) {
	var data []byte
	cut := false
	if l.read != nil {
		d, err := l.read()
		if err != nil {
			return nil, err
		}
		data = d
		if len(data) > logTail {
			data, cut = data[len(data)-logTail:], true
		}
	} else {
		d, c, err := readTail(l.path, logTail)
		if err != nil {
			return nil, err
		}
		data, cut = d, c
	}
	return logLines(data, cut), nil
}

// readTail reads the last max bytes of a file, reporting whether it had
// more.
func readTail(path string, max int64) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	start := info.Size() - max
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, max))
	return data, start > 0, err
}

// logLines splits a log into lines fit for the screen: tabs as spaces,
// other control characters (colours...) and broken characters dropped. A
// log that was cut starts at its first whole line, after a note saying so.
func logLines(data []byte, cut bool) []string {
	if cut {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	text := strings.TrimRight(string(data), "\n")
	var lines []string
	if cut {
		lines = append(lines, fmt.Sprintf("(the last %s of this log)", sizeText(logTail)))
	}
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, cleanLine(line))
	}
	return lines
}

// cleanLine makes a log line safe to draw.
func cleanLine(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\x1b': // a colour code: skip it to its end
			j := i + 1
			if j < len(line) && line[j] == '[' {
				for j++; j < len(line) && (line[j] < '@' || line[j] > '~'); j++ {
				}
			}
			i = j + 1
			continue
		case r == utf8.RuneError && size <= 1, r < ' ', r == 0x7f:
		default:
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// ago says how long ago something was: "just now", "12 min ago"...
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

// sizeText writes a size as bytes, KB or MB.
func sizeText(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%d KB", n/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

// logsScreen is Options -> Logs: the logs there are, in two groups.
func logsScreen(stdscr *gc.Window) {
	section := func(title string, logs []logSource) []menuLine {
		lines := []menuLine{heading(title + " logs")}
		shown := 0
		for _, l := range logs {
			l := l
			about, ok := l.about()
			if !ok {
				continue
			}
			lines = append(lines, opens(settingIndented(l.name, about), func() { viewLog(stdscr, l) }))
			shown++
		}
		if shown == 0 {
			lines = append(lines, info("  None yet"))
		}
		return lines
	}
	(&menuScreen{title: "Logs", lines: func() []menuLine {
		return append(section("SAMenu", samenuLogs()), section("MiSTer", misterLogs())...)
	}}).run(stdscr)
}

// viewLog shows a log, from its end, until Back. Refresh reads it again
// (for one that's still being written).
func viewLog(stdscr *gc.Window, l logSource) {
	const refresh, back = 2, 3
	for {
		lines, err := l.load()
		if err != nil {
			message(stdscr, fmt.Sprintf("Couldn't read it: %v", err))
			return
		}
		if len(lines) == 0 {
			lines = []string{"(empty)"}
		}
		clearScreen(stdscr)
		button, _, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Shortcuts:     menuShortcuts(),
			Title:         l.name,
			Buttons:       []string{"PgUp", "PgDn", "Refresh", "Back"},
			ActionButton:  refresh,
			DefaultButton: refresh,
			ShowTotal:     true,
			Width:         systemListWidth,
			Height:        listHeight,
			InitialIndex:  len(lines) - 1, // the newest line
		}, lines)
		clearScreen(stdscr)
		if err != nil || button == back || button < 0 {
			return
		}
	}
}
