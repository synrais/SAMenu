package mister

import (
	"strings"
)

// MiSTer.ini editing
//
// Options -> Screen changes a few MiSTer.ini settings. The file is the
// user's, full of comments and hand-made layout, so it's edited line by
// line: a setting's value is replaced where it is, keeping the spacing and
// the comment after it; a new one goes at the end of its section; one set
// back to MiSTer's own default is commented out, not deleted. Everything
// else, line endings included, is left exactly as it was.
//
// Sections are [MiSTer] (every core; settings before any section header
// count as [MiSTer] too) and the per-core ones, e.g. [Menu] for the MiSTer
// menu core only, which SAMenu's screen is drawn on. A section header can
// span lines ("[Amiga" ... "+Amiga500]"); such a section is never one
// SAMenu edits, but it's skipped over correctly.

// iniLine is one line of MiSTer.ini, taken apart.
type iniLine struct {
	section string // the section it's in, lower case ("mister" for the top)
	key     string // lower case, "" for anything but a setting
	valFrom int    // where the value starts in the line
	valTo   int    // where it ends (before spaces and an inline comment)
}

// parseIni takes MiSTer.ini's lines apart.
func parseIni(lines []string) []iniLine {
	out := make([]iniLine, len(lines))
	section, inHeader := "mister", false
	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		t := strings.TrimSpace(line)
		switch {
		case inHeader: // a header over several lines, until its "]"
			if strings.Contains(t, "]") {
				inHeader = false
			}
			out[i].section = section
			continue
		case strings.HasPrefix(t, "["):
			if end := strings.Index(t, "]"); end > 0 {
				section = strings.ToLower(strings.TrimSpace(t[1:end]))
			} else {
				section, inHeader = "\x00multi", true // never one SAMenu edits
			}
			out[i].section = section
			continue
		}
		out[i].section = section
		if t == "" || strings.HasPrefix(t, ";") || strings.HasPrefix(t, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		out[i].key = strings.ToLower(strings.TrimSpace(line[:eq]))
		from := eq + 1
		for from < len(line) && (line[from] == ' ' || line[from] == '\t') {
			from++
		}
		to := len(line)
		if c := strings.Index(line[from:], ";"); c >= 0 {
			to = from + c
		}
		for to > from && (line[to-1] == ' ' || line[to-1] == '\t') {
			to--
		}
		out[i].valFrom, out[i].valTo = from, to
	}
	return out
}

// IniGet returns a setting's value in a section of MiSTer.ini's text (the
// last one, as MiSTer reads them in order), and whether it's set at all.
func IniGet(text, section, key string) (string, bool) {
	lines := strings.Split(text, "\n")
	section, key = strings.ToLower(section), strings.ToLower(key)
	val, found := "", false
	for i, l := range parseIni(lines) {
		if l.section == section && l.key == key {
			line := strings.TrimRight(lines[i], "\r")
			val, found = line[l.valFrom:l.valTo], true
		}
	}
	return val, found
}

// IniSet returns MiSTer.ini's text with a setting changed. With remove,
// the setting goes back to MiSTer's own default: its lines are commented
// out. A new setting goes after the last setting of its section; a
// section that isn't there yet is added at the end of the file.
func IniSet(text, section, key, value string, remove bool) string {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(text, "\n")
	parsed := parseIni(lines)
	lsec, lkey := strings.ToLower(section), strings.ToLower(key)
	cr := func(i int) string { // the line's own "\r", if it had one
		if strings.HasSuffix(lines[i], "\r") {
			return "\r"
		}
		return ""
	}

	last, lastInSection, headerAt := -1, -1, -1
	for i, l := range parsed {
		if l.section != lsec {
			continue
		}
		if headerAt < 0 && strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			headerAt = i
		}
		if l.key != "" {
			lastInSection = i
		}
		if l.key == lkey {
			last = i
		}
	}

	if remove {
		for i, l := range parsed {
			if l.section == lsec && l.key == lkey {
				lines[i] = ";" + lines[i]
			}
		}
		return strings.Join(lines, "\n")
	}
	if last >= 0 {
		line := strings.TrimRight(lines[last], "\r")
		l := parsed[last]
		rest := line[l.valTo:]
		// Keep an inline comment in its column, if there's room: the
		// comments in MiSTer.ini are lined up.
		if comment := strings.TrimLeft(rest, " \t"); comment != "" {
			col := l.valTo + len(rest) - len(comment)
			pad := col - (l.valFrom + len(value))
			if pad < 1 {
				pad = 1
			}
			rest = strings.Repeat(" ", pad) + comment
		}
		lines[last] = line[:l.valFrom] + value + rest + cr(last)
		return strings.Join(lines, "\n")
	}
	add := key + "=" + value
	switch {
	case lastInSection >= 0:
		at := lastInSection + 1
		lines = append(lines[:at], append([]string{add + cr(lastInSection)}, lines[at:]...)...)
	case headerAt >= 0:
		at := headerAt + 1
		lines = append(lines[:at], append([]string{add + cr(headerAt)}, lines[at:]...)...)
	case lsec == "mister":
		lines = append([]string{"[MiSTer]" + strings.TrimSuffix(nl, "\n"), add + strings.TrimSuffix(nl, "\n")}, lines...)
	default:
		out := strings.TrimRight(strings.Join(lines, "\n"), "\r\n")
		return out + nl + nl + "[" + section + "]" + nl + add + nl
	}
	return strings.Join(lines, "\n")
}
