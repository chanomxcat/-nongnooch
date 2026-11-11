// Command ticket issues signed QR tickets from order.paid and checks them at the gate.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	ticketv1 "nongnooch/platform/gen/go/ticket/v1"
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
	cfg, err := config.Load("ticket")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := run(ctx, cfg); err != nil {
		slog.Error("ticket stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Base) error {
	key := os.Getenv("TICKET_SIGNING_KEY")
	if key == "" {
		return errors.New("config: TICKET_SIGNING_KEY is required")
	}
	signer, err := NewSigner(key, strings.Split(os.Getenv("TICKET_PREVIOUS_PUBLIC_KEYS"), ","))
	if err != nil {
		return err
	}
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

	svc := &Service{DB: d, Signer: signer}
	go func() {
		err := consumer.Run(ctx, d, js, consumer.Config{Name: "ticket-issuer", EventTypes: []string{events.OrderPaid}}, svc.Issue)
		if err != nil && ctx.Err() == nil {
			slog.Error("ticket-issuer stopped", "err", err)
		}
	}()
	srv := server.New(cfg, d)
	(&API{Svc: svc, DemoMode: os.Getenv("DEMO_MODE") == "true"}).Register(srv.Mux)
	ticketv1.RegisterTicketServiceServer(srv.GRPC, &GRPCServer{Svc: svc})
	return srv.Run(ctx)
}
