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

func TestEntriesKept(t *testing.T) {
	var files []FileInfo
	for _, n := range []string{"Game 10", "Game 2", "The Zelda", "Aladdin", "FF7 (Disc 1)", "FF7 (Disc 2)"} {
		files = append(files, FileInfo{SystemId: "PSX", Name: n, Ext: "chd", MenuDir: "PSX"})
	}
	files = append(files, FileInfo{SystemId: "PSX", Name: "x", Ext: "chd", MenuDir: "PSX/Sub"})
	psx := BuildTree(files, nil).Children["PSX"]
	orders := []GameOrder{{}, {Natural: true}, {IgnoreThe: true}, {GroupDiscs: true}, {Natural: true, GroupDiscs: true}}
	for round := 0; round < 2; round++ {
		for _, o := range orders {
			for _, folders := range []string{"First", "Last", "Mixed"} {
				got := psx.Entries(o, folders)
				want := psx.entries(o, folders)
				if len(got) != len(want) {
					t.Fatalf("%+v %s: %d entries, want %d", o, folders, len(got), len(want))
				}
				for i := range got {
					if got[i].Folder != want[i].Folder || got[i].File != want[i].File || got[i].sortKey != want[i].sortKey {
						t.Fatalf("%+v %s: entry %d differs", o, folders, i)
					}
				}
			}
		}
	}
	// The same settings again: the kept listing itself.
	a := psx.Entries(GameOrder{Natural: true}, "First")
	b := psx.Entries(GameOrder{Natural: true}, "First")
	if &a[0] != &b[0] {
		t.Error("listing worked out again for the same settings")
	}
	psx.Forget()
	if c := psx.Entries(GameOrder{Natural: true}, "First"); &c[0] == &a[0] {
		t.Error("Forget kept the listing")
	}
	// Only the last few folders keep theirs.
	var nodes []*Node
	for i := 0; i < keptListings+3; i++ {
		n := newNode("f")
		n.Files = []*FileInfo{&files[0]}
		n.Entries(GameOrder{}, "First")
		nodes = append(nodes, n)
	}
	if len(recentListings) != keptListings || nodes[0].listed != nil || nodes[len(nodes)-1].listed == nil {
		t.Errorf("kept %d listings; oldest kept: %v", len(recentListings), nodes[0].listed != nil)
	}
	BuildTree(nil, nil)
	if len(recentListings) != 0 || nodes[len(nodes)-1].listed != nil {
		t.Error("a new tree didn't drop the kept listings")
	}
}
