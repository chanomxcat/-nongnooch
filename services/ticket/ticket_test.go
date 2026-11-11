//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/testutil"
)

type env struct {
	t   *testing.T
	d   *sql.DB
	svc *Service
	mux *http.ServeMux
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d, _ := testutil.DB(t)
	sub, _ := fs.Sub(migrationsFS, "migrations")
	if err := db.Migrate(testutil.Ctx(t), d, sub); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: d, Signer: vectorSigner(t)}
	mux := http.NewServeMux()
	(&API{Svc: svc, DemoMode: true}).Register(mux)
	return &env{t: t, d: d, svc: svc, mux: mux}
}

func (e *env) paid(orderID string, items ...map[string]any) events.Envelope {
	for i, it := range items {
		if _, ok := it["orderItemId"]; !ok {
			it["orderItemId"] = orderID + "-item-" + string(rune('a'+i))
		}
	}
	env, err := events.New(events.OrderPaid, map[string]any{
		"orderId": orderID, "orderNo": "NN270301000001", "memberId": "m-1", "lang": "th", "paidAmountSatang": 100, "items": items,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return env
}

func dayItem(date, name, last4 string) map[string]any {
	return map[string]any{"kind": "day", "visitDate": date, "holder": map[string]any{"name": name, "idLast4": last4}}
}

func (e *env) issue(env events.Envelope) {
	e.t.Helper()
	tx, err := e.d.BeginTx(context.Background(), nil)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.svc.Issue(context.Background(), tx, env); err != nil {
		tx.Rollback()
		e.t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) do(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

var (
	gateHdr  = map[string]string{"X-User-Type": "staff", "X-User-Sub": "s1", "X-Roles": "gate"}
	salesHdr = map[string]string{"X-User-Type": "staff", "X-User-Sub": "s2", "X-Roles": "sales"}
	userHdr  = map[string]string{"X-User-Type": "user", "X-User-Sub": "+66811111111", "X-Member-Id": "m-1"}
)

func (e *env) scan(in map[string]any) ScanResult {
	e.t.Helper()
	w := e.do("POST", "/gate/scans", in, gateHdr)
	if w.Code != 200 {
		e.t.Fatalf("scan %v: %d %s", in, w.Code, w.Body)
	}
	var r ScanResult
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	return r
}

func (e *env) mine() []TicketView {
	e.t.Helper()
	w := e.do("GET", "/tickets/me", nil, userHdr)
	if w.Code != 200 {
		e.t.Fatalf("me: %d %s", w.Code, w.Body)
	}
	var out struct{ Items []TicketView }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.Items
}

func (e *env) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.d.QueryRow(q, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func today(e *env) string { return e.svc.today() }

func TestIssueReplayGivesSameTickets(t *testing.T) {
	e := newEnv(t)
	d := today(e)
	ev := e.paid("o-1", dayItem(d, "Somchai Jaidee", "0708"), dayItem(d, "Wei Zhang", "5678"))
	e.issue(ev)
	first := e.mine()
	if len(first) != 2 {
		t.Fatalf("want 2 tickets, got %d", len(first))
	}
	if first[0].TicketID == first[1].TicketID || first[0].QR == "" || first[0].Status != "active" || first[0].Kind != "day" {
		t.Fatalf("bad tickets: %+v", first)
	}
	if _, ok := e.svc.Signer.Verify(first[0].QR); !ok {
		t.Fatal("issued QR must verify")
	}
	// Same event again, and the same order under a brand-new event id.
	replay := ev
	e.issue(replay)
	again := e.paid("o-1", dayItem(d, "Somchai Jaidee", "0708"), dayItem(d, "Wei Zhang", "5678"))
	e.issue(again)
	second := e.mine()
	if len(second) != 2 || second[0].TicketID != first[0].TicketID || second[1].TicketID != first[1].TicketID {
		t.Fatalf("replay changed tickets: %+v vs %+v", first, second)
	}
	if n := e.count(`SELECT COUNT(*) FROM outbox WHERE type='ticket.issued'`); n != 1 {
		t.Fatalf("ticket.issued emitted %d times, want 1", n)
	}
}

func TestAnnualTicketHasNoDate(t *testing.T) {
	e := newEnv(t)
	e.issue(e.paid("o-2", map[string]any{"kind": "annual", "holder": map[string]any{"name": "A", "idLast4": "1111"}}))
	ts := e.mine()
	if len(ts) != 1 || ts[0].VisitDate != "" || ts[0].Kind != "annual" {
		t.Fatalf("%+v", ts)
	}
	p, ok := e.svc.Signer.Verify(ts[0].QR)
	if !ok || p.D != nil || p.K != "annual" {
		t.Fatalf("payload %+v ok=%v", p, ok)
	}
}

func TestScanWrongSignatureInvalid(t *testing.T) {
	e := newEnv(t)
	e.issue(e.paid("o-3", dayItem(today(e), "A", "1234")))
	tk := e.mine()[0]
	forged := tk.QR[:len(tk.QR)-3] + "AAA"
	if r := e.scan(map[string]any{"qr": forged, "gateId": "g1"}); r.Result != "invalid" {
		t.Fatalf("forged: %+v", r)
	}
	// Valid format signed by a different key.
	other, _ := NewSigner("AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=", nil)
	d := today(e)
	qr := other.Sign(QRPayload{T: tk.TicketID, K: "day", D: &d, H: "1234"})
	if r := e.scan(map[string]any{"qr": qr, "gateId": "g1"}); r.Result != "invalid" {
		t.Fatalf("other key: %+v", r)
	}
	if r := e.scan(map[string]any{"qr": "garbage", "gateId": "g1"}); r.Result != "invalid" {
		t.Fatalf("garbage: %+v", r)
	}
	// None of that consumed the ticket.
	if r := e.scan(map[string]any{"qr": tk.QR, "gateId": "g1"}); r.Result != "ok" {
		t.Fatalf("genuine after forged: %+v", r)
	}
}

func TestSecondScanIsUsedWithFirstTime(t *testing.T) {
	e := newEnv(t)
	e.issue(e.paid("o-4", dayItem(today(e), "Somchai Jaidee", "0708")))
	tk := e.mine()[0]
	first := e.scan(map[string]any{"qr": tk.QR, "gateId": "g1"})
	if first.Result != "ok" || first.HolderName != "Somchai Jaidee" || first.IDLast4 != "0708" || first.UsedAt == "" {
		t.Fatalf("first: %+v", first)
	}
	time.Sleep(20 * time.Millisecond)
	second := e.scan(map[string]any{"qr": tk.QR, "gateId": "g2"})
	if second.Result != "used" || second.UsedAt == "" {
		t.Fatalf("second: %+v", second)
	}
	a, _ := time.Parse(time.RFC3339Nano, first.UsedAt)
	b, _ := time.Parse(time.RFC3339Nano, second.UsedAt)
	if a.Truncate(time.Microsecond).UnixMicro() != b.UnixMicro() {
		t.Fatalf("usedAt must be the first scan time: %s vs %s", first.UsedAt, second.UsedAt)
	}
	if m := e.mine()[0]; m.Status != "used" || m.UsedAt == "" {
		t.Fatalf("my ticket: %+v", m)
	}
	var payload string
	if err := e.d.QueryRow(`SELECT payload FROM outbox WHERE type='ticket.scanned' ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var env struct{ Payload map[string]any }
	_ = json.Unmarshal([]byte(payload), &env)
	p := env.Payload
	if p["result"] != "used" || p["duplicate"] != true || p["method"] != "qr" || p["offline"] != false || p["gateId"] != "g2" || p["memberId"] != "m-1" || p["kind"] != "day" {
		t.Fatalf("event payload: %v", p)
	}
	// manual entry by ticket id uses method manual
	e.scan(map[string]any{"ticketId": tk.TicketID, "gateId": "g1"})
	_ = e.d.QueryRow(`SELECT payload FROM outbox WHERE type='ticket.scanned' ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&payload)
	if !bytes.Contains([]byte(payload), []byte(`"method":"manual"`)) {
		t.Fatalf("manual method missing: %s", payload)
	}
}

func TestConcurrentScansOnlyOneOK(t *testing.T) {
	e := newEnv(t)
	e.issue(e.paid("o-5", dayItem(today(e), "A", "1234")))
	tk := e.mine()[0]
	var wg sync.WaitGroup
	results := make([]string, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := e.do("POST", "/gate/scans", map[string]any{"qr": tk.QR, "gateId": "g"}, gateHdr)
			var r ScanResult
			_ = json.Unmarshal(w.Body.Bytes(), &r)
			results[i] = r.Result
		}(i)
	}
	wg.Wait()
	ok, used := 0, 0
	for _, r := range results {
		switch r {
		case "ok":
			ok++
		case "used":
			used++
		}
	}
	if ok != 1 || used != len(results)-1 {
		t.Fatalf("results %v", results)
	}
	if n := e.count(`SELECT COUNT(*) FROM scans WHERE result='ok'`); n != 1 {
		t.Fatalf("ok scans %d", n)
	}
}

func TestWrongDateAndVoid(t *testing.T) {
	e := newEnv(t)
	e.issue(e.paid("o-6", dayItem("2099-01-01", "A", "1234"), dayItem(today(e), "B", "5678")))
	var future, voidT TicketView
	for _, tk := range e.mine() {
		if tk.VisitDate == "2099-01-01" {
			future = tk
		} else {
			voidT = tk
		}
	}
	if r := e.scan(map[string]any{"qr": future.QR, "gateId": "g"}); r.Result != "wrong_date" || r.VisitDate != "2099-01-01" {
		t.Fatalf("wrong date: %+v", r)
	}
	// wrong_date must not consume the ticket
	if e.mine()[0].Status != "active" && e.mine()[1].Status != "active" {
		t.Fatal("ticket consumed by wrong_date")
	}
	if n, err := e.svc.Void(context.Background(), "o-6", []string{"o-6-item-b"}); err != nil || n != 1 {
		t.Fatalf("void %d %v", n, err)
	}
	if r := e.scan(map[string]any{"qr": voidT.QR, "gateId": "g"}); r.Result != "void" {
		t.Fatalf("void: %+v", r)
	}
	if r := e.scan(map[string]any{"ticketId": "no-such", "gateId": "g"}); r.Result != "invalid" {
		t.Fatalf("unknown id: %+v", r)
	}
}

func TestAuthAndEndpoints(t *testing.T) {
	e := newEnv(t)
	d := today(e)
	e.issue(e.paid("o-7", dayItem(d, "A", "1234")))
	if w := e.do("POST", "/gate/scans", map[string]any{"ticketId": "x", "gateId": "g"}, salesHdr); w.Code != 403 {
		t.Fatalf("non-gate role: %d", w.Code)
	}
	if w := e.do("POST", "/gate/scans", map[string]any{"ticketId": "x", "gateId": "g"}, userHdr); w.Code != 401 {
		t.Fatalf("user on gate: %d", w.Code)
	}
	if w := e.do("GET", "/tickets/me", nil, nil); w.Code != 401 {
		t.Fatalf("anon me: %d", w.Code)
	}
	if w := e.do("GET", "/tickets/me", nil, map[string]string{"X-User-Type": "user", "X-User-Sub": "x", "X-Member-Id": "m-other"}); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"items":[]`)) {
		t.Fatalf("other member sees tickets: %s", w.Body)
	}
	w := e.do("GET", "/gate/snapshot?date="+d, nil, map[string]string{"X-User-Type": "staff", "X-Roles": "admin"})
	var snap struct {
		Date       string
		PublicKeys []string
		Tickets    []SnapshotTicket
	}
	_ = json.Unmarshal(w.Body.Bytes(), &snap)
	if w.Code != 200 || snap.Date != d || len(snap.PublicKeys) != 1 || len(snap.Tickets) != 1 || snap.Tickets[0].Status != "active" {
		t.Fatalf("snapshot: %d %s", w.Code, w.Body)
	}
	e.scan(map[string]any{"ticketId": e.mine()[0].TicketID, "gateId": "g"})
	w = e.do("GET", "/admin/stats/today", nil, salesHdr)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"scansToday":1`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"ticketsIssuedToday":1`)) {
		t.Fatalf("stats: %d %s", w.Code, w.Body)
	}
	if w := e.do("POST", "/internal/seed", nil, nil); w.Code != 204 {
		t.Fatalf("seed: %d", w.Code)
	}
}

func TestTicketsKeepOrderItemOrder(t *testing.T) {
	e := newEnv(t)
	d := today(e)
	e.issue(e.paid("o-ord", dayItem(d, "Somchai Jaidee", "0708"), dayItem(d, "Wei Zhang", "5678"), dayItem(d, "Third Person", "9999")))
	got := e.mine()
	if len(got) != 3 || got[0].HolderName != "Somchai Jaidee" || got[1].HolderName != "Wei Zhang" || got[2].HolderName != "Third Person" {
		t.Fatalf("order: %+v", got)
	}
}
