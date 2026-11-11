//go:build integration

package outbox_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
	"nongnooch/platform/pkg/chassis/testutil"
)

func setup(t *testing.T) (*sql.DB, jetstream.JetStream, string) {
	d, _ := testutil.DB(t)
	if err := db.Migrate(context.Background(), d, fstest.MapFS{}); err != nil {
		t.Fatal(err)
	}
	_, js := testutil.NATS(t)
	if err := outbox.EnsureStream(context.Background(), js); err != nil {
		t.Fatal(err)
	}
	typ := fmt.Sprintf("t%d.evt", time.Now().UnixNano())
	return d, js, typ
}

func write(t *testing.T, d *sql.DB, typ string, n int) []string {
	var ids []string
	for i := 0; i < n; i++ {
		e, _ := events.New(typ, map[string]int{"i": i})
		tx, _ := d.Begin()
		if err := outbox.Write(context.Background(), tx, e); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID)
	}
	return ids
}

func count(t *testing.T, js jetstream.JetStream, typ string) int {
	ctx := context.Background()
	st, err := js.Stream(ctx, events.Stream)
	if err != nil {
		t.Fatal(err)
	}
	info, err := st.Info(ctx, jetstream.WithSubjectFilter(events.Subject(typ)))
	if err != nil {
		t.Fatal(err)
	}
	return int(info.State.Subjects[events.Subject(typ)])
}

func unpublished(d *sql.DB) int {
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM outbox WHERE published_at IS NULL`).Scan(&n)
	return n
}

func TestWriteRollbackLeavesNoEvent(t *testing.T) {
	d, _, typ := setup(t)
	e, _ := events.New(typ, nil)
	tx, _ := d.Begin()
	if err := outbox.Write(context.Background(), tx, e); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&n)
	if n != 0 {
		t.Fatalf("outbox has %d rows", n)
	}
}

func TestTickPublishesAndMarks(t *testing.T) {
	d, js, typ := setup(t)
	ids := write(t, d, typ, 3)
	r := &outbox.Relay{DB: d, JS: js}
	n, err := r.Tick(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if u := unpublished(d); u != 0 {
		t.Fatalf("unpublished=%d", u)
	}
	ctx := context.Background()
	cons, err := js.OrderedConsumer(ctx, events.Stream, jetstream.OrderedConsumerConfig{FilterSubjects: []string{events.Subject(typ)}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		m, err := cons.Next(jetstream.FetchMaxWait(3 * time.Second))
		if err != nil {
			t.Fatal(err)
		}
		var got struct{ ID string }
		if err := jsonUnmarshal(m.Data(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != ids[i] {
			t.Fatalf("msg %d out of order: %s want %s", i, got.ID, ids[i])
		}
	}
	_ = nats.MsgIdHdr
}

func TestTwoRelaysNoDoublePublish(t *testing.T) {
	d, js, typ := setup(t)
	write(t, d, typ, 200)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := &outbox.Relay{DB: d, JS: js, Batch: 50}
			for {
				n, err := r.Tick(context.Background())
				if err != nil {
					t.Error(err)
					return
				}
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()
	if c := count(t, js, typ); c != 200 {
		t.Fatalf("stream has %d messages, want 200", c)
	}
	if u := unpublished(d); u != 0 {
		t.Fatalf("unpublished=%d", u)
	}
}

func TestTickWhenNATSDownKeepsRows(t *testing.T) {
	d, _, typ := setup(t)
	write(t, d, typ, 3)
	nc, js := testutil.NATS(t)
	r := &outbox.Relay{DB: d, JS: js}
	nc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if n, err := r.Tick(ctx); err == nil || n != 0 {
		t.Fatalf("want error with n=0, got n=%d err=%v", n, err)
	}
	if u := unpublished(d); u != 3 {
		t.Fatalf("unpublished=%d want 3", u)
	}
	_, js2 := testutil.NATS(t)
	r.JS = js2
	n, err := r.Tick(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if c := count(t, js2, typ); c != 3 {
		t.Fatalf("stream has %d", c)
	}
}
