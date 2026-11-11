package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

var seedNS = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

const dailyQuota = 5000

type seedProduct struct {
	key, kind, age string
	thai, foreign  int64
	names          map[string]string
}

// Demo prices in satang (not real prices).
var seedProducts = []seedProduct{
	{"day-adult", "day", "adult", 30000, 60000, map[string]string{"th": "บัตรรายวัน ผู้ใหญ่", "en": "Day ticket (adult)", "zh": "日票（成人）"}},
	{"day-child", "day", "child", 15000, 30000, map[string]string{"th": "บัตรรายวัน เด็ก", "en": "Day ticket (child)", "zh": "日票（儿童）"}},
	{"annual", "annual", "adult", 150000, 300000, map[string]string{"th": "บัตรรายปี", "en": "Annual pass", "zh": "年票"}},
}

// Seed upserts the demo products and gives the next 60 days a 5,000 quota. Idempotent.
func (s *Service) Seed(ctx context.Context) error {
	for _, p := range seedProducts {
		names, _ := json.Marshal(p.names)
		id := uuid.NewSHA1(seedNS, []byte("product:"+p.key)).String()
		if _, err := s.DB.ExecContext(ctx,
			`INSERT INTO products(id, kind, age_group, names, thai_satang, foreign_satang) VALUES(?,?,?,?,?,?)
			 ON DUPLICATE KEY UPDATE kind=VALUES(kind), age_group=VALUES(age_group), names=VALUES(names), thai_satang=VALUES(thai_satang), foreign_satang=VALUES(foreign_satang)`,
			id, p.kind, p.age, string(names), p.thai, p.foreign); err != nil {
			return err
		}
	}
	today := s.now().In(ict)
	for i := 0; i < 60; i++ {
		d := time.Date(today.Year(), today.Month(), today.Day()+i, 0, 0, 0, 0, ict).Format(dateLayout)
		if _, err := s.DB.ExecContext(ctx, `INSERT INTO quota(visit_date, total) VALUES(?,?) ON DUPLICATE KEY UPDATE visit_date=visit_date`, d, dailyQuota); err != nil {
			return err
		}
	}
	return nil
}

type productView struct {
	ProductID     string `json:"productId"`
	Kind          string `json:"kind"`
	AgeGroup      string `json:"ageGroup"`
	Name          string `json:"name"`
	ThaiSatang    *int64 `json:"thaiSatang,omitempty"`
	ForeignSatang *int64 `json:"foreignSatang,omitempty"`
}

// Products lists the catalog with names in lang (falling back to en).
func (s *Service) Products(ctx context.Context, lang string) ([]productView, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, kind, age_group, names, thai_satang, foreign_satang FROM products ORDER BY kind='annual', age_group`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []productView{}
	for rows.Next() {
		var p productView
		var names string
		var th, fo sql.NullInt64
		if err := rows.Scan(&p.ProductID, &p.Kind, &p.AgeGroup, &names, &th, &fo); err != nil {
			return nil, err
		}
		m := map[string]string{}
		_ = json.Unmarshal([]byte(names), &m)
		p.Name = m[lang]
		if p.Name == "" {
			p.Name = m["en"]
		}
		if th.Valid {
			p.ThaiSatang = &th.Int64
		}
		if fo.Valid {
			p.ForeignSatang = &fo.Int64
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type avail struct {
	VisitDate string `json:"visitDate"`
	Remaining int    `json:"remaining"`
}

// Availability returns remaining day-ticket quota per date in [from, to].
func (s *Service) Availability(ctx context.Context, from, to string) ([]avail, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DATE_FORMAT(visit_date,'%Y-%m-%d'), total-held-sold FROM quota WHERE visit_date BETWEEN ? AND ? ORDER BY visit_date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []avail{}
	for rows.Next() {
		var a avail
		if err := rows.Scan(&a.VisitDate, &a.Remaining); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
