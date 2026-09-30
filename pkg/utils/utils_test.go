package utils

import (
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestListZip(t *testing.T) {
	contents, err := ListZip("testdata/test.zip")
	if err != nil {
		t.Error(err)
	}
	if len(contents) != 5 {
		t.Errorf("got %d files, want 5", len(contents))
	}
	if contents[0] != "f1" {
		t.Errorf("got %q, want f1", contents[0])
	}
	if _, err = ListZip("testdata/not_zip.zip"); err == nil {
		t.Error("expected error for non-zip file")
	}
}

func TestMapKeysSorted(t *testing.T) {
	// Map order is random, so the keys are sorted before comparing.
	var tests = []struct {
		m    map[string]int
		want []string
	}{
		{map[string]int{}, []string{}},
		{map[string]int{"a": 1}, []string{"a"}},
		{map[string]int{"a": 1, "b": 2}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		got := mapKeys(tt.m)
		sort.Strings(got)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("mapKeys(%v) = %v, want %v", tt.m, got, tt.want)
		}
	}
}

func TestLessFoldMatchesToLower(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	parts := []string{"a", "A", "b", "B", "z", "Z", " ", "(", "[", "_", "1", "9", "é", "É", "ß", "İ", "ǅ", "Ⅻ", "\xff"}
	gen := func() string {
		var sb strings.Builder
		for n := r.Intn(6); n > 0; n-- {
			sb.WriteString(parts[r.Intn(len(parts))])
		}
		return sb.String()
	}
	for i := 0; i < 200000; i++ {
		a, b := gen(), gen()
		if got, want := LessFold(a, b), lessFoldUnicode(a, b); got != want {
			t.Fatalf("LessFold(%q, %q) = %v, want %v", a, b, got, want)
		}
	}
}
