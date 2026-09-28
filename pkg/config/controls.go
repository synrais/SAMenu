package config

import (
	"os"
	"sort"
	"strconv"
	"strings"
)

// DefaultMenuControls: the games menu's actions and their inputs. Each
// applies in one place, so the same button can do different things: on
// the systems list Y (tab) = Search and X (space) = Options; in a game
// list Y = Favourite; in [Favourites] X (or Y) = Remove. An input is a key
// name ("tab"), or "pad:<name>" for a controller button MiSTer doesn't pass
// on as a key (Start, Select, L2...), read by SAMenu's input detectors.
var DefaultMenuControls = map[string][]string{
	"search":    {"tab"},
	"options":   {"space"},
	"favourite": {"tab"},
	"remove":    {"space"},
}

// MenuActions are the games menu's actions, in the order they're listed.
var MenuActions = []string{"search", "options", "favourite", "remove"}

// Attract mode actions ("back" is the previous game). Favourite, mute and
// screenshot act without leaving the game.
var AttractActions = []string{"next", "back", "play", "stop", "blacklist", "stay", "menu", "search", "favourite", "mute", "screenshot"}

// Attract input kinds, matching the [InputDetector.X] section names.
var AttractKinds = []string{"keyboard", "mouse", "joystick"}

// DefaultAttractControls: kind -> input -> action.
var DefaultAttractControls = map[string]map[string]string{
	"keyboard": {"right": "next", "left": "back", "enter": "play", "esc": "stop", "delete": "blacklist", "space": "stay", "`": "search"},
	"mouse":    {"right": "next", "left": "back"},
	"joystick": {"dpright": "next", "dpleft": "back", "leftx+": "next", "leftx-": "back", "start": "play", "back": "stop"},
}

func copyMenuControls(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func copyAttractControls(m map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(m))
	for kind, binds := range m {
		out[kind] = make(map[string]string, len(binds))
		for in, act := range binds {
			out[kind][in] = act
		}
	}
	return out
}

// DefaultMenuControlsCopy and DefaultAttractControlsCopy return fresh
// copies of the defaults, safe to change.
func DefaultMenuControlsCopy() map[string][]string { return copyMenuControls(DefaultMenuControls) }
func DefaultAttractControlsCopy() map[string]map[string]string {
	return copyAttractControls(DefaultAttractControls)
}

// sdlToMister swaps SDL's face button names for MiSTer's: the same
// button, by position (SDL's a, the bottom one, is MiSTer's b).
var sdlToMister = map[string]string{"a": "b", "b": "a", "x": "y", "y": "x"}

// misterPadNames brings an older SAMenu.ini over to MiSTer's button names,
// once: controller bindings saved with SDL's names (a = bottom) are
// renamed, so each stays on the same button. [InputDetector] PadNames =
// MiSTer marks it done.
func misterPadNames(cfg *Config) {
	if strings.EqualFold(cfg.InputDetector.PadNames, "MiSTer") {
		return
	}
	previous := copyAttractControls(cfg.AttractControls)
	attract := false
	binds := map[string]string{}
	for in, act := range cfg.AttractControls["joystick"] {
		if m, ok := sdlToMister[in]; ok {
			in, attract = m, true
		}
		binds[in] = act
	}
	menu := false
	for act, ins := range cfg.MenuControls {
		for i, in := range ins {
			if m, ok := sdlToMister[strings.TrimPrefix(in, "pad:")]; ok && strings.HasPrefix(in, "pad:") {
				ins[i], menu = "pad:"+m, true
			}
		}
		cfg.MenuControls[act] = ins
	}
	if attract {
		cfg.AttractControls["joystick"] = binds
		_ = SaveAttractControls(cfg, previous)
	}
	if menu {
		_ = SaveMenuControls(cfg)
	}
	cfg.InputDetector.PadNames = "MiSTer"
	_ = SaveValues(cfg.Path, "InputDetector", [][2]string{{"PadNames", "MiSTer"}})
}

// SaveMenuLayout writes [Controls.Menu] Layout.
// SaveMenuControls writes the games menu's actions to [Controls.Menu].
func SaveMenuControls(cfg *Config) error {
	var values [][2]string
	for _, act := range MenuActions {
		values = append(values, [2]string{act, strings.Join(cfg.MenuControls[act], ", ")})
	}
	return SaveValues(cfg.Path, "Controls.Menu", values)
}

func SaveMenuLayout(cfg *Config) error {
	return SaveValues(cfg.Path, "Controls.Menu", [][2]string{{"Layout", cfg.MenuLayout}})
}

// SaveInputDetector writes the [InputDetector] on/off switches.
func SaveInputDetector(cfg *Config) error {
	d := cfg.InputDetector
	return SaveValues(cfg.Path, "InputDetector", [][2]string{
		{"Mouse", strconv.FormatBool(d.Mouse)}, {"Keyboard", strconv.FormatBool(d.Keyboard)}, {"Joystick", strconv.FormatBool(d.Joystick)},
		{"Sticks", strconv.FormatBool(d.Sticks)}, {"StickHoldMs", strconv.Itoa(d.StickHoldMs)},
	})
}

// SaveAttractControls writes the three [InputDetector.X] sections. Inputs
// that were bound before but aren't now are kept with an empty value, so
// the file's own list of buttons stays intact.
func SaveAttractControls(cfg *Config, previous map[string]map[string]string) error {
	for _, kind := range AttractKinds {
		var values [][2]string
		seen := map[string]bool{}
		for in, act := range cfg.AttractControls[kind] {
			values = append(values, [2]string{in, act})
			seen[in] = true
		}
		for in := range previous[kind] {
			if !seen[in] {
				values = append(values, [2]string{in, ""})
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i][0] < values[j][0] })
		if err := SaveValues(cfg.Path, "InputDetector."+titleCase(kind), values); err != nil {
			return err
		}
	}
	return nil
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// SaveValues sets keys in one section of an INI file, keeping everything
// else (comments, spacing, other sections) exactly as it is.
func SaveValues(path, section string, values [][2]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	newline := "\n"
	if strings.Contains(string(data), "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines = setSectionValues(lines, section, values)

	// Write to a temp file first, so a failed write can't leave the file
	// half written.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, newline)), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// iniKey writes a key so the INI reader gets it back: keys with characters
// that mean something in INI files are quoted.
func iniKey(k string) string {
	if strings.ContainsAny(k, "=:;#[]\"` ") {
		if strings.Contains(k, "\"") {
			return "`" + k + "`"
		}
		return "\"" + k + "\""
	}
	return k
}

// SaveSetCores writes the [Systems] set_core lines (system ID -> core; an
// empty core means the system's own), replacing the old ones and leaving
// the rest of [Systems] (games_folder, system_folder, comments) alone.
func SaveSetCores(cfg *Config, cores map[string]string) error {
	data, err := os.ReadFile(cfg.Path)
	if err != nil {
		return err
	}
	newline := "\n"
	if strings.Contains(string(data), "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	ids := make([]string, 0, len(cores))
	for id, c := range cores {
		if strings.TrimSpace(c) != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var add []string
	var setCore []string
	for _, id := range ids {
		add = append(add, "set_core = "+id+":"+cores[id])
		setCore = append(setCore, id+":"+cores[id])
	}

	var out []string
	inSystems, found, inserted := false, false, false
	insertHere := func() {
		if !inserted {
			// keep a blank line before the next section
			n := len(out)
			for n > 0 && strings.TrimSpace(out[n-1]) == "" {
				n--
			}
			tail := append([]string(nil), out[n:]...)
			out = append(append(out[:n], add...), tail...)
			inserted = true
		}
	}
	for _, line := range lines {
		if name, ok := sectionName(line); ok {
			if inSystems {
				insertHere()
			}
			inSystems = strings.EqualFold(name, "Systems")
			found = found || inSystems
			out = append(out, line)
			continue
		}
		if inSystems {
			if key, _, ok := keyOnLine(line); ok && strings.EqualFold(key, "set_core") {
				continue // replaced below
			}
		}
		out = append(out, line)
	}
	if inSystems {
		insertHere()
	}
	if !found && len(add) > 0 {
		out = append(out, "", "[Systems]")
		out = append(out, add...)
	}
	cfg.Systems.SetCore = setCore
	tmp := cfg.Path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(out, newline)), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, cfg.Path)
}

// SaveBiosSequence writes a system's BIOS skip sequence ([BiosSkip.<ID>]
// Sequence), or removes the section when the sequence is empty.
func SaveBiosSequence(cfg *Config, systemID, seq string) error {
	if cfg.AutoInput.Sequences == nil {
		cfg.AutoInput.Sequences = map[string]string{}
	}
	section := "BiosSkip." + systemID
	if strings.TrimSpace(seq) == "" {
		delete(cfg.AutoInput.Sequences, strings.ToLower(systemID))
		return RemoveSection(cfg.Path, section)
	}
	cfg.AutoInput.Sequences[strings.ToLower(systemID)] = seq
	return SaveValues(cfg.Path, section, [][2]string{{"Sequence", seq}})
}
