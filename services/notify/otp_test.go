//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nongnooch/platform/pkg/chassis/consumer"
	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
	"nongnooch/platform/pkg/chassis/testutil"
)

type sent struct{ to, text string }

type fakeSMS struct {
	mu       sync.Mutex
	got      []sent
	failOnce map[string]bool
}

func (f *fakeSMS) Send(_ context.Context, to, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOnce[to] {
		delete(f.failOnce, to)
		return errors.New("boom")
	}
	f.got = append(f.got, sent{to, text})
	return nil
}

func (f *fakeSMS) to(to string) []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sent
	for _, s := range f.got {
		if s.to == to {
			out = append(out, s)
		}
	}
	return out
}

type fakeMail struct {
	mu  sync.Mutex
	got []sent
}

func (f *fakeMail) Send(_ context.Context, to, _, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, sent{to, body})
	return nil
}

func (f *fakeMail) count(to string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, s := range f.got {
		if s.to == to {
			n++
		}
	}
	return n
}

// start runs relay + consumer on an isolated DB. The EVENTS stream is shared and a new
// durable replays history, so tests use unique contacts and filter on them.
func start(t *testing.T, sms SMSSender, mail EmailSender) func(contact, kind, lang, code, id string) {
	t.Helper()
	d, _ := testutil.DB(t)
	if err := db.Migrate(testutil.Ctx(t), d, nil); err != nil {
		t.Fatal(err)
	}
	_, js := testutil.NATS(t)
	ctx := testutil.Ctx(t)
	if err := outbox.EnsureStream(ctx, js); err != nil {
		t.Fatal(err)
	}
	tpl, err := LoadTemplates(templatesFS)
	if err != nil {
		t.Fatal(err)
	}
	go (&outbox.Relay{DB: d, JS: js, Interval: 50 * time.Millisecond}).Run(ctx)
	name := fmt.Sprintf("notify-otp-test-%d", time.Now().UnixNano())
	go consumer.Run(ctx, d, js, consumer.Config{Name: name, EventTypes: []string{events.OTPRequested}}, OTPHandler(sms, mail, tpl))

	return func(contact, kind, lang, code, id string) {
		e, _ := events.New(events.OTPRequested, map[string]string{
			"requestId": id, "kind": kind, "contact": contact, "code": code, "lang": lang,
		})
		tx, _ := d.Begin()
		if err := outbox.Write(ctx, tx, e); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if ok() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// uniq returns a contact unique to this run (the shared stream replays older runs' events).
func uniq(prefix string) string { return fmt.Sprintf("%s%08d", prefix, time.Now().UnixNano()%100000000) }

func TestPhoneOTPInLang(t *testing.T) {
	sms := &fakeSMS{}
	publish := start(t, sms, nil)
	to := uniq("+86138")
	publish(to, "phone", "zh", "482913", "req-1")
	waitFor(t, "sms", func() bool { return len(sms.to(to)) >= 1 })
	got := sms.to(to)
	if len(got) != 1 || !strings.Contains(got[0].text, "农努乐") || !strings.Contains(got[0].text, "482913") {
		t.Fatalf("%+v", got)
	}
}

func TestSameEventRedeliveredSendsOnce(t *testing.T) {
	sms := &fakeSMS{}
	d, _ := testutil.DB(t)
	if err := db.Migrate(testutil.Ctx(t), d, nil); err != nil {
		t.Fatal(err)
	}
	_, js := testutil.NATS(t)
	ctx := testutil.Ctx(t)
	_ = outbox.EnsureStream(ctx, js)
	tpl, _ := LoadTemplates(templatesFS)
	to := uniq("+668")
	e, _ := events.New(events.OTPRequested, map[string]string{"requestId": "r", "kind": "phone", "contact": to, "code": "111222", "lang": "th"})
	h := OTPHandler(sms, nil, tpl)
	// the chassis consumer dedups by event id through processed_events: simulate redelivery of the same message
	name := fmt.Sprintf("notify-dedup-%d", time.Now().UnixNano())
	b, _ := json.Marshal(e)
	if _, err := js.Publish(ctx, events.Subject(events.OTPRequested), b); err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish(ctx, events.Subject(events.OTPRequested), b); err != nil { // no msg-id: delivered twice
		t.Fatal(err)
	}
	go consumer.Run(ctx, d, js, consumer.Config{Name: name, EventTypes: []string{events.OTPRequested}}, h)
	waitFor(t, "sms", func() bool { return len(sms.to(to)) >= 1 })
	time.Sleep(700 * time.Millisecond)
	if n := len(sms.to(to)); n != 1 {
		t.Fatalf("sent %d times", n)
	}
	if !strings.Contains(sms.to(to)[0].text, "สวนนงนุช") {
		t.Fatal("not thai template")
	}
}

func TestEmailOTP(t *testing.T) {
	mail := &fakeMail{}
	publish := start(t, &fakeSMS{}, mail)
	publish("diner@example.com", "email", "en", "654321", "req-e")
	waitFor(t, "email", func() bool { return mail.count("diner@example.com") >= 1 })
}

func TestAllSevenTemplatesRender(t *testing.T) {
	tpl, err := LoadTemplates(templatesFS)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range otpLangs {
		s, err := tpl.Render(l, "123456")
		if err != nil || !strings.Contains(s, "123456") || !strings.Contains(s, "5") {
			t.Errorf("%s: %q %v", l, s, err)
		}
	}
	if s, _ := tpl.Render("ru", "123456"); !strings.Contains(s, "verification code") {
		t.Errorf("fallback: %q", s)
	}
}

func TestSenderErrorRetries(t *testing.T) {
	to := uniq("+668")
	sms := &fakeSMS{failOnce: map[string]bool{to: true}}
	publish := start(t, sms, nil)
	publish(to, "phone", "en", "999111", "req-r")
	waitFor(t, "retry delivery", func() bool { return len(sms.to(to)) == 1 })
}

func TestNoCodeInLogs(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&lockedWriter{&buf, &mu}, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202) }))
	defer srv.Close()
	to := uniq("+668")
	const code = "731946"
	sms := HTTPSMSSender{BaseURL: srv.URL}
	publish := start(t, sms, nil)
	publish(to, "phone", "en", code, "req-l")
	waitFor(t, "log line", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(buf.String(), "otp sms sent")
	})
	mu.Lock()
	defer mu.Unlock()
	if strings.Contains(buf.String(), code) || strings.Contains(buf.String(), to) {
		t.Fatalf("log leaks code or full number:\n%s", buf.String())
	}
}

type lockedWriter struct {
	b  *bytes.Buffer
	mu *sync.Mutex
}

func (w *lockedWriter) Write(p []byte) (int, error) { w.mu.Lock(); defer w.mu.Unlock(); return w.b.Write(p) }
