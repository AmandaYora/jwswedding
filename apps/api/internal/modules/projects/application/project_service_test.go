package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

// fakeProjectRepo is a minimal in-memory stand-in for ProjectRepository —
// good enough to exercise Update's control flow (PLAN.md
// mom-25082026-item-sebagian §7.1) without a real database. Every method
// beyond FindByID/Update is left unimplemented since Update is the only one
// under test here, matching client_payment_service_test.go's own
// fakeClientPaymentRepo idiom.
type fakeProjectRepo struct {
	project *domain.Project
	updated *domain.Project
}

func (f *fakeProjectRepo) List(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64) ([]domain.Project, error) {
	panic("not implemented")
}
func (f *fakeProjectRepo) CountAll(ctx context.Context, tenantID int64) (int64, error) {
	panic("not implemented")
}
func (f *fakeProjectRepo) ListForDashboard(ctx context.Context, tenantID int64, since time.Time) ([]domain.Project, error) {
	panic("not implemented")
}
func (f *fakeProjectRepo) ListPaginated(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64, params pagination.Params, search, status string, showArchived bool) ([]domain.Project, int64, error) {
	panic("not implemented")
}
func (f *fakeProjectRepo) ListByVenueID(ctx context.Context, tenantID, venueID int64) ([]domain.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectRepo) ListByStaffPIC(ctx context.Context, tenantID, staffID int64) ([]domain.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectRepo) FindByID(ctx context.Context, tenantID, id int64) (*domain.Project, error) {
	return f.project, nil
}
func (f *fakeProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	panic("not implemented")
}
func (f *fakeProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	f.updated = p
	return nil
}
func (f *fakeProjectRepo) SetStatus(ctx context.Context, tenantID, id int64, status domain.ProjectStatus) error {
	panic("not implemented")
}
func (f *fakeProjectRepo) SetArchived(ctx context.Context, tenantID, id int64, archived bool) error {
	panic("not implemented")
}
func (f *fakeProjectRepo) DeleteCascade(ctx context.Context, tenantID, id int64) error {
	panic("not implemented")
}

// fakeActivityRepoForProject is a no-op ActivityRepository -- Update always
// calls s.activity.Record after a successful write, so a real
// *ActivityService needs a repo to hand its Create call to, even though this
// test suite never inspects what got recorded.
type fakeActivityRepoForProject struct{}

func (f *fakeActivityRepoForProject) Create(ctx context.Context, entry *domain.ActivityLogEntry) error {
	return nil
}
func (f *fakeActivityRepoForProject) ListByProject(ctx context.Context, projectID int64, limit int) ([]domain.ActivityLogEntry, error) {
	return nil, nil
}

func int64Ptr(v int64) *int64 { return &v }

// baseProject and baseInputFor(p) together form a project whose stored state
// and next-submitted input are identical field-for-field -- every test below
// starts from this "no changes at all" baseline and mutates exactly one
// field, so a Forbidden result can only be attributed to that one field.
func baseProject() *domain.Project {
	return &domain.Project{
		ID: 1, TenantID: 1,
		Name: "Aurelia & Bagas Wedding", BrideName: "Aurelia", GroomName: "Bagas",
		EventDate:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Venue:           "Grand Ballroom",
		PrepStartDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PackageName:     "Paket Premium",
		ContractValue:   50_000_000,
		Status:          domain.StatusPreparation,
		PICStaffID:      5,
		PICSalesStaffID: 7,
		Description:     "Deskripsi awal",
	}
}

func baseInputFor(p *domain.Project) ProjectInput {
	return ProjectInput{
		Name: p.Name, BrideName: p.BrideName, GroomName: p.GroomName, EventDate: p.EventDate,
		Venue: p.Venue, PrepStartDate: p.PrepStartDate, PackageName: p.PackageName,
		ContractValue: p.ContractValue, Status: p.Status, PICStaffID: p.PICStaffID,
		PICSalesStaffID: p.PICSalesStaffID, Description: p.Description,
		VenueID: p.VenueID, VenueRentalPrice: p.VenueRentalPrice, VenueCharge: p.VenueCharge,
	}
}

func newProjectServiceForTest(project *domain.Project) *ProjectService {
	return &ProjectService{
		repo:     &fakeProjectRepo{project: project},
		activity: NewActivityService(&fakeActivityRepoForProject{}),
	}
}

func assertForbidden(t *testing.T, err error) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindForbidden {
		t.Fatalf("Update() error = %v, want a Forbidden apperror", err)
	}
}

func TestProjectServiceUpdate_StaffMengubahNilaiKontrak_Forbidden(t *testing.T) {
	p := baseProject()
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.ContractValue = p.ContractValue + 1
	_, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Staff", input)
	assertForbidden(t, err)
}

func TestProjectServiceUpdate_StaffMengubahTanggalAcara_Forbidden(t *testing.T) {
	p := baseProject()
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.EventDate = p.EventDate.AddDate(0, 0, 1)
	_, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Staff", input)
	assertForbidden(t, err)
}

func TestProjectServiceUpdate_SalesMengubahPaket_Forbidden(t *testing.T) {
	p := baseProject()
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.PackageName = "Paket Lain"
	_, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Sales", input)
	assertForbidden(t, err)
}

func TestProjectServiceUpdate_StaffHanyaMengubahStatus_Berhasil(t *testing.T) {
	p := baseProject()
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.Status = domain.StatusReady
	got, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Staff", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.Status != domain.StatusReady {
		t.Errorf("Status = %q, want %q", got.Status, domain.StatusReady)
	}
}

func TestProjectServiceUpdate_StaffHanyaMengubahDeskripsi_Berhasil(t *testing.T) {
	p := baseProject()
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.Description = "Catatan baru dari WP"
	got, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Staff", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.Description != "Catatan baru dari WP" {
		t.Errorf("Description = %q, want %q", got.Description, "Catatan baru dari WP")
	}
}

// Regresi timezone (ditemukan lewat verifikasi HTTP nyata terhadap MySQL,
// bukan lewat unit test -- setiap unit test lain di berkas ini memakai
// time.UTC di kedua sisi sehingga tidak pernah memicu bug ini). Nilai yang
// dimuat dari database membawa lokasi apa pun yang diset DATABASE_URL's
// loc= (proyek ini: "Local", yaitu zona OS server -- WIB/+07:00 di
// produksi), sedangkan presentation.parseDate selalu menghasilkan UTC.
// Sebelum sameCalendarDate ada, guardKonteksUmum memakai time.Time.Equal
// (instan absolut) sehingga tanggal kalender yang identik tapi berlokasi
// beda selalu terbaca "berubah" -- WP tertolak mengubah APAPUN, termasuk
// Status, walau tidak menyentuh EventDate sama sekali.
func TestProjectServiceUpdate_StaffTanggalSamaBedaLokasi_Berhasil(t *testing.T) {
	p := baseProject()
	jakarta := time.FixedZone("WIB", 7*60*60)
	// EventDate/PrepStartDate "tersimpan" dalam lokasi +07:00, meniru nilai
	// yang baru dimuat dari MySQL lewat loc=Local -- midnight WIB adalah
	// instan yang berbeda dari midnight UTC pada tanggal kalender yang sama.
	p.EventDate = time.Date(2026, 6, 1, 0, 0, 0, 0, jakarta)
	p.PrepStartDate = time.Date(2026, 1, 1, 0, 0, 0, 0, jakarta)
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	// input meniru presentation.parseDate: UTC, tanggal kalender sama persis.
	input.EventDate = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	input.PrepStartDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	input.Status = domain.StatusReady
	got, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Staff", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success (tanggal kalender sama, lokasi beda bukan perubahan)", err)
	}
	if got.Status != domain.StatusReady {
		t.Errorf("Status = %q, want %q", got.Status, domain.StatusReady)
	}
}

// Regresi nil-pointer: VenueID == nil berarti "body request tidak
// menyertakan kunci ini", bukan "kosongkan" -- harus tetap berhasil untuk
// Staff walau project saat ini punya venue terpasang, dan venue itu tidak
// boleh ikut terlepas.
func TestProjectServiceUpdate_StaffVenueIDNilSaatVenueTerpasang_Berhasil(t *testing.T) {
	p := baseProject()
	p.VenueID = int64Ptr(42)
	p.VenueRentalPrice = int64Ptr(10_000_000)
	p.VenueCharge = int64Ptr(2_000_000)
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.VenueID = nil
	input.VenueRentalPrice = nil
	input.VenueCharge = nil
	got, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Staff", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.VenueID == nil || *got.VenueID != 42 {
		t.Errorf("VenueID = %v, want tetap 42 (tidak ikut terlepas)", got.VenueID)
	}
}

// Perbandingan harus berdasar nilai yang ditunjuk, bukan alamat pointer --
// mengirim VenueID baru dengan nilai yang sama seperti yang tersimpan bukan
// perubahan.
func TestProjectServiceUpdate_StaffVenueIDNilaiSama_Berhasil(t *testing.T) {
	p := baseProject()
	p.VenueID = int64Ptr(42)
	p.VenueRentalPrice = int64Ptr(10_000_000)
	p.VenueCharge = int64Ptr(2_000_000)
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.VenueID = int64Ptr(42)
	input.VenueRentalPrice = int64Ptr(10_000_000)
	input.VenueCharge = int64Ptr(2_000_000)
	_, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Staff", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success (nilai sama, pointer beda)", err)
	}
}

func TestProjectServiceUpdate_Owner_MengubahSeluruhField_Berhasil(t *testing.T) {
	p := baseProject()
	svc := newProjectServiceForTest(p)
	input := baseInputFor(p)
	input.Name = "Nama Baru"
	input.BrideName = "Bride Baru"
	input.GroomName = "Groom Baru"
	input.EventDate = p.EventDate.AddDate(0, 1, 0)
	input.Venue = "Venue Baru"
	input.PrepStartDate = p.PrepStartDate.AddDate(0, 1, 0)
	input.PackageName = "Paket Baru"
	input.ContractValue = 99_000_000
	got, err := svc.Update(context.Background(), p.TenantID, p.ID, 99, "Owner", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.ContractValue != 99_000_000 || got.Name != "Nama Baru" {
		t.Errorf("Owner update tidak diterapkan: got %+v", got)
	}
}

// --- guardStatusSelesai (PLAN.md mom-25082026-item-belum item 5) ---

// fakeMilestoneRepo is a minimal in-memory stand-in for MilestoneRepository
// -- only FindByID/Update matter for UpdateMilestone's own tests.
type fakeMilestoneRepo struct {
	milestone *domain.ProjectMilestone
	updated   *domain.ProjectMilestone
}

func (f *fakeMilestoneRepo) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectMilestone, error) {
	panic("not implemented")
}
func (f *fakeMilestoneRepo) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectMilestone, error) {
	panic("not implemented")
}
func (f *fakeMilestoneRepo) FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectMilestone, error) {
	return f.milestone, nil
}
func (f *fakeMilestoneRepo) Create(ctx context.Context, m *domain.ProjectMilestone) error {
	panic("not implemented")
}
func (f *fakeMilestoneRepo) Update(ctx context.Context, m *domain.ProjectMilestone) error {
	f.updated = m
	return nil
}
func (f *fakeMilestoneRepo) NextSortOrder(ctx context.Context, projectID int64) (int, error) {
	panic("not implemented")
}
func (f *fakeMilestoneRepo) Reorder(ctx context.Context, projectID int64, orderedIDs []int64) error {
	panic("not implemented")
}

// fakeEvidenceRepoForGuard is a minimal in-memory stand-in for
// EvidenceRepository -- only ListByRelated matters for guardStatusSelesai,
// and it counts calls so the "guard never even asks" performance claim
// (PLAN.md §5) is something a test actually verifies, not just prose.
type fakeEvidenceRepoForGuard struct {
	hasEvidence        bool
	listByRelatedCalls int
}

func (f *fakeEvidenceRepoForGuard) ListByProject(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForGuard) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForGuard) ListByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) ([]domain.Evidence, error) {
	f.listByRelatedCalls++
	if f.hasEvidence {
		return []domain.Evidence{{ID: 1}}, nil
	}
	return nil, nil
}
func (f *fakeEvidenceRepoForGuard) ListClientVisibleGeneral(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForGuard) FindByID(ctx context.Context, projectID, id int64) (*domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForGuard) Create(ctx context.Context, e *domain.Evidence) error {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForGuard) SetClientVisible(ctx context.Context, projectID, id int64, visible bool) error {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForGuard) DeleteByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) error {
	panic("not implemented")
}

func baseTestMilestone() *domain.ProjectMilestone {
	return &domain.ProjectMilestone{
		ID: 1, ProjectID: 10, SortOrder: 1, Name: "Technical Meeting",
		Status: domain.MilestoneInProgress, TargetDate: time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC),
	}
}

func newProjectServiceForMilestoneTest(p *domain.Project, m *domain.ProjectMilestone, evidenceRepo *fakeEvidenceRepoForGuard) (*ProjectService, *fakeMilestoneRepo) {
	milestones := &fakeMilestoneRepo{milestone: m}
	evidence := NewEvidenceService(evidenceRepo, nil, nil, NewActivityService(&fakeActivityRepoForProject{}))
	return &ProjectService{
		repo:       &fakeProjectRepo{project: p},
		milestones: milestones,
		evidence:   evidence,
		activity:   NewActivityService(&fakeActivityRepoForProject{}),
	}, milestones
}

func TestUpdateMilestone_StatusInProgressTanpaLampiran_Berhasil(t *testing.T) {
	p, m := baseProject(), baseTestMilestone()
	evidenceRepo := &fakeEvidenceRepoForGuard{hasEvidence: false}
	svc, _ := newProjectServiceForMilestoneTest(p, m, evidenceRepo)
	_, err := svc.UpdateMilestone(context.Background(), p.TenantID, p.ID, m.ID, 99, MilestoneUpdateInput{
		Status: domain.MilestoneInProgress, TargetDate: m.TargetDate,
	})
	if err != nil {
		t.Fatalf("UpdateMilestone() error = %v, want success", err)
	}
}

func TestUpdateMilestone_TransisiCompletedTanpaCompletedDate_Validation(t *testing.T) {
	p, m := baseProject(), baseTestMilestone()
	evidenceRepo := &fakeEvidenceRepoForGuard{hasEvidence: true}
	svc, _ := newProjectServiceForMilestoneTest(p, m, evidenceRepo)
	_, err := svc.UpdateMilestone(context.Background(), p.TenantID, p.ID, m.ID, 99, MilestoneUpdateInput{
		Status: domain.MilestoneCompleted, TargetDate: m.TargetDate, CompletedDate: nil,
	})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
		t.Fatalf("UpdateMilestone() error = %v, want Validation (completedDate)", err)
	}
}

func TestUpdateMilestone_TransisiCompletedTanpaLampiran_Validation(t *testing.T) {
	p, m := baseProject(), baseTestMilestone()
	evidenceRepo := &fakeEvidenceRepoForGuard{hasEvidence: false}
	svc, _ := newProjectServiceForMilestoneTest(p, m, evidenceRepo)
	completedDate := time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.UpdateMilestone(context.Background(), p.TenantID, p.ID, m.ID, 99, MilestoneUpdateInput{
		Status: domain.MilestoneCompleted, TargetDate: m.TargetDate, CompletedDate: &completedDate,
	})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
		t.Fatalf("UpdateMilestone() error = %v, want Validation (lampiran)", err)
	}
}

func TestUpdateMilestone_TransisiCompletedDenganLampiran_Berhasil(t *testing.T) {
	p, m := baseProject(), baseTestMilestone()
	evidenceRepo := &fakeEvidenceRepoForGuard{hasEvidence: true}
	svc, _ := newProjectServiceForMilestoneTest(p, m, evidenceRepo)
	completedDate := time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC)
	got, err := svc.UpdateMilestone(context.Background(), p.TenantID, p.ID, m.ID, 99, MilestoneUpdateInput{
		Status: domain.MilestoneCompleted, TargetDate: m.TargetDate, CompletedDate: &completedDate,
	})
	if err != nil {
		t.Fatalf("UpdateMilestone() error = %v, want success", err)
	}
	if got.Status != domain.MilestoneCompleted {
		t.Errorf("Status = %q, want Completed", got.Status)
	}
}

// D1: a row already Completed stays editable without re-proving evidence --
// only the transition INTO Completed is guarded.
func TestUpdateMilestone_SudahCompletedTanpaLampiran_TetapBisaDisunting(t *testing.T) {
	p, m := baseProject(), baseTestMilestone()
	m.Status = domain.MilestoneCompleted
	newTarget := time.Date(2027, 5, 2, 0, 0, 0, 0, time.UTC)
	evidenceRepo := &fakeEvidenceRepoForGuard{hasEvidence: false}
	svc, _ := newProjectServiceForMilestoneTest(p, m, evidenceRepo)
	got, err := svc.UpdateMilestone(context.Background(), p.TenantID, p.ID, m.ID, 99, MilestoneUpdateInput{
		Status: domain.MilestoneCompleted, TargetDate: newTarget, CompletedDate: m.CompletedDate,
	})
	if err != nil {
		t.Fatalf("UpdateMilestone() error = %v, want success (D1: baris lama tetap bisa disunting)", err)
	}
	if !got.TargetDate.Equal(newTarget) {
		t.Errorf("TargetDate = %v, want %v", got.TargetDate, newTarget)
	}
	if evidenceRepo.listByRelatedCalls != 0 {
		t.Errorf("ListByRelated dipanggil %d kali, want 0 -- guard semestinya tidak menanyakan evidence sama sekali untuk baris yang sudah Completed", evidenceRepo.listByRelatedCalls)
	}
}

func TestUpdateMilestone_StatusInProgress_TidakMemanggilHasForRelated(t *testing.T) {
	p, m := baseProject(), baseTestMilestone()
	evidenceRepo := &fakeEvidenceRepoForGuard{hasEvidence: false}
	svc, _ := newProjectServiceForMilestoneTest(p, m, evidenceRepo)
	_, err := svc.UpdateMilestone(context.Background(), p.TenantID, p.ID, m.ID, 99, MilestoneUpdateInput{
		Status: domain.MilestoneBlocked, TargetDate: m.TargetDate,
	})
	if err != nil {
		t.Fatalf("UpdateMilestone() error = %v, want success", err)
	}
	if evidenceRepo.listByRelatedCalls != 0 {
		t.Errorf("ListByRelated dipanggil %d kali, want 0 untuk status non-Completed", evidenceRepo.listByRelatedCalls)
	}
}
