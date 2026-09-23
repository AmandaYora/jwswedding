package application

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"

	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/compress"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/storage"
)

// maxSignatureBytes adalah batas gambar TTD sebelum diproses — sama dengan
// batas specimen TTD klien (clients/application), supaya kedua jalur TTD di
// sistem ini punya ambang yang sama.
const maxSignatureBytes = 2 * 1024 * 1024

// SignatureObjectStorage adalah irisan sempit storage.Client yang dipakai
// service ini — interface lokal supaya bisa difake tanpa bucket sungguhan,
// mengikuti idiom yang sama di clients/application.
type SignatureObjectStorage interface {
	Save(ctx context.Context, key string, data []byte, contentType string) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// StaffSignatureService mengelola TTD milik pengguna internal WO sebagai
// MASTER DATA (PLAN tanda-tangan-pengguna): satu objek per staff, ditimpa saat
// ada TTD baru, boleh dihapus kapan saja.
//
// Dokumen tidak menyalin gambar ini — dokumen hanya memaku SIAPA pengesahnya
// (created_by_staff_id) saat terbit, lalu gambarnya dibaca dari sini saat
// dicetak. Maka menghapus TTD di sini membuat dokumen lama tercetak dengan
// ruang kosong untuk tanda tangan basah, bukan dengan TTD orang lain.
type StaffSignatureService struct {
	repo    StaffRepository
	storage SignatureObjectStorage
}

func NewStaffSignatureService(repo StaffRepository, storage SignatureObjectStorage) *StaffSignatureService {
	return &StaffSignatureService{repo: repo, storage: storage}
}

// SaveSignature menyimpan/menimpa TTD milik satu pengguna.
//
// Urutan disengaja: objek disimpan dulu, baru barisnya diperbarui. Kegagalan
// di langkah kedua meninggalkan objek yatim — jauh lebih murah daripada baris
// yang menunjuk berkas yang tidak ada (idiom yang sama dipakai EvidenceService
// dan ClientSignatureService).
func (s *StaffSignatureService) SaveSignature(ctx context.Context, tenantID, staffID int64, img []byte, mimeType string) error {
	member, err := s.repo.FindByID(ctx, tenantID, staffID)
	if err != nil {
		return err
	}
	if member == nil {
		return apperror.NotFound("Pengguna tidak ditemukan")
	}
	data, contentType, err := normalizeStaffSignatureImage(img, mimeType)
	if err != nil {
		return err
	}
	key := storage.BuildStaffSignatureKey(strconv.FormatInt(tenantID, 10), strconv.FormatInt(staffID, 10))
	if _, err := s.storage.Save(ctx, key, data, contentType); err != nil {
		return apperror.Internal("Gagal mengunggah tanda tangan ke object storage")
	}
	return s.repo.UpdateSignature(ctx, tenantID, staffID, &key)
}

// SignatureImage mengembalikan bytes TTD milik satu pengguna.
//
// ok=false BUKAN error, dan itu disengaja: pengguna yang belum punya TTD,
// baris staff yang sudah terhapus, dan objek yang gagal dibaca semuanya
// menghasilkan ok=false. Kegagalan aset tidak boleh menggagalkan PDF yang
// memanggilnya — kontrak yang sama persis dengan platform's GetTenantLogo.
func (s *StaffSignatureService) SignatureImage(ctx context.Context, tenantID, staffID int64) (data []byte, contentType string, ok bool, err error) {
	member, err := s.repo.FindByID(ctx, tenantID, staffID)
	if err != nil {
		return nil, "", false, err
	}
	if member == nil || member.SignatureStoragePath == nil {
		return nil, "", false, nil
	}
	rc, openErr := s.storage.Open(ctx, *member.SignatureStoragePath)
	if openErr != nil {
		logger.Error("staff signature: gagal membuka objek TTD staff %d (tenant %d): %v", staffID, tenantID, openErr)
		return nil, "", false, nil
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, rc); copyErr != nil {
		logger.Error("staff signature: gagal membaca objek TTD staff %d (tenant %d): %v", staffID, tenantID, copyErr)
		return nil, "", false, nil
	}
	// Tipe di-sniff dari bytes-nya, TIDAK diturunkan dari akhiran kunci objek:
	// compress.Image mempertahankan format masukan, jadi TTD yang diunggah
	// sebagai JPEG tetap JPEG walau kuncinya berakhiran .png. Idiom yang sama
	// dipakai fpdfImageType di sisi PDF.
	return buf.Bytes(), sniffSignatureContentType(buf.Bytes()), true, nil
}

// sniffSignatureContentType menentukan Content-Type dari bytes-nya sendiri,
// dibatasi ke dua format yang memang diizinkan masuk.
func sniffSignatureContentType(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "image/jpeg"
	default:
		return "image/png"
	}
}

// DeleteSignature mengosongkan TTD milik satu pengguna: barisnya dulu, lalu
// objeknya (best-effort). Objek yatim bukan kerusakan; baris yang menunjuk
// berkas hilang adalah kerusakan.
func (s *StaffSignatureService) DeleteSignature(ctx context.Context, tenantID, staffID int64) error {
	member, err := s.repo.FindByID(ctx, tenantID, staffID)
	if err != nil {
		return err
	}
	if member == nil {
		return apperror.NotFound("Pengguna tidak ditemukan")
	}
	if member.SignatureStoragePath == nil {
		return nil
	}
	key := *member.SignatureStoragePath
	if err := s.repo.UpdateSignature(ctx, tenantID, staffID, nil); err != nil {
		return err
	}
	if delErr := s.storage.Delete(ctx, key); delErr != nil {
		logger.Error("staff signature: objek TTD staff %d (tenant %d) yatim setelah hapus baris: %v", staffID, tenantID, delErr)
	}
	return nil
}

// normalizeStaffSignatureImage menolak selain PNG/JPEG dan yang lebih dari
// 2 MB, lalu me-re-encode lewat compress.Image sebelum disimpan — apa pun yang
// menumpang di luar piksel ikut hilang.
//
// Sengaja diduplikasi dari clients/application alih-alih diangkat ke shared/:
// aturan modular monolith melarang lintas-modul mengimpor application, dan
// memindahkannya ke shared/ akan memaksa refactor modul `clients` yang di luar
// lingkup PLAN ini (keputusan A5).
func normalizeStaffSignatureImage(img []byte, mimeType string) (data []byte, contentType string, err error) {
	switch mimeType {
	case "image/png", "image/jpeg", "image/jpg":
	default:
		return nil, "", apperror.Validation("Jenis berkas harus gambar (PNG atau JPEG)", map[string][]string{
			"mimeType": {"Unggah tanda tangan berekstensi PNG atau JPG"},
		})
	}
	if len(img) == 0 {
		return nil, "", apperror.Validation("Gambar tanda tangan kosong", map[string][]string{
			"base64Data": {"Gambar tanda tangan tidak boleh kosong"},
		})
	}
	if len(img) > maxSignatureBytes {
		return nil, "", apperror.Validation("Gambar tanda tangan terlalu besar (maksimal 2 MB)", map[string][]string{
			"base64Data": {"Perkecil gambarnya lalu coba lagi"},
		})
	}
	out, err := compress.Image(img, mimeType)
	if err != nil {
		return nil, "", apperror.Internal("Gagal memproses gambar tanda tangan")
	}
	if mimeType == "image/jpg" {
		mimeType = "image/jpeg"
	}
	return out, mimeType, nil
}
