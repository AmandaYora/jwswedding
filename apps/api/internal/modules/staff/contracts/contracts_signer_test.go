package contracts

// Kontrak GetSigner (PLAN tanda-tangan-pengguna T31). Ini satu-satunya pintu
// yang dipakai `projects` dan `quotations` untuk mengisi blok tanda tangan PDF,
// jadi yang dikunci di sini adalah janjinya: tidak ter-resolve = (nil, nil),
// BUKAN error — karena satu ID yatim tidak boleh menggagalkan pencetakan
// dokumen.

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"jwswedding/internal/modules/staff/application"
	"jwswedding/internal/modules/staff/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

type fakeRepo struct{ members map[int64]*domain.StaffMember }

func newFakeRepo(members ...*domain.StaffMember) *fakeRepo {
	m := map[int64]*domain.StaffMember{}
	for _, member := range members {
		m[member.ID] = member
	}
	return &fakeRepo{members: m}
}

func (f *fakeRepo) FindByID(ctx context.Context, tenantID, id int64) (*domain.StaffMember, error) {
	member, ok := f.members[id]
	if !ok || member.TenantID != tenantID {
		return nil, nil
	}
	return member, nil
}

func (f *fakeRepo) UpdateSignature(ctx context.Context, tenantID, id int64, path *string) error {
	member, ok := f.members[id]
	if !ok {
		return apperror.NotFound("pengguna tidak ditemukan")
	}
	member.SignatureStoragePath = path
	return nil
}

func (f *fakeRepo) List(ctx context.Context, tenantID int64) ([]domain.StaffMember, error) {
	return nil, nil
}

func (f *fakeRepo) ListPaginated(ctx context.Context, tenantID int64, params pagination.Params, search, role string) ([]domain.StaffMember, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Create(ctx context.Context, member *domain.StaffMember) error { return nil }
func (f *fakeRepo) Update(ctx context.Context, member *domain.StaffMember) error { return nil }
func (f *fakeRepo) SetActive(ctx context.Context, tenantID, id int64, isActive bool) error {
	return nil
}
func (f *fakeRepo) Delete(ctx context.Context, tenantID, id int64) error { return nil }

type fakeStorage struct{ objects map[string][]byte }

func (f *fakeStorage) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	f.objects[key] = data
	return key, nil
}

func (f *fakeStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, apperror.NotFound("objek tidak ada")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeStorage) Delete(ctx context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

func validPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 30, G: 41, B: 59, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// build menyiapkan contracts dengan satu staff bertanda tangan (id 7), satu
// tanpa tanda tangan (id 8), satu nonaktif bertanda tangan (id 9), dan satu
// milik tenant lain (id 10).
func build(t *testing.T) Contracts {
	t.Helper()
	pathSeven := "jwswedding/signature/staff/1/7/specimen.png"
	pathNine := "jwswedding/signature/staff/1/9/specimen.png"
	repo := newFakeRepo(
		&domain.StaffMember{ID: 7, TenantID: 1, Name: "Anisa Putri", Title: "Lead Planner", Role: domain.RoleStaff, IsActive: true, SignatureStoragePath: &pathSeven},
		&domain.StaffMember{ID: 8, TenantID: 1, Name: "Rara", Title: "Koordinator", Role: domain.RoleStaff, IsActive: true},
		&domain.StaffMember{ID: 9, TenantID: 1, Name: "Bayu", Title: "Sales", Role: domain.RoleSales, IsActive: false, SignatureStoragePath: &pathNine},
		&domain.StaffMember{ID: 10, TenantID: 2, Name: "Tenant Lain", Title: "Owner", Role: domain.RoleOwner, IsActive: true},
	)
	store := &fakeStorage{objects: map[string][]byte{
		pathSeven: validPNG(t),
		pathNine:  validPNG(t),
	}}
	return New(application.NewStaffService(repo, nil), application.NewStaffSignatureService(repo, store))
}

func TestGetSigner_StaffBertandaTangan(t *testing.T) {
	signer, err := build(t).GetSigner(context.Background(), 1, 7)
	if err != nil {
		t.Fatalf("GetSigner: %v", err)
	}
	if signer == nil {
		t.Fatal("mau *Signer, got nil")
	}
	if signer.Name != "Anisa Putri" || signer.Title != "Lead Planner" {
		t.Errorf("nama/jabatan = %q/%q", signer.Name, signer.Title)
	}
	if len(signer.SignatureImage) == 0 {
		t.Error("SignatureImage harus terisi")
	}
	if signer.SignatureContentType != "image/png" {
		t.Errorf("SignatureContentType = %q, mau image/png", signer.SignatureContentType)
	}
}

// K4: staff-nya ada tapi belum punya TTD — namanya TETAP tercetak, hanya
// gambarnya yang kosong.
func TestGetSigner_StaffTanpaTandaTangan(t *testing.T) {
	signer, err := build(t).GetSigner(context.Background(), 1, 8)
	if err != nil {
		t.Fatalf("GetSigner: %v", err)
	}
	if signer == nil {
		t.Fatal("mau *Signer walau tanpa TTD, got nil")
	}
	if signer.Name != "Rara" {
		t.Errorf("nama = %q, mau Rara", signer.Name)
	}
	if signer.SignatureImage != nil {
		t.Error("SignatureImage harus nil")
	}
}

// Staff nonaktif tetap sah sebagai pengesah dokumen yang dulu dia terbitkan —
// GetSigner sengaja tidak menyaring is_active.
func TestGetSigner_StaffNonaktifTetapTerResolve(t *testing.T) {
	signer, err := build(t).GetSigner(context.Background(), 1, 9)
	if err != nil {
		t.Fatalf("GetSigner: %v", err)
	}
	if signer == nil {
		t.Fatal("staff nonaktif harus tetap ter-resolve")
	}
	if len(signer.SignatureImage) == 0 {
		t.Error("TTD staff nonaktif harus tetap terbaca")
	}
}

// K6: ketiga bentuk "tidak ter-resolve" harus (nil, nil), bukan error.
func TestGetSigner_TidakTerResolveBukanError(t *testing.T) {
	c := build(t)
	cases := []struct {
		nama     string
		tenantID int64
		staffID  int64
	}{
		{"sentinel 0 pada baris lama", 1, 0},
		{"staff sudah dihapus permanen", 1, 404},
		{"staff milik tenant lain", 1, 10},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			signer, err := c.GetSigner(context.Background(), tc.tenantID, tc.staffID)
			if err != nil {
				t.Fatalf("mau nil error, got %v", err)
			}
			if signer != nil {
				t.Errorf("mau nil Signer, got %+v", signer)
			}
		})
	}
}
