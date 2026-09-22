// Package domain holds all of projects' entities. Kept as one package with a
// handful of files (grouped by aggregate) rather than one-file-per-type,
// since this module owns 8 tightly related tables — see MODULE_MAP.md.
package domain

import "time"

type ProjectStatus string

const (
	StatusDraft       ProjectStatus = "Draft"
	StatusPreparation ProjectStatus = "Preparation"
	StatusReady       ProjectStatus = "Ready"
	StatusCompleted   ProjectStatus = "Completed"
	StatusCancelled   ProjectStatus = "Cancelled"
)

type Project struct {
	ID       int64
	TenantID int64
	// ClientID adalah relasi lintas modul ke master Client (modul clients) --
	// ID primitif TANPA foreign key (database.md). Satu-satunya tempat relasi
	// client<->project hidup sejak Fase 1 penawaran-client-master.
	ClientID int64
	// QuotationID menunjuk penawaran yang melahirkan project ini (modul
	// quotations) -- ID primitif TANPA foreign key, UNIQUE (1 penawaran
	// Diterima = 1 project, sekaligus membuat Accept idempoten). 0/NULL untuk
	// project lama yang lahir sebelum fase penawaran (D11 hanya untuk baru).
	QuotationID int64
	Name        string
	BrideName   string
	GroomName   string
	EventDate   time.Time
	// EventStartTime/EventEndTime are the project-level Jam Acara (PLAN.md
	// revisi-putri-mom-25082026, Blok A / item 11), stored as "HH:MM" strings.
	// Both optional: nil means "Belum ditentukan". Independent of the per-vendor
	// project_vendors.event_start_time/end_time (D4) -- this is the ceremony's
	// own time slot (e.g. Sesi Pagi 08:00-13:00). Part of "konteks umum", so
	// guarded by guardKonteksUmum (D3).
	EventStartTime *string
	EventEndTime   *string
	// Pax is the guest count shown on the PO Paket's header box (PLAN.md
	// po-paket-client, blok B1). 0 means "Belum ditentukan" -- the same
	// sentinel convention as PICSalesStaffID below, not a real headcount of
	// zero. Part of "konteks umum", so guarded by guardKonteksUmum like the
	// event-hour fields above.
	Pax   int
	Venue string
	// VenueID is a cross-module primitive reference into vendors' Venue
	// directory (ADR-0016) -- resolved via a module contract, never a SQL
	// foreign key. nil means no structured venue is attached yet; Venue
	// (the free-text field above) stays as the fallback display in that case.
	VenueID *int64
	// VenueRentalPrice/VenueCharge are a per-project SNAPSHOT of the
	// attached venue's cost, captured (and freely editable) at attach time
	// -- never a live join against venues' own current price. This mirrors
	// project_vendors.contract_value's exact lifecycle and exists so a
	// completed project's recorded Margin can never drift just because
	// venue master data changed later. Both nil whenever VenueID is nil;
	// ProjectService.Update force-clears both on detach regardless of what
	// the request body contains -- see PLAN.md "Financial Calculation
	// Correctness".
	VenueRentalPrice *int64
	VenueCharge      *int64
	PrepStartDate    time.Time
	PackageName      string
	ContractValue    int64
	Status           ProjectStatus
	PICStaffID       int64
	// PICSalesStaffID is the second, independent PIC slot ("PIC Sales") --
	// sentinel 0 means "belum ditugaskan", same convention as PICStaffID
	// itself. Set automatically to the creating Sales staff member on
	// create, immutable afterward for that role; Owner/Admin may set or
	// clear it freely. Never cleared by handover (assigning PICStaffID) --
	// see PLAN.md revisi-timeline-vendor-role-sales.
	PICSalesStaffID int64
	Description     string
	IsArchived      bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ProjectRef is a minimal {id, name} projection of a project — backs the
// hard-delete "impact" endpoints (PLAN.md's Vendor/Venue/Staff hard delete):
// naming exactly which projects would lose a data source, without exposing
// the rest of the project record cross-module.
type ProjectRef struct {
	ID   int64
	Name string
}

// QuotationProjectRef is the project born from an accepted quotation plus its
// Wedding Planner PIC — one row of the batch map backing the quotation list's
// WP filter and WP-name display (PLAN wording-role-dan-filter-sales-wp §5.3).
type QuotationProjectRef struct {
	ProjectID  int64
	PICStaffID int64 // 0 = belum ditugaskan
}

// ClientPICs is the set of distinct PICs across all projects of one client —
// one row of the batch map backing the client list's WP/Sales name display
// (PLAN wording-role-dan-filter-sales-wp §5.3).
type ClientPICs struct {
	PICStaffIDs      []int64
	PICSalesStaffIDs []int64
}

type MilestoneStatus string

const (
	MilestoneNotStarted MilestoneStatus = "Not Started"
	MilestoneInProgress MilestoneStatus = "In Progress"
	MilestoneCompleted  MilestoneStatus = "Completed"
	MilestoneBlocked    MilestoneStatus = "Blocked"
	MilestoneCancelled  MilestoneStatus = "Cancelled"
)

type ProjectMilestone struct {
	ID        int64
	ProjectID int64
	SortOrder int
	Name      string
	// Category is a display-grouping label for the timeline (PLAN.md
	// revisi-putri-mom-25082026, Blok B / item 14). "" means "Tanpa Kategori".
	// A plain VARCHAR, not a master table (D1) and not reused from
	// vendor_categories (D2).
	Category      string
	Status        MilestoneStatus
	TargetDate    time.Time
	CompletedDate *time.Time
}

// ProjectCostBasis adalah bagian ringkasan biaya yang TIDAK berubah karena
// tulisan engagement — dipisah supaya gerbang anggaran bisa membacanya sekali
// lalu memproyeksikan sendiri, tanpa membuat repository membaca ulang daftar
// engagement yang sudah ada di tangannya.
//
// Lihat docs/plan/vendor-engagement-500/PLAN.md T6: sebelum ini, satu
// penambahan vendor membayar tiga query (daftar engagement, project, lalu
// daftar engagement LAGI dari dalam CostSummary).
type ProjectCostBasis struct {
	ContractValue int64
	// VenueCost dibaca dari snapshot milik project (venue_rental_price +
	// venue_charge), bukan dari tabel venue milik modul `vendors`.
	VenueCost int64
}

// ProjectCostSummary adalah sisi biaya sebuah project berhadapan dengan nilai
// kontraknya — dasar "Sisa Anggaran" dan gerbang komitmen vendor.
//
// Dihitung di SERVER, bukan lagi sebagai ekspresi di komponen header: sebuah
// aturan bisnis yang hanya hidup di frontend bukan aturan, hanya saran yang
// bisa dilewati siapa pun yang memanggil API langsung.
//
// Remaining SENGAJA boleh negatif. Biaya yang sudah disepakati adalah fakta;
// menolak merepresentasikannya tidak menghilangkan kerugiannya, hanya
// memindahkannya ke luar sistem.
type ProjectCostSummary struct {
	ContractValue int64
	// VendorCost menjumlahkan contract_value setiap engagement yang TIDAK
	// Cancelled — termasuk yang masih Planned/Negotiation. Yang belum pasti
	// pun tetap dihitung di sini karena inilah eksposur terburuknya; UI yang
	// membedakan "rencana" dari "komitmen", bukan angka ini.
	VendorCost int64
	// VenueCost dibaca dari snapshot milik project (venue_rental_price +
	// venue_charge), bukan dari master venue — supaya Margin project yang
	// sudah selesai tidak bergeser saat harga venue berubah kemudian.
	VenueCost int64
	// CommittedCost = VendorCost + VenueCost.
	CommittedCost int64
	// Remaining = ContractValue - CommittedCost. Negatif berarti biaya sudah
	// melampaui yang disepakati klien.
	Remaining int64
}
