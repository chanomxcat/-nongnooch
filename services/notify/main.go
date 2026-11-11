// Command notify delivers OTP messages (SMS via SMS_BASE_URL) from otp.requested events.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
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
	cfg, err := config.Load("notify")
	if err == nil && os.Getenv("SMS_BASE_URL") == "" {
		err = errors.New("SMS_BASE_URL is required")
	}
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := run(ctx, cfg); err != nil {
		slog.Error("notify stopped", "err", err)
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
	tpl, err := LoadTemplates(templatesFS)
	if err != nil {
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
	var consumerReady atomic.Bool
	go func() {
		err := consumer.Run(ctx, d, js, consumer.Config{Name: "notify-otp", EventTypes: []string{events.OTPRequested}, Ready: &consumerReady},
			OTPHandler(HTTPSMSSender{BaseURL: os.Getenv("SMS_BASE_URL")}, nil, tpl))
		if err != nil {
			slog.Error("otp consumer stopped", "err", err)
		}
	}()

	srv := server.New(cfg, d)
	srv.ReadyChecks = append(srv.ReadyChecks, func() error {
		if !consumerReady.Load() {
			return errors.New("notify-otp consumer not running")
		}
		return nil
	})
	demo := os.Getenv("DEMO_MODE") == "true"
	srv.Mux.HandleFunc("POST /internal/seed", func(w http.ResponseWriter, r *http.Request) {
		if !demo {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent) // nothing to seed yet
	})
	return srv.Run(ctx)
}
