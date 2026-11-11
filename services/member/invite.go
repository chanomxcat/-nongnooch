package main

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
)

// InviteResult mirrors AgentService.ValidateInvite (contracts section 4).
type InviteResult struct {
	Valid               bool
	GuideID, CompanyID string
}

// InviteValidator checks an invite code. The demo uses DBInviteValidator (no agent service).
type InviteValidator interface {
	ValidateInvite(ctx context.Context, code string) (InviteResult, error)
}

var inviteRe = regexp.MustCompile(`^[A-Z0-9]{4,32}$`)

// NormalizeInvite strips spaces and dashes and upper-cases (contracts section 10).
func NormalizeInvite(s string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(s))
}

// DBInviteValidator reads the seeded demo_invite_codes table.
type DBInviteValidator struct{ DB *sql.DB }

func (v DBInviteValidator) ValidateInvite(ctx context.Context, code string) (InviteResult, error) {
	if !inviteRe.MatchString(code) {
		return InviteResult{}, nil
	}
	var r InviteResult
	err := v.DB.QueryRowContext(ctx, `SELECT guide_id, company_id FROM demo_invite_codes WHERE code = ?`, code).
		Scan(&r.GuideID, &r.CompanyID)
	if errors.Is(err, sql.ErrNoRows) {
		return InviteResult{}, nil
	}
	if err != nil {
		return InviteResult{}, err
	}
	r.Valid = true
	return r, nil
}
