package input

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Input detection
//
// Watches keyboards, mice and controllers and reports each press as an
// Event. Nothing acts on the events yet; attract mode and the games
// menu's -inputs test mode just print them.
//
// While a core is running, MiSTer holds the normal input devices
// exclusively (the Linux "grab"), so ordinary reads see nothing. Each
// detector works around that:
//   - keyboards and mice are read as raw HID data (hidraw), which the
//     grab doesn't affect
//   - controllers are polled, asking the kernel for their live state,
//     which it knows even while they're grabbed (see joystick.go)

// Event is one detected press.
type Event struct {
	Kind   string // "keyboard", "mouse" or "joystick"
	Device string // the device's name
	Path   string // the device itself (/dev/input/js0, /dev/hidraw1): names can repeat
	Name   string // what was pressed, as used in SAMenu.ini ("a", "dpleft", "leftx-", ...)
	Hint   string // plain-English meaning, when Name isn't obvious ("left stick up")
	Axis   bool   // a stick or d-pad direction, not a button or key
	// Only with SetDebug(true) (the input test screen):
	Up      bool   // a release, not a press
	Ignored string // why this input didn't count ("too short", "sticks off")
}

func (e Event) String() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s (%s): %s (%s)", e.Kind, e.Device, e.Name, e.Hint)
	}
	return fmt.Sprintf("%s (%s): %s", e.Kind, e.Device, e.Name)
}

// Options picks which detectors run ([InputDetector] in SAMenu.ini).
type Options struct {
	Keyboard bool
	Mouse    bool
	Joystick bool
	Quiet    bool // don't print devices being found or lost
	// JoystickEvery is how often controllers are read: 0 for the usual
	// 25 ms. Something that only needs to know a controller was used at
	// all (the idle watcher) can read them less often and cost less.
	JoystickEvery time.Duration
	// Gate, if set, can pause these detectors (see Gate).
	Gate *Gate
}

// Gate pauses a set of detectors: paused, they let go of every device and
// read nothing until resumed, then find the devices again.
type Gate struct {
	mu     sync.Mutex
	paused bool
	wake   chan struct{} // closed on Resume
}

// Pause stops the detectors reading (within a couple of seconds).
func (g *Gate) Pause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused {
		g.paused, g.wake = true, make(chan struct{})
	}
}

// Resume starts them again, reporting whether they were paused.
func (g *Gate) Resume() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused {
		return false
	}
	g.paused = false
	close(g.wake)
	return true
}

// waiting is what a paused detector waits on, or nil when it isn't paused
// (or has no gate).
func (g *Gate) waiting() <-chan struct{} {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.paused {
		return g.wake
	}
	return nil
}

// Start runs the enabled detectors in the background and returns the
// channel their events arrive on. If nobody reads the channel fast
// enough, events are dropped rather than holding the detectors up.
func Start(opts Options) <-chan Event {
	quiet = opts.Quiet
	out := make(chan Event, 64)
	if opts.Keyboard || opts.Mouse {
		go watchHID(out, opts.Keyboard, opts.Mouse, opts.Gate)
	}
	if opts.Joystick {
		go watchJoysticks(out, opts.Gate, opts.JoystickEvery)
	}
	return out
}

func send(out chan<- Event, ev Event) {
	select {
	case out <- ev:
	default:
	}
}

var quiet bool

func logf(format string, args ...interface{}) {
	if quiet {
		return
	}
	fmt.Printf("[Input] "+format+"\n", args...)
}

// cleanName turns a label like "PAGE UP" into a SAMenu.ini style name
// ("pageup").
func cleanName(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "")
}

// -------------------------
// Debug view (the input test screen)
// -------------------------

// debug makes the detectors also report releases and ignored inputs, and
// keep stick positions. Only the input test screen turns it on; attract
// mode never sees these.
var debug bool

// SetDebug turns the debug view on or off.
func SetDebug(on bool) { debug = on }

// DeviceInfo is a device the detectors are watching.
type DeviceInfo struct {
	Path string // /dev/input/js0, /dev/hidraw1...
	Kind string // "joystick", "keyboard", "mouse", "keyboard + mouse"
	Name string
	Info string // how a controller's buttons were named
}

var (
	regMu     sync.Mutex
	registry  = map[string]DeviceInfo{}
	positions = map[string][]Stick{}
)

func register(d DeviceInfo) {
	regMu.Lock()
	registry[d.Path] = d
	regMu.Unlock()
}

func unregister(path string) {
	regMu.Lock()
	delete(registry, path)
	delete(positions, path)
	regMu.Unlock()
}

// Devices lists the devices being watched, controllers first.
func Devices() []DeviceInfo {
	regMu.Lock()
	defer regMu.Unlock()
	out := make([]DeviceInfo, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Kind == "joystick") != (out[j].Kind == "joystick") {
			return out[i].Kind == "joystick"
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// Stick is one axis of a controller: its name (as in SAMenu.ini, without
// the + or -), whether it's a d-pad, and where it is (-32768..32767).
type Stick struct {
	Name  string
	Hat   bool
	Value int16
}

// Sticks is a controller's axes as last read (debug view only).
func Sticks(path string) []Stick {
	regMu.Lock()
	defer regMu.Unlock()
	return append([]Stick(nil), positions[path]...)
}

// StickLine is how far (of 32767) a stick must go to count as pushed.
func StickLine() int { return axisThreshold }

func setPositions(path string, s []Stick) {
	regMu.Lock()
	positions[path] = s
	regMu.Unlock()
}
