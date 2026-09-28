package curses

import (
	"os"
	"strings"

	gc "github.com/rthornton128/goncurses"
)

type Coords struct {
	Y int
	X int
}

type SetupWindowError struct {
	Ctx error
}

func (e *SetupWindowError) Error() string {
	return e.Ctx.Error()
}

func Setup() (*gc.Window, error) {
	// Esc on its own (B on a controller) should register straight away,
	// not after ncurses' default one second wait for an escape sequence.
	if os.Getenv("ESCDELAY") == "" {
		_ = os.Setenv("ESCDELAY", "25")
	}
	stdscr, err := gc.Init()
	if err != nil {
		return nil, err
	}

	gc.Echo(false)
	gc.CBreak(true)
	gc.Cursor(0)

	gc.StartColor()
	gc.InitPair(1, gc.C_BLACK, gc.C_WHITE)

	return stdscr, nil
}

func NewWindow(stdscr *gc.Window, height int, width int, title string, timeout int) (*gc.Window, error) {
	return NewWindowAt(stdscr, -1, height, width, title, timeout)
}

// NewWindowAt is NewWindow with the window's top row given (-1 = centred
// vertically).
func NewWindowAt(stdscr *gc.Window, top, height, width int, title string, timeout int) (*gc.Window, error) {
	rows, cols := stdscr.MaxYX()
	// Never bigger than the screen (ncurses refuses a window that doesn't
	// fit), and never off its edge.
	height, width = FitSize(stdscr, height, width)
	y, x := (rows-height)/2, (cols-width)/2
	if top >= 0 {
		y = top
	}
	if y+height > rows {
		y = rows - height
	}
	if y < 0 {
		y = 0
	}
	if x < 0 {
		x = 0
	}

	var win *gc.Window
	win, err := gc.NewWindow(height, width, y, x)
	if err != nil {
		return nil, &SetupWindowError{Ctx: err}
	}
	win.Keypad(true)
	win.Timeout(timeout)

	win.Erase()
	win.NoutRefresh()

	win.Box(gc.ACS_VLINE, gc.ACS_HLINE)
	if len(title) > 0 {
		titleX := (width - len(title)) / 2
		win.MovePrint(0, titleX, title)
	}
	win.NoutRefresh()

	return win, nil
}

func DrawBox(win *gc.Window, y int, x int, height int, width int) {
	win.HLine(y, x+1, gc.ACS_HLINE, width-1)
	win.HLine(height, x+1, gc.ACS_HLINE, width-1)
	win.VLine(y+1, x, gc.ACS_VLINE, height-1)
	win.VLine(y+1, x+width-1, gc.ACS_VLINE, height-1)
	win.MoveAddChar(y, x, gc.ACS_ULCORNER)
	win.MoveAddChar(y, x+width-1, gc.ACS_URCORNER)
	win.MoveAddChar(height, x, gc.ACS_LLCORNER)
	win.MoveAddChar(height, x+width-1, gc.ACS_LRCORNER)
	win.NoutRefresh()
}

func DrawActionButtons(win *gc.Window, buttons []string, selected int) {
	height, width := win.MaxYX()

	// Draw horizontal separator
	win.HLine(height-3, 1, gc.ACS_HLINE, width-2)
	win.MoveAddChar(height-3, 0, gc.ACS_LTEE)
	win.MoveAddChar(height-3, width-1, gc.ACS_RTEE)

	// Clear the whole row where buttons will sit
	win.MovePrint(height-2, 1, strings.Repeat(" ", width-2))

	// Build button texts, with 3-space gaps, or 1 if that won't fit
	buttonTexts := make([]string, len(buttons))
	textWidth, shown := 0, 0
	for i, button := range buttons {
		buttonTexts[i] = "<" + button + ">"
		if button != "" {
			textWidth += len(buttonTexts[i])
			shown++
		}
	}
	gap := 3
	if shown > 1 && textWidth+gap*(shown-1) > width-2 {
		gap = 1
	}
	totalWidth := textWidth
	if shown > 1 {
		totalWidth += gap * (shown - 1)
	}

	// Center the whole row. A button with no label (a line where pressing
	// does nothing) isn't shown.
	x := (width - totalWidth) / 2
	for i, text := range buttonTexts {
		if buttons[i] == "" {
			continue
		}
		if i == selected {
			win.ColorOn(1)
		}
		win.MovePrint(height-2, x, text)
		win.ColorOff(1)
		x += len(text) + gap
	}

	win.NoutRefresh()
}

func InfoBox(stdscr *gc.Window, title string, text string, clear bool, ok bool) error {
	if clear {
		stdscr.Erase()
		stdscr.NoutRefresh()
		gc.Update()
	}

	// Wrap the text to the screen (and at any line breaks in it), and
	// size the box to fit it.
	rows, cols := stdscr.MaxYX()
	lines := wrapText(text, cols-8)
	if len(lines) > rows-4 && rows > 4 {
		lines = lines[:rows-4]
	}
	width := len(title) + 4
	for _, l := range lines {
		if len(l)+4 > width {
			width = len(l) + 4
		}
	}

	win, err := NewWindow(stdscr, len(lines)+2, width, title, -1)
	if err != nil {
		return err
	}
	defer win.Delete()

	gc.Cursor(0)

	for i, l := range lines {
		win.MovePrint(1+i, 2, l)
	}

	win.NoutRefresh()
	gc.Update()

	if ok {
		win.GetChar()
	}

	return nil
}

// SwapConfirmBack is the Japanese button layout: B confirms and A backs
// out. MiSTer sends a controller's A as Enter and B as Esc, so the two
// keys are swapped (a keyboard's Enter and Esc swap too).
var SwapConfirmBack bool

// Injected are keys from outside the terminal: controller buttons MiSTer
// doesn't pass on as keys (Start, Select, L2...), which SAMenu reads with
// its input detectors and hands to the menu as the action they're mapped
// to. A list takes one whenever no real key is waiting.
var Injected = make(chan gc.Key, 16)

// readKey reads a key, applying the button layout.
func readKey(win *gc.Window) gc.Key {
	k := win.GetChar()
	if k == 0 {
		select {
		case k = <-Injected:
			return k
		default:
		}
	}
	if SwapConfirmBack {
		switch k {
		case gc.KEY_ESC:
			return 10
		case gc.KEY_ENTER, 10, 13:
			return gc.KEY_ESC
		}
	}
	return k
}

// FitSize limits a window size to the screen.
func FitSize(stdscr *gc.Window, height, width int) (int, int) {
	rows, cols := stdscr.MaxYX()
	if height > rows {
		height = rows
	}
	if width > cols {
		width = cols
	}
	return height, width
}

// wrapText splits text at its line breaks and wraps each line at word
// boundaries to at most width characters.
func wrapText(text string, width int) []string {
	if width < 10 {
		width = 10
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, w := range strings.Fields(para) {
			for len(w) > width { // a word too long for a line is split
				if line != "" {
					out = append(out, line)
					line = ""
				}
				out = append(out, w[:width])
				w = w[width:]
			}
			switch {
			case line == "":
				line = w
			case len(line)+1+len(w) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		out = append(out, line)
	}
	return out
}
