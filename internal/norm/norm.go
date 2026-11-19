// Package norm turns names and texts into comparison keys: lowercase, no
// accents, words separated by single spaces.
package norm

import "strings"

var accents = strings.NewReplacer(
	"é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a", "î", "i", "ï", "i",
	"ô", "o", "ö", "o", "û", "u", "ù", "u", "ü", "u", "ç", "c",
	"á", "a", "ã", "a", "å", "a", "í", "i", "ì", "i", "ó", "o", "ò", "o", "õ", "o", "ø", "o",
	"ú", "u", "ñ", "n", "ÿ", "y", "ß", "ss", "œ", "oe", "æ", "ae",
)

// Words lowercases s, drops accents and turns every other character run into
// one space, padded with spaces: " free mobile mandat ". A word is then found
// with strings.Contains(Words(text), Words(word)).
func Words(s string) string {
	s = accents.Replace(strings.ToLower(s))
	var b strings.Builder
	b.WriteByte(' ')
	space := true
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
	}
	return b.String()
}

// Key is the comparison key of a name: "OVH cloud" → "ovhcloud".
func Key(s string) string { return strings.ReplaceAll(strings.TrimSpace(Words(s)), " ", "") }

// Clip shortens s to n characters, the last one an ellipsis.
func Clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
