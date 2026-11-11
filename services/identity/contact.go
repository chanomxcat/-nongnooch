package main

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

var (
	ErrInvalidPhone    = errors.New("invalid_phone")
	ErrInvalidEmail    = errors.New("invalid_email")
	ErrContactRequired = errors.New("contact_required")
)

// Contact is a normalized phone (E.164) or email (lower-case).
type Contact struct {
	Kind  string // "phone" | "email"
	Value string
	Lang  string // set by Verify from the OTP request
}

// NormalizeContact accepts exactly one of phone or email. Default phone region is TH.
func NormalizeContact(phone, email string) (Contact, error) {
	phone, email = strings.TrimSpace(phone), strings.TrimSpace(email)
	if (phone == "") == (email == "") {
		return Contact{}, ErrContactRequired
	}
	if phone != "" {
		n, err := phonenumbers.Parse(phone, "TH")
		if err != nil || !phonenumbers.IsValidNumber(n) {
			return Contact{}, ErrInvalidPhone
		}
		return Contact{Kind: "phone", Value: phonenumbers.Format(n, phonenumbers.E164)}, nil
	}
	a, err := mail.ParseAddress(email)
	if err != nil || a.Name != "" || !strings.Contains(a.Address, ".") {
		return Contact{}, ErrInvalidEmail
	}
	return Contact{Kind: "email", Value: strings.ToLower(a.Address)}, nil
}
