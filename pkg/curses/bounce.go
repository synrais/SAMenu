package curses

import "time"

// Long lines that don't fit bounce: they hold on their start, slide left
// until their end shows, hold there, then slide back (faster) and start
// again. Only one continuous piece of the line is ever shown, and both
// ends stay still long enough to read.
const (
	bounceHold    = time.Second            // on the start, and on the end
	bounceForward = 200 * time.Millisecond // a character, sliding left
	bounceBack    = 100 * time.Millisecond // a character, sliding back
)

// BounceOffset is where a line of n characters, shown width at a time,
// starts after t (the time since it began showing).
func BounceOffset(n, width int, t time.Duration) int {
	span := n - width
	if span <= 0 || t < 0 {
		return 0
	}
	forward := time.Duration(span) * bounceForward
	back := time.Duration(span) * bounceBack
	t %= bounceHold + forward + bounceHold + back
	switch {
	case t < bounceHold:
		return 0
	case t < bounceHold+forward:
		return int((t - bounceHold) / bounceForward)
	case t < 2*bounceHold+forward:
		return span
	default:
		return span - int((t-2*bounceHold-forward)/bounceBack)
	}
}
