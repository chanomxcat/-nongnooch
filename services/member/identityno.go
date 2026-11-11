package main

import (
	"errors"
	"regexp"
	"strings"

	"nongnooch/platform/pkg/thaiid"
)

var (
	ErrInvalidNationality = errors.New("invalid_nationality")
	ErrIDNumberInvalid    = errors.New("id_number_invalid")
	ErrInvalidCountry     = errors.New("invalid_country")
)

const isoList = `AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ
CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR
GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP
KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT
MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW
SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG
UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW`

var isoCountries = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range strings.Fields(isoList) {
		m[c] = true
	}
	return m
}()

var passportRe = regexp.MustCompile(`^[A-Z0-9]{6,9}$`)

// Identity is a parsed, normalized ID document.
type Identity struct {
	Kind         string // "thai_id" | "passport"
	Number       string
	Country      string
	Last4        string
	ThaiVerified bool
}

// ParseIdentity validates the ID number for the nationality. Thai nationals must use a Thai ID.
func ParseIdentity(nationality, idNumber, passportCountry string) (Identity, error) {
	nat := strings.ToUpper(strings.TrimSpace(nationality))
	if !isoCountries[nat] {
		return Identity{}, ErrInvalidNationality
	}
	if nat == "TH" {
		if !thaiid.Valid(idNumber) {
			return Identity{}, ErrIDNumberInvalid
		}
		n := thaiid.Normalize(idNumber)
		return Identity{Kind: "thai_id", Number: n, Country: "TH", Last4: n[9:], ThaiVerified: true}, nil
	}
	n := strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(idNumber))
	if !passportRe.MatchString(n) {
		return Identity{}, ErrIDNumberInvalid
	}
	c := strings.ToUpper(strings.TrimSpace(passportCountry))
	if c == "" {
		c = nat
	}
	if !isoCountries[c] {
		return Identity{}, ErrInvalidCountry
	}
	return Identity{Kind: "passport", Number: n, Country: c, Last4: n[len(n)-4:]}, nil
}

// MaskID is the only form of an ID number allowed in logs.
func MaskID(last4 string) string { return "****" + last4 }
