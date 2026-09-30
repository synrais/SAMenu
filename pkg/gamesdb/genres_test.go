package gamesdb

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/synrais/SAMenu/pkg/config"
)

// genresForOld is the way a game's genres were worked out before
// GenreFinder: every folder of every game again, with the ini's [Genres]
// and [Genres.Files] additions. It's kept only for this test, which
// checks that GenreFinder, doing each folder once, gives the same genres.
func genresForOld(cfg *config.Config, menuPath, fileName string) []string {
	loadGenreNames()
	found := map[string]bool{}
	add := func(key string) {
		found[key] = true
		if p := GenreParent(key); p != "" {
			found[p] = true
		}
	}
	parts := strings.Split(strings.ReplaceAll(menuPath, "\\", "/"), "/")
	// The first part is the system, the last the file itself.
	if len(parts) > 2 {
		for _, folder := range parts[1 : len(parts)-1] {
			if key, ok := genreLookup[normaliseGenreFolder(folder)]; ok {
				add(key)
			}
			if cfg != nil {
				for g, pats := range cfg.GenreFolders {
					for _, p := range pats {
						if matchPattern(p, folder) || matchPattern(p, normaliseGenreFolder(folder)) {
							add(genreKey(g))
						}
					}
				}
			}
		}
	}
	if cfg != nil {
		for g, pats := range cfg.GenreFiles {
			for _, p := range pats {
				if matchPattern(p, fileName) || matchPattern(p, strings.TrimSuffix(fileName, fileExt(fileName))) {
					add(genreKey(g))
				}
			}
		}
	}
	if len(found) == 0 {
		return nil
	}
	out := make([]string, 0, len(found))
	for k := range found {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestGenreFinderUnchanged(t *testing.T) {
	cfg := &config.Config{
		GenreFolders: map[string][]string{"Party": {"*party*"}, "Sports/Golf": {"links*"}},
		GenreFiles:   map[string][]string{"Puzzle": {"*tetris*"}, "Racing": {"*kart*"}},
	}
	paths := []string{
		"SNES/Genres/Fighting/Street Fighter II.sfc",
		"SNES/Genres/Fighting/Tetris Attack.sfc",
		"SNES/2 Sports/Golf/Links Pro.sfc",
		"SNES/_Racing/Mario Kart.sfc",
		"SNES/[Puzzle]/Tetris.sfc",
		"PSX/Party Games.zip/Mario Party.chd",
		"PSX/Genres/Shoot'em up/R-Type.chd",
		"PSX/Game.chd",
		"Game.chd",
		"NES/Links Folder/Street Fighter II.nes",
	}
	nonEmpty := 0
	for _, c := range []*config.Config{nil, cfg} {
		finder := NewGenreFinder(c)
		for round := 0; round < 2; round++ { // the second round comes from the cache
			for _, p := range paths {
				name := p[strings.LastIndexByte(p, '/')+1:]
				want := genresForOld(c, p, name)
				got := finder.For(p, name)
				if len(want) > 0 {
					nonEmpty++
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s: got %v, want %v", p, got, want)
				}
			}
		}
	}
	if nonEmpty < 10 {
		t.Errorf("only %d paths had genres: the test isn't testing much", nonEmpty)
	}
}
