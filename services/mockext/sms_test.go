package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func send(mux *http.ServeMux, to, text string) int {
	b, _ := json.Marshal(map[string]string{"to": to, "text": text})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/sms/send", strings.NewReader(string(b))))
	return rec.Code
}

func TestSMSChineseAndUndeliverable(t *testing.T) {
	a := newTestApp()
	mux := http.NewServeMux()
	a.Routes(mux)
	if c := send(mux, "+8613800000000", "您的验证码 123456"); c != 202 {
		t.Fatal(c)
	}
	if got := a.Store.List("sms")[0].Body; got != "您的验证码 123456" {
		t.Fatal(got)
	}
	if c := send(mux, "+99912345", "x"); c != 400 {
		t.Fatal(c)
	}
}

func TestSMSMessagesByRecipient(t *testing.T) {
	a := newTestApp()
	mux := http.NewServeMux()
	a.Routes(mux)
	send(mux, "+66811111111", "first")
	send(mux, "+66822222222", "other")
	send(mux, "+66811111111", "second")
	get := func(q string) []map[string]any {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/sms/messages?to="+q, nil))
		var out struct{ Items []map[string]any }
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err, rec.Body.String())
		}
		return out.Items
	}
	for _, q := range []string{"+66811111111", "%2B66811111111"} {
		it := get(q)
		if len(it) != 2 || it[0]["text"] != "second" || it[1]["text"] != "first" {
			t.Fatal(q, it)
		}
	}
	if it := get("%2B66899999999"); it == nil || len(it) != 0 {
		t.Fatal(it)
	}
}
