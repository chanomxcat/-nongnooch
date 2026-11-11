//go:build integration

package consumer_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"nongnooch/platform/pkg/chassis/consumer"
	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
	"nongnooch/platform/pkg/chassis/testutil"
)

type env struct {
	d   *sql.DB
	js  jetstream.JetStream
	typ string
	cfg consumer.Config
}

func setup(t *testing.T, maxDeliver int) env {
	d, _ := testutil.DB(t)
	if err := db.Migrate(context.Background(), d, fstest.MapFS{}); err != nil {
		t.Fatal(err)
	}
	_, js := testutil.NATS(t)
	if err := outbox.EnsureStream(context.Background(), js); err != nil {
		t.Fatal(err)
	}
	n := time.Now().UnixNano()
	typ := fmt.Sprintf("c%d.evt", n)
	name := fmt.Sprintf("cons%d", n)
	t.Cleanup(func() { js.DeleteConsumer(context.Background(), events.Stream, name) })
	return env{d, js, typ, consumer.Config{Name: name, EventTypes: []string{typ}, MaxDeliver: maxDeliver, AckWait: 2 * time.Second}}
}

func (e env) publish(t *testing.T, ev events.Envelope) {
	b := []byte(fmt.Sprintf(`{"id":%q,"type":%q,"occurredAt":%q,"payload":%s}`, ev.ID, ev.Type, ev.OccurredAt.Format(time.RFC3339Nano), ev.Payload))
	// unique Nats-Msg-Id per publish so JetStream dedup does not hide duplicates
	_, err := e.js.PublishMsg(context.Background(), &nats.Msg{Subject: events.Subject(e.typ), Data: b},
		jetstream.WithMsgID(fmt.Sprintf("%s-%d", ev.ID, time.Now().UnixNano())))
	if err != nil {
		t.Fatal(err)
	}
}

func (e env) run(t *testing.T, h consumer.Handler) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go consumer.Run(ctx, e.d, e.js, e.cfg, h)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func processedCount(d *sql.DB) int {
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM processed_events`).Scan(&n)
	return n
}

func TestDuplicateDeliveryHandledOnce(t *testing.T) {
	e := setup(t, 5)
	var calls atomic.Int32
	e.run(t, func(ctx context.Context, tx *sql.Tx, ev events.Envelope) error { calls.Add(1); return nil })
	ev, _ := events.New(e.typ, map[string]string{"k": "v"})
	e.publish(t, ev)
	e.publish(t, ev)
	waitFor(t, "processed", func() bool { return processedCount(e.d) == 1 })
	time.Sleep(500 * time.Millisecond) // let the duplicate be consumed
	if calls.Load() != 1 {
		t.Fatalf("handler called %d times", calls.Load())
	}
}

func TestHandlerErrorRollsBackMarker(t *testing.T) {
	e := setup(t, 5)
	var calls atomic.Int32
	e.run(t, func(ctx context.Context, tx *sql.Tx, ev events.Envelope) error {
		if calls.Add(1) == 1 {
			return errors.New("boom")
		}
		return nil
	})
	ev, _ := events.New(e.typ, nil)
	e.publish(t, ev)
	waitFor(t, "second attempt", func() bool { return processedCount(e.d) == 1 })
	if calls.Load() != 2 {
		t.Fatalf("calls=%d want 2", calls.Load())
	}
}

func TestPoisonGoesToDLQ(t *testing.T) {
	e := setup(t, 3)
	var calls atomic.Int32
	e.run(t, func(ctx context.Context, tx *sql.Tx, ev events.Envelope) error { calls.Add(1); return errors.New("always") })
	ev, _ := events.New(e.typ, nil)
	e.publish(t, ev)
	ctx := context.Background()
	dlq, err := e.js.OrderedConsumer(ctx, events.Stream, jetstream.OrderedConsumerConfig{FilterSubjects: []string{events.DLQPrefix + e.cfg.Name}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dlq.Next(jetstream.FetchMaxWait(10 * time.Second)); err != nil {
		t.Fatalf("no dlq message: %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls=%d want 3", calls.Load())
	}
	if processedCount(e.d) != 0 {
		t.Fatal("processed marker must not remain")
	}
	time.Sleep(500 * time.Millisecond)
	if calls.Load() != 3 {
		t.Fatalf("redelivered after DLQ: calls=%d", calls.Load())
	}
}

func TestHandlerWritesInSameTx(t *testing.T) {
	e := setup(t, 2)
	if _, err := e.d.Exec(`CREATE TABLE side (v VARCHAR(10))`); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	e.run(t, func(ctx context.Context, tx *sql.Tx, ev events.Envelope) error {
		calls.Add(1)
		if _, err := tx.ExecContext(ctx, `INSERT INTO side VALUES('x')`); err != nil {
			return err
		}
		return errors.New("fail after write")
	})
	ev, _ := events.New(e.typ, nil)
	e.publish(t, ev)
	waitFor(t, "2 attempts", func() bool { return calls.Load() >= 2 })
	time.Sleep(300 * time.Millisecond)
	var n int
	e.d.QueryRow(`SELECT COUNT(*) FROM side`).Scan(&n)
	if n != 0 {
		t.Fatalf("side table has %d rows, want 0", n)
	}
}

func TestThaiPayloadRoundTrip(t *testing.T) {
	e := setup(t, 5)
	const s = "สวนนงนุช 农努奇 🌺"
	got := make(chan string, 1)
	e.run(t, func(ctx context.Context, tx *sql.Tx, ev events.Envelope) error {
		var p map[string]string
		if err := jsonUnmarshal(ev.Payload, &p); err != nil {
			return err
		}
		got <- p["name"]
		return nil
	})
	// full path: outbox -> relay -> NATS -> consumer
	ev, _ := events.New(e.typ, map[string]string{"name": s})
	tx, _ := e.d.Begin()
	if err := outbox.Write(context.Background(), tx, ev); err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	r := &outbox.Relay{DB: e.d, JS: e.js}
	if n, err := r.Tick(context.Background()); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	select {
	case v := <-got:
		if v != s {
			t.Fatalf("got %q want %q", v, s)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

// flakyJS fails CreateOrUpdateConsumer the first n times.
type flakyJS struct {
	jetstream.JetStream
	fails atomic.Int32
	calls atomic.Int32
}

func (f *flakyJS) CreateOrUpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	f.calls.Add(1)
	if f.fails.Add(-1) >= 0 {
		return nil, errors.New("nats unavailable")
	}
	return f.JetStream.CreateOrUpdateConsumer(ctx, stream, cfg)
}

func TestStartupRetriesUntilReady(t *testing.T) {
	e := setup(t, 3)
	fj := &flakyJS{JetStream: e.js}
	fj.fails.Store(2)
	var ready atomic.Bool
	e.cfg.Ready = &ready
	e.cfg.RetryMin, e.cfg.RetryMax = 20*time.Millisecond, 50*time.Millisecond
	var got atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go consumer.Run(ctx, e.d, fj, e.cfg, func(context.Context, *sql.Tx, events.Envelope) error { got.Add(1); return nil })
	waitFor(t, "consumer ready after retries", ready.Load)
	if c := fj.calls.Load(); c != 3 {
		t.Fatalf("calls=%d want 3", c)
	}
	e.publish(t, events.Envelope{ID: "retry-1", Type: e.typ, OccurredAt: time.Now(), Payload: []byte(`{}`)})
	waitFor(t, "event handled", func() bool { return got.Load() == 1 })
	cancel()
	waitFor(t, "not ready after stop", func() bool { return !ready.Load() })
}
