//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"nongnooch/platform/pkg/chassis/config"
	"nongnooch/platform/pkg/chassis/testutil"
)

func freeAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestPingFlowsToPongsWithSameTrace(t *testing.T) {
	d, dsn := testutil.DB(t)
	cfg := config.Base{ServiceName: "example", HTTPAddr: freeAddr(t), GRPCAddr: freeAddr(t), DBDSN: dsn, NATSURL: os.Getenv("TEST_NATS_URL")}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- start(ctx, cfg) }()
	defer func() { cancel(); <-done }()

	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	var resp *http.Response
	for i := 0; i < 100; i++ {
		req, _ := http.NewRequest("POST", "http://"+cfg.HTTPAddr+"/ping", strings.NewReader(""))
		req.Header.Set("traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")
		var err error
		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("ping failed: %v", resp)
	}
	var out struct{ EventID, TraceID string }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if out.TraceID != traceID {
		t.Fatalf("http trace id %s", out.TraceID)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var got string
		if err := d.QueryRow(`SELECT trace_id FROM pongs WHERE event_id = ?`, out.EventID).Scan(&got); err == nil {
			if got != traceID {
				t.Fatalf("consumer trace id %s, want %s", got, traceID)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("no pongs row within 5s")
}
