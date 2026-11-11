package main

import "testing"

func TestResolveAndValidate(t *testing.T) {
	def := Registry["faq"]
	tx := Texts{"th": {"question": "ถาม", "answer": "ตอบ"}, "en": {"question": "Q"}, "zh": {"answer": "答"}}
	got := Resolve(def, tx, "zh")
	if got["question"] != "Q" || got["answer"] != "答" {
		t.Fatal(got)
	}
	if miss := missingRequired(def, map[string]any{"order": 1}, tx); len(miss) != 1 || miss[0] != "en.answer" {
		t.Fatal(miss)
	}
	if _, ok := cleanAttrs(Registry["show"], map[string]any{"times": []any{"25:00"}}); ok {
		t.Fatal("bad time accepted")
	}
	a, ok := cleanAttrs(Registry["show"], map[string]any{"order": 2.0, "times": []any{"13:30", "10:30"}, "x": 1})
	if !ok || a["order"] != 2 || a["times"].([]string)[0] != "10:30" || a["x"] != nil {
		t.Fatal(a)
	}
}
