package domain

import (
	"errors"
	"strings"
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
	// Address/BankName/BankAccountNumber/BankAccountHolderName back the
	// Invoice/Kwitansi PDF kop surat and bank-transfer info (PLAN.md
	// invoice-kwitansi-client, §1.7) -- editable only via the self-service
	// "Profil Usaha" page (PATCH /tenants/me, Owner-only), since Platform
	// Console's admin tenant CRUD no longer exists.
	Address                 string
	BankName                string
	BankAccountNumber       string
	BankAccountHolderName   string
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

// ProfileMissingFields is the single source of truth for "is this tenant's
// business profile complete enough to print an Invoice/Kwitansi PDF" (PLAN.md
// redesain-pdf-invoice-kwitansi-v2 §6.1) — every consumer (GET /tenants/me,
// GET /tenants/me/branding, and the PDF-download gate in the `projects`
// module, reached only via platform/contracts) calls this same function
// rather than re-implementing the rule. Email and logo are deliberately
// excluded (K1) — only identity, contact, and bank-transfer
// destination are required. Returns a non-nil empty slice, never nil, when
// complete, and preserves this fixed field order regardless of which fields
// are missing, so the dialog always lists them the same way.
func ProfileMissingFields(t Tenant) []string {
	missing := make([]string, 0, 8)
	type check struct {
		value string
		label string
	}
	for _, c := range []check{
		{t.BusinessName, "Nama Usaha"},
		{t.OwnerName, "Nama Pemilik"},
		{t.Phone, "Telepon"},
		{t.Address, "Alamat"},
		{t.City, "Kota"},
		{t.BankName, "Nama Bank"},
		{t.BankAccountNumber, "No. Rekening"},
		{t.BankAccountHolderName, "Nama Pemilik Rekening"},
	} {
		if strings.TrimSpace(c.value) == "" {
			missing = append(missing, c.label)
		}
	}
	return missing
}
