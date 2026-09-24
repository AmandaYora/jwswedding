package application

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/apperror"
)

// fakeTemplateRepo menyimpan template di memori, dengan semantik yang sama
// dengan MySQLRundownTemplateRepository: ReplaceSection hanya mengganti satu
// seksi, ReplaceAll mengganti semuanya.
type fakeTemplateRepo struct {
	t      *domain.Template
	getErr error
}

func (f *fakeTemplateRepo) Get(_ context.Context, _ int64) (*domain.Template, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.t == nil {
		return nil, nil
	}
	copied := *f.t
	return &copied, nil
}

func (f *fakeTemplateRepo) ReplaceSection(_ context.Context, tenantID int64, section domain.SectionKey, p SectionPayload) error {
	if f.t == nil {
		f.t = &domain.Template{TenantID: tenantID}
	}
	switch section {
	case domain.SectionKeyRoles:
		f.t.Roles = p.Roles
	case domain.SectionKeyCommittees:
		f.t.Committees = p.Committees
	case domain.SectionKeyMakeup:
		f.t.MakeupRooms = p.MakeupRooms
	case domain.SectionKeyAcaraAkad:
		f.t.ItemsAkad = p.Items
	case domain.SectionKeyAcaraResepsi:
		f.t.ItemsResepsi = p.Items
	case domain.SectionKeyLayout:
		f.t.LayoutNotes = p.LayoutNotes
	}
	return nil
}

func (f *fakeTemplateRepo) ReplaceAll(_ context.Context, t *domain.Template) error {
	copied := *t
	f.t = &copied
	return nil
}

// aggregateRepo mengembalikan aggregate yang sudah terisi untuk SaveFromRundown.
type aggregateRepo struct {
	RundownRepository
	view *domain.View
}

func (a *aggregateRepo) LoadAggregate(context.Context, int64, int64) (*domain.View, error) {
	return a.view, nil
}

// --------------------------------------------------------- RundownTemplateService

func TestTemplateService_Get_EmptyWhenNoRow(t *testing.T) {
	svc := NewRundownTemplateService(&fakeTemplateRepo{}, nil)
	tpl, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tpl == nil || !tpl.IsEmpty() {
		t.Errorf("tenant tanpa template harus menerima template kosong, dapat %+v", tpl)
	}
}

func TestTemplateService_ReplaceSection_RejectsNonTemplateKey(t *testing.T) {
	svc := NewRundownTemplateService(&fakeTemplateRepo{}, nil)
	for _, key := range []domain.SectionKey{domain.SectionKeyCover, domain.SectionKeyVendors, domain.SectionKeyTamuVIP} {
		_, err := svc.ReplaceSection(context.Background(), 1, key, SectionPayload{})
		if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindValidation {
			t.Errorf("seksi %s: galat = %v, mau Validation", key, err)
		}
	}
}

func TestTemplateService_ReplaceSection_RenumbersAcara(t *testing.T) {
	repo := &fakeTemplateRepo{}
	svc := NewRundownTemplateService(repo, nil)
	tpl, err := svc.ReplaceSection(context.Background(), 1, domain.SectionKeyAcaraAkad, SectionPayload{
		Items: []domain.Item{{NoLabel: "-", Item: "Checking dekor"}, {NoLabel: "9", Item: "Akad"}, {Item: "Sungkeman"}},
	})
	if err != nil {
		t.Fatalf("ReplaceSection: %v", err)
	}
	got := []string{tpl.ItemsAkad[0].NoLabel, tpl.ItemsAkad[1].NoLabel, tpl.ItemsAkad[2].NoLabel}
	want := []string{"", "1", "2"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("baris %d: nomor = %q, mau %q", i, got[i], want[i])
		}
	}
}

func TestTemplateService_ReplaceSection_ValidatesLength(t *testing.T) {
	svc := NewRundownTemplateService(&fakeTemplateRepo{}, nil)
	long := string(bytes.Repeat([]byte("a"), 151))
	_, err := svc.ReplaceSection(context.Background(), 1, domain.SectionKeyRoles,
		SectionPayload{Roles: []domain.Role{{RoleLabel: long}}})
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("galat = %v, mau Validation", err)
	}
	if _, has := appErr.Fields["roles[0].roleLabel"]; !has {
		t.Errorf("field galat = %v, mau berisi roles[0].roleLabel", appErr.Fields)
	}
}

func TestTemplateService_SaveFromRundown_ClearsPersonNames(t *testing.T) {
	repo := &fakeTemplateRepo{}
	view := &domain.View{
		Roles:      []domain.Role{{RoleLabel: "Wali Nikah CPW", PersonName: "Bapak Lukman", Note: "ayah kandung"}},
		Committees: []domain.Committee{{RoleLabel: "Among Tamu", PersonText: "Rina\nSari", JobDesc: "Menyambut tamu"}},
		Items: []domain.Item{
			{Section: domain.SectionAkad, NoLabel: "1", Item: "Akad nikah", PIC: "MC"},
			{Section: domain.SectionResepsi, NoLabel: "1", Item: "Kirab"},
		},
		MakeupRooms: []domain.MakeupRoom{{RoomLabel: "Ruang CPW"}},
		LayoutNotes: []domain.LayoutNote{{Kind: domain.LayoutNoteRule, NumberLabel: "1", Content: "Tamu duduk"}},
	}
	svc := NewRundownTemplateService(repo, &aggregateRepo{view: view})

	tpl, err := svc.SaveFromRundown(context.Background(), 1, 5)
	if err != nil {
		t.Fatalf("SaveFromRundown: %v", err)
	}
	if tpl.Roles[0].PersonName != "" || tpl.Committees[0].PersonText != "" {
		t.Error("nama orang ikut tersalin ke template")
	}
	if tpl.Roles[0].RoleLabel != "Wali Nikah CPW" || tpl.Roles[0].Note != "ayah kandung" ||
		tpl.Committees[0].JobDesc != "Menyambut tamu" {
		t.Error("peran/keterangan tidak tersalin utuh")
	}
	if len(tpl.ItemsAkad) != 1 || len(tpl.ItemsResepsi) != 1 {
		t.Errorf("susunan acara tidak terbagi akad/resepsi: akad=%d resepsi=%d", len(tpl.ItemsAkad), len(tpl.ItemsResepsi))
	}
	if len(tpl.MakeupRooms) != 1 || len(tpl.LayoutNotes) != 1 {
		t.Error("ruangan makeup / catatan layout tidak tersalin")
	}
}

func TestTemplateService_SaveFromRundown_NotFound(t *testing.T) {
	svc := NewRundownTemplateService(&fakeTemplateRepo{}, &aggregateRepo{})
	_, err := svc.SaveFromRundown(context.Background(), 1, 5)
	if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindNotFound {
		t.Errorf("galat = %v, mau NotFound", err)
	}
}

// ------------------------------------------------------ RundownService + template

func newServiceWithTemplate(tpl *domain.Template) (*RundownService, *fakeRepo) {
	_, repo, st, pr := newService()
	return NewRundownService(repo, pr, st, &fakeTemplateRepo{t: tpl}), repo
}

func TestCreateFromProject_AppliesTemplateWhenRequested(t *testing.T) {
	svc, repo := newServiceWithTemplate(&domain.Template{
		Roles:     []domain.Role{{RoleLabel: "Saksi"}},
		ItemsAkad: []domain.Item{{NoLabel: "1", Item: "Akad"}},
		// Seksi template yang kosong dilewati, bukan ditulis sebagai kosong.
	})
	if _, err := svc.CreateFromProject(context.Background(), 1, CreateInput{ProjectID: 42, UseTemplate: true}); err != nil {
		t.Fatalf("CreateFromProject: %v", err)
	}
	if got := repo.sections[domain.SectionKeyRoles].Roles; len(got) != 1 || got[0].RoleLabel != "Saksi" {
		t.Errorf("seksi List Nama tidak tersalin: %+v", got)
	}
	if got := repo.sections[domain.SectionKeyAcaraAkad].Items; len(got) != 1 {
		t.Errorf("susunan akad tidak tersalin: %+v", got)
	}
	if _, written := repo.sections[domain.SectionKeyCommittees]; written {
		t.Error("seksi template yang kosong tidak boleh ditulis")
	}
}

func TestCreateFromProject_IgnoresTemplateWhenNotRequested(t *testing.T) {
	svc, repo := newServiceWithTemplate(&domain.Template{Roles: []domain.Role{{RoleLabel: "Saksi"}}})
	if _, err := svc.CreateFromProject(context.Background(), 1, CreateInput{ProjectID: 42}); err != nil {
		t.Fatalf("CreateFromProject: %v", err)
	}
	if len(repo.sections) != 0 {
		t.Errorf("tanpa UseTemplate tidak ada seksi yang boleh ditulis, dapat %v", repo.sections)
	}
}

// Template yang gagal dibaca tidak boleh menggagalkan pembuatan rundown.
func TestCreateFromProject_TemplateErrorDoesNotFailCreate(t *testing.T) {
	_, repo, st, pr := newService()
	svc := NewRundownService(repo, pr, st, &fakeTemplateRepo{getErr: context.DeadlineExceeded})
	if _, err := svc.CreateFromProject(context.Background(), 1, CreateInput{ProjectID: 42, UseTemplate: true}); err != nil {
		t.Fatalf("CreateFromProject gagal karena template: %v", err)
	}
}

// -------------------------------------------------------------- tarik ulang

func TestProjectPrefill_MatchesCreateMapping(t *testing.T) {
	svc, _, _, pr := newService()
	pr.ctx.EventStartTime = "08.00"
	pr.ctx.EventEndTime = "13.00"
	ctx := context.Background()
	view, err := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	if err != nil {
		t.Fatalf("CreateFromProject: %v", err)
	}

	cover, err := svc.ProjectPrefill(ctx, 1, view.Rundown.ID)
	if err != nil {
		t.Fatalf("ProjectPrefill: %v", err)
	}
	r := view.Rundown
	if cover.GroomName != r.GroomName || cover.BrideName != r.BrideName ||
		cover.EventDateLabel != r.EventDateLabel || cover.VenueLabel != r.VenueLabel ||
		cover.EventTimeLabel != r.EventTimeLabel || cover.CoupleTitle != r.CoupleTitle {
		t.Errorf("tarik ulang = %+v, berbeda dari hasil create %+v", cover, r)
	}
	if cover.EventTimeLabel != "08.00 - 13.00 wib" {
		t.Errorf("jam = %q, mau \"08.00 - 13.00 wib\"", cover.EventTimeLabel)
	}
}

func TestProjectPrefill_ProjectGone(t *testing.T) {
	svc, _, _, pr := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})
	pr.ctxErr = apperror.NotFound("Project tidak ditemukan")

	_, err := svc.ProjectPrefill(ctx, 1, view.Rundown.ID)
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindNotFound || appErr.Message != "Project rundown ini sudah tidak ada" {
		t.Errorf("galat = %v, mau NotFound dengan pesan yang jelas", err)
	}
}

// -------------------------------------------------------------- denah JPG

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 80, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// storageCapture menyimpan byte terakhir yang di-Save, untuk memeriksa format.
type storageCapture struct {
	fakeStorage
	lastData []byte
	lastType string
}

func (s *storageCapture) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	s.lastData, s.lastType = data, contentType
	return s.fakeStorage.Save(ctx, key, data, contentType)
}

func TestSaveLayoutImage_AcceptsJPEGAsPNG(t *testing.T) {
	_, repo, _, pr := newService()
	st := &storageCapture{}
	svc := NewRundownService(repo, pr, st, nil)
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})

	if err := svc.SaveLayoutImage(ctx, 1, view.Rundown.ID, testJPEG(t, 40, 30)); err != nil {
		t.Fatalf("SaveLayoutImage JPEG: %v", err)
	}
	if st.lastType != "image/png" {
		t.Errorf("content-type = %q, mau image/png", st.lastType)
	}
	if _, err := png.Decode(bytes.NewReader(st.lastData)); err != nil {
		t.Errorf("yang tersimpan bukan PNG: %v", err)
	}
}

func TestSaveLayoutImage_RejectsGIF(t *testing.T) {
	svc, _, st, _ := newService()
	ctx := context.Background()
	view, _ := svc.CreateFromProject(ctx, 1, CreateInput{ProjectID: 42})

	gif := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	err := svc.SaveLayoutImage(ctx, 1, view.Rundown.ID, gif)
	if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("galat = %v, mau Validation untuk GIF", err)
	}
	if len(st.saved) != 0 {
		t.Error("GIF yang ditolak tidak boleh tersimpan")
	}
}
