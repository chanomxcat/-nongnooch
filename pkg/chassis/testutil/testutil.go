// Package testutil provides helpers for integration tests: an isolated MariaDB
// database per test and a NATS JetStream connection. Tests fail (never skip)
// when the environment is missing.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"nongnooch/platform/pkg/chassis/db"
)

// DB creates a fresh database on TEST_DB_DSN's server, returns a handle to it
// and its DSN, and drops it on cleanup.
func DB(t testing.TB) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Fatal("TEST_DB_DSN not set")
	}
	admin, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("t_%d_%d", time.Now().UnixNano(), rand.Intn(1000000))
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := mysql.ParseDSN(dsn)
	cfg.DBName = name
	newDSN := cfg.FormatDSN()
	d, err := db.Open(newDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		d.Close()
		admin.Exec("DROP DATABASE `" + name + "`")
		admin.Close()
	})
	return d, newDSN
}

// NATS connects to TEST_NATS_URL and returns the connection and JetStream.
func NATS(t testing.TB) (*nats.Conn, jetstream.JetStream) {
	t.Helper()
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		t.Fatal("TEST_NATS_URL not set")
	}
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	return nc, js
}

// Ctx returns a context cancelled at test end.
func Ctx(t testing.TB) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
