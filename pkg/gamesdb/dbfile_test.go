package gamesdb

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"reflect"
	"testing"
)

func sampleGames() []FileInfo {
	return []FileInfo{
		{SystemId: "SNES", Name: "Chrono Trigger (USA)", Ext: "sfc", Path: "/media/fat/games/SNES/RPG/Chrono Trigger (USA).sfc",
			MenuDir: "SNES/RPG", Genres: []string{"RPG"}},
		{SystemId: "SNES", Name: "Secret of Mana (USA)", Ext: "sfc", Path: "/media/fat/games/SNES/RPG/Secret of Mana (USA).sfc",
			MenuDir: "SNES/RPG", Genres: []string{"RPG"}},
		{SystemId: "NES", Name: "Tetris", Ext: "nes", Path: "/media/fat/games/NES/All.zip/Tetris.nes", MenuDir: "NES"},
		{SystemId: "Amiga", Name: "Game", Ext: "", Path: "/media/fat/games/Amiga/Game", MenuDir: "Amiga"},
		{SystemId: "Amiga", Name: "Listed", Ext: "ags", Path: "/media/fat/games/Amiga/games.txt", MenuDir: "Amiga/Listed"},
		{SystemId: "Arcade", Name: "Pac-Man", Ext: "mra", Path: "/media/fat/_Arcade/Pac-Man.mra", MenuDir: "Arcade",
			Rotation: "vertical cw", Genres: []string{"Maze", "Classic"}},
		{SystemId: "PSX", Name: "Pokémon – Ōkami (Japan) [b]", Ext: "chd", Path: "/media/usb0/games/PSX/Pokémon – Ōkami (Japan) [b].chd",
			MenuDir: "Playstation", Genres: []string{"Sports", "Sports/Golf"}},
		{SystemId: "X", Name: "no folder", Ext: "", Path: "no folder"},
		{SystemId: "X", Name: ".hidden", Ext: "", Path: "/x/.hidden"}, // explicit: the build would split it otherwise
		{SystemId: "X", Name: "", Ext: "", Path: ""},
	}
}

func roundTrip(t *testing.T, files []FileInfo) []FileInfo {
	t.Helper()
	var buf bytes.Buffer
	if err := writeDB(&buf, files); err != nil {
		t.Fatal(err)
	}
	got, err := readDB(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestDBRoundTrip(t *testing.T) {
	files := sampleGames()
	got := roundTrip(t, files)
	if !reflect.DeepEqual(got, files) {
		for i := range files {
			if !reflect.DeepEqual(got[i], files[i]) {
				t.Errorf("game %d:\n got %+v\nwant %+v", i, got[i], files[i])
			}
		}
	}
	// Names point into the shared path text; games with the same genres
	// share one list.
	if dataOf(got[0].Name) != dataOf(got[0].Path)+uintptr(len("/media/fat/games/SNES/RPG/")) {
		t.Error("name doesn't point into the path")
	}
	if &got[0].Genres[0] != &got[1].Genres[0] {
		t.Error("genre lists not shared")
	}
	// A big made-up library comes back the same too.
	r := rand.New(rand.NewSource(1))
	var many []FileInfo
	for i := 0; i < 5000; i++ {
		f := files[r.Intn(len(files))]
		f.Name += string(rune('a' + r.Intn(26)))
		if f.Ext != "" {
			f.Path = f.Path[:len(f.Path)-len(f.Ext)-1] + string(f.Name[len(f.Name)-1]) + "." + f.Ext
		} else {
			f.Path += string(f.Name[len(f.Name)-1])
		}
		many = append(many, f)
	}
	if got := roundTrip(t, many); !reflect.DeepEqual(got, many) {
		t.Error("5,000 games didn't come back the same")
	}
	if got := roundTrip(t, nil); len(got) != 0 {
		t.Error("an empty database didn't come back empty")
	}
}

func TestDBDamaged(t *testing.T) {
	var buf bytes.Buffer
	if err := writeDB(&buf, sampleGames()); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	// Cut short anywhere: an error, never a crash.
	for n := 0; n < len(data); n++ {
		if _, err := readDB(data[:n]); err == nil {
			t.Fatalf("cut to %d of %d bytes: no error", n, len(data))
		}
	}
	// Bytes changed: never a crash (it may or may not notice).
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		bad := append([]byte(nil), data...)
		for k := 0; k < 1+r.Intn(4); k++ {
			bad[len(dbMagic)+r.Intn(len(bad)-len(dbMagic))] = byte(r.Intn(256))
		}
		_, _ = readDB(bad)
	}
	if _, err := readDB([]byte("something else")); err == nil {
		t.Error("not a games database: no error")
	}
}

func TestLegacyDB(t *testing.T) {
	old := []legacyFileInfo{
		{SystemId: "SNES", Name: "Chrono Trigger (USA)", Ext: "sfc", Path: "/media/fat/games/SNES/RPG/Chrono Trigger (USA).sfc",
			MenuPath: "SNES/RPG/Chrono Trigger (USA).sfc", Genres: []string{"RPG"}},
		{SystemId: "NES", Name: "Tetris", Ext: "nes", Path: "/media/fat/games/NES/Tetris.nes", MenuPath: "NES/Tetris.nes"},
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	got, err := readLegacyDB(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for i, o := range old {
		if got[i].MenuPath() != o.MenuPath || got[i].Path != o.Path || got[i].Name != o.Name ||
			got[i].Ext != o.Ext || !reflect.DeepEqual(got[i].Genres, o.Genres) {
			t.Errorf("game %d: %+v (menu path %q)", i, got[i], got[i].MenuPath())
		}
	}
	if got[0].MenuDir != "SNES/RPG" {
		t.Errorf("menu folder %q", got[0].MenuDir)
	}
}

func TestKeepOwnText(t *testing.T) {
	files := roundTrip(t, sampleGames())
	kept := append([]FileInfo(nil), files[:3]...)
	KeepOwnText(kept)
	for i := range kept {
		if !reflect.DeepEqual(kept[i], files[i]) {
			t.Errorf("game %d changed: %+v", i, kept[i])
		}
		if dataOf(kept[i].Path) == dataOf(files[i].Path) {
			t.Errorf("game %d still points into the database's text", i)
		}
	}
	if dataOf(kept[0].Name) != dataOf(kept[0].Path)+uintptr(len("/media/fat/games/SNES/RPG/")) {
		t.Error("name doesn't point into the new text")
	}
}
