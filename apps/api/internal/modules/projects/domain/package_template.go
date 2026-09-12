package domain

import "time"

// PackageTemplate is a tenant's reusable sales package (PLAN.md
// po-paket-client D20/T8) — the master a project's own composition is copied
// FROM at ApplyTemplate time, never referenced again afterward. Exactly the
// lifecycle ProjectMilestoneTemplate already has.
type PackageTemplate struct {
	ID       int64
	TenantID int64
	Name     string
	// BasePrice is the list price. It loses to a project's own already-set
	// ContractValue when the template is applied — see PackageOrder.BasePrice
	// (D23): the agreed number is a negotiation result, this is only the
	// starting point shown in the picker.
	BasePrice int64
	// DefaultTerms/DefaultBonusNote live here rather than on `tenants` (D16)
	// so this whole feature never reaches into the `platform` module, and so
	// terms may differ per package tier.
	DefaultTerms     string
	DefaultBonusNote string
	IsActive         bool
	SortOrder        int
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// Blocks/Terms are loaded on demand (Get), not by List — a picker only
	// needs the header row. Nil means "not loaded", not "empty".
	Blocks []PackageTemplateBlock
	Terms  []PackageTemplateTerm
}

// PackageTemplateBlock is ONE ROW of the PDF's composition table, not one
// item (D20). The source document's QTY and BONUS cells attach to the table
// row — CATERING occupies two rows carrying different bonuses — so holding
// them per item would leave "which item's bonus is the block's?" undefined.
type PackageTemplateBlock struct {
	ID         int64
	TemplateID int64
	// Category is the merged first column. Plain text, deliberately not an FK
	// into vendors' vendor_categories (D4).
	Category string
	// Body is the item list, one per line. A line in ALL CAPS renders bold as
	// a sub-heading (D21) — the source document already writes BUFFET/DESSERT/
	// MINUMAN that way, so there is no markup syntax for the user to learn.
	Body string
	// QtyText is free text and may span several lines: the STALL/GUBUKAN block
	// carries four "150 PORSI" lines, and DEKORASI carries the range
	// "10-12 METER". Never summed, never parsed (D3).
	QtyText   string
	BonusNote string
	SortOrder int
}

// PackageTemplateTerm is one step of the payment schedule preset (D11).
// Copied into PackageOrder.TermsPlan at ApplyTemplate time (D22) — Issue
// never reads this table, so editing a template later can never alter the
// schedule of a contract already signed.
type PackageTemplateTerm struct {
	ID         int64
	TemplateID int64
	Sequence   int
	Label      string
	// Type reuses PaymentType so a seeded ClientInvoice takes it verbatim.
	// Only DP/Termin/Pelunasan are valid here — Tambahan is born from a
	// revision (D24), never from a preset, and Refund is not a bill at all.
	Type PaymentType
	// Exactly one of Percent/FixedAmount is set. Percent drives the 30%/50%
	// steps; FixedAmount drives a flat DP.
	Percent     *float64
	FixedAmount *int64
	// DaysBeforeEvent is an offset against Project.EventDate, positive =
	// before hari-H — same convention as ProjectMilestoneTemplate.
	DaysBeforeEvent int
}
