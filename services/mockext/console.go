package main

import (
	"embed"
	"html/template"
	"net/http"
	"time"
)

//go:embed templates/*.html
var tplFS embed.FS

// ict is Asia/Bangkok (UTC+7, no DST); a fixed zone avoids needing tzdata in distroless.
var ict = time.FixedZone("ICT", 7*3600)

var tpl = template.Must(template.New("").Funcs(template.FuncMap{
	"clock": func(t time.Time) string { return t.In(ict).Format("15:04:05") },
}).ParseFS(tplFS, "templates/*.html"))

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
