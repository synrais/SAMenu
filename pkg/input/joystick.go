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
// MiSTer holds controllers exclusively while a core runs, so a device that
// stays open never receives events. But the kernel keeps every
// controller's *live* state (every button and axis), and says what it is
// when asked. So each poll asks, and compares the answer with the last.
//
// It asks the controller's event device (/dev/input/event*), kept open,
// with ioctls: its buttons in one call, and one more for each axis. Axis
// values are corrected the way the joystick device (js*) does it, so they
// read exactly as js* reports them. A controller with no event device is
// read by reopening js* each poll instead (opening it reports the live
// state), which costs more than twice as much.

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
	jsIOCGBTNMAP  = 0x84006a34 // 1024 bytes: the key code of each button (uint16s)
	jsIOCGCORR    = 0x80246a22 // each axis's correction, jsCorrSize bytes each
	jsCorrSize    = 36         // struct js_corr: 8 int32 coefficients, int16 prec, uint16 type
)

// Event device ioctls (see linux/input.h).
const (
	evIOCGKEY    = 0x80604518 // keyBytes: a bit for each key or button, set while down
	evIOCGABS    = 0x80184540 // + ABS code: absInfoBytes, the axis's value first (int32)
	keyBytes     = 96
	absInfoBytes = 24
)

// openRetry opens a device, trying again when the open is interrupted.
// While a core loads, the kernel can keep an open waiting, and Go's own
// scheduler signal then interrupts it (EINTR). That's not the device
// going away.
func openRetry(path string) (int, error) {
	for tries := 1; ; tries++ {
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != unix.EINTR || tries == 5 {
			return fd, err
		}
	}
}

func le32(b []byte) int32 {
	return int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
}

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
	// The state before last, reused for the next read's.
	spareBtn  []bool
	spareAxis []int8
	spareVals []int16

	every time.Duration // how often it's polled (for the stick hold)
	ev    *evReader     // its event device, or nil to reopen js* each poll
}

// evReader reads a controller's live state from its event device.
type evReader struct {
	fd      int
	keys    []uint16 // key code of each button
	corr    []jsCorr // the joystick device's correction of each axis
	keyBits []byte   // EVIOCGKEY's answer
	absInfo []byte   // EVIOCGABS's answer
}

// jsCorr is the joystick device's correction for an axis (struct js_corr).
type jsCorr struct {
	coef [4]int32 // the first 4 of its 8 (the rest are unused)
	typ  uint16   // 0 none, 1 "broken line" (the kernel's default)
}

// correct turns an event device axis value into the one the joystick
// device reports (joydev_correct in the kernel, int32 sums and all).
func (c jsCorr) correct(v int32) int16 {
	switch c.typ {
	case 0:
	case 1:
		switch {
		case v <= c.coef[0]:
			v = (c.coef[2] * (v - c.coef[0])) >> 14
		case v < c.coef[1]:
			v = 0
		default:
			v = (c.coef[3] * (v - c.coef[1])) >> 14
		}
	default:
		return 0
	}
	if v < -32767 {
		v = -32767
	} else if v > 32767 {
		v = 32767
	}
	return int16(v)
}

// openEvReader opens the event device belonging to the joystick device
// open as fd, with what it needs to read it as the joystick device would.
// nil if there isn't one or it can't be read.
func openEvReader(jsPath string, fd, buttons, axes int) *evReader {
	nodes, _ := filepath.Glob(filepath.Join("/sys/class/input", filepath.Base(jsPath), "device", "event*"))
	if len(nodes) != 1 {
		return nil
	}
	btnMap := make([]byte, 1024)
	if ioctlBytes(fd, jsIOCGBTNMAP, btnMap) != nil || 2*buttons > len(btnMap) {
		return nil
	}
	r := &evReader{keyBits: make([]byte, keyBytes), absInfo: make([]byte, absInfoBytes)}
	for i := 0; i < buttons; i++ {
		code := uint16(btnMap[2*i]) | uint16(btnMap[2*i+1])<<8
		if int(code)/8 >= keyBytes {
			return nil
		}
		r.keys = append(r.keys, code)
	}
	if axes > 0 {
		raw := make([]byte, jsCorrSize*axes)
		if ioctlBytes(fd, jsIOCGCORR, raw) != nil {
			return nil
		}
		for i := 0; i < axes; i++ {
			b := raw[i*jsCorrSize:]
			var c jsCorr
			for k := range c.coef {
				c.coef[k] = le32(b[4*k:])
			}
			c.typ = uint16(b[34]) | uint16(b[35])<<8
			r.corr = append(r.corr, c)
		}
	}
	evFd, err := openRetry(filepath.Join("/dev/input", filepath.Base(nodes[0])))
	if err != nil {
		return nil
	}
	r.fd = evFd
	return r
}

// read fills btn and vals with the live state (codes: each axis's ABS
// code).
func (r *evReader) read(codes []byte, btn []bool, vals []int16) error {
	if err := ioctlBytes(r.fd, evIOCGKEY, r.keyBits); err != nil {
		return err
	}
	for i, code := range r.keys {
		if i < len(btn) {
			btn[i] = r.keyBits[code/8]&(1<<(code%8)) != 0
		}
	}
	for i, code := range codes {
		if i >= len(vals) || i >= len(r.corr) {
			break
		}
		if err := ioctlBytes(r.fd, evIOCGABS+uintptr(code), r.absInfo); err != nil {
			return err
		}
		vals[i] = r.corr[i].correct(le32(r.absInfo))
	}
	return nil
}

// close lets go of the controller's event device, if it has one.
func (d *jsDevice) close() {
	if d.ev != nil {
		_ = unix.Close(d.ev.fd)
		d.ev = nil
	}
}

// refill is from copied into spare, grown if it's too small.
func refill[T any](spare, from []T) []T {
	if cap(spare) < len(from) {
		spare = make([]T, len(from))
	}
	spare = spare[:len(from)]
	copy(spare, from)
	return spare
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

func openJoystick(path string, sdl []*mappingEntry, every time.Duration) (*jsDevice, error) {
	name, bus, vid, pid, ver := jsInfo(path)
	if name == virtualinput.DeviceName {
		return nil, errSelf
	}
	fd, err := openRetry(path)
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
		every:   every,
		ev:      openEvReader(path, fd, int(nBtn[0]), axes),
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
	sticksOn    = true
	stickHoldMs = 75
)

// SetStickRules sets the analog stick rules for the detectors.
func SetStickRules(on bool, holdMs int) {
	sticksOn = on
	stickHoldMs = holdMs
}

// stickHoldReads is how many reads in a row a stick must stay pushed, on
// a controller read every so often: at least one.
func stickHoldReads(every time.Duration) int {
	if every <= 0 {
		every = jsPollEvery
	}
	ms := int(every / time.Millisecond)
	if stickHoldMs <= 0 || ms <= 0 {
		return 1
	}
	return (stickHoldMs + ms - 1) / ms
}

// An error from poll means the controller is gone. A read that was only
// interrupted (EINTR, while a core loads) is skipped as if it never
// happened, and the next poll tries again.
func (d *jsDevice) poll(out chan<- Event, buf []byte) error {
	// This read's state starts as the last one, in the spare copies (40
	// reads a second: no new memory for each).
	btn := refill(d.spareBtn, d.btn)
	axis := refill(d.spareAxis, d.axis)
	vals := refill(d.spareVals, d.vals)
	skip := func(err error) error {
		d.spareBtn, d.spareAxis, d.spareVals = btn, axis, vals
		if err == unix.EINTR {
			return nil
		}
		return err
	}

	if d.ev != nil {
		if err := d.ev.read(d.codes, btn, vals); err != nil {
			return skip(err)
		}
		for i, val := range vals {
			if i < len(axis) {
				axis[i] = axisSide(val)
			}
		}
	} else {
		fd, err := openRetry(d.path)
		if err != nil {
			return skip(err)
		}
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
					axis[num] = axisSide(val)
				}
			}
		}
		_ = unix.Close(fd)
	}
	holdReads := stickHoldReads(d.every)

	if d.have {
		for i, down := range btn {
			if down && !d.btn[i] {
				send(out, Event{Kind: "joystick", Device: d.name, Path: d.path, Name: d.layout.btnNames[i], Hint: faceButtonHints[d.layout.btnNames[i]]})
			} else if !down && d.btn[i] && (debug || releases) {
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
				if debug && !hat && sticksOn && d.runSide[i] != 0 && d.runSide[i] != d.rest[i] && d.run[i] < holdReads {
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
				case i < len(d.run) && d.run[i] < holdReads:
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
	d.spareBtn, d.spareAxis, d.spareVals = d.btn, d.axis, d.vals
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

// axisSide is the end an axis value is pushed to: -1, 0 or +1.
func axisSide(val int16) int8 {
	switch {
	case val <= -axisThreshold:
		return -1
	case val >= axisThreshold:
		return 1
	}
	return 0
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

// watchJoysticks polls every controller, every so often (jsPollEvery if
// 0), and follows them being plugged in and out.
func watchJoysticks(out chan<- Event, gate *Gate, every time.Duration) {
	if every <= 0 {
		every = jsPollEvery
	}
	sdl := loadSDLDB()
	devices := map[string]*jsDevice{}
	ignored := map[string]bool{} // SAMenu's own virtual pad
	drop := func(p string, d *jsDevice) {
		d.close()
		unregister(p)
		logf("- %s (%s)", d.name, filepath.Base(p))
		delete(devices, p)
	}

	rescan := func() {
		paths, _ := filepath.Glob("/dev/input/js*")
		present := map[string]bool{}
		for _, p := range paths {
			present[p] = true
			if devices[p] != nil || ignored[p] {
				continue
			}
			d, err := openJoystick(p, sdl, every)
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
				drop(p, d)
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
	tick := time.NewTicker(every)
	defer tick.Stop()

	for range tick.C {
		if wake := gate.waiting(); wake != nil {
			for p, d := range devices {
				drop(p, d)
			}
			<-wake
			lastScan = time.Time{} // find them again now
		}
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
				drop(p, d)
			}
		}
	}
}

// faceButtonHints say what each face button is called elsewhere. SDL's
// names (used in SAMenu.ini) are Xbox style, by position; MiSTer's are
// SNES style (its A is on the right, where SDL has b). Plain ASCII, as
// the menu's screens can't show other characters (the PlayStation symbols
// drawn with letters: X, O, [], /\), and short, so the lines don't scroll.
var faceButtonHints = map[string]string{
	"a": "MiSTer B, Xbox A, PS X",
	"b": "MiSTer A, Xbox B, PS O",
	"x": "MiSTer Y, Xbox X, PS []",
	"y": "MiSTer X, Xbox Y, PS /\\",
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
