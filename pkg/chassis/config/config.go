// Package config loads the shared service configuration from environment variables.
package config

import (
	"errors"
	"os"
)

// Base is the configuration every service shares.
type Base struct {
	ServiceName  string
	HTTPAddr     string
	GRPCAddr     string
	DBDSN        string
	NATSURL      string
	OTLPEndpoint string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads the base config. DB_DSN is required.
func Load(service string) (Base, error) {
	b := Base{
		ServiceName:  service,
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		GRPCAddr:     env("GRPC_ADDR", ":9090"),
		DBDSN:        os.Getenv("DB_DSN"),
		NATSURL:      env("NATS_URL", "nats://localhost:4222"),
		OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
	if b.DBDSN == "" {
		return b, errors.New("config: DB_DSN is required")
	}
	return b, nil
}
