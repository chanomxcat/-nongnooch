//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	memberv1 "nongnooch/platform/gen/go/member/v1"
)

const thaiPhone = "+66811111111"

func TestRegisterThaiCreatesMemberAndEvents(t *testing.T) {
	e := newEnv(t)
	w := e.as(thaiPhone, "POST", "/members", reg(nil))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var out struct {
		Member MemberView `json:"member"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if !out.Member.ThaiVerified || out.Member.IDLast4 != "0708" || out.Member.Contact != thaiPhone {
		t.Fatalf("%+v", out.Member)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM outbox WHERE type='member.registered'`); n != 1 {
		t.Fatalf("member.registered rows=%d", n)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM outbox WHERE type='points.changed'`); n != 1 {
		t.Fatalf("points.changed rows=%d", n)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM consent_log`); n != 3 {
		t.Fatalf("consent rows=%d", n)
	}
	if n := e.count(t, `SELECT delta_points FROM point_ledger WHERE reason='welcome'`); n != 100 {
		t.Fatalf("welcome=%d", n)
	}
}

func TestRegisterForeignPassport(t *testing.T) {
	e := newEnv(t)
	w := e.as("+8613800138000", "POST", "/members", reg(map[string]any{
		"nationality": "CN", "idNumber": "E12345678", "passportCountry": "CN", "lang": "zh"}))
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"idLast4":"5678"`) || !strings.Contains(w.Body.String(), `"thaiVerified":false`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestBadCheckDigit422(t *testing.T) {
	e := newEnv(t)
	w := e.as(thaiPhone, "POST", "/members", reg(map[string]any{"idNumber": "1101700230705"}))
	if w.Code != 422 || !strings.Contains(w.Body.String(), "id_number_invalid") {
		t.Fatal(w.Code, w.Body.String())
	}
	if e.count(t, `SELECT COUNT(*) FROM members`) != 0 {
		t.Fatal("member created")
	}
}

func TestExistingContactNoExtraPoints(t *testing.T) {
	e := newEnv(t)
	e.as(thaiPhone, "POST", "/members", reg(nil))
	w := e.as(thaiPhone, "POST", "/members", reg(nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"existing":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if e.count(t, `SELECT COUNT(*) FROM members`) != 1 || e.count(t, `SELECT COUNT(*) FROM point_ledger`) != 1 ||
		e.count(t, `SELECT COUNT(*) FROM outbox WHERE type='member.registered'`) != 1 {
		t.Fatal("duplicate side effects")
	}
}

func TestSameIDDifferentContact409(t *testing.T) {
	e := newEnv(t)
	e.as(thaiPhone, "POST", "/members", reg(nil))
	w := e.as("+66899999999", "POST", "/members", reg(map[string]any{"idNumber": "1-1017-00230-70-8"}))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "id_number_taken") {
		t.Fatal(w.Code, w.Body.String())
	}
	if e.count(t, `SELECT COUNT(*) FROM members`) != 1 || e.count(t, `SELECT COUNT(*) FROM point_ledger`) != 1 {
		t.Fatal("second account leaked")
	}
}

func TestConcurrentSameContact(t *testing.T) {
	e := newEnv(t)
	var wg sync.WaitGroup
	codes := make([]int, 10)
	for i := range codes {
		wg.Add(1)
		go func() { defer wg.Done(); codes[i] = e.as(thaiPhone, "POST", "/members", reg(nil)).Code }()
	}
	wg.Wait()
	created := 0
	for _, c := range codes {
		if c == 201 {
			created++
		} else if c != 200 {
			t.Fatalf("codes=%v", codes)
		}
	}
	if created != 1 || e.count(t, `SELECT COUNT(*) FROM members`) != 1 || e.count(t, `SELECT COUNT(*) FROM point_ledger`) != 1 {
		t.Fatalf("codes=%v", codes)
	}
}

func TestConsentRequired(t *testing.T) {
	e := newEnv(t)
	w := e.as(thaiPhone, "POST", "/members", reg(map[string]any{"consents": map[string]bool{"terms": true, "pdpa": false}}))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "consent_required") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestUnauthenticated401(t *testing.T) {
	e := newEnv(t)
	if w := e.as("", "POST", "/members", reg(nil)); w.Code != 401 {
		t.Fatal(w.Code)
	}
}

func TestWelcomePlusInvite150(t *testing.T) {
	e := newEnv(t)
	w := e.as(thaiPhone, "POST", "/members", reg(map[string]any{"inviteCode": "demo-guide1"}))
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"inviteStatus":"applied"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	p := e.as(thaiPhone, "GET", "/members/me/points", nil)
	var pr struct {
		Available     int    `json:"available"`
		Pending       int    `json:"pending"`
		RedeemEnabled bool   `json:"redeemEnabled"`
		Items         []Entry `json:"items"`
	}
	json.Unmarshal(p.Body.Bytes(), &pr)
	if p.Code != 200 || pr.Available != 150 || pr.RedeemEnabled || len(pr.Items) != 2 {
		t.Fatal(p.Code, p.Body.String())
	}
	var payload string
	e.d.QueryRow(`SELECT payload FROM outbox WHERE type='member.registered'`).Scan(&payload)
	if !strings.Contains(payload, `"guideId":"g-1"`) || !strings.Contains(payload, `"companyId":"c-1"`) {
		t.Fatal(payload)
	}
	if e.count(t, `SELECT COUNT(*) FROM outbox WHERE type='points.changed'`) != 2 {
		t.Fatal("want 2 points.changed")
	}
}

func TestInvalidInvite422(t *testing.T) {
	e := newEnv(t)
	w := e.as(thaiPhone, "POST", "/members", reg(map[string]any{"inviteCode": "NOSUCHCODE"}))
	if w.Code != 422 || !strings.Contains(w.Body.String(), "invite_invalid") {
		t.Fatal(w.Code, w.Body.String())
	}
	if e.count(t, `SELECT COUNT(*) FROM members`) != 0 || e.count(t, `SELECT COUNT(*) FROM point_ledger`) != 0 {
		t.Fatal("rejected registration left rows")
	}
}

func TestInviteValidatorDown503(t *testing.T) {
	e := newEnv(t)
	e.svc.Invites = fakeInvites{err: context.DeadlineExceeded}
	w := e.as(thaiPhone, "POST", "/members", reg(map[string]any{"inviteCode": "DEMOGUIDE1"}))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "invite_unavailable") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestNoFullIDAnywhere(t *testing.T) {
	logs := captureLogs(t)
	e := newEnv(t)
	const full = "1101700230708"
	var bodies []string
	bodies = append(bodies, e.as(thaiPhone, "POST", "/members", reg(nil)).Body.String())
	bodies = append(bodies, e.as(thaiPhone, "POST", "/members", reg(nil)).Body.String())
	bodies = append(bodies, e.as("+66899999999", "POST", "/members", reg(nil)).Body.String()) // 409
	bodies = append(bodies, e.as(thaiPhone, "GET", "/members/me", nil).Body.String())
	bodies = append(bodies, e.as(thaiPhone, "GET", "/members/me/points", nil).Body.String())
	bodies = append(bodies, e.as(thaiPhone, "POST", "/members", reg(map[string]any{"idNumber": "1101700230705"})).Body.String())
	rows, _ := e.d.Query(`SELECT payload FROM outbox`)
	for rows.Next() {
		var p string
		rows.Scan(&p)
		bodies = append(bodies, p)
	}
	rows.Close()
	bodies = append(bodies, logs.String())
	for _, b := range bodies {
		if strings.Contains(b, full) {
			t.Fatalf("full id leaked in: %s", b)
		}
	}
	var enc []byte
	e.d.QueryRow(`SELECT id_enc FROM members`).Scan(&enc)
	if len(enc) == 0 || strings.Contains(string(enc), full) {
		t.Fatal("id_enc not encrypted")
	}
}

func TestLedgerAppendOnly(t *testing.T) {
	e := newEnv(t)
	e.as(thaiPhone, "POST", "/members", reg(nil))
	if _, err := e.d.Exec(`UPDATE point_ledger SET delta_points = 999`); err == nil {
		t.Fatal("update must fail")
	}
	if _, err := e.d.Exec(`DELETE FROM point_ledger`); err == nil {
		t.Fatal("delete must fail")
	}
	if e.as(thaiPhone, "POST", "/members/me/points/redeem", nil).Code != 404 {
		t.Fatal("redeem route must not exist")
	}
}

func TestMeNotFoundBeforeRegister(t *testing.T) {
	e := newEnv(t)
	if w := e.as(thaiPhone, "GET", "/members/me", nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestSeedIdempotent(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 2; i++ {
		if w := e.as("", "POST", "/internal/seed", nil); w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if e.count(t, `SELECT COUNT(*) FROM members`) != 3 || e.count(t, `SELECT COUNT(*) FROM point_ledger`) != 3 {
		t.Fatal("seed not idempotent")
	}
	if e.count(t, `SELECT COUNT(*) FROM demo_invite_codes WHERE code='DEMOGUIDE1'`) != 1 {
		t.Fatal("invite code missing")
	}
	// the seeded Thai member cannot be registered again with another contact
	if w := e.as("+66877777777", "POST", "/members", reg(nil)); w.Code != 409 {
		t.Fatal(w.Code)
	}
}

func TestGRPC(t *testing.T) {
	e := newEnv(t)
	e.as("", "POST", "/internal/seed", nil)
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	memberv1.RegisterMemberServiceServer(s, &GRPCServer{Svc: e.svc})
	go s.Serve(lis)
	t.Cleanup(s.Stop)
	cc, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cc.Close() })
	c := memberv1.NewMemberServiceClient(cc)
	ctx := context.Background()

	m, err := c.GetMemberByContact(ctx, &memberv1.GetMemberByContactRequest{Contact: "+66811111111"})
	if err != nil || m.MemberId == "" || !m.ThaiVerified || m.IdLast4 != "0708" || m.Name != "Somchai Jaidee" {
		t.Fatal(m, err)
	}
	if _, err := c.GetMemberByContact(ctx, &memberv1.GetMemberByContactRequest{Contact: "+66800000000"}); status.Code(err) != codes.NotFound {
		t.Fatal(err)
	}
	jp, err := c.GetMemberByContact(ctx, &memberv1.GetMemberByContactRequest{Contact: "Taro@Example.jp"})
	if err != nil || jp.Nationality != "JP" || jp.ThaiVerified {
		t.Fatal(jp, err)
	}
	h, err := c.GetCompanion(ctx, &memberv1.GetCompanionRequest{MemberId: m.MemberId, CompanionId: "me"})
	if err != nil || h.Name != "Somchai Jaidee" || h.IdLast4 != "0708" || !h.ThaiVerified || h.Nationality != "TH" {
		t.Fatal(h, err)
	}
	if _, err := c.GetCompanion(ctx, &memberv1.GetCompanionRequest{MemberId: m.MemberId, CompanionId: "x"}); status.Code(err) != codes.NotFound {
		t.Fatal(err)
	}
	p, err := c.GetContactPrefs(ctx, &memberv1.GetContactPrefsRequest{MemberId: m.MemberId})
	if err != nil || p.Lang != "th" || p.Phone != "+66811111111" || p.MarketingConsent {
		t.Fatal(p, err)
	}
}
