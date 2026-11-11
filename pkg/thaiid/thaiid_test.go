package thaiid

import "testing"

func TestValid(t *testing.T) {
	for _, c := range []struct {
		in string
		ok bool
	}{
		{"1101700230708", true}, {"1-1017-00230-70-8", true}, {"1101700230705", false},
		{"1101700230707", false}, {"110170023070", false}, {"abc", false}, {"", false},
	} {
		if got := Valid(c.in); got != c.ok {
			t.Errorf("Valid(%q)=%v want %v", c.in, got, c.ok)
		}
	}
}
