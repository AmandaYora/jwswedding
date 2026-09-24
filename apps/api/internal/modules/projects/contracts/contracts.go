// Package contracts is the ONLY package other modules may import from
// projects — used by `clients` (validate a project_id belongs to the
// caller's tenant) and `vendors` (resolve a vendor's cross-project
// engagement history for "Lihat Project", since `project_vendors` is owned
// by `projects`, not `vendors`).
package contracts

import (
	"context"
	"time"

	"jwswedding/internal/modules/projects/application"
)

// VendorEngagementHistoryItem is the cross-module-safe shape of one history
// row — a primitive projection of domain.VendorEngagementHistoryRow, never
// the domain type itself.
type VendorEngagementHistoryItem struct {
	ProjectID        int64
	ProjectName      string
	EventDate        time.Time
	Venue            string
	EngagementStatus string
}

// ProjectRef is a minimal {id, name} projection of a project — a
// cross-module-safe copy of domain.ProjectRef, same convention as
// VendorEngagementHistoryItem mirroring VendorEngagementHistoryRow above.
type ProjectRef struct {
	ID   int64
	Name string
}

// CreateFromQuotationInput is everything projects needs to birth a project
// from an accepted quotation in ONE transaction (PLAN penawaran-client-master
// §7). Strings are denormalized at the edge by the caller (quotations
// resolves couple names via clients, venue details resolve inside via the
// existing VenueResolver) so this module never queries another module's
// tables mid-transaction.
type CreateFromQuotationInput struct {
	TenantID      int64
	QuotationID   int64
	ClientID      int64
	ProjectName   string
	BrideName     string
	GroomName     string
	EventDate     time.Time
	PrepStartDate time.Time
	Pax           int
	VenueID       *int64
	// VenueRentalPrice/Charge adalah snapshot biaya venue yang ditulis WO
	// (dibaca dari master venue di sisi pemanggil, pola yang sama dengan alur
	// pasang-venue di Update) — nil berarti tanpa snapshot.
	VenueRentalPrice *int64
	VenueCharge      *int64
	PackageName      string
	ContractValue    int64
	PICStaffID       int64
	PICSalesStaffID  int64
	Description      string
	ActorStaffID     int64
}

// QuotationProjectRef is the project born from an accepted quotation plus
// its Wedding Planner PIC — one row of the batch map backing the quotation
// list's WP filter and WP-name display (PLAN
// wording-role-dan-filter-sales-wp §5.3).
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
// ProjectImpact backs the hard-delete "impact" dialog for a Client
// (D14 — informed consent, never a block): how many projects would go with
// it, and — the number that hurts most if deleted by mistake — how many of
// their invoices are already paid and for how much.
type ProjectImpact struct {
	ProjectCount     int
	ProjectNames     []string
	PaidInvoiceCount int
	PaidInvoiceTotal int64
}

// ClientPaymentInfo is the minimal live-ledger row the quotation PDF needs
// from projects (the "PEMBAYARAN DITERIMA" block stays live even on a frozen
// document). Primitives only — quotations may not import projects' domain.
type ClientPaymentInfo struct {
	Type        string
	Amount      int64
	PaymentDate time.Time
	Method      string
}

// RundownProjectContext adalah proyeksi primitif sebuah project untuk mengisi
// awal buku acara di modul `rundowns` — hanya kolom milik `projects` sendiri.
// Nama vendor/kategori sengaja TIDAK di sini: keduanya milik modul `vendors`
// dan diselesaikan frontend dari store-nya sendiri (MODULE_MAP.md).
type RundownProjectContext struct {
	ProjectID      int64
	ProjectName    string
	BrideName      string
	GroomName      string
	EventDate      time.Time
	EventStartTime string
	EventEndTime   string
	Venue          string
	PackageName    string
	PICStaffID     int64
}

// GeneratedDocInput adalah berkas hasil generate yang disimpan ke tab Dokumen
// project. ReplaceEvidenceID 0 berarti tidak ada yang digantikan.
type GeneratedDocInput struct {
	Name              string
	FileName          string
	MimeType          string
	Data              []byte
	ReplaceEvidenceID int64
}

// GeneratedDocResult adalah hasil pengarsipan berkas generate. ClientVisible
// diwarisi dari versi sebelumnya, supaya pemanggil bisa memberi tahu WO apakah
// klien masih melihat dokumen itu.
type GeneratedDocResult struct {
	EvidenceID    int64
	ClientVisible bool
}

// ProjectDeleteImpact is one project's share of a delete confirmation (T3.5).
type ProjectDeleteImpact struct {
	ProjectID        int64
	ProjectName      string
	PaidInvoiceCount int
	PaidInvoiceTotal int64
}

type Contracts interface {
	ProjectExists(ctx context.Context, tenantID, projectID int64) (bool, error)
	// ProjectPICStaffID resolves a project's current PIC — used by `clients`
	// to scope a Wedding Planner's client-read access to only projects
	// they're PIC of (see PLAN.md's RBAC section).
	ProjectPICStaffID(ctx context.Context, tenantID, projectID int64) (int64, error)
	// ProjectPICSalesStaffID resolves a project's current PIC Sales — used by
	// `clients` to scope a Sales staff member's client read/write access to
	// only projects they're PIC Sales of (PLAN.md
	// revisi-timeline-vendor-role-sales).
	ProjectPICSalesStaffID(ctx context.Context, tenantID, projectID int64) (int64, error)
	ListVendorEngagementHistory(ctx context.Context, tenantID, vendorID int64) ([]VendorEngagementHistoryItem, error)
	// ListAffectedProjectsForVendor/Venue/Staff back the hard-delete "impact"
	// endpoints (PLAN.md's Vendor/Venue/Staff hard delete) -- every project
	// that would lose a data source if the given vendor/venue/staff row were
	// hard-deleted, named so the frontend's confirmation dialog can say
	// exactly what's affected instead of a generic warning.
	ListAffectedProjectsForVendor(ctx context.Context, tenantID, vendorID int64) ([]ProjectRef, error)
	ListAffectedProjectsForVenue(ctx context.Context, tenantID, venueID int64) ([]ProjectRef, error)
	ListAffectedProjectsForStaff(ctx context.Context, tenantID, staffID int64) ([]ProjectRef, error)
	// SeedDefaultMilestoneTemplate gives a newly registered tenant a starting
	// Timeline Default template (PLAN.md) — called by `platform`'s tenant
	// registration flow, the same moment `vendors.SeedDefaultCategories` is.
	SeedDefaultMilestoneTemplate(ctx context.Context, tenantID int64) error
	// --- Penawaran-client-master ports (PLAN §4/§6) ---
	// ProjectDirectory answers from projects.client_id — the only place the
	// client<->project relation lives. Consumed by `clients` (which must not
	// query the projects table itself).
	ClientHasProject(ctx context.Context, tenantID, clientID int64) (bool, error)
	ProjectIDsForClient(ctx context.Context, tenantID, clientID int64) ([]int64, error)
	// ProjectCountsForClients menjawab "client ini punya berapa project"
	// untuk SATU halaman daftar Client — satu query agregat, bukan N+1 (§11).
	ProjectCountsForClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]int, error)
	// ProjectCleaner backs Client's cascading hard delete (D14): impact first
	// (for the confirmation dialog), then the deletion, project by project.
	ImpactForClient(ctx context.Context, tenantID, clientID int64) (ProjectImpact, error)
	DeleteProjectsForClient(ctx context.Context, tenantID, clientID int64) error
	// ProjectCreator is the single door new projects are born through (D11):
	// one transaction (project + Timeline Default + invoices), idempotent via
	// projects.quotation_id UNIQUE. Consumed by `quotations` at Accept time.
	CreateFromQuotation(ctx context.Context, input CreateFromQuotationInput) (ProjectRef, error)
	// SyncFromQuotation pushes a quotation's derived values into its project:
	// the recomputed total (D15) and the package name. Both travel in ONE
	// call so a price revision and a name revision can never land as two
	// half-applied changes. No-op routing: a quotation with no project yet
	// has nothing to sync. An empty packageName leaves the project's own
	// value untouched — that is a pre-000065 quotation, not a rename to "".
	SyncFromQuotation(ctx context.Context, tenantID, projectID, total int64, packageName string) error
	// ProjectIDForQuotation resolves the project born from a quotation —
	// 0, nil when the quotation has no project yet.
	ProjectIDForQuotation(ctx context.Context, tenantID, quotationID int64) (int64, error)
	// ClientPaymentLedger feeds the quotation PDF's live payment block.
	ClientPaymentLedger(ctx context.Context, tenantID, projectID int64) ([]ClientPaymentInfo, int64, error)
	// DeleteProjectCascade force-removes one project with everything in it
	// (plus its quotation) — the engine behind both the Owner's manual
	// project delete and the quotation/client cascades (D14). No
	// archived/cancelled precondition: the guardrail is the delete-impact
	// dialog, not a block.
	DeleteProjectCascade(ctx context.Context, tenantID, projectID int64) error
	// PONumberForQuotation backs the project delete-impact dialog (T3.5):
	// the PO number that would go with the project. "" when none.
	PONumberForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error)
	// ProjectDeleteImpact backs the quotation delete-impact dialog for an
	// accepted quotation (T3.5): the project that would go with it, and the
	// paid-invoice figure that hurts most if deleted by mistake.
	ProjectDeleteImpact(ctx context.Context, tenantID, projectID int64) (ProjectDeleteImpact, error)
	// ProjectRefsForQuotations maps accepted quotations to their projects
	// plus their WP PICs for ONE page of the quotation list — one query, not
	// N+1 (PLAN wording-role-dan-filter-sales-wp §5.3).
	ProjectRefsForQuotations(ctx context.Context, tenantID int64, quotationIDs []int64) (map[int64]QuotationProjectRef, error)
	// QuotationIDsForPICStaff answers which quotations already birthed a
	// project held by one Wedding Planner — the WP filter on the quotation
	// list.
	QuotationIDsForPICStaff(ctx context.Context, tenantID, picStaffID int64) ([]int64, error)
	// ClientIDsForPIC answers which clients own a project held by the given
	// PIC(s) — the Sales/WP filter on the client list. 0 = no filter for
	// that slot.
	ClientIDsForPIC(ctx context.Context, tenantID, picStaffID, picSalesStaffID int64) ([]int64, error)
	// PICsForClients answers the distinct PIC sets across all projects of
	// each client on ONE client-list page — one query, not N+1.
	PICsForClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]ClientPICs, error)
	// --- dipakai modul `rundowns` (PLAN rundown-generator §8.1) ---
	// RundownProjectContext memasok data prefill buku acara sekaligus menjadi
	// gerbang keberadaan project: NotFound bila project tidak ada di tenant ini.
	RundownProjectContext(ctx context.Context, tenantID, projectID int64) (RundownProjectContext, error)
	// ProjectIDsForPICStaff menyaring daftar Rundown untuk Wedding Planner.
	ProjectIDsForPICStaff(ctx context.Context, tenantID, picStaffID int64) ([]int64, error)
	// SaveGeneratedDocument menaruh berkas hasil generate ke tab Dokumen
	// project (evidence kind `general`), menggantikan hasil sebelumnya untuk
	// format yang sama, dengan mewarisi visibilitas kliennya.
	SaveGeneratedDocument(ctx context.Context, tenantID, projectID, actorStaffID int64, in GeneratedDocInput) (GeneratedDocResult, error)
}

type impl struct {
	projects           *application.ProjectService
	vendorEngagements  *application.VendorEngagementService
	milestoneTemplates *application.MilestoneTemplateService
	clientPayments     *application.ClientPaymentService
}

func New(projects *application.ProjectService, vendorEngagements *application.VendorEngagementService, milestoneTemplates *application.MilestoneTemplateService, clientPayments *application.ClientPaymentService) Contracts {
	return &impl{projects: projects, vendorEngagements: vendorEngagements, milestoneTemplates: milestoneTemplates, clientPayments: clientPayments}
}

func (c *impl) ProjectExists(ctx context.Context, tenantID, projectID int64) (bool, error) {
	return c.projects.ExistsForTenant(ctx, tenantID, projectID)
}

func (c *impl) ProjectPICStaffID(ctx context.Context, tenantID, projectID int64) (int64, error) {
	p, err := c.projects.Get(ctx, tenantID, projectID)
	if err != nil {
		return 0, err
	}
	return p.PICStaffID, nil
}

func (c *impl) ProjectPICSalesStaffID(ctx context.Context, tenantID, projectID int64) (int64, error) {
	p, err := c.projects.Get(ctx, tenantID, projectID)
	if err != nil {
		return 0, err
	}
	return p.PICSalesStaffID, nil
}

func (c *impl) ListVendorEngagementHistory(ctx context.Context, tenantID, vendorID int64) ([]VendorEngagementHistoryItem, error) {
	rows, err := c.vendorEngagements.ListHistoryForVendor(ctx, tenantID, vendorID)
	if err != nil {
		return nil, err
	}
	items := make([]VendorEngagementHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, VendorEngagementHistoryItem{
			ProjectID: r.ProjectID, ProjectName: r.ProjectName, EventDate: r.EventDate,
			Venue: r.Venue, EngagementStatus: string(r.EngagementStatus),
		})
	}
	return items, nil
}

// ListAffectedProjectsForVendor reuses ListVendorEngagementHistory's own data
// (project_vendors joined to projects) -- the exact same set the existing
// "Lihat Project" feature already surfaces, just projected down to {id, name}
// and de-duplicated (a vendor could in principle have more than one
// project_vendors row against the same project across its history).
func (c *impl) ListAffectedProjectsForVendor(ctx context.Context, tenantID, vendorID int64) ([]ProjectRef, error) {
	rows, err := c.vendorEngagements.ListHistoryForVendor(ctx, tenantID, vendorID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(rows))
	var refs []ProjectRef
	for _, r := range rows {
		if seen[r.ProjectID] {
			continue
		}
		seen[r.ProjectID] = true
		refs = append(refs, ProjectRef{ID: r.ProjectID, Name: r.ProjectName})
	}
	return refs, nil
}

func (c *impl) ListAffectedProjectsForVenue(ctx context.Context, tenantID, venueID int64) ([]ProjectRef, error) {
	rows, err := c.projects.ListAffectedProjectsForVenue(ctx, tenantID, venueID)
	if err != nil {
		return nil, err
	}
	refs := make([]ProjectRef, 0, len(rows))
	for _, r := range rows {
		refs = append(refs, ProjectRef{ID: r.ID, Name: r.Name})
	}
	return refs, nil
}

func (c *impl) ListAffectedProjectsForStaff(ctx context.Context, tenantID, staffID int64) ([]ProjectRef, error) {
	rows, err := c.projects.ListAffectedProjectsForStaff(ctx, tenantID, staffID)
	if err != nil {
		return nil, err
	}
	refs := make([]ProjectRef, 0, len(rows))
	for _, r := range rows {
		refs = append(refs, ProjectRef{ID: r.ID, Name: r.Name})
	}
	return refs, nil
}

func (c *impl) SeedDefaultMilestoneTemplate(ctx context.Context, tenantID int64) error {
	return c.milestoneTemplates.SeedDefaults(ctx, tenantID)
}

func (c *impl) ClientHasProject(ctx context.Context, tenantID, clientID int64) (bool, error) {
	return c.projects.ClientHasProject(ctx, tenantID, clientID)
}

func (c *impl) ProjectIDsForClient(ctx context.Context, tenantID, clientID int64) ([]int64, error) {
	return c.projects.ProjectIDsForClient(ctx, tenantID, clientID)
}

func (c *impl) ProjectCountsForClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]int, error) {
	return c.projects.ProjectCountsForClients(ctx, tenantID, clientIDs)
}

func (c *impl) ImpactForClient(ctx context.Context, tenantID, clientID int64) (ProjectImpact, error) {
	impact, err := c.projects.ImpactForClient(ctx, tenantID, clientID)
	if err != nil {
		return ProjectImpact{}, err
	}
	return ProjectImpact{
		ProjectCount: impact.ProjectCount, ProjectNames: impact.ProjectNames,
		PaidInvoiceCount: impact.PaidInvoiceCount, PaidInvoiceTotal: impact.PaidInvoiceTotal,
	}, nil
}

func (c *impl) DeleteProjectsForClient(ctx context.Context, tenantID, clientID int64) error {
	return c.projects.DeleteProjectsForClient(ctx, tenantID, clientID)
}

func (c *impl) CreateFromQuotation(ctx context.Context, input CreateFromQuotationInput) (ProjectRef, error) {
	id, err := c.projects.CreateFromQuotation(ctx, application.CreateFromQuotationInput{
		TenantID: input.TenantID, QuotationID: input.QuotationID, ClientID: input.ClientID,
		ProjectName: input.ProjectName, BrideName: input.BrideName, GroomName: input.GroomName,
		EventDate: input.EventDate, PrepStartDate: input.PrepStartDate, Pax: input.Pax,
		VenueID: input.VenueID, VenueRentalPrice: input.VenueRentalPrice, VenueCharge: input.VenueCharge,
		PackageName: input.PackageName, ContractValue: input.ContractValue,
		PICStaffID: input.PICStaffID, PICSalesStaffID: input.PICSalesStaffID,
		Description: input.Description, ActorStaffID: input.ActorStaffID,
	})
	if err != nil {
		return ProjectRef{}, err
	}
	p, err := c.projects.Get(ctx, input.TenantID, id)
	if err != nil {
		return ProjectRef{}, err
	}
	return ProjectRef{ID: p.ID, Name: p.Name}, nil
}

func (c *impl) SyncFromQuotation(ctx context.Context, tenantID, projectID, total int64, packageName string) error {
	return c.projects.SyncFromQuotation(ctx, tenantID, projectID, total, packageName)
}

func (c *impl) ProjectIDForQuotation(ctx context.Context, tenantID, quotationID int64) (int64, error) {
	return c.projects.ProjectIDForQuotation(ctx, tenantID, quotationID)
}

func (c *impl) ClientPaymentLedger(ctx context.Context, tenantID, projectID int64) ([]ClientPaymentInfo, int64, error) {
	if _, err := c.projects.Get(ctx, tenantID, projectID); err != nil {
		return nil, 0, err
	}
	list, err := c.clientPayments.List(ctx, projectID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ClientPaymentInfo, 0, len(list))
	for _, p := range list {
		out = append(out, ClientPaymentInfo{
			Type: string(p.Type), Amount: p.Amount,
			PaymentDate: p.PaymentDate, Method: p.Method,
		})
	}
	total, err := c.clientPayments.TotalReceived(ctx, projectID)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (c *impl) DeleteProjectCascade(ctx context.Context, tenantID, projectID int64) error {
	return c.projects.DeleteProjectCascade(ctx, tenantID, projectID)
}

func (c *impl) PONumberForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	return c.projects.PONumberForQuotation(ctx, tenantID, quotationID)
}

func (c *impl) ProjectDeleteImpact(ctx context.Context, tenantID, projectID int64) (ProjectDeleteImpact, error) {
	impact, err := c.projects.ProjectDeleteImpact(ctx, tenantID, projectID)
	if err != nil {
		return ProjectDeleteImpact{}, err
	}
	return ProjectDeleteImpact{
		ProjectID: impact.ProjectID, ProjectName: impact.ProjectName,
		PaidInvoiceCount: impact.PaidInvoiceCount, PaidInvoiceTotal: impact.PaidInvoiceTotal,
	}, nil
}

func (c *impl) ProjectRefsForQuotations(ctx context.Context, tenantID int64, quotationIDs []int64) (map[int64]QuotationProjectRef, error) {
	refs, err := c.projects.ProjectRefsForQuotations(ctx, tenantID, quotationIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]QuotationProjectRef, len(refs))
	for qid, ref := range refs {
		out[qid] = QuotationProjectRef{ProjectID: ref.ProjectID, PICStaffID: ref.PICStaffID}
	}
	return out, nil
}

func (c *impl) QuotationIDsForPICStaff(ctx context.Context, tenantID, picStaffID int64) ([]int64, error) {
	return c.projects.QuotationIDsForPICStaff(ctx, tenantID, picStaffID)
}

func (c *impl) ClientIDsForPIC(ctx context.Context, tenantID, picStaffID, picSalesStaffID int64) ([]int64, error) {
	return c.projects.ClientIDsForPIC(ctx, tenantID, picStaffID, picSalesStaffID)
}

func (c *impl) PICsForClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]ClientPICs, error) {
	rows, err := c.projects.PICsForClients(ctx, tenantID, clientIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]ClientPICs, len(rows))
	for clientID, row := range rows {
		out[clientID] = ClientPICs{PICStaffIDs: row.PICStaffIDs, PICSalesStaffIDs: row.PICSalesStaffIDs}
	}
	return out, nil
}

// --- dipakai modul `rundowns` (PLAN rundown-generator §8.1) ---

func (c *impl) RundownProjectContext(ctx context.Context, tenantID, projectID int64) (RundownProjectContext, error) {
	s, err := c.projects.RundownProjectSnapshotFor(ctx, tenantID, projectID)
	if err != nil {
		return RundownProjectContext{}, err
	}
	return RundownProjectContext{
		ProjectID: s.ProjectID, ProjectName: s.ProjectName,
		BrideName: s.BrideName, GroomName: s.GroomName,
		EventDate: s.EventDate, EventStartTime: s.EventStartTime,
		EventEndTime: s.EventEndTime, Venue: s.Venue,
		PackageName: s.PackageName, PICStaffID: s.PICStaffID,
	}, nil
}

func (c *impl) ProjectIDsForPICStaff(ctx context.Context, tenantID, picStaffID int64) ([]int64, error) {
	return c.projects.ProjectIDsForPICStaff(ctx, tenantID, picStaffID)
}

func (c *impl) SaveGeneratedDocument(ctx context.Context, tenantID, projectID, actorStaffID int64, in GeneratedDocInput) (GeneratedDocResult, error) {
	id, visible, err := c.projects.SaveGeneratedRundownDocument(ctx, tenantID, projectID, actorStaffID,
		in.Name, in.FileName, in.MimeType, in.Data, in.ReplaceEvidenceID)
	if err != nil {
		return GeneratedDocResult{}, err
	}
	return GeneratedDocResult{EvidenceID: id, ClientVisible: visible}, nil
}
