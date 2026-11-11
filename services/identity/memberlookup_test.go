package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	memberv1 "nongnooch/platform/gen/go/member/v1"
)

type fakeMemberSrv struct {
	memberv1.UnimplementedMemberServiceServer
}

func (fakeMemberSrv) GetMemberByContact(_ context.Context, r *memberv1.GetMemberByContactRequest) (*memberv1.Member, error) {
	switch r.Contact {
	case "+66811111111":
		return &memberv1.Member{MemberId: "m-1"}, nil
	case "boom":
		return nil, status.Error(codes.Unavailable, "down")
	}
	return nil, status.Error(codes.NotFound, "member_not_found")
}

func newLookup(t *testing.T) *GRPCMemberLookup {
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	memberv1.RegisterMemberServiceServer(s, fakeMemberSrv{})
	go s.Serve(lis)
	t.Cleanup(s.Stop)
	cc, err := grpc.NewClient("passthrough:///b", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cc.Close() })
	return &GRPCMemberLookup{Client: memberv1.NewMemberServiceClient(cc)}
}

func TestGRPCLookupFoundNotFoundError(t *testing.T) {
	l := newLookup(t)
	if id, err := l.GetMemberIDByContact(context.Background(), "+66811111111"); err != nil || id != "m-1" {
		t.Fatal(id, err)
	}
	if id, err := l.GetMemberIDByContact(context.Background(), "+66800000000"); err != nil || id != "" {
		t.Fatal(id, err)
	}
	if _, err := l.GetMemberIDByContact(context.Background(), "boom"); err == nil {
		t.Fatal("want error")
	}
}

type failingMembers struct{}

func (failingMembers) GetMemberIDByContact(context.Context, string) (string, error) {
	return "", errors.New("down")
}

func TestLookupErrorDoesNotFailAccessToken(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	tk := &TokenIssuer{Key: priv, Members: failingMembers{}}
	if _, err := tk.access(context.Background(), Contact{Kind: "phone", Value: "+66811111111"}); err != nil {
		t.Fatal(err)
	}
}
