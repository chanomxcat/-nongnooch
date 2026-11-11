// Package events defines the event envelope and the event catalog (contracts section 5).
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	OTPRequested         = "otp.requested"
	MemberRegistered     = "member.registered"
	ReferralCreated      = "referral.created"
	AgentApproved        = "agent.approved"
	OrderPaid            = "order.paid"
	OrderRefunded        = "order.refunded"
	TicketIssued         = "ticket.issued"
	TicketScanned        = "ticket.scanned"
	PriceApproved        = "price.approved"
	ContentPublished     = "content.published"
	PointsChanged        = "points.changed"
	MissionCompleted     = "mission.completed"
	OrderExpired         = "order.expired"
	PaymentDiscrepancy   = "payment.discrepancy"
	BillPaid             = "bill.paid"
	PartnerOrderUpdated  = "partner.order.updated"
	TicketTransferred    = "ticket.transferred"
	AccountDeleted       = "account.deleted"
	MemberInviteAdded    = "member.invite_added"
	MemberConsentChanged = "member.consent_changed"
)

const (
	Stream    = "EVENTS"
	DLQPrefix = "dlq."
)

// All lists every event type in the catalog.
var All = []string{
	OTPRequested, MemberRegistered, ReferralCreated, AgentApproved, OrderPaid,
	OrderRefunded, TicketIssued, TicketScanned, PriceApproved, ContentPublished,
	PointsChanged, MissionCompleted, OrderExpired, PaymentDiscrepancy, BillPaid,
	PartnerOrderUpdated, TicketTransferred, AccountDeleted, MemberInviteAdded,
	MemberConsentChanged,
}

// Envelope wraps every event on the bus.
type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload"`
}

// Subject returns the NATS subject for an event type.
func Subject(eventType string) string { return "evt." + eventType }

// New builds an Envelope with a UUIDv7 id and UTC timestamp.
func New(eventType string, payload any) (Envelope, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Envelope{}, err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{ID: id.String(), Type: eventType, OccurredAt: time.Now().UTC(), Payload: b}, nil
}
