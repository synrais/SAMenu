package gamesdb

import (
	"reflect"
	"testing"
	"unsafe"
)

func TestShareStrings(t *testing.T) {
	files := []FileInfo{
		{SystemId: "SNES", Name: "Chrono Trigger (USA)", Ext: "sfc", Path: "/media/fat/games/SNES/RPG/Chrono Trigger (USA).sfc",
			MenuPath: "SNES/RPG/Chrono Trigger (USA).sfc", Genres: []string{"RPG"}},
		{SystemId: "SNES", Name: "Secret of Mana (USA)", Ext: "sfc", Path: "/media/fat/games/SNES/RPG/Secret of Mana (USA).sfc",
			MenuPath: "SNES/RPG/Secret of Mana (USA).sfc", Genres: []string{"RPG"}},
		// Inside a zip, no extension, and a name that isn't the path's end:
		// all must come through unchanged.
		{SystemId: "NES", Name: "Tetris", Ext: "nes", Path: "/media/fat/games/NES/All.zip/Tetris.nes", MenuPath: "NES/Tetris.nes"},
		{SystemId: "Amiga", Name: "Game", Ext: "", Path: "/media/fat/games/Amiga/Game"},
		{SystemId: "Amiga", Name: "Listed", Ext: "ags", Path: "/media/fat/games/Amiga/games.txt"},
		{SystemId: "X", Name: "a", Ext: "b", Path: "b"}, // shorter than its name: left alone
	}
	want := make([]FileInfo, len(files))
	for i, f := range files {
		want[i] = f
		want[i].Genres = append([]string(nil), f.Genres...)
		if f.Genres == nil {
			want[i].Genres = nil
		}
	}
	// Separate copies, as the decoder makes them.
	files[1].SystemId = string([]byte("SNES"))
	files[1].Genres = []string{string([]byte("RPG"))}

	shareStrings(files)
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("changed:\n got %+v\nwant %+v", files, want)
	}
	if dataOf(files[0].SystemId) != dataOf(files[1].SystemId) {
		t.Error("system IDs not shared")
	}
	if &files[0].Genres[0] != &files[1].Genres[0] {
		t.Error("genre lists not shared")
	}
	if dataOf(files[0].Name) != dataOf(files[0].Path)+uintptr(len("/media/fat/games/SNES/RPG/")) {
		t.Error("name doesn't point into the path")
	}
}

// dataOf is where a string's text is in memory.
func dataOf(s string) uintptr { return (*reflect.StringHeader)(unsafe.Pointer(&s)).Data }

func TestBuildTreePointsAtEachGame(t *testing.T) {
	files := []FileInfo{
		{SystemId: "NES", Name: "Zelda", Ext: "nes", MenuPath: "NES/Zelda.nes"},
		{SystemId: "NES", Name: "Metroid", Ext: "nes", MenuPath: "NES/Action/Metroid.nes"},
		{SystemId: "NES", Name: "Contra", Ext: "nes", MenuPath: "NES/Action/Contra.nes"},
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
