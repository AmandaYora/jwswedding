package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
)

// fakeVendorEngagementRepo is a minimal in-memory stand-in for
// VendorEngagementRepository -- good enough to exercise Update's control
// flow (PLAN.md mom-25082026-item-sebagian §7.1, item 12) without a real
// database. Create records instead of panicking: validation happens before
// any repo call, so reaching it with a bad tier still surfaces as err == nil
// (and assertValidation then fails the test for the right reason).
type fakeVendorEngagementRepo struct {
	engagement *domain.ProjectVendor
	updated    *domain.ProjectVendor
	created    *domain.ProjectVendor
}

func (f *fakeVendorEngagementRepo) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectVendor, error) {
	panic("not implemented")
}
func (f *fakeVendorEngagementRepo) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectVendor, error) {
	panic("not implemented")
}
func (f *fakeVendorEngagementRepo) FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectVendor, error) {
	return f.engagement, nil
}
func (f *fakeVendorEngagementRepo) Create(ctx context.Context, pv *domain.ProjectVendor) error {
	f.created = pv
	return nil
}
func (f *fakeVendorEngagementRepo) Update(ctx context.Context, pv *domain.ProjectVendor) error {
	f.updated = pv
	return nil
}
func (f *fakeVendorEngagementRepo) SetStatus(ctx context.Context, projectID, id int64, status domain.EngagementStatus) error {
	panic("not implemented")
}
func (f *fakeVendorEngagementRepo) ListByVendor(ctx context.Context, tenantID, vendorID int64) ([]domain.VendorEngagementHistoryRow, error) {
	panic("not implemented")
}

func baseEngagement() *domain.ProjectVendor {
	return &domain.ProjectVendor{
		ID: 1, ProjectID: 10, VendorID: 20, CategoryID: 3,
		Scope: "Sewa ballroom + basic lighting rigging", ContractValue: 5_000_000,
		PricingTier: domain.PricingTierAkad, EngagementStatus: domain.EngagementBooked,
		EventDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		DPAmount:  1_000_000, PICStaffID: 5, Notes: "Catatan awal",
	}
}

func baseEngagementInputFor(pv *domain.ProjectVendor) VendorEngagementInput {
	return VendorEngagementInput{
		VendorID: pv.VendorID, CategoryID: pv.CategoryID, Scope: pv.Scope,
		ContractValue: pv.ContractValue, PricingTier: pv.PricingTier, EngagementStatus: pv.EngagementStatus,
		BookingDate: pv.BookingDate, EventDate: pv.EventDate, EventStartTime: pv.EventStartTime, EventEndTime: pv.EventEndTime,
		DPAmount: pv.DPAmount, DueDate: pv.DueDate, PICStaffID: pv.PICStaffID, Notes: pv.Notes,
	}
}

func newVendorEngagementServiceForTest(pv *domain.ProjectVendor) *VendorEngagementService {
	return &VendorEngagementService{
		repo:     &fakeVendorEngagementRepo{engagement: pv},
		activity: NewActivityService(&fakeActivityRepoForProject{}),
	}
}

// A Wedding Planner's form never renders Nilai Kerja Sama at all (PLAN.md
// §12a), so its submission always carries ContractValue's zero value --
// Update must keep the stored figure instead of overwriting it with that
// zero, or an Admin-entered contract value silently disappears the moment a
// WP edits anything else on the same engagement (§12b's whole reason for
// existing).
func TestVendorEngagementUpdate_StaffContractValueZero_NilaiTersimpanTetapUtuh(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.ContractValue = 0
	input.Scope = "Scope diperbarui oleh WP"
	got, err := svc.Update(context.Background(), 1, pv.ProjectID, pv.ID, 99, "Staff", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.ContractValue != 5_000_000 {
		t.Errorf("ContractValue = %d, want tetap 5000000 (nilai Admin tidak boleh hilang)", got.ContractValue)
	}
	if got.Scope != "Scope diperbarui oleh WP" {
		t.Errorf("Scope = %q, want %q (edit lain WP tetap tersimpan)", got.Scope, "Scope diperbarui oleh WP")
	}
}

func TestVendorEngagementUpdate_StaffDPAmountZero_NilaiTersimpanTetapUtuh(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.DPAmount = 0
	got, err := svc.Update(context.Background(), 1, pv.ProjectID, pv.ID, 99, "Sales", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.DPAmount != 1_000_000 {
		t.Errorf("DPAmount = %d, want tetap 1000000 (nilai Admin tidak boleh hilang)", got.DPAmount)
	}
}

func TestVendorEngagementUpdate_Owner_ContractValueBerubah_Tersimpan(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.ContractValue = 7_000_000
	got, err := svc.Update(context.Background(), 1, pv.ProjectID, pv.ID, 99, "Owner", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.ContractValue != 7_000_000 {
		t.Errorf("ContractValue = %d, want 7000000 (Owner boleh mengubah)", got.ContractValue)
	}
}

func assertValidation(t *testing.T, err error, field string) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
		t.Fatalf("error = %v, want a Validation apperror (HTTP 422)", err)
	}
	if _, ok := appErr.Fields[field]; !ok {
		t.Errorf("error fields = %v, want key %q agar frontend bisa menempelkannya di field", appErr.Fields, field)
	}
}

// Regression: engagement_status is a MySQL ENUM — an unknown status used to
// sail through to the INSERT/UPDATE and come back as a bare 500. The service
// must reject it with a field-keyed 422 before touching the repository
// (a skipped validation would surface here as err == nil, which
// assertValidation rejects).
func TestVendorEngagementCreate_StatusInvalid_Ditolak422(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.EngagementStatus = "Confirmed"

	_, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input)
	assertValidation(t, err, "engagementStatus")
}

func TestVendorEngagementUpdate_TierInvalid_Ditolak422(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.PricingTier = "VIP"

	_, err := svc.Update(context.Background(), 1, pv.ProjectID, pv.ID, 99, "Owner", input)
	assertValidation(t, err, "pricingTier")
}

// Custom (ADR-0034, D1) adalah paket di luar ketiga preset harga vendor —
// server wajib menerimanya pada Create maupun Update, dengan nilai tersimpan
// apa adanya.
func TestVendorEngagementCreate_TierCustom_Diterima(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.PricingTier = domain.PricingTierCustom
	input.ContractValue = 12_500_000

	got, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input)
	if err != nil {
		t.Fatalf("Create() error = %v, want success untuk tier Custom", err)
	}
	if got.PricingTier != domain.PricingTierCustom {
		t.Errorf("PricingTier = %q, want %q", got.PricingTier, domain.PricingTierCustom)
	}
	if got.ContractValue != 12_500_000 {
		t.Errorf("ContractValue = %d, want 12500000 (nilai manual Custom wajib tersimpan)", got.ContractValue)
	}
}

func TestVendorEngagementUpdate_TierCustom_Diterima(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.PricingTier = domain.PricingTierCustom

	got, err := svc.Update(context.Background(), 1, pv.ProjectID, pv.ID, 99, "Owner", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success untuk tier Custom", err)
	}
	if got.PricingTier != domain.PricingTierCustom {
		t.Errorf("PricingTier = %q, want %q", got.PricingTier, domain.PricingTierCustom)
	}
}

func TestVendorEngagementUpdateMilestone_StatusInvalid_Ditolak422(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	svc.milestones = &stubVendorMilestoneRepoForValidationTest{}
	_, err := svc.UpdateMilestone(context.Background(), pv.ProjectID, pv.ID, 1, 99, VendorMilestoneUpdateInput{
		Status:     "Confirmed",
		TargetDate: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	})
	assertValidation(t, err, "status")
}

// stubVendorMilestoneRepoForValidationTest exists only so the milestone
// existence check passes and the test actually reaches the status
// validation under test.
type stubVendorMilestoneRepoForValidationTest struct{}

func (stubVendorMilestoneRepoForValidationTest) ListByProjectVendor(ctx context.Context, projectVendorID int64) ([]domain.VendorMilestone, error) {
	panic("not implemented")
}
func (stubVendorMilestoneRepoForValidationTest) ListByProjectVendors(ctx context.Context, projectVendorIDs []int64) ([]domain.VendorMilestone, error) {
	panic("not implemented")
}
func (stubVendorMilestoneRepoForValidationTest) FindByID(ctx context.Context, projectVendorID, id int64) (*domain.VendorMilestone, error) {
	return &domain.VendorMilestone{ID: id, ProjectVendorID: projectVendorID, Status: domain.MilestoneNotStarted}, nil
}
func (stubVendorMilestoneRepoForValidationTest) Create(ctx context.Context, m *domain.VendorMilestone) error {
	panic("not implemented")
}
func (stubVendorMilestoneRepoForValidationTest) Update(ctx context.Context, m *domain.VendorMilestone) error {
	panic("not implemented")
}
func (stubVendorMilestoneRepoForValidationTest) NextSortOrder(ctx context.Context, projectVendorID int64) (int, error) {
	panic("not implemented")
}

// --- Gerbang anggaran (guardBudget) ------------------------------------
//
// Aturan yang dikunci di sini bukan "biaya tidak boleh melampaui nilai
// kontrak", melainkan "tidak boleh melampaui tanpa ada yang menyadarinya dan
// bertanggung jawab" -- jadi yang diuji adalah kapan gerbangnya menolak,
// kapan ia meloloskan sambil mencatat, dan kapan ia tidak berbunyi sama
// sekali.

type fakeBudgetReader struct {
	summary domain.ProjectCostSummary
}

func (f *fakeBudgetReader) CostSummary(ctx context.Context, tenantID, projectID int64) (domain.ProjectCostSummary, error) {
	return f.summary, nil
}

// countingActivityRepo menangkap apa yang tercatat. Gerbang ini meloloskan
// komitmen yang melampaui batas HANYA dengan meninggalkan jejak, jadi
// jejaknya ikut diuji, bukan cuma nilai kembaliannya.
type countingActivityRepo struct {
	entries []*domain.ActivityLogEntry
}

func (r *countingActivityRepo) Create(ctx context.Context, entry *domain.ActivityLogEntry) error {
	r.entries = append(r.entries, entry)
	return nil
}

func (r *countingActivityRepo) ListByProject(ctx context.Context, projectID int64, limit int) ([]domain.ActivityLogEntry, error) {
	return nil, nil
}

// budgetRepo melengkapi fakeVendorEngagementRepo dengan ListByProject -- yang
// dibaca gerbang untuk tahu berapa yang sudah terkomitmen sebelum tulisan ini.
type budgetRepo struct {
	fakeVendorEngagementRepo
	existing []domain.ProjectVendor
	created  *domain.ProjectVendor
}

func (r *budgetRepo) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectVendor, error) {
	return r.existing, nil
}

func (r *budgetRepo) Create(ctx context.Context, pv *domain.ProjectVendor) error {
	pv.ID = 99
	r.created = pv
	return nil
}

// newGuardedServiceForTest merakit layanan dengan gerbang terpasang: nilai
// kontrak 100jt, venue 10jt, dan satu engagement 50jt yang sudah berjalan --
// jadi ruang yang tersisa persis 40jt.
func newGuardedServiceForTest() (*VendorEngagementService, *budgetRepo, *countingActivityRepo) {
	repo := &budgetRepo{existing: []domain.ProjectVendor{
		{ID: 1, ProjectID: 10, ContractValue: 50_000_000, EngagementStatus: domain.EngagementBooked},
	}}
	repo.fakeVendorEngagementRepo.engagement = &domain.ProjectVendor{
		ID: 1, ProjectID: 10, VendorID: 20, CategoryID: 3, Scope: "Katering",
		ContractValue: 50_000_000, PricingTier: domain.PricingTierAkad,
		EngagementStatus: domain.EngagementBooked,
		EventDate:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	acts := &countingActivityRepo{}
	svc := &VendorEngagementService{repo: repo, activity: NewActivityService(acts)}
	svc.SetBudgetReader(&fakeBudgetReader{summary: domain.ProjectCostSummary{
		ContractValue: 100_000_000, VendorCost: 50_000_000, VenueCost: 10_000_000,
		CommittedCost: 60_000_000, Remaining: 40_000_000,
	}})
	return svc, repo, acts
}

func guardedInput(contractValue int64) VendorEngagementInput {
	return VendorEngagementInput{
		VendorID: 21, CategoryID: 4, Scope: "Dekorasi", ContractValue: contractValue,
		PricingTier: domain.PricingTierAkad, EngagementStatus: domain.EngagementBooked,
		EventDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), PICStaffID: 5,
	}
}

func TestGuardBudget_Create_MelampauiTanpaAlasan_Ditolak422(t *testing.T) {
	svc, repo, _ := newGuardedServiceForTest()

	_, err := svc.Create(context.Background(), 1, 10, 99, guardedInput(45_000_000))

	assertValidation(t, err, "overBudgetReason")
	// Angka pelampauannya ikut dikirim supaya layar bisa menyebutnya tanpa
	// menghitung ulang: 50 + 45 + 10 venue = 105, melampaui 100 sebesar 5jt.
	var appErr *apperror.AppError
	errors.As(err, &appErr)
	if got := appErr.Fields["overage"]; len(got) != 1 || got[0] != "5000000" {
		t.Errorf("overage = %v, want [5000000]", got)
	}
	if repo.created != nil {
		t.Error("engagement tersimpan padahal gerbang menolak -- penolakan 422 tidak boleh meninggalkan baris")
	}
}

func TestGuardBudget_Create_MelampauiDenganAlasan_LolosDanTercatat(t *testing.T) {
	svc, repo, acts := newGuardedServiceForTest()
	input := guardedInput(45_000_000)
	input.OverBudgetReason = "Ditanggung sendiri, klien sudah menolak tambahan biaya"

	got, err := svc.Create(context.Background(), 1, 10, 99, input)
	if err != nil {
		t.Fatalf("Create() error = %v, want lolos setelah alasan diisi", err)
	}
	if got == nil || repo.created == nil {
		t.Fatal("engagement tidak tersimpan padahal alasannya sudah diisi")
	}
	if len(acts.entries) == 0 {
		t.Fatal("tidak ada aktivitas tercatat -- pelampauan tanpa jejak persis yang dicegah aturan ini")
	}
	e := acts.entries[0]
	if e.Type != domain.ActivityVendorOverBudget {
		t.Errorf("Type = %q, want %q", e.Type, domain.ActivityVendorOverBudget)
	}
	if e.ActorStaffID != 99 {
		t.Errorf("ActorStaffID = %d, want 99 (siapa yang menyetujui harus ikut tercatat)", e.ActorStaffID)
	}
	if !strings.Contains(e.Description, input.OverBudgetReason) {
		t.Errorf("Description = %q, want memuat alasan yang diketik", e.Description)
	}
	if !strings.Contains(e.Description, "Rp 5.000.000") {
		t.Errorf("Description = %q, want menyebut angka pelampauan Rp 5.000.000", e.Description)
	}
}

func TestGuardBudget_Create_MasihDalamAnggaran_TidakMengusik(t *testing.T) {
	svc, repo, acts := newGuardedServiceForTest()

	if _, err := svc.Create(context.Background(), 1, 10, 99, guardedInput(40_000_000)); err != nil {
		t.Fatalf("Create() error = %v, want lolos -- 40jt pas menghabiskan sisa, belum melampaui", err)
	}
	if repo.created == nil {
		t.Fatal("engagement tidak tersimpan")
	}
	// Create selalu mencatat aktivitas "vendor ditambahkan"; yang TIDAK boleh
	// muncul adalah catatan pelampauan anggaran.
	for _, e := range acts.entries {
		if e.Type == domain.ActivityVendorOverBudget {
			t.Errorf("tercatat %q padahal komitmennya masih di dalam anggaran", e.Type)
		}
	}
}

// Tanpa excludeID, memperbarui satu engagement akan menghitung nilai LAMA dan
// nilai BARUnya sekaligus -- mengubah 50jt jadi 55jt akan terbaca sebagai
// 105jt dan ditolak, padahal sisanya masih cukup.
func TestGuardBudget_Update_NilaiLamaTidakDihitungDobel(t *testing.T) {
	svc, _, _ := newGuardedServiceForTest()

	got, err := svc.Update(context.Background(), 1, 10, 1, 99, "Owner", guardedInput(55_000_000))
	if err != nil {
		t.Fatalf("Update() error = %v, want lolos -- 55jt + 10jt venue masih di bawah 100jt", err)
	}
	if got.ContractValue != 55_000_000 {
		t.Errorf("ContractValue = %d, want 55000000", got.ContractValue)
	}
}

// Engagement yang dibatalkan bukan biaya. Menyimpan angka berapa pun sebagai
// Cancelled tidak boleh membangunkan gerbangnya.
func TestGuardBudget_Cancelled_TidakDihitungSebagaiBiaya(t *testing.T) {
	svc, _, _ := newGuardedServiceForTest()
	input := guardedInput(500_000_000)
	input.EngagementStatus = domain.EngagementCancelled

	if _, err := svc.Create(context.Background(), 1, 10, 99, input); err != nil {
		t.Fatalf("Create() error = %v, want lolos -- engagement Cancelled bukan komitmen biaya", err)
	}
}

// Tanpa gerbang terpasang, jalur tulis tidak boleh membayar satu pembacaan
// daftar pun. fakeVendorEngagementRepo.ListByProject sengaja panic: sampai ke
// sana berarti urutan keluar-awal di guardBudget tergeser.
func TestGuardBudget_TidakTerpasang_TidakMembacaApaPun(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)

	if _, err := svc.Update(context.Background(), 1, pv.ProjectID, pv.ID, 99, "Owner", baseEngagementInputFor(pv)); err != nil {
		t.Fatalf("Update() error = %v, want lolos tanpa gerbang", err)
	}
}

// newOverBudgetServiceForTest merakit project yang SUDAH melampaui sebelum
// tulisan apa pun: kontrak 100jt, venue 10jt, satu engagement 120jt — minus
// 30jt.
func newOverBudgetServiceForTest() (*VendorEngagementService, *countingActivityRepo) {
	repo := &budgetRepo{existing: []domain.ProjectVendor{
		{ID: 1, ProjectID: 10, ContractValue: 120_000_000, EngagementStatus: domain.EngagementBooked},
	}}
	repo.fakeVendorEngagementRepo.engagement = &domain.ProjectVendor{
		ID: 1, ProjectID: 10, VendorID: 20, CategoryID: 3, Scope: "Katering",
		ContractValue: 120_000_000, PricingTier: domain.PricingTierAkad,
		EngagementStatus: domain.EngagementBooked,
		EventDate:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	acts := &countingActivityRepo{}
	svc := &VendorEngagementService{repo: repo, activity: NewActivityService(acts)}
	svc.SetBudgetReader(&fakeBudgetReader{summary: domain.ProjectCostSummary{
		ContractValue: 100_000_000, VendorCost: 120_000_000, VenueCost: 10_000_000,
		CommittedCost: 130_000_000, Remaining: -30_000_000,
	}})
	return svc, acts
}

// Sebuah project yang terlanjur minus tidak boleh meminta alasan pada SETIAP
// penyuntingan vendor sesudahnya. Yang perlu diakui adalah pelampauan yang
// dibuat atau diperdalam, bukan yang sedang ditinggalkan apa adanya.
func TestGuardBudget_SudahMinus_EditTanpaMenambahBiaya_Lolos(t *testing.T) {
	svc, acts := newOverBudgetServiceForTest()
	input := guardedInput(120_000_000)
	input.Scope = "Scope diperbarui, nilainya tidak"

	if _, err := svc.Update(context.Background(), 1, 10, 1, 99, "Owner", input); err != nil {
		t.Fatalf("Update() error = %v, want lolos tanpa alasan -- beban tidak bertambah", err)
	}
	for _, e := range acts.entries {
		if e.Type == domain.ActivityVendorOverBudget {
			t.Error("tercatat pelampauan baru padahal tidak ada yang bertambah")
		}
	}
}

// Menurunkan nilai pada project yang minus adalah perbaikan; memperlakukannya
// sebagai pelanggaran akan menghukum satu-satunya tindakan yang benar.
func TestGuardBudget_SudahMinus_NilaiDiturunkan_Lolos(t *testing.T) {
	svc, _ := newOverBudgetServiceForTest()

	got, err := svc.Update(context.Background(), 1, 10, 1, 99, "Owner", guardedInput(80_000_000))
	if err != nil {
		t.Fatalf("Update() error = %v, want lolos -- nilainya justru diturunkan", err)
	}
	if got.ContractValue != 80_000_000 {
		t.Errorf("ContractValue = %d, want 80000000", got.ContractValue)
	}
}

// Memperdalam minus tetap butuh pengakuan.
func TestGuardBudget_SudahMinus_DiperdalamTanpaAlasan_Ditolak422(t *testing.T) {
	svc, _ := newOverBudgetServiceForTest()

	_, err := svc.Update(context.Background(), 1, 10, 1, 99, "Owner", guardedInput(150_000_000))

	assertValidation(t, err, "overBudgetReason")
	var appErr *apperror.AppError
	errors.As(err, &appErr)
	// Angkanya adalah pelampauan TOTAL (150 + 10 - 100 = 60jt), bukan
	// pertambahannya -- itu yang harus ditandatangani orangnya.
	if got := appErr.Fields["overage"]; len(got) != 1 || got[0] != "60000000" {
		t.Errorf("overage = %v, want [60000000]", got)
	}
}
