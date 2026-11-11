// Command mockext simulates every external system the demo needs (KBank PG, Turnstile, SMS).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/server"
)

// App holds mock state. Everything lives in memory.
type App struct {
	Store     *Store
	PublicURL string // browser-facing base URL (contracts §2)
	APIKey    string
	Secret    string
	Client    *http.Client
	Sleep     func(time.Duration) // injectable for tests
	Sync      bool                // deliver webhooks inline (tests)

	mu       sync.Mutex
	payments map[string]*Payment
	order    []string
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// Routes registers every mock endpoint on mux.
func (a *App) Routes(mux *http.ServeMux) {
	a.consoleRoutes(mux)
	a.turnstileRoutes(mux)
	a.smsRoutes(mux)
	a.kbankRoutes(mux)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" { // distroless has no curl/wget
		resp, err := http.Get("http://127.0.0.1:8090/healthz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}
	cfg := config.Base{
		ServiceName: "mockext",
		HTTPAddr:    env("HTTP_ADDR", ":8090"),
		GRPCAddr:    env("GRPC_ADDR", "127.0.0.1:0"),
	}
	a := &App{
		Store:     NewStore(),
		PublicURL: env("MOCKEXT_PUBLIC_URL", "http://localhost:8090"),
		APIKey:    env("KBANK_API_KEY", "demo-key"),
		Secret:    env("KBANK_WEBHOOK_SECRET", "demo-secret"),
		Client:    &http.Client{Timeout: 10 * time.Second},
		Sleep:     time.Sleep,
		payments:  map[string]*Payment{},
	}
	s := server.New(cfg, nil)
	a.Routes(s.Mux)
	if err := s.Run(context.Background()); err != nil {
		slog.Error("mockext stopped", "err", err)
		os.Exit(1)
	}
}
