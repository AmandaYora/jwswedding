package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
)

type VendorEngagementRepository interface {
	ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectVendor, error)
	// ListByProjects backs ComputeProgressBatch: every matching row across
	// the given projects in one query (WHERE project_id IN (...)).
	ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectVendor, error)
	FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectVendor, error)
	Create(ctx context.Context, pv *domain.ProjectVendor) error
	Update(ctx context.Context, pv *domain.ProjectVendor) error
	SetStatus(ctx context.Context, projectID, id int64, status domain.EngagementStatus) error
	ListByVendor(ctx context.Context, tenantID, vendorID int64) ([]domain.VendorEngagementHistoryRow, error)
}

type VendorMilestoneRepository interface {
	ListByProjectVendor(ctx context.Context, projectVendorID int64) ([]domain.VendorMilestone, error)
	// ListByProjectVendors backs ComputeProgressBatch: every matching row
	// across the given engagement ids in one query.
	ListByProjectVendors(ctx context.Context, projectVendorIDs []int64) ([]domain.VendorMilestone, error)
	FindByID(ctx context.Context, projectVendorID, id int64) (*domain.VendorMilestone, error)
	Create(ctx context.Context, m *domain.VendorMilestone) error
	Update(ctx context.Context, m *domain.VendorMilestone) error
	NextSortOrder(ctx context.Context, projectVendorID int64) (int, error)
}

// ProjectBudgetReader adalah satu hal yang dibutuhkan gerbang komitmen:
// sisi biaya project berhadapan dengan nilai kontraknya. Dipenuhi
// *ProjectService (satu modul — ini bukan lintas batas), dipasang lewat
// setter supaya signature konstruktor dan seluruh pemanggilnya tidak berubah.
//
// nil berarti gerbangnya tidak terpasang: engagement tetap tersimpan tanpa
// pemeriksaan. Sengaja — jalur uji dan skrip tidak boleh gagal hanya karena
// bagian ini tidak dirakit.
type ProjectBudgetReader interface {
	CostBasis(ctx context.Context, tenantID, projectID int64) (domain.ProjectCostBasis, error)
}

type VendorEngagementService struct {
	repo       VendorEngagementRepository
	milestones VendorMilestoneRepository
	activity   *ActivityService
	budget     ProjectBudgetReader
}

func NewVendorEngagementService(repo VendorEngagementRepository, milestones VendorMilestoneRepository, activity *ActivityService) *VendorEngagementService {
	return &VendorEngagementService{repo: repo, milestones: milestones, activity: activity}
}

// SetBudgetReader melengkapi perakitan dua-fase — lihat ProjectBudgetReader.
func (s *VendorEngagementService) SetBudgetReader(r ProjectBudgetReader) {
	s.budget = r
}

// sumActiveContracts menjumlahkan nilai kontrak engagement yang tidak
// Cancelled, dengan satu baris dikecualikan (excludeID) — jalur Update
// memakainya untuk mengeluarkan nilai lama sebelum menambahkan yang baru.
// excludeID 0 tidak mengecualikan apa pun.
func sumActiveContracts(list []domain.ProjectVendor, excludeID int64) int64 {
	var total int64
	for _, e := range list {
		if e.ID == excludeID || e.EngagementStatus == domain.EngagementCancelled {
			continue
		}
		total += e.ContractValue
	}
	return total
}

// budgetCheck mengumpulkan bahan satu pemeriksaan anggaran. Struct, bukan
// tujuh parameter posisional yang empat di antaranya int64.
type budgetCheck struct {
	tenantID     int64
	projectID    int64
	actorStaffID int64
	// excludeID adalah engagement yang sedang DIGANTI (jalur Update); 0 pada
	// jalur Create. Tanpanya, memperbarui satu engagement akan menghitung
	// nilai lamanya dan nilai barunya sekaligus.
	excludeID   int64
	newContract int64
	newStatus   domain.EngagementStatus
	reason      string
}

// guardBudget adalah gerbang komitmen biaya.
//
// Aturannya BUKAN "biaya tidak boleh melampaui nilai kontrak" — harga vendor
// yang sudah dinegosiasikan adalah fakta, dan menolak menyimpannya hanya
// memindahkan biayanya ke luar sistem atau mendorong orang mengecilkan
// angkanya. Aturannya: **tidak boleh melampaui tanpa ada yang menyadarinya
// dan tanpa ada yang bertanggung jawab.** Karena itu penolakan hanya terjadi
// saat alasan belum diisi, dan begitu diisi, komitmennya lolos sambil
// meninggalkan jejak siapa-kapan-kenapa.
//
// projectedVendorCost adalah total biaya vendor SESUDAH tulisan ini jadi —
// pemanggil yang menghitungnya, karena hanya ia yang tahu baris lama mana
// yang sedang digantikan.
func (s *VendorEngagementService) guardBudget(ctx context.Context, c budgetCheck) error {
	// Keluar lebih dulu saat gerbangnya tidak terpasang: tanpa ini, setiap
	// tulisan engagement membayar satu pembacaan daftar + satu pembacaan
	// project untuk hasil yang tidak dipakai.
	if s.budget == nil {
		return nil
	}
	// SATU pembacaan daftar engagement, dipakai untuk biaya kini DAN proyeksi.
	existing, err := s.repo.ListByProject(ctx, c.projectID)
	if err != nil {
		return err
	}
	basis, err := s.budget.CostBasis(ctx, c.tenantID, c.projectID)
	if err != nil {
		return err
	}
	projectedVendorCost := sumActiveContracts(existing, c.excludeID)
	if c.newStatus != domain.EngagementCancelled {
		projectedVendorCost += c.newContract
	}
	// sumActiveContracts(existing, 0) melewati baris Cancelled dan menjumlahkan
	// ContractValue — identik dengan yang dulu dihitung CostSummary di dalam
	// dirinya sendiri, jadi `current` di bawah sama persis dengan `summary` lama.
	current := summaryFromBasis(basis, sumActiveContracts(existing, 0))
	projected := summaryFromBasis(basis, projectedVendorCost)
	if projected.Remaining >= 0 {
		return nil
	}
	// Sudah melampaui SEBELUM tulisan ini, dan tulisan ini tidak menambah
	// beban: lewat tanpa bertanya. Tanpa syarat ini, sebuah project yang
	// terlanjur minus akan meminta alasan pada setiap penyuntingan vendor —
	// termasuk saat nilainya justru DITURUNKAN — dan mencatat peristiwa yang
	// tidak pernah terjadi. Yang perlu diakui adalah pelampauan yang dibuat
	// atau diperdalam, bukan yang sedang diperbaiki.
	if projected.CommittedCost <= current.CommittedCost {
		return nil
	}
	over := -projected.Remaining
	if strings.TrimSpace(c.reason) == "" {
		return apperror.Validation(
			"Biaya melampaui Nilai Kontrak sebesar "+formatRupiahShort(over),
			map[string][]string{
				"overBudgetReason": {
					"Kalau tambahan ini disepakati klien, revisi penawarannya lebih dulu. " +
						"Kalau memang ditanggung sendiri, isi alasannya agar tercatat.",
				},
				"overage": {strconv.FormatInt(over, 10)},
			},
		)
	}
	s.activity.Record(ctx, &c.projectID, domain.ActivityVendorOverBudget, c.actorStaffID, "project", formatID(c.projectID),
		formatRupiahShort(over),
		truncateTo("Komitmen vendor melampaui Nilai Kontrak sebesar "+formatRupiahShort(over)+". Alasan: "+strings.TrimSpace(c.reason), maxActivityDescription))
	return nil
}

// summaryFromBasis merakit ringkasan biaya dari basis project (nilai kontrak +
// venue) dan satu angka biaya vendor — yang nyata maupun yang hipotetis
// (sesudah engagement yang sedang ditulis). Satu helper untuk keduanya, karena
// perbedaannya hanya pada angka yang dimasukkan, bukan pada rumusnya.
func summaryFromBasis(basis domain.ProjectCostBasis, vendorCost int64) domain.ProjectCostSummary {
	committed := vendorCost + basis.VenueCost
	return domain.ProjectCostSummary{
		ContractValue: basis.ContractValue,
		VendorCost:    vendorCost,
		VenueCost:     basis.VenueCost,
		CommittedCost: committed,
		Remaining:     basis.ContractValue - committed,
	}
}

// formatRupiahShort dipakai pesan galat dan catatan aktivitas — keduanya
// dibaca manusia, jadi angkanya harus terbaca seperti di layar.
func formatRupiahShort(v int64) string {
	s := strconv.FormatInt(v, 10)
	out := ""
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += "."
		}
		out += string(s[i])
	}
	return "Rp " + out
}

func (s *VendorEngagementService) List(ctx context.Context, projectID int64) ([]domain.ProjectVendor, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *VendorEngagementService) Get(ctx context.Context, projectID, id int64) (*domain.ProjectVendor, error) {
	pv, err := s.repo.FindByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if pv == nil {
		return nil, apperror.NotFound("Kerja sama vendor tidak ditemukan")
	}
	return pv, nil
}

type VendorEngagementInput struct {
	VendorID         int64
	CategoryID       int64
	Scope            string
	ContractValue    int64
	PricingTier      domain.VendorPricingTier
	EngagementStatus domain.EngagementStatus
	BookingDate      *time.Time
	EventDate        time.Time
	EventStartTime   *string
	EventEndTime     *string
	DPAmount         int64
	DueDate          *time.Time
	PICStaffID       int64
	Notes            string
	// OverBudgetReason hanya dibaca saat komitmen ini membuat total biaya
	// melampaui nilai kontrak — lihat guardBudget.
	OverBudgetReason string
}

func (s *VendorEngagementService) Create(ctx context.Context, tenantID, projectID int64, actorStaffID int64, input VendorEngagementInput) (*domain.ProjectVendor, error) {
	if err := validateEngagementInput(input); err != nil {
		return nil, err
	}
	// Gerbang SEBELUM menulis: penolakan 422 tidak boleh meninggalkan baris
	// yang sudah tersimpan, idiom yang sama dipakai validateTotal di
	// quotations.
	if err := s.guardBudget(ctx, budgetCheck{
		tenantID: tenantID, projectID: projectID, actorStaffID: actorStaffID,
		newContract: input.ContractValue, newStatus: input.EngagementStatus,
		reason: input.OverBudgetReason,
	}); err != nil {
		return nil, err
	}
	pv := &domain.ProjectVendor{
		ProjectID: projectID, VendorID: input.VendorID, CategoryID: input.CategoryID, Scope: input.Scope,
		ContractValue: input.ContractValue, PricingTier: input.PricingTier, EngagementStatus: input.EngagementStatus, BookingDate: input.BookingDate,
		EventDate: input.EventDate, EventStartTime: input.EventStartTime, EventEndTime: input.EventEndTime,
		DPAmount: input.DPAmount, DueDate: input.DueDate,
		PICStaffID: input.PICStaffID, Notes: input.Notes,
	}
	if err := s.repo.Create(ctx, pv); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityVendorAdded, actorStaffID, "project_vendor", formatID(pv.ID), truncateTo(input.Scope, maxActivityLabel),
		"Vendor ditambahkan ke project")
	return pv, nil
}

// Update takes callerRole to decide whether ContractValue/DPAmount get
// overwritten from input -- confirmed role rule, PLAN.md
// mom-25082026-item-sebagian §12b: a non-Owner/Admin caller's form never
// renders those two fields at all, so input always carries a zero value for
// them. Silently keeping pv's stored values instead of overwriting is
// deliberate (D1 in the plan) -- unlike ProjectService.Update's
// guardKonteksUmum, which rejects the whole request, this preserves the
// caller's other edits (scope, jam acara, status) rather than blocking them
// over fields the form never gave them a way to send correctly.
// callerRole == "" (or any other non-Staff/Sales value) behaves exactly as
// before: both fields overwrite freely.
func (s *VendorEngagementService) Update(ctx context.Context, tenantID, projectID, id int64, actorStaffID int64, callerRole string, input VendorEngagementInput) (*domain.ProjectVendor, error) {
	pv, err := s.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	// Nilai kontrak yang BENAR-BENAR akan tersimpan: peran Staff/Sales tidak
	// pernah mengirimnya (lihat komentar di atas), jadi yang lama yang
	// bertahan — gerbangnya harus menghitung angka itu, bukan nol dari form.
	nextContract := input.ContractValue
	if callerRole == "Staff" || callerRole == "Sales" {
		nextContract = pv.ContractValue
	}
	// Bentuk masukan diperiksa SEBELUM gerbang anggaran: status yang tidak
	// dikenal membuat "apakah komitmen ini dihitung" tidak terjawab, dan
	// gerbangnya akan menebak.
	if err := validateEngagementInput(input); err != nil {
		return nil, err
	}
	if err := s.guardBudget(ctx, budgetCheck{
		tenantID: tenantID, projectID: projectID, actorStaffID: actorStaffID,
		excludeID: pv.ID, newContract: nextContract, newStatus: input.EngagementStatus,
		reason: input.OverBudgetReason,
	}); err != nil {
		return nil, err
	}
	pv.VendorID = input.VendorID
	pv.CategoryID = input.CategoryID
	pv.Scope = input.Scope
	if callerRole != "Staff" && callerRole != "Sales" {
		pv.ContractValue = input.ContractValue
		pv.DPAmount = input.DPAmount
	}
	pv.PricingTier = input.PricingTier
	pv.EngagementStatus = input.EngagementStatus
	pv.BookingDate = input.BookingDate
	pv.EventDate = input.EventDate
	pv.EventStartTime = input.EventStartTime
	pv.EventEndTime = input.EventEndTime
	pv.DueDate = input.DueDate
	pv.PICStaffID = input.PICStaffID
	pv.Notes = input.Notes
	if err := s.repo.Update(ctx, pv); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityVendorStatusChanged, actorStaffID, "project_vendor", formatID(pv.ID), truncateTo(pv.Scope, maxActivityLabel),
		"Informasi kerja sama vendor diperbarui")
	return pv, nil
}

// Batas panjang teks yang ditegakkan sebelum menyentuh DB.
//
// maxScope bukan batas kolom — sejak migrasi 000072 `scope` adalah TEXT
// (65.535 byte) — melainkan batas produk: cukup untuk scope sepanjang
// paragraf, sekaligus menjadikan penolakan sebuah 422 yang bisa dirender
// form alih-alih 500 di tebing TEXT. maxReason menjaga activity_log.
// description (VARCHAR(500)), yang dirangkai guardBudget dari alasan ini
// plus prefiks ±77 karakter.
const (
	maxScope  = 2000
	maxReason = 400
)

// validateEngagementInput rejects values the database itself would refuse,
// atau yang diam-diam merusak jejak aktivitas. Empat penjaga:
//
//  1. engagement_status — MySQL ENUM, so without this check an unknown
//     status sails through to repo.Create/repo.Update and comes back as a
//     bare 500 ("Terjadi kesalahan pada server") instead of a 422 the
//     frontend can render under the field.
//  2. pricing_tier — plain VARCHAR (no DB guard at all), so the same helper
//     also keeps typos there from silently persisting as free text.
//  3. panjang scope & alasan over-budget — kegagalan yang SAMA BENTUKNYA
//     dengan (1): sebelum migrasi 000072, scope > 255 karakter adalah
//     satu-satunya sumber 500 di produksi (56 dari 56 dalam 45 jam, lihat
//     docs/plan/vendor-engagement-500/PLAN.md §3.3).
//  4. format jam acara — lihat validateHHMM.
//
// Mirrors validateTerms' shape in the quotations module: one helper,
// field-keyed apperror.Validation, called from both Create and Update.
func validateEngagementInput(input VendorEngagementInput) error {
	status, tier := input.EngagementStatus, input.PricingTier
	switch status {
	case domain.EngagementPlanned, domain.EngagementNegotiation, domain.EngagementBooked,
		domain.EngagementDPPaid, domain.EngagementInProgress, domain.EngagementFullyPaid,
		domain.EngagementReady, domain.EngagementCompleted, domain.EngagementCancelled:
	default:
		return apperror.Validation("Status kerja sama vendor tidak valid", map[string][]string{
			"engagementStatus": {"Status kerja sama vendor tidak valid"},
		})
	}
	switch tier {
	case domain.PricingTierAkad, domain.PricingTierAkadResepsi,
		domain.PricingTierResepsi, domain.PricingTierCustom:
	default:
		return apperror.Validation("Tier harga vendor tidak valid", map[string][]string{
			"pricingTier": {"Tier harga vendor tidak valid"},
		})
	}
	if len([]rune(input.Scope)) > maxScope {
		return apperror.Validation("Scope pekerjaan terlalu panjang", map[string][]string{
			"scope": {"Maksimal 2000 karakter"},
		})
	}
	if len([]rune(input.OverBudgetReason)) > maxReason {
		return apperror.Validation("Alasan terlalu panjang", map[string][]string{
			"overBudgetReason": {"Maksimal 400 karakter"},
		})
	}
	if err := validateHHMM("eventStartTime", input.EventStartTime); err != nil {
		return err
	}
	return validateHHMM("eventEndTime", input.EventEndTime)
}

// validateHHMM menjaga kolom TIME dari string sembarang: tanpa ini, "19.00"
// atau "" lolos ke repo.Create dan kembali sebagai 500 telanjang, persis
// kegagalan yang sudah dijelaskan doc comment validateEngagementInput untuk
// ENUM. Repository hanya punya arah baca (timeToHHMM); ini pasangan arah
// tulisnya. nil berarti "Belum ditentukan" dan selalu sah.
func validateHHMM(field string, v *string) error {
	if v == nil {
		return nil
	}
	if _, err := time.Parse("15:04", *v); err != nil {
		return apperror.Validation("Format jam tidak valid", map[string][]string{
			field: {"Gunakan format HH:MM"},
		})
	}
	return nil
}

// validateMilestoneStatus guards the same ENUM-at-the-DB hazard as
// validateEngagementInput above for both milestone tables
// (project_milestones.status, vendor_milestones.status). Shared by
// ProjectService.UpdateMilestone and VendorEngagementService.UpdateMilestone
// (same package) so the two sibling entities can never drift apart.
func validateMilestoneStatus(status domain.MilestoneStatus) error {
	switch status {
	case domain.MilestoneNotStarted, domain.MilestoneInProgress, domain.MilestoneCompleted,
		domain.MilestoneBlocked, domain.MilestoneCancelled:
		return nil
	default:
		return apperror.Validation("Status timeline tidak valid", map[string][]string{
			"status": {"Status timeline tidak valid"},
		})
	}
}

// ListHistoryForVendor backs the `vendors` module's "Lihat Project" feature
// via `contracts.Contracts.ListVendorEngagementHistory` — see that method's
// doc comment for why this lives here rather than in `vendors`.
func (s *VendorEngagementService) ListHistoryForVendor(ctx context.Context, tenantID, vendorID int64) ([]domain.VendorEngagementHistoryRow, error) {
	return s.repo.ListByVendor(ctx, tenantID, vendorID)
}

func (s *VendorEngagementService) Cancel(ctx context.Context, projectID, id int64, actorStaffID int64) (*domain.ProjectVendor, error) {
	pv, err := s.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetStatus(ctx, projectID, id, domain.EngagementCancelled); err != nil {
		return nil, err
	}
	pv.EngagementStatus = domain.EngagementCancelled
	s.activity.Record(ctx, &projectID, domain.ActivityVendorStatusChanged, actorStaffID, "project_vendor", formatID(pv.ID), truncateTo(pv.Scope, maxActivityLabel),
		"Kerja sama vendor dibatalkan")
	return pv, nil
}

// --- Vendor milestones ---

type VendorMilestoneInput struct {
	Name        string
	Description string
	TargetDate  time.Time
	PICStaffID  int64
}

func (s *VendorEngagementService) ListMilestones(ctx context.Context, projectID, projectVendorID int64) ([]domain.VendorMilestone, error) {
	if _, err := s.Get(ctx, projectID, projectVendorID); err != nil {
		return nil, err
	}
	return s.milestones.ListByProjectVendor(ctx, projectVendorID)
}

// ListMilestonesByProjectVendors backs listVendorEngagements: one query
// across every given engagement id instead of the frontend's own
// per-engagement Promise.all loop (PLAN.md "Performance remediation",
// Phase E). Returns a map so the caller can attach each engagement's own
// milestones to its response row.
func (s *VendorEngagementService) ListMilestonesByProjectVendors(ctx context.Context, projectVendorIDs []int64) (map[int64][]domain.VendorMilestone, error) {
	all, err := s.milestones.ListByProjectVendors(ctx, projectVendorIDs)
	if err != nil {
		return nil, err
	}
	byEngagement := make(map[int64][]domain.VendorMilestone, len(projectVendorIDs))
	for _, m := range all {
		byEngagement[m.ProjectVendorID] = append(byEngagement[m.ProjectVendorID], m)
	}
	return byEngagement, nil
}

func (s *VendorEngagementService) CreateMilestone(ctx context.Context, projectID, projectVendorID int64, actorStaffID int64, input VendorMilestoneInput) (*domain.VendorMilestone, error) {
	if _, err := s.Get(ctx, projectID, projectVendorID); err != nil {
		return nil, err
	}
	order, err := s.milestones.NextSortOrder(ctx, projectVendorID)
	if err != nil {
		return nil, err
	}
	m := &domain.VendorMilestone{
		ProjectVendorID: projectVendorID, SortOrder: order, Name: input.Name, Description: input.Description,
		Status: domain.MilestoneNotStarted, TargetDate: input.TargetDate, PICStaffID: input.PICStaffID,
	}
	if err := s.milestones.Create(ctx, m); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityMilestoneUpdated, actorStaffID, "vendor_milestone", formatID(m.ID), m.Name,
		"Timeline vendor ditambahkan: "+m.Name)
	return m, nil
}

type VendorMilestoneUpdateInput struct {
	Status        domain.MilestoneStatus
	TargetDate    time.Time
	CompletedDate *time.Time
	PICStaffID    int64
	Description   string
	Notes         string
}

func (s *VendorEngagementService) UpdateMilestone(ctx context.Context, projectID, projectVendorID, milestoneID int64, actorStaffID int64, input VendorMilestoneUpdateInput) (*domain.VendorMilestone, error) {
	if _, err := s.Get(ctx, projectID, projectVendorID); err != nil {
		return nil, err
	}
	m, err := s.milestones.FindByID(ctx, projectVendorID, milestoneID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperror.NotFound("Timeline vendor tidak ditemukan")
	}
	if err := validateMilestoneStatus(input.Status); err != nil {
		return nil, err
	}
	m.Status = input.Status
	m.TargetDate = input.TargetDate
	m.CompletedDate = input.CompletedDate
	m.PICStaffID = input.PICStaffID
	m.Description = input.Description
	m.Notes = input.Notes
	if err := s.milestones.Update(ctx, m); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityMilestoneUpdated, actorStaffID, "vendor_milestone", formatID(m.ID), m.Name,
		"Timeline vendor diperbarui")
	return m, nil
}
