package main

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var tplFS embed.FS

var tpl = template.Must(template.ParseFS(tplFS, "templates/*.html"))

func (a *App) consoleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "console.html", map[string]any{
			"SMS":      a.Store.List("sms"),
			"Webhooks": a.Store.List("webhook"),
			"Payments": a.paymentList(),
			"Public":   a.PublicURL,
		})
	})
}
