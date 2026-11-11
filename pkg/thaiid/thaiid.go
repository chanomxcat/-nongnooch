// Package thaiid validates Thai national ID numbers (13 digits with check digit).
package thaiid

import "strings"

// Normalize strips every non-digit character.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Valid reports whether s (after Normalize) is 13 digits with a correct check digit.
func Valid(s string) bool {
	d := Normalize(s)
	if len(d) != 13 {
		return false
	}
	sum := 0
	for i := 0; i < 12; i++ {
		sum += int(d[i]-'0') * (13 - i)
	}
	return int(d[12]-'0') == (11-sum%11)%10
}
