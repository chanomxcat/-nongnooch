//go:build integration

package main

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nongnooch/platform/pkg/chassis/authn"
	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/testutil"
)

var (
	editor   = authn.Principal{Type: "staff", Sub: "u-editor", Roles: []string{"editor"}}
	approver = authn.Principal{Type: "staff", Sub: "u-approver", Roles: []string{"approver"}}
	admin    = authn.Principal{Type: "staff", Sub: "u-admin", Roles: []string{"admin"}}
)

func newSvc(t *testing.T) (*Svc, *sql.DB) {
	t.Helper()
	d, _ := testutil.DB(t)
	sub, _ := fs.Sub(migrationsFS, "migrations")
	if err := db.Migrate(testutil.Ctx(t), d, sub); err != nil {
		t.Fatal(err)
	}
	return &Svc{DB: d, FourEyes: true, SiteURL: "http://localhost:3000"}, d
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func heroInput(title string) EditInput {
	return EditInput{Type: "hero_slide", Attrs: map[string]any{"order": float64(1), "imageUrl": "/images/hero.webp"},
		Texts: Texts{"th": {"title": "หัวข้อ " + title, "subtitle": "คำโปรย"}, "en": {"title": "Title " + title}}}
}

// pending creates an item by the editor and submits it.
func pending(t *testing.T, s *Svc, ctx context.Context) (string, string) {
	t.Helper()
	id, err := s.CreateItem(ctx, editor, heroInput("x"))
	if err != nil {
		t.Fatal(err)
	}
	aid, err := s.Submit(ctx, editor, id)
	if err != nil {
		t.Fatal(err)
	}
	return id, aid
}

func TestFourEyes(t *testing.T) {
	s, _ := newSvc(t)
	ctx := testutil.Ctx(t)
	// admin edits -> admin cannot approve even with every role.
	id, err := s.CreateItem(ctx, admin, heroInput("a"))
	if err != nil {
		t.Fatal(err)
	}
	aid, err := s.Submit(ctx, admin, id)
	if err != nil {
		t.Fatal(err)
	}
	wantCode(t, s.Decide(ctx, admin, aid, true, ""), "four_eyes_violation")
	other := authn.Principal{Type: "staff", Sub: "u-editor2", Roles: []string{"editor"}}
	wantCode(t, s.Decide(ctx, other, aid, true, ""), "forbidden")
	if err := s.Decide(ctx, approver, aid, true, ""); err != nil {
		t.Fatal(err)
	}

	// Over HTTP: a plain editor approving their own change gets 403 four_eyes_violation (not forbidden).
	_, aid2 := pending(t, s, ctx)
	mux := http.NewServeMux()
	(&API{Svc: s}).Register(mux)
	req := httptest.NewRequest("POST", "/admin/approvals/"+aid2+"/approve", nil)
	req.Header = editor.Headers()
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "four_eyes_violation") {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
}

func TestConcurrentApprovePublishesOnce(t *testing.T) {
	s, d := newSvc(t)
	ctx := testutil.Ctx(t)
	_, aid := pending(t, s, ctx)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := authn.Principal{Type: "staff", Sub: "approver-" + string(rune('a'+i)), Roles: []string{"approver"}}
			if err := s.Decide(ctx, p, aid, true, ""); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			} else {
				var ae *APIError
				if !errors.As(err, &ae) || ae.Code != "not_pending" {
					if !strings.Contains(err.Error(), "Deadlock") {
						t.Errorf("unexpected: %v", err)
					}
				}
			}
		}(i)
	}
	wg.Wait()
	var outbox, audits int
	d.QueryRow(`SELECT COUNT(*) FROM outbox WHERE type = 'content.published'`).Scan(&outbox)
	d.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'content.published'`).Scan(&audits)
	if ok != 1 || outbox != 1 || audits != 1 {
		t.Fatalf("ok=%d outbox=%d audits=%d", ok, outbox, audits)
	}
}

func TestNotEditableWhilePending(t *testing.T) {
	s, _ := newSvc(t)
	ctx := testutil.Ctx(t)
	id, aid := pending(t, s, ctx)
	wantCode(t, s.SaveDraft(ctx, editor, id, heroInput("y")), "not_editable")
	// After reject the editor can edit again.
	wantCode(t, s.Decide(ctx, approver, aid, false, " "), "reason_required")
	if err := s.Decide(ctx, approver, aid, false, "typo"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveDraft(ctx, editor, id, heroInput("y")); err != nil {
		t.Fatal(err)
	}
}

func TestFallbackPerField(t *testing.T) {
	s, _ := newSvc(t)
	ctx := testutil.Ctx(t)
	in := heroInput("f")
	in.Texts["ja"] = map[string]string{"subtitle": "サブ"}
	in.Texts["en"]["ctaLabel"] = "Buy"
	in.Texts["th"]["ctaLabel"] = "ซื้อ"
	id, _ := s.CreateItem(ctx, editor, in)
	aid, _ := s.Submit(ctx, editor, id)
	if err := s.Decide(ctx, approver, aid, true, ""); err != nil {
		t.Fatal(err)
	}
	items, err := s.Published(ctx, "hero_slide", "ja", "", nil)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	got := items[0].Texts
	if got["title"] != "Title f" || got["subtitle"] != "サブ" || got["ctaLabel"] != "Buy" {
		t.Fatalf("ja: %v", got)
	}
	items, _ = s.Published(ctx, "hero_slide", "xx", "", nil) // unknown lang -> en
	if items[0].Texts["title"] != "Title f" {
		t.Fatalf("xx: %v", items[0].Texts)
	}
	items, _ = s.Published(ctx, "hero_slide", "de", "", nil) // en subtitle missing -> th
	if items[0].Texts["subtitle"] != "คำโปรย" {
		t.Fatalf("de: %v", items[0].Texts)
	}
}

func TestUnknownType(t *testing.T) {
	s, _ := newSvc(t)
	mux := http.NewServeMux()
	(&API{Svc: s}).Register(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/public/items?type=nope&lang=th", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"unknown_type"`) {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
}

func TestPreviewExpiry(t *testing.T) {
	s, _ := newSvc(t)
	ctx := testutil.Ctx(t)
	if _, err := Seed(ctx, s.DB, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if n, _ := Seed(ctx, s.DB, time.Now().UTC()); n != 0 {
		t.Fatalf("seed not idempotent: %d", n)
	}
	items, _ := s.ListItems(ctx, "hero_slide")
	id := items[0].ID
	in := heroInput("preview")
	if err := s.SaveDraft(ctx, editor, id, in); err != nil {
		t.Fatal(err)
	}
	tok, _, err := s.CreatePreview(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Preview(ctx, tok, "th")
	if err != nil || got[0].ID != id || got[0].Texts["title"] != "หัวข้อ preview" || len(got) != len(items) {
		t.Fatalf("preview: %v %v", got, err)
	}
	pub, _ := s.Published(ctx, "hero_slide", "th", "", nil)
	if pub[0].Texts["title"] == "หัวข้อ preview" {
		t.Fatal("draft leaked to public")
	}
	s.Now = func() time.Time { return time.Now().Add(31 * time.Minute) }
	_, err = s.Preview(ctx, tok, "th")
	wantCode(t, err, "preview_expired")
	_, err = s.Preview(ctx, "nope", "th")
	wantCode(t, err, "preview_expired")
}
