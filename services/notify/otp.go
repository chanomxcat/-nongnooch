package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"text/template"
	"time"

	"nongnooch/platform/pkg/chassis/consumer"
	"nongnooch/platform/pkg/chassis/events"
)

//go:embed templates/*.txt
var templatesFS embed.FS

var otpLangs = []string{"th", "en", "zh", "ja", "ko", "de", "fr"}

const otpMinutes = 5

// Templates holds one parsed OTP template per language.
type Templates map[string]*template.Template

func LoadTemplates(fsys fs.FS) (Templates, error) {
	t := Templates{}
	for _, l := range otpLangs {
		b, err := fs.ReadFile(fsys, "templates/otp."+l+".txt")
		if err != nil {
			return nil, err
		}
		tpl, err := template.New(l).Parse(string(b))
		if err != nil {
			return nil, err
		}
		t[l] = tpl
	}
	return t, nil
}

// Render falls back to English for unknown languages.
func (t Templates) Render(lang, code string) (string, error) {
	tpl, ok := t[lang]
	if !ok {
		tpl = t["en"]
	}
	var b bytes.Buffer
	if err := tpl.Execute(&b, map[string]any{"Code": code, "Minutes": otpMinutes}); err != nil {
		return "", err
	}
	return b.String(), nil
}

type SMSSender interface {
	Send(ctx context.Context, e164, text string) error
}

type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

func mask(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

// HTTPSMSSender posts {to,text} to {BaseURL}/send (mockext contract).
type HTTPSMSSender struct {
	BaseURL string
	Client  *http.Client
}

func (s HTTPSMSSender) Send(ctx context.Context, e164, text string) error {
	c := s.Client
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Second}
	}
	b, _ := json.Marshal(map[string]string{"to": e164, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.BaseURL, "/")+"/send", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("sms send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sms send: status %d", resp.StatusCode)
	}
	slog.InfoContext(ctx, "otp sms sent", "to", mask(e164)) // never log the text
	return nil
}

// OTPHandler delivers otp.requested events. It must never log the payload or code.
// mail may be nil (email delivery is not wired in the demo): such events are dropped.
func OTPHandler(sms SMSSender, mail EmailSender, tpl Templates) consumer.Handler {
	return func(ctx context.Context, _ *sql.Tx, e events.Envelope) error {
		var p struct {
			RequestID, Kind, Contact, Code, Lang string
		}
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return fmt.Errorf("bad otp.requested payload: %w", err)
		}
		text, err := tpl.Render(p.Lang, p.Code)
		if err != nil {
			return err
		}
		switch p.Kind {
		case "phone":
			return sms.Send(ctx, p.Contact, text)
		case "email":
			if mail == nil {
				slog.WarnContext(ctx, "email otp not delivered: no email sender configured", "to", mask(p.Contact))
				return nil
			}
			return mail.Send(ctx, p.Contact, "Nong Nooch verification code", text)
		}
		return fmt.Errorf("unknown otp kind %q", p.Kind)
	}
}
