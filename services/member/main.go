// Command member serves registration, point ledger and the MemberService gRPC API.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	memberv1 "nongnooch/platform/gen/go/member/v1"
	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/obs"
	"nongnooch/platform/pkg/chassis/outbox"
	"nongnooch/platform/pkg/chassis/server"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	cfg, err := config.Load("member")
	if err == nil && (os.Getenv("MEMBER_ID_ENC_KEY") == "" || os.Getenv("MEMBER_ID_HMAC_KEY") == "") {
		err = errors.New("MEMBER_ID_ENC_KEY and MEMBER_ID_HMAC_KEY are required")
	}
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := run(ctx, cfg); err != nil {
		slog.Error("member stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Base) error {
	cipher, err := NewIDCipher(os.Getenv("MEMBER_ID_ENC_KEY"), byte(envInt("MEMBER_ID_ENC_KID", 1)), os.Getenv("MEMBER_ID_HMAC_KEY"))
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

	svc := &MemberService{DB: d, Cipher: cipher, Invites: DBInviteValidator{DB: d},
		WelcomePoints: envInt("POINTS_WELCOME", 100), InvitePoints: envInt("POINTS_INVITE", 50)}
	srv := server.New(cfg, d)
	(&API{Svc: svc, DemoMode: os.Getenv("DEMO_MODE") == "true"}).Register(srv.Mux)
	memberv1.RegisterMemberServiceServer(srv.GRPC, &GRPCServer{Svc: svc})
	return srv.Run(ctx)
}
