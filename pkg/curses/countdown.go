package curses

import (
	"fmt"
	"time"

	gc "github.com/rthornton128/goncurses"
)

// CountdownChoice asks a question with buttons, and picks the default
// button by itself when the time runs out. It's for a change the person
// may not be able to see (a picture setting the screen can't show): left
// alone, or answered blind, it goes back. Left and right move between the
// buttons, Enter chooses, Back picks the default.
func CountdownChoice(stdscr *gc.Window, title, text string, buttons []string, def int, secs int) int {
	lines := wrapText(text, 56)
	width := 60
	win, err := NewWindow(stdscr, len(lines)+6, width, title, 250)
	if err != nil {
		return def
	}
	defer win.Delete()
	gc.Cursor(0)

	selected := def
	end := time.Now().Add(time.Duration(secs) * time.Second)
	for {
		left := int(time.Until(end).Seconds() + 0.999)
		if left <= 0 {
			return def
		}
		for i, l := range lines {
			win.MovePrint(1+i, 2, l)
		}
		note := fmt.Sprintf("%s in %d s ", buttons[def], left)
		win.MovePrint(len(lines)+2, 2, fmt.Sprintf("%-*s", width-4, note))
		DrawActionButtons(win, buttons, selected)
		gc.Update()

		switch readKey(win) {
		case gc.KEY_LEFT:
			if selected > 0 {
				selected--
			}
		case gc.KEY_RIGHT:
			if selected < len(buttons)-1 {
				selected++
			}
		case gc.KEY_ENTER, 10, 13:
			return selected
		case gc.KEY_ESC, gc.KEY_BACKSPACE, 127, 8:
			return def
		}
	}
}
