// Package termtext cleans strings before they are written to a terminal.
package termtext

import "strings"

// Sanitize replaces C0 controls, C1 controls (U+0080–U+009F), and DEL with a
// space. The result has no byte that starts a terminal escape, and it keeps
// the surrounding text, including non-ASCII characters, in place.
func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return ' '
		}
		return r
	}, s)
}
