package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (a *App) smsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sms/send", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ To, Text string }
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.To == "" {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		if strings.HasPrefix(in.To, "+999") {
			writeJSON(w, 400, map[string]string{"error": "undeliverable"})
			return
		}
		id := newID("sms_")
		a.Store.Add("sms", Entry{Title: in.To, Body: in.Text, Meta: map[string]string{"messageId": id, "to": in.To}})
		writeJSON(w, 202, map[string]string{"messageId": id})
	})
	mux.HandleFunc("GET /sms/messages", func(w http.ResponseWriter, r *http.Request) {
		to := r.URL.Query().Get("to")
		if strings.HasPrefix(to, " ") { // an unescaped "+" arrives as a space
			to = "+" + strings.TrimLeft(to, " ")
		}
		items := []map[string]any{}
		for _, e := range a.Store.List("sms") {
			if to == "" || e.Meta["to"] == to {
				items = append(items, map[string]any{"messageId": e.Meta["messageId"], "to": e.Meta["to"], "text": e.Body, "at": e.At})
			}
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
}
