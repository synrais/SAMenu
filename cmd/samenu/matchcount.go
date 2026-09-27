package main

import (
	"fmt"
	"sync"
	"time"
	"unicode"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/gamesdb"
)

// -------------------------
// Live match count (the search keyboard)
// -------------------------
//
// While you type a search, the number of games it would find shows on the
// text box's bottom edge ("1,284 matches", "1 match", "No matches"), from
// 3 letters or numbers on. It's counted in the background after a short
// pause in typing, with the same rules as the results list (including the
// menu's hidden tags when Search hides them), so it matches exactly.

const (
	countFrom  = 3                      // letters or numbers before counting
	countPause = 200 * time.Millisecond // typing pause before counting
)

type matchCounter struct {
	count func(query string) (int, error)
	mu    sync.Mutex
	gen   int    // bumped on every change: older counts are thrown away
	text  string // what status shows
}

// newMatchCounter counts what Search would find (with the menu's hidden
// tags, when Search hides them).
func newMatchCounter(cfg *config.Config) *matchCounter {
	return &matchCounter{count: func(query string) (int, error) {
		hide := cfg.Menu.SearchHidden && !menuHide.Empty()
		return gamesdb.CountNamesWords(query, func(name string) bool {
			return hide && menuHide.Matches(name)
		})
	}}
}

// newPatternCounter counts the games in files matching the patterns the
// typed text turns into (a custom genre's phrase, or Skip's words).
func newPatternCounter(files []MenuFile, patterns func(text string) []string) *matchCounter {
	return &matchCounter{count: func(text string) (int, error) {
		pats := patterns(text)
		n := 0
		for i := range files {
			if gamesdb.MatchesAny(pats, files[i]) {
				n++
			}
		}
		return n, nil
	}}
}

// changed is the keyboard's OnTextChange: count the new text after a pause.
func (c *matchCounter) changed(query string) {
	c.mu.Lock()
	c.gen++
	gen := c.gen
	if significant(query) < countFrom {
		c.text = "" // too short to be worth counting
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	go func() {
		time.Sleep(countPause)
		if c.stale(gen) {
			return // typed more meanwhile
		}
		n, err := c.count(query)
		if err != nil {
			return
		}
		c.mu.Lock()
		if c.gen == gen {
			c.text = matchesText(n)
		}
		c.mu.Unlock()
	}()
}

func (c *matchCounter) stale(gen int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen != gen
}

// status is the keyboard's Status: the latest count, or nothing.
func (c *matchCounter) status() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text
}

// significant counts a query's letters and numbers (not spaces or dots).
func significant(query string) int {
	n := 0
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}

func matchesText(n int) string {
	switch n {
	case 0:
		return "No matches"
	case 1:
		return "1 match"
	}
	return withCommas(n) + " matches"
}

// withCommas writes 1284 as "1,284".
func withCommas(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
