package gamesdb

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/synrais/SAMenu/pkg/config"
)

// Genres
//
// A game's genres come from the folders it sits in, as the menu shows them
// (zip folders included): "SNES/Genres/Fighting/Street Fighter II.sfc" is
// Fighting. A folder only counts when its whole name is a genre name
// (genrenames.go), so "Street Fighter II" as a folder doesn't. Names are
// compared after tidying: numbers, underscores and brackets in front
// ("2 Genres", "_Fighter", "[Sports]"), a zip folder's ".zip", capitals.
// A sub-genre ("Sports/Golf") also counts as its genre ("Sports").
// [Genres] and [Genres.Files] in the ini add your own folder and file
// names.

var genreLeading = regexp.MustCompile(`^[\d_\-\s.\[(]+`)

// normaliseGenreFolder tidies a folder name for comparing with genre names.
func normaliseGenreFolder(f string) string {
	f = strings.ToLower(strings.TrimSpace(f))
	f = strings.TrimSuffix(f, ".zip")
	f = genreLeading.ReplaceAllString(f, "")
	f = strings.Trim(f, "[]_ ")
	f = strings.ReplaceAll(f, "’", "'")
	return strings.Join(strings.Fields(f), " ")
}

var (
	genreOnce   sync.Once
	genreLookup map[string]string // tidied folder name -> "Genre" or "Genre/Sub"
	genreKeys   map[string]string // lower-case key -> key as written
)

func loadGenreNames() {
	genreOnce.Do(func() {
		genreLookup, genreKeys = map[string]string{}, map[string]string{}
		for key, names := range genreFolders {
			genreKeys[strings.ToLower(key)] = key
			for _, n := range names {
				genreLookup[normaliseGenreFolder(n)] = key
			}
		}
	})
}

// genreKey turns a genre as typed in the ini into the dictionary's own
// spelling, or tidies a new one ("my genre" -> "My Genre").
func genreKey(k string) string {
	loadGenreNames()
	k = strings.Trim(strings.TrimSpace(k), "/")
	if known, ok := genreKeys[strings.ToLower(k)]; ok {
		return known
	}
	parts := strings.Split(k, "/")
	for i, p := range parts {
		words := strings.Fields(p)
		for j, w := range words {
			words[j] = strings.ToUpper(w[:1]) + w[1:]
		}
		parts[i] = strings.Join(words, " ")
	}
	return strings.Join(parts, "/")
}

// GenreParent is a sub-genre's genre ("Sports/Golf" -> "Sports"), or ""
// for a genre.
func GenreParent(key string) string {
	if i := strings.Index(key, "/"); i >= 0 {
		return key[:i]
	}
	return ""
}

// GenresFor works out a game's genres from its menu path and file name,
// with the ini's [Genres] / [Genres.Files] additions.
func GenresFor(cfg *config.Config, menuPath, fileName string) []string {
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

func fileExt(name string) string {
	if i := strings.LastIndex(name, "."); i > 0 {
		return name[i:]
	}
	return ""
}

// HasGenre reports whether a game is in a genre ("Sports" matches any
// sports sub-genre too, since those also carry "Sports").
// InPick reports whether a game is in a playlist's genre pick: it has
// that genre, or, for a custom genre ("*mario*"), the word is in its name
// or folder (the same matching as a playlist's Skip).
func (f FileInfo) InPick(pick string) bool {
	if config.IsCustomGenre(pick) {
		return f.containsWord(strings.TrimSpace(pick))
	}
	return f.HasGenre(pick)
}

// containsWord is MatchesAny for one "contains" pattern ("*mario*"), done
// the quick way: the menu path ("System/Folder/Game.ext") holds the game's
// folders and file name, so one scan of it (after the system) covers them
// all, without making any new text. It runs for every game when a playlist
// has a custom genre, so on the MiSTer that matters.
func (f FileInfo) containsWord(pattern string) bool {
	word := strings.ToLower(pattern[1 : len(pattern)-1]) // spaces kept, as the general matcher does
	if word == "" || f.MenuPath == "" || strings.Contains(word, "/") {
		return MatchesAny([]string{pattern}, f) // unusual: the general way
	}
	path := f.MenuPath
	if i := strings.IndexByte(path, '/'); i >= 0 {
		path = path[i+1:] // not the system's own name
	}
	return containsFold(path, word)
}

// containsFold reports whether s contains the lowercase word, ignoring
// case. Plain-letter text is compared as it is; text with accents and so
// on is lowercased first, the same as the general matcher does.
func containsFold(s, word string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return strings.Contains(strings.ToLower(s), word)
		}
	}
	for i := 0; i+len(word) <= len(s); i++ {
		match := true
		for j := 0; j < len(word); j++ {
			c := s[i+j]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != word[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (f FileInfo) HasGenre(key string) bool {
	for _, g := range f.Genres {
		if strings.EqualFold(g, key) {
			return true
		}
	}
	return false
}
