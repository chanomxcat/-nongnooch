//go:build integration

package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStatsTodayCountsBangkokDay(t *testing.T) {
	e := newEnv(t)
	ict := time.FixedZone("ICT", 7*3600)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, ict)
	e.svc.Now = func() time.Time { return now }
	ins := func(id string, at time.Time) {
		if _, err := e.d.Exec(`INSERT INTO members(id, phone, first_name, last_name, nationality, id_kind, id_country, id_last4, thai_verified, lang, channel, created_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, "+66"+id, "A", "B", "TH", "national", "TH", "0708", true, "th", "web", at.UTC()); err != nil {
			t.Fatal(err)
		}
	}
	ins("800000001", time.Date(2026, 11, 11, 0, 30, 0, 0, ict))  // today ICT (previous day in UTC)
	ins("800000002", time.Date(2026, 11, 11, 11, 0, 0, 0, ict))  // today
	ins("800000003", time.Date(2026, 11, 10, 23, 59, 0, 0, ict)) // yesterday ICT
	e.d.Exec(`UPDATE members SET deleted_at = ? WHERE phone='+66800000002'`, now.UTC())

	if w := e.as("", "GET", "/admin/stats/today", nil); w.Code != 401 {
		t.Fatalf("anon: %d", w.Code)
	}
	if w := e.as("+66811111111", "GET", "/admin/stats/today", nil); w.Code != 401 && w.Code != 403 {
		t.Fatalf("user token: %d", w.Code)
	}
	w := e.staff("GET", "/admin/stats/today")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var out struct{ NewMembersToday, TotalMembers int }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.NewMembersToday != 1 || out.TotalMembers != 2 {
		t.Fatalf("got %+v", out)
	}
}
