package curses

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	gc "github.com/rthornton128/goncurses"
)

type ListPickerOpts struct {
	Title              string
	Buttons            []string
	DefaultButton      int
	ActionButton       int
	ShowTotal          bool
	Width              int
	Height             int
	DynamicActionLabel func(selectedItem int) string
	InitialIndex       int // the line highlighted first
	// Shortcuts maps extra keys to buttons by label (e.g. esc -> "Back").
	// A shortcut acts as if that button were pressed, keeping the current
	// list position. Labels not on this list's buttons are ignored.
	Shortcuts map[gc.Key]string
	// ItemButtons are buttons (by label) that act on the highlighted item,
	// like Fav: pressed from the button bar they report it too (other
	// buttons, like Back, report no item).
	ItemButtons []string
	// Headers are item indexes shown as headings: drawn, but never
	// highlighted or chosen. The highlight skips over them.
	Headers map[int]bool
	// PageSkip is how many rows at the top the page jumps (PgUp/PgDn, the
	// shoulder buttons) never land on, e.g. 1 for [Pick Random Game].
	PageSkip int
	// Top is the window's top row; 0 (the default) centres it.
	Top int
	// ScrollKey names this list for remembering its scroll position
	// between openings (see scrollMemory). Empty uses the Title; set it
	// when the title changes while the list is in use, e.g. a tick count.
	ScrollKey string
}

// scrollMemory is where each list was last scrolled to, by its ScrollKey
// or title. Menus close and reopen their list after every change (a tick,
// a setting), and a list that only knew which line to highlight worked
// out a fresh scroll position, so the page jumped around the highlight.
var scrollMemory = map[string]int{}

func ListPicker(stdscr *gc.Window, opts ListPickerOpts, items []string) (int, int, error) {
	// Start on InitialIndex (the first line if it's out of range).
	selectedItem := opts.InitialIndex
	if selectedItem < 0 || selectedItem >= len(items) {
		selectedItem = 0
	}
	selectedButton := opts.DefaultButton

	height, width := FitSize(stdscr, opts.Height, opts.Width)

	viewHeight := height - 4
	viewWidth := width - 4
	pgAmount := viewHeight - 1

	// Scroll so the starting line is visible.
	viewStart := 0
	if selectedItem >= viewHeight {
		if selectedItem > len(items)-viewHeight {
			viewStart = len(items) - viewHeight
		} else {
			viewStart = selectedItem - (viewHeight / 2)
			if viewStart < 0 {
				viewStart = 0
			}
		}
	}
	// Reopened (the same list, after a change): keep the page where it
	// was, as long as the highlight is still on it and it still fits.
	scrollKey := opts.ScrollKey
	if scrollKey == "" {
		scrollKey = opts.Title
	}
	if saved, ok := scrollMemory[scrollKey]; ok && saved >= 0 &&
		selectedItem >= saved && selectedItem < saved+viewHeight &&
		(saved == 0 || saved+viewHeight <= len(items)) {
		viewStart = saved
	}
	defer func() { scrollMemory[scrollKey] = viewStart }()

	// the highlighted line, and when it was highlighted (see BounceOffset)
	currentSelection := -1
	selectedAt := time.Now()

	top := -1
	if opts.Top > 0 {
		top = opts.Top
	}
	win, err := NewWindowAt(stdscr, top, height, width, opts.Title, -1)
	if err != nil {
		return -1, -1, err
	}
	defer win.Delete()

	isHeader := func(i int) bool { return opts.Headers[i] }
	// settle moves the highlight off a heading, in direction dir first.
	settle := func(dir int) {
		for _, d := range []int{dir, -dir} {
			i := selectedItem
			for i >= 0 && i < len(items) && isHeader(i) {
				i += d
			}
			if i >= 0 && i < len(items) {
				selectedItem = i
				return
			}
		}
	}
	// showSelected scrolls so the highlight is visible, along with the
	// heading just above it.
	showSelected := func() {
		if selectedItem < viewStart {
			viewStart = selectedItem
		}
		if selectedItem >= viewStart+viewHeight {
			viewStart = selectedItem - viewHeight + 1
		}
		if selectedItem > 0 && isHeader(selectedItem-1) && viewStart == selectedItem {
			viewStart--
		}
		if viewStart < 0 {
			viewStart = 0
		}
	}
	// Page jumps (the shoulder buttons, PgUp/PgDn). They never land on the
	// rows above first (e.g. [Pick Random Game]), and stop at either end:
	// only Up and Down go round. first is also where Down goes from the last
	// line.
	first := 0
	if opts.PageSkip < len(items) {
		first = opts.PageSkip
	}
	land := func(i, dir int) {
		selectedItem = i
		settle(dir)
		if selectedItem < first {
			selectedItem = first
			settle(1)
		}
		showSelected()
	}
	pageUp := func() {
		switch {
		case viewStart-pgAmount <= 0: // the first page: its first line
			viewStart = 0
			land(first, 1)
		default:
			viewStart -= pgAmount
			land(viewStart, 1)
		}
	}
	pageDown := func() {
		switch {
		case viewStart+viewHeight >= len(items): // the last page (or a short list): its last line
			land(len(items)-1, -1)
		default:
			viewStart += pgAmount
			if viewStart+viewHeight > len(items) {
				viewStart = len(items) - viewHeight
			}
			land(viewStart, -1)
		}
	}
	// countable is the number of real entries, and the highlight's position
	// among them, for the "3/40" counter.
	countable := func() (int, int) {
		total, pos := 0, 0
		for i := range items {
			if !isHeader(i) {
				total++
				if i <= selectedItem {
					pos++
				}
			}
		}
		return pos, total
	}
	if len(opts.Headers) > 0 {
		settle(1)
		showSelected()
	}

	// non-blocking input with 100ms tick
	win.Timeout(100)
	var ch gc.Key

	for {
		// A long highlighted line bounces from when it was highlighted.
		if selectedItem != currentSelection {
			currentSelection = selectedItem
			selectedAt = time.Now()
		}

		// list items
		max := len(items) - viewStart
		if viewHeight < max {
			max = viewHeight
		}

		// A scroll bar only when the list is longer than the page; it has the
		// last column, so the text stops before it.
		scrollBar := len(items) > viewHeight
		textWidth := viewWidth
		if scrollBar {
			textWidth--
		}

		for i := 0; i < max; i++ {
			item := items[viewStart+i]
			display := item
			selected := viewStart+i == selectedItem

			// Long lines: the highlighted one bounces (see BounceOffset), the
			// others end in "...". Measured in bytes, as the screen library
			// counts them, but never cut in the middle of an accented or
			// other non-English letter (see cutBytes).
			scrolling := len(item) > textWidth
			if scrolling {
				if selected {
					offset := BounceOffset(len(item), textWidth, time.Since(selectedAt))
					display = cutBytes(item, offset, offset+textWidth)
				} else {
					display = cutBytes(item, 0, textWidth-3) + "..."
				}
			}

			win.MovePrint(i+1, 2, strings.Repeat(" ", viewWidth))
			switch {
			case !selected:
				win.MovePrint(i+1, 2, display)
			case scrolling:
				// A bouncing line is highlighted across the whole width, so
				// the highlight doesn't change shape as it moves.
				win.ColorOn(1)
				win.MovePrint(i+1, 2, display+strings.Repeat(" ", textWidth-len(display)))
				win.ColorOff(1)
			default:
				// Otherwise just the text is highlighted, not its indent or
				// the padding after it (a blank line shows one block).
				lead := len(display) - len(strings.TrimLeft(display, " "))
				text := strings.TrimRight(display[lead:], " ")
				if text == "" {
					text = " "
				}
				win.ColorOn(1)
				win.MovePrint(i+1, 2+lead, text)
				win.ColorOff(1)
			}
		}

		// --- Buttons ---
		buttons := make([]string, len(opts.Buttons))
		copy(buttons, opts.Buttons)
		if opts.DynamicActionLabel != nil && len(buttons) > opts.ActionButton {
			buttons[opts.ActionButton] = opts.DynamicActionLabel(selectedItem)
		}

		DrawActionButtons(win, buttons, selectedButton)

		// --- Position/scroll indicators ---
		if opts.ShowTotal {
			pos, total := countable()
			totalStatus := fmt.Sprintf("%*d/%d", len(fmt.Sprint(total)), pos, total)
			win.MovePrint(0, 2, totalStatus)
		}

		// arrows
		if viewStart > 0 {
			win.MoveAddChar(0, width-3, gc.ACS_UARROW)
		} else {
			win.MoveAddChar(0, width-3, gc.ACS_HLINE)
		}
		if viewStart+viewHeight < len(items) {
			win.MoveAddChar(height-3, width-3, gc.ACS_DARROW)
		} else {
			win.MoveAddChar(height-3, width-3, gc.ACS_HLINE)
		}

		// --- Scroll bar ---
		scrollHeight := viewHeight
		if scrollHeight > 0 && scrollBar {
			var gripHeight int
			if len(items) <= scrollHeight {
				gripHeight = scrollHeight
			} else {
				gripHeight = int(float64(scrollHeight) * (float64(scrollHeight) / float64(len(items))))
				if gripHeight < 1 {
					gripHeight = 1
				}
			}

			gripOffset := 0
			if len(items) > scrollHeight {
				gripOffset = int(float64(viewStart) * float64(scrollHeight-gripHeight) / float64(len(items)-scrollHeight))
			}

			for i := 0; i < scrollHeight; i++ {
				if i >= gripOffset && i < gripOffset+gripHeight {
					win.ColorOn(1)
					win.MoveAddChar(i+1, width-3, ' ') // highlight grip
					win.ColorOff(1)
				} else {
					win.MoveAddChar(i+1, width-3, gc.ACS_CKBOARD) // track background
				}
			}
		}

		win.NoutRefresh()
		gc.Update()

		// Wait for a key, or the 100ms tick (for the scrolling title).
		ch = readKey(win)

		// Esc (B) and Backspace always press Back, where there is one. Where
		// there isn't (the main menu, with Exit), they do nothing: the list
		// used to close and be drawn again, which flashed the screen.
		if ch == gc.KEY_ESC || ch == gc.KEY_BACKSPACE || ch == 127 || ch == 8 {
			for i, b := range opts.Buttons {
				if strings.EqualFold(b, "Back") {
					return i, selectedItem, nil
				}
			}
			continue
		}

		if label, ok := opts.Shortcuts[ch]; ok {
			for i, b := range opts.Buttons {
				if b != "" && strings.EqualFold(b, label) {
					return i, selectedItem, nil
				}
			}
		}

		// A held button's repeats already waiting are taken now too: the list
		// moves for all of them, then draws once (see heldRepeats).
		for n := heldRepeats(win, ch); n > 0; n-- {
			// Moving (Up, Down, a page) always puts the highlight back on the
			// line's button (Open, Select...), from wherever it was.
			switch ch {
			case gc.KEY_UP, gc.KEY_DOWN, gc.KEY_PAGEUP, gc.KEY_PAGEDOWN:
				selectedButton = opts.ActionButton
			}
			switch ch {
			case gc.KEY_DOWN:
				if selectedItem < len(items)-1 {
					selectedItem++
					settle(1)
				} else {
					// Past the bottom: back to the first real line (not
					// [Pick Random Game]), with the top of the list shown.
					viewStart = 0
					selectedItem = first
					settle(1)
				}
				showSelected()
			case gc.KEY_UP:
				if selectedItem > 0 {
					selectedItem--
					settle(-1)
				} else {
					selectedItem = len(items) - 1 // past the top: round to the bottom
					settle(-1)
				}
				showSelected()
			case gc.KEY_LEFT:
				if selectedButton > 0 {
					selectedButton--
				} else {
					selectedButton = len(opts.Buttons) - 1
				}
			case gc.KEY_RIGHT:
				if selectedButton < len(opts.Buttons)-1 {
					selectedButton++
				} else {
					selectedButton = 0
				}
			case gc.KEY_PAGEUP:
				pageUp()
			case gc.KEY_PAGEDOWN:
				pageDown()
			case gc.KEY_ENTER, 10, 13:
				if selectedButton == opts.ActionButton {
					return selectedButton, selectedItem, nil
				} else if selectedButton < len(opts.Buttons) && opts.Buttons[selectedButton] == "PgUp" {
					pageUp()
				} else if selectedButton < len(opts.Buttons) && opts.Buttons[selectedButton] == "PgDn" {
					pageDown()
				} else {
					if selectedButton < len(opts.Buttons) {
						for _, b := range opts.ItemButtons {
							if strings.EqualFold(b, opts.Buttons[selectedButton]) {
								return selectedButton, selectedItem, nil
							}
						}
					}
					return selectedButton, -1, nil
				}
			}
		}
	}
}

// heldRepeats counts a movement key and its repeats already waiting,
// taking them from the input. A held button (a shoulder button's Page Down)
// repeats faster than MiSTer's console can draw a page, so drawing after
// each one fell behind, then caught up in a burst, and carried on after the
// button was let go. The first other key is put back. Other keys count once.
func heldRepeats(win *gc.Window, ch gc.Key) int {
	switch ch {
	case gc.KEY_UP, gc.KEY_DOWN, gc.KEY_PAGEUP, gc.KEY_PAGEDOWN:
	default:
		return 1
	}
	n := 1
	win.Timeout(0)
	defer win.Timeout(100)
	for {
		next := win.GetChar()
		if next == ch {
			n++
			continue
		}
		if next != 0 {
			gc.UnGetChar(gc.Char(next))
		}
		return n
	}
}

// cutBytes is s[from:to], moved inwards so it starts and ends on whole
// characters: a letter made of several bytes (é, ö, Japanese...) is left
// out rather than cut in half into a broken symbol.
func cutBytes(s string, from, to int) string {
	if to > len(s) {
		to = len(s)
	}
	for from < to && !utf8.RuneStart(s[from]) {
		from++
	}
	for to > from && to < len(s) && !utf8.RuneStart(s[to]) {
		to--
	}
	return s[from:to]
}
