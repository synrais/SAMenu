package curses

import (
	"testing"
	"time"
)

func TestBounceOffset(t *testing.T) {
	ms := time.Millisecond
	// 30 characters in 20: 10 to travel. Hold 1s, 10 steps of 200ms,
	// hold 1s, 10 steps of 100ms back: a 5s round trip.
	for _, c := range []struct {
		at   time.Duration
		want int
	}{
		{0, 0}, {999 * ms, 0}, // holding on the start
		{1000 * ms, 0}, {1200 * ms, 1}, {1999 * ms, 4}, {2800 * ms, 9}, // sliding left
		{3000 * ms, 10}, {3999 * ms, 10}, // holding on the end
		{4000 * ms, 10}, {4100 * ms, 9}, {4900 * ms, 1}, // sliding back, faster
		{5000 * ms, 0}, {5999 * ms, 0}, {6200 * ms, 1}, // and round again
	} {
		if got := BounceOffset(30, 20, c.at); got != c.want {
			t.Errorf("at %v: offset %d, want %d", c.at, got, c.want)
		}
	}
	if got := BounceOffset(20, 20, 3*time.Second); got != 0 {
		t.Errorf("a line that fits moved: offset %d", got)
	}
	// Never past either end.
	for at := time.Duration(0); at < 20*time.Second; at += 10 * ms {
		if o := BounceOffset(30, 20, at); o < 0 || o > 10 {
			t.Fatalf("at %v: offset %d is outside 0-10", at, o)
		}
	}
}
