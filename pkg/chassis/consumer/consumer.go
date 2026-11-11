// Package consumer runs idempotent JetStream consumers with dead-lettering.
package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"

	"nongnooch/platform/pkg/chassis/events"
)

// Handler does the work for one event inside tx. Return an error to roll back and retry.
type Handler func(ctx context.Context, tx *sql.Tx, e events.Envelope) error

// Config of a durable consumer.
type Config struct {
	Name       string
	EventTypes []string
	MaxDeliver int           // default 10
	AckWait    time.Duration // default 30s

	// Ready, if set, is true while the consumer is subscribed (for /readyz).
	Ready *atomic.Bool
	// RetryMin/RetryMax bound the startup retry backoff (default 1s / 30s).
	RetryMin, RetryMax time.Duration
}

// Run consumes until ctx is cancelled. Each message is processed in one DB
// transaction together with its processed_events marker. Startup failures
// (consumer creation / subscribe) are logged and retried with exponential
// backoff forever; cfg.Ready (if set) is true only while subscribed.
func Run(ctx context.Context, db *sql.DB, js jetstream.JetStream, cfg Config, h Handler) error {
	if cfg.MaxDeliver <= 0 {
		cfg.MaxDeliver = 10
	}
	if cfg.AckWait <= 0 {
		cfg.AckWait = 30 * time.Second
	}
	if cfg.RetryMin <= 0 {
		cfg.RetryMin = time.Second
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = 30 * time.Second
	}
	subjects := make([]string, len(cfg.EventTypes))
	for i, t := range cfg.EventTypes {
		subjects[i] = events.Subject(t)
	}
	cc := jetstream.ConsumerConfig{
		Durable:    cfg.Name,
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: cfg.MaxDeliver,
		AckWait:    cfg.AckWait,
	}
	if len(subjects) == 1 {
		cc.FilterSubject = subjects[0]
	} else {
		cc.FilterSubjects = subjects
	}
	if cfg.Ready != nil {
		cfg.Ready.Store(false)
		defer cfg.Ready.Store(false)
	}
	start := func() (jetstream.ConsumeContext, error) {
		cons, err := js.CreateOrUpdateConsumer(ctx, events.Stream, cc)
		if err != nil {
			return nil, err
		}
		return cons.Consume(func(msg jetstream.Msg) {
			process(ctx, db, js, cfg, h, msg)
		})
	}
	backoff := cfg.RetryMin
	for {
		cctx, err := start()
		if err == nil {
			if cfg.Ready != nil {
				cfg.Ready.Store(true)
			}
			<-ctx.Done()
			cctx.Stop()
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		slog.ErrorContext(ctx, "consumer start failed, will retry", "consumer", cfg.Name, "err", err, "retry_in", backoff.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > cfg.RetryMax {
			backoff = cfg.RetryMax
		}
	}
}

func process(ctx context.Context, db *sql.DB, js jetstream.JetStream, cfg Config, h Handler, msg jetstream.Msg) {
	delivered := uint64(1)
	if meta, err := msg.Metadata(); err == nil {
		delivered = meta.NumDelivered
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(msg.Headers()))

	var env events.Envelope
	err := json.Unmarshal(msg.Data(), &env)
	if err == nil {
		err = handle(ctx, db, cfg, h, env)
		if err == nil {
			_ = msg.Ack()
			return
		}
	}

	if delivered >= uint64(cfg.MaxDeliver) {
		dlq := &nats.Msg{Subject: events.DLQPrefix + cfg.Name, Data: msg.Data(), Header: nats.Header{}}
		dlq.Header.Set("error", err.Error())
		if _, perr := js.PublishMsg(ctx, dlq); perr != nil {
			slog.ErrorContext(ctx, "dlq publish failed", "consumer", cfg.Name, "err", perr)
			_ = msg.Nak()
			return
		}
		slog.ErrorContext(ctx, "event moved to dead-letter", "alert", "oncall",
			"consumer", cfg.Name, "event_id", env.ID, "type", env.Type, "deliveries", delivered, "err", err)
		_ = msg.Term()
		return
	}
	slog.WarnContext(ctx, "event handler failed, will retry", "consumer", cfg.Name, "event_id", env.ID, "err", err)
	_ = msg.NakWithDelay(time.Duration(delivered) * 100 * time.Millisecond)
}

// handle runs one event in a transaction. Already-processed events are a no-op success.
func handle(ctx context.Context, db *sql.DB, cfg Config, h Handler, env events.Envelope) error {
	ctx, span := otel.Tracer("chassis/consumer").Start(ctx, "consume "+env.Type)
	defer span.End()

	err := func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		res, err := tx.ExecContext(ctx,
			`INSERT IGNORE INTO processed_events(consumer, event_id, processed_at) VALUES(?,?,?)`,
			cfg.Name, env.ID, time.Now().UTC())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		if err := h(ctx, tx, env); err != nil {
			return err
		}
		return tx.Commit()
	}()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
