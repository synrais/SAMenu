package virtualinput

import (
	"strings"
)

// ToKeyboardCode looks up a key by name: "a", "{f9}" or "f9" (named keys
// work with or without braces).
func ToKeyboardCode(name string) (int, bool) {
	if v, ok := KeyboardMap[name]; ok { // direct reference, no prefix
		return v, ok
	}
	if v, ok := KeyboardMap["{"+strings.Trim(name, "{}")+"}"]; ok {
		return v, ok
	}
	return 0, false
}
