package main

import (
	"encoding/json"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *App) turnstileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /turnstile/v0/siteverify", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.PostForm.Get("response") {
		case "mock-fail":
			writeJSON(w, 200, map[string]any{"success": false, "error-codes": []string{"invalid-input-response"}})
		case "mock-timeout":
			a.Sleep(5 * time.Second)
			writeJSON(w, 200, map[string]any{"success": true})
		default:
			writeJSON(w, 200, map[string]any{"success": true})
		}
	})
}
