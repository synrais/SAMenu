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
	InitialIndex       int  // 🔹 where to start highlight
	SnapToAction       bool // Up/Down/PgUp/PgDn move the highlight back to the action button
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

// closedByBack is whether the last list closed was backed out of.
var closedByBack bool

// ClosedByBack reports whether the last list to close was closed with its
// Back button (or Esc): the user backed out, rather than choosing something.
// A screen that opened another uses it to land on its own Back button, so
// backing out carries on backing out.
func ClosedByBack() bool { return closedByBack }

func ListPicker(stdscr *gc.Window, opts ListPickerOpts, items []string) (button int, item int, err error) {
	defer func() {
		closedByBack = button >= 0 && button < len(opts.Buttons) && strings.EqualFold(opts.Buttons[button], "Back")
	}()
	// Apply InitialIndex safely
	selectedItem := opts.InitialIndex
	if selectedItem < 0 || selectedItem >= len(items) {
		selectedItem = 0
	}
	selectedButton := opts.DefaultButton

	height, width := FitSize(stdscr, opts.Height, opts.Width)

	viewHeight := height - 4
	viewWidth := width - 4
	pgAmount := viewHeight - 1

	// 🔹 Ensure InitialIndex is visible
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

	// marquee tracking
	currentSelection := -1
	scrollOffset := 0
	lastScroll := time.Now()

	pageUp := func() {
		if viewStart == 0 {
			selectedItem = 0
		} else if (viewStart - pgAmount) < 0 {
			viewStart = 0
			selectedItem = 0
		} else {
			viewStart -= pgAmount
			selectedItem = viewStart
		}
		if selectedItem >= len(items) {
			selectedItem = len(items) - 1
		}
	}

	pageDown := func() {
		if len(items) <= viewHeight {
			return
		}
		if viewStart+viewHeight >= len(items) {
			selectedItem = len(items) - 1
		} else {
			viewStart += pgAmount
			if viewStart+viewHeight > len(items) {
				viewStart = len(items) - viewHeight
			}
			selectedItem = viewStart
		}
		if selectedItem >= len(items) {
			selectedItem = len(items) - 1
		}
	}

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

	for ch != gc.KEY_ESC {
		// reset scroll when selection changes
		if selectedItem != currentSelection {
			currentSelection = selectedItem
			scrollOffset = 0
			lastScroll = time.Now()
		}

		// advance marquee scroll every 200ms
		if time.Since(lastScroll) > 200*time.Millisecond {
			scrollOffset++
			lastScroll = time.Now()
		}

		// list items
		max := len(items) - viewStart
		if viewHeight < max {
			max = viewHeight
		}

		for i := 0; i < max; i++ {
			item := items[viewStart+i]
			display := item

			// Long lines: the highlighted one scrolls, the others end in
			// "...". Measured in bytes, as the screen library counts them,
			// but never cut in the middle of an accented or other
			// non-English letter (see cutBytes).
			if len(item) > viewWidth {
				if viewStart+i == selectedItem {
					// build marquee string with gap
					marquee := item + "   "
					marquee = marquee + marquee
					offset := scrollOffset % (len(item) + 3)
					display = cutBytes(marquee, offset, offset+viewWidth)
				} else {
					display = cutBytes(item, 0, viewWidth-3) + "..."
				}
			}

			if viewStart+i == selectedItem {
				win.ColorOn(1)
			}
			win.MovePrint(i+1, 2, strings.Repeat(" ", viewWidth))
			win.MovePrint(i+1, 2, display)
			win.ColorOff(1)
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

		// --- Scroll bar (old style patched in) ---
		scrollHeight := viewHeight
		if scrollHeight > 0 {
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

		// non-blocking read
		ch = readKey(win)

		// Esc (B) and Backspace always press Back, where there is one.
		if ch == gc.KEY_ESC || ch == gc.KEY_BACKSPACE || ch == 127 || ch == 8 {
			for i, b := range opts.Buttons {
				if strings.EqualFold(b, "Back") {
					return i, selectedItem, nil
				}
			}
		}

		if label, ok := opts.Shortcuts[ch]; ok {
			for i, b := range opts.Buttons {
				if b != "" && strings.EqualFold(b, label) {
					return i, selectedItem, nil
				}
			}
		}

		switch ch {
		case gc.KEY_DOWN:
			if opts.SnapToAction {
				selectedButton = opts.ActionButton
			}
			if selectedItem < len(items)-1 {
				selectedItem++
				settle(1)
			} else {
				selectedItem = 0 // past the bottom: back to the top
				settle(1)
			}
			showSelected()
		case gc.KEY_UP:
			if opts.SnapToAction {
				selectedButton = opts.ActionButton
			}
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
			if opts.SnapToAction {
				selectedButton = opts.ActionButton
			}
			pageUp()
			settle(1)
			showSelected()
		case gc.KEY_PAGEDOWN:
			if opts.SnapToAction {
				selectedButton = opts.ActionButton
			}
			pageDown()
			settle(-1)
			showSelected()
		case gc.KEY_ENTER, 10, 13:
			if selectedButton == opts.ActionButton {
				return selectedButton, selectedItem, nil
			} else if selectedButton < len(opts.Buttons) && opts.Buttons[selectedButton] == "PgUp" {
				pageUp()
				settle(1)
				showSelected()
			} else if selectedButton < len(opts.Buttons) && opts.Buttons[selectedButton] == "PgDn" {
				pageDown()
				settle(-1)
				showSelected()
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

	return -1, -1, nil
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
