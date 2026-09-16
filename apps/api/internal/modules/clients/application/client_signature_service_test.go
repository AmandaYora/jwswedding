package application

// Tes specimen TTD (§13 PLAN ttd-penawaran): menimpa bukan menumpuk (D6b),
// menimpa lintas peran (D12), penolakan jenis/ukuran, hapus baris+objek
// (D6d), dan opsi penanda tangan dari clients (T1).

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

type fakeClientRepo struct {
	rows map[int64]*domain.Client
}

func newFakeClientRepo() *fakeClientRepo {
	return &fakeClientRepo{rows: map[int64]*domain.Client{
		7: {ID: 7, TenantID: 1, BrideName: "Rara", GroomName: "Dafa"},
	}}
}

func (f *fakeClientRepo) FindByID(_ context.Context, _ int64, id int64) (*domain.Client, error) {
	if c, ok := f.rows[id]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeClientRepo) FindByIDs(_ context.Context, _ int64, ids []int64) ([]domain.Client, error) {
	var out []domain.Client
	for _, id := range ids {
		if c, ok := f.rows[id]; ok {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (f *fakeClientRepo) ListPaginated(_ context.Context, _ int64, _ pagination.Params, _ ClientListFilter) ([]domain.Client, int64, error) {
	panic("not implemented")
}

func (f *fakeClientRepo) Create(_ context.Context, _ *domain.Client) error { panic("not implemented") }

func (f *fakeClientRepo) Update(_ context.Context, _ *domain.Client) error { panic("not implemented") }

func (f *fakeClientRepo) Delete(_ context.Context, _ int64, _ int64) error { panic("not implemented") }

type fakeSignatureRepo struct {
	rows map[int64]*domain.ClientSignature
}

func newFakeSignatureRepo() *fakeSignatureRepo {
	return &fakeSignatureRepo{rows: map[int64]*domain.ClientSignature{}}
}

func (f *fakeSignatureRepo) Upsert(_ context.Context, s *domain.ClientSignature) error {
	cp := *s
	f.rows[s.ClientID] = &cp
	return nil
}

func (f *fakeSignatureRepo) FindByClient(_ context.Context, _ int64, clientID int64) (*domain.ClientSignature, error) {
	if s, ok := f.rows[clientID]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeSignatureRepo) Delete(_ context.Context, _ int64, clientID int64) (string, error) {
	s, ok := f.rows[clientID]
	if !ok {
		return "", nil
	}
	delete(f.rows, clientID)
	return s.StorageKey, nil
}

type fakeSignatureStorage struct {
	docs    map[string][]byte
	deleted []string
}

func newFakeSignatureStorage() *fakeSignatureStorage {
	return &fakeSignatureStorage{docs: map[string][]byte{}}
}

func (f *fakeSignatureStorage) Save(_ context.Context, key string, data []byte, _ string) (string, error) {
	f.docs[key] = append([]byte(nil), data...)
	return key, nil
}

func (f *fakeSignatureStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := f.docs[key]
	if !ok {
		return nil, errors.New("object tidak ditemukan: " + key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeSignatureStorage) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	delete(f.docs, key)
	return nil
}

func newSpecimenFixture() (*ClientSignatureService, *fakeSignatureRepo, *fakeSignatureStorage) {
	repo := newFakeSignatureRepo()
	storage := newFakeSignatureStorage()
	svc := NewClientSignatureService(repo, newFakeClientRepo(), storage)
	return svc, repo, storage
}

func specimenPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.Black)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG uji: %v", err)
	}
	return buf.Bytes()
}

func TestClientSignature_Save_MenimpaBukanMenumpuk(t *testing.T) {
	svc, repo, _ := newSpecimenFixture()
	ctx := context.Background()
	img := specimenPNG(t)

	if _, err := svc.SaveSpecimen(ctx, 1, 7, domain.RoleBride, "Rara", img, "image/png", domain.SignatureSourceUpload); err != nil {
		t.Fatalf("simpan pertama: %v", err)
	}
	if _, err := svc.SaveSpecimen(ctx, 1, 7, domain.RoleBride, "Rara", img, "image/png", domain.SignatureSourceDraw); err != nil {
		t.Fatalf("simpan kedua: %v", err)
	}
	// D6b: dua kali simpan menghasilkan SATU baris — bukan riwayat.
	if len(repo.rows) != 1 {
		t.Errorf("baris = %d, mau 1 (menimpa, bukan menumpuk)", len(repo.rows))
	}
}

func TestClientSignature_Save_MenimpaLintasPeran(t *testing.T) {
	svc, repo, _ := newSpecimenFixture()
	ctx := context.Background()
	img := specimenPNG(t)

	if _, err := svc.SaveSpecimen(ctx, 1, 7, domain.RoleGroom, "Dafa", img, "image/png", domain.SignatureSourceUpload); err != nil {
		t.Fatalf("simpan Groom: %v", err)
	}
	spec, err := svc.SaveSpecimen(ctx, 1, 7, domain.RoleBride, "Rara", img, "image/png", domain.SignatureSourceDraw)
	if err != nil {
		t.Fatalf("simpan Bride: %v", err)
	}
	// D12: tetap satu baris, pemiliknya kini Bride — yang lama hilang.
	if len(repo.rows) != 1 {
		t.Fatalf("baris = %d, mau 1", len(repo.rows))
	}
	if spec.Role != domain.RoleBride || spec.SignerName != "Rara" {
		t.Errorf("specimen = %+v, mau milik Bride/Rara", spec)
	}
	if repo.rows[7].Role != domain.RoleBride {
		t.Errorf("baris tersimpan milik %q, mau Bride", repo.rows[7].Role)
	}
}

func TestClientSignature_Save_TolakJenisBerkasLain(t *testing.T) {
	svc, _, storage := newSpecimenFixture()

	_, err := svc.SaveSpecimen(context.Background(), 1, 7, domain.RoleBride, "Rara",
		[]byte("%PDF-1.4"), "application/pdf", domain.SignatureSourceUpload)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
		t.Errorf("error = %v, mau Validation", err)
	}
	if len(storage.docs) != 0 {
		t.Error("storage tersentuh padahal jenis berkas ditolak")
	}
}

func TestClientSignature_Save_TolakTerlaluBesar(t *testing.T) {
	svc, _, _ := newSpecimenFixture()

	_, err := svc.SaveSpecimen(context.Background(), 1, 7, domain.RoleBride, "Rara",
		make([]byte, maxSpecimenBytes+1), "image/png", domain.SignatureSourceUpload)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
		t.Errorf("error = %v, mau Validation", err)
	}
}

func TestClientSignature_Delete_MenghapusBarisDanObjek(t *testing.T) {
	svc, repo, storage := newSpecimenFixture()
	ctx := context.Background()

	if _, err := svc.SaveSpecimen(ctx, 1, 7, domain.RoleBride, "Rara", specimenPNG(t), "image/png", domain.SignatureSourceUpload); err != nil {
		t.Fatalf("simpan: %v", err)
	}
	key := repo.rows[7].StorageKey
	if err := svc.DeleteSpecimen(ctx, 1, 7); err != nil {
		t.Fatalf("hapus: %v", err)
	}
	// D6d: baris hilang, storage.Delete terpanggil dengan kunci specimen.
	if len(repo.rows) != 0 {
		t.Error("baris specimen masih ada setelah hapus")
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != key {
		t.Errorf("deleted = %v, mau [%q]", storage.deleted, key)
	}
}

func TestClientSignature_SignerOptions_DariClientsBukanContacts(t *testing.T) {
	svc, _, _ := newSpecimenFixture()

	// Client 7 tidak punya satu pun client_contacts — opsi tetap dua (T1).
	options, err := svc.SignerOptions(context.Background(), 1, 7)
	if err != nil {
		t.Fatalf("SignerOptions: %v", err)
	}
	if len(options) != 2 || options[0] != (SignerOption{Role: domain.RoleBride, Name: "Rara"}) ||
		options[1] != (SignerOption{Role: domain.RoleGroom, Name: "Dafa"}) {
		t.Errorf("options = %+v, mau [{Bride Rara} {Groom Dafa}]", options)
	}
}

func TestClientSignature_SpecimenImage_TanpaSpecimen_404(t *testing.T) {
	svc, _, _ := newSpecimenFixture()

	_, err := svc.SpecimenImage(context.Background(), 1, 7)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindNotFound {
		t.Errorf("error = %v, mau 404", err)
	}
}
