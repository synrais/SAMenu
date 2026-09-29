package gamesdb

import (
	"math/rand"
	"strings"
	"testing"
)

// naturalCompareOld is naturalCompare as it was, lowercasing whole names
// first: the new one must order everything the same.
func naturalCompareOld(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				if len(na) < len(nb) {
					return -1
				}
				return 1
			}
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
			continue
		}
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case len(a)-i < len(b)-j:
		return -1
	case len(a)-i > len(b)-j:
		return 1
	}
	return 0
}

func TestNaturalCompareUnchanged(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	parts := []string{"a", "A", "b", "Z", " ", "(", "0", "1", "2", "9", "10", "007", "é", "É", "ß"}
	gen := func() string {
		var sb strings.Builder
		for n := r.Intn(7); n > 0; n-- {
			sb.WriteString(parts[r.Intn(len(parts))])
		}
		return sb.String()
	}
	for i := 0; i < 200000; i++ {
		a, b := gen(), gen()
		if got, want := naturalCompare(a, b), naturalCompareOld(a, b); got != want {
			t.Fatalf("naturalCompare(%q, %q) = %d, want %d", a, b, got, want)
		}
	}
}
