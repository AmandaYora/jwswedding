package application

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"

	"jwswedding/internal/modules/staff/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

// --- fakes ---

type fakeSignatureRepo struct {
	mu      sync.Mutex
	members map[int64]*domain.StaffMember
	// updateSignatureCalls menghitung tulisan ke kolom TTD, supaya tes bisa
	// membuktikan kolomnya TIDAK tersentuh saat langkah sebelumnya gagal.
	updateSignatureCalls int
}

func newFakeSignatureRepo(members ...*domain.StaffMember) *fakeSignatureRepo {
	m := map[int64]*domain.StaffMember{}
	for _, member := range members {
		m[member.ID] = member
	}
	return &fakeSignatureRepo{members: m}
}

func (f *fakeSignatureRepo) FindByID(ctx context.Context, tenantID, id int64) (*domain.StaffMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	member, ok := f.members[id]
	if !ok || member.TenantID != tenantID {
		return nil, nil
	}
	return member, nil
}

func (f *fakeSignatureRepo) UpdateSignature(ctx context.Context, tenantID, id int64, path *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	member, ok := f.members[id]
	if !ok || member.TenantID != tenantID {
		return apperror.NotFound("pengguna tidak ditemukan")
	}
	f.updateSignatureCalls++
	member.SignatureStoragePath = path
	return nil
}

// Sisa StaffRepository tidak dipakai service TTD — cukup dipenuhi agar
// interface-nya terpenuhi.
func (f *fakeSignatureRepo) List(ctx context.Context, tenantID int64) ([]domain.StaffMember, error) {
	return nil, nil
}

func (f *fakeSignatureRepo) ListPaginated(ctx context.Context, tenantID int64, params pagination.Params, search, role string) ([]domain.StaffMember, int64, error) {
	return nil, 0, nil
}
func (f *fakeSignatureRepo) Create(ctx context.Context, member *domain.StaffMember) error { return nil }
func (f *fakeSignatureRepo) Update(ctx context.Context, member *domain.StaffMember) error { return nil }
func (f *fakeSignatureRepo) SetActive(ctx context.Context, tenantID, id int64, isActive bool) error {
	return nil
}
func (f *fakeSignatureRepo) Delete(ctx context.Context, tenantID, id int64) error { return nil }

type fakeSignatureStorage struct {
	mu        sync.Mutex
	saved     map[string][]byte
	saveErr   error
	openErr   error
	deleteErr error
	deleted   []string
}

func newFakeSignatureStorage() *fakeSignatureStorage {
	return &fakeSignatureStorage{saved: map[string][]byte{}}
}

func (f *fakeSignatureStorage) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved[key] = data
	return key, nil
}

func (f *fakeSignatureStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.saved[key]
	if !ok {
		return nil, apperror.NotFound("objek tidak ada")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeSignatureStorage) Delete(ctx context.Context, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, key)
	delete(f.saved, key)
	return nil
}

// validPNG membuat PNG sungguhan — compress.Image benar-benar men-decode dan
// meng-encode ulang, jadi byte sembarangan akan lewat begitu saja tanpa
// membuktikan apa pun.
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

func staffWithoutSignature() *domain.StaffMember {
	return &domain.StaffMember{ID: 7, TenantID: 1, Name: "Anisa Putri", Title: "Lead Planner", Role: domain.RoleStaff, IsActive: true}
}

// --- SaveSignature ---

func TestSaveSignature_PNGValid(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	store := newFakeSignatureStorage()
	svc := NewStaffSignatureService(repo, store)

	if err := svc.SaveSignature(context.Background(), 1, 7, validPNG(t), "image/png"); err != nil {
		t.Fatalf("SaveSignature: %v", err)
	}
	member, _ := repo.FindByID(context.Background(), 1, 7)
	if member.SignatureStoragePath == nil {
		t.Fatal("kolom signature_storage_path harus terisi")
	}
	key := *member.SignatureStoragePath
	if !strings.Contains(key, "signature/staff/1/7") {
		t.Errorf("kunci objek harus ber-namespace staff dan ber-scope tenant+staff, got %q", key)
	}
	if _, ok := store.saved[key]; !ok {
		t.Error("objek tidak tersimpan di kunci yang dicatat kolomnya")
	}
}

func TestSaveSignature_TolakMimeSelainPNGatauJPEG(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	store := newFakeSignatureStorage()
	svc := NewStaffSignatureService(repo, store)

	err := svc.SaveSignature(context.Background(), 1, 7, validPNG(t), "image/webp")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("mau galat validasi untuk MIME tak diizinkan, got %v", err)
	}
	if len(store.saved) != 0 {
		t.Error("storage tidak boleh tersentuh saat MIME ditolak")
	}
	if repo.updateSignatureCalls != 0 {
		t.Error("kolom tidak boleh ditulis saat MIME ditolak")
	}
}

func TestSaveSignature_TolakGambarKosong(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	svc := NewStaffSignatureService(repo, newFakeSignatureStorage())

	err := svc.SaveSignature(context.Background(), 1, 7, nil, "image/png")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("mau galat validasi untuk gambar kosong, got %v", err)
	}
}

func TestSaveSignature_TolakLebihDari2MB(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	svc := NewStaffSignatureService(repo, newFakeSignatureStorage())

	oversized := make([]byte, maxSignatureBytes+1)
	err := svc.SaveSignature(context.Background(), 1, 7, oversized, "image/png")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("mau galat validasi untuk gambar > 2 MB, got %v", err)
	}
}

// Kolom tidak boleh menunjuk objek yang gagal disimpan — baris yang menunjuk
// berkas hilang jauh lebih merusak daripada objek yatim.
func TestSaveSignature_StorageGagalTidakMenulisKolom(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	store := newFakeSignatureStorage()
	store.saveErr = apperror.Internal("storage down")
	svc := NewStaffSignatureService(repo, store)

	err := svc.SaveSignature(context.Background(), 1, 7, validPNG(t), "image/png")
	if err == nil {
		t.Fatal("mau galat saat storage.Save gagal")
	}
	if repo.updateSignatureCalls != 0 {
		t.Error("UpdateSignature tidak boleh dipanggil saat Save sudah gagal")
	}
	member, _ := repo.FindByID(context.Background(), 1, 7)
	if member.SignatureStoragePath != nil {
		t.Error("kolom harus tetap kosong")
	}
}

func TestSaveSignature_StaffTenantLainDitolak(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	svc := NewStaffSignatureService(repo, newFakeSignatureStorage())

	err := svc.SaveSignature(context.Background(), 999, 7, validPNG(t), "image/png")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindNotFound {
		t.Fatalf("mau not-found untuk staff milik tenant lain, got %v", err)
	}
}

// --- SignatureImage ---

func TestSignatureImage_MengembalikanBytesDanTipe(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	store := newFakeSignatureStorage()
	svc := NewStaffSignatureService(repo, store)
	if err := svc.SaveSignature(context.Background(), 1, 7, validPNG(t), "image/png"); err != nil {
		t.Fatalf("SaveSignature: %v", err)
	}

	data, contentType, ok, err := svc.SignatureImage(context.Background(), 1, 7)
	if err != nil {
		t.Fatalf("SignatureImage: %v", err)
	}
	if !ok || len(data) == 0 {
		t.Fatal("mau ok=true dengan bytes terisi")
	}
	if contentType != "image/png" {
		t.Errorf("contentType = %q, mau image/png", contentType)
	}
}

func TestSignatureImage_BelumPunyaTTD(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	svc := NewStaffSignatureService(repo, newFakeSignatureStorage())

	_, _, ok, err := svc.SignatureImage(context.Background(), 1, 7)
	if err != nil {
		t.Fatalf("belum punya TTD bukan galat: %v", err)
	}
	if ok {
		t.Error("mau ok=false")
	}
}

// Kegagalan aset tidak boleh menggagalkan PDF yang memanggilnya — ini kontrak
// yang menahan seluruh alur cetak tetap hidup.
func TestSignatureImage_ObjekGagalDibacaTidakMenggagalkan(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	store := newFakeSignatureStorage()
	svc := NewStaffSignatureService(repo, store)
	if err := svc.SaveSignature(context.Background(), 1, 7, validPNG(t), "image/png"); err != nil {
		t.Fatalf("SaveSignature: %v", err)
	}
	store.openErr = apperror.Internal("object storage down")

	_, _, ok, err := svc.SignatureImage(context.Background(), 1, 7)
	if err != nil {
		t.Fatalf("objek gagal dibaca harus diperlakukan sebagai tanpa TTD, bukan galat: %v", err)
	}
	if ok {
		t.Error("mau ok=false")
	}
}

func TestSignatureImage_StaffTidakDikenal(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	svc := NewStaffSignatureService(repo, newFakeSignatureStorage())

	_, _, ok, err := svc.SignatureImage(context.Background(), 1, 404)
	if err != nil {
		t.Fatalf("staff tak dikenal bukan galat di sini: %v", err)
	}
	if ok {
		t.Error("mau ok=false")
	}
}

// --- DeleteSignature ---

func TestDeleteSignature_MengosongkanKolomDanMenghapusObjek(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	store := newFakeSignatureStorage()
	svc := NewStaffSignatureService(repo, store)
	if err := svc.SaveSignature(context.Background(), 1, 7, validPNG(t), "image/png"); err != nil {
		t.Fatalf("SaveSignature: %v", err)
	}
	member, _ := repo.FindByID(context.Background(), 1, 7)
	key := *member.SignatureStoragePath

	if err := svc.DeleteSignature(context.Background(), 1, 7); err != nil {
		t.Fatalf("DeleteSignature: %v", err)
	}
	member, _ = repo.FindByID(context.Background(), 1, 7)
	if member.SignatureStoragePath != nil {
		t.Error("kolom harus kosong setelah hapus")
	}
	if len(store.deleted) != 1 || store.deleted[0] != key {
		t.Errorf("objek %q harus ikut dihapus, deleted=%v", key, store.deleted)
	}
}

// Objek yatim bukan kerusakan: barisnya sudah bersih, jadi hapus tetap sukses.
func TestDeleteSignature_ObjekGagalDihapusTetapSukses(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	store := newFakeSignatureStorage()
	svc := NewStaffSignatureService(repo, store)
	if err := svc.SaveSignature(context.Background(), 1, 7, validPNG(t), "image/png"); err != nil {
		t.Fatalf("SaveSignature: %v", err)
	}
	store.deleteErr = apperror.Internal("storage down")

	if err := svc.DeleteSignature(context.Background(), 1, 7); err != nil {
		t.Fatalf("kegagalan hapus objek tidak boleh menggagalkan operasi: %v", err)
	}
	member, _ := repo.FindByID(context.Background(), 1, 7)
	if member.SignatureStoragePath != nil {
		t.Error("kolom tetap harus kosong")
	}
}

func TestDeleteSignature_BelumPunyaTTDBukanGalat(t *testing.T) {
	repo := newFakeSignatureRepo(staffWithoutSignature())
	svc := NewStaffSignatureService(repo, newFakeSignatureStorage())

	if err := svc.DeleteSignature(context.Background(), 1, 7); err != nil {
		t.Fatalf("menghapus TTD yang memang belum ada bukan galat: %v", err)
	}
}
