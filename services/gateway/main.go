// Command gateway is the single public entry: JWT verification + reverse proxy to /api/<svc>/*.
//
// Access model: the gateway authenticates (bad token -> 401 invalid_token, no token -> anonymous)
// but does not authorize. Services enforce with authn.Require / RequireRole, so the public-route
// list of contracts section 2 needs no allowlist here.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/server"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func set(csv string) map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Split(csv, ",") {
		if s = strings.TrimSpace(s); s != "" {
			m[s] = true
		}
	}
	return m
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" { // distroless has no curl/wget
		resp, err := http.Get("http://127.0.0.1:8080/healthz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}
	ctx := context.Background()
	jwks := &JWKS{URL: env("JWKS_URL", "http://identity:8080/.well-known/jwks.json")}
	jwks.Start(ctx)
	g := &Gateway{
		JWKS:     jwks,
		Services: set(env("GATEWAY_SERVICES", "identity,member,commerce,ticket,notify,content,agent,report")),
		Origins:  set(env("CORS_ORIGINS", "http://localhost:3000,http://localhost:3001,http://localhost:3002")),
		Trusted:  &Trusted{Entries: strings.Split(env("TRUSTED_PROXIES", "caddy,web,admin,gate"), ",")},
	}
	s := server.New(config.Base{ServiceName: "gateway", HTTPAddr: env("HTTP_ADDR", ":8080"), GRPCAddr: env("GRPC_ADDR", "127.0.0.1:0")}, nil)
	s.Mux.Handle("/", g)
	if err := s.Run(ctx); err != nil {
		slog.Error("gateway stopped", "err", err)
		os.Exit(1)
	}
}
