package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrTurnstileFailed      = errors.New("turnstile_failed")
	ErrTurnstileUnavailable = errors.New("turnstile_unavailable")
)

// Verifier checks a Turnstile token. Rejection -> ErrTurnstileFailed, any other
// problem (timeout, network, bad status) -> ErrTurnstileUnavailable (fail closed).
type Verifier interface {
	Verify(ctx context.Context, token, ip string) error
}

type TurnstileVerifier struct {
	URL, Secret string
	Client      *http.Client
}

func (v TurnstileVerifier) Verify(ctx context.Context, token, ip string) error {
	c := v.Client
	if c == nil {
		c = &http.Client{Timeout: 3 * time.Second}
	}
	form := url.Values{"secret": {v.Secret}, "response": {token}, "remoteip": {ip}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrTurnstileUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return ErrTurnstileUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrTurnstileUnavailable
	}
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ErrTurnstileUnavailable
	}
	if !out.Success {
		return ErrTurnstileFailed
	}
	return nil
}
