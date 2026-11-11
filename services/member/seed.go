package main

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

var seedNS = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// Fixed ids for the demo invite code (stand-in for the agent service guide and company).
var (
	demoGuideID   = uuid.NewSHA1(seedNS, []byte("guide:+66822222222")).String()
	demoCompanyID = uuid.NewSHA1(seedNS, []byte("company:Demo Tours")).String()
)

// Seed inserts the demo members (contracts section 7) and invite code. Idempotent.
func (s *MemberService) Seed(ctx contextT) error {
	if _, err := s.DB.ExecContext(ctx,
		`INSERT IGNORE INTO demo_invite_codes(code, guide_id, company_id) VALUES('DEMOGUIDE1', ?, ?)`,
		demoGuideID, demoCompanyID); err != nil {
		return err
	}
	all := consents()
	for _, in := range []RegisterInput{
		{Contact: "+66811111111", FirstName: "Somchai", LastName: "Jaidee", Nationality: "TH", IDNumber: "1101700230708", Lang: "th"},
		{Contact: "+8613800138000", FirstName: "Wei", LastName: "Zhang", Nationality: "CN", IDNumber: "E12345678", PassportCountry: "CN", Lang: "zh"},
		{Contact: "taro@example.jp", FirstName: "Taro", LastName: "Yamada", Nationality: "JP", IDNumber: "TK1234567", PassportCountry: "JP", Lang: "ja"},
	} {
		in.ID = uuid.NewSHA1(seedNS, []byte("member:"+in.Contact)).String()
		in.Consents, in.Channel = all, "web"
		if _, err := s.Register(ctx, in); err != nil {
			return err
		}
	}
	return nil
}

func consents() ConsentInput { return ConsentInput{Terms: true, PDPA: true} }

func (a *API) seed(w http.ResponseWriter, r *http.Request) {
	if !a.DemoMode {
		http.NotFound(w, r)
		return
	}
	if err := a.Svc.Seed(r.Context()); err != nil {
		slog.ErrorContext(r.Context(), "seed failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
