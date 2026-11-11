package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
)

// KBank is the (mock) KBank payment gateway client.
type KBank struct {
	BaseURL, APIKey string
	HTTP            *http.Client
}

// CreatePayment registers a payment; ref is our payment id.
func (k *KBank) CreatePayment(ctx context.Context, ref string, amount int64, method, returnURL, webhookURL string) (id, url string, err error) {
	body, _ := json.Marshal(map[string]any{"ref": ref, "amount": amount, "currency": "THB", "method": method, "returnUrl": returnURL, "webhookUrl": webhookURL})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.BaseURL+"/payments", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+k.APIKey)
	req.Header.Set("Content-Type", "application/json")
	c := k.HTTP
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var out struct {
		PaymentID  string `json:"paymentId"`
		PaymentURL string `json:"paymentUrl"`
	}
	if resp.StatusCode/100 != 2 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || out.PaymentURL == "" {
		return "", "", fmt.Errorf("kbank create payment: status %d", resp.StatusCode)
	}
	return out.PaymentID, out.PaymentURL, nil
}

// CreatePayment starts a payment for a pending order owned by memberID.
func (s *Service) CreatePayment(ctx context.Context, tx *sql.Tx, memberID, orderID, method, returnURL string) (result, error) {
	if method != "card" && method != "promptpay" {
		return fail(http.StatusBadRequest, "bad_request"), nil
	}
	var status string
	var total int64
	var hold time.Time
	err := tx.QueryRowContext(ctx, `SELECT status, total_satang, hold_expires_at FROM orders WHERE id=? AND member_id=? FOR UPDATE`, orderID, memberID).Scan(&status, &total, &hold)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(http.StatusNotFound, "order_not_found"), nil
	}
	if err != nil {
		return result{}, err
	}
	if status != "pending" || !hold.After(s.now().UTC()) {
		return fail(http.StatusConflict, "order_not_payable"), nil
	}
	pid := uuid.NewString()
	kid, url, err := s.KBank.CreatePayment(ctx, pid, total, method, returnURL, s.WebhookURL)
	if err != nil {
		return fail(http.StatusBadGateway, "payment_provider_unavailable"), nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO payments(id, order_id, amount_satang, method, status, kbank_payment_id, created_at) VALUES(?,?,?,?,?,?,?)`,
		pid, orderID, total, method, "pending", kid, s.now().UTC()); err != nil {
		return result{}, err
	}
	return ok(http.StatusCreated, map[string]string{"paymentId": pid, "paymentUrl": url}), nil
}

// Sign returns hex(HMAC-SHA256(secret, body)), the X-Mock-Signature value.
func Sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Service) validSignature(body []byte, sig string) bool {
	return hmac.Equal([]byte(Sign(s.WebhookSecret, body)), []byte(sig))
}

type webhookBody struct {
	EventID   string `json:"eventId"`
	PaymentID string `json:"paymentId"`
	Ref       string `json:"ref"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
}

// HandleWebhook applies a verified KBank event. Idempotent by eventId: payment paid, order confirmed,
// quota committed and the order.paid outbox row are written in one transaction.
func (s *Service) HandleWebhook(ctx context.Context, wb webhookBody) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO webhook_events(event_id, received_at) VALUES(?,?)`, wb.EventID, s.now().UTC()); err != nil {
		if isDup(err) {
			return nil // replay
		}
		return err
	}
	var orderID, pstatus string
	var expected int64
	err = tx.QueryRowContext(ctx, `SELECT order_id, status, amount_satang FROM payments WHERE id=? FOR UPDATE`, wb.Ref).Scan(&orderID, &pstatus, &expected)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit() // unknown ref: ack so the provider stops retrying
	}
	if err != nil {
		return err
	}
	if pstatus != "pending" {
		return tx.Commit() // already settled by an earlier event
	}
	if wb.Status != "paid" {
		if _, err := tx.ExecContext(ctx, `UPDATE payments SET status=? WHERE id=?`, wb.Status, wb.Ref); err != nil {
			return err
		}
		return tx.Commit()
	}
	discrepancy := func(status string, expectedSatang int64) error {
		if _, err := tx.ExecContext(ctx, `UPDATE payments SET status=? WHERE id=?`, status, wb.Ref); err != nil {
			return err
		}
		env, err := events.New(events.PaymentDiscrepancy, map[string]any{"paymentId": wb.Ref, "orderId": orderID, "expectedSatang": expectedSatang, "actualSatang": wb.Amount})
		if err != nil {
			return err
		}
		if err := outbox.Write(ctx, tx, env); err != nil {
			return err
		}
		return tx.Commit()
	}
	if wb.Amount != expected {
		return discrepancy("mismatch", expected)
	}

	var ostatus, orderNo, memberID, lang, visit string
	var total int64
	var day int
	err = tx.QueryRowContext(ctx, `SELECT status, order_no, member_id, lang, DATE_FORMAT(visit_date,'%Y-%m-%d'), total_satang, day_tickets FROM orders WHERE id=? FOR UPDATE`, orderID).
		Scan(&ostatus, &orderNo, &memberID, &lang, &visit, &total, &day)
	if err != nil {
		return err
	}
	switch ostatus {
	case "pending":
		if day > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE quota SET held=held-?, sold=sold+? WHERE visit_date=?`, day, day, visit); err != nil {
				return err
			}
		}
	case "expired":
		// Paid after the hold lapsed: take the quota again if still available, otherwise flag for refund.
		if day > 0 {
			r, err := tx.ExecContext(ctx, `UPDATE quota SET sold=sold+? WHERE visit_date=? AND total-held-sold>=?`, day, visit, day)
			if err != nil {
				return err
			}
			if n, _ := r.RowsAffected(); n == 0 {
				return discrepancy("paid_unfulfilled", 0)
			}
		}
	default:
		return discrepancy("paid_duplicate", 0)
	}

	now := s.now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE payments SET status='paid', paid_at=? WHERE id=?`, now, wb.Ref); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='confirmed', paid_at=? WHERE id=?`, now, orderID); err != nil {
		return err
	}
	type holder struct {
		Name        string `json:"name"`
		Nationality string `json:"nationality"`
		IDLast4     string `json:"idLast4"`
		ThaiPrice   bool   `json:"thaiPrice"`
	}
	type item struct {
		OrderItemID string `json:"orderItemId"`
		ProductID   string `json:"productId"`
		Kind        string `json:"kind"`
		VisitDate   string `json:"visitDate,omitempty"`
		Holder      holder `json:"holder"`
		PriceSatang int64  `json:"priceSatang"`
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, product_id, kind, price_satang, holder_name, holder_nationality, holder_id_last4, thai_price FROM order_items WHERE order_id=? ORDER BY id`, orderID)
	if err != nil {
		return err
	}
	items := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.OrderItemID, &it.ProductID, &it.Kind, &it.PriceSatang, &it.Holder.Name, &it.Holder.Nationality, &it.Holder.IDLast4, &it.Holder.ThaiPrice); err != nil {
			rows.Close()
			return err
		}
		if it.Kind == "day" {
			it.VisitDate = visit
		}
		items = append(items, it)
	}
	rows.Close()
	env, err := events.New(events.OrderPaid, map[string]any{
		"orderId": orderID, "orderNo": orderNo, "memberId": memberID, "lang": lang,
		"paidAmountSatang": total, "paymentId": wb.Ref, "items": items,
	})
	if err != nil {
		return err
	}
	if err := outbox.Write(ctx, tx, env); err != nil {
		return err
	}
	return tx.Commit()
}
