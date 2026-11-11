package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	memberv1 "nongnooch/platform/gen/go/member/v1"
)

type contextT = context.Context

// GRPCServer implements memberv1.MemberServiceServer.
type GRPCServer struct {
	memberv1.UnimplementedMemberServiceServer
	Svc *MemberService
}

func toProto(m MemberView) *memberv1.Member {
	return &memberv1.Member{MemberId: m.MemberID, Lang: m.Lang, Nationality: m.Nationality,
		ThaiVerified: m.ThaiVerified, Name: m.FirstName + " " + m.LastName, IdLast4: m.IDLast4}
}

func (g *GRPCServer) GetMemberByContact(ctx context.Context, req *memberv1.GetMemberByContactRequest) (*memberv1.Member, error) {
	m, err := g.Svc.ByContact(ctx, req.Contact)
	if errors.Is(err, ErrNotFound) {
		return nil, status.Error(codes.NotFound, "member_not_found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "internal")
	}
	return toProto(m), nil
}

func (g *GRPCServer) GetContactPrefs(ctx context.Context, req *memberv1.GetContactPrefsRequest) (*memberv1.ContactPrefs, error) {
	var p memberv1.ContactPrefs
	var phone, email sql.NullString
	err := g.Svc.DB.QueryRowContext(ctx, `SELECT lang, phone, email FROM members WHERE id = ? AND deleted_at IS NULL`, req.MemberId).
		Scan(&p.Lang, &phone, &email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "member_not_found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "internal")
	}
	p.Phone, p.Email = phone.String, email.String
	var granted sql.NullBool
	_ = g.Svc.DB.QueryRowContext(ctx,
		`SELECT granted FROM consent_log WHERE member_id = ? AND kind='marketing' ORDER BY created_at DESC, id DESC LIMIT 1`,
		req.MemberId).Scan(&granted)
	p.MarketingConsent = granted.Valid && granted.Bool
	return &p, nil
}

// GetCompanion supports only companion_id "me" in the demo (the member's own holder data).
func (g *GRPCServer) GetCompanion(ctx context.Context, req *memberv1.GetCompanionRequest) (*memberv1.Holder, error) {
	if strings.TrimSpace(req.CompanionId) != "me" {
		return nil, status.Error(codes.NotFound, "companion_not_found")
	}
	var h memberv1.Holder
	var first, last string
	err := g.Svc.DB.QueryRowContext(ctx,
		`SELECT first_name, last_name, nationality, id_last4, thai_verified FROM members WHERE id = ? AND deleted_at IS NULL`,
		req.MemberId).Scan(&first, &last, &h.Nationality, &h.IdLast4, &h.ThaiVerified)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "member_not_found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "internal")
	}
	h.Name = first + " " + last
	return &h, nil
}
