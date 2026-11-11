package server_test

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/server"
)

func freeAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestHealthEndpoints(t *testing.T) {
	// unreachable DB: /readyz must be 503
	d, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/x")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Base{ServiceName: "t", HTTPAddr: freeAddr(t), GRPCAddr: freeAddr(t)}
	s := server.New(cfg, d)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	get := func(path string) int {
		var last error
		for i := 0; i < 50; i++ {
			resp, err := http.Get("http://" + cfg.HTTPAddr + path)
			if err == nil {
				resp.Body.Close()
				return resp.StatusCode
			}
			last = err
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal(last)
		return 0
	}
	if c := get("/healthz"); c != 200 {
		t.Fatalf("healthz %d", c)
	}
	if c := get("/readyz"); c != 503 {
		t.Fatalf("readyz %d", c)
	}

	conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cctx, c2 := context.WithTimeout(ctx, 3*time.Second)
	defer c2()
	r, err := healthpb.NewHealthClient(conn).Check(cctx, &healthpb.HealthCheckRequest{})
	if err != nil || r.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("grpc health: %v %v", r, err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timeout")
	}
}

func TestReadyzWithoutDB(t *testing.T) {
	s := server.New(config.Base{ServiceName: "t"}, nil)
	rec := httptest.NewRecorder()
	s.Mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}
