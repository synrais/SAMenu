package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestReadTail(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(p, []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	data, cut, err := readTail(p, 1000)
	if err != nil || cut || string(data) != "one\ntwo\nthree\n" {
		t.Fatalf("whole file: %q %v %v", data, cut, err)
	}
	data, cut, _ = readTail(p, 8) // "o\nthree\n"
	if !cut || string(data) != "o\nthree\n" {
		t.Fatalf("tail: %q %v", data, cut)
	}
	// Cut, the partial first line goes and a note says why.
	got := logLines(data, cut)
	want := []string{"(the last 256 KB of this log)", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines: %q", got)
	}
	if _, _, err := readTail(filepath.Join(t.TempDir(), "none"), 10); err == nil {
		t.Error("missing file: no error")
	}
}

func TestCleanLine(t *testing.T) {
	for in, want := range map[string]string{
		"plain line":                   "plain line",
		"a\tb":                         "a    b",
		"\x1b[31mred\x1b[0m text":      "red text",
		"bell\x07 and cr\r":            "bell and cr",
		"bad \xff byte, good é":        "bad  byte, good é",
		"07:02:11.4  +2.3s  [Attract]": "07:02:11.4  +2.3s  [Attract]",
	} {
		if got := cleanLine(in); got != want {
			t.Errorf("cleanLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLogLabels(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Second: "just now", 12 * time.Minute: "12 min ago",
		5 * time.Hour: "5 h ago", 72 * time.Hour: "3 days ago",
	} {
		if got := ago(d); got != want {
			t.Errorf("ago(%v) = %q, want %q", d, got, want)
		}
	}
	for n, want := range map[int64]string{512: "512 bytes", 48 * 1024: "48 KB", 3 * 1024 * 1024: "3.0 MB"} {
		if got := sizeText(n); got != want {
			t.Errorf("sizeText(%d) = %q, want %q", n, got, want)
		}
	}
	// A log file that's missing or empty isn't listed.
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.log")
	_ = os.WriteFile(empty, nil, 0644)
	for _, p := range []string{empty, filepath.Join(dir, "missing.log")} {
		if _, ok := (logSource{name: "x", path: p}).about(); ok {
			t.Errorf("%s listed", p)
		}
	}
	full := filepath.Join(dir, "full.log")
	_ = os.WriteFile(full, []byte(strings.Repeat("x\n", 600)), 0644)
	if about, ok := (logSource{name: "x", path: full}).about(); !ok || about != "(just now, 1 KB)" {
		t.Errorf("about: %q %v", about, ok)
	}
}

func TestLogOutputTo(t *testing.T) {
	p := filepath.Join(t.TempDir(), "idle.log")
	_ = os.WriteFile(p, []byte("12:00:00.0  from before\n"), 0644)
	keep := logCap
	defer func() { logCap = keep }()
	logCap = 200

	logOutputTo(p)
	fmt.Println("first line")
	fmt.Println("second line")
	flushLog()
	data, _ := os.ReadFile(p)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || lines[0] != "12:00:00.0  from before" ||
		!strings.HasSuffix(lines[1], "  first line") || !strings.HasSuffix(lines[2], "  second line") {
		t.Fatalf("log: %q", lines)
	}
	if !regexp.MustCompile(`^\d\d:\d\d:\d\d\.\d  `).MatchString(lines[1]) {
		t.Errorf("no time: %q", lines[1])
	}

	// Past the cap it starts again, saying so.
	logOutputTo(p)
	for i := 0; i < 20; i++ {
		fmt.Printf("line %d\n", i)
	}
	flushLog()
	data, _ = os.ReadFile(p)
	if int64(len(data)) > logCap+100 || !strings.Contains(string(data), "started again") ||
		!strings.HasSuffix(strings.TrimSpace(string(data)), "line 19") {
		t.Errorf("capped log (%d bytes):\n%s", len(data), data)
	}
}
