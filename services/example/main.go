// Command example is a smoke-test service using every chassis part:
// POST /ping -> outbox (example.pinged) -> relay -> consumer -> pongs.
package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/trace"

	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/consumer"
	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/obs"
	"nongnooch/platform/pkg/chassis/outbox"
	"nongnooch/platform/pkg/chassis/server"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const pinged = "example.pinged"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	cfg, err := config.Load("example")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := start(ctx, cfg); err != nil {
		slog.Error("example stopped", "err", err)
		os.Exit(1)
	}
}

// start runs migrations, relay, consumer and the HTTP/gRPC server until ctx ends.
func start(ctx context.Context, cfg config.Base) error {
	shutdownObs, err := obs.Setup(ctx, cfg)
	if err != nil {
		return err
	}
	defer shutdownObs(context.Background())

	d, err := db.Open(cfg.DBDSN)
	if err != nil {
		return err
	}
	defer d.Close()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	if err := db.Migrate(ctx, d, sub); err != nil {
		return err
	}

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	if err := outbox.EnsureStream(ctx, js); err != nil {
		return err
	}

	go (&outbox.Relay{DB: d, JS: js}).Run(ctx)
	go consumer.Run(ctx, d, js, consumer.Config{Name: "example-pong", EventTypes: []string{pinged}},
		func(ctx context.Context, tx *sql.Tx, e events.Envelope) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO pongs(event_id, trace_id, created_at) VALUES(?,?,?)`,
				e.ID, trace.SpanContextFromContext(ctx).TraceID().String(), time.Now().UTC())
			return err
		})

	srv := server.New(cfg, d)
	srv.Mux.HandleFunc("POST /ping", func(w http.ResponseWriter, r *http.Request) {
		e, err := events.New(pinged, map[string]string{"msg": "ping"})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		tx, err := d.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer tx.Rollback()
		if err := outbox.Write(r.Context(), tx, e); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"eventId": e.ID,
			"traceId": trace.SpanContextFromContext(r.Context()).TraceID().String(),
		})
	})
	return srv.Run(ctx)
}
