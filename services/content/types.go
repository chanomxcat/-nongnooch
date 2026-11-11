package main

import (
	"regexp"
	"sort"
)

// Langs are the site languages; TH and EN are required on submit.
var Langs = []string{"th", "en", "zh", "ja", "ko", "de", "fr"}

type AttrKind string

const (
	KindInt    AttrKind = "int"
	KindString AttrKind = "string"
	KindTimes  AttrKind = "times" // []"HH:MM"
)

type Field struct {
	Key      string   `json:"key"`
	Kind     AttrKind `json:"kind,omitempty"`
	Label    string   `json:"label"`
	Required bool     `json:"required"`
	Long     bool     `json:"long,omitempty"`
}

type TypeDef struct {
	Type  string  `json:"type"`
	Label string  `json:"label"`
	Attrs []Field `json:"attrs"`
	Texts []Field `json:"texts"`
}

// Registry is the demo type registry (contract: docs/demo-cms-app-contract.md).
var Registry = map[string]TypeDef{
	"hero_slide": {Type: "hero_slide", Label: "สไลด์หน้าแรก",
		Attrs: []Field{{Key: "order", Kind: KindInt, Label: "ลำดับ", Required: true}, {Key: "imageUrl", Kind: KindString, Label: "รูปภาพ (URL)", Required: true}, {Key: "ctaHref", Kind: KindString, Label: "ลิงก์ปุ่ม"}},
		Texts: []Field{{Key: "title", Label: "หัวข้อ", Required: true}, {Key: "subtitle", Label: "คำโปรย"}, {Key: "ctaLabel", Label: "ข้อความปุ่ม"}}},
	"show": {Type: "show", Label: "รอบการแสดง",
		Attrs: []Field{{Key: "order", Kind: KindInt, Label: "ลำดับ", Required: true}, {Key: "times", Kind: KindTimes, Label: "เวลา (HH:MM คั่นด้วย ,)", Required: true}, {Key: "imageUrl", Kind: KindString, Label: "รูปภาพ (URL)"}},
		Texts: []Field{{Key: "name", Label: "ชื่อการแสดง", Required: true}, {Key: "venue", Label: "สถานที่"}, {Key: "description", Label: "รายละเอียด", Long: true}}},
	"faq": {Type: "faq", Label: "คำถามที่พบบ่อย",
		Attrs: []Field{{Key: "order", Kind: KindInt, Label: "ลำดับ", Required: true}},
		Texts: []Field{{Key: "question", Label: "คำถาม", Required: true}, {Key: "answer", Label: "คำตอบ", Required: true, Long: true}}},
}

// TypeOrder is the display order of types in the admin.
var TypeOrder = []string{"hero_slide", "show", "faq"}

type Texts map[string]map[string]string

var hhmm = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// cleanAttrs keeps only registry attrs with the right kind (JSON numbers become int).
func cleanAttrs(def TypeDef, in map[string]any) (map[string]any, bool) {
	out := map[string]any{}
	for _, f := range def.Attrs {
		v, ok := in[f.Key]
		if !ok || v == nil {
			continue
		}
		switch f.Kind {
		case KindInt:
			n, ok := v.(float64)
			if !ok || n != float64(int(n)) {
				return nil, false
			}
			out[f.Key] = int(n)
		case KindString:
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			if s != "" {
				out[f.Key] = s
			}
		case KindTimes:
			arr, ok := v.([]any)
			if !ok {
				return nil, false
			}
			ts := []string{}
			for _, x := range arr {
				s, ok := x.(string)
				if !ok || !hhmm.MatchString(s) {
					return nil, false
				}
				ts = append(ts, s)
			}
			sort.Strings(ts)
			out[f.Key] = ts
		}
	}
	return out, true
}

// cleanTexts keeps only known languages / fields with non-empty values.
func cleanTexts(def TypeDef, in Texts) Texts {
	out := Texts{}
	for _, l := range Langs {
		for _, f := range def.Texts {
			if v := in[l][f.Key]; v != "" {
				if out[l] == nil {
					out[l] = map[string]string{}
				}
				out[l][f.Key] = v
			}
		}
	}
	return out
}

// missingRequired lists "lang.field" / "attrs.key" that must be filled before submit.
func missingRequired(def TypeDef, attrs map[string]any, texts Texts) []string {
	var miss []string
	for _, f := range def.Attrs {
		if f.Required && attrs[f.Key] == nil {
			miss = append(miss, "attrs."+f.Key)
		}
	}
	for _, l := range []string{"th", "en"} {
		for _, f := range def.Texts {
			if f.Required && texts[l][f.Key] == "" {
				miss = append(miss, l+"."+f.Key)
			}
		}
	}
	return miss
}

func normLang(l string) string {
	for _, x := range Langs {
		if x == l {
			return l
		}
	}
	return "en"
}

// Resolve picks each field in lang, falling back to en, then th.
func Resolve(def TypeDef, texts Texts, lang string) map[string]string {
	lang = normLang(lang)
	out := map[string]string{}
	for _, f := range def.Texts {
		v := texts[lang][f.Key]
		if v == "" {
			v = texts["en"][f.Key]
		}
		if v == "" {
			v = texts["th"][f.Key]
		}
		out[f.Key] = v
	}
	return out
}

func orderOf(attrs map[string]any) int {
	switch v := attrs["order"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 1 << 30
}
