package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type hook struct {
	body []byte
	sig  string
}

// receiver records webhooks and answers with the given status codes in order (then 200).
func receiver(codes ...int) (*httptest.Server, func() []hook) {
	var mu sync.Mutex
	var got []hook
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		n := len(got)
		got = append(got, hook{b, r.Header.Get("X-Mock-Signature")})
		mu.Unlock()
		if n < len(codes) {
			w.WriteHeader(codes[n])
		}
	}))
	return srv, func() []hook { mu.Lock(); defer mu.Unlock(); return append([]hook(nil), got...) }
}

func kb(a *App, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	a.Routes(mux)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer demo-key")
	if strings.HasPrefix(path, "/pg/checkout") {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func create(t *testing.T, a *App, ref, hookURL string, amount int) string {
	b, _ := json.Marshal(map[string]any{"ref": ref, "amount": amount, "currency": "THB", "method": "card",
		"returnUrl": "http://shop/return", "webhookUrl": hookURL})
	rec := kb(a, "POST", "/pg/payments", string(b))
	if rec.Code != 201 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out struct{ PaymentId, Status string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.PaymentId
}

func pay(a *App, id, action string) *httptest.ResponseRecorder {
	return kb(a, "POST", "/pg/checkout/"+id+"/pay", url.Values{"action": {action}}.Encode())
}

func inquiry(t *testing.T, a *App, ref string) (int, map[string]any) {
	rec := kb(a, "GET", "/pg/payments?ref="+ref, "")
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestPaySuccessSendsSignedWebhook(t *testing.T) {
	srv, got := receiver()
	defer srv.Close()
	a := newTestApp()
	id := create(t, a, "ORD1", srv.URL, 50000)
	rec := pay(a, id, "success")
	if rec.Code != 303 || rec.Header().Get("Location") != "http://shop/return?ref=ORD1" {
		t.Fatal(rec.Code, rec.Header())
	}
	h := got()
	if len(h) != 1 || h[0].sig != sign("demo-secret", h[0].body) {
		t.Fatalf("hooks=%v", h)
	}
	var ev map[string]any
	_ = json.Unmarshal(h[0].body, &ev)
	if ev["status"] != "paid" || ev["paymentId"] != id || ev["amount"] != float64(50000) || ev["eventId"] == "" {
		t.Fatal(ev)
	}
	if _, in := inquiry(t, a, "ORD1"); in["status"] != "paid" {
		t.Fatal(in)
	}
}

func TestWebhookRetryOnFailure(t *testing.T) {
	srv, got := receiver(500, 500)
	defer srv.Close()
	a := newTestApp()
	var slept []time.Duration
	a.Sleep = func(d time.Duration) { slept = append(slept, d) }
	id := create(t, a, "ORD2", srv.URL, 1000)
	pay(a, id, "success")
	h := got()
	if len(h) != 3 || string(h[0].body) != string(h[2].body) {
		t.Fatalf("attempts=%d", len(h))
	}
	if len(slept) != 2 || slept[0] != 2*time.Second || slept[1] != 4*time.Second {
		t.Fatal(slept)
	}
}

func TestResendKeepsEventID(t *testing.T) {
	srv, got := receiver()
	defer srv.Close()
	a := newTestApp()
	id := create(t, a, "ORD3", srv.URL, 1000)
	pay(a, id, "success")
	if rec := kb(a, "POST", "/pg/payments/"+id+"/webhook:resend", ""); rec.Code != 202 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	h := got()
	if len(h) != 2 || string(h[0].body) != string(h[1].body) {
		t.Fatalf("hooks=%d", len(h))
	}
}

func TestPaidWithoutWebhookInquiryShowsPaid(t *testing.T) {
	srv, got := receiver()
	defer srv.Close()
	a := newTestApp()
	id := create(t, a, "ORD4", srv.URL, 1000)
	pay(a, id, "silent")
	if len(got()) != 0 {
		t.Fatal("webhook sent")
	}
	if code, in := inquiry(t, a, "ORD4"); code != 200 || in["status"] != "paid" || in["amount"] != float64(1000) {
		t.Fatal(code, in)
	}
}

func TestAmountMismatch(t *testing.T) {
	srv, got := receiver()
	defer srv.Close()
	a := newTestApp()
	id := create(t, a, "ORD5", srv.URL, 50000)
	pay(a, id, "mismatch")
	if _, in := inquiry(t, a, "ORD5"); in["amount"] != float64(49900) || in["status"] != "paid" {
		t.Fatal(in)
	}
	var ev map[string]any
	_ = json.Unmarshal(got()[0].body, &ev)
	if ev["amount"] != float64(49900) {
		t.Fatal(ev)
	}
}

func TestInquiryUnknownRef404(t *testing.T) {
	a := newTestApp()
	if code, in := inquiry(t, a, "nope"); code != 404 || in["error"] != "not_found" {
		t.Fatal(code, in)
	}
}

func TestRefundExceedsPaid422(t *testing.T) {
	srv, _ := receiver()
	defer srv.Close()
	a := newTestApp()
	id := create(t, a, "ORD6", srv.URL, 1000)
	pay(a, id, "success")
	if rec := kb(a, "POST", "/pg/refunds", `{"paymentId":"`+id+`","amount":1001}`); rec.Code != 422 {
		t.Fatal(rec.Code)
	}
	if rec := kb(a, "POST", "/pg/refunds", `{"paymentId":"`+id+`","amount":1000}`); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if rec := kb(a, "POST", "/pg/refunds", `{"paymentId":"x-slow","amount":5}`); rec.Code != 202 {
		t.Fatal(rec.Code)
	}
}

func TestPaymentURLUsesPublicURL(t *testing.T) {
	a := newTestApp()
	a.PublicURL = "http://mockext:8090"
	b := `{"ref":"R","amount":100,"currency":"THB","method":"promptpay","returnUrl":"x","webhookUrl":"y"}`
	rec := kb(a, "POST", "/pg/payments", b)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["paymentUrl"] != "http://mockext:8090/pg/checkout/"+out["paymentId"].(string) || out["promptpayPayload"] == nil {
		t.Fatal(out)
	}
	co := kb(a, "GET", "/pg/checkout/"+out["paymentId"].(string), "")
	if co.Code != 200 || !strings.Contains(co.Body.String(), "KBank Payment Gateway (จำลอง)") || !strings.Contains(co.Body.String(), "1.00 บาท") {
		t.Fatal(co.Code, co.Body.String())
	}
}

func TestRequiresAPIKey(t *testing.T) {
	a := newTestApp()
	mux := http.NewServeMux()
	a.Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/pg/payments?ref=x", nil))
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
}
