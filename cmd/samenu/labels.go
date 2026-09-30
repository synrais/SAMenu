package main

import (
	"fmt"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/games"
)

// -------------------------
// Menu list options
// -------------------------
//
// These settings control how the main system list shows each
// "manufacturer + system" label, e.g. "[Nintendo] NES". They're changed from
// Options -> Display & Sorting -> Menu list options and saved to the [Menu] section of SAMenu.ini.

// minManufacturerWidth is the narrowest the manufacturer column is ever cut
// to, and only when a full line wouldn't otherwise fit on screen.
const minManufacturerWidth = 8

// systemListWidth is the width of the main system list window. Its text area
// is 4 columns narrower (borders and scroll bar).
var systemListWidth = 70 // set from the screen by fitToScreen

type labelOption struct {
	name   string
	values []string
	index  int
}

func (o *labelOption) value() string { return o.values[o.index] }
func (o *labelOption) next()         { o.index = (o.index + 1) % len(o.values) }

// set picks the value with the given name, ignoring case. Unknown or empty
// names keep the current (default) choice.
func (o *labelOption) set(name string) {
	for i, v := range o.values {
		if strings.EqualFold(v, strings.TrimSpace(name)) {
			o.index = i
			return
		}
	}
}

// setBool picks between a two-value option's values: [0] for false, [1]
// for true.
func (o *labelOption) setBool(b bool) {
	if b {
		o.index = 1
	} else {
		o.index = 0
	}
}

func (o *labelOption) isOn() bool { return o.index == 1 }

// applyMenuConfig loads the saved menu settings from SAMenu.ini's [Menu].
func applyMenuConfig(m config.MenuConfig) {
	if m.GenresSystem != "" {
		genresSystem = m.GenresSystem
	}
	if m.GenresOrder != "" {
		genresOrder = m.GenresOrder
	}
	genresLeftOut, _ = games.ResolveSystems(m.GenresExclude)
	optAlign.set(m.LabelAlign)
	optDivider.set(m.LabelDivider)
	optAlignDivider.setBool(m.LabelAlignDivider)
	optEncapsulate.set(m.LabelEncapsulate)
	optShowManufacturer.setBool(!m.ShowManufacturer)
	optTextSize.set(m.TextSize)
	optScreenMargin.set(m.ScreenMargin)
	optRememberPos.setBool(m.RememberPosition)
	applySortConfig(m)
}

// saveMenuConfig stores the current menu settings in SAMenu.ini's [Menu].
// Only these fields change: the rest of cfg.Menu (HideTags, the [History]
// and [Favourites] switches...) is kept, not reset.
func saveMenuConfig(cfg *config.Config) error {
	m := &cfg.Menu
	m.LabelAlign = optAlign.value()
	m.LabelDivider = optDivider.value()
	m.LabelAlignDivider = optAlignDivider.isOn()
	m.LabelEncapsulate = optEncapsulate.value()
	m.ShowManufacturer = !optShowManufacturer.isOn()
	m.TextSize = optTextSize.value()
	m.ScreenMargin = optScreenMargin.value()
	m.RememberPosition = optRememberPos.isOn()
	storeSortConfig(m)
	return config.SaveMenu(cfg)
}

var (
	optAlign        = labelOption{"Alignment", []string{"Left", "Center", "Right"}, 0}
	optDivider      = labelOption{"Divider", []string{"Space", "Dash", "Colon", "Pipe", "Arrow"}, 0}
	optAlignDivider = labelOption{"Align to divider", []string{"Off", "On"}, 0}
	optEncapsulate  = labelOption{"Encapsulate", []string{
		"None", "[ ]", "( )", "< >", "{ }",
		"[ spaced ]", "( spaced )", "< spaced >", "{ spaced }",
	}, 1}
	optShowManufacturer = labelOption{"Manufacturer", []string{"Show", "Hide"}, 0}
)

var dividers = map[string]string{
	"Space": " ",
	"Dash":  " - ",
	"Colon": " : ",
	"Pipe":  " | ",
	"Arrow": " > ",
}

// systemLabels builds the list labels for the given system names using the
// current Menu list options, positioned within a text area of viewWidth.
func systemLabels(names []string, viewWidth int) []string {
	// Manufacturer hidden: just the full system name, e.g. "Atari 2600".
	if optShowManufacturer.isOn() {
		labels := make([]string, len(names))
		for i, name := range names {
			labels[i] = strings.Repeat(" ", screenOffset(len(name), viewWidth)) + name
		}
		return labels
	}

	manufacturers := make([]string, len(names))
	shorts := make([]string, len(names))
	width, longestShort := 0, 0
	for i, name := range names {
		manufacturers[i], shorts[i] = games.SystemLabelParts(name)
		if len(manufacturers[i]) > width {
			width = len(manufacturers[i])
		}
		if len(shorts[i]) > longestShort {
			longestShort = len(shorts[i])
		}
	}

	// The manufacturer column fits the longest manufacturer in full. It's only
	// cut short (with "..") if the longest line wouldn't fit on screen.
	open, close := encapsulators()
	fits := viewWidth - len(open) - len(close) - len(dividers[optDivider.value()]) - longestShort
	if width > fits {
		width = fits
		if width < minManufacturerWidth {
			width = minManufacturerWidth
		}
	}

	labels := make([]string, len(names))
	longest := 0
	for i := range names {
		labels[i] = formatLabel(manufacturers[i], shorts[i], width)
		if len(labels[i]) > longest {
			longest = len(labels[i])
		}
	}

	// Place the lines on screen. With aligned dividers the whole block moves
	// together, so the dividers stay in one column; centred, that column sits
	// in the middle of the screen.
	aligned := optAlignDivider.isOn()
	for i, label := range labels {
		var offset int
		switch {
		case aligned && optAlign.value() == "Center":
			offset = dividerCenterOffset(width, viewWidth)
		case aligned:
			offset = screenOffset(longest, viewWidth)
		default:
			offset = screenOffset(len(label), viewWidth)
		}
		labels[i] = strings.Repeat(" ", offset) + label
	}
	return labels
}

// formatLabel builds one label, e.g. "(Nintendo) NES". With aligned dividers
// the start of the line is padded so every divider lines up.
func formatLabel(manufacturer, short string, width int) string {
	open, close := encapsulators()
	divider := dividers[optDivider.value()]

	if !optAlignDivider.isOn() {
		return open + manufacturer + close + divider + short
	}
	block := open + fitText(manufacturer, width) + close
	pad := width + len(open) + len(close) - len(block)
	return strings.Repeat(" ", pad) + block + divider + short
}

// rightMargin keeps right-aligned text clear of the list's scroll bar.
const rightMargin = 2

// screenOffset returns how far to indent a line of lineWidth to place it
// left, center or right within viewWidth.
func screenOffset(lineWidth, viewWidth int) int {
	gap := viewWidth - lineWidth
	switch optAlign.value() {
	case "Center":
		gap /= 2
	case "Right":
		gap -= rightMargin
	default:
		gap = 0
	}
	if gap < 0 {
		return 0
	}
	return gap
}

// dividerCenterOffset returns the indent that puts the aligned divider in
// the middle of the screen.
func dividerCenterOffset(width, viewWidth int) int {
	open, close := encapsulators()
	divider := dividers[optDivider.value()]
	dividerMid := width + len(open) + len(close) + len(divider)/2
	if offset := viewWidth/2 - dividerMid; offset > 0 {
		return offset
	}
	return 0
}

// encapsulators returns what goes before and after the manufacturer, e.g.
// "[" and "]", or "[ " and " ]" for the spaced versions. Both are empty for
// "None".
func encapsulators() (string, string) {
	enc := optEncapsulate.value()
	if enc == "None" {
		return "", ""
	}
	open, close := enc[:1], enc[len(enc)-1:]
	if strings.Contains(enc, "spaced") {
		return open + " ", " " + close
	}
	return open, close
}

// fitText shortens s to width with "..", if it's too long.
func fitText(s string, width int) string {
	if len(s) > width {
		return s[:width-2] + ".."
	}
	return s
}

// optionsWidth is the width of the options windows.
var optionsWidth = 70 // set from the screen by fitToScreen

// menuListOptions shows the label settings with a live preview.
func menuListOptions(stdscr *gc.Window, sysIds []string, cfg *config.Config) {
	preview := previewIndexes(sysIds)
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title: "Menu list options",
		options: func() []*labelOption {
			if optShowManufacturer.isOn() {
				// The other settings only shape the manufacturer part.
				return []*labelOption{&optRememberPos, &optShowManufacturer, &optAlign}
			}
			return []*labelOption{&optRememberPos, &optShowManufacturer, &optAlign, &optDivider, &optAlignDivider, &optEncapsulate}
		},
		preview: func() []string {
			labels := systemLabels(sysIds, optionsWidth-4)
			lines := make([]string, 0, len(preview))
			for _, i := range preview {
				lines = append(lines, labels[i])
			}
			return lines
		},
	})
}

// optionsScreen describes one settings screen: its visible options (which
// can depend on other settings), a live preview, and an optional hook run
// after a setting changes (to keep settings consistent).
type optionsScreen struct {
	title     string
	options   func() []*labelOption
	preview   func() []string
	changed   func(o *labelOption)
	save      func() error // default: save [Menu]
	noPreview bool
}

// runOptionsScreen shows the settings on a menu screen, like every other
// options screen, with the preview in its own box underneath. Selecting a
// setting moves it on to its next value; one with a single value is chosen
// on another screen (the changed hook opens it), so it reads Select.
// Changes are saved to SAMenu.ini once, on the way out: some saves do more
// (Startup rewrites user-startup.sh), so not after every press.
func runOptionsScreen(stdscr *gc.Window, cfg *config.Config, sc optionsScreen) {
	changed := false
	m := &menuScreen{title: sc.title}
	m.lines = func() []menuLine {
		var lines []menuLine
		for _, o := range sc.options() {
			o := o
			press := func() {
				o.next()
				if sc.changed != nil {
					sc.changed(o)
				}
				changed = true
			}
			text := settingText(o.name+":", o.value())
			if len(o.values) == 1 {
				lines = append(lines, opens(text, press))
			} else {
				lines = append(lines, setting(text, press))
			}
		}
		return lines
	}
	if !sc.noPreview {
		m.preview = sc.preview
	}
	m.leave = func() {
		clearScreen(stdscr)
		if !changed {
			return
		}
		save := sc.save
		if save == nil {
			save = func() error { return saveMenuConfig(cfg) }
		}
		if err := save(); err != nil {
			_ = curses.InfoBox(stdscr, "Error", fmt.Sprintf("Couldn't save settings: %v", err), false, true)
			clearScreen(stdscr)
		}
	}
	m.run(stdscr)
}

// drawPreview draws a "Preview" box just below the centred options window.
// It returns nil if the box couldn't be created.
func drawPreview(stdscr *gc.Window, lines []string, y int) *gc.Window {
	rows, cols := stdscr.MaxYX()
	h, w := len(lines)+2, optionsWidth
	if y+h > rows-curses.Margin.Y {
		y = rows - curses.Margin.Y - h
	}
	x := (cols - w) / 2
	if y < 0 || x < 0 {
		return nil
	}

	win, err := gc.NewWindow(h, w, y, x)
	if err != nil {
		return nil
	}
	win.Box(gc.ACS_VLINE, gc.ACS_HLINE)
	win.MovePrint(0, 2, " Preview ")
	for i, line := range lines {
		if len(line) > w-4 {
			line = line[:w-4]
		}
		win.MovePrint(1+i, 2, line)
	}
	win.NoutRefresh()
	_ = gc.Update()
	return win
}

// previewIndexes picks a few systems to preview: the first, the one with the
// longest manufacturer, and the last.
func previewIndexes(names []string) []int {
	if len(names) == 0 {
		return nil
	}
	longest := 0
	for i, name := range names {
		m, _ := games.SystemLabelParts(name)
		if l, _ := games.SystemLabelParts(names[longest]); len(m) > len(l) {
			longest = i
		}
	}
	idx := []int{0}
	if longest != 0 {
		idx = append(idx, longest)
	}
	if last := len(names) - 1; last != 0 && last != longest {
		idx = append(idx, last)
	}
	return idx
}
