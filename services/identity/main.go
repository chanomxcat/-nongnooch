// Command identity serves OTP login and JWT issuing.
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
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/obs"
	"nongnooch/platform/pkg/chassis/outbox"
	"nongnooch/platform/pkg/chassis/server"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	cfg, err := config.Load("identity")
	if err == nil && (os.Getenv("OTP_PEPPER") == "" || os.Getenv("JWT_ED25519_KEY") == "") {
		err = errors.New("OTP_PEPPER and JWT_ED25519_KEY are required")
	}
	if err == nil && os.Getenv("TURNSTILE_VERIFY_URL") == "" {
		err = errors.New("TURNSTILE_VERIFY_URL is required")
	}
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := run(ctx, cfg); err != nil {
		slog.Error("identity stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Base) error {
	key, err := ParseKey(os.Getenv("JWT_ED25519_KEY"))
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

	var members MemberLookup = noMember{}
	if addr := os.Getenv("MEMBER_GRPC_ADDR"); addr != "" {
		l, cc, err := NewGRPCMemberLookup(addr)
		if err != nil {
			return err
		}
		defer cc.Close()
		members = l
	}
	api := &API{
		OTP: &OTPService{DB: d, Pepper: []byte(os.Getenv("OTP_PEPPER")), Verifier: TurnstileVerifier{
			URL: os.Getenv("TURNSTILE_VERIFY_URL"), Secret: os.Getenv("TURNSTILE_SECRET"),
			Client: &http.Client{Timeout: 3 * time.Second},
		}},
		Tokens:   &TokenIssuer{DB: d, Key: key, Members: members},
		Members:  members,
		DemoMode: os.Getenv("DEMO_MODE") == "true",
	}
	srv := server.New(cfg, d)
	api.Register(srv.Mux)
	return srv.Run(ctx)
}
