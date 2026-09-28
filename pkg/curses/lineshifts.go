package curses

// #cgo !darwin,!openbsd,!windows pkg-config: ncurses
// #include <curses.h>
// #include <term.h>
//
// static void no_line_shifts(void) {
// 	change_scroll_region = 0;
// 	insert_line = 0;
// 	delete_line = 0;
// 	parm_insert_line = 0;
// 	parm_delete_line = 0;
// 	scroll_forward = 0;
// 	scroll_reverse = 0;
// 	parm_index = 0;
// 	parm_rindex = 0;
// }
import "C"

// noLineShifts stops ncurses moving lines already on screen with the
// terminal's scroll commands (a newline in a scroll region, insert and
// delete line), so it rewrites them instead. On MiSTer's console a scroll
// moves every pixel of the area in software, reading them back from the
// framebuffer, which is far slower than writing the new lines: holding
// Down past the bottom of a long list stuttered.
func noLineShifts() {
	C.no_line_shifts()
}
