// Package outbox writes events transactionally and relays them to NATS JetStream.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"nongnooch/platform/pkg/chassis/events"
)

// stored is the outbox payload: the Envelope plus the trace context of the writer.
type stored struct {
	events.Envelope
	Traceparent string `json:"traceparent,omitempty"`
}

// Write inserts an event into the outbox inside the callers transaction.
func Write(ctx context.Context, tx *sql.Tx, e events.Envelope) error {
	s := stored{Envelope: e}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	s.Traceparent = carrier["traceparent"]
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox(id, type, payload, created_at) VALUES(?,?,?,?)`,
		e.ID, e.Type, string(b), e.OccurredAt.UTC())
	return err
}

// EnsureStream creates or updates the EVENTS stream.
func EnsureStream(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       events.Stream,
		Subjects:   []string{"evt.>", events.DLQPrefix + ">"},
		Storage:    jetstream.FileStorage,
		Duplicates: 2 * time.Minute,
	})
	return err
}

// Relay publishes unpublished outbox rows to JetStream.
type Relay struct {
	DB       *sql.DB
	JS       jetstream.JetStream
	Batch    int
	Interval time.Duration
}

// Run ticks until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) error {
	iv := r.Interval
	if iv <= 0 {
		iv = 200 * time.Millisecond
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	for {
		for {
			n, err := r.Tick(ctx)
			if err != nil {
				if ctx.Err() == nil {
					slog.Error("outbox relay tick failed", "err", err)
				}
				break
			}
			if n == 0 || ctx.Err() != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// Tick publishes one batch and returns how many rows were published. Rows whose
// publish fails stay unpublished; the ones before the failure are still committed.
func (r *Relay) Tick(ctx context.Context) (int, error) {
	batch := r.Batch
	if batch <= 0 {
		batch = 100
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT id, type, payload FROM outbox WHERE published_at IS NULL
		 ORDER BY created_at, id LIMIT ? FOR UPDATE SKIP LOCKED`, batch)
	if err != nil {
		return 0, err
	}
	type row struct{ id, typ, payload string }
	var pending []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.typ, &x.payload); err != nil {
			rows.Close()
			return 0, err
		}
		pending = append(pending, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	var done []string
	var pubErr error
	for _, x := range pending {
		msg := &nats.Msg{Subject: events.Subject(x.typ), Data: []byte(x.payload), Header: nats.Header{}}
		var s stored
		if json.Unmarshal([]byte(x.payload), &s) == nil && s.Traceparent != "" {
			propagation.HeaderCarrier(msg.Header).Set("traceparent", s.Traceparent)
		}
		if _, err := r.JS.PublishMsg(ctx, msg, jetstream.WithMsgID(x.id)); err != nil {
			pubErr = err
			break
		}
		done = append(done, x.id)
	}
	now := time.Now().UTC()
	for _, id := range done {
		if _, err := tx.ExecContext(ctx, `UPDATE outbox SET published_at = ? WHERE id = ?`, now, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(done), pubErr
}
