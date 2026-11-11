// Package main is the commerce service: catalog, orders (with quota holds) and KBank payment.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
)

// ict is Asia/Bangkok (UTC+7, no DST). A fixed zone avoids needing tzdata in the distroless image.
var ict = time.FixedZone("ICT", 7*3600)

const (
	holdTTL    = 15 * time.Minute
	idemTTL    = 24 * time.Hour
	dateLayout = "2006-01-02"
)

// Holder is a ticket holder as known to the member service.
type Holder struct {
	Name, Nationality, IDLast4 string
	ThaiVerified               bool
}

// MemberClient is the slice of MemberService commerce needs.
type MemberClient interface {
	GetCompanion(ctx context.Context, memberID, companionID string) (Holder, error)
}

// Service holds the commerce dependencies.
type Service struct {
	DB            *sql.DB
	Member        MemberClient
	KBank         *KBank
	WebhookSecret string
	WebhookURL    string
	Now           func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// result is an HTTP outcome; bodies of non-2xx results are error codes.
type result struct {
	Status int
	Body   []byte
}

func ok(status int, v any) result {
	b, _ := json.Marshal(v)
	return result{status, b}
}

func fail(status int, code string) result {
	return ok(status, map[string]string{"error": code, "message": code})
}

func isDup(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// runIdem runs fn in a transaction. With a key, the first caller claims it (a concurrent duplicate
// blocks on the unique row until the first commits) and later callers within 24 h get the stored
// response. Failed (non-2xx) results roll back, so the key stays unused.
// runIdemOnce: see runIdem.
func (s *Service) runIdemOnce(ctx context.Context, member, endpoint, key string, fn func(tx *sql.Tx) (result, error)) (result, error) {
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return result{}, err
		}
		if key != "" {
			now := s.now().UTC()
			if _, err := tx.ExecContext(ctx, `DELETE FROM idempotency WHERE member_id=? AND endpoint=? AND idem_key=? AND created_at < ?`,
				member, endpoint, key, now.Add(-idemTTL)); err != nil {
				tx.Rollback()
				return result{}, err
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO idempotency(member_id, endpoint, idem_key, created_at) VALUES(?,?,?,?)`, member, endpoint, key, now)
			if isDup(err) {
				tx.Rollback()
				var st int
				var body string
				qerr := s.DB.QueryRowContext(ctx, `SELECT status, body FROM idempotency WHERE member_id=? AND endpoint=? AND idem_key=? AND status>0`,
					member, endpoint, key).Scan(&st, &body)
				if qerr == nil {
					return result{st, []byte(body)}, nil
				}
				if errors.Is(qerr, sql.ErrNoRows) {
					continue // the first attempt rolled back; try to claim the key ourselves
				}
				return result{}, qerr
			}
			if err != nil {
				tx.Rollback()
				return result{}, err
			}
		}
		res, err := fn(tx)
		if err != nil || res.Status >= 300 {
			tx.Rollback()
			return res, err
		}
		if key != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE idempotency SET status=?, body=? WHERE member_id=? AND endpoint=? AND idem_key=?`,
				res.Status, string(res.Body), member, endpoint, key); err != nil {
				tx.Rollback()
				return result{}, err
			}
		}
		return res, tx.Commit()
	}
	return fail(http.StatusConflict, "idempotency_conflict"), nil
}

type orderItemReq struct {
	ProductID string `json:"productId"`
	Audience  string `json:"audience"`
	Holder    struct {
		CompanionID string `json:"companionId"`
		Name        string `json:"name"`
		Nationality string `json:"nationality"`
		IDLast4     string `json:"idLast4"`
	} `json:"holder"`
}

type orderReq struct {
	VisitDate string         `json:"visitDate"`
	Items     []orderItemReq `json:"items"`
}

type lineItem struct {
	ID, ProductID, Kind, Audience, Name, Nationality, IDLast4 string
	Price                                                     int64
	ThaiPrice                                                 bool
}

func (s *Service) nextOrderNo(ctx context.Context, now time.Time) (string, error) {
	day := now.In(ict).Format("060102")
	res, err := s.DB.ExecContext(ctx, `INSERT INTO order_seq(day, n) VALUES(?, LAST_INSERT_ID(1)) ON DUPLICATE KEY UPDATE n=LAST_INSERT_ID(n+1)`, day)
	if err != nil {
		return "", err
	}
	n, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("NN%s%06d", day, n), nil
}

// CreateOrder prices the items, holds quota and inserts a pending order.
func (s *Service) CreateOrder(ctx context.Context, tx *sql.Tx, memberID, lang string, req orderReq) (result, error) {
	now := s.now()
	visit, err := time.ParseInLocation(dateLayout, req.VisitDate, ict)
	if err != nil || len(req.Items) == 0 || len(req.Items) > 20 {
		return fail(http.StatusBadRequest, "bad_request"), nil
	}
	if req.VisitDate < now.In(ict).Format(dateLayout) {
		return fail(http.StatusUnprocessableEntity, "visit_date_past"), nil
	}
	var me *Holder // lazily fetched own holder data
	var items []lineItem
	var total int64
	dayCount := 0
	for _, it := range req.Items {
		var kind string
		var thai, foreign sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT kind, thai_satang, foreign_satang FROM products WHERE id=?`, it.ProductID).Scan(&kind, &thai, &foreign)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(http.StatusUnprocessableEntity, "product_not_found"), nil
		}
		if err != nil {
			return result{}, err
		}
		li := lineItem{ID: uuid.NewString(), ProductID: it.ProductID, Kind: kind, Audience: it.Audience}
		switch it.Audience {
		case "thai", "foreign":
		default:
			return fail(http.StatusBadRequest, "bad_request"), nil
		}
		if it.Holder.CompanionID != "" {
			if it.Holder.CompanionID != "me" {
				return fail(http.StatusUnprocessableEntity, "holder_not_found"), nil
			}
			if me == nil {
				h, err := s.Member.GetCompanion(ctx, memberID, "me")
				if err != nil {
					return result{}, err
				}
				me = &h
			}
			li.Name, li.Nationality, li.IDLast4 = me.Name, me.Nationality, me.IDLast4
			li.ThaiPrice = me.ThaiVerified
		} else {
			// Ad-hoc holders can never get the Thai price: nothing verifies their nationality.
			if it.Holder.Name == "" || len(it.Holder.Nationality) != 2 || len(it.Holder.IDLast4) != 4 {
				return fail(http.StatusBadRequest, "bad_request"), nil
			}
			li.Name, li.Nationality, li.IDLast4 = it.Holder.Name, it.Holder.Nationality, it.Holder.IDLast4
		}
		price := foreign
		if it.Audience == "thai" {
			if !li.ThaiPrice {
				return fail(http.StatusUnprocessableEntity, "thai_price_not_allowed"), nil
			}
			price = thai
		} else {
			li.ThaiPrice = false
		}
		if !price.Valid {
			return fail(http.StatusUnprocessableEntity, "price_unavailable"), nil
		}
		li.Price = price.Int64
		total += li.Price
		if kind == "day" {
			dayCount++
		}
		items = append(items, li)
	}
	if dayCount > 0 {
		r, err := tx.ExecContext(ctx, `UPDATE quota SET held=held+? WHERE visit_date=? AND total-held-sold>=?`, dayCount, req.VisitDate, dayCount)
		if err != nil {
			return result{}, err
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return fail(http.StatusConflict, "sold_out"), nil
		}
	}
	orderNo, err := s.nextOrderNo(ctx, now)
	if err != nil {
		return result{}, err
	}
	id := uuid.NewString()
	hold := now.Add(holdTTL)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO orders(id, order_no, member_id, lang, visit_date, status, total_satang, day_tickets, hold_expires_at, created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		id, orderNo, memberID, lang, visit.Format(dateLayout), "pending", total, dayCount, hold.UTC(), now.UTC()); err != nil {
		return result{}, err
	}
	for _, li := range items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO order_items(id, order_id, product_id, kind, audience, price_satang, holder_name, holder_nationality, holder_id_last4, thai_price) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			li.ID, id, li.ProductID, li.Kind, li.Audience, li.Price, li.Name, li.Nationality, li.IDLast4, li.ThaiPrice); err != nil {
			return result{}, err
		}
	}
	return ok(http.StatusCreated, map[string]any{
		"orderId": id, "orderNo": orderNo, "totalSatang": total, "holdExpiresAt": hold.UTC().Format(time.RFC3339),
	}), nil
}

// ReleaseExpired expires unpaid orders past their hold, returning quota (outbox order.expired).
func (s *Service) ReleaseExpired(ctx context.Context) (int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM orders WHERE status='pending' AND hold_expires_at < ? LIMIT 200`, s.now().UTC())
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	n := 0
	for _, id := range ids {
		done, err := s.expireOne(ctx, id)
		if err != nil {
			return n, err
		}
		if done {
			n++
		}
	}
	return n, nil
}

func (s *Service) expireOne(ctx context.Context, id string) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var status, visit string
	var day int
	var hold time.Time
	err = tx.QueryRowContext(ctx, `SELECT status, DATE_FORMAT(visit_date,'%Y-%m-%d'), day_tickets, hold_expires_at FROM orders WHERE id=? FOR UPDATE`, id).Scan(&status, &visit, &day, &hold)
	if err != nil {
		return false, err
	}
	if status != "pending" || !hold.Before(s.now().UTC()) {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='expired' WHERE id=?`, id); err != nil {
		return false, err
	}
	if day > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE quota SET held=held-? WHERE visit_date=?`, day, visit); err != nil {
			return false, err
		}
	}
	env, err := events.New(events.OrderExpired, map[string]string{"orderId": id, "reason": "hold_timeout"})
	if err != nil {
		return false, err
	}
	if err := outbox.Write(ctx, tx, env); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

type orderView struct {
	OrderID    string     `json:"orderId"`
	OrderNo    string     `json:"orderNo"`
	Status     string     `json:"status"`
	Total      int64      `json:"totalSatang"`
	VisitDate  string     `json:"visitDate"`
	HoldExpiry string     `json:"holdExpiresAt"`
	PaidAt     string     `json:"paidAt,omitempty"`
	Items      []itemView `json:"items"`
}

type itemView struct {
	OrderItemID string `json:"orderItemId"`
	ProductID   string `json:"productId"`
	Kind        string `json:"kind"`
	Audience    string `json:"audience"`
	PriceSatang int64  `json:"priceSatang"`
	Holder      struct {
		Name        string `json:"name"`
		Nationality string `json:"nationality"`
		IDLast4     string `json:"idLast4"`
	} `json:"holder"`
}

// Orders returns the member's orders (just orderID when given), newest first.
func (s *Service) Orders(ctx context.Context, memberID, orderID string) ([]orderView, error) {
	q := `SELECT id, order_no, status, total_satang, DATE_FORMAT(visit_date,'%Y-%m-%d'), hold_expires_at, paid_at FROM orders WHERE member_id=?`
	args := []any{memberID}
	if orderID != "" {
		q += ` AND id=?`
		args = append(args, orderID)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY created_at DESC LIMIT 100`, args...)
	if err != nil {
		return nil, err
	}
	out := []orderView{}
	for rows.Next() {
		var o orderView
		var hold time.Time
		var paid sql.NullTime
		if err := rows.Scan(&o.OrderID, &o.OrderNo, &o.Status, &o.Total, &o.VisitDate, &hold, &paid); err != nil {
			rows.Close()
			return nil, err
		}
		o.HoldExpiry = hold.UTC().Format(time.RFC3339)
		if paid.Valid {
			o.PaidAt = paid.Time.UTC().Format(time.RFC3339)
		}
		o.Items = []itemView{}
		out = append(out, o)
	}
	rows.Close()
	for i := range out {
		ir, err := s.DB.QueryContext(ctx, `SELECT id, product_id, kind, audience, price_satang, holder_name, holder_nationality, holder_id_last4 FROM order_items WHERE order_id=? ORDER BY id`, out[i].OrderID)
		if err != nil {
			return nil, err
		}
		for ir.Next() {
			var it itemView
			if err := ir.Scan(&it.OrderItemID, &it.ProductID, &it.Kind, &it.Audience, &it.PriceSatang, &it.Holder.Name, &it.Holder.Nationality, &it.Holder.IDLast4); err != nil {
				ir.Close()
				return nil, err
			}
			out[i].Items = append(out[i].Items, it)
		}
		ir.Close()
	}
	return out, nil
}

// Stats returns today's (Asia/Bangkok) confirmed sales.
func (s *Service) Stats(ctx context.Context) (map[string]any, error) {
	n := s.now().In(ict)
	start := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, ict).UTC()
	end := start.Add(24 * time.Hour)
	var sales, orders int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_satang),0), COUNT(*) FROM orders WHERE status='confirmed' AND paid_at>=? AND paid_at<?`, start, end).Scan(&sales, &orders); err != nil {
		return nil, err
	}
	by := map[string]int64{"day": 0, "annual": 0}
	rows, err := s.DB.QueryContext(ctx, `SELECT i.kind, COUNT(*) FROM order_items i JOIN orders o ON o.id=i.order_id WHERE o.status='confirmed' AND o.paid_at>=? AND o.paid_at<? GROUP BY i.kind`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets int64
	for rows.Next() {
		var k string
		var c int64
		if err := rows.Scan(&k, &c); err != nil {
			return nil, err
		}
		by[k] = c
		tickets += c
	}
	return map[string]any{"salesSatang": sales, "orders": orders, "tickets": tickets, "byKind": by}, rows.Err()
}

func isDeadlock(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && (me.Number == 1213 || me.Number == 1205)
}

// runIdem wraps runIdemOnce, retrying when InnoDB picks this transaction as a deadlock victim
// (concurrent duplicates of one Idempotency-Key, or orders racing for the same quota row).
func (s *Service) runIdem(ctx context.Context, member, endpoint, key string, fn func(tx *sql.Tx) (result, error)) (result, error) {
	for i := 0; ; i++ {
		res, err := s.runIdemOnce(ctx, member, endpoint, key, fn)
		if err == nil || !isDeadlock(err) || i >= 5 {
			return res, err
		}
		time.Sleep(time.Duration(10*(i+1)) * time.Millisecond)
	}
}
