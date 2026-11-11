package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/authn"
	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
)

// APIError is returned to the client as {"error": Code}.
type APIError struct {
	Status int
	Code   string
	Extra  map[string]any
}

func (e *APIError) Error() string { return e.Code }

func apiErr(status int, code string) *APIError { return &APIError{Status: status, Code: code} }

var (
	errUnknownType    = apiErr(400, "unknown_type")
	errBadRequest     = apiErr(400, "bad_request")
	errNotFound       = apiErr(404, "not_found")
	errNotEditable    = apiErr(409, "not_editable")
	errNoDraft        = apiErr(409, "no_draft")
	errNotPending     = apiErr(409, "not_pending")
	errFourEyes       = apiErr(403, "four_eyes_violation")
	errReasonRequired = apiErr(400, "reason_required")
	errPreviewExpired = apiErr(404, "preview_expired")
	errForbidden      = apiErr(403, "forbidden")
)

const previewTTL = 30 * time.Minute

type Svc struct {
	DB       *sql.DB
	FourEyes bool
	SiteURL  string
	Now      func() time.Time
}

func (s *Svc) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func newID() string { return uuid.Must(uuid.NewV7()).String() }

// Rev is one stored version of an item.
type Rev struct {
	ID    string         `json:"revId"`
	Attrs map[string]any `json:"attrs"`
	Texts Texts          `json:"texts"`
}

type q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func loadRev(ctx context.Context, db q, id sql.NullString) (*Rev, error) {
	if !id.Valid {
		return nil, nil
	}
	var a, t string
	if err := db.QueryRowContext(ctx, `SELECT attrs, texts FROM content_revisions WHERE id = ?`, id.String).Scan(&a, &t); err != nil {
		return nil, err
	}
	r := &Rev{ID: id.String}
	if err := json.Unmarshal([]byte(a), &r.Attrs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(t), &r.Texts); err != nil {
		return nil, err
	}
	return r, nil
}

func insertRev(ctx context.Context, tx *sql.Tx, itemID, actor string, attrs map[string]any, texts Texts, at time.Time) (string, error) {
	a, _ := json.Marshal(attrs)
	t, _ := json.Marshal(texts)
	id := newID()
	_, err := tx.ExecContext(ctx, `INSERT INTO content_revisions(id, item_id, attrs, texts, created_by, created_at) VALUES(?,?,?,?,?,?)`,
		id, itemID, string(a), string(t), actor, at)
	return id, err
}

func audit(ctx context.Context, tx *sql.Tx, p authn.Principal, action, entity, entityID string, before, after any, reason string, at time.Time) error {
	js := func(v any) sql.NullString {
		if v == nil {
			return sql.NullString{}
		}
		b, _ := json.Marshal(v)
		if string(b) == "null" {
			return sql.NullString{}
		}
		return sql.NullString{String: string(b), Valid: true}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_log(id, at, actor_id, actor_roles, action, entity, entity_id, before_json, after_json, reason) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		newID(), at, p.Sub, strings.Join(p.Roles, ","), action, entity, entityID, js(before), js(after), sql.NullString{String: reason, Valid: reason != ""})
	return err
}

func (s *Svc) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ---------- public ----------

type PublicItem struct {
	ID    string            `json:"id"`
	Type  string            `json:"type"`
	Attrs map[string]any    `json:"attrs"`
	Texts map[string]string `json:"texts"`
}

// Published returns the published items of typ resolved to lang; override replaces/adds one item (preview).
func (s *Svc) Published(ctx context.Context, typ, lang string, overrideItem string, override *Rev) ([]PublicItem, error) {
	def, ok := Registry[typ]
	if !ok {
		return nil, errUnknownType
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT i.id, r.id, r.attrs, r.texts FROM content_items i JOIN content_revisions r ON r.id = i.published_rev_id WHERE i.type = ?`, typ)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicItem{}
	seen := false
	for rows.Next() {
		var id, rid, a, t string
		if err := rows.Scan(&id, &rid, &a, &t); err != nil {
			return nil, err
		}
		r := Rev{ID: rid}
		_ = json.Unmarshal([]byte(a), &r.Attrs)
		_ = json.Unmarshal([]byte(t), &r.Texts)
		if id == overrideItem && override != nil {
			r, seen = *override, true
		}
		out = append(out, PublicItem{ID: id, Type: typ, Attrs: r.Attrs, Texts: Resolve(def, r.Texts, lang)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if override != nil && !seen {
		out = append(out, PublicItem{ID: overrideItem, Type: typ, Attrs: override.Attrs, Texts: Resolve(def, override.Texts, lang)})
	}
	sort.SliceStable(out, func(i, j int) bool { return orderOf(out[i].Attrs) < orderOf(out[j].Attrs) })
	return out, nil
}

// Preview returns the type list with the token's draft in place.
func (s *Svc) Preview(ctx context.Context, token, lang string) ([]PublicItem, error) {
	var itemID, revID, typ string
	var exp time.Time
	err := s.DB.QueryRowContext(ctx, `SELECT p.item_id, p.rev_id, p.expires_at, i.type FROM preview_tokens p JOIN content_items i ON i.id = p.item_id WHERE p.token = ?`, token).
		Scan(&itemID, &revID, &exp, &typ)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !s.now().Before(exp)) {
		return nil, errPreviewExpired
	}
	if err != nil {
		return nil, err
	}
	rev, err := loadRev(ctx, s.DB, sql.NullString{String: revID, Valid: true})
	if err != nil {
		return nil, err
	}
	return s.Published(ctx, typ, lang, itemID, rev)
}

// ---------- staff ----------

type Approval struct {
	ID          string    `json:"id"`
	ItemID      string    `json:"itemId"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Editors     []string  `json:"editors"`
	SubmittedBy string    `json:"submittedBy,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	DecidedBy   string    `json:"decidedBy,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Before      *Rev      `json:"before"`
	After       *Rev      `json:"after"`
}

const approvalCols = `a.id, a.item_id, i.type, a.status, a.editors, COALESCE(a.submitted_by,''), COALESCE(a.reason,''), COALESCE(a.decided_by,''), a.created_at, a.updated_at, a.rev_id, i.published_rev_id`

type scanner interface{ Scan(dest ...any) error }

func scanApproval(sc scanner) (*Approval, string, sql.NullString, error) {
	var a Approval
	var editors, revID string
	var pub sql.NullString
	if err := sc.Scan(&a.ID, &a.ItemID, &a.Type, &a.Status, &editors, &a.SubmittedBy, &a.Reason, &a.DecidedBy, &a.CreatedAt, &a.UpdatedAt, &revID, &pub); err != nil {
		return nil, "", pub, err
	}
	_ = json.Unmarshal([]byte(editors), &a.Editors)
	return &a, revID, pub, nil
}

type ItemSummary struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Status    string            `json:"status"` // published | draft | pending
	Published bool              `json:"published"`
	Attrs     map[string]any    `json:"attrs"`
	Texts     map[string]string `json:"texts"` // resolved th
	UpdatedAt time.Time         `json:"updatedAt"`
}

func (s *Svc) ListItems(ctx context.Context, typ string) ([]ItemSummary, error) {
	def, ok := Registry[typ]
	if !ok {
		return nil, errUnknownType
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT i.id, i.updated_at, i.published_rev_id IS NOT NULL, COALESCE(a.status,'published'), r.attrs, r.texts
FROM content_items i
LEFT JOIN approval_requests a ON a.item_id = i.id AND a.status IN ('draft','pending')
JOIN content_revisions r ON r.id = COALESCE(a.rev_id, i.published_rev_id)
WHERE i.type = ?`, typ)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ItemSummary{}
	for rows.Next() {
		var it ItemSummary
		var a, t string
		var tx Texts
		if err := rows.Scan(&it.ID, &it.UpdatedAt, &it.Published, &it.Status, &a, &t); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(a), &it.Attrs)
		_ = json.Unmarshal([]byte(t), &tx)
		it.Type, it.Texts = typ, Resolve(def, tx, "th")
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool { return orderOf(out[i].Attrs) < orderOf(out[j].Attrs) })
	return out, rows.Err()
}

type ItemDetail struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Published    *Rev      `json:"published"`
	Draft        *Approval `json:"draft"`        // open request (draft|pending), After = draft content
	LastDecision *Approval `json:"lastDecision"` // latest approved/rejected request
}

func (s *Svc) GetItem(ctx context.Context, id string) (*ItemDetail, error) {
	d := &ItemDetail{ID: id}
	var pub sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT type, published_rev_id FROM content_items WHERE id = ?`, id).Scan(&d.Type, &pub)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if d.Published, err = loadRev(ctx, s.DB, pub); err != nil {
		return nil, err
	}
	load := func(where string) (*Approval, error) {
		row := s.DB.QueryRowContext(ctx, `SELECT `+approvalCols+` FROM approval_requests a JOIN content_items i ON i.id = a.item_id WHERE a.item_id = ? AND `+where+` ORDER BY a.updated_at DESC LIMIT 1`, id)
		a, revID, _, err := scanApproval(row)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		a.After, err = loadRev(ctx, s.DB, sql.NullString{String: revID, Valid: true})
		return a, err
	}
	if d.Draft, err = load(`a.status IN ('draft','pending')`); err != nil {
		return nil, err
	}
	if d.LastDecision, err = load(`a.status IN ('approved','rejected')`); err != nil {
		return nil, err
	}
	return d, nil
}

type EditInput struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs"`
	Texts Texts          `json:"texts"`
}

func validate(typ string, in EditInput) (map[string]any, Texts, error) {
	def, ok := Registry[typ]
	if !ok {
		return nil, nil, errUnknownType
	}
	attrs, ok := cleanAttrs(def, in.Attrs)
	if !ok {
		return nil, nil, apiErr(400, "invalid_attrs")
	}
	return attrs, cleanTexts(def, in.Texts), nil
}

// CreateItem creates an unpublished item with a draft request.
func (s *Svc) CreateItem(ctx context.Context, p authn.Principal, in EditInput) (string, error) {
	attrs, texts, err := validate(in.Type, in)
	if err != nil {
		return "", err
	}
	id, now := newID(), s.now()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_items(id, type, created_at, updated_at) VALUES(?,?,?,?)`, id, in.Type, now, now); err != nil {
			return err
		}
		return s.saveDraftTx(ctx, tx, p, id, in.Type, attrs, texts, now)
	})
	return id, err
}

// SaveDraft stores a new revision as the item's draft (409 not_editable while pending).
func (s *Svc) SaveDraft(ctx context.Context, p authn.Principal, id string, in EditInput) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var typ string
		err := tx.QueryRowContext(ctx, `SELECT type FROM content_items WHERE id = ? FOR UPDATE`, id).Scan(&typ)
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		attrs, texts, err := validate(typ, in)
		if err != nil {
			return err
		}
		return s.saveDraftTx(ctx, tx, p, id, typ, attrs, texts, s.now())
	})
}

func (s *Svc) saveDraftTx(ctx context.Context, tx *sql.Tx, p authn.Principal, id, typ string, attrs map[string]any, texts Texts, now time.Time) error {
	var aid, status, editors string
	err := tx.QueryRowContext(ctx, `SELECT id, status, editors FROM approval_requests WHERE item_id = ? AND status IN ('draft','pending') FOR UPDATE`, id).Scan(&aid, &status, &editors)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if status == "pending" {
		return errNotEditable
	}
	revID, err := insertRev(ctx, tx, id, p.Sub, attrs, texts, now)
	if err != nil {
		return err
	}
	var eds []string
	_ = json.Unmarshal([]byte(editors), &eds)
	if !slices.Contains(eds, p.Sub) {
		eds = append(eds, p.Sub)
	}
	eb, _ := json.Marshal(eds)
	if aid == "" {
		aid = newID()
		_, err = tx.ExecContext(ctx, `INSERT INTO approval_requests(id, item_id, rev_id, status, editors, created_at, updated_at) VALUES(?,?,?,'draft',?,?,?)`,
			aid, id, revID, string(eb), now, now)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE approval_requests SET rev_id = ?, editors = ?, updated_at = ? WHERE id = ?`, revID, string(eb), now, aid)
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE content_items SET updated_at = ? WHERE id = ?`, now, id); err != nil {
		return err
	}
	return audit(ctx, tx, p, "content.draft_saved", "content_item", id, nil, Rev{ID: revID, Attrs: attrs, Texts: texts}, "", now)
}

// CreatePreview issues a 30-minute token for the item's current draft.
func (s *Svc) CreatePreview(ctx context.Context, id string) (string, time.Time, error) {
	var revID string
	err := s.DB.QueryRowContext(ctx, `SELECT rev_id FROM approval_requests WHERE item_id = ? AND status IN ('draft','pending')`, id).Scan(&revID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, errNoDraft
	}
	if err != nil {
		return "", time.Time{}, err
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)
	exp := s.now().Add(previewTTL)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO preview_tokens(token, item_id, rev_id, expires_at) VALUES(?,?,?,?)`, tok, id, revID, exp)
	return tok, exp, err
}

// Submit moves the draft to pending after checking TH+EN required fields.
func (s *Svc) Submit(ctx context.Context, p authn.Principal, id string) (string, error) {
	var aid string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var revID, status, typ string
		err := tx.QueryRowContext(ctx, `SELECT a.id, a.rev_id, a.status, i.type FROM approval_requests a JOIN content_items i ON i.id = a.item_id WHERE a.item_id = ? AND a.status IN ('draft','pending') FOR UPDATE`, id).
			Scan(&aid, &revID, &status, &typ)
		if errors.Is(err, sql.ErrNoRows) {
			return errNoDraft
		}
		if err != nil {
			return err
		}
		if status == "pending" {
			return errNotEditable
		}
		rev, err := loadRev(ctx, tx, sql.NullString{String: revID, Valid: true})
		if err != nil {
			return err
		}
		if miss := missingRequired(Registry[typ], rev.Attrs, rev.Texts); len(miss) > 0 {
			return &APIError{Status: 400, Code: "missing_required", Extra: map[string]any{"fields": miss}}
		}
		now := s.now()
		if _, err := tx.ExecContext(ctx, `UPDATE approval_requests SET status = 'pending', submitted_by = ?, updated_at = ? WHERE id = ?`, p.Sub, now, aid); err != nil {
			return err
		}
		return audit(ctx, tx, p, "content.submitted", "approval_request", aid, nil, map[string]any{"itemId": id, "revId": revID}, "", now)
	})
	return aid, err
}

func (s *Svc) ListApprovals(ctx context.Context, status string) ([]Approval, error) {
	if status == "" {
		status = "pending"
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+approvalCols+` FROM approval_requests a JOIN content_items i ON i.id = a.item_id WHERE a.status = ? ORDER BY a.updated_at DESC LIMIT 100`, status)
	if err != nil {
		return nil, err
	}
	type pair struct {
		rev string
		pub sql.NullString
	}
	var list []Approval
	var revs []pair
	for rows.Next() {
		a, rev, pub, err := scanApproval(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, *a)
		revs = append(revs, pair{rev, pub})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].After, err = loadRev(ctx, s.DB, sql.NullString{String: revs[i].rev, Valid: true}); err != nil {
			return nil, err
		}
		// ponytail: "before" is the item's current published rev, not the one at submit time; fine for a single open request per item.
		if list[i].Before, err = loadRev(ctx, s.DB, revs[i].pub); err != nil {
			return nil, err
		}
	}
	if list == nil {
		list = []Approval{}
	}
	return list, nil
}

// Decide approves (publishes) or rejects a pending request. Exactly one concurrent caller wins.
func (s *Svc) Decide(ctx context.Context, p authn.Principal, aid string, approve bool, reason string) error {
	reason = strings.TrimSpace(reason)
	if !approve && reason == "" {
		return errReasonRequired
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `SELECT `+approvalCols+` FROM approval_requests a JOIN content_items i ON i.id = a.item_id WHERE a.id = ?`, aid)
		a, revID, pub, err := scanApproval(row)
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if a.Status != "pending" {
			return errNotPending
		}
		if s.FourEyes && slices.Contains(a.Editors, p.Sub) {
			return errFourEyes
		}
		// Role is checked after four-eyes so an editor trying to approve their own change sees why.
		if !p.HasAnyRole("approver", "admin") {
			return errForbidden
		}
		status := map[bool]string{true: "approved", false: "rejected"}[approve]
		now := s.now()
		res, err := tx.ExecContext(ctx, `UPDATE approval_requests SET status = ?, decided_by = ?, decided_at = ?, updated_at = ?, reason = ? WHERE id = ? AND status = 'pending'`,
			status, p.Sub, now, now, sql.NullString{String: reason, Valid: reason != ""}, aid)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errNotPending
		}
		before, err := loadRev(ctx, tx, pub)
		if err != nil {
			return err
		}
		after, err := loadRev(ctx, tx, sql.NullString{String: revID, Valid: true})
		if err != nil {
			return err
		}
		if !approve {
			return audit(ctx, tx, p, "content.rejected", "content_item", a.ItemID, before, after, reason, now)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE content_items SET published_rev_id = ?, updated_at = ? WHERE id = ?`, revID, now, a.ItemID); err != nil {
			return err
		}
		if err := audit(ctx, tx, p, "content.published", "content_item", a.ItemID, before, after, "", now); err != nil {
			return err
		}
		locales := []string{}
		urls := []string{}
		for _, l := range Langs {
			if len(after.Texts[l]) > 0 {
				locales = append(locales, l)
			}
			urls = append(urls, "/"+l)
		}
		ev, err := events.New(events.ContentPublished, map[string]any{
			"approvalId": aid, "action": "publish", "contentType": a.Type, "contentId": a.ItemID, "locales": locales, "urls": urls,
		})
		if err != nil {
			return err
		}
		return outbox.Write(ctx, tx, ev)
	})
}

type AuditRow struct {
	ID         string          `json:"id"`
	At         time.Time       `json:"at"`
	ActorID    string          `json:"actorId"`
	ActorRoles []string        `json:"actorRoles"`
	Action     string          `json:"action"`
	Entity     string          `json:"entity"`
	EntityID   string          `json:"entityId"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
	Reason     string          `json:"reason,omitempty"`
}

func (s *Svc) Audit(ctx context.Context, entity, entityID, cursor string, limit int) ([]AuditRow, string, error) {
	where, args := []string{"1=1"}, []any{}
	if entity != "" {
		where, args = append(where, "entity = ?"), append(args, entity)
	}
	if entityID != "" {
		where, args = append(where, "entity_id = ?"), append(args, entityID)
	}
	if cursor != "" {
		where, args = append(where, "id < ?"), append(args, cursor)
	}
	args = append(args, limit+1)
	rows, err := s.DB.QueryContext(ctx, `SELECT id, at, actor_id, actor_roles, action, entity, entity_id, COALESCE(before_json,'null'), COALESCE(after_json,'null'), COALESCE(reason,'')
FROM audit_log WHERE `+strings.Join(where, " AND ")+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []AuditRow{}
	for rows.Next() {
		var r AuditRow
		var roles, b, a string
		if err := rows.Scan(&r.ID, &r.At, &r.ActorID, &roles, &r.Action, &r.Entity, &r.EntityID, &b, &a, &r.Reason); err != nil {
			return nil, "", err
		}
		r.ActorRoles = strings.Split(roles, ",")
		r.Before, r.After = json.RawMessage(b), json.RawMessage(a)
		out = append(out, r)
	}
	next := ""
	if len(out) > limit {
		out = out[:limit]
		next = out[limit-1].ID
	}
	return out, next, rows.Err()
}

// RememberName stores the display name of a staff caller (from the gateway-verified JWT).
func (s *Svc) RememberName(ctx context.Context, sub, name string) {
	if sub == "" || name == "" {
		return
	}
	_, _ = s.DB.ExecContext(ctx, `INSERT INTO staff_names(sub, name) VALUES(?,?) ON DUPLICATE KEY UPDATE name = VALUES(name)`, sub, name)
}

// Names maps staff sub -> display name (demo scale: the whole table).
func (s *Svc) Names(ctx context.Context) map[string]string {
	out := map[string]string{}
	rows, err := s.DB.QueryContext(ctx, `SELECT sub, name FROM staff_names`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			out[k] = v
		}
	}
	return out
}
