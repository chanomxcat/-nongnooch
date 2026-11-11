// Command commerce serves the catalog, orders and KBank payment.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	memberv1 "nongnooch/platform/gen/go/member/v1"
	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/obs"
	"nongnooch/platform/pkg/chassis/outbox"
	"nongnooch/platform/pkg/chassis/server"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// grpcMember adapts the generated MemberService client.
type grpcMember struct{ c memberv1.MemberServiceClient }

func (g grpcMember) GetCompanion(ctx context.Context, memberID, companionID string) (Holder, error) {
	h, err := g.c.GetCompanion(ctx, &memberv1.GetCompanionRequest{MemberId: memberID, CompanionId: companionID})
	if err != nil {
		return Holder{}, err
	}
	return Holder{Name: h.Name, Nationality: h.Nationality, IDLast4: h.IdLast4, ThaiVerified: h.ThaiVerified}, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	cfg, err := config.Load("commerce")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := run(ctx, cfg); err != nil {
		slog.Error("commerce stopped", "err", err)
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

	conn, err := grpc.NewClient(getenv("MEMBER_GRPC", "member:9090"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	svc := &Service{
		DB:            d,
		Member:        grpcMember{memberv1.NewMemberServiceClient(conn)},
		KBank:         &KBank{BaseURL: getenv("KBANK_BASE_URL", "http://mockext:8090/pg"), APIKey: os.Getenv("KBANK_API_KEY")},
		WebhookSecret: os.Getenv("KBANK_WEBHOOK_SECRET"),
		WebhookURL:    getenv("KBANK_WEBHOOK_URL", "http://commerce:8080/webhooks/kbank"),
	}
	if svc.WebhookSecret == "" {
		return errConfig("KBANK_WEBHOOK_SECRET is required")
	}
	go func() { // release job: unpaid orders past their 15-minute hold
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := svc.ReleaseExpired(ctx); err != nil {
					slog.Error("release expired", "err", err)
				} else if n > 0 {
					slog.Info("released expired orders", "count", n)
				}
			}
		}
	}()
	srv := server.New(cfg, d)
	(&API{Svc: svc, DemoMode: os.Getenv("DEMO_MODE") == "true"}).Register(srv.Mux)
	return srv.Run(ctx)
}

type errConfig string

func (e errConfig) Error() string { return "config: " + string(e) }
