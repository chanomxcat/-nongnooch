//go:build integration

package db_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/testutil"
)

var ctx = context.Background()

func TestMigrateCreatesChassisTables(t *testing.T) {
	d, _ := testutil.DB(t)
	if err := db.Migrate(ctx, d, fstest.MapFS{}); err != nil {
		t.Fatal(err)
	}
	for _, tbl := range []string{"outbox", "processed_events"} {
		var n int
		err := d.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, tbl).Scan(&n)
		if err != nil || n != 1 {
			t.Fatalf("table %s missing (n=%d err=%v)", tbl, n, err)
		}
	}
}

func TestMigrateRunsServiceMigrations(t *testing.T) {
	d, _ := testutil.DB(t)
	fs := fstest.MapFS{"00001_x.sql": {Data: []byte("-- +goose Up\nCREATE TABLE x (id INT);\n-- +goose Down\nDROP TABLE x;\n")}}
	if err := db.Migrate(ctx, d, fs); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO x VALUES (1)`); err != nil {
		t.Fatal(err)
	}
}

func TestUTF8RoundTrip(t *testing.T) {
	d, _ := testutil.DB(t)
	if err := db.Migrate(ctx, d, fstest.MapFS{}); err != nil {
		t.Fatal(err)
	}
	const s = "สวนนงนุช 农努奇 🌺"
	if _, err := d.Exec(`INSERT INTO outbox(id,type,payload,created_at) VALUES('a','t',?,?)`, s, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := d.QueryRow(`SELECT payload FROM outbox WHERE id = 'a'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != s {
		t.Fatalf("got %q want %q", got, s)
	}
}

func TestMigrateTwiceIsNoop(t *testing.T) {
	d, _ := testutil.DB(t)
	for i := 0; i < 2; i++ {
		if err := db.Migrate(ctx, d, fstest.MapFS{}); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
}
