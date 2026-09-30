package config

import (
	"os"
	"sort"
	"strings"

	"gopkg.in/ini.v1"
)

// Attract playlists
//
// A playlist is a separate layer over attract mode's normal setup: it
// decides which games play by genre, while everything else (play time,
// detector, blacklists, orientation, skip tags) still applies. [Attract]
// Playlist picks the active one ("Normal" or empty = the usual setup).
//
//	[Playlist.Fighters Night]
//	Name   = Fighters Night
//	All    = Fighting, Beat 'em Up    ; genres for every system
//	SNES   = Fighting                 ; extra genres for one system
//	Saturn = Puzzle
//	Others = Leave out                ; systems with no genres: Leave out / As normal
//
// Genres are the names the games database uses: "Fighting", or a
// sub-genre as "Sports/Golf". A system with its own picks plays those plus
// the All ones. "As normal" systems follow [Attract] Include/Exclude as
// usual; while a playlist is on, Include/Exclude only matter for those.

const (
	OthersLeaveOut = "Leave out"
	OthersAsNormal = "As normal"
)

// Playlist is one [Playlist.X] section.
type Playlist struct {
	Name    string
	All     []string
	Systems map[string][]string // lower-case system ID -> genres
	Others  string
	// Skip: patterns (wildcards) for games this playlist never plays,
	// checked against the file name and the folders it's in. SystemSkip
	// ("SNES.Skip = ..." in the ini) only for one system.
	Skip       []string
	SystemSkip map[string][]string // lower-case system ID -> patterns
	// Exclude: systems (or groups) this playlist never plays, whatever
	// genres are picked, e.g. "Exclude = Computer, NES".
	Exclude []string
}

// SkipFor is the skip patterns for a system: the playlist's and its own.
func (p *Playlist) SkipFor(systemID string) []string {
	return append(append([]string(nil), p.Skip...), p.SystemSkip[strings.ToLower(systemID)]...)
}

// PicksFor is the genres a system plays: its own and the All ones.
func (p *Playlist) PicksFor(systemID string) []string {
	return append(append([]string(nil), p.All...), p.Systems[strings.ToLower(systemID)]...)
}

func splitGenres(v string) []string {
	var out []string
	for _, g := range strings.Split(v, ",") {
		if g = strings.TrimSpace(g); g != "" {
			out = append(out, g)
		}
	}
	return out
}

// loadPlaylists reads the [Playlist.X] sections.
func loadPlaylists(cfg *Config, file *ini.File) {
	cfg.Playlists = map[string]*Playlist{}
	for _, sec := range file.Sections() {
		rest, ok := strings.CutPrefix(strings.ToLower(sec.Name()), "playlist.")
		if !ok || rest == "" {
			continue
		}
		p := &Playlist{Name: strings.TrimSpace(sec.Key("name").String()), Systems: map[string][]string{},
			SystemSkip: map[string][]string{}, Others: OthersLeaveOut}
		if p.Name == "" {
			p.Name = sec.Name()[len("playlist."):]
		}
		for _, k := range sec.Keys() {
			switch key := strings.ToLower(k.Name()); key {
			case "name":
			case "all":
				p.All = splitGenres(k.String())
			case "skip":
				p.Skip = splitGenres(k.String())
			case "exclude":
				p.Exclude = splitGenres(k.String())
			case "others":
				if strings.EqualFold(strings.TrimSpace(k.String()), OthersAsNormal) {
					p.Others = OthersAsNormal
				}
			default:
				if sys, ok := strings.CutSuffix(key, ".skip"); ok {
					if pats := splitGenres(k.String()); len(pats) > 0 {
						p.SystemSkip[sys] = pats
					}
				} else if g := splitGenres(k.String()); len(g) > 0 {
					p.Systems[key] = g
				}
			}
		}
		cfg.Playlists[strings.ToLower(p.Name)] = p
	}
}

// ActivePlaylist is the playlist attract mode uses, or nil for Normal.
func (c *Config) ActivePlaylist() *Playlist {
	name := strings.TrimSpace(c.Attract.Playlist)
	if name == "" || strings.EqualFold(name, "Normal") {
		return nil
	}
	return c.Playlists[strings.ToLower(name)]
}

// PlaylistNames lists the playlists, sorted.
func (c *Config) PlaylistNames() []string {
	var names []string
	for _, p := range c.Playlists {
		names = append(names, p.Name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

// SystemIDCase, if set, gives a lower-case system ID its proper spelling
// ("snes" -> "SNES") when playlists are written. The games menu sets it
// (this package can't look systems up itself).
var SystemIDCase func(id string) string

func properID(id string) string {
	if SystemIDCase != nil {
		if s := SystemIDCase(id); s != "" {
			return s
		}
	}
	return id
}

// SavePlaylist writes a playlist's section afresh (so systems taken off it
// are gone from the file too).
func SavePlaylist(cfg *Config, p *Playlist) error {
	if err := RemoveSection(cfg.Path, "Playlist."+p.Name); err != nil {
		return err
	}
	values := [][2]string{{"Name", p.Name}, {"All", strings.Join(p.All, ", ")}}
	ids := make([]string, 0, len(p.Systems))
	for id := range p.Systems {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if len(p.Systems[id]) > 0 {
			values = append(values, [2]string{properID(id), strings.Join(p.Systems[id], ", ")})
		}
	}
	values = append(values, [2]string{"Others", p.Others}, [2]string{"Skip", strings.Join(p.Skip, ", ")},
		[2]string{"Exclude", strings.Join(p.Exclude, ", ")})
	skipIDs := make([]string, 0, len(p.SystemSkip))
	for id := range p.SystemSkip {
		skipIDs = append(skipIDs, id)
	}
	sort.Strings(skipIDs)
	for _, id := range skipIDs {
		if len(p.SystemSkip[id]) > 0 {
			values = append(values, [2]string{properID(id) + ".Skip", strings.Join(p.SystemSkip[id], ", ")})
		}
	}
	cfg.Playlists[strings.ToLower(p.Name)] = p
	return SaveValues(cfg.Path, "Playlist."+p.Name, values)
}

// DeletePlaylist removes a playlist, and makes Normal active if it was.
func DeletePlaylist(cfg *Config, name string) error {
	if err := RemoveSection(cfg.Path, "Playlist."+name); err != nil {
		return err
	}
	delete(cfg.Playlists, strings.ToLower(name))
	if strings.EqualFold(cfg.Attract.Playlist, name) {
		return SetActivePlaylist(cfg, "Normal")
	}
	return nil
}

// SetActivePlaylist saves [Attract] Playlist.
func SetActivePlaylist(cfg *Config, name string) error {
	cfg.Attract.Playlist = name
	return SaveValues(cfg.Path, "Attract", [][2]string{{"Playlist", name}})
}

// RemoveSection deletes a whole [section] (its header and lines) from an
// INI file, if it's there.
func RemoveSection(path, section string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	newline := "\n"
	if strings.Contains(string(data), "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var out []string
	skipping := false
	for _, line := range lines {
		if name, ok := sectionName(line); ok {
			skipping = strings.EqualFold(name, section)
		}
		if !skipping {
			out = append(out, line)
		}
	}
	if len(out) == len(lines) {
		return nil
	}
	for len(out) > 1 && strings.TrimSpace(out[len(out)-1]) == "" && strings.TrimSpace(out[len(out)-2]) == "" {
		out = out[:len(out)-1]
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(out, newline)), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DescribePlaylist sums a playlist up in one line, e.g. "All: Fighting;
// SNES: Puzzle; other systems left out". sysName, if given, turns system
// IDs into display names.
func DescribePlaylist(p *Playlist, sysName func(id string) string) string {
	var parts []string
	if len(p.All) > 0 {
		parts = append(parts, GenreLabels(p.All)+" from every system")
	}
	ids := make([]string, 0, len(p.Systems))
	for id := range p.Systems {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		name := id
		if sysName != nil {
			name = sysName(id)
		}
		parts = append(parts, name+": "+GenreLabels(p.Systems[id]))
	}
	if len(parts) == 0 {
		parts = append(parts, "no genres picked yet")
	}
	if len(p.Skip) > 0 {
		parts = append(parts, "skipping "+strings.Join(p.Skip, ", "))
	}
	if len(p.SystemSkip) > 0 {
		parts = append(parts, "plus per-system skips")
	}
	if len(p.Exclude) > 0 {
		names := make([]string, len(p.Exclude))
		for i, id := range p.Exclude {
			names[i] = id
			if sysName != nil {
				names[i] = sysName(id)
			}
		}
		parts = append(parts, "leaving out "+strings.Join(names, ", "))
	}
	switch {
	case len(p.All) > 0:
		// All genres apply to every system: no system is left over.
	case p.Others == OthersAsNormal:
		parts = append(parts, "systems with no picks as normal")
	default:
		parts = append(parts, "systems with no picks left out")
	}
	return strings.Join(parts, "; ")
}

// Custom genres ("Create custom genre..." in the menu's genre pickers) are
// a word or phrase instead of a genre: they pick every game with it in its
// name or folder. They're stored among the genres as the ini's usual
// "contains" pattern, e.g. "All = Fighting, *mario*".

// IsCustomGenre reports whether a genre pick is a custom one ("*mario*").
func IsCustomGenre(g string) bool {
	g = strings.TrimSpace(g)
	return len(g) > 2 && strings.HasPrefix(g, "*") && strings.HasSuffix(g, "*") &&
		!strings.Contains(g[1:len(g)-1], "*")
}

// CustomGenre is the pick for a typed word or phrase: "Mario" -> "*mario*".
func CustomGenre(words string) string {
	return "*" + strings.ToLower(strings.Join(strings.Fields(words), " ")) + "*"
}

// GenreLabel shows a genre pick: a custom one as its words, first letter
// capitalised ("*mario*" -> "Mario"), a real one as it is.
func GenreLabel(g string) string {
	if !IsCustomGenre(g) {
		return g
	}
	w := strings.TrimSpace(g)
	w = w[1 : len(w)-1]
	for i, r := range w {
		return strings.ToUpper(string(r)) + w[i+len(string(r)):]
	}
	return w
}

// GenreLabels shows a list of genre picks.
func GenreLabels(picks []string) string {
	out := make([]string, len(picks))
	for i, g := range picks {
		out[i] = GenreLabel(g)
	}
	return strings.Join(out, ", ")
}
