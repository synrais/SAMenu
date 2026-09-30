package main

import (
	"testing"
	"time"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input"
)

func pad(path, name string, up bool) input.Event {
	return input.Event{Kind: "joystick", Path: path, Name: name, Up: up}
}

func key(name string, up bool) input.Event {
	return input.Event{Kind: "keyboard", Path: "/dev/hidraw1", Name: name, Up: up}
}

// fired reports whether the hold timer goes off within d.
func fired(h *hotkey, d time.Duration) bool {
	if h.holdDone == nil {
		return false
	}
	select {
	case <-h.holdDone:
		return true
	case <-time.After(d):
		return false
	}
}

func TestHotkeyHold(t *testing.T) {
	cfg := config.DefaultBackToMenu()
	cfg.HoldTime = 0.2
	h := newHotkey(cfg)

	// SDL's back is BIOS skip's select.
	h.event(pad("/dev/input/js0", "start", false))
	if h.holdDone != nil {
		t.Fatal("start alone started the hold")
	}
	h.event(pad("/dev/input/js0", "back", false))
	if !fired(h, 400*time.Millisecond) {
		t.Fatal("start + select held: no hold")
	}

	// Let go before the time is up: nothing.
	h.reset()
	h.event(pad("/dev/input/js0", "start", false))
	h.event(pad("/dev/input/js0", "back", false))
	h.event(pad("/dev/input/js0", "back", true))
	if h.holdDone != nil {
		t.Fatal("released, but the hold is still timing")
	}

	// Other buttons, sticks and the d-pad don't count.
	h.reset()
	h.event(pad("/dev/input/js0", "start", false))
	h.event(pad("/dev/input/js0", "a", false))
	h.event(input.Event{Kind: "joystick", Path: "/dev/input/js0", Name: "dpdown", Axis: true})
	if h.holdDone != nil {
		t.Fatal("start + another button started the hold")
	}
}

func TestHotkeyKeyTwice(t *testing.T) {
	h := newHotkey(config.DefaultBackToMenu())
	if h.event(key("`", false)) {
		t.Fatal("one press fired")
	}
	if h.event(key("`", true)) {
		t.Fatal("a release fired")
	}
	if !h.event(key("`", false)) {
		t.Fatal("second quick press didn't fire")
	}
	// Twice, but too slowly.
	h.reset()
	h.event(key("`", false))
	h.keyAt = time.Now().Add(-time.Second)
	if h.event(key("`", false)) {
		t.Fatal("two slow presses fired")
	}
	// Another key between doesn't matter; a different key never fires.
	h.reset()
	if h.event(key("a", false)) || h.event(key("a", false)) {
		t.Fatal("another key fired")
	}
	// No key set: never.
	cfg := config.DefaultBackToMenu()
	cfg.Key = ""
	h = newHotkey(cfg)
	if h.event(key("`", false)) || h.event(key("`", false)) {
		t.Fatal("fired with no key set")
	}
}
