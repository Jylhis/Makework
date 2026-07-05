// Package terminal contains helpers for safe terminal output.
package terminal

import (
	"strings"
	"unicode"
)

// SanitizeText replaces terminal control and formatting characters with a
// visible replacement rune so attacker-influenced text cannot emit ANSI/OSC
// escape sequences or hidden bidi controls.
func SanitizeText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return '\uFFFD'
		}
		return r
	}, text)
}
