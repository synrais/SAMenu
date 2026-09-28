package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/synrais/SAMenu/pkg/assets"
	"github.com/synrais/SAMenu/pkg/input/virtualinput"
)

// Controllers
//
// MiSTer holds controllers exclusively while a core runs, so a joystick
// device that stays open never receives events. But each time one is
// opened, the kernel reports its *live* state (every button and axis), so
// the detector reopens each controller every poll and compares that
// state with the last one.

const (
	jsPollEvery   = 25 * time.Millisecond
	jsRescanEvery = 2 * time.Second
	axisThreshold = 20000 // of 32767: how far an axis must move to count

	absHat0X = 0x10 // first d-pad ("hat") axis code; hats run to 0x17
	absHat3Y = 0x17
)

// -------- SDL DB parsing ----------

func le16(x int) int {
	return ((x & 0xFF) << 8) | ((x >> 8) & 0xFF)
}

func makeGUID(bus, vid, pid, version int) string {
	if vid == 0 || pid == 0 {
		return ""
	}
	return fmt.Sprintf("%02x000000%04x0000%04x0000%04x0000",
		bus&0xFF, le16(vid), le16(pid), le16(version))
}

type mappingEntry struct {
	guid     string
	name     string
	platform string
	mapping  map[string]string
}

func parseMappingLine(line string) *mappingEntry {
	parts := strings.Split(strings.TrimSpace(line), ",")
	if len(parts) < 3 {
		return nil
	}
	guid := strings.ToLower(parts[0])
	name := parts[1]
	items := parts[2:]
	mapping := map[string]string{}
	var platform string
	for _, item := range items {
		if !strings.Contains(item, ":") {
			continue
		}
		kv := strings.SplitN(item, ":", 2)
		k, v := kv[0], kv[1]
		if k == "platform" {
			platform = v
		} else {
			mapping[k] = v
		}
	}
	return &mappingEntry{guid: guid, name: name, platform: platform, mapping: mapping}
}

// Use embedded DB content instead of file
func loadSDLDB() []*mappingEntry {
	var entries []*mappingEntry
	content := assets.GameControllerDB

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if e := parseMappingLine(scanner.Text()); e != nil {
			entries = append(entries, e)
		}
	}
	return entries
}

// chooseMapping finds a controller's SDL database entry: its button names,
// the entry's name, and how closely it matched.
func chooseMapping(entries []*mappingEntry, bus, vid, pid, ver int) (map[string]string, string, string) {
	attempts := []struct {
		guid string
		msg  string
	}{
		{makeGUID(bus, vid, pid, ver), "exact match"},
		{makeGUID(0, vid, pid, ver), "close match, ignoring bus type"},
		{makeGUID(0, vid, pid, 0), "close match, ignoring bus type and version"},
	}

	for _, a := range attempts {
		if a.guid == "" {
			continue
		}
		g := strings.ToLower(a.guid)
		for _, e := range entries {
			if strings.ToLower(e.guid) == g && e.platform == "Linux" {
				return e.mapping, e.name, a.msg
			}
		}
	}
	return nil, "", ""
}

// -------- Device layout ----------

// Joystick ioctls (see linux/joystick.h).
const (
	jsIOCGAXES    = 0x80016a11
	jsIOCGBUTTONS = 0x80016a12
	jsIOCGAXMAP   = 0x80406a32 // 64 bytes: the ABS code of each axis
)

func ioctlBytes(fd int, req uintptr, buf []byte) error {
	_, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&buf[0])))
	if e != 0 {
		return e
	}
	return nil
}

// jsLayout names a controller's buttons and axis directions.
type jsLayout struct {
	btnNames []string    // per button
	axNames  [][2]string // per axis: [0] negative side, [1] positive side
}

func isHat(code byte) bool { return code >= absHat0X && code <= absHat3Y }

// hatName gives d-pad names to an unmapped hat axis: hat 0 is "dpup" etc,
// further hats "hat1up" etc.
func hatName(code byte, positive bool) string {
	n := int(code-absHat0X) / 2
	isY := (code-absHat0X)%2 == 1
	dir := map[[2]bool]string{
		{false, false}: "left", {false, true}: "right",
		{true, false}: "up", {true, true}: "down",
	}[[2]bool{isY, positive}]
	if n == 0 {
		return "dp" + dir
	}
	return fmt.Sprintf("hat%d%s", n, dir)
}

// buildLayout names buttons and axes from an SDL mapping (which may be
// nil), with raw names for anything the mapping doesn't cover.
//
// SDL numbers buttons the same way the joystick device does. It numbers
// axes the same way too, except that it leaves the hats (d-pads) out and
// refers to them as h0.1 etc, so its axis numbers are translated here.
func buildLayout(nButtons int, axisCodes []byte, mapping map[string]string) jsLayout {
	l := jsLayout{
		btnNames: make([]string, nButtons),
		axNames:  make([][2]string, len(axisCodes)),
	}
	var nonHat []int // SDL axis number -> joystick axis number
	for i, c := range axisCodes {
		if !isHat(c) {
			nonHat = append(nonHat, i)
		}
	}
	findAxis := func(code byte) int {
		for i, c := range axisCodes {
			if c == code {
				return i
			}
		}
		return -1
	}

	// Sorted, so the result doesn't depend on map order.
	friendly := make([]string, 0, len(mapping))
	for k := range mapping {
		friendly = append(friendly, k)
	}
	sort.Strings(friendly)

	for _, name := range friendly {
		raw := mapping[name]
		invert := strings.HasSuffix(raw, "~")
		raw = strings.TrimSuffix(raw, "~")
		half := 0 // -1 / +1 for "-a0" / "+a0"
		if strings.HasPrefix(raw, "+") {
			half, raw = 1, raw[1:]
		} else if strings.HasPrefix(raw, "-") {
			half, raw = -1, raw[1:]
		}
		// An output like "+leftx" maps only half of an axis.
		out := strings.TrimLeft(name, "+-")

		switch {
		case strings.HasPrefix(raw, "b"):
			if n, err := strconv.Atoi(raw[1:]); err == nil && n >= 0 && n < nButtons {
				l.btnNames[n] = out
			}
		case strings.HasPrefix(raw, "a"):
			k, err := strconv.Atoi(raw[1:])
			if err != nil || k < 0 || k >= len(nonHat) {
				continue
			}
			ax := nonHat[k]
			neg, pos := 0, 1
			if invert {
				neg, pos = 1, 0
			}
			switch half {
			case 0:
				l.axNames[ax][neg] = out + "-"
				l.axNames[ax][pos] = out + "+"
			case -1:
				l.axNames[ax][neg] = out
			case 1:
				l.axNames[ax][pos] = out
			}
		case strings.HasPrefix(raw, "h"):
			parts := strings.SplitN(raw[1:], ".", 2)
			if len(parts) != 2 {
				continue
			}
			hat, err1 := strconv.Atoi(parts[0])
			mask, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || hat < 0 || hat > 3 {
				continue
			}
			x := findAxis(byte(absHat0X + 2*hat))
			y := findAxis(byte(absHat0X + 2*hat + 1))
			switch {
			case mask&1 != 0 && y >= 0: // up
				l.axNames[y][0] = out
			case mask&2 != 0 && x >= 0: // right
				l.axNames[x][1] = out
			case mask&4 != 0 && y >= 0: // down
				l.axNames[y][1] = out
			case mask&8 != 0 && x >= 0: // left
				l.axNames[x][0] = out
			}
		}
	}

	// Raw names for whatever is left.
	for i := range l.btnNames {
		if l.btnNames[i] == "" {
			l.btnNames[i] = fmt.Sprintf("btn%d", i)
		}
	}
	for i, c := range axisCodes {
		for side := 0; side < 2; side++ {
			if l.axNames[i][side] != "" {
				continue
			}
			if isHat(c) {
				l.axNames[i][side] = hatName(c, side == 1)
			} else if side == 0 {
				l.axNames[i][side] = fmt.Sprintf("axis%d-", i)
			} else {
				l.axNames[i][side] = fmt.Sprintf("axis%d+", i)
			}
		}
	}
	return l
}

// -------- Devices ----------

type jsDevice struct {
	path   string
	name   string
	layout jsLayout

	btn   []bool // last state
	axis  []int8 // last side of each axis: -1, 0, +1
	rest  []int8 // side an axis rests on (triggers often rest at -32767), never reported
	codes []byte // ABS code of each axis
	have  bool   // a first state has been read
	// For the stick hold: the end each axis was last read at, and for how
	// many reads in a row.
	runSide []int8
	run     []int
	vals    []int16 // each axis's last raw value (the debug view's stick bars)
}

// jsInfo reads a controller's name and USB/Bluetooth IDs from sysfs.
func jsInfo(path string) (name string, bus, vid, pid, ver int) {
	sysdir := filepath.Join("/sys/class/input", filepath.Base(path), "device")
	readHex := func(f string) int {
		b, err := os.ReadFile(filepath.Join(sysdir, "id", f))
		if err != nil {
			return 0
		}
		v, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 16, 32)
		return int(v)
	}
	if b, err := os.ReadFile(filepath.Join(sysdir, "name")); err == nil {
		name = strings.TrimSpace(string(b))
	}
	return name, readHex("bustype"), readHex("vendor"), readHex("product"), readHex("version")
}

func openJoystick(path string, sdl []*mappingEntry) (*jsDevice, error) {
	name, bus, vid, pid, ver := jsInfo(path)
	if name == virtualinput.DeviceName {
		return nil, errSelf
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	var nAx, nBtn [1]byte
	axMap := make([]byte, 64)
	if err := ioctlBytes(fd, jsIOCGAXES, nAx[:]); err != nil {
		return nil, err
	}
	if err := ioctlBytes(fd, jsIOCGBUTTONS, nBtn[:]); err != nil {
		return nil, err
	}
	if err := ioctlBytes(fd, jsIOCGAXMAP, axMap); err != nil {
		return nil, err
	}
	axes := int(nAx[0])
	if axes > len(axMap) {
		axes = len(axMap)
	}

	var mapping map[string]string
	mapName, mapHow := "", ""
	if makeGUID(bus, vid, pid, ver) != "" {
		mapping, mapName, mapHow = chooseMapping(sdl, bus, vid, pid, ver)
	}
	d := &jsDevice{
		path:    path,
		name:    name,
		layout:  buildLayout(int(nBtn[0]), axMap[:axes], mapping),
		btn:     make([]bool, nBtn[0]),
		axis:    make([]int8, axes),
		rest:    make([]int8, axes),
		runSide: make([]int8, axes),
		run:     make([]int, axes),
		vals:    make([]int16, axes),
		codes:   append([]byte(nil), axMap[:axes]...),
	}
	info := "raw names (btn0, axis0+)"
	if mapName != "" {
		info = fmt.Sprintf("named: %s (%s)", mapName, mapHow)
	}
	register(DeviceInfo{Path: path, Kind: "joystick", Name: name, Info: info})
	if mapName != "" {
		logf("+ joystick: %s (%s), %d buttons, %d axes, named from SDL entry %q (%s)",
			name, filepath.Base(path), nBtn[0], axes, mapName, mapHow)
	} else {
		logf("+ joystick: %s (%s), %d buttons, %d axes, no SDL entry (raw names like btn0, axis0+)",
			name, filepath.Base(path), nBtn[0], axes)
	}
	return d, nil
}

var errSelf = fmt.Errorf("SAMenu's own virtual device")

// poll reads the controller's live state and sends an Event for each
// button newly pressed and each axis newly pushed to one side.
// Analog stick rules ([InputDetector] Sticks and StickHoldMs): whether
// sticks count, and for how many reads in a row a stick must stay pushed
// before it does. A glitch (an adapter's empty port jumping to one end for
// a read or two) then never counts. D-pads (hats) are always instant.
var (
	sticksOn       = true
	stickHoldReads = 3
)

// SetStickRules sets the analog stick rules for the detectors.
func SetStickRules(on bool, holdMs int) {
	sticksOn = on
	stickHoldReads = 1
	if holdMs > 0 {
		stickHoldReads = (holdMs + int(jsPollEvery/time.Millisecond) - 1) / int(jsPollEvery/time.Millisecond)
	}
}

func (d *jsDevice) poll(out chan<- Event, buf []byte) error {
	fd, err := unix.Open(d.path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	btn := make([]bool, len(d.btn))
	copy(btn, d.btn)
	axis := make([]int8, len(d.axis))
	copy(axis, d.axis)
	vals := make([]int16, len(d.vals))
	copy(vals, d.vals)

	for {
		n, err := unix.Read(fd, buf)
		if err != nil || n < 8 {
			break
		}
		for off := 0; off+8 <= n; off += 8 {
			val := int16(uint16(buf[off+4]) | uint16(buf[off+5])<<8)
			typ := buf[off+6] &^ 0x80 // drop the "initial state" flag
			num := int(buf[off+7])
			switch {
			case typ == 1 && num < len(btn):
				btn[num] = val != 0
			case typ == 2 && num < len(axis):
				if num < len(vals) {
					vals[num] = val
				}
				switch {
				case val <= -axisThreshold:
					axis[num] = -1
				case val >= axisThreshold:
					axis[num] = 1
				default:
					axis[num] = 0
				}
			}
		}
	}
	_ = unix.Close(fd)

	if d.have {
		for i, down := range btn {
			if down && !d.btn[i] {
				send(out, Event{Kind: "joystick", Device: d.name, Path: d.path, Name: d.layout.btnNames[i], Hint: faceButtonHints[d.layout.btnNames[i]]})
			} else if !down && d.btn[i] && debug {
				send(out, Event{Kind: "joystick", Device: d.name, Path: d.path, Name: d.layout.btnNames[i], Up: true})
			}
		}
		for i, raw := range axis {
			hat := i < len(d.codes) && isHat(d.codes[i])
			// How many reads in a row this axis has been at this end.
			if raw != 0 && i < len(d.run) && raw == d.runSide[i] {
				d.run[i]++
			} else if i < len(d.run) {
				// Debug view: a stick push that ended before the hold time
				// was up didn't count; say so.
				if debug && !hat && sticksOn && d.runSide[i] != 0 && d.runSide[i] != d.rest[i] && d.run[i] < stickHoldReads {
					d.sendAxis(out, i, d.runSide[i], "too short")
				}
				d.runSide[i], d.run[i] = raw, 1
				if debug && !hat && !sticksOn && raw != 0 && raw != d.rest[i] {
					d.sendAxis(out, i, raw, "sticks off")
				}
			}
			side := raw
			if !hat && raw != 0 {
				switch {
				case !sticksOn:
					side = 0 // sticks don't count
				case i < len(d.run) && d.run[i] < stickHoldReads:
					side = d.axis[i] // not held long enough yet
					if side != raw {
						side = 0
					}
				}
			}
			axis[i] = side
			if side != 0 && side != d.axis[i] && side != d.rest[i] {
				d.sendAxis(out, i, side, "")
			}
		}
	}
	if !d.have {
		// An axis already at one end on the first read is resting there
		// (a trigger), so that end isn't a press. D-pads never rest at an
		// end, so a d-pad held while a pad is plugged in still counts.
		for i, side := range axis {
			if i < len(d.rest) && (i >= len(d.codes) || !isHat(d.codes[i])) {
				d.rest[i] = side
			}
		}
	}
	d.btn, d.axis, d.vals, d.have = btn, axis, vals, true
	if debug {
		sticks := make([]Stick, len(vals))
		for i, v := range vals {
			name := strings.TrimRight(d.layout.axNames[i][0], "+-")
			hat := i < len(d.codes) && isHat(d.codes[i])
			sticks[i] = Stick{Name: name, Hat: hat, Value: v}
		}
		setPositions(d.path, sticks)
	}
	return nil
}

// sendAxis reports an axis pushed to one end (side -1 or +1); ignored
// says why it didn't count (debug view), or is empty.
func (d *jsDevice) sendAxis(out chan<- Event, i int, side int8, ignored string) {
	name := d.layout.axNames[i][0]
	if side > 0 {
		name = d.layout.axNames[i][1]
	}
	code := byte(0xFF)
	if i < len(d.codes) {
		code = d.codes[i]
	}
	send(out, Event{Kind: "joystick", Device: d.name, Path: d.path, Name: name,
		Hint: axisHint(name, code, side > 0), Ignored: ignored, Axis: true})
}

// watchJoysticks polls every controller and follows them being plugged
// in and out.
func watchJoysticks(out chan<- Event) {
	sdl := loadSDLDB()
	devices := map[string]*jsDevice{}
	ignored := map[string]bool{} // SAMenu's own virtual pad

	rescan := func() {
		paths, _ := filepath.Glob("/dev/input/js*")
		present := map[string]bool{}
		for _, p := range paths {
			present[p] = true
			if devices[p] != nil || ignored[p] {
				continue
			}
			d, err := openJoystick(p, sdl)
			if err == errSelf {
				ignored[p] = true
				continue
			}
			if err == nil {
				devices[p] = d
			}
		}
		for p, d := range devices {
			if !present[p] {
				unregister(p)
				logf("- %s (%s)", d.name, filepath.Base(p))
				delete(devices, p)
			}
		}
		for p := range ignored {
			if !present[p] {
				delete(ignored, p)
			}
		}
	}

	// Plug-in events arrive on their own goroutine, which only raises a
	// flag: the device list itself is only ever touched by this loop.
	hotplug := make(chan struct{}, 1)
	if inFd, err := unix.InotifyInit1(unix.IN_CLOEXEC); err == nil {
		if _, err := unix.InotifyAddWatch(inFd, "/dev/input", unix.IN_CREATE|unix.IN_DELETE); err == nil {
			go func() {
				buf := make([]byte, 4096)
				for {
					if n, err := unix.Read(inFd, buf); err != nil || n <= 0 {
						time.Sleep(time.Second)
						continue
					}
					select {
					case hotplug <- struct{}{}:
					default:
					}
				}
			}()
		}
	}

	rescan()
	lastScan := time.Now()
	buf := make([]byte, 8*(256+64))
	tick := time.NewTicker(jsPollEvery)
	defer tick.Stop()

	for range tick.C {
		scan := time.Since(lastScan) > jsRescanEvery
		select {
		case <-hotplug:
			scan = true
		default:
		}
		if scan {
			rescan()
			lastScan = time.Now()
		}
		for p, d := range devices {
			if err := d.poll(out, buf); err != nil {
				unregister(p)
				logf("- %s (%s)", d.name, filepath.Base(p))
				delete(devices, p)
			}
		}
	}
}

// faceButtonHints say what each face button is called elsewhere. SDL's
// names (used in SAMenu.ini) are Xbox style, by position; MiSTer's are
// SNES style (its A is on the right, where SDL has b). The PlayStation
// symbols are ones MiSTer's console font draws (its filled circle isn't,
// so the circle is hollow), and keep the lines short enough not to scroll.
var faceButtonHints = map[string]string{
	"a": "MiSTer B · Xbox A · PS ×",
	"b": "MiSTer A · Xbox B · PS ○",
	"x": "MiSTer Y · Xbox X · PS ■",
	"y": "MiSTer X · Xbox Y · PS ▲",
}

// axisHint describes an axis name in plain words: "lefty-" is "left stick
// up". Raw names on unknown pads ("axis1+") are described from the axis
// type where that's reliable (X/Y and RX/RY); negative Y is up.
func axisHint(name string, code byte, positive bool) string {
	named := map[string]string{
		"leftx-": "left stick left", "leftx+": "left stick right",
		"lefty-": "left stick up", "lefty+": "left stick down",
		"rightx-": "right stick left", "rightx+": "right stick right",
		"righty-": "right stick up", "righty+": "right stick down",
		"lefttrigger+": "left trigger", "righttrigger+": "right trigger",
	}
	if h, ok := named[name]; ok {
		return h
	}
	if !strings.HasPrefix(name, "axis") {
		return ""
	}
	dirs := map[byte][2]string{
		0x00: {"left", "right"},                         // X
		0x01: {"up", "down"},                            // Y
		0x03: {"right stick left", "right stick right"}, // RX
		0x04: {"right stick up", "right stick down"},    // RY
	}
	if d, ok := dirs[code]; ok {
		if positive {
			return d[1]
		}
		return d[0]
	}
	return ""
}
