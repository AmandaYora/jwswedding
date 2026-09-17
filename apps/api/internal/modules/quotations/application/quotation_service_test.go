package application

import (
	"context"
	"errors"
	"testing"
	"time"

	projectscontracts "jwswedding/internal/modules/projects/contracts"

	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

func TestQuotationTotalAdjustments_SignedSum(t *testing.T) {
	adjs := []domain.QuotationAdjustment{
		{Description: "Takeout", Amount: -1_500_000},
		{Description: "Add", Amount: 9_900_000},
	}
	if got := domain.TotalAdjustments(adjs); got != 8_400_000 {
		t.Errorf("TotalAdjustments = %d, mau 8400000", got)
	}
}

// --- Transisi status: setiap transisi ilegal ditolak (§9). ---

type fakeQuotationRepo struct {
	rows        map[int64]*domain.Quotation
	blocks      map[int64][]domain.QuotationBlock
	adjustments map[int64][]domain.QuotationAdjustment
	nextID      int64
}

func newFakeQuotationRepo() *fakeQuotationRepo {
	return &fakeQuotationRepo{rows: map[int64]*domain.Quotation{}, blocks: map[int64][]domain.QuotationBlock{}, adjustments: map[int64][]domain.QuotationAdjustment{}, nextID: 1}
}

func (f *fakeQuotationRepo) seed(o *domain.Quotation) *domain.Quotation {
	o.ID = f.nextID
	f.nextID++
	cp := *o
	f.rows[o.ID] = &cp
	return &cp
}

func (f *fakeQuotationRepo) FindByID(_ context.Context, _ int64, id int64) (*domain.Quotation, error) {
	if o, ok := f.rows[id]; ok {
		cp := *o
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeQuotationRepo) ListByTenant(_ context.Context, _ int64, _ QuotationListFilter, _ pagination.Params) ([]domain.Quotation, int64, error) {
	list := make([]domain.Quotation, 0, len(f.rows))
	for _, o := range f.rows {
		list = append(list, *o)
	}
	return list, int64(len(list)), nil
}
func (f *fakeQuotationRepo) ListIDsByClient(_ context.Context, _ int64, _ int64, _ ...string) ([]domain.Quotation, error) {
	panic("not implemented")
}
func (f *fakeQuotationRepo) Create(_ context.Context, o *domain.Quotation) error {
	o.ID = f.nextID
	f.nextID++
	cp := *o
	f.rows[o.ID] = &cp
	return nil
}
func (f *fakeQuotationRepo) Update(_ context.Context, o *domain.Quotation) error {
	cp := *o
	f.rows[o.ID] = &cp
	return nil
}
func (f *fakeQuotationRepo) Delete(_ context.Context, _ int64, id int64) error {
	delete(f.rows, id)
	return nil
}
func (f *fakeQuotationRepo) DeleteForClient(_ context.Context, _ int64, _ int64) error {
	panic("not implemented")
}
func (f *fakeQuotationRepo) NextPOSequence(_ context.Context, _ int64, _ string) (int, error) {
	return 1, nil
}
func (f *fakeQuotationRepo) ListBlocks(_ context.Context, quotationID int64) ([]domain.QuotationBlock, error) {
	return f.blocks[quotationID], nil
}
func (f *fakeQuotationRepo) ReplaceBlocks(_ context.Context, quotationID int64, blocks []domain.QuotationBlock) error {
	f.blocks[quotationID] = blocks
	return nil
}
func (f *fakeQuotationRepo) ListAdjustments(_ context.Context, quotationID int64) ([]domain.QuotationAdjustment, error) {
	return f.adjustments[quotationID], nil
}
func (f *fakeQuotationRepo) ReplaceAdjustments(_ context.Context, quotationID int64, adjustments []domain.QuotationAdjustment) error {
	f.adjustments[quotationID] = adjustments
	return nil
}
func (f *fakeQuotationRepo) DistinctCategories(_ context.Context, _ int64) ([]string, error) {
	return nil, nil
}
func (f *fakeQuotationRepo) AdjustmentTotals(_ context.Context, quotationIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for _, id := range quotationIDs {
		var sum int64
		for _, a := range f.adjustments[id] {
			sum += a.Amount
		}
		out[id] = sum
	}
	return out, nil
}

type fakeQuotationTemplates struct{}

func (fakeQuotationTemplates) List(_ context.Context, _ int64, _ bool) ([]domain.PackageTemplateSummary, error) {
	panic("not implemented")
}
func (fakeQuotationTemplates) FindByID(_ context.Context, _ int64, _ int64) (*domain.PackageTemplate, error) {
	panic("not implemented")
}
func (fakeQuotationTemplates) Create(_ context.Context, _ *domain.PackageTemplate) error {
	panic("not implemented")
}
func (fakeQuotationTemplates) Update(_ context.Context, _ *domain.PackageTemplate) error {
	panic("not implemented")
}
func (fakeQuotationTemplates) Delete(_ context.Context, _ int64, _ int64) error {
	panic("not implemented")
}
func (fakeQuotationTemplates) NextSortOrder(_ context.Context, _ int64) (int, error) {
	panic("not implemented")
}
func (fakeQuotationTemplates) ReplaceBlocks(_ context.Context, _ int64, _ []domain.PackageTemplateBlock) error {
	panic("not implemented")
}

// fakeProjectsContracts memenuhi projectscontracts.Contracts dengan panic di
// semua metode kecuali dua yang dipakai jalur transisi (ProjectIDForQuotation
// tanpa project, SyncFromQuotation no-op) — cukup untuk mengunci mesin
// status tanpa database.
type fakeProjectsContracts struct {
	projectID int64
	synced    []int64
	// created meniru kelahiran project oleh CreateFromQuotation: tiap
	// panggilan mengembalikan ID baru dan mengingatnya sebagai projectID,
	// supaya cabang idempoten menemukannya sesudahnya.
	created  int64
	nextID   int64
	accepted []projectscontracts.CreateFromQuotationInput
}

func (f *fakeProjectsContracts) ProjectExists(_ context.Context, _, _ int64) (bool, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ProjectPICStaffID(_ context.Context, _, _ int64) (int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ProjectPICSalesStaffID(_ context.Context, _, _ int64) (int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ListVendorEngagementHistory(_ context.Context, _, _ int64) ([]projectscontracts.VendorEngagementHistoryItem, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ListAffectedProjectsForVendor(_ context.Context, _, _ int64) ([]projectscontracts.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ListAffectedProjectsForVenue(_ context.Context, _, _ int64) ([]projectscontracts.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ListAffectedProjectsForStaff(_ context.Context, _, _ int64) ([]projectscontracts.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) SeedDefaultMilestoneTemplate(_ context.Context, _ int64) error {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ClientHasProject(_ context.Context, _, _ int64) (bool, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ProjectIDsForClient(_ context.Context, _, _ int64) ([]int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ProjectCountsForClients(_ context.Context, _ int64, _ []int64) (map[int64]int, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ImpactForClient(_ context.Context, _, _ int64) (projectscontracts.ProjectImpact, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) DeleteProjectsForClient(_ context.Context, _, _ int64) error {
	panic("not implemented")
}
func (f *fakeProjectsContracts) CreateFromQuotation(_ context.Context, input projectscontracts.CreateFromQuotationInput) (projectscontracts.ProjectRef, error) {
	f.nextID++
	f.created = f.nextID
	f.projectID = f.nextID
	f.accepted = append(f.accepted, input)
	return projectscontracts.ProjectRef{ID: f.nextID}, nil
}
func (f *fakeProjectsContracts) SyncFromQuotation(_ context.Context, _, projectID, _ int64, _ string) error {
	f.synced = append(f.synced, projectID)
	return nil
}
func (f *fakeProjectsContracts) ProjectIDForQuotation(_ context.Context, _ int64, _ int64) (int64, error) {
	return f.projectID, nil
}
func (f *fakeProjectsContracts) ClientPaymentLedger(_ context.Context, _, _ int64) ([]projectscontracts.ClientPaymentInfo, int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) DeleteProjectCascade(_ context.Context, _, _ int64) error {
	panic("not implemented")
}
func (f *fakeProjectsContracts) PONumberForQuotation(_ context.Context, _ int64, _ int64) (string, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ProjectDeleteImpact(_ context.Context, _, _ int64) (projectscontracts.ProjectDeleteImpact, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ProjectRefsForQuotations(_ context.Context, _ int64, ids []int64) (map[int64]projectscontracts.QuotationProjectRef, error) {
	out := make(map[int64]projectscontracts.QuotationProjectRef, len(ids))
	for _, id := range ids {
		out[id] = projectscontracts.QuotationProjectRef{ProjectID: f.projectID}
	}
	return out, nil
}
func (f *fakeProjectsContracts) QuotationIDsForPICStaff(_ context.Context, _ int64, _ int64) ([]int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) ClientIDsForPIC(_ context.Context, _ int64, _, _ int64) ([]int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsContracts) PICsForClients(_ context.Context, _ int64, _ []int64) (map[int64]projectscontracts.ClientPICs, error) {
	panic("not implemented")
}

type fakeClientDirectory struct{}

func (fakeClientDirectory) CoupleNames(_ context.Context, _, _ int64) (string, string, error) {
	return "Rara", "Dafa", nil
}
func (fakeClientDirectory) CoupleNamesBatch(_ context.Context, _ int64, ids []int64) (map[int64][2]string, error) {
	out := make(map[int64][2]string, len(ids))
	for _, id := range ids {
		out[id] = [2]string{"Rara", "Dafa"}
	}
	return out, nil
}
func (fakeClientDirectory) PhoneForClient(_ context.Context, _, _ int64) (string, error) {
	return "08170043310", nil
}
func (fakeClientDirectory) SaveSpecimen(_ context.Context, _ int64, _ int64, _, _ string, _ []byte, _, _ string) error {
	return nil
}
func (fakeClientDirectory) SpecimenImage(_ context.Context, _, _ int64) ([]byte, error) {
	return nil, nil
}
func (fakeClientDirectory) SpecimenMeta(_ context.Context, _, _ int64) ([4]string, bool, error) {
	return [4]string{}, false, nil
}
func (fakeClientDirectory) SignerOptions(_ context.Context, _, _ int64) ([][2]string, error) {
	return [][2]string{{"Bride", "Rara"}, {"Groom", "Dafa"}}, nil
}
func (fakeClientDirectory) ClientIDForContact(_ context.Context, _, contactID int64) (int64, error) {
	return contactID, nil
}

func newTransitionService() (*QuotationService, *fakeQuotationRepo, *fakeProjectsContracts) {
	repo := newFakeQuotationRepo()
	projects := &fakeProjectsContracts{}
	svc := NewQuotationService(repo, fakeQuotationTemplates{}, projects, nil)
	svc.SetClientDirectory(fakeClientDirectory{})
	return svc, repo, projects
}

func seedDraft(repo *fakeQuotationRepo, status domain.QuotationStatus) *domain.Quotation {
	// Penawaran contoh yang "sehat" harus memenuhi SEMUA syarat Kirim:
	// PackageName, EventDate, dan total > 0. Penolakan tiap syarat dikunci
	// terpisah di TestQuotation_Issue_* di bawah, jadi seed ini tidak boleh
	// melanggar satu pun — kalau melanggar, setiap tes lain gagal karena
	// alasan yang tidak ada hubungannya dengan apa yang sedang diujinya.
	eventDate := time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)
	return repo.seed(&domain.Quotation{
		TenantID: 1, ClientID: 7, Status: status, BasePrice: 100_000_000, PackageName: "Silver",
		EventDate: &eventDate,
	})
}

// Tanggal acara wajib pada titik KIRIM, bukan saat Terima. Tanpa syarat ini
// penawaran bisa terkirim tanpa tanggal, lalu Terima menolaknya lewat dialog
// yang tidak punya kotak isian untuk itu — dan penawarannya sudah tidak bisa
// diedit lagi. Buntu.
func TestQuotation_Issue_TanpaTanggalAcaraDitolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := repo.seed(&domain.Quotation{
		TenantID: 1, ClientID: 7, Status: domain.QuotationDraft, BasePrice: 100_000_000, PackageName: "Silver",
	})

	_, err := svc.Issue(context.Background(), 1, o.ID, 1)
	if err == nil {
		t.Fatal("Issue tanpa tanggal acara berhasil, seharusnya ditolak")
	}
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Fields["eventDate"] == nil {
		t.Errorf("error = %v, mau Validation ber-field eventDate agar bisa ditempel di kotaknya", err)
	}
	if o.PONumber != "" {
		t.Error("nomor PO terlanjur diberikan padahal Issue ditolak")
	}
	if o.Status != domain.QuotationDraft {
		t.Errorf("status = %q, mau tetap Draft", o.Status)
	}
}

// Sama alasannya: total nol sudah ditolak CreateFromQuotation, jadi
// membiarkannya lolos Kirim hanya menunda penolakan sampai titik yang tidak
// bisa diperbaiki lagi.
func TestQuotation_Issue_TotalNolDitolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	eventDate := time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)
	o := repo.seed(&domain.Quotation{
		TenantID: 1, ClientID: 7, Status: domain.QuotationDraft, BasePrice: 0, PackageName: "Silver",
		EventDate: &eventDate,
	})

	if _, err := svc.Issue(context.Background(), 1, o.ID, 1); err == nil {
		t.Fatal("Issue dengan total nol berhasil, seharusnya ditolak")
	}
	if o.Status != domain.QuotationDraft {
		t.Errorf("status = %q, mau tetap Draft", o.Status)
	}
}

// Nama paket wajib pada titik komitmen, bukan saat Draft dibuat: tanpa syarat
// ini Accept bisa melahirkan project tanpa nama paket, dan itulah yang membuat
// Portal Klien menampilkan kolom "Paket / Layanan" kosong.
func TestQuotation_Issue_TanpaNamaPaketDitolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := repo.seed(&domain.Quotation{TenantID: 1, ClientID: 7, Status: domain.QuotationDraft, BasePrice: 100_000_000})

	if _, err := svc.Issue(context.Background(), 1, o.ID, 1); err == nil {
		t.Fatal("Issue tanpa nama paket berhasil, seharusnya ditolak")
	}
	if o.PONumber != "" {
		t.Error("nomor PO terlanjur diberikan padahal Issue ditolak")
	}
	if o.Status != domain.QuotationDraft {
		t.Errorf("status = %q, mau tetap Draft", o.Status)
	}
}

// Spasi saja bukan nama.
func TestQuotation_Issue_NamaPaketSpasiSajaDitolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := repo.seed(&domain.Quotation{
		TenantID: 1, ClientID: 7, Status: domain.QuotationDraft, BasePrice: 100_000_000, PackageName: "   ",
	})

	if _, err := svc.Issue(context.Background(), 1, o.ID, 1); err == nil {
		t.Fatal("Issue dengan nama paket spasi berhasil, seharusnya ditolak")
	}
}

// Snapshot membekukan nama paket: PO yang sudah diteken lalu dicetak ulang
// harus menampilkan nama saat itu, bukan hasil revisi berikutnya.
func TestQuotation_Issue_SnapshotMembekukanNamaPaket(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationDraft)

	view, err := svc.Issue(context.Background(), 1, o.ID, 1)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if got := view.Quotation.Snapshot.Current.PackageName; got != "Silver" {
		t.Errorf("snapshot.PackageName = %q, mau \"Silver\"", got)
	}
}

func TestQuotation_IssueDraft_MenjadiDitawarkanBernomor(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationDraft)

	view, err := svc.Issue(context.Background(), 1, o.ID, 1)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if view.Quotation.Status != domain.QuotationOffered {
		t.Errorf("status = %q, mau Ditawarkan", view.Quotation.Status)
	}
	if view.Quotation.PONumber == "" {
		t.Error("penawaran Ditawarkan tanpa nomor PO")
	}
	if view.Quotation.Snapshot == nil {
		t.Error("snapshot tidak dibekukan saat Issue")
	}
}

func TestQuotation_IssueBukanDraft_Ditolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationOffered)

	if _, err := svc.Issue(context.Background(), 1, o.ID, 1); err == nil {
		t.Error("Issue dari Ditawarkan berhasil, seharusnya ditolak")
	}
}

func TestQuotation_AcceptBukanDitawarkan_Ditolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	for _, status := range []domain.QuotationStatus{
		domain.QuotationDraft, domain.QuotationRejected, domain.QuotationExpired, domain.QuotationCancelled,
	} {
		o := seedDraft(repo, status)
		eventDate := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
		o.EventDate = &eventDate
		if _, _, err := svc.Accept(context.Background(), 1, o.ID, AcceptQuotationInput{
			PrepStartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}); err == nil {
			t.Errorf("Accept dari %q berhasil, seharusnya ditolak", status)
		}
	}
}

func TestQuotation_DitolakKeDiterima_Ditolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationRejected)

	if _, err := svc.Revise(context.Background(), 1, o.ID); err == nil {
		t.Error("Revise dari Ditolak berhasil, seharusnya ditolak")
	}
	eventDate := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	o.EventDate = &eventDate
	if _, _, err := svc.Accept(context.Background(), 1, o.ID, AcceptQuotationInput{}); err == nil {
		t.Error("Accept dari Ditolak berhasil, seharusnya ditolak")
	}
}

func TestQuotation_WithdrawDanRevise_MembukaDraft(t *testing.T) {
	svc, repo, _ := newTransitionService()

	offered := seedDraft(repo, domain.QuotationOffered)
	view, err := svc.Withdraw(context.Background(), 1, offered.ID)
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if view.Quotation.Status != domain.QuotationDraft || view.Quotation.Revision != 1 {
		t.Errorf("Withdraw = %q rev %d, mau Draft rev 1", view.Quotation.Status, view.Quotation.Revision)
	}

	accepted := seedDraft(repo, domain.QuotationAccepted)
	view, err = svc.Revise(context.Background(), 1, accepted.ID)
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}
	if view.Quotation.Status != domain.QuotationDraft || view.Quotation.Revision != 1 {
		t.Errorf("Revise = %q rev %d, mau Draft rev 1", view.Quotation.Status, view.Quotation.Revision)
	}
}

func TestQuotation_RejectExpireCancel_HanyaDariDitawarkan(t *testing.T) {
	svc, repo, _ := newTransitionService()

	o := seedDraft(repo, domain.QuotationOffered)
	if view, err := svc.Reject(context.Background(), 1, o.ID); err != nil || view.Quotation.Status != domain.QuotationRejected {
		t.Errorf("Reject: %v, status %q", err, view.Quotation.Status)
	}
	o2 := seedDraft(repo, domain.QuotationOffered)
	if view, err := svc.Expire(context.Background(), 1, o2.ID); err != nil || view.Quotation.Status != domain.QuotationExpired {
		t.Errorf("Expire: %v, status %q", err, view.Quotation.Status)
	}
	o3 := seedDraft(repo, domain.QuotationDraft)
	if view, err := svc.Cancel(context.Background(), 1, o3.ID); err != nil || view.Quotation.Status != domain.QuotationCancelled {
		t.Errorf("Cancel dari Draft: %v, status %q", err, view.Quotation.Status)
	}
	o4 := seedDraft(repo, domain.QuotationDraft)
	if _, err := svc.Reject(context.Background(), 1, o4.ID); err == nil {
		t.Error("Reject dari Draft berhasil, seharusnya ditolak")
	}
}

func TestQuotation_TulisSaatTerkirim_Ditolak(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationOffered)

	if _, err := svc.ReplaceBlocks(context.Background(), 1, o.ID, nil); err == nil {
		t.Error("ReplaceBlocks saat Ditawarkan berhasil, seharusnya ditolak")
	}
	if _, err := svc.SetHeader(context.Background(), 1, o.ID, SetHeaderInput{BasePrice: 1}); err == nil {
		t.Error("SetHeader saat Ditawarkan berhasil, seharusnya ditolak")
	}
}

func TestQuotation_Duplicate_DraftTanpaNomor(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationOffered)
	o.PONumber = "PO/202609/0007"
	repo.rows[o.ID].PONumber = "PO/202609/0007"
	repo.blocks[o.ID] = []domain.QuotationBlock{{Category: "CATERING", Body: "x"}}

	view, err := svc.Duplicate(context.Background(), 1, o.ID, 1)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if view.Quotation.Status != domain.QuotationDraft || view.Quotation.PONumber != "" {
		t.Errorf("duplikat = %q %q, mau Draft tanpa nomor", view.Quotation.Status, view.Quotation.PONumber)
	}
	if len(view.Blocks) != 1 {
		t.Errorf("blok duplikat = %d, mau 1", len(view.Blocks))
	}
}

// --- Validasi harus mendahului penulisan, dan client terkunci setelah jadi
// project. Keduanya ditemukan saat code review implementasi PLAN. ---

// Total minus ditolak 422; yang penting BUKAN sekadar penolakannya, melainkan
// bahwa nilai yang ditolak tidak ikut tersimpan. Versi sebelumnya menulis dulu
// lalu memvalidasi, sehingga 422 dikembalikan atas nilai yang sudah tertulis.
func TestQuotation_ReplaceAdjustments_TotalMinusTidakMenyisakanTulisan(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationDraft)

	_, err := svc.ReplaceAdjustments(context.Background(), 1, o.ID, []domain.QuotationAdjustment{
		{QuotationID: o.ID, Description: "Takeout kelewat besar", Amount: -900_000_000},
	})
	if err == nil {
		t.Fatal("penyesuaian yang membuat total minus diterima")
	}
	if got := len(repo.adjustments[o.ID]); got != 0 {
		t.Errorf("tersimpan %d penyesuaian padahal permintaan ditolak — validasi harus mendahului tulis", got)
	}
}

func TestQuotation_SetHeader_HargaMinusTidakMenyisakanTulisan(t *testing.T) {
	svc, repo, _ := newTransitionService()
	o := seedDraft(repo, domain.QuotationDraft)
	if err := repo.ReplaceAdjustments(context.Background(), o.ID, []domain.QuotationAdjustment{
		{QuotationID: o.ID, Description: "Takeout", Amount: -80_000_000},
	}); err != nil {
		t.Fatalf("seed penyesuaian: %v", err)
	}

	// Harga baru 50jt dengan takeout 80jt => total minus, harus ditolak tanpa
	// menyentuh harga yang tersimpan.
	_, err := svc.SetHeader(context.Background(), 1, o.ID, SetHeaderInput{BasePrice: 50_000_000, TermsText: "S&K", BonusNote: "Bonus", Pax: 700})
	if err == nil {
		t.Fatal("harga yang membuat total minus diterima")
	}
	if repo.rows[o.ID].BasePrice != 100_000_000 {
		t.Errorf("basePrice tersimpan = %d, mau tetap 100000000", repo.rows[o.ID].BasePrice)
	}
}

// Penawaran yang sudah melahirkan project tidak boleh berpindah client:
// project menyimpan client_id dan nama mempelainya sendiri, jadi perpindahan
// diam-diam membuat keduanya menunjuk client yang berbeda.
func TestQuotation_SetHeader_ClientTerkunciSetelahJadiProject(t *testing.T) {
	svc, repo, projects := newTransitionService()
	o := seedDraft(repo, domain.QuotationDraft)
	projects.projectID = 42

	_, err := svc.SetHeader(context.Background(), 1, o.ID, SetHeaderInput{BasePrice: 100_000_000, TermsText: "S&K", BonusNote: "Bonus", Pax: 700, ClientID: 99})
	if err == nil {
		t.Fatal("client berhasil diganti padahal penawaran sudah jadi project")
	}
	if repo.rows[o.ID].ClientID != 7 {
		t.Errorf("clientID tersimpan = %d, mau tetap 7", repo.rows[o.ID].ClientID)
	}
}

// Selama belum ada project, ganti client masih sah — prospek memang bisa salah
// pilih di awal.
func TestQuotation_SetHeader_ClientMasihBisaDigantiSebelumJadiProject(t *testing.T) {
	svc, repo, projects := newTransitionService()
	o := seedDraft(repo, domain.QuotationDraft)
	projects.projectID = 0

	if _, err := svc.SetHeader(context.Background(), 1, o.ID, SetHeaderInput{BasePrice: 100_000_000, TermsText: "S&K", BonusNote: "Bonus", Pax: 700, ClientID: 99}); err != nil {
		t.Fatalf("SetHeader: %v", err)
	}
	if repo.rows[o.ID].ClientID != 99 {
		t.Errorf("clientID = %d, mau 99", repo.rows[o.ID].ClientID)
	}
}
