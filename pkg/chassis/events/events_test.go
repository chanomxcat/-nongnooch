package events_test

import (
	"testing"

	"nongnooch/platform/pkg/chassis/events"
)

func TestSubject(t *testing.T) {
	if got := events.Subject("order.paid"); got != "evt.order.paid" {
		t.Fatal(got)
	}
}

func TestNewIDsAreOrdered(t *testing.T) {
	a, err := events.New(events.OrderPaid, map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := events.New(events.OrderPaid, nil)
	if !(b.ID > a.ID) {
		t.Fatalf("ids not ordered: %s %s", a.ID, b.ID)
	}
	if a.OccurredAt.Location().String() != "UTC" {
		t.Fatal("not UTC")
	}
}

func TestCatalog(t *testing.T) {
	if len(events.All) != 20 {
		t.Fatalf("len=%d", len(events.All))
	}
	seen := map[string]bool{}
	for _, e := range events.All {
		if seen[e] {
			t.Fatalf("dup %s", e)
		}
		seen[e] = true
	}
	for _, want := range []string{"account.deleted", "member.invite_added", "member.consent_changed"} {
		if !seen[want] {
			t.Fatalf("missing %s", want)
		}
	}
}
