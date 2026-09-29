package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// ListZip returns a slice of all filenames in a zip file.
func ListZip(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var files []string
	for _, f := range r.File {
		files = append(files, f.Name)
	}

	return files, nil
}

func CopyFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outputFile, inputFile); err != nil {
		outputFile.Close()
		return err
	}
	// Sync and Close errors mean the copy may not have been written.
	if err := outputFile.Sync(); err != nil {
		outputFile.Close()
		return err
	}
	return outputFile.Close()
}

// mapKeys returns a list of all keys in a map.
func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return keys
}

func AlphaMapKeys[V any](m map[string]V) []string {
	keys := mapKeys(m)
	sort.Strings(keys)
	return keys
}

func RemoveFileExt(s string) string {
	return strings.TrimSuffix(s, filepath.Ext(s))
}

// ParseLine extracts (timestamp, path) from a gamelist entry.
// If no timestamp is present, ts = 0.
func ParseLine(line string) (float64, string) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx != -1 {
			tsStr := strings.TrimSpace(line[1:idx])
			path := strings.TrimSpace(line[idx+1:])
			if t, err := strconv.ParseFloat(tsStr, 64); err == nil {
				return t, path
			}
			// malformed timestamp: ignore, return as plain path
			return 0, path
		}
	}
	return 0, line
}

// --- Normalization helpers ---

// NormalizeTitle turns a game title into a comparison key: lowercase,
// Unicode-decomposed, letters and numbers only, single spaces. Nothing is
// treated as a file extension, so titles like "Dr. Mario" stay whole.
func NormalizeTitle(title string) string {
	name := norm.NFKD.String(strings.ToLower(title))

	// Walk runes: keep letters, numbers, spaces. Drop punctuation only.
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteRune(' ')
		}
	}

	// Collapse multiple spaces into one
	return strings.Join(strings.Fields(b.String()), " ")
}

// LessFold reports whether a sorts before b, ignoring case.
// Names that differ only in case fall back to exact order, so sorting is stable.
//
// It's the sort order of every list, so it runs hundreds of thousands of
// times as the menu starts. Plain-letter names (nearly all) are compared
// letter by letter, lowercasing as it goes, without making lowercase
// copies; the result is the same as comparing strings.ToLower copies.
func LessFold(a, b string) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ca, cb := a[i], b[i]
		if ca >= utf8.RuneSelf || cb >= utf8.RuneSelf {
			return lessFoldUnicode(a, b) // accents etc.: the full rules
		}
		ca, cb = LowerASCII(ca), LowerASCII(cb)
		if ca != cb {
			return ca < cb
		}
	}
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// lessFoldUnicode is LessFold for names with non-ASCII letters.
func lessFoldUnicode(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}

// LowerASCII lowercases an ASCII capital letter; any other byte is
// returned as it is.
func LowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// IsASCII reports whether s is plain ASCII.
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
