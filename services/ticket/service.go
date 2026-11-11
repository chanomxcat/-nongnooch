package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
)

const dateLayout = "2006-01-02"

// ict is Bangkok time (distroless has no tzdata).
var ict = time.FixedZone("ICT", 7*3600)

// Service issues tickets from order.paid and checks them at the gate.
type Service struct {
	DB     *sql.DB
	Signer *Signer
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) today() string { return s.now().In(ict).Format(dateLayout) }

// TicketView is one ticket as returned by the API.
type TicketView struct {
	TicketID   string `json:"ticketId"`
	OrderID    string `json:"orderId"`
	Kind       string `json:"kind"`
	VisitDate  string `json:"visitDate,omitempty"`
	HolderName string `json:"holderName"`
	IDLast4    string `json:"idLast4"`
	Status     string `json:"status"`
	UsedAt     string `json:"usedAt,omitempty"`
	QR         string `json:"qr"`
}

type orderPaid struct {
	OrderID  string     `json:"orderId"`
	OrderNo  string     `json:"orderNo"`
	MemberID string     `json:"memberId"`
	Guest    string     `json:"guestContact"`
	Lang     string     `json:"lang"`
	Items    []paidItem `json:"items"`
}

type paidItem struct {
	OrderItemID string `json:"orderItemId"`
	Kind        string `json:"kind"`
	VisitDate   string `json:"visitDate"`
	Holder      struct {
		Name    string `json:"name"`
		IDLast4 string `json:"idLast4"`
	} `json:"holder"`
}

// Issue handles one order.paid event inside the consumer transaction.
// Idempotent by order item: a replay (even with a new event id) adds nothing.
func (s *Service) Issue(ctx context.Context, tx *sql.Tx, env events.Envelope) error {
	var op orderPaid
	if err := json.Unmarshal(env.Payload, &op); err != nil {
		return err
	}
	type issued struct {
		TicketID   string `json:"ticketId"`
		Kind       string `json:"kind"`
		VisitDate  string `json:"visitDate,omitempty"`
		HolderName string `json:"holderName"`
	}
	var out []issued
	now := s.now().UTC()
	for _, it := range op.Items {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		var visit *string
		if it.VisitDate != "" {
			v := it.VisitDate
			visit = &v
		}
		qr := s.Signer.Sign(QRPayload{T: id.String(), K: it.Kind, D: visit, H: it.Holder.IDLast4})
		var member any
		if op.MemberID != "" {
			member = op.MemberID
		}
		res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO tickets(id, order_id, order_item_id, member_id, kind, visit_date, holder_name, id_last4, status, qr, created_at)
			VALUES(?,?,?,?,?,?,?,?, 'active', ?, ?)`,
			id.String(), op.OrderID, it.OrderItemID, member, it.Kind, visit, it.Holder.Name, it.Holder.IDLast4, qr, now)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		out = append(out, issued{id.String(), it.Kind, it.VisitDate, it.Holder.Name})
	}
	if len(out) == 0 {
		return nil
	}
	payload := map[string]any{"orderId": op.OrderID, "orderNo": op.OrderNo, "lang": op.Lang, "tickets": out}
	if op.MemberID != "" {
		payload["memberId"] = op.MemberID
	}
	if op.Guest != "" {
		payload["guestContact"] = op.Guest
	}
	e, err := events.New(events.TicketIssued, payload)
	if err != nil {
		return err
	}
	return outbox.Write(ctx, tx, e)
}

const ticketCols = `id, order_id, kind, IFNULL(DATE_FORMAT(visit_date,'%Y-%m-%d'),''), holder_name, id_last4, status, used_at, qr`

type scanner interface{ Scan(...any) error }

func scanTicket(r scanner) (TicketView, error) {
	var t TicketView
	var used sql.NullTime
	if err := r.Scan(&t.TicketID, &t.OrderID, &t.Kind, &t.VisitDate, &t.HolderName, &t.IDLast4, &t.Status, &used, &t.QR); err != nil {
		return t, err
	}
	if used.Valid {
		t.UsedAt = used.Time.UTC().Format(time.RFC3339Nano)
	}
	return t, nil
}

func (s *Service) list(ctx context.Context, where string, args ...any) ([]TicketView, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+ticketCols+` FROM tickets WHERE `+where+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TicketView{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Service) ByMember(ctx context.Context, memberID string) ([]TicketView, error) {
	return s.list(ctx, `member_id=?`, memberID)
}

func (s *Service) ByOrder(ctx context.Context, orderID string) ([]TicketView, error) {
	return s.list(ctx, `order_id=?`, orderID)
}

// Void voids active tickets of an order (all, or only the given order items).
func (s *Service) Void(ctx context.Context, orderID string, itemIDs []string) (int64, error) {
	q := `UPDATE tickets SET status='void' WHERE order_id=? AND status='active'`
	args := []any{orderID}
	if len(itemIDs) > 0 {
		q += ` AND order_item_id IN (?` + strings.Repeat(",?", len(itemIDs)-1) + `)`
		for _, id := range itemIDs {
			args = append(args, id)
		}
	}
	r, err := s.DB.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

// SnapshotTicket is the gate's offline-capable view of a ticket.
type SnapshotTicket struct {
	TicketID  string `json:"ticketId"`
	Kind      string `json:"kind"`
	VisitDate string `json:"visitDate,omitempty"`
	Status    string `json:"status"`
}

func (s *Service) Snapshot(ctx context.Context, date string) (map[string]any, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, kind, IFNULL(DATE_FORMAT(visit_date,'%Y-%m-%d'),''), status FROM tickets
		WHERE visit_date=? OR visit_date IS NULL ORDER BY id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ts := []SnapshotTicket{}
	for rows.Next() {
		var t SnapshotTicket
		if err := rows.Scan(&t.TicketID, &t.Kind, &t.VisitDate, &t.Status); err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return map[string]any{"date": date, "publicKeys": s.Signer.PublicKeys(), "tickets": ts}, rows.Err()
}

// ScanInput is a gate scan: a QR string or (manual entry) a ticket id.
type ScanInput struct {
	QR       string `json:"qr"`
	TicketID string `json:"ticketId"`
	GateID   string `json:"gateId"`
}

// ScanResult is the gate's verdict.
type ScanResult struct {
	Result     string `json:"result"`
	TicketID   string `json:"ticketId,omitempty"`
	Kind       string `json:"kind,omitempty"`
	HolderName string `json:"holderName,omitempty"`
	IDLast4    string `json:"idLast4,omitempty"`
	VisitDate  string `json:"visitDate,omitempty"`
	UsedAt     string `json:"usedAt,omitempty"`
}

var errBadScan = errors.New("qr or ticketId required")

// Scan verifies and consumes a ticket. The row lock (SELECT ... FOR UPDATE) makes
// concurrent scans of one ticket serialize, so exactly one returns ok.
func (s *Service) Scan(ctx context.Context, in ScanInput) (ScanResult, error) {
	method, ticketID := "qr", ""
	switch {
	case in.QR != "":
		p, ok := s.Signer.Verify(in.QR)
		if !ok {
			return ScanResult{Result: "invalid"}, s.logScan(ctx, in.GateID, "invalid", method)
		}
		ticketID = p.T
	case in.TicketID != "":
		method, ticketID = "manual", in.TicketID
	default:
		return ScanResult{}, errBadScan
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ScanResult{}, err
	}
	defer tx.Rollback()
	var memberID sql.NullString
	var t TicketView
	var used sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT `+ticketCols+`, member_id FROM tickets WHERE id=? FOR UPDATE`, ticketID).
		Scan(&t.TicketID, &t.OrderID, &t.Kind, &t.VisitDate, &t.HolderName, &t.IDLast4, &t.Status, &used, &t.QR, &memberID)
	if errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return ScanResult{Result: "invalid"}, s.logScan(ctx, in.GateID, "invalid", method)
	}
	if err != nil {
		return ScanResult{}, err
	}
	now := s.now().UTC()
	res := ScanResult{TicketID: t.TicketID, Kind: t.Kind, HolderName: t.HolderName, IDLast4: t.IDLast4, VisitDate: t.VisitDate}
	duplicate := false
	switch {
	case t.Status == "void":
		res.Result = "void"
	case t.Status == "used":
		res.Result, duplicate = "used", true
		res.UsedAt = used.Time.UTC().Format(time.RFC3339Nano)
	case t.VisitDate != "" && t.VisitDate != s.today():
		res.Result = "wrong_date"
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET status='used', used_at=? WHERE id=? AND status='active'`, now, t.TicketID); err != nil {
			return ScanResult{}, err
		}
		res.Result, res.UsedAt = "ok", now.Format(time.RFC3339Nano)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO scans(ticket_id, gate_id, result, method, scanned_at) VALUES(?,?,?,?,?)`,
		t.TicketID, in.GateID, res.Result, method, now); err != nil {
		return ScanResult{}, err
	}
	payload := map[string]any{"ticketId": t.TicketID, "kind": t.Kind, "gateId": in.GateID, "scannedAt": now,
		"result": res.Result, "method": method, "offline": false, "duplicate": duplicate}
	if memberID.Valid {
		payload["memberId"] = memberID.String
	}
	e, err := events.New(events.TicketScanned, payload)
	if err != nil {
		return ScanResult{}, err
	}
	if err := outbox.Write(ctx, tx, e); err != nil {
		return ScanResult{}, err
	}
	return res, tx.Commit()
}

// logScan records a scan that matched no ticket (no event: nothing trustworthy to report).
func (s *Service) logScan(ctx context.Context, gate, result, method string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO scans(ticket_id, gate_id, result, method, scanned_at) VALUES(NULL,?,?,?,?)`,
		gate, result, method, s.now().UTC())
	return err
}

// Stats counts accepted entries and issued tickets for the Bangkok day.
func (s *Service) Stats(ctx context.Context) (map[string]int, error) {
	n := s.now().In(ict)
	from := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, ict).UTC()
	to := from.Add(24 * time.Hour)
	var scans, issued int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM scans WHERE result='ok' AND scanned_at>=? AND scanned_at<?`, from, to).Scan(&scans); err != nil {
		return nil, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE created_at>=? AND created_at<?`, from, to).Scan(&issued); err != nil {
		return nil, err
	}
	return map[string]int{"scansToday": scans, "ticketsIssuedToday": issued}, nil
}
