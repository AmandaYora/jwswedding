package domain

import (
	"errors"
	"time"
)

type SubscriptionStatus string

const (
	StatusActive        SubscriptionStatus = "active"
	StatusExpiringSoon   SubscriptionStatus = "expiring_soon"
	StatusExpired        SubscriptionStatus = "expired"
	StatusPendingPayment SubscriptionStatus = "pending_payment"
)

// PendingCharge remembers, for a subscription charge still awaiting ElProof's
// confirmation, which tenant+plan it was for, plus a snapshot of that plan's
// name/price/duration at the moment the charge was created (D8 — the
// activation path, webhook or reconciler, never calls ElProof's plan catalog
// again) and an atomic claim marker (D11 — ResolvedAt) so a webhook and the
// reconciler racing on the same OrderRef can never both activate it.
type PendingCharge struct {
	OrderRef           string
	TenantID           int64
	PlanID             int64
	PlanName           string
	PlanPrice          int64
	PlanDurationMonths int
	CreatedAt          time.Time
	ResolvedAt         *time.Time
}

type Tenant struct {
	ID                    int64
	BusinessName          string
	OwnerName             string
	Username              string
	Email                 string
	Phone                 string
	City                  string
	JoinedAt              time.Time
	PlanID                *int64
	SubscriptionStatus    SubscriptionStatus
	SubscriptionExpiresAt *time.Time
	IsSuspended           bool
	LastCredentialResetAt *time.Time
	BrandColorPreset      string
	LogoStoragePath       *string
	CustomDomain          *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// ErrDuplicateCustomDomain signals a unique-key collision on custom_domain —
// the repository returns this instead of a raw driver error so the
// application layer can translate it into a field-level validation error
// (see ADR-0015) without knowing MySQL error codes itself.
var ErrDuplicateCustomDomain = errors.New("domain kustom sudah digunakan tenant lain")
