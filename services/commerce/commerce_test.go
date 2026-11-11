//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/testutil"
)

const secret = "test-secret"

type fakeMember map[string]Holder

func (f fakeMember) GetCompanion(_ context.Context, memberID, _ string) (Holder, error) {
	return f[memberID], nil
}

type env struct {
	t   *testing.T
	d   *sql.DB
	svc *Service
	mux *http.ServeMux
	now atomic.Int64 // unix nanos
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d, _ := testutil.DB(t)
	sub, _ := fs.Sub(migrationsFS, "migrations")
	if err := db.Migrate(testutil.Ctx(t), d, sub); err != nil {
		t.Fatal(err)
	}
	kb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if r.Header.Get("Authorization") != "Bearer demo-key" || in["currency"] != "THB" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"paymentId":"pay_x","paymentUrl":"http://mock/pg/checkout/pay_x?ref=%v"}`, in["ref"])
	}))
	t.Cleanup(kb.Close)
	e := &env{t: t, d: d}
	e.now.Store(time.Now().UnixNano())
	e.svc = &Service{
		DB: d, WebhookSecret: secret, WebhookURL: "http://commerce:8080/webhooks/kbank",
		KBank: &KBank{BaseURL: kb.URL, APIKey: "demo-key"},
		Member: fakeMember{
			"m-thai": {Name: "Somchai Jaidee", Nationality: "TH", IDLast4: "0708", ThaiVerified: true},
			"m-fore": {Name: "Wei Zhang", Nationality: "CN", IDLast4: "5678", ThaiVerified: false},
		},
		Now: func() time.Time { return time.Unix(0, e.now.Load()) },
	}
	if err := e.svc.Seed(testutil.Ctx(t)); err != nil {
		t.Fatal(err)
	}
	e.mux = http.NewServeMux()
	(&API{Svc: e.svc, DemoMode: true}).Register(e.mux)
	return e
}

func (e *env) today() string { return e.svc.now().In(ict).Format(dateLayout) }

func (e *env) pid(key string) string {
	ps, err := e.svc.Products(testutil.Ctx(e.t), "en")
	if err != nil {
		e.t.Fatal(err)
	}
	for _, p := range ps {
		if (key == "day-adult" && p.Kind == "day" && p.AgeGroup == "adult") || (key == "annual" && p.Kind == "annual") {
			return p.ProductID
		}
	}
	e.t.Fatal("no product", key)
	return ""
}

func (e *env) do(member, method, path string, hdr map[string]string, body any) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	switch b := body.(type) {
	case []byte:
		rd = bytes.NewReader(b)
	default:
		j, _ := json.Marshal(body)
		rd = bytes.NewReader(j)
	}
	r := httptest.NewRequest(method, path, rd)
	if member != "" {
		r.Header.Set("X-User-Type", "user")
		r.Header.Set("X-User-Sub", "+66800000000")
		r.Header.Set("X-Member-Id", member)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func me() map[string]any { return map[string]any{"companionId": "me"} }

func (e *env) orderBody(n int, audience string, holder any) map[string]any {
	items := []map[string]any{}
	for i := 0; i < n; i++ {
		items = append(items, map[string]any{"productId": e.pid("day-adult"), "audience": audience, "holder": holder})
	}
	return map[string]any{"visitDate": e.today(), "items": items}
}

func (e *env) mustOrder(member string, n int, audience string) (id string) {
	w := e.do(member, "POST", "/orders", nil, e.orderBody(n, audience, me()))
	if w.Code != 201 {
		e.t.Fatalf("order: %d %s", w.Code, w.Body)
	}
	var out struct{ OrderID string }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.OrderID
}

func errCode(w *httptest.ResponseRecorder) string {
	var out struct{ Error string }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.Error
}

func (e *env) count(q string, args ...any) int {
	var n int
	if err := e.d.QueryRow(q, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) pay(member, orderID string) (paymentID string) {
	w := e.do(member, "POST", "/orders/"+orderID+"/payments", nil, map[string]string{"method": "card", "returnUrl": "http://x/return"})
	if w.Code != 201 {
		e.t.Fatalf("payment: %d %s", w.Code, w.Body)
	}
	var out struct{ PaymentID, PaymentURL string }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if !strings.Contains(out.PaymentURL, "/pg/checkout/") {
		e.t.Fatal("paymentUrl", out.PaymentURL)
	}
	return out.PaymentID
}

func (e *env) webhook(ref, eventID, status string, amount int64, sig string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"eventId": eventID, "paymentId": "pay_x", "ref": ref, "status": status, "amount": amount, "paidAt": "2026-11-11T10:00:00Z"})
	if sig == "" {
		sig = Sign(secret, body)
	}
	return e.do("", "POST", "/webhooks/kbank", map[string]string{"X-Mock-Signature": sig}, body)
}

func (e *env) setQuota(total int) {
	if _, err := e.d.Exec(`UPDATE quota SET total=?, held=0, sold=0 WHERE visit_date=?`, total, e.today()); err != nil {
		e.t.Fatal(err)
	}
}

func TestThaiPriceNeedsThaiVerifiedHolder(t *testing.T) {
	e := newEnv(t)
	// foreign member, own holder data (not thai_verified)
	if w := e.do("m-fore", "POST", "/orders", nil, e.orderBody(1, "thai", me())); w.Code != 422 || errCode(w) != "thai_price_not_allowed" {
		t.Fatalf("own non-thai: %d %s", w.Code, w.Body)
	}
	// even a Thai member cannot give a Thai price to an ad-hoc holder
	adhoc := map[string]any{"name": "Friend", "nationality": "TH", "idLast4": "1234"}
	if w := e.do("m-thai", "POST", "/orders", nil, e.orderBody(1, "thai", adhoc)); w.Code != 422 || errCode(w) != "thai_price_not_allowed" {
		t.Fatalf("adhoc: %d %s", w.Code, w.Body)
	}
	// Thai verified self at Thai price: 30000 satang
	w := e.do("m-thai", "POST", "/orders", nil, e.orderBody(1, "thai", me()))
	var out struct{ TotalSatang int64 }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 201 || out.TotalSatang != 30000 {
		t.Fatalf("thai: %d %s", w.Code, w.Body)
	}
	// foreign audience for an ad-hoc holder: 60000
	w = e.do("m-thai", "POST", "/orders", nil, e.orderBody(1, "foreign", adhoc))
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 201 || out.TotalSatang != 60000 {
		t.Fatalf("foreign: %d %s", w.Code, w.Body)
	}
}

func TestOrderRequiresMemberAndFutureDate(t *testing.T) {
	e := newEnv(t)
	if w := e.do("", "POST", "/orders", nil, e.orderBody(1, "foreign", me())); w.Code != 401 {
		t.Fatalf("anon %d", w.Code)
	}
	b := e.orderBody(1, "foreign", me())
	b["visitDate"] = e.svc.now().In(ict).AddDate(0, 0, -1).Format(dateLayout)
	if w := e.do("m-fore", "POST", "/orders", nil, b); w.Code != 422 || errCode(w) != "visit_date_past" {
		t.Fatalf("past %d %s", w.Code, w.Body)
	}
}

func TestSoldOut(t *testing.T) {
	e := newEnv(t)
	e.setQuota(1)
	w := e.do("m-fore", "POST", "/orders", nil, e.orderBody(2, "foreign", me()))
	if w.Code != 409 || errCode(w) != "sold_out" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if e.count(`SELECT COUNT(*) FROM orders`) != 0 {
		t.Fatal("order must not exist")
	}
	e.mustOrder("m-fore", 1, "foreign")
	if w := e.do("m-fore", "POST", "/orders", nil, e.orderBody(1, "foreign", me())); w.Code != 409 {
		t.Fatalf("second %d", w.Code)
	}
}

func TestConcurrentOrdersNeverOversell(t *testing.T) {
	e := newEnv(t)
	e.setQuota(5)
	var wg sync.WaitGroup
	var created, soldOut, other atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch w := e.do("m-fore", "POST", "/orders", nil, e.orderBody(1, "foreign", me())); w.Code {
			case 201:
				created.Add(1)
			case 409:
				soldOut.Add(1)
			default:
				other.Add(1)
				t.Logf("unexpected %d %s", w.Code, w.Body)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 5 || soldOut.Load() != 15 || other.Load() != 0 {
		t.Fatalf("created=%d soldOut=%d other=%d", created.Load(), soldOut.Load(), other.Load())
	}
	if h := e.count(`SELECT held FROM quota WHERE visit_date=?`, e.today()); h != 5 {
		t.Fatalf("held=%d", h)
	}
}

func TestIdempotencyKeyReplaysSameOrder(t *testing.T) {
	e := newEnv(t)
	h := map[string]string{"Idempotency-Key": "k-1"}
	a := e.do("m-fore", "POST", "/orders", h, e.orderBody(1, "foreign", me()))
	b := e.do("m-fore", "POST", "/orders", h, e.orderBody(1, "foreign", me()))
	if a.Code != 201 || b.Code != 201 || a.Body.String() != b.Body.String() {
		t.Fatalf("a=%d %s b=%d %s", a.Code, a.Body, b.Code, b.Body)
	}
	if e.count(`SELECT COUNT(*) FROM orders`) != 1 || e.count(`SELECT held FROM quota WHERE visit_date=?`, e.today()) != 1 {
		t.Fatal("replay created a second order or hold")
	}
	// concurrent duplicates also collapse into one order
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.do("m-fore", "POST", "/orders", map[string]string{"Idempotency-Key": "k-2"}, e.orderBody(1, "foreign", me()))
		}()
	}
	wg.Wait()
	if n := e.count(`SELECT COUNT(*) FROM orders`); n != 2 {
		t.Fatalf("orders=%d want 2", n)
	}
	// a failed request does not consume the key
	bad := e.orderBody(1, "thai", me())
	if w := e.do("m-fore", "POST", "/orders", map[string]string{"Idempotency-Key": "k-3"}, bad); w.Code != 422 {
		t.Fatalf("%d", w.Code)
	}
	if w := e.do("m-fore", "POST", "/orders", map[string]string{"Idempotency-Key": "k-3"}, e.orderBody(1, "foreign", me())); w.Code != 201 {
		t.Fatalf("retry after failure: %d", w.Code)
	}
}

func TestWebhookConfirmsOrderOnceAndEmitsOrderPaid(t *testing.T) {
	e := newEnv(t)
	id := e.mustOrder("m-thai", 2, "thai")
	pay := e.pay("m-thai", id)
	if w := e.webhook(pay, "evt-1", "paid", 60000, ""); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	e.webhook(pay, "evt-1", "paid", 60000, "") // replay, same eventId
	e.webhook(pay, "evt-2", "paid", 60000, "") // provider re-sent under another id
	if n := e.count(`SELECT COUNT(*) FROM outbox WHERE type='order.paid'`); n != 1 {
		t.Fatalf("order.paid count %d", n)
	}
	var st string
	_ = e.d.QueryRow(`SELECT status FROM orders WHERE id=?`, id).Scan(&st)
	if st != "confirmed" {
		t.Fatal("status", st)
	}
	q := e.d.QueryRow(`SELECT held, sold FROM quota WHERE visit_date=?`, e.today())
	var held, sold int
	_ = q.Scan(&held, &sold)
	if held != 0 || sold != 2 {
		t.Fatalf("held=%d sold=%d", held, sold)
	}
	var payload string
	_ = e.d.QueryRow(`SELECT payload FROM outbox WHERE type='order.paid'`).Scan(&payload)
	var env struct {
		Payload struct {
			OrderID, OrderNo, MemberID, Lang, PaymentID string
			PaidAmountSatang                            int64
			Items                                       []struct {
				VisitDate string
				Holder    struct {
					Name, IDLast4 string
					ThaiPrice     bool
				}
				PriceSatang int64
			}
		}
	}
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		t.Fatal(err)
	}
	p := env.Payload
	if p.OrderID != id || p.PaymentID != pay || p.MemberID != "m-thai" || p.PaidAmountSatang != 60000 || len(p.Items) != 2 ||
		p.Items[0].VisitDate != e.today() || !p.Items[0].Holder.ThaiPrice || p.Items[0].PriceSatang != 30000 || !strings.HasPrefix(p.OrderNo, "NN") || len(p.OrderNo) != 14 {
		t.Fatalf("payload %s", payload)
	}
	// GET /orders/{id}
	w := e.do("m-thai", "GET", "/orders/"+id, nil, nil)
	var ov struct {
		Status, PaidAt string
		TotalSatang    int64
	}
	_ = json.Unmarshal(w.Body.Bytes(), &ov)
	if w.Code != 200 || ov.Status != "confirmed" || ov.PaidAt == "" || ov.TotalSatang != 60000 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := e.do("m-fore", "GET", "/orders/"+id, nil, nil); w.Code != 404 {
		t.Fatalf("other member sees order: %d", w.Code)
	}
}

func TestWebhookBadSignature401(t *testing.T) {
	e := newEnv(t)
	id := e.mustOrder("m-fore", 1, "foreign")
	pay := e.pay("m-fore", id)
	for _, sig := range []string{"deadbeef", Sign("wrong-secret", []byte("x"))} {
		if w := e.webhook(pay, "evt-bad", "paid", 60000, sig); w.Code != 401 {
			t.Fatalf("%d", w.Code)
		}
	}
	if w := e.do("", "POST", "/webhooks/kbank", nil, []byte(`{}`)); w.Code != 401 {
		t.Fatalf("no sig: %d", w.Code)
	}
	if e.count(`SELECT COUNT(*) FROM outbox`) != 0 || e.count(`SELECT COUNT(*) FROM webhook_events`) != 0 {
		t.Fatal("bad signature must have no effect")
	}
}

func TestWebhookAmountMismatchDoesNotConfirm(t *testing.T) {
	e := newEnv(t)
	id := e.mustOrder("m-fore", 1, "foreign")
	pay := e.pay("m-fore", id)
	e.webhook(pay, "evt-m", "paid", 59900, "")
	var st string
	_ = e.d.QueryRow(`SELECT status FROM orders WHERE id=?`, id).Scan(&st)
	if st != "pending" || e.count(`SELECT COUNT(*) FROM outbox WHERE type='payment.discrepancy'`) != 1 || e.count(`SELECT COUNT(*) FROM outbox WHERE type='order.paid'`) != 0 {
		t.Fatalf("status %s", st)
	}
}

func TestReleaseExpiredReturnsQuota(t *testing.T) {
	e := newEnv(t)
	e.setQuota(3)
	id := e.mustOrder("m-fore", 2, "foreign")
	if n, _ := e.svc.ReleaseExpired(testutil.Ctx(t)); n != 0 {
		t.Fatal("released before hold elapsed")
	}
	e.now.Add(int64(16 * time.Minute))
	// paying an expired-hold order is refused
	if w := e.do("m-fore", "POST", "/orders/"+id+"/payments", nil, map[string]string{"method": "card", "returnUrl": "http://x"}); w.Code != 409 {
		t.Fatalf("%d", w.Code)
	}
	if n, err := e.svc.ReleaseExpired(testutil.Ctx(t)); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	e.svc.ReleaseExpired(testutil.Ctx(t))
	var st string
	_ = e.d.QueryRow(`SELECT status FROM orders WHERE id=?`, id).Scan(&st)
	if st != "expired" || e.count(`SELECT held FROM quota WHERE visit_date=?`, e.today()) != 0 ||
		e.count(`SELECT COUNT(*) FROM outbox WHERE type='order.expired'`) != 1 {
		t.Fatalf("status %s", st)
	}
}

func TestStatsToday(t *testing.T) {
	e := newEnv(t)
	staff := map[string]string{"X-User-Type": "staff", "X-User-Sub": "s1", "X-Roles": "cashier"}
	if w := e.do("m-fore", "GET", "/admin/stats/today", nil, nil); w.Code != 403 {
		t.Fatalf("user token: %d", w.Code)
	}
	id := e.mustOrder("m-thai", 2, "thai")
	e.webhook(e.pay("m-thai", id), "evt-s", "paid", 60000, "")
	r := httptest.NewRequest("GET", "/admin/stats/today", nil)
	for k, v := range staff {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	var st struct {
		SalesSatang, Orders, Tickets int64
		ByKind                       map[string]int64
	}
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if w.Code != 200 || st.SalesSatang != 60000 || st.Orders != 1 || st.Tickets != 2 || st.ByKind["day"] != 2 || st.ByKind["annual"] != 0 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestCatalogSeedIdempotentAndI18n(t *testing.T) {
	e := newEnv(t)
	if err := e.svc.Seed(testutil.Ctx(t)); err != nil {
		t.Fatal(err)
	}
	if e.count(`SELECT COUNT(*) FROM products`) != 3 || e.count(`SELECT COUNT(*) FROM quota`) != 60 {
		t.Fatal("seed counts")
	}
	var out struct {
		Items []struct {
			Name                      string
			ThaiSatang, ForeignSatang int64
		}
	}
	w := e.do("", "GET", "/products?lang=zh-Hans&visitDate="+e.today(), nil, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Items) != 3 || out.Items[0].Name != "日票（成人）" || out.Items[0].ThaiSatang != 30000 || out.Items[0].ForeignSatang != 60000 {
		t.Fatalf("%s", w.Body)
	}
	w = e.do("", "GET", "/products?lang=ja", nil, nil)
	if !strings.Contains(w.Body.String(), "Day ticket (adult)") {
		t.Fatalf("fallback: %s", w.Body)
	}
	var av struct {
		Items []struct {
			VisitDate string
			Remaining int
		}
	}
	w = e.do("", "GET", "/products/availability?from="+e.today()+"&to="+e.today(), nil, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &av)
	if len(av.Items) != 1 || av.Items[0].Remaining != 5000 {
		t.Fatalf("%s", w.Body)
	}
}

func TestOrderItemsBuyerFirst(t *testing.T) {
	e := newEnv(t)
	friend := map[string]any{"name": "Friend", "nationality": "CN", "idLast4": "1234"}
	body := map[string]any{"visitDate": e.today(), "items": []map[string]any{
		{"productId": e.pid("day-adult"), "audience": "foreign", "holder": friend},
		{"productId": e.pid("day-adult"), "audience": "thai", "holder": me()},
	}}
	w := e.do("m-thai", "POST", "/orders", nil, body)
	if w.Code != 201 {
		t.Fatalf("order: %d %s", w.Code, w.Body)
	}
	var out struct{ OrderID string }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	rows, err := e.d.Query(`SELECT holder_name FROM order_items WHERE order_id=? ORDER BY id`, out.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		names = append(names, n)
	}
	if len(names) != 2 || names[0] != "Somchai Jaidee" || names[1] != "Friend" {
		t.Fatalf("order %v", names)
	}
}
