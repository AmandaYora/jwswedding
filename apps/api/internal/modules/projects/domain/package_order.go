package domain

import "time"

type PackageOrderStatus string

const (
	PackageOrderDraft     PackageOrderStatus = "Draft"
	PackageOrderIssued    PackageOrderStatus = "Terbit"
	PackageOrderCancelled PackageOrderStatus = "Dibatalkan"
)

// PackageOrder is the PO Paket — the contract document a project is sold
// under (PLAN.md po-paket-client D5), exactly one per project. Deliberately
// NOT a fattened ClientInvoice: an invoice is one bill of one amount with a
// due date whose MarkPaid creates a ClientPayment, while this is the whole
// agreement, issued once at deal close and reprinted as payments come in.
//
// Its row is born in Draft the moment a template is applied, long before it
// is ever issued — which is why every numbering column is a sentinel until
// Issue (D26).
type PackageOrder struct {
	ID        int64
	ProjectID int64
	// PONumber is "" until the first Issue, then permanent — Revise bumps
	// Revision and never renumbers, so a client is never handed two
	// differently-numbered documents for the same contract (D26). Empty-string
	// sentinel is the same convention ClientPayment.ReceiptNumber already uses
	// for a receipt never printed.
	PONumber     string
	NumberPeriod string
	NumberSeq    int
	Revision     int
	// BasePrice is "HARGA PAKET AWAL". When a template is applied to a project
	// that already has a ContractValue > 0, that value wins over the template's
	// own BasePrice (D23) — this is what lets an existing project adopt the
	// feature without having its contract value wiped.
	BasePrice int64
	TermsText string
	BonusNote string
	// TermsPlan is the payment schedule, copied from the template at
	// ApplyTemplate time (D22). Issue seeds Draft ClientInvoices from THIS,
	// never from package_template_terms.
	TermsPlan []TermPlanEntry
	Status    PackageOrderStatus
	// Snapshot freezes composition + adjustments + event identity at Issue
	// (D6). Nil while Draft — a Draft PDF renders from the live tables, which
	// is what makes it safe to print repeatedly during negotiation.
	Snapshot         *PackageOrderSnapshot
	IssuedAt         *time.Time
	CreatedByStaffID int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsNumbered reports whether this PO has already been through its first
// Issue. Guards the one call to NextPOSequence (D26) — see §9 R6.
func (o *PackageOrder) IsNumbered() bool { return o.PONumber != "" }

// TermPlanEntry is one step of the schedule frozen onto the PO. Mirrors
// PackageTemplateTerm minus its identity columns.
type TermPlanEntry struct {
	Sequence        int         `json:"sequence"`
	Label           string      `json:"label"`
	Type            PaymentType `json:"type"`
	Percent         *float64    `json:"percent,omitempty"`
	FixedAmount     *int64      `json:"fixedAmount,omitempty"`
	DaysBeforeEvent int         `json:"daysBeforeEvent"`
}

// PackageOrderSnapshot is the frozen document (D30). Stored as an object with
// an explicit `current`, never a bare array, so "which revision is the one we
// print?" is never a question.
type PackageOrderSnapshot struct {
	Current PackageOrderRevision   `json:"current"`
	History []PackageOrderRevision `json:"history,omitempty"`
}

// PackageOrderRevision is one frozen state of the agreement. Event identity
// is captured alongside the composition because a signed contract must keep
// showing the venue, pax and times as they stood at signing, even if the
// project record moves afterwards.
type PackageOrderRevision struct {
	Revision    int                        `json:"revision"`
	IssuedAt    time.Time                  `json:"issuedAt"`
	BasePrice   int64                      `json:"basePrice"`
	TermsText   string                     `json:"termsText"`
	BonusNote   string                     `json:"bonusNote"`
	Blocks      []ProjectPackageBlock      `json:"blocks"`
	Adjustments []ProjectPackageAdjustment `json:"adjustments"`
	TermsPlan   []TermPlanEntry            `json:"termsPlan"`
	Event       PackageOrderEventSnapshot  `json:"event"`
}

// PackageOrderEventSnapshot is the B1 header box as it stood at issue time.
type PackageOrderEventSnapshot struct {
	ClientName string `json:"clientName"`
	Phone      string `json:"phone"`
	EventDate  string `json:"eventDate"`
	EventStart string `json:"eventStart"`
	EventEnd   string `json:"eventEnd"`
	Venue      string `json:"venue"`
	Pax        int    `json:"pax"`
}

// ProjectPackageBlock is a project's own copy of a template block. It keeps no
// template_id at all (D6/D22): once copied, the agreement is independent of
// the master it came from.
type ProjectPackageBlock struct {
	ID        int64  `json:"-"`
	ProjectID int64  `json:"-"`
	Category  string `json:"category"`
	Body      string `json:"body"`
	QtyText   string `json:"qtyText"`
	BonusNote string `json:"bonusNote"`
	SortOrder int    `json:"sortOrder"`
}

// ProjectPackageAdjustment is one ADDITIONAL/TAKEOUT line — the only priced
// part of the composition (D1).
type ProjectPackageAdjustment struct {
	ID          int64  `json:"-"`
	ProjectID   int64  `json:"-"`
	Description string `json:"description"`
	// Amount is SIGNED: negative is a takeout/cashback (D2). One signed column
	// instead of a separate kind flag means the total is a plain SUM and sign
	// can never contradict category.
	Amount    int64 `json:"amount"`
	SortOrder int   `json:"sortOrder"`
}

// TotalAdjustments sums the signed adjustments. Callers add this to BasePrice
// to get the project's ContractValue (D15).
func TotalAdjustments(adjustments []ProjectPackageAdjustment) int64 {
	var total int64
	for _, a := range adjustments {
		total += a.Amount
	}
	return total
}
