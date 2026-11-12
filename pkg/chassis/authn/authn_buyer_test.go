package authn

import "testing"

func TestBuyerID(t *testing.T) {
	if got := (Principal{Type: "user", Sub: "+66899990001", MemberID: "m1"}).BuyerID(); got != "m1" {
		t.Fatalf("member: %q", got)
	}
	g := (Principal{Type: "user", Sub: "+66899990001"}).BuyerID()
	if len(g) != 36 || !IsGuestBuyer(g) || g != (Principal{Type: "user", Sub: "+66899990001"}).BuyerID() {
		t.Fatalf("guest: %q", g)
	}
	if (Principal{Type: "staff", Sub: "s1"}).BuyerID() != "" || (Principal{}).BuyerID() != "" {
		t.Fatal("staff/anonymous must not buy")
	}
	if IsGuestBuyer("0190a1b2-0000-7000-8000-000000000000") {
		t.Fatal("uuid is not a guest")
	}
}
