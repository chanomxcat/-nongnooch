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
	for _, bad := range []string{"javascript:alert(1)", "//evil.example", `/\evil.example`, "http://evil.example", "data:text/html,x"} {
		if _, ok := cleanAttrs(Registry["hero_slide"], map[string]any{"ctaHref": bad}); ok {
			t.Fatalf("unsafe ctaHref accepted: %s", bad)
		}
		if _, ok := cleanAttrs(Registry["hero_slide"], map[string]any{"imageUrl": bad}); ok {
			t.Fatalf("unsafe imageUrl accepted: %s", bad)
		}
	}
	for _, good := range []string{"/checkout", "https://cdn.example/x.webp"} {
		if _, ok := cleanAttrs(Registry["hero_slide"], map[string]any{"ctaHref": good, "imageUrl": good}); !ok {
			t.Fatalf("safe url rejected: %s", good)
		}
	}
	a, ok := cleanAttrs(Registry["show"], map[string]any{"order": 2.0, "times": []any{"13:30", "10:30"}, "x": 1})
	if !ok || a["order"] != 2 || a["times"].([]string)[0] != "10:30" || a["x"] != nil {
		t.Fatal(a)
	}
}
