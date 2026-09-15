package application

import (
	"context"
	"time"

	"jwswedding/internal/modules/projects/domain"
	vendorscontracts "jwswedding/internal/modules/vendors/contracts"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

type ProjectRepository interface {
	List(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64) ([]domain.Project, error) // CountAll backs the dashboard's TotalProjects stat.
	CountAll(ctx context.Context, tenantID int64) (int64, error)
	// ListForDashboard backs the dashboard's trend/near-D-day/upcoming/lagging
	// computations — bounded to rows that could matter for any of them
	// instead of List's entire unbounded tenant history. See the
	// infrastructure implementation's doc comment for `since`'s exact meaning.
	ListForDashboard(ctx context.Context, tenantID int64, since time.Time) ([]domain.Project, error)
	ListPaginated(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64, params pagination.Params, search, status string, showArchived bool, eventMonth *string) ([]domain.Project, int64, error)
	// ListByVenueID/ListByStaffPIC back the hard-delete "impact" endpoints
	// (PLAN.md) -- see their infrastructure implementations' doc comments.
	ListByVenueID(ctx context.Context, tenantID, venueID int64) ([]domain.ProjectRef, error)
	ListByStaffPIC(ctx context.Context, tenantID, staffID int64) ([]domain.ProjectRef, error)
	FindByID(ctx context.Context, tenantID, id int64) (*domain.Project, error)
	Create(ctx context.Context, p *domain.Project) error
	// CreateSeeded menyisipkan project + Timeline Default + tagihan awal
	// dalam SATU transaksi (jalur Accept, T3.2) — lihat implementasinya untuk
	// urutan dan alasan tiap sisipan.
	CreateSeeded(ctx context.Context, p *domain.Project, milestones []domain.ProjectMilestone) error
	// ListByClient / FindByQuotationID menopang port penawaran-client-master
	// (ProjectDirectory + idempotensi Accept) — jawab dari projects.client_id
	// dan projects.quotation_id, kolom milik modul ini sendiri.
	ListByClient(ctx context.Context, tenantID, clientID int64) ([]domain.Project, error)
	FindByQuotationID(ctx context.Context, tenantID, quotationID int64) (*domain.Project, error)
	// CountByClients menjawab hitungan project per client untuk satu halaman
	// daftar Client — satu query agregat, bukan N+1 (§11 PLAN).
	CountByClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]int, error)
	// ProjectIDsForQuotations memetakan penawaran ke project-nya untuk satu
	// halaman daftar Penawaran — satu query, bukan N+1.
	ProjectIDsForQuotations(ctx context.Context, tenantID int64, quotationIDs []int64) (map[int64]int64, error)
	Update(ctx context.Context, p *domain.Project) error
	SetStatus(ctx context.Context, tenantID, id int64, status domain.ProjectStatus) error
	SetArchived(ctx context.Context, tenantID, id int64, archived bool) error
	// DeleteCascade permanently removes the project and every same-module row
	// referencing it — see the infrastructure implementation's doc comment
	// (ADR-0013) for the exact deletion order and why it matters.
	DeleteCascade(ctx context.Context, tenantID, id int64) error
}

// VenueResolver is the narrow shape ProjectService needs from `vendors` to
// resolve a project's attached venue_id into display data (ADR-0016) --
// deliberately a local interface (not an import of vendors/application) so
// this file only depends on vendors' public contracts package for the return
// type. Bridged from main.go via Module.SetVenueResolver, the same two-phase
// idiom as SetClientAccessResolver -- needed because `vendors` itself depends
// on `projects.Contracts()` (built after `projects`), so `projects` can't
// take this as a constructor argument.
type VenueResolver interface {
	GetVenueSummary(ctx context.Context, tenantID, venueID int64) (vendorscontracts.VenueSummary, error)
}

// StaffNameResolver is the narrow shape ProjectService needs from `staff` to
// embed a project's PIC name directly in its own response (PLAN.md's Client
// Portal restructure). Deliberately typed with primitives only (not
// staffcontracts.Summary) -- staff/application already imports
// projects/contracts (for its own hard-delete impact-lookup bridge), so
// projects/application importing staff/contracts back would create an
// import cycle (projects/application -> staff/contracts -> staff/application
// -> projects/contracts -> projects/application). main.go bridges the real
// staffcontracts.Contracts into this shape with a small adapter instead.
// Unlike ClientCleaner/VenueResolver, `staff` is already built before
// `projects` in main.go, so this is a direct constructor argument, no
// two-phase setter needed.
type StaffNameResolver interface {
	// GetName resolves a staff member's name. ok is false when the ID
	// doesn't resolve (e.g. the staff row was hard-deleted, PLAN.md's Staff
	// hard delete) -- never an error, so a stale pic_staff_id degrades to
	// "no name available" rather than breaking the whole project response.
	GetName(ctx context.Context, tenantID, staffID int64) (name string, ok bool, err error)
}

type MilestoneRepository interface {
	ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectMilestone, error)
	// ListByProjects backs ComputeProgressBatch: every matching row across
	// the given projects in one query (WHERE project_id IN (...)).
	ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectMilestone, error)
	FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectMilestone, error)
	Create(ctx context.Context, m *domain.ProjectMilestone) error
	Update(ctx context.Context, m *domain.ProjectMilestone) error
	NextSortOrder(ctx context.Context, projectID int64) (int, error)
	Reorder(ctx context.Context, projectID int64, orderedIDs []int64) error
}

type ProjectService struct {
	repo               ProjectRepository
	milestones         MilestoneRepository
	milestoneTemplates MilestoneTemplateRepository
	vendorEngagements  VendorEngagementRepository
	vendorMilestones   VendorMilestoneRepository
	issues             IssueRepository
	payments           PaymentRepository
	venuePayments      VenuePaymentRepository
	evidence           *EvidenceService
	activity           *ActivityService
	venues             VenueResolver
	staff              StaffNameResolver
	// Penawaran-client-master: dipasang via setter two-phase (lihat
	// quotation_bridge.go) supaya signature konstruktor tidak berubah.
	activator  ClientActivator
	quotations QuotationResolver
	invoices   *ClientInvoiceService
}

func NewProjectService(
	repo ProjectRepository,
	milestones MilestoneRepository,
	milestoneTemplates MilestoneTemplateRepository,
	vendorEngagements VendorEngagementRepository,
	vendorMilestones VendorMilestoneRepository,
	issues IssueRepository,
	payments PaymentRepository,
	venuePayments VenuePaymentRepository,
	evidence *EvidenceService,
	activity *ActivityService,
	staff StaffNameResolver,
) *ProjectService {
	return &ProjectService{
		repo: repo, milestones: milestones, milestoneTemplates: milestoneTemplates, vendorEngagements: vendorEngagements,
		vendorMilestones: vendorMilestones, issues: issues, payments: payments, venuePayments: venuePayments,
		evidence: evidence, activity: activity, staff: staff,
	}
}

// CostSummary menghitung sisi biaya sebuah project terhadap nilai kontraknya.
//
// Pindah dari frontend ke sini dengan sengaja: gerbang komitmen vendor
// (VendorEngagementService) memakai angka yang SAMA, dan dua salinan
// perhitungan uang di dua lapisan adalah cara paling pasti untuk membuat
// layar dan aturan berselisih.
func (s *ProjectService) CostSummary(ctx context.Context, tenantID, projectID int64) (domain.ProjectCostSummary, error) {
	p, err := s.repo.FindByID(ctx, tenantID, projectID)
	if err != nil {
		return domain.ProjectCostSummary{}, err
	}
	if p == nil {
		return domain.ProjectCostSummary{}, apperror.NotFound("Project tidak ditemukan")
	}
	engagements, err := s.vendorEngagements.ListByProject(ctx, projectID)
	if err != nil {
		return domain.ProjectCostSummary{}, err
	}
	var vendorCost int64
	for _, e := range engagements {
		if e.EngagementStatus == domain.EngagementCancelled {
			continue
		}
		vendorCost += e.ContractValue
	}
	return newCostSummary(p, vendorCost), nil
}

// newCostSummary merakit ringkasan dari dua bahan yang sudah dibaca pemanggil
// — dipisah supaya gerbang komitmen bisa menghitung ulang dengan nilai vendor
// HIPOTETIS (sesudah engagement yang sedang ditulis) tanpa membaca ulang DB.
func newCostSummary(p *domain.Project, vendorCost int64) domain.ProjectCostSummary {
	var venueCost int64
	if p.VenueRentalPrice != nil {
		venueCost += *p.VenueRentalPrice
	}
	if p.VenueCharge != nil {
		venueCost += *p.VenueCharge
	}
	committed := vendorCost + venueCost
	return domain.ProjectCostSummary{
		ContractValue: p.ContractValue,
		VendorCost:    vendorCost,
		VenueCost:     venueCost,
		CommittedCost: committed,
		Remaining:     p.ContractValue - committed,
	}
}

// ResolvePICName backs the `picName` field on a project's response (PLAN.md's
// Client Portal restructure) -- returns "" on any failure (unresolved ID,
// nil resolver) rather than erroring, since this is display-only metadata,
// never a required field.
func (s *ProjectService) ResolvePICName(ctx context.Context, tenantID, staffID int64) string {
	if s.staff == nil {
		return ""
	}
	name, ok, err := s.staff.GetName(ctx, tenantID, staffID)
	if err != nil || !ok {
		return ""
	}
	return name
}

// SetVenueResolver completes the two-phase wiring described on VenueResolver
// above. main.go calls this right after vendorsModule is built, the same
// slot as SetClientActivator/SetClientAccessResolver.
func (s *ProjectService) SetVenueResolver(resolver VenueResolver) {
	s.venues = resolver
}

// GetVenue resolves this project's attached venue (if any) into display
// data via the vendors module's contract. Returns (nil, nil) — not an error
// — when no venue is attached, so callers (both the WO Console tab and
// Client Portal's) can render an empty state rather than treat it as a
// failure. A NotFound from the vendors module (the venue itself has since
// been hard-deleted — PLAN.md's Vendor/Venue hard delete, which never
// cleans up this stale reference) is treated exactly the same way: the
// venue is simply no longer resolvable, not a real error to surface.
func (s *ProjectService) GetVenue(ctx context.Context, tenantID, projectID int64) (*vendorscontracts.VenueSummary, error) {
	p, err := s.Get(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if p.VenueID == nil || s.venues == nil {
		return nil, nil
	}
	summary, err := s.venues.GetVenueSummary(ctx, tenantID, *p.VenueID)
	if err != nil {
		if appErr, ok := apperror.As(err); ok && appErr.Kind == apperror.KindNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

// picStaffID/picSalesStaffID scope results to a single PIC's own projects —
// nil/nil means every project in the tenant (see
// MySQLProjectRepository.List's own doc comment).
func (s *ProjectService) List(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64) ([]domain.Project, error) {
	return s.repo.List(ctx, tenantID, picStaffID, picSalesStaffID)
}

// ListAffectedProjectsForVenue backs the hard-delete "impact" endpoint for
// Venue (PLAN.md) -- see ListByVenueID's doc comment.
func (s *ProjectService) ListAffectedProjectsForVenue(ctx context.Context, tenantID, venueID int64) ([]domain.ProjectRef, error) {
	return s.repo.ListByVenueID(ctx, tenantID, venueID)
}

// ListAffectedProjectsForStaff backs the hard-delete "impact" endpoint for
// Staff (PLAN.md) -- see ListByStaffPIC's doc comment.
func (s *ProjectService) ListAffectedProjectsForStaff(ctx context.Context, tenantID, staffID int64) ([]domain.ProjectRef, error) {
	return s.repo.ListByStaffPIC(ctx, tenantID, staffID)
}

// CountAll backs the dashboard's TotalProjects stat.
func (s *ProjectService) CountAll(ctx context.Context, tenantID int64) (int64, error) {
	return s.repo.CountAll(ctx, tenantID)
}

// ListForDashboard backs every other dashboard computation — see
// ProjectRepository.ListForDashboard's doc comment.
func (s *ProjectService) ListForDashboard(ctx context.Context, tenantID int64, since time.Time) ([]domain.Project, error) {
	return s.repo.ListForDashboard(ctx, tenantID, since)
}

// ListPaginated backs the real project list page — showArchived splits the
// result into two disjoint views (active vs. archived), never merged, so
// archived projects stay genuinely out of the way of day-to-day work (see
// ADR-0013) rather than just visually deprioritized in a mixed list.
func (s *ProjectService) ListPaginated(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64, params pagination.Params, search, status string, showArchived bool, eventMonth *string) ([]domain.Project, int64, error) {
	return s.repo.ListPaginated(ctx, tenantID, picStaffID, picSalesStaffID, params, search, status, showArchived, eventMonth)
}

func (s *ProjectService) Get(ctx context.Context, tenantID, id int64) (*domain.Project, error) {
	p, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperror.NotFound("Project tidak ditemukan")
	}
	return p, nil
}

// ExistsForTenant is used by the `clients` module (via contracts) to validate
// a project_id before creating a client row — no cross-module FK, so this is
// the only way `clients` can be sure the project it's pointed at is real and
// belongs to the same tenant.
func (s *ProjectService) ExistsForTenant(ctx context.Context, tenantID, id int64) (bool, error) {
	p, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return false, err
	}
	return p != nil, nil
}

type ProjectInput struct {
	Name      string
	BrideName string
	GroomName string
	EventDate time.Time
	// EventStartTime/EventEndTime are the project-level Jam Acara ("HH:MM"),
	// PLAN.md revisi-putri-mom-25082026 Blok A. Presentation converts an empty
	// string in the body to nil before this struct is built, so nil here means
	// "Belum ditentukan"; a non-nil pointer holds "HH:MM". Both keys are always
	// present in the body, so emptying one (non-nil -> nil) is a real change and
	// guardKonteksUmum must treat it as such (sameOptionalString, not
	// venueRefChanged).
	EventStartTime *string
	EventEndTime   *string
	// Pax is the guest count (blok B1 of the PO Paket). 0 = belum ditentukan,
	// so an absent key is indistinguishable from an explicit 0 -- deliberate,
	// same as Project.Pax's own sentinel.
	Pax           int
	Venue         string
	PrepStartDate time.Time
	PackageName   string
	ContractValue int64
	Status        domain.ProjectStatus
	PICStaffID    int64
	// PICSalesStaffID is the "PIC Sales" slot -- see Project.PICSalesStaffID's
	// doc comment. Create forces this to the caller's own staff id when
	// callerRole is "Sales" (ignoring whatever's sent here); Owner/Admin's
	// value passes through untouched.
	PICSalesStaffID int64
	Description     string
	// VenueID is only ever read by Update (Create always leaves a new
	// project's VenueID nil — attaching a venue is a separate, post-creation
	// action, see ADR-0016). nil means the caller's JSON body omitted the
	// key entirely — leave the project's current attachment untouched; `0`
	// explicitly detaches; a positive ID attaches that venue. Venue IDs are
	// AUTO_INCREMENT starting at 1, so `0` is never a real one.
	VenueID *int64
	// VenueRentalPrice/VenueCharge ride along with VenueID whenever it's
	// present at all (see Update below) -- the per-project cost snapshot,
	// PLAN.md "Financial Calculation Correctness". Only meaningful when
	// VenueID is a positive attach/change; ignored on detach (force-cleared
	// regardless) and left nil-safe when VenueID itself is nil (every other
	// project edit that never touches venue attachment at all).
	VenueRentalPrice *int64
	VenueCharge      *int64
}

// Update rejects a Wedding Planner ("Staff" role) reassigning PICStaffID —
// confirmed role rule: only Owner/Admin assign or reassign who a project's
// PIC is, even on a project the Wedding Planner already manages day to day.
// callerRole == "" (or any non-"Staff"/"Sales" value) skips both checks
// entirely, so Owner/Admin keep reassigning freely exactly as before. A
// Sales caller may NOT move PICSalesStaffID to someone else (their own PIC
// Sales slot is immutable from their side), but MAY freely change
// PICStaffID -- that's the handover mechanism (PLAN.md
// revisi-timeline-vendor-role-sales); reaching this method at all as Sales
// already implies resolveProjectAccess confirmed they own this project.
func (s *ProjectService) Update(ctx context.Context, tenantID, id int64, actorStaffID int64, callerRole string, input ProjectInput) (*domain.Project, error) {
	p, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := validateProjectStatus(input.Status); err != nil {
		return nil, err
	}
	if callerRole == "Staff" && input.PICStaffID != p.PICStaffID {
		return nil, apperror.Forbidden("Hanya Owner atau Admin yang dapat menugaskan ulang PIC project")
	}
	if callerRole == "Sales" && input.PICSalesStaffID != p.PICSalesStaffID {
		return nil, apperror.Forbidden("Sales tidak dapat memindahkan PIC Sales project ini")
	}
	if err := guardKonteksUmum(callerRole, p, input); err != nil {
		return nil, err
	}
	p.Name = input.Name
	p.BrideName = input.BrideName
	p.GroomName = input.GroomName
	p.EventDate = input.EventDate
	p.EventStartTime = input.EventStartTime
	p.EventEndTime = input.EventEndTime
	p.Venue = input.Venue
	p.PrepStartDate = input.PrepStartDate
	p.Pax = input.Pax
	p.PackageName = input.PackageName
	p.ContractValue = input.ContractValue
	p.Status = input.Status
	p.PICStaffID = input.PICStaffID
	p.PICSalesStaffID = input.PICSalesStaffID
	p.Description = input.Description
	if input.VenueID != nil {
		if *input.VenueID == 0 {
			// Detach always force-clears the cost snapshot too, regardless of
			// whatever the request body happened to send for these two
			// fields -- there is no cost without a venue.
			p.VenueID = nil
			p.VenueRentalPrice = nil
			p.VenueCharge = nil
		} else {
			p.VenueID = input.VenueID
			p.VenueRentalPrice = input.VenueRentalPrice
			p.VenueCharge = input.VenueCharge
		}
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &p.ID, domain.ActivityProjectUpdated, actorStaffID, "project", formatID(p.ID), p.Name,
		"Informasi project diperbarui")
	return p, nil
}

// validateProjectStatus rejects status values the database itself would
// refuse (projects.status is a MySQL ENUM) — without this, an unknown
// status sails through to repo.Update and comes back as a bare 500 instead
// of a field-keyed 422. Same hazard class as validateEngagementInput in
// vendor_engagement_service.go.
func validateProjectStatus(s domain.ProjectStatus) error {
	switch s {
	case domain.StatusDraft, domain.StatusPreparation, domain.StatusReady,
		domain.StatusCompleted, domain.StatusCancelled:
		return nil
	default:
		return apperror.Validation("Status project tidak valid", map[string][]string{
			"status": {"Status project tidak valid"},
		})
	}
}

// guardKonteksUmum rejects a non-Owner/Admin caller (a Wedding Planner
// "Staff", or "Sales") changing any field of a project's "konteks umum" --
// confirmed role rule, PLAN.md mom-25082026-item-sebagian §3a: only Status
// and Description stay freely editable by every role; everything else
// (identity, schedule, package, contract value, venue attachment) is
// Owner/Admin only. callerRole == "" (or any other non-Staff/Sales value)
// skips the check entirely, same convention as the PIC guards above it in
// Update.
//
// VenueID/VenueRentalPrice/VenueCharge are *int64 -- nil means "the request
// body omitted this key, leave the current attachment untouched" (see
// ProjectInput.VenueID's doc comment), so a nil input is never treated as a
// change even when the project currently has a venue attached. Comparisons
// below dereference both sides rather than comparing pointers directly,
// since two distinct pointers holding the same value would otherwise always
// register as "changed".
//
// EventDate/PrepStartDate are compared by calendar date (sameCalendarDate),
// never time.Time.Equal -- found live in mom-25082026-item-sebagian's own
// HTTP verification pass: the request-parsed value is always UTC
// (presentation's parseDate uses time.Parse with no location), while a
// value freshly loaded from the database carries whatever DATABASE_URL's
// loc= param says (this project's is "Local", i.e. the server OS's zone --
// WIB/+07:00 in production). Equal() compares absolute instants, so an
// unrelated 7-hour offset between two midnights representing the exact same
// calendar date made this guard see EventDate as "changed" on every single
// request, rejecting a Wedding Planner even when they only touched Status --
// exactly the regression this guard exists to avoid. sameCalendarDate reads
// each side's Y/M/D as carried by its own time.Time (time.Time.Date() never
// converts location first), which is location-independent and matches what
// a DATE column actually means: no time-of-day component to lose.
func guardKonteksUmum(callerRole string, p *domain.Project, input ProjectInput) error {
	if callerRole != "Staff" && callerRole != "Sales" {
		return nil
	}
	changed := input.Name != p.Name ||
		input.BrideName != p.BrideName ||
		input.GroomName != p.GroomName ||
		!sameCalendarDate(input.EventDate, p.EventDate) ||
		!sameOptionalString(input.EventStartTime, p.EventStartTime) ||
		!sameOptionalString(input.EventEndTime, p.EventEndTime) ||
		input.Pax != p.Pax ||
		input.Venue != p.Venue ||
		!sameCalendarDate(input.PrepStartDate, p.PrepStartDate) ||
		input.PackageName != p.PackageName ||
		input.ContractValue != p.ContractValue ||
		venueRefChanged(input.VenueID, p.VenueID) ||
		venueRefChanged(input.VenueRentalPrice, p.VenueRentalPrice) ||
		venueRefChanged(input.VenueCharge, p.VenueCharge)
	if changed {
		return apperror.Forbidden("Hanya Owner atau Admin yang dapat mengubah data umum project")
	}
	return nil
}

// sameCalendarDate reports whether a and b fall on the same Y/M/D, ignoring
// time-of-day and location entirely -- see guardKonteksUmum's doc comment
// for why time.Time.Equal is the wrong tool for comparing two DATE-only
// values that may have been constructed under different locations.
func sameCalendarDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// venueRefChanged reports whether a nullable int64 field actually changed,
// treating a nil input (key omitted from the request body) as "no change"
// regardless of what the project currently holds -- see guardKonteksUmum's
// doc comment.
func venueRefChanged(input, current *int64) bool {
	if input == nil {
		return false
	}
	return current == nil || *input != *current
}

// sameOptionalString reports whether two optional string fields are equal,
// treating both-nil as equal and exactly-one-nil as different. Unlike
// venueRefChanged, a nil input is NOT "no change": the project-level Jam Acara
// (Blok A) always sends both keys in the body, so clearing a value (non-nil ->
// nil) is a genuine edit that guardKonteksUmum must catch. See ProjectInput's
// EventStartTime doc comment.
func sameOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (s *ProjectService) Cancel(ctx context.Context, tenantID, id, actorStaffID int64) (*domain.Project, error) {
	p, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetStatus(ctx, tenantID, id, domain.StatusCancelled); err != nil {
		return nil, err
	}
	p.Status = domain.StatusCancelled
	s.activity.Record(ctx, &p.ID, domain.ActivityProjectStatusChanged, actorStaffID, "project", formatID(p.ID), p.Name,
		"Project dibatalkan")
	return p, nil
}

// SetArchived toggles a project's archive flag — reversible, orthogonal to
// Status (see ADR-0013). No restriction on which status can be archived.
func (s *ProjectService) SetArchived(ctx context.Context, tenantID, id, actorStaffID int64, archived bool) (*domain.Project, error) {
	p, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetArchived(ctx, tenantID, id, archived); err != nil {
		return nil, err
	}
	p.IsArchived = archived
	action := "diarsipkan"
	if !archived {
		action = "dipulihkan dari arsip"
	}
	s.activity.Record(ctx, &p.ID, domain.ActivityProjectStatusChanged, actorStaffID, "project", formatID(p.ID), p.Name,
		"Project "+action)
	return p, nil
}

// Delete menghapus permanen satu project beserta seluruh isinya dan
// penawarannya (D14 — informed consent lewat dialog delete-impact, bukan
// blokir arsip/batal). Hanya Owner yang boleh memanggil (gate di handler).
// Tidak ada entri activity log untuk penghapusan itu sendiri: baris
// activity_log project ini ikut terhapus dalam transaksi yang sama.
func (s *ProjectService) Delete(ctx context.Context, tenantID, id int64) error {
	if _, err := s.Get(ctx, tenantID, id); err != nil {
		return err
	}
	return s.DeleteProjectCascade(ctx, tenantID, id)
}

// --- Project milestones ---

type MilestoneInput struct {
	Name       string
	Category   string
	TargetDate time.Time
}

func (s *ProjectService) ListMilestones(ctx context.Context, tenantID, projectID int64) ([]domain.ProjectMilestone, error) {
	if _, err := s.Get(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	return s.milestones.ListByProject(ctx, projectID)
}

func (s *ProjectService) CreateMilestone(ctx context.Context, tenantID, projectID int64, actorStaffID int64, input MilestoneInput) (*domain.ProjectMilestone, error) {
	if _, err := s.Get(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	order, err := s.milestones.NextSortOrder(ctx, projectID)
	if err != nil {
		return nil, err
	}
	m := &domain.ProjectMilestone{
		ProjectID: projectID, SortOrder: order, Name: input.Name, Category: input.Category,
		Status: domain.MilestoneNotStarted, TargetDate: input.TargetDate,
	}
	if err := s.milestones.Create(ctx, m); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityMilestoneUpdated, actorStaffID, "project_milestone", formatID(m.ID), m.Name,
		"Timeline project ditambahkan: "+m.Name)
	return m, nil
}

type MilestoneUpdateInput struct {
	Category      string
	Status        domain.MilestoneStatus
	TargetDate    time.Time
	CompletedDate *time.Time
}

// UpdateMilestone lets the WO managing this project correct its own
// schedule — Status, TargetDate, and CompletedDate are all set directly from
// client input (no more server-side auto-stamping of CompletedDate), mirroring
// VendorEngagementService.UpdateMilestone's shape for the sibling
// vendor-milestone entity.
func (s *ProjectService) UpdateMilestone(ctx context.Context, tenantID, projectID, milestoneID int64, actorStaffID int64, input MilestoneUpdateInput) (*domain.ProjectMilestone, error) {
	if _, err := s.Get(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	m, err := s.milestones.FindByID(ctx, projectID, milestoneID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperror.NotFound("Timeline tidak ditemukan")
	}
	if err := validateMilestoneStatus(input.Status); err != nil {
		return nil, err
	}
	if err := guardStatusSelesai(ctx, s.evidence, m, input, milestoneID); err != nil {
		return nil, err
	}
	m.Category = input.Category
	m.Status = input.Status
	m.TargetDate = input.TargetDate
	m.CompletedDate = input.CompletedDate
	if err := s.milestones.Update(ctx, m); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityMilestoneUpdated, actorStaffID, "project_milestone", formatID(m.ID), m.Name,
		"Timeline project diperbarui: "+m.Name)
	return m, nil
}

// guardStatusSelesai enforces MOM 25/08/2026 item 5: a timeline can't be
// marked Completed without a CompletedDate and at least one attached
// lampiran. Only guards the TRANSITION into Completed (m.Status was
// something else, input.Status is now Completed) — deliberate (PLAN.md
// mom-25082026-item-belum D1): production already holds Completed rows from
// before this rule existed, some without evidence, and if this guard also
// re-validated every save on an already-Completed row, editing any of
// those old rows (e.g. fixing a typo'd TargetDate) would be permanently
// blocked with no way out from the UI. A row already Completed stays
// editable; only the transition into Completed is guarded, so no new row
// can become Completed without proof going forward.
func guardStatusSelesai(ctx context.Context, evidence *EvidenceService, m *domain.ProjectMilestone, input MilestoneUpdateInput, milestoneID int64) error {
	if input.Status != domain.MilestoneCompleted || m.Status == domain.MilestoneCompleted {
		return nil
	}
	if input.CompletedDate == nil {
		return apperror.Validation("Tanggal selesai wajib diisi", map[string][]string{
			"completedDate": {"Timeline tidak bisa ditandai Completed tanpa tanggal selesai"},
		})
	}
	has, err := evidence.HasForRelated(ctx, domain.RelatedProjectMilestone, milestoneID)
	if err != nil {
		return err
	}
	if !has {
		return apperror.Validation("Lampiran wajib diisi", map[string][]string{
			"lampiran": {"Timeline tidak bisa ditandai Completed tanpa lampiran. Bila tidak ada dokumen asli, unggah screenshot jadwal meeting atau dokumen kosong."},
		})
	}
	return nil
}

// ReorderMilestones rewrites every milestone's SortOrder for this project to
// match the position of its ID in orderedIDs. orderedIDs must be an exact
// permutation of the project's existing milestone IDs — rejected otherwise,
// so a stale/corrupted client payload can't silently drop or duplicate a row.
func (s *ProjectService) ReorderMilestones(ctx context.Context, tenantID, projectID int64, actorStaffID int64, orderedIDs []int64) error {
	if _, err := s.Get(ctx, tenantID, projectID); err != nil {
		return err
	}
	existing, err := s.milestones.ListByProject(ctx, projectID)
	if err != nil {
		return err
	}
	if len(orderedIDs) != len(existing) {
		return apperror.Validation("Urutan timeline tidak valid", nil)
	}
	existingIDs := make(map[int64]bool, len(existing))
	for _, m := range existing {
		existingIDs[m.ID] = true
	}
	seen := make(map[int64]bool, len(orderedIDs))
	for _, id := range orderedIDs {
		if !existingIDs[id] || seen[id] {
			return apperror.Validation("Urutan timeline tidak valid", nil)
		}
		seen[id] = true
	}
	if err := s.milestones.Reorder(ctx, projectID, orderedIDs); err != nil {
		return err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityMilestoneUpdated, actorStaffID, "project_milestone", "", "",
		"Urutan timeline diubah")
	return nil
}

// --- Progress computation (mirrors mock/selectors.ts computeProjectProgress) ---

func (s *ProjectService) ComputeProgress(ctx context.Context, tenantID, projectID int64, asOf time.Time) (*domain.ProjectProgress, error) {
	if _, err := s.Get(ctx, tenantID, projectID); err != nil {
		return nil, err
	}

	projectMilestones, err := s.milestones.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	projectStats := domain.ComputeMilestoneStats(toMilestoneLikes(projectMilestones), asOf)

	vendorEngagements, err := s.vendorEngagements.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var allVendorMilestones []domain.VendorMilestone
	for _, pv := range vendorEngagements {
		vms, err := s.vendorMilestones.ListByProjectVendor(ctx, pv.ID)
		if err != nil {
			return nil, err
		}
		allVendorMilestones = append(allVendorMilestones, vms...)
	}
	vendorStats := domain.ComputeMilestoneStats(toVendorMilestoneLikes(allVendorMilestones), asOf)

	issues, err := s.issues.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var openIssues []domain.VendorIssue
	for _, i := range issues {
		if i.Status.IsOpen() {
			openIssues = append(openIssues, i)
		}
	}

	payments, err := s.payments.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	evidences, err := s.evidence.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	hasInvoice, hasProof := domain.PaymentEvidenceStatus(evidences, domain.RelatedPayment)
	incompleteCount := 0
	for _, p := range payments {
		if !domain.IsPaymentEvidenceComplete(p.Type, p.ID, hasInvoice, hasProof) {
			incompleteCount++
		}
	}

	venuePayments, err := s.venuePayments.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	vHasInvoice, vHasProof := domain.PaymentEvidenceStatus(evidences, domain.RelatedVenuePayment)
	for _, p := range venuePayments {
		if !domain.IsPaymentEvidenceComplete(p.Type, p.ID, vHasInvoice, vHasProof) {
			incompleteCount++
		}
	}

	progress := domain.ComputeProjectProgress(projectStats, vendorStats, openIssues, incompleteCount)
	return &progress, nil
}

// ComputeProgressBatch computes progress for every id in projectIDs using
// ONE query per data source (WHERE project_id IN (...)) instead of
// ComputeProgress's ~7 queries repeated per project — see PLAN.md
// "Performance remediation". Every project in projectIDs is assumed to
// already belong to tenantID (callers fetch the roster themselves), so,
// unlike ComputeProgress, this does not re-verify each id via s.Get.
func (s *ProjectService) ComputeProgressBatch(ctx context.Context, tenantID int64, projectIDs []int64, asOf time.Time) (map[int64]domain.ProjectProgress, error) {
	result := make(map[int64]domain.ProjectProgress, len(projectIDs))
	if len(projectIDs) == 0 {
		return result, nil
	}

	allMilestones, err := s.milestones.ListByProjects(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	milestonesByProject := make(map[int64][]domain.ProjectMilestone)
	for _, m := range allMilestones {
		milestonesByProject[m.ProjectID] = append(milestonesByProject[m.ProjectID], m)
	}

	allEngagements, err := s.vendorEngagements.ListByProjects(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	engagementsByProject := make(map[int64][]domain.ProjectVendor)
	engagementIDs := make([]int64, 0, len(allEngagements))
	for _, pv := range allEngagements {
		engagementsByProject[pv.ProjectID] = append(engagementsByProject[pv.ProjectID], pv)
		engagementIDs = append(engagementIDs, pv.ID)
	}

	allVendorMilestones, err := s.vendorMilestones.ListByProjectVendors(ctx, engagementIDs)
	if err != nil {
		return nil, err
	}
	vendorMilestonesByEngagement := make(map[int64][]domain.VendorMilestone)
	for _, vm := range allVendorMilestones {
		vendorMilestonesByEngagement[vm.ProjectVendorID] = append(vendorMilestonesByEngagement[vm.ProjectVendorID], vm)
	}

	allIssues, err := s.issues.ListByProjects(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	issuesByProject := make(map[int64][]domain.VendorIssue)
	for _, i := range allIssues {
		issuesByProject[i.ProjectID] = append(issuesByProject[i.ProjectID], i)
	}

	allPayments, err := s.payments.ListByProjects(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	paymentsByProject := make(map[int64][]domain.VendorPayment)
	for _, p := range allPayments {
		paymentsByProject[p.ProjectID] = append(paymentsByProject[p.ProjectID], p)
	}

	allVenuePayments, err := s.venuePayments.ListByProjects(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	venuePaymentsByProject := make(map[int64][]domain.VenuePayment)
	for _, p := range allVenuePayments {
		venuePaymentsByProject[p.ProjectID] = append(venuePaymentsByProject[p.ProjectID], p)
	}

	allEvidence, err := s.evidence.ListByProjects(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	evidenceByProject := make(map[int64][]domain.Evidence)
	for _, e := range allEvidence {
		evidenceByProject[e.ProjectID] = append(evidenceByProject[e.ProjectID], e)
	}

	for _, projectID := range projectIDs {
		projectStats := domain.ComputeMilestoneStats(toMilestoneLikes(milestonesByProject[projectID]), asOf)

		var vendorMilestonesForProject []domain.VendorMilestone
		for _, pv := range engagementsByProject[projectID] {
			vendorMilestonesForProject = append(vendorMilestonesForProject, vendorMilestonesByEngagement[pv.ID]...)
		}
		vendorStats := domain.ComputeMilestoneStats(toVendorMilestoneLikes(vendorMilestonesForProject), asOf)

		var openIssues []domain.VendorIssue
		for _, i := range issuesByProject[projectID] {
			if i.Status.IsOpen() {
				openIssues = append(openIssues, i)
			}
		}

		evidences := evidenceByProject[projectID]
		hasInvoice, hasProof := domain.PaymentEvidenceStatus(evidences, domain.RelatedPayment)
		incompleteCount := 0
		for _, p := range paymentsByProject[projectID] {
			if !domain.IsPaymentEvidenceComplete(p.Type, p.ID, hasInvoice, hasProof) {
				incompleteCount++
			}
		}

		vHasInvoice, vHasProof := domain.PaymentEvidenceStatus(evidences, domain.RelatedVenuePayment)
		for _, p := range venuePaymentsByProject[projectID] {
			if !domain.IsPaymentEvidenceComplete(p.Type, p.ID, vHasInvoice, vHasProof) {
				incompleteCount++
			}
		}

		result[projectID] = domain.ComputeProjectProgress(projectStats, vendorStats, openIssues, incompleteCount)
	}

	return result, nil
}

func toMilestoneLikes(ms []domain.ProjectMilestone) []domain.MilestoneLike {
	out := make([]domain.MilestoneLike, len(ms))
	for i, m := range ms {
		out[i] = domain.MilestoneLike{Status: m.Status, TargetDate: m.TargetDate}
	}
	return out
}

func toVendorMilestoneLikes(ms []domain.VendorMilestone) []domain.MilestoneLike {
	out := make([]domain.MilestoneLike, len(ms))
	for i, m := range ms {
		out[i] = domain.MilestoneLike{Status: m.Status, TargetDate: m.TargetDate}
	}
	return out
}
