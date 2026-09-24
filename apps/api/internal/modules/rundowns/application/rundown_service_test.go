package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

// --------------------------------------------------------------- test double

type fakeRepo struct {
	byID        map[int64]*domain.Rundown
	byProject   map[int64]*domain.Rundown
	nextID      int64
	sections    map[domain.SectionKey]SectionPayload
	usedScope   *[]int64
	deletedID   int64
	listFilter  ListFilter
	listReturns []domain.Summary
	// tenantID yang diterima tiap method tulis, berurutan.
	writeTenantIDs []int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byID:      map[int64]*domain.Rundown{},
		byProject: map[int64]*domain.Rundown{},
		nextID:    1,
		sections:  map[domain.SectionKey]SectionPayload{},
	}
}

func (f *fakeRepo) List(_ context.Context, _ int64, filter ListFilter, _ pagination.Params) ([]domain.Summary, int, error) {
	f.listFilter = filter
	if filter.ProjectIDs != nil && len(*filter.ProjectIDs) == 0 {
		return []domain.Summary{}, 0, nil
	}
	return f.listReturns, len(f.listReturns), nil
}

func (f *fakeRepo) UsedProjectIDs(_ context.Context, _ int64, scope *[]int64) ([]int64, error) {
	f.usedScope = scope
	if scope != nil && len(*scope) == 0 {
		return []int64{}, nil
	}
	out := []int64{}
	for pid := range f.byProject {
		out = append(out, pid)
	}
	return out, nil
}

func (f *fakeRepo) Create(_ context.Context, r *domain.Rundown) error {
	r.ID = f.nextID
	f.nextID++
	copied := *r
	f.byID[r.ID] = &copied
	f.byProject[r.ProjectID] = &copied
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, _, id int64) (*domain.Rundown, error) {
	return f.byID[id], nil
}

func (f *fakeRepo) FindByProject(_ context.Context, _, projectID int64) (*domain.Rundown, error) {
	return f.byProject[projectID], nil
}

func (f *fakeRepo) LoadAggregate(_ context.Context, _, id int64) (*domain.View, error) {
	r, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	return &domain.View{Rundown: *r}, nil
}

// Keempat method tulis di bawah mencatat tenantID yang diterimanya. Itu yang
// membuat tes bisa memastikan gerbang tenant benar-benar diteruskan sampai ke
// repository, bukan berhenti di pemeriksaan service.
func (f *fakeRepo) ReplaceSection(_ context.Context, tenantID, _ int64, section domain.SectionKey, payload SectionPayload) error {
	f.writeTenantIDs = append(f.writeTenantIDs, tenantID)
	f.sections[section] = payload
	return nil
}

func (f *fakeRepo) UpdateProjectName(_ context.Context, tenantID, id int64, name string) error {
	f.writeTenantIDs = append(f.writeTenantIDs, tenantID)
	if r, ok := f.byID[id]; ok {
		r.ProjectName = name
	}
	return nil
}

func (f *fakeRepo) UpdateEvidenceID(_ context.Context, tenantID, _ int64, _ string, _ int64) error {
	f.writeTenantIDs = append(f.writeTenantIDs, tenantID)
	return nil
}

func (f *fakeRepo) UpdateLayoutImagePath(_ context.Context, tenantID, id int64, path string) error {
	f.writeTenantIDs = append(f.writeTenantIDs, tenantID)
	if r, ok := f.byID[id]; ok {
		r.LayoutImagePath = path
	}
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, _, id int64) error {
	f.deletedID = id
	delete(f.byID, id)
	return nil
}

type fakeStorage struct{ saved, deleted []string }

func (f *fakeStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	f.saved = append(f.saved, key)
	return key, nil
}
func (f *fakeStorage) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (f *fakeStorage) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

// fakeProjects hanya mengimplementasi method yang dipakai modul ini; sisanya
// panic supaya pemakaian yang tak disengaja langsung terlihat di tes.
type fakeProjects struct {
	projectscontracts.Contracts
	ctx      projectscontracts.RundownProjectContext
	ctxErr   error
	picIDs   []int64
	picIDErr error
}

func (f *fakeProjects) RundownProjectContext(_ context.Context, _, _ int64) (projectscontracts.RundownProjectContext, error) {
	return f.ctx, f.ctxErr
}

func (f *fakeProjects) ProjectIDsForPICStaff(_ context.Context, _, _ int64) ([]int64, error) {
	return f.picIDs, f.picIDErr
}

func newService() (*RundownService, *fakeRepo, *fakeStorage, *fakeProjects) {
	repo := newFakeRepo()
	st := &fakeStorage{}
	pr := &fakeProjects{ctx: projectscontracts.RundownProjectContext{
		ProjectID: 42, ProjectName: "Ayu & Bagas", BrideName: "Ayu Lestari",
		GroomName: "Bagas Arya", Venue: "Grand Mercure",
		EventDate: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC),
	}}
	return NewRundownService(repo, pr, st, nil), repo, st, pr
}

// ------------------------------------------------------------------- tes

func TestCreateFromProject_PrefillsFromProject(t *testing.T) {
	svc, _, _, _ := newService()
	view, err := svc.CreateFromProject(context.Background(), 1, CreateInput{ProjectID: 42})
	if err != nil {
		t.Fatalf("CreateFromProject: %v", err)
	}
	r := view.Rundown
	if r.EventDateLabel != "Minggu, 14 Juni 2026" {
		t.Errorf("tanggal = %q, mau \"Minggu, 14 Juni 2026\"", r.EventDateLabel)
	}
	if r.VenueLabel != "Grand Mercure" {
		t.Errorf("venue = %q", r.VenueLabel)
	}
	if r.CoupleTitle != "“THE WEDDING OF AYU & BAGAS”" {
		t.Errorf("judul lampiran = %q", r.CoupleTitle)
	}
	if r.BrideName != "AYU LESTARI" || r.GroomName != "BAGAS ARYA" {
		t.Errorf("nama pengantin tidak dikapitalkan: %q / %q", r.BrideName, r.GroomName)
	}
}

func TestCreateFromProject_RejectsDuplicate(t *testing.T) {
	svc, _, _, _ := newService()
	ctx := context.Background()
	if _, err := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42}); err != nil {
		t.Fatalf("pembuatan pertama gagal: %v", err)
	}
	_, err := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	if err == nil {
		t.Fatal("project yang sudah punya rundown seharusnya ditolak")
	}
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindConflict {
		t.Errorf("galat = %v, mau Conflict", err)
	}
}

func TestCreateFromProject_PropagatesMissingProject(t *testing.T) {
	svc, _, _, pr := newService()
	pr.ctxErr = apperror.NotFound("Project tidak ditemukan")
	_, err := svc.CreateFromProject(context.Background(), 1, CreateInput{ProjectID: 99})
	if err == nil {
		t.Fatal("project yang tidak ada seharusnya menolak pembuatan")
	}
	if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindNotFound {
		t.Errorf("galat = %v, mau NotFound", err)
	}
}

// Penomoran adalah otoritas server: apa pun yang dikirim klien ditimpa.
func TestReplaceSection_RenumbersAcaraIgnoringClientInput(t *testing.T) {
	svc, repo, _, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})

	_, err := svc.ReplaceSection(ctx, 1, view.Rundown.ID, domain.SectionKeyAcaraAkad,
		SectionPayload{Items: []domain.Item{
			{NoLabel: "-", Item: "Checking dekor"}, // baris tanpa nomor
			{NoLabel: "99", Item: "Clear area"},
			{NoLabel: "", Item: "Soft opening"},
			{NoLabel: "3", Item: "Sambutan"},
		}})
	if err != nil {
		t.Fatalf("ReplaceSection: %v", err)
	}
	got := []string{}
	for _, it := range repo.sections[domain.SectionKeyAcaraAkad].Items {
		got = append(got, it.NoLabel)
	}
	want := []string{"", "1", "2", "3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("penomoran = %v, mau %v", got, want)
	}
}

func TestReplaceSection_RenumbersLayoutPerKind(t *testing.T) {
	svc, repo, _, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})

	if _, err := svc.ReplaceSection(ctx, 1, view.Rundown.ID, domain.SectionKeyLayout,
		SectionPayload{LayoutNotes: []domain.LayoutNote{
			{Kind: domain.LayoutNoteRule, Content: "a"},
			{Kind: domain.LayoutNoteLegend, Content: "b"},
			{Kind: domain.LayoutNoteRule, Content: "c"},
			{Kind: domain.LayoutNoteLegend, Content: "d"},
		}}); err != nil {
		t.Fatalf("ReplaceSection: %v", err)
	}
	notes := repo.sections[domain.SectionKeyLayout].LayoutNotes
	want := []string{"1", "1", "2", "2"}
	for i, n := range notes {
		if n.NumberLabel != want[i] {
			t.Errorf("catatan %d bernomor %q, mau %q", i, n.NumberLabel, want[i])
		}
	}
}

func TestReplaceSection_RejectsUnknownSection(t *testing.T) {
	svc, _, _, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	_, err := svc.ReplaceSection(ctx, 1, view.Rundown.ID, domain.SectionKey("tidak-ada"), SectionPayload{})
	if err == nil {
		t.Fatal("seksi yang tidak dikenal seharusnya ditolak")
	}
}

func TestReplaceSection_RejectsInvalidEnum(t *testing.T) {
	svc, _, _, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	_, err := svc.ReplaceSection(ctx, 1, view.Rundown.ID, domain.SectionKeyDataLainnya,
		SectionPayload{MenuItems: []domain.MenuItem{{GroupKey: "Ngawur", Style: "Numbered"}}})
	if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("galat = %v, mau Validation", err)
	}
}

func TestSaveLayoutImage_RejectsNonPNG(t *testing.T) {
	svc, _, st, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})

	err := svc.SaveLayoutImage(ctx, 1, view.Rundown.ID, []byte("\xff\xd8\xff bukan png"))
	if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("galat = %v, mau Validation", err)
	}
	if len(st.saved) != 0 {
		t.Error("berkas yang ditolak tidak boleh tersimpan ke storage")
	}
}

func TestSaveLayoutImage_ReplacesOldObject(t *testing.T) {
	svc, repo, st, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	repo.byID[view.Rundown.ID].LayoutImagePath = "rundowns/1/1/lama.png"

	if err := svc.SaveLayoutImage(ctx, 1, view.Rundown.ID, tinyPNG(t)); err != nil {
		t.Fatalf("SaveLayoutImage: %v", err)
	}
	if len(st.saved) != 1 {
		t.Fatalf("objek baru tidak tersimpan: %v", st.saved)
	}
	if len(st.deleted) != 1 || st.deleted[0] != "rundowns/1/1/lama.png" {
		t.Errorf("objek lama tidak dihapus: %v", st.deleted)
	}
}

// Project tanpa rundown bukan kegagalan — keadaan yang dituju sudah tercapai.
func TestDeleteRundownForProject_IsIdempotent(t *testing.T) {
	svc, _, _, _ := newService()
	if err := svc.DeleteRundownForProject(context.Background(), 1, 777); err != nil {
		t.Fatalf("project tanpa rundown seharusnya no-op, dapat: %v", err)
	}
}

func TestDeleteRundownForProject_RemovesLayoutObject(t *testing.T) {
	svc, repo, st, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	repo.byID[view.Rundown.ID].LayoutImagePath = "rundowns/1/1/denah.png"

	if err := svc.DeleteRundownForProject(ctx, 1, 42); err != nil {
		t.Fatalf("DeleteRundownForProject: %v", err)
	}
	if repo.deletedID != view.Rundown.ID {
		t.Errorf("baris rundown tidak dihapus (deletedID=%d)", repo.deletedID)
	}
	if len(st.deleted) != 1 {
		t.Errorf("denah tidak ikut dihapus dari storage: %v", st.deleted)
	}
}

// Wedding Planner tanpa satu pun project: hasilnya WAJIB kosong, dan slice
// kosong tidak boleh disamakan dengan nil (yang berarti "tanpa pembatasan").
func TestList_EmptyScopeReturnsNothing(t *testing.T) {
	svc, repo, _, _ := newService()
	repo.listReturns = []domain.Summary{{ID: 1}}
	empty := []int64{}
	rows, total, err := svc.List(context.Background(), 1, ListFilter{ProjectIDs: &empty},
		pagination.Params{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 0 || total != 0 {
		t.Errorf("scope kosong menghasilkan %d baris (total %d), mau 0", len(rows), total)
	}
}

func TestGet_RefreshesStaleProjectName(t *testing.T) {
	svc, repo, _, pr := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	pr.ctx.ProjectName = "Ayu & Bagas (revisi)"

	got, err := svc.Get(ctx, 1, view.Rundown.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Rundown.ProjectName != "Ayu & Bagas (revisi)" {
		t.Errorf("nama project = %q, mau tersegarkan", got.Rundown.ProjectName)
	}
	if repo.byID[view.Rundown.ID].ProjectName != "Ayu & Bagas (revisi)" {
		t.Error("nama project tersegarkan tidak ikut tersimpan")
	}
}

// Rundown yatim (project-nya sudah hilang) tetap harus bisa dibuka Owner/Admin
// supaya barisnya bisa dibersihkan manual.
func TestGet_OrphanRundownStillReadable(t *testing.T) {
	svc, _, _, pr := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	pr.ctxErr = errors.New("project hilang")

	if _, err := svc.Get(ctx, 1, view.Rundown.ID); err != nil {
		t.Fatalf("rundown yatim seharusnya tetap terbaca, dapat: %v", err)
	}
}
