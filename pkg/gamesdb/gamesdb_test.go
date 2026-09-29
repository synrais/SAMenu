package gamesdb

import (
	"reflect"
	"testing"
	"unsafe"
)

// dataOf is where a string's text is in memory.
func dataOf(s string) uintptr { return (*reflect.StringHeader)(unsafe.Pointer(&s)).Data }

func TestBuildTreePointsAtEachGame(t *testing.T) {
	files := []FileInfo{
		{SystemId: "NES", Name: "Zelda", Ext: "nes", MenuDir: "NES"},
		{SystemId: "NES", Name: "Metroid", Ext: "nes", MenuDir: "NES/Action"},
		{SystemId: "NES", Name: "Contra", Ext: "nes", MenuDir: "NES/Action"},
	}
	tree := BuildTree(files, nil)
	nes := tree.Children["NES"]
	if len(nes.Files) != 1 || nes.Files[0] != &files[0] {
		t.Fatalf("NES folder: %+v", nes.Files)
	}
	act := nes.Children["Action"].Files
	if len(act) != 2 || act[0].Name != "Contra" || act[1].Name != "Metroid" || act[0] == act[1] {
		t.Fatalf("Action folder (sorted): %v, %v", act[0].Name, act[1].Name)
	}
	nes.AddAZFolder("[Games A-Z]")
	if az := nes.Children["[Games A-Z]"].Files; len(az) != 3 {
		t.Fatalf("A-Z folder has %d games", len(az))
	}
}
