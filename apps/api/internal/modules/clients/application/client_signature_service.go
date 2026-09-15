package application

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/compress"
	"jwswedding/internal/shared/storage"
)

// maxSpecimenBytes adalah batas gambar specimen sebelum diproses (2 MB —
// sama dengan batas unggah endpoint publik magic link, §9).
const maxSpecimenBytes = 2 * 1024 * 1024

// ClientSignatureRepository adalah port penyimpanan specimen — implementasi
// MySQL-nya di infrastructure.
type ClientSignatureRepository interface {
	Upsert(ctx context.Context, s *domain.ClientSignature) error
	FindByClient(ctx context.Context, tenantID, clientID int64) (*domain.ClientSignature, error)
	Delete(ctx context.Context, tenantID, clientID int64) (storageKey string, err error)
}

// SignatureObjectStorage adalah irisan sempit storage.Client yang dipakai
// service ini — interface lokal mengikuti evidence_service.go:55 supaya bisa
// difake tanpa bucket sungguhan.
type SignatureObjectStorage interface {
	Save(ctx context.Context, key string, data []byte, contentType string) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// SignerOption adalah satu pilihan "Atas Nama": peran + nama pemiliknya.
// Dibaca dari clients.bride_name/groom_name (T1) — BUKAN client_contacts
// yang boleh berjumlah nol baris.
type SignerOption struct {
	Role domain.ClientRole
	Name string
}

// ClientSignatureService mengelola specimen TTD (TTD Penawaran, D6b/D12).
type ClientSignatureService struct {
	repo    ClientSignatureRepository
	clients ClientRepository
	storage SignatureObjectStorage
}

func NewClientSignatureService(repo ClientSignatureRepository, clients ClientRepository, storage SignatureObjectStorage) *ClientSignatureService {
	return &ClientSignatureService{repo: repo, clients: clients, storage: storage}
}

// SaveSpecimen menyimpan/menimpa specimen milik client. Menimpa, tidak
// menumpuk (D6b): masukan baru menggantikan siapa pun pemilik lamanya (D12).
func (s *ClientSignatureService) SaveSpecimen(ctx context.Context, tenantID, clientID int64, role domain.ClientRole, signerName string, img []byte, mimeType string, source domain.ClientSignatureSource) (*domain.ClientSignature, error) {
	if _, err := s.requireClient(ctx, tenantID, clientID); err != nil {
		return nil, err
	}
	if role != domain.RoleBride && role != domain.RoleGroom && role != domain.RoleFamilyRepresentative {
		return nil, apperror.Validation("Peran penanda tangan tidak dikenal", map[string][]string{
			"role": {"Pilih Bride, Groom, atau Family Representative"},
		})
	}
	if strings.TrimSpace(signerName) == "" {
		return nil, apperror.Validation("Nama penanda tangan wajib diisi", map[string][]string{
			"signerName": {"Isi nama penanda tangan"},
		})
	}
	if source != domain.SignatureSourceDraw && source != domain.SignatureSourceUpload {
		return nil, apperror.Validation("Sumber tanda tangan tidak dikenal", map[string][]string{
			"source": {"Sumber harus draw atau upload"},
		})
	}
	data, contentType, err := normalizeSignatureImage(img, mimeType)
	if err != nil {
		return nil, err
	}
	key := storage.BuildClientSignatureKey(strconv.FormatInt(tenantID, 10), strconv.FormatInt(clientID, 10))
	if _, err := s.storage.Save(ctx, key, data, contentType); err != nil {
		return nil, err
	}
	spec := &domain.ClientSignature{
		TenantID: tenantID, ClientID: clientID, Role: role,
		SignerName: strings.TrimSpace(signerName), StorageKey: key, Source: source,
	}
	// Objek yatim bila Upsert gagal setelah Save berhasil — dibiarkan, sama
	// dengan urutan yang dipakai EvidenceService (§14).
	if err := s.repo.Upsert(ctx, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

// Specimen mengembalikan specimen milik client, atau nil bila belum ada —
// bukan error. Tanpa parameter role (D12): hasilnya selalu nol atau satu.
func (s *ClientSignatureService) Specimen(ctx context.Context, tenantID, clientID int64) (*domain.ClientSignature, error) {
	if _, err := s.requireClient(ctx, tenantID, clientID); err != nil {
		return nil, err
	}
	return s.repo.FindByClient(ctx, tenantID, clientID)
}

// SpecimenImage mengembalikan bytes gambar specimen untuk dipakai ulang
// (jalur "Pakai TTD tersimpan"). Gagal bila tidak ada specimen.
func (s *ClientSignatureService) SpecimenImage(ctx context.Context, tenantID, clientID int64) ([]byte, error) {
	spec, err := s.Specimen(ctx, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, apperror.NotFound("TTD tersimpan tidak ditemukan — unggah foto TTD sebagai gantinya")
	}
	rc, err := s.storage.Open(ctx, spec.StorageKey)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DeleteSpecimen menghapus specimen (baris LALU objeknya, D6d). Urutan
// disengaja — kegagalan di langkah kedua meninggalkan objek yatim, jauh
// lebih murah daripada baris yang menunjuk berkas yang tidak ada (idiom
// DeleteProjectCascade). Dokumen yang sudah diteken tidak tersentuh: mereka
// memegang salinannya sendiri.
func (s *ClientSignatureService) DeleteSpecimen(ctx context.Context, tenantID, clientID int64) error {
	if _, err := s.requireClient(ctx, tenantID, clientID); err != nil {
		return err
	}
	key, err := s.repo.Delete(ctx, tenantID, clientID)
	if err != nil {
		return err
	}
	if key == "" {
		return nil
	}
	// Best-effort: barisnya sudah hilang, objek yatim bukan kerusakan.
	_ = s.storage.Delete(ctx, key)
	return nil
}

// SignerOptions mengembalikan dua opsi Atas Nama dari master clients (T1).
func (s *ClientSignatureService) SignerOptions(ctx context.Context, tenantID, clientID int64) ([]SignerOption, error) {
	c, err := s.requireClient(ctx, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	return []SignerOption{
		{Role: domain.RoleBride, Name: c.BrideName},
		{Role: domain.RoleGroom, Name: c.GroomName},
	}, nil
}

func (s *ClientSignatureService) requireClient(ctx context.Context, tenantID, clientID int64) (*domain.Client, error) {
	c, err := s.clients.FindByID(ctx, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperror.NotFound("Client tidak ditemukan")
	}
	return c, nil
}

// normalizeSignatureImage menolak selain PNG/JPEG dan yang lebih dari 2 MB,
// lalu me-re-encode lewat compress.Image sebelum disimpan — apa pun yang
// menumpang di luar piksel ikut hilang (§9). Dipakai kedua jalur masuk
// (unggah pengelola maupun goresan klien).
func normalizeSignatureImage(img []byte, mimeType string) (data []byte, contentType string, err error) {
	switch mimeType {
	case "image/png", "image/jpeg", "image/jpg":
	default:
		return nil, "", apperror.Validation("Jenis berkas harus gambar (PNG atau JPEG)", map[string][]string{
			"mimeType": {"Unggah foto TTD berekstensi PNG atau JPG"},
		})
	}
	if len(img) == 0 {
		return nil, "", apperror.Validation("Gambar tanda tangan kosong", map[string][]string{
			"base64Data": {"Gambar tanda tangan tidak boleh kosong"},
		})
	}
	if len(img) > maxSpecimenBytes {
		return nil, "", apperror.Validation("Gambar tanda tangan terlalu besar (maksimal 2 MB)", map[string][]string{
			"base64Data": {"Perkecil gambarnya lalu coba lagi"},
		})
	}
	out, err := compress.Image(img, mimeType)
	if err != nil {
		return nil, "", err
	}
	if mimeType == "image/jpg" {
		mimeType = "image/jpeg"
	}
	return out, mimeType, nil
}
