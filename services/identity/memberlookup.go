package main

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	memberv1 "nongnooch/platform/gen/go/member/v1"
)

// GRPCMemberLookup resolves memberId through MemberService.GetMemberByContact.
type GRPCMemberLookup struct {
	Client  memberv1.MemberServiceClient
	Timeout time.Duration
}

// NewGRPCMemberLookup dials addr lazily (the member service may start after identity).
func NewGRPCMemberLookup(addr string) (*GRPCMemberLookup, *grpc.ClientConn, error) {
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return &GRPCMemberLookup{Client: memberv1.NewMemberServiceClient(cc), Timeout: time.Second}, cc, nil
}

// GetMemberIDByContact returns "" (nil error) when the contact is not a member yet.
func (g *GRPCMemberLookup) GetMemberIDByContact(ctx context.Context, contact string) (string, error) {
	to := g.Timeout
	if to <= 0 {
		to = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()
	m, err := g.Client.GetMemberByContact(ctx, &memberv1.GetMemberByContactRequest{Contact: contact})
	if status.Code(err) == codes.NotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return m.MemberId, nil
}
