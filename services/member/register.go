package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
)

var (
	ErrConsentRequired = errors.New("consent_required")
	ErrNameRequired    = errors.New("name_required")
	ErrIDNumberTaken   = errors.New("id_number_taken")
	ErrInviteInvalid   = errors.New("invite_invalid")
	ErrInviteDown      = errors.New("invite_unavailable")
	ErrNotFound        = errors.New("member_not_found")
)

var langs = map[string]bool{"th": true, "en": true, "zh": true, "ja": true, "ko": true, "de": true, "fr": true}

const consentVersion = "2027-01"

// MemberService holds registration logic.
type MemberService struct {
	DB            *sql.DB
	Cipher        *IDCipher
	Invites       InviteValidator
	WelcomePoints int
	InvitePoints  int
	Now           func() time.Time // test clock; nil = time.Now
}

type ConsentInput struct{ Terms, PDPA, Marketing bool }

type RegisterInput struct {
	ID                                                                    string // optional (seed uses fixed ids)
	Contact, FirstName, LastName, Nationality, IDNumber, PassportCountry string
	InviteCode, Lang, Channel, IP, DeviceID                               string
	Consents                                                              ConsentInput
}

// MemberView is the API shape of a member. It never carries the full ID number.
type MemberView struct {
	MemberID     string    `json:"memberId"`
	FirstName    string    `json:"firstName"`
	LastName     string    `json:"lastName"`
	Nationality  string    `json:"nationality"`
	IDKind       string    `json:"idKind"`
	IDLast4      string    `json:"idLast4"`
	ThaiVerified bool      `json:"thaiVerified"`
	Lang         string    `json:"lang"`
	Contact      string    `json:"contact"`
	InviteCode   string    `json:"inviteCode,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type RegisterResult struct {
	Member       MemberView `json:"member"`
	Existing     bool       `json:"existing,omitempty"`
	InviteStatus string     `json:"inviteStatus,omitempty"`
}

const memberCols = `id, COALESCE(phone, email), first_name, last_name, nationality, id_kind, id_last4, thai_verified, lang, COALESCE(invite_code,''), created_at`

func scanMember(row interface{ Scan(...any) error }) (MemberView, error) {
	var m MemberView
	err := row.Scan(&m.MemberID, &m.Contact, &m.FirstName, &m.LastName, &m.Nationality, &m.IDKind, &m.IDLast4,
		&m.ThaiVerified, &m.Lang, &m.InviteCode, &m.CreatedAt)
	return m, err
}

func contactColumn(contact string) string {
	if strings.Contains(contact, "@") {
		return "email"
	}
	return "phone"
}

// ByContact returns the live member for a contact (ErrNotFound when none).
func (s *MemberService) ByContact(ctx context.Context, contact string) (MemberView, error) {
	contact = strings.TrimSpace(contact)
	if strings.Contains(contact, "@") {
		contact = strings.ToLower(contact)
	}
	m, err := scanMember(s.DB.QueryRowContext(ctx,
		`SELECT `+memberCols+` FROM members WHERE `+contactColumn(contact)+` = ? AND deleted_at IS NULL`, contact))
	if errors.Is(err, sql.ErrNoRows) {
		return MemberView{}, ErrNotFound
	}
	return m, err
}

// Register creates a member (or returns the existing one for the contact) and gives welcome/invite points.
func (s *MemberService) Register(ctx context.Context, in RegisterInput) (RegisterResult, error) {
	if m, err := s.ByContact(ctx, in.Contact); err == nil {
		return RegisterResult{Member: m, Existing: true}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return RegisterResult{}, err
	}
	if !in.Consents.Terms || !in.Consents.PDPA {
		return RegisterResult{}, ErrConsentRequired
	}
	in.FirstName, in.LastName = strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName)
	if in.FirstName == "" || in.LastName == "" {
		return RegisterResult{}, ErrNameRequired
	}
	id, err := ParseIdentity(in.Nationality, in.IDNumber, in.PassportCountry)
	if err != nil {
		return RegisterResult{}, err
	}
	invite := NormalizeInvite(in.InviteCode)
	var inv InviteResult
	inviteStatus := "none"
	if invite != "" {
		r, err := s.Invites.ValidateInvite(ctx, invite)
		switch {
		case err != nil:
			return RegisterResult{}, ErrInviteDown
		case !r.Valid:
			return RegisterResult{}, ErrInviteInvalid
		}
		inv, inviteStatus = r, "applied"
	}
	lang := strings.ToLower(in.Lang)
	if !langs[lang] {
		lang = "en"
	}
	channel := in.Channel
	if channel != "app" && channel != "qr" {
		channel = "web"
	}
	memberID := in.ID
	if memberID == "" {
		u, err := uuid.NewV7()
		if err != nil {
			return RegisterResult{}, err
		}
		memberID = u.String()
	}
	enc, err := s.Cipher.Encrypt(id.Number, "member:"+memberID)
	if err != nil {
		return RegisterResult{}, err
	}
	now := time.Now().UTC()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return RegisterResult{}, err
	}
	defer tx.Rollback()
	var phone, email, inviteCol, guide, company any
	if contactColumn(in.Contact) == "email" {
		email = strings.ToLower(in.Contact)
	} else {
		phone = in.Contact
	}
	if inviteStatus == "applied" {
		inviteCol, guide, company = invite, inv.GuideID, inv.CompanyID
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO members(id, phone, email, first_name, last_name, nationality, id_kind, id_country, id_enc, id_hmac, id_last4,
		   thai_verified, lang, channel, invite_code, guide_id, company_id, invite_bound_at, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		memberID, phone, email, in.FirstName, in.LastName, strings.ToUpper(strings.TrimSpace(in.Nationality)), id.Kind, id.Country,
		enc, s.Cipher.Hash(id), id.Last4, id.ThaiVerified, lang, channel, inviteCol, guide, company,
		nullTime(inviteStatus == "applied", now), now)
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		tx.Rollback()
		// Same contact registered concurrently -> existing account; otherwise the ID number is taken.
		if m, e := s.ByContact(ctx, in.Contact); e == nil {
			return RegisterResult{Member: m, Existing: true}, nil
		}
		return RegisterResult{}, ErrIDNumberTaken
	}
	if err != nil {
		return RegisterResult{}, err
	}
	for _, c := range []struct {
		kind    string
		granted bool
	}{{"terms", true}, {"pdpa", true}, {"marketing", in.Consents.Marketing}} {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO consent_log(id, member_id, kind, granted, version, channel, ip, created_at) VALUES(?,?,?,?,?,?,?,?)`,
			uuid.NewString(), memberID, c.kind, c.granted, consentVersion, channel, nullStr(in.IP), now); err != nil {
			return RegisterResult{}, err
		}
	}
	if _, err := Append(ctx, tx, Entry{MemberID: memberID, DeltaPoints: s.WelcomePoints, Reason: "welcome",
		Status: "available", RefKind: "member", RefID: memberID}); err != nil {
		return RegisterResult{}, err
	}
	if inviteStatus == "applied" {
		if _, err := Append(ctx, tx, Entry{MemberID: memberID, DeltaPoints: s.InvitePoints, Reason: "invite",
			Status: "available", RefKind: "member", RefID: memberID}); err != nil {
			return RegisterResult{}, err
		}
	}
	payload := map[string]any{
		"memberId": memberID, "nationality": strings.ToUpper(strings.TrimSpace(in.Nationality)), "thaiVerified": id.ThaiVerified,
		"lang": lang, "channel": channel, "marketingConsent": in.Consents.Marketing,
		"ip": in.IP, "deviceId": in.DeviceID, "registeredAt": now,
	}
	if inviteStatus == "applied" {
		payload["inviteCode"], payload["guideId"], payload["companyId"] = invite, inv.GuideID, inv.CompanyID
	}
	ev, err := events.New(events.MemberRegistered, payload)
	if err != nil {
		return RegisterResult{}, err
	}
	if err := outbox.Write(ctx, tx, ev); err != nil {
		return RegisterResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RegisterResult{}, err
	}
	m, err := s.ByContact(ctx, in.Contact)
	return RegisterResult{Member: m, InviteStatus: inviteStatus}, err
}

func nullTime(ok bool, t time.Time) any {
	if !ok {
		return nil
	}
	return t
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
