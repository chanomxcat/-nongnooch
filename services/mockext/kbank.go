package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Payment is one KBank PG payment (amounts in satang).
type Payment struct {
	ID, Ref, Method, ReturnURL, WebhookURL, Status string
	Amount, Paid, Refunded                        int
	EventID                                       string
	Body                                          []byte // last webhook body, re-sent verbatim
}

func (p Payment) BahtAmount() float64 { return float64(p.Amount) / 100 }

var retryDelays = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

func (a *App) paymentList() []Payment {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Payment, 0, len(a.order))
	for i := len(a.order) - 1; i >= 0; i-- {
		out = append(out, *a.payments[a.order[i]])
	}
	return out
}

func (a *App) authed(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer "+a.APIKey {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return false
	}
	return true
}

// sign returns hex(HMAC-SHA256(secret, body)).
func sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// buildWebhook freezes a new event for p (caller holds no lock).
func (a *App) buildWebhook(p *Payment, status string, amount int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p.EventID = newID("evt_")
	p.Body, _ = json.Marshal(map[string]any{
		"eventId": p.EventID, "paymentId": p.ID, "ref": p.Ref, "status": status,
		"amount": amount, "paidAt": time.Now().UTC().Format(time.RFC3339),
	})
}

// send delivers the stored webhook body with signature; retries on non-2xx.
func (a *App) send(p Payment) {
	run := func() {
		for attempt := 0; ; attempt++ {
			req, err := http.NewRequest("POST", p.WebhookURL, bytes.NewReader(p.Body))
			ok := false
			note := ""
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Mock-Signature", sign(a.Secret, p.Body))
				resp, derr := a.Client.Do(req)
				if derr != nil {
					note = derr.Error()
				} else {
					resp.Body.Close()
					ok = resp.StatusCode/100 == 2
					note = fmt.Sprint(resp.StatusCode)
				}
			} else {
				note = err.Error()
			}
			a.Store.Add("webhook", Entry{Title: p.Ref + " " + p.EventID, Body: fmt.Sprintf("attempt %d -> %s", attempt+1, note)})
			if ok || attempt >= len(retryDelays) {
				return
			}
			a.Sleep(retryDelays[attempt])
		}
	}
	if a.Sync {
		run()
	} else {
		go run()
	}
}

func (a *App) get(id string) *Payment {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.payments[id]
}

func (a *App) kbankRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /pg/payments", func(w http.ResponseWriter, r *http.Request) {
		if !a.authed(w, r) {
			return
		}
		var in struct {
			Ref, Currency, Method, ReturnURL, WebhookURL string
			Amount                                       int
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Ref == "" || in.Amount <= 0 {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		p := &Payment{ID: newID("pay_"), Ref: in.Ref, Method: in.Method, ReturnURL: in.ReturnURL,
			WebhookURL: in.WebhookURL, Amount: in.Amount, Status: "pending"}
		a.mu.Lock()
		a.payments[p.ID] = p
		a.order = append(a.order, p.ID)
		a.mu.Unlock()
		out := map[string]any{"paymentId": p.ID, "status": "pending", "paymentUrl": a.PublicURL + "/pg/checkout/" + p.ID}
		if in.Method == "promptpay" {
			out["promptpayPayload"] = fmt.Sprintf("00020101021230MOCK-PROMPTPAY-%s-%d", p.ID, p.Amount)
		}
		writeJSON(w, 201, out)
	})

	mux.HandleFunc("GET /pg/payments", func(w http.ResponseWriter, r *http.Request) {
		if !a.authed(w, r) {
			return
		}
		ref := r.URL.Query().Get("ref")
		a.mu.Lock()
		defer a.mu.Unlock()
		for i := len(a.order) - 1; i >= 0; i-- {
			p := a.payments[a.order[i]]
			if p.Ref == ref {
				amt := p.Amount
				if p.Status == "paid" {
					amt = p.Paid
				}
				writeJSON(w, 200, map[string]any{"paymentId": p.ID, "ref": p.Ref, "status": p.Status, "amount": amt})
				return
			}
		}
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	})

	mux.HandleFunc("GET /pg/checkout/{id}", func(w http.ResponseWriter, r *http.Request) {
		p := a.get(r.PathValue("id"))
		if p == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		a.mu.Lock()
		cp := *p
		a.mu.Unlock()
		payload := ""
		if cp.Method == "promptpay" {
			payload = fmt.Sprintf("00020101021230MOCK-PROMPTPAY-%s-%d", cp.ID, cp.Amount)
		}
		_ = tpl.ExecuteTemplate(w, "checkout.html", map[string]any{"P": cp, "Payload": payload})
	})

	mux.HandleFunc("POST /pg/checkout/{id}/pay", func(w http.ResponseWriter, r *http.Request) {
		p := a.get(r.PathValue("id"))
		if p == nil {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		action := r.PostForm.Get("action")
		a.mu.Lock()
		pending := p.Status == "pending"
		a.mu.Unlock()
		if pending {
			status, paid, hook := "", p.Amount, true
			switch action {
			case "success":
				status = "paid"
			case "silent":
				status, hook = "paid", false
			case "mismatch":
				status, paid = "paid", p.Amount-100
			case "fail":
				status = "failed"
			default:
				status = "cancelled"
			}
			if paid <= 0 {
				paid = p.Amount
			}
			a.mu.Lock()
			p.Status = status
			if status == "paid" {
				p.Paid = paid
			}
			a.mu.Unlock()
			a.Store.Add("payment", Entry{Title: p.Ref, Body: action})
			if hook {
				amt := p.Amount
				if status == "paid" {
					amt = paid
				}
				a.buildWebhook(p, status, amt)
				a.mu.Lock()
				cp := *p
				a.mu.Unlock()
				a.send(cp)
			}
		}
		dest := p.ReturnURL
		if dest == "" {
			dest = "/"
		} else {
			sep := "?"
			if strings.Contains(dest, "?") {
				sep = "&"
			}
			dest += sep + "ref=" + url.QueryEscape(p.Ref)
		}
		http.Redirect(w, r, dest, http.StatusSeeOther)
	})

	mux.HandleFunc("POST /pg/payments/{id}/webhook:resend", func(w http.ResponseWriter, r *http.Request) {
		p := a.get(r.PathValue("id"))
		if p == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		a.mu.Lock()
		status, paid := p.Status, p.Paid
		a.mu.Unlock()
		if status == "pending" {
			writeJSON(w, 409, map[string]string{"error": "not_final"})
			return
		}
		a.mu.Lock()
		noEvent := p.EventID == ""
		a.mu.Unlock()
		if noEvent { // payment was completed without a webhook: create the first event now
			amt := p.Amount
			if status == "paid" {
				amt = paid
			}
			a.buildWebhook(p, status, amt)
		}
		a.mu.Lock()
		cp := *p
		a.mu.Unlock()
		a.send(cp)
		if strings.Contains(r.Header.Get("Accept"), "text/html") || r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		writeJSON(w, 202, map[string]string{"eventId": cp.EventID})
	})

	mux.HandleFunc("POST /pg/refunds", func(w http.ResponseWriter, r *http.Request) {
		if !a.authed(w, r) {
			return
		}
		var in struct {
			PaymentID string
			Amount    int
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Amount <= 0 {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		if strings.HasSuffix(in.PaymentID, "-slow") {
			writeJSON(w, 202, map[string]string{"refundId": newID("ref_"), "status": "pending"})
			return
		}
		p := a.get(in.PaymentID)
		if p == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if p.Status != "paid" || in.Amount > p.Paid-p.Refunded {
			writeJSON(w, 422, map[string]string{"error": "amount_exceeds_paid"})
			return
		}
		p.Refunded += in.Amount
		writeJSON(w, 200, map[string]string{"refundId": newID("ref_"), "status": "succeeded"})
	})
}
