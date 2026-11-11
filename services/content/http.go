package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nongnooch/platform/pkg/chassis/authn"
)

// API serves the content HTTP endpoints (behind the gateway at /api/content).
type API struct {
	Svc      *Svc
	DemoMode bool
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var ae *APIError
	if errors.As(err, &ae) {
		body := map[string]any{"error": ae.Code, "message": ae.Code}
		for k, v := range ae.Extra {
			body[k] = v
		}
		writeJSON(w, ae.Status, body)
		return
	}
	slog.Error("content: internal error", "err", err)
	writeJSON(w, 500, map[string]string{"error": "internal", "message": "internal"})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, errBadRequest)
		return false
	}
	return true
}

func (a *API) Register(mux *http.ServeMux) {
	staff := authn.Require("staff")
	edit := authn.RequireRole("editor", "admin")
	h := func(fn http.HandlerFunc) http.Handler { return a.names(fn) }

	mux.HandleFunc("GET /public/items", a.publicItems)
	mux.HandleFunc("GET /public/preview/{token}", a.publicPreview)

	mux.Handle("GET /admin/types", staff(h(a.types)))
	mux.Handle("GET /admin/items", staff(h(a.listItems)))
	mux.Handle("GET /admin/items/{id}", staff(h(a.getItem)))
	mux.Handle("POST /admin/items", edit(h(a.createItem)))
	mux.Handle("POST /admin/items/{id}/draft", edit(h(a.saveDraft)))
	mux.Handle("POST /admin/items/{id}/preview", edit(h(a.preview)))
	mux.Handle("POST /admin/items/{id}/submit", edit(h(a.submit)))
	mux.Handle("GET /admin/approvals", staff(h(a.approvals)))
	// Decide checks four-eyes first, then the approver/admin role.
	mux.Handle("POST /admin/approvals/{id}/approve", staff(h(a.approve)))
	mux.Handle("POST /admin/approvals/{id}/reject", staff(h(a.reject)))
	mux.Handle("GET /admin/audit", staff(h(a.audit)))
	mux.HandleFunc("POST /internal/seed", a.seed)
}

func (a *API) publicItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := a.Svc.Published(r.Context(), q.Get("type"), q.Get("lang"), "", nil)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) publicPreview(w http.ResponseWriter, r *http.Request) {
	items, err := a.Svc.Preview(r.Context(), r.PathValue("token"), r.URL.Query().Get("lang"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"items": items, "preview": true})
}

func (a *API) types(w http.ResponseWriter, r *http.Request) {
	out := []TypeDef{}
	for _, t := range TypeOrder {
		out = append(out, Registry[t])
	}
	writeJSON(w, 200, map[string]any{"types": out, "langs": Langs})
}

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	items, err := a.Svc.ListItems(r.Context(), r.URL.Query().Get("type"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) getItem(w http.ResponseWriter, r *http.Request) {
	d, err := a.Svc.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": d.ID, "type": d.Type, "published": d.Published, "draft": d.Draft, "lastDecision": d.LastDecision, "names": a.Svc.Names(r.Context())})
}

func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	var in EditInput
	if !decode(w, r, &in) {
		return
	}
	id, err := a.Svc.CreateItem(r.Context(), authn.From(r.Context()), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func (a *API) saveDraft(w http.ResponseWriter, r *http.Request) {
	var in EditInput
	if !decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	if err := a.Svc.SaveDraft(r.Context(), authn.From(r.Context()), id, in); err != nil {
		writeErr(w, err)
		return
	}
	a.getItem(w, r)
}

func (a *API) preview(w http.ResponseWriter, r *http.Request) {
	tok, exp, err := a.Svc.CreatePreview(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"token": tok, "expiresAt": exp, "url": a.Svc.SiteURL + "/th?preview=" + tok})
}

func (a *API) submit(w http.ResponseWriter, r *http.Request) {
	aid, err := a.Svc.Submit(r.Context(), authn.From(r.Context()), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"approvalId": aid, "status": "pending"})
}

func (a *API) approvals(w http.ResponseWriter, r *http.Request) {
	list, err := a.Svc.ListApprovals(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": list, "names": a.Svc.Names(r.Context())})
}

func (a *API) approve(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.Decide(r.Context(), authn.From(r.Context()), r.PathValue("id"), true, ""); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "approved"})
}

func (a *API) reject(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &b) {
		return
	}
	if err := a.Svc.Decide(r.Context(), authn.From(r.Context()), r.PathValue("id"), false, b.Reason); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "rejected"})
}

func (a *API) audit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, next, err := a.Svc.Audit(r.Context(), q.Get("entity"), q.Get("entityId"), q.Get("cursor"), limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows, "nextCursor": next, "names": a.Svc.Names(r.Context())})
}

// names records the caller's display name on writes. The gateway has already verified the bearer
// token and set X-User-Sub from it, so reading the payload's "name" here is safe.
func (a *API) names(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := authn.From(r.Context())
		if p.Sub == "" {
			p = authn.FromHeaders(r.Header)
		}
		if r.Method != http.MethodGet && p.Type == "staff" {
			if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
				if parts := strings.Split(tok, "."); len(parts) == 3 {
					var c struct{ Sub, Name string }
					if b, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil && json.Unmarshal(b, &c) == nil && c.Sub == p.Sub {
						a.Svc.RememberName(r.Context(), c.Sub, c.Name)
					}
				}
			}
		}
		next(w, r)
	})
}

//go:embed seed.json
var seedJSON []byte

func (a *API) seed(w http.ResponseWriter, r *http.Request) {
	if !a.DemoMode {
		http.NotFound(w, r)
		return
	}
	n, err := Seed(r.Context(), a.Svc.DB, time.Now().UTC())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"inserted": n})
}

// Seed inserts the published demo items from seed.json once (keyed by seed_key).
func Seed(ctx context.Context, d *sql.DB, now time.Time) (int, error) {
	var f struct {
		Items []struct {
			Key   string         `json:"key"`
			Type  string         `json:"type"`
			Attrs map[string]any `json:"attrs"`
			Texts Texts          `json:"texts"`
		} `json:"items"`
	}
	if err := json.Unmarshal(seedJSON, &f); err != nil {
		return 0, err
	}
	n := 0
	for _, it := range f.Items {
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return n, err
		}
		id := newID()
		res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO content_items(id, seed_key, type, created_at, updated_at) VALUES(?,?,?,?,?)`, id, it.Key, it.Type, now, now)
		if err != nil {
			_ = tx.Rollback()
			return n, err
		}
		if k, _ := res.RowsAffected(); k == 0 {
			_ = tx.Rollback()
			continue
		}
		rev, err := insertRev(ctx, tx, id, "seed", it.Attrs, it.Texts, now)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE content_items SET published_rev_id = ? WHERE id = ?`, rev, id)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			_ = tx.Rollback()
			return n, err
		}
		n++
	}
	return n, nil
}
