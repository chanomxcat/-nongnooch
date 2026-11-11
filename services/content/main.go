// Command content serves the CMS: typed multilingual items, drafts, four-eyes approval, audit and public read API.
package main

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	cfg, err := config.Load("content")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := run(ctx, cfg); err != nil {
		slog.Error("content stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Base) error {
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
	go func() {
		// ponytail: no CDN in the demo; the purge consumer only logs what it would purge.
		err := consumer.Run(ctx, d, js, consumer.Config{Name: "content-cdn-purge", EventTypes: []string{events.ContentPublished}},
			func(ctx context.Context, _ *sql.Tx, e events.Envelope) error {
				slog.Info("cdn purge (log mode)", "event", e.ID, "payload", string(e.Payload))
				return nil
			})
		if err != nil {
			slog.Error("cdn purge consumer stopped", "err", err)
		}
	}()

	svc := &Svc{DB: d, FourEyes: os.Getenv("APPROVAL_FOUR_EYES") != "false", SiteURL: envOr("SITE_URL", "http://localhost:3000")}
	srv := server.New(cfg, d)
	(&API{Svc: svc, DemoMode: os.Getenv("DEMO_MODE") == "true"}).Register(srv.Mux)
	return srv.Run(ctx)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
