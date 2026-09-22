package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

// fakeBudgetReader kini hanya memasok BASIS biaya (nilai kontrak + venue).
// Biaya vendor tidak lagi disuntik dari sini: guardBudget menghitungnya dari
// daftar engagement yang sudah dibacanya sendiri (budgetRepo.existing), yang
// juga menghapus satu query duplikat -- lihat PLAN.md T6.
type fakeBudgetReader struct {
	basis domain.ProjectCostBasis
}

func (f *fakeBudgetReader) CostBasis(ctx context.Context, tenantID, projectID int64) (domain.ProjectCostBasis, error) {
	return f.basis, nil
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
	// listCalls menghitung pembacaan daftar engagement. Sebelum PLAN.md T6,
	// satu Create membayar query ini DUA kali: sekali di guardBudget, sekali
	// lagi dari dalam CostSummary yang dipanggilnya.
	listCalls int
}

func (r *budgetRepo) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectVendor, error) {
	r.listCalls++
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
	// VendorCost 50jt tidak lagi disuntik di sini -- ia lahir dari
	// repo.existing di atas, dan totalnya tetap sama: 50jt + 10jt venue = 60jt
	// terpakai dari 100jt, sisa 40jt.
	svc.SetBudgetReader(&fakeBudgetReader{basis: domain.ProjectCostBasis{
		ContractValue: 100_000_000, VenueCost: 10_000_000,
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
	// Sama seperti di atas: VendorCost 120jt datang dari repo.existing, jadi
	// 120jt + 10jt venue = 130jt terpakai dari 100jt -- project ini sudah minus
	// 30jt sebelum tulisan apa pun.
	svc.SetBudgetReader(&fakeBudgetReader{basis: domain.ProjectCostBasis{
		ContractValue: 100_000_000, VenueCost: 10_000_000,
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

// --- Batas panjang teks & format jam (PLAN.md vendor-engagement-500, T4) ---
//
// Regresi atas satu-satunya sumber 500 di produksi: 56 dari 56 `unhandled
// error` dalam 45 jam adalah "Error 1406 Data too long for column 'scope'".
// Kolomnya kini TEXT (migrasi 000072), tapi gerbang panjangnya tetap ada
// supaya penolakan berupa 422 yang bisa dirender form, bukan 500 di tebing
// 65.535 byte milik TEXT.

func repeatRunes(r rune, n int) string {
	return strings.Repeat(string(r), n)
}

func TestVendorEngagementCreate_ScopeTerlaluPanjang_Ditolak422(t *testing.T) {
	pv := baseEngagement()
	repo := &fakeVendorEngagementRepo{engagement: pv}
	svc := &VendorEngagementService{repo: repo, activity: NewActivityService(&fakeActivityRepoForProject{})}
	input := baseEngagementInputFor(pv)
	input.Scope = repeatRunes('a', maxScope+1)

	_, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input)
	assertValidation(t, err, "scope")
	if repo.created != nil {
		t.Error("repo.Create terpanggil padahal validasi menolak — gerbangnya harus SEBELUM tulisan")
	}
}

func TestVendorEngagementCreate_ScopeTepatDiBatas_Diterima(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.Scope = repeatRunes('a', maxScope)

	got, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil pada panjang tepat batas", err)
	}
	if len([]rune(got.Scope)) != maxScope {
		t.Errorf("Scope tersimpan %d rune, want %d — tidak boleh dipangkas diam-diam", len([]rune(got.Scope)), maxScope)
	}
}

// Regresi langsung atas root cause: 300 karakter dulu menghasilkan 500 karena
// kolomnya VARCHAR(255). Sekarang harus tersimpan utuh.
func TestVendorEngagementCreate_ScopeDiAtas255_Diterima(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.Scope = repeatRunes('a', 300)

	got, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil — 300 karakter adalah kasus yang dulu 500", err)
	}
	if len([]rune(got.Scope)) != 300 {
		t.Errorf("Scope tersimpan %d rune, want 300", len([]rune(got.Scope)))
	}
}

func TestVendorEngagementCreate_AlasanOverBudgetTerlaluPanjang_Ditolak422(t *testing.T) {
	pv := baseEngagement()
	repo := &fakeVendorEngagementRepo{engagement: pv}
	svc := &VendorEngagementService{repo: repo, activity: NewActivityService(&fakeActivityRepoForProject{})}
	input := baseEngagementInputFor(pv)
	input.OverBudgetReason = repeatRunes('a', maxReason+1)

	_, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input)
	assertValidation(t, err, "overBudgetReason")
	if repo.created != nil {
		t.Error("repo.Create terpanggil padahal validasi menolak")
	}
}

func TestVendorEngagementCreate_JamAcaraTidakValid_Ditolak422(t *testing.T) {
	for _, tc := range []struct {
		nama  string
		jam   string
		field string
	}{
		{"string kosong", "", "eventStartTime"},
		{"pakai titik", "19.00", "eventStartTime"},
		{"jam di luar rentang", "25:00", "eventStartTime"},
		{"bukan jam sama sekali", "malam", "eventStartTime"},
	} {
		t.Run(tc.nama, func(t *testing.T) {
			pv := baseEngagement()
			svc := newVendorEngagementServiceForTest(pv)
			input := baseEngagementInputFor(pv)
			jam := tc.jam
			input.EventStartTime = &jam

			_, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input)
			assertValidation(t, err, tc.field)
		})
	}
}

func TestVendorEngagementCreate_JamAcaraValidAtauKosong_Diterima(t *testing.T) {
	jam := "19:00"
	for _, tc := range []struct {
		nama  string
		mulai *string
	}{
		{"HH:MM valid", &jam},
		{"nil = Belum ditentukan", nil},
	} {
		t.Run(tc.nama, func(t *testing.T) {
			pv := baseEngagement()
			svc := newVendorEngagementServiceForTest(pv)
			input := baseEngagementInputFor(pv)
			input.EventStartTime = tc.mulai

			if _, err := svc.Create(context.Background(), 1, pv.ProjectID, 99, input); err != nil {
				t.Fatalf("Create() error = %v, want nil", err)
			}
		})
	}
}

// --- truncateTo (PLAN.md T2) ---

func TestTruncateTo(t *testing.T) {
	t.Run("memangkas tepat ke batas", func(t *testing.T) {
		got := truncateTo(repeatRunes('a', maxScope), maxActivityLabel)
		if n := len([]rune(got)); n != maxActivityLabel {
			t.Errorf("panjang hasil = %d rune, want %d", n, maxActivityLabel)
		}
	})

	t.Run("tidak mengusik teks yang sudah muat", func(t *testing.T) {
		in := repeatRunes('a', 200)
		if got := truncateTo(in, maxActivityDescription); got != in {
			t.Error("teks yang sudah muat tidak boleh diubah")
		}
	})

	// Pemangkasan berbasis rune, bukan byte: memotong di tengah karakter
	// multibyte menghasilkan UTF-8 rusak yang justru DITOLAK MySQL — kegagalan
	// yang sama persis dengan yang sedang diperbaiki.
	t.Run("tidak memotong rune multibyte di tengah", func(t *testing.T) {
		got := truncateTo(repeatRunes('é', 300), maxActivityLabel)
		if !utf8.ValidString(got) {
			t.Error("hasil pemangkasan bukan UTF-8 yang sah")
		}
		if n := len([]rune(got)); n != maxActivityLabel {
			t.Errorf("panjang hasil = %d rune, want %d", n, maxActivityLabel)
		}
	})
}

// --- Query ganda (PLAN.md T6) ---

// Sebelum T6, guardBudget membaca daftar engagement lalu memanggil
// CostSummary yang membacanya LAGI — dua query identik untuk satu tulisan.
func TestGuardBudget_Create_HanyaSatuPembacaanDaftar(t *testing.T) {
	svc, repo, _ := newGuardedServiceForTest()

	if _, err := svc.Create(context.Background(), 1, 10, 99, guardedInput(10_000_000)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if repo.listCalls != 1 {
		t.Errorf("ListByProject terpanggil %d kali, want tepat 1", repo.listCalls)
	}
}

// Alasan sepanjang batas tetap menghasilkan baris audit yang muat di
// activity_log.description (VARCHAR(500)) — kalau tidak, Record menelan
// errornya dan komitmen over-budget tersimpan TANPA jejak siapa-kapan-kenapa,
// yaitu justru satu-satunya alasan gerbang ini dirancang.
func TestGuardBudget_AlasanPanjangDiBatas_JejakTetapTercatatDanMuat(t *testing.T) {
	svc, _, acts := newGuardedServiceForTest()
	input := guardedInput(60_000_000) // 50jt + 60jt + 10jt venue = 120jt > 100jt
	input.OverBudgetReason = repeatRunes('a', maxReason)

	if _, err := svc.Create(context.Background(), 1, 10, 99, input); err != nil {
		t.Fatalf("Create() error = %v, want lolos karena alasan terisi", err)
	}
	var found *domain.ActivityLogEntry
	for _, e := range acts.entries {
		if e.Type == domain.ActivityVendorOverBudget {
			found = e
		}
	}
	if found == nil {
		t.Fatal("tidak ada entri ActivityVendorOverBudget — jejaknya hilang")
	}
	if n := len([]rune(found.Description)); n > maxActivityDescription {
		t.Errorf("description %d rune, melebihi kolom VARCHAR(%d) — Record akan menelan Error 1406", n, maxActivityDescription)
	}
}
