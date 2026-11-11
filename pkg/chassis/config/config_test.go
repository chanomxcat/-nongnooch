package config_test

import (
	"strings"
	"testing"

	"nongnooch/platform/pkg/chassis/config"
)

func TestLoadRequiresDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")
	_, err := config.Load("identity")
	if err == nil || !strings.Contains(err.Error(), "DB_DSN") {
		t.Fatalf("want DB_DSN error, got %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DSN", "u:p@tcp(localhost:3306)/identity")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("GRPC_ADDR", "")
	c, err := config.Load("identity")
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.GRPCAddr != ":9090" || c.ServiceName != "identity" {
		t.Fatal(c)
	}
}
