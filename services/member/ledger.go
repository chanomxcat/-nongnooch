package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
)

// Entry is one append-only PointLedger row.
type Entry struct {
	ID          string    `json:"id"`
	MemberID    string    `json:"memberId"`
	DeltaPoints int       `json:"deltaPoints"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"`
	RefKind     string    `json:"refKind"`
	RefID       string    `json:"refId"`
	CreatedAt   time.Time `json:"createdAt"`
}

type querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Balance sums available and pending points.
func Balance(ctx context.Context, q querier, memberID string) (available, pending int, err error) {
	err = q.QueryRowContext(ctx, `SELECT
	  COALESCE(SUM(CASE WHEN status='available' THEN delta_points END),0),
	  COALESCE(SUM(CASE WHEN status='pending' THEN delta_points END),0)
	  FROM point_ledger WHERE member_id = ?`, memberID).Scan(&available, &pending)
	return
}

// Append inserts a ledger row and writes points.changed in the same tx.
// A duplicate (ref_kind, ref_id, reason) is a no-op (created=false).
func Append(ctx context.Context, tx *sql.Tx, e Entry) (bool, error) {
	if e.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return false, err
		}
		e.ID = id.String()
	}
	now := time.Now().UTC()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO point_ledger(id, member_id, delta_points, reason, status, ref_kind, ref_id, created_at) VALUES(?,?,?,?,?,?,?,?)`,
		e.ID, e.MemberID, e.DeltaPoints, e.Reason, e.Status, e.RefKind, e.RefID, now)
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	avail, _, err := Balance(ctx, tx, e.MemberID)
	if err != nil {
		return false, err
	}
	ev, err := events.New(events.PointsChanged, map[string]any{
		"memberId": e.MemberID, "entryId": e.ID, "deltaPoints": e.DeltaPoints,
		"reason": e.Reason, "status": e.Status, "balance": avail,
	})
	if err != nil {
		return false, err
	}
	return true, outbox.Write(ctx, tx, ev)
}

// History lists entries newest first.
func History(ctx context.Context, d *sql.DB, memberID string, limit int) ([]Entry, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, member_id, delta_points, reason, status, ref_kind, ref_id, created_at
		 FROM point_ledger WHERE member_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, memberID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.MemberID, &e.DeltaPoints, &e.Reason, &e.Status, &e.RefKind, &e.RefID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
