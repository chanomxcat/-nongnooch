package main

import "testing"

func TestNormalizeContact(t *testing.T) {
	cases := []struct {
		phone, email, want string
		err                error
	}{
		{"0812345678", "", "+66812345678", nil},
		{"+66 81 234 5678", "", "+66812345678", nil},
		{"66812345678", "", "+66812345678", nil},
		{"+86 138 0013 8000", "", "+8613800138000", nil},
		{"12345", "", "", ErrInvalidPhone},
		{"", "  Foo@Example.COM ", "foo@example.com", nil},
		{"", "not-an-email", "", ErrInvalidEmail},
		{"0812345678", "a@b.co", "", ErrContactRequired},
		{"", "", "", ErrContactRequired},
	}
	for _, c := range cases {
		got, err := NormalizeContact(c.phone, c.email)
		if err != c.err || got.Value != c.want {
			t.Errorf("(%q,%q) = %q,%v want %q,%v", c.phone, c.email, got.Value, err, c.want, c.err)
		}
	}
}
