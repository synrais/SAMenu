package attract

import (
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
	"github.com/synrais/SAMenu/pkg/utils"
)

// Game picker
//
// How attract mode (and SAMenu's [Pick Random Game], and -random)
// choose a game, from [Attract] in SAMenu.ini:
//
//	Selection   who gets picked (SelectionModes):
//	  Per game         every game equally likely: big libraries dominate
//	  Per system       a random system, then a game: equal airtime
//	  Balanced         systems weighted by the square root of their size
//	  By category      a random category, then a system (balanced), then a game
//	  By manufacturer  the same with manufacturers
//	  Round robin      each system in turn (A-Z), a random game from each
//	  Time machine     each system in turn by release date, oldest first
//	  In order         every game in turn, system by system, A-Z
//	NoRepeats   every game plays once before any repeats (a shuffled deck
//	            per system)
//	MixSystems  never the same system twice in a row
//	OneVersion  one version per title: "Tetris (USA)", "Tetris (Europe)"
//	            and "Tetris (Japan)" count as one game (USA first), and a
//	            multi-disc game as one (disc 1)
//
// [Weights] multiplies a system's chance: "SNES = 3", "Computer = 0.5"
// (system IDs or groups; 0 = never).

// SelectionModes are the [Attract] Selection choices, in menu order.
var SelectionModes = []string{
	"Balanced", "Per game", "Per system", "By category", "By manufacturer",
	"Round robin", "Time machine", "In order",
}

// orderedModes play systems or games in turn, not at random.
var orderedModes = map[string]bool{"round robin": true, "time machine": true, "in order": true}

// Picker picks games from a pool.
type Picker struct {
	mode                  string // lowercase Selection
	noRepeats, mixSystems bool
	oneVersion            bool
	systems               []*sysPool // A-Z, or by release date for Time machine
	lastSystem            *sysPool
	cursor                int // next system in turn (Round robin, Time machine)
	title                 int // next title of the current system (In order)

	// pool is the list the picker was made from. It holds game numbers
	// into it, not copies, so the caller must not change it while the
	// picker's in use (make a new list instead, as attract mode does).
	pool    []gamesdb.FileInfo
	removed []bool // by game number
}

// sysPool is one system's games: their numbers in the pool, grouped by
// title (each title's best version first), and where each title starts.
// One flat list per system, rather than a little list per title, keeps the
// MiSTer's memory work down. Until organise has run, games is just the
// system's games in database order, with no titles yet.
type sysPool struct {
	id, category, maker string
	games               []int32 // game numbers, grouped by title once organised
	starts              []int32 // where each title begins in games
	organised           bool
	deck                []int // NoRepeats: titles left this round
	weight              float64
	mult                float64 // [Weights] for this system
}

func (sp *sysPool) numTitles() int { return len(sp.starts) }

// titleGames returns title t's game numbers, best version first.
func (sp *sysPool) titleGames(t int) []int32 {
	end := len(sp.games)
	if t+1 < len(sp.starts) {
		end = int(sp.starts[t+1])
	}
	return sp.games[sp.starts[t]:end]
}

// NewPicker organises files for picking with cfg's [Attract] settings. For a
// single pick (SAMenu) the in-turn modes don't apply, so they use
// Balanced.
func NewPicker(cfg *config.Config, files []gamesdb.FileInfo, singlePick bool) *Picker {
	a := cfg.Attract
	p := &Picker{
		mode:       strings.ToLower(strings.TrimSpace(a.Selection)),
		noRepeats:  a.NoRepeats,
		mixSystems: a.MixSystems,
		oneVersion: a.OneVersion,
		pool:       files,
		removed:    make([]bool, len(files)),
	}
	if p.mode == "" || (singlePick && orderedModes[p.mode]) {
		p.mode = "balanced"
	}

	// Only split the games into systems here (one cheap pass). Each
	// system's games are organised into titles the first time it's picked
	// (organise): the first game needs only its own system's, and the rest
	// get done as attract mode goes along. Games come grouped by system (the
	// database is built a system at a time), so a system is only looked up
	// when it changes.
	byID := map[string]*sysPool{}
	var cur *sysPool
	curID := "\x00"
	for i := range files {
		if id := files[i].SystemId; id != curID {
			curID = id
			key := strings.ToLower(id)
			cur = byID[key]
			if cur == nil {
				cur = &sysPool{id: id, category: "Other", maker: "Other"}
				if s, err := games.GetSystem(id); err == nil {
					cur.category = s.Category
					if s.Manufacturer != "" {
						cur.maker = s.Manufacturer
					}
				}
				byID[key] = cur
				p.systems = append(p.systems, cur)
			}
		}
		cur.games = append(cur.games, int32(i))
	}

	for _, sp := range p.systems {
		sp.mult = customWeight(cfg, sp.id, sp.category, sp.maker)
		// Until it's organised, a system counts its games as titles.
		p.setWeight(sp, len(sp.games))
	}

	sort.SliceStable(p.systems, func(i, j int) bool {
		a, b := p.systems[i], p.systems[j]
		if p.mode == "time machine" {
			da, db := releaseDate(a.id), releaseDate(b.id)
			if da != db {
				if da == "" || db == "" {
					return db == "" // undated last
				}
				return da < db
			}
		}
		return utils.LessFold(games.DisplayName(a.id), games.DisplayName(b.id))
	})
	return p
}

// setWeight sets a system's chance of being picked from its number of
// titles (and [Weights]).
func (p *Picker) setWeight(sp *sysPool, titles int) {
	n := float64(titles)
	switch p.mode {
	case "per game":
		sp.weight = n
	case "per system":
		sp.weight = 1
	default: // balanced, and systems within a group
		sp.weight = math.Sqrt(n)
	}
	sp.weight *= sp.mult
}

// organise sorts a system's games into titles (with OneVersion, a title's
// regions, revisions and discs together, best version first), once, the
// first time the system is picked, and sets its weight from its real
// number of titles.
func (p *Picker) organise(sp *sysPool) {
	if sp.organised {
		return
	}
	sp.organised = true
	files := p.pool

	// Pass 1: each game's title number.
	var index map[string]int32
	if p.oneVersion {
		index = make(map[string]int32, len(sp.games))
	}
	var counts []int32
	titleOf := make([]int32, len(sp.games))
	var keyBuf []byte // reused for each title's key
	for k, g := range sp.games {
		t := int32(len(counts)) // each game its own title, unless OneVersion
		if p.oneVersion {
			// Looking up by the reused buffer copies nothing; the key is
			// only copied into a string for a new title.
			keyBuf = appendTitleKey(keyBuf[:0], files[g].Name)
			if known, ok := index[string(keyBuf)]; ok {
				t = known
			} else {
				index[string(keyBuf)] = t
			}
		}
		if int(t) == len(counts) {
			counts = append(counts, 0)
		}
		counts[t]++
		titleOf[k] = t
	}

	// Pass 2: lay the games out title by title (each title in the order its
	// games came), best version first.
	sp.starts = make([]int32, len(counts))
	pos := make([]int32, len(counts))
	n := int32(0)
	for t, c := range counts {
		sp.starts[t], pos[t] = n, n
		n += c
	}
	laid := make([]int32, len(sp.games))
	for k, g := range sp.games {
		t := titleOf[k]
		laid[pos[t]] = g
		pos[t]++
	}
	sp.games = laid
	for t := range sp.starts {
		if v := sp.titleGames(t); len(v) > 1 {
			sortVersions(files, v)
		}
	}
	if p.mode == "in order" { // the only mode that uses the title order
		sortTitles(files, sp)
	}
	if sp.weight > 0 {
		p.setWeight(sp, sp.numTitles())
	}
}

// Remove takes a game out of the pool (blacklisted, or won't launch). It's
// rare, so it just looks through the pool for the path.
func (p *Picker) Remove(path string) {
	for i := range p.pool {
		if p.pool[i].Path == path {
			p.removed[i] = true
		}
	}
}

// Next picks the next game, or reports false when nothing is left.
func (p *Picker) Next() (gamesdb.FileInfo, bool) {
	if p.mode == "in order" {
		return p.nextInOrder()
	}
	for tries := 0; tries < 2*len(p.systems)+10; tries++ {
		sp := p.pickSystem()
		if sp == nil {
			return gamesdb.FileInfo{}, false
		}
		if f, ok := p.pickFrom(sp); ok {
			p.lastSystem = sp
			return f, true
		}
		sp.weight = 0 // nothing left in it
	}
	return gamesdb.FileInfo{}, false
}

// pickSystem chooses the system the next game comes from.
func (p *Picker) pickSystem() *sysPool {
	var live []*sysPool
	for _, sp := range p.systems {
		if sp.weight > 0 && p.hasGames(sp) {
			live = append(live, sp)
		}
	}
	if len(live) == 0 {
		return nil
	}

	// In turn: the next system with games.
	if p.mode == "round robin" || p.mode == "time machine" {
		for range p.systems {
			sp := p.systems[p.cursor%len(p.systems)]
			p.cursor++
			if sp.weight > 0 && p.hasGames(sp) {
				return sp
			}
		}
		return nil
	}

	// Never the same system twice in a row, if there's a choice.
	if p.mixSystems && len(live) > 1 && p.lastSystem != nil {
		for i, sp := range live {
			if sp == p.lastSystem {
				live = append(live[:i:i], live[i+1:]...)
				break
			}
		}
	}

	// By group: a random group first, then a system in it.
	if p.mode == "by category" || p.mode == "by manufacturer" {
		groups := map[string][]*sysPool{}
		var names []string
		for _, sp := range live {
			g := sp.category
			if p.mode == "by manufacturer" {
				g = sp.maker
			}
			if groups[g] == nil {
				names = append(names, g)
			}
			groups[g] = append(groups[g], sp)
		}
		sort.Strings(names)
		live = groups[names[rand.Intn(len(names))]]
	}

	total := 0.0
	for _, sp := range live {
		total += sp.weight
	}
	r := rand.Float64() * total
	for _, sp := range live {
		if r -= sp.weight; r < 0 {
			return sp
		}
	}
	return live[len(live)-1]
}

// pickFrom picks a title from a system (from its shuffled deck with
// NoRepeats), and that title's best version still in the pool.
func (p *Picker) pickFrom(sp *sysPool) (gamesdb.FileInfo, bool) {
	p.organise(sp)
	for refills := 0; refills < 2; refills++ {
		if !p.noRepeats {
			var live []int
			for i := 0; i < sp.numTitles(); i++ {
				if _, ok := p.bestVersion(sp.titleGames(i)); ok {
					live = append(live, i)
				}
			}
			if len(live) == 0 {
				return gamesdb.FileInfo{}, false
			}
			f, _ := p.bestVersion(sp.titleGames(live[rand.Intn(len(live))]))
			return f, true
		}
		for len(sp.deck) > 0 {
			i := sp.deck[len(sp.deck)-1]
			sp.deck = sp.deck[:len(sp.deck)-1]
			if f, ok := p.bestVersion(sp.titleGames(i)); ok {
				return f, true
			}
		}
		// Deck used up: a new round, freshly shuffled.
		sp.deck = rand.Perm(sp.numTitles())
	}
	return gamesdb.FileInfo{}, false
}

// nextInOrder is In order: every title of every system in turn, A-Z.
func (p *Picker) nextInOrder() (gamesdb.FileInfo, bool) {
	total := 0
	for _, sp := range p.systems {
		total += len(sp.games) // at least as many as titles
	}
	for step := 0; step < total+len(p.systems)+1; step++ {
		sp := p.systems[p.cursor%len(p.systems)]
		p.organise(sp)
		if p.title >= sp.numTitles() {
			p.cursor++
			p.title = 0
			continue
		}
		t := sp.titleGames(p.title)
		p.title++
		if sp.weight == 0 {
			continue
		}
		if f, ok := p.bestVersion(t); ok {
			return f, true
		}
	}
	return gamesdb.FileInfo{}, false
}

// bestVersion is a title's best version still in the pool.
func (p *Picker) bestVersion(versions []int32) (gamesdb.FileInfo, bool) {
	for _, g := range versions {
		if !p.removed[g] {
			return p.pool[g], true
		}
	}
	return gamesdb.FileInfo{}, false
}

func (p *Picker) hasGames(sp *sysPool) bool {
	for _, g := range sp.games {
		if !p.removed[g] {
			return true
		}
	}
	return false
}

// ----- titles and versions -----

// appendTitleKey appends titleKey(name) to buf, without making a string:
// plain-letter names are lowercased directly, others (accents and so on)
// the same way titleKey does.
func appendTitleKey(buf []byte, name string) []byte {
	if i := strings.IndexAny(name, "(["); i > 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	for i := 0; i < len(name); i++ {
		if name[i] >= 0x80 {
			return append(buf, strings.ToLower(name)...)
		}
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf = append(buf, c)
	}
	return buf
}

// titleKey is a game's title without its tags: "Tetris (USA) (Rev 1)" and
// "Tetris [!]" are both "tetris".
func titleKey(name string) string {
	if i := strings.IndexAny(name, "(["); i > 0 {
		name = name[:i]
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// regionOrder: earlier is preferred.
var regionOrder = []string{"usa", "world", "europe", "uk", "australia", "japan"}

// badTags make a version less wanted than a clean release.
var badTags = []string{"beta", "proto", "demo", "sample", "hack", "pirate", "unl", "[b"}

// versionScore ranks versions of a title: lower is better. Disc 1 first,
// then clean releases, then by region (USA, World, Europe, ...).
func versionScore(name string) int {
	l := strings.ToLower(name)
	score := 0
	if i := strings.Index(l, "disc "); i >= 0 && i+5 < len(l) && l[i+5] != '1' {
		score += 10000 // a later disc
	}
	for _, t := range badTags {
		if strings.Contains(l, t) {
			score += 1000
		}
	}
	region := len(regionOrder)
	for i, r := range regionOrder {
		if strings.Contains(l, r) {
			region = i
			break
		}
	}
	return score + region
}

// ----- systems -----

// customWeight multiplies all [Weights] entries that match a system: its ID,
// category and manufacturer. No entries = 1.
func customWeight(cfg *config.Config, id, category, maker string) float64 {
	w := 1.0
	for _, k := range []string{id, category, maker} {
		if v, ok := cfg.Weights[strings.ToLower(k)]; ok {
			w *= v
		}
	}
	return w
}

func releaseDate(id string) string {
	if s, err := games.GetSystem(id); err == nil {
		return s.ReleaseDate
	}
	return ""
}

// sortVersions puts a title's best version first (versionScore), working
// out each version's score once.
func sortVersions(pool []gamesdb.FileInfo, versions []int32) {
	type scored struct {
		game  int32
		score int
	}
	list := make([]scored, len(versions))
	for i, g := range versions {
		list[i] = scored{g, versionScore(pool[g].Name)}
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].score < list[b].score })
	for i, v := range list {
		versions[i] = v.game
	}
}

// sortTitles puts a system's titles A-Z (for In order), working out each
// title's key once.
func sortTitles(pool []gamesdb.FileInfo, sp *sysPool) {
	n := sp.numTitles()
	keys := make([]string, n)
	order := make([]int, n)
	for t := 0; t < n; t++ {
		keys[t], order[t] = titleKey(pool[sp.titleGames(t)[0]].Name), t
	}
	sort.SliceStable(order, func(a, b int) bool { return utils.LessFold(keys[order[a]], keys[order[b]]) })
	games := make([]int32, 0, len(sp.games))
	starts := make([]int32, 0, n)
	for _, t := range order {
		starts = append(starts, int32(len(games)))
		games = append(games, sp.titleGames(t)...)
	}
	sp.games, sp.starts = games, starts
}
