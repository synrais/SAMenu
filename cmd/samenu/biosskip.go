package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/input"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// BIOS Skip
// -------------------------
//
// Options -> Controls -> BIOS Skip: when BIOS skip runs (after attract
// mode's launches, yours, both or neither), and each system's sequence
// ([BiosSkip.<ID>] Sequence), made by pressing the buttons and keys.
//
// The virtual pad's names are by position, Nintendo style: a = right,
// b = bottom, x = top, y = left. A controller's buttons are read with
// SDL's names (Xbox style: a = bottom), so a press is saved by position:
// the bottom button (Cross) becomes b, the virtual pad's bottom button.

// biosPositions names each virtual pad button's place.
var biosPositions = map[string]string{"a": "right", "b": "bottom", "x": "top", "y": "left"}

// sdlToBios turns a controller input (SDL names) into a virtual pad name.
var sdlToBios = map[string]string{
	"a": "b", "b": "a", "x": "y", "y": "x",
	"leftshoulder": "l", "rightshoulder": "r",
	"back": "select", "start": "start", "guide": "home",
	"dpup": "up", "dpdown": "down", "dpleft": "left", "dpright": "right",
	"leftx-": "left", "leftx+": "right", "lefty-": "up", "lefty+": "down",
}

// biosButtons are the virtual pad's buttons, to pick from when a press
// can't be named.
var biosButtons = []string{"a", "b", "x", "y", "l", "r", "select", "start", "home", "up", "down", "left", "right"}

var biosWaits = []string{"0.5", "1", "2", "3", "5", "10", "15", "20", "30"}

// stepText describes one step of a sequence.
func stepText(t string) string {
	if _, err := strconv.ParseFloat(t, 64); err == nil {
		return "Wait " + t + " s"
	}
	if k, ok := strings.CutPrefix(t, "key:"); ok {
		return "Key " + k
	}
	if p, ok := biosPositions[t]; ok {
		return "Press " + t + " (" + p + ")"
	}
	return "Press " + t
}

func splitSequence(seq string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(seq, func(r rune) bool { return r == ',' || r == ' ' }) {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// biosSkipScreen is Options -> Controls -> BIOS Skip.
func biosSkipScreen(stdscr *gc.Window, cfg *config.Config, sysNames []string) {
	names := systemNamesByID()
	nameOf := func(id string) string {
		if n, ok := names[strings.ToLower(id)]; ok {
			return n
		}
		return id
	}
	selected := 0
	for {
		ids := make([]string, 0, len(cfg.AutoInput.Sequences))
		for id := range cfg.AutoInput.Sequences {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return strings.ToLower(nameOf(ids[i])) < strings.ToLower(nameOf(ids[j])) })
		items := []string{fmt.Sprintf("%-22s %s", "Used for:", biosSkipText(cfg))}
		for _, id := range ids {
			items = append(items, fmt.Sprintf("%-22s %s", nameOf(id)+":", strings.Join(splitSequence(cfg.AutoInput.Sequences[id]), ", ")))
		}
		items = append(items, "Add a system...")
		sel := bindingPicker(stdscr, "BIOS Skip", items, selected)
		if sel < 0 {
			break
		}
		selected = sel
		switch {
		case sel == len(items):
			// Restore defaults: Both, and only the FDS's sequence.
			for _, id := range ids {
				saveErr(stdscr, config.SaveBiosSequence(cfg, properSystemID(id), ""))
			}
			cfg.AutoInput.Attract, cfg.AutoInput.Menu = true, true
			saveErr(stdscr, config.SaveBiosSequence(cfg, "FDS", "10, a"), saveBiosWhen(cfg))
		case sel == 0:
			nextBiosSkip(cfg)
			saveErr(stdscr, saveBiosWhen(cfg))
		case sel == len(items)-1:
			// Pick a system without a sequence yet.
			var choices, choiceIDs []string
			for _, id := range systemIDs(sysNames) {
				if _, has := cfg.AutoInput.Sequences[strings.ToLower(id)]; !has && !strings.EqualFold(id, "Arcade") {
					choices = append(choices, nameOf(id))
					choiceIDs = append(choiceIDs, id)
				}
			}
			if len(choices) == 0 {
				message(stdscr, "Every system already has a sequence.")
				continue
			}
			if c, ok := optionsList(stdscr, "Add BIOS skip for", choices, 0); ok {
				biosSequenceEditor(stdscr, cfg, choiceIDs[c], nameOf(choiceIDs[c]))
			}
		default:
			id := ids[sel-1]
			biosSequenceEditor(stdscr, cfg, properSystemID(id), nameOf(id))
		}
	}
	clearScreen(stdscr)
}

func saveBiosWhen(cfg *config.Config) error {
	return config.SaveValues(cfg.Path, "BiosSkip", [][2]string{
		{"Attract", strconv.FormatBool(cfg.AutoInput.Attract)}, {"Menu", strconv.FormatBool(cfg.AutoInput.Menu)},
	})
}

// biosSequenceEditor edits one system's sequence: its steps, and adding a
// press (by pressing it) or a wait.
func biosSequenceEditor(stdscr *gc.Window, cfg *config.Config, id, name string) {
	steps := splitSequence(cfg.AutoInput.Sequences[strings.ToLower(id)])
	save := func() {
		saveErr(stdscr, config.SaveBiosSequence(cfg, id, strings.Join(steps, ", ")))
	}
	selected := len(steps)
	for {
		var items []string
		for i, t := range steps {
			items = append(items, fmt.Sprintf("%d. %s", i+1, stepText(t)))
		}
		addPress, addWait := len(items), len(items)+1
		items = append(items, "Add a press...", "Add a wait...")
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "BIOS skip: " + name,
			Buttons:       []string{"Choose", "Back"},
			ActionButton:  0,
			DefaultButton: 0,
			Width:         optionsWidth,
			Height:        len(items) + 4,
			InitialIndex:  selected,
		}, items)
		if err != nil || button != 0 {
			break
		}
		selected = sel
		switch sel {
		case addPress:
			if t, ok := captureBiosStep(stdscr); ok {
				steps = append(steps, t)
				save()
				selected = len(steps)
			}
		case addWait:
			if c, ok := optionsList(stdscr, "Wait how many seconds?", biosWaits, 5); ok {
				steps = append(steps, biosWaits[c])
				save()
				selected = len(steps) + 1
			}
		default:
			if c, ok := optionsList(stdscr, fmt.Sprintf("Step %d: %s", sel+1, stepText(steps[sel])), []string{"Remove it", "Keep it"}, 1); ok && c == 0 {
				steps = append(steps[:sel], steps[sel+1:]...)
				save()
			}
		}
	}
	clearScreen(stdscr)
}

// captureBiosStep waits for a button or key for a BIOS skip step, named
// for the virtual pad (by position) or as key:<name>. A button with no
// virtual pad name offers a list to pick from. The press that opened this
// is ignored.
func captureBiosStep(stdscr *gc.Window) (string, bool) {
	events := takeInputs()
	defer releaseInputs()
	input.SetDebug(true) // releases, for the gate
	defer input.SetDebug(false)
	accept := captureGate()
	_ = curses.InfoBox(stdscr, "", fmt.Sprintf("Press the button or key for this step\n(wait %ds to cancel)", int(bindTimeout.Seconds())), true, false)
	defer func() {
		// The same press usually also reaches the menu as a key.
		time.Sleep(150 * time.Millisecond)
		_ = gc.FlushInput()
	}()
	timeout := time.After(bindTimeout)
	for {
		select {
		case ev := <-events:
			if !accept(ev) {
				continue
			}
			switch ev.Kind {
			case "keyboard":
				t := "key:" + strings.ToLower(ev.Name)
				if _, err := mister.ParseSequence(t); err == nil {
					return t, true
				}
			case "joystick":
				if t, ok := sdlToBios[strings.ToLower(ev.Name)]; ok {
					return t, true
				}
				// Not a name the virtual pad knows (e.g. btn7): pick one.
				labels := make([]string, len(biosButtons))
				for i, b := range biosButtons {
					labels[i] = stepText(b)
				}
				if c, ok := optionsList(stdscr, ev.Name+": which virtual pad button?", labels, 0); ok {
					return biosButtons[c], true
				}
				return "", false
			}
		case <-timeout:
			return "", false
		}
	}
}

// properSystemID is a system ID as the systems list spells it ("fds" ->
// "FDS").
func properSystemID(id string) string {
	if config.SystemIDCase != nil {
		if p := config.SystemIDCase(id); p != "" {
			return p
		}
	}
	return id
}
