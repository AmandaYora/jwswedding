package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/logger"
)

// SignatureLinkRepository adalah port penyimpanan token magic link.
type SignatureLinkRepository interface {
	Create(ctx context.Context, l *domain.SignatureLink) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*domain.SignatureLink, error)
	MarkUsed(ctx context.Context, id int64) (usedAt time.Time, ok bool, err error)
	ReleaseUsed(ctx context.Context, id int64, usedAt time.Time) error
	RevokeAllForQuotation(ctx context.Context, tenantID, quotationID int64) error
}

// SignatureLinkService menerbitkan dan menebus magic link tanda tangan
// (jalur C). Token mentah 32 byte acak (base64url) dikembalikan ke pemanggil
// SEKALI — yang disimpan hanya SHA-256-nya.
type SignatureLinkService struct {
	links      SignatureLinkRepository
	quotations *QuotationService
}

func NewSignatureLinkService(links SignatureLinkRepository, quotations *QuotationService) *SignatureLinkService {
	return &SignatureLinkService{links: links, quotations: quotations}
}

func hashSignatureToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Issue menerbitkan link baru untuk penawaran: mencabut link lama yang masih
// hidup lebih dulu (D8). Gerbang statusnya SAMA PERSIS dengan SignQuotation
// (D13) — Ditawarkan, atau Diterima-berproject yang revisinya belum berTTD;
// tolak bila revisi yang berlaku sudah bertanda tangan.
func (s *SignatureLinkService) Issue(ctx context.Context, tenantID, quotationID, staffID int64) (token string, expiresAt time.Time, err error) {
	view, err := s.quotations.Get(ctx, tenantID, quotationID)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := checkSignable(view); err != nil {
		return "", time.Time{}, err
	}
	if err := s.links.RevokeAllForQuotation(ctx, tenantID, quotationID); err != nil {
		return "", time.Time{}, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	expiresAt = time.Now().Add(domain.SignatureLinkTTL)
	link := &domain.SignatureLink{
		TenantID: tenantID, QuotationID: quotationID,
		TokenHash: hashSignatureToken(token), ExpiresAt: expiresAt, CreatedByStaffID: staffID,
	}
	if err := s.links.Create(ctx, link); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// checkSignable adalah gerbang D13 dalam bentuk murni (tanpa DB): boleh
// diteken bila revisi yang berlaku belum berTTD DAN (Ditawarkan ATAU
// Diterima-berproject). Dipakai Issue dan sebagai cermin SignQuotation.
func checkSignable(view *QuotationView) error {
	if view == nil || view.Quotation == nil {
		return apperror.NotFound("Penawaran tidak ditemukan")
	}
	o := view.Quotation
	if o.Snapshot != nil && o.Snapshot.Current.Signature != nil {
		return apperror.Validation("Revisi penawaran ini sudah ditandatangani", map[string][]string{
			"signature": {"Revisi ini sudah memiliki tanda tangan"},
		})
	}
	switch {
	case o.Status == domain.QuotationOffered:
		return nil
	case o.Status == domain.QuotationAccepted && view.ProjectID != 0:
		return nil
	default:
		return apperror.Validation("Penawaran ini sudah tidak bisa ditandatangani", nil)
	}
}

// ResolvedLink adalah hasil penebusan token untuk halaman publik: dokumen +
// pilihan Atas Nama. Tenant selalu dari baris token (§9).
type ResolvedLink struct {
	Link    *domain.SignatureLink
	View    *QuotationView
	Options *QuotationSignatureOptions
}

func (s *SignatureLinkService) Resolve(ctx context.Context, token string) (*ResolvedLink, error) {
	link, err := s.usableLink(ctx, token)
	if err != nil {
		return nil, err
	}
	view, err := s.quotations.Get(ctx, link.TenantID, link.QuotationID)
	if err != nil {
		return nil, err
	}
	// Penawaran bisa berubah setelah link terbit (ditutup pengelola):
	// halaman publik menolak dengan pesan yang sama — tidak membocorkan
	// apakah penawarannya ada.
	if err := checkSignable(view); err != nil {
		return nil, apperror.NotFound("Link tidak berlaku")
	}
	options, err := s.quotations.SignatureOptions(ctx, link.TenantID, link.QuotationID)
	if err != nil {
		return nil, err
	}
	return &ResolvedLink{Link: link, View: view, Options: options}, nil
}

func (s *SignatureLinkService) usableLink(ctx context.Context, token string) (*domain.SignatureLink, error) {
	if token == "" {
		return nil, apperror.NotFound("Link tidak berlaku")
	}
	link, err := s.links.FindByTokenHash(ctx, hashSignatureToken(token))
	if err != nil {
		return nil, err
	}
	if !link.IsUsable(time.Now()) {
		// Pesan sama untuk token salah, kedaluwarsa, sudah dipakai, maupun
		// dicabut — agar tidak membocorkan apakah penawaran itu ada.
		return nil, apperror.NotFound("Link tidak berlaku")
	}
	return link, nil
}

// signerNameForLink menyelesaikan nama penanda tangan dari opsi Atas Nama —
// halaman publik mengirim role, backend yang tahu namanya.
func (s *SignatureLinkService) signerNameForLink(ctx context.Context, link *domain.SignatureLink, role string) (string, error) {
	options, err := s.quotations.SignatureOptions(ctx, link.TenantID, link.QuotationID)
	if err != nil {
		return "", err
	}
	for _, o := range options.Options {
		if o[0] == role {
			return o[1], nil
		}
	}
	return "", apperror.Validation("Peran penanda tangan tidak dikenal", map[string][]string{
		"role": {"Pilih Atas Nama yang tersedia"},
	})
}

// AcceptViaLink menebus token menjadi tanda tangan (jalur C — Terima).
// Urutan: MarkUsed bersyarat DULU (menangkan balapan dua klik), baru
// SignQuotation. Project TIDAK dibuat di sini (D1).
func (s *SignatureLinkService) AcceptViaLink(ctx context.Context, token, role string, img []byte, mimeType string) error {
	link, err := s.usableLink(ctx, token)
	if err != nil {
		return err
	}
	usedAt, ok, err := s.links.MarkUsed(ctx, link.ID)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Conflict("Link sudah dipakai")
	}
	signerName, err := s.signerNameForLink(ctx, link, role)
	if err != nil {
		s.releaseQuietly(ctx, link.ID, usedAt)
		return err
	}
	if _, err := s.quotations.SignQuotation(ctx, link.TenantID, link.QuotationID,
		role, signerName, img, mimeType, SignatureChannelMagicLink); err != nil {
		return s.compensate(ctx, link.ID, usedAt, err)
	}
	return nil
}

// RejectViaLink menebus token menjadi penolakan — sama persis dengan "Tutup
// Penawaran" pilihan Ditolak (T3): status Ditolak, tanpa menyimpan alasan.
func (s *SignatureLinkService) RejectViaLink(ctx context.Context, token string) error {
	link, err := s.usableLink(ctx, token)
	if err != nil {
		return err
	}
	usedAt, ok, err := s.links.MarkUsed(ctx, link.ID)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Conflict("Link sudah dipakai")
	}
	if _, err := s.quotations.Reject(ctx, link.TenantID, link.QuotationID); err != nil {
		return s.compensate(ctx, link.ID, usedAt, err)
	}
	return nil
}

// compensate adalah kompensasi D14: pulihkan link bila langkah sesudah
// MarkUsed gagal, supaya satu gangguan sesaat tidak membakar link klien
// secara permanen. Berlaku untuk SEMUA galat — bila penawarannya memang
// sudah tidak layak diteken, percobaan berikutnya gagal lagi di gerbang yang
// sama, jadi memulihkan link tidak melonggarkan apa pun.
//
// Galat validasi diteruskan apa adanya (pesannya akurat); kegagalan
// penyimpanan dibungkus "Gagal menyimpan, silakan coba lagi" dengan halaman
// tetap terbuka dan tombol Terima aktif kembali.
func (s *SignatureLinkService) compensate(ctx context.Context, linkID int64, usedAt time.Time, cause error) error {
	if err := s.links.ReleaseUsed(ctx, linkID, usedAt); err != nil {
		// Batas akhirnya: link hangus. Bukan transaksi dua fase — jangan
		// dicoba ulang, ubah pesannya (D14).
		logger.Error("kompensasi link tanda tangan %d gagal: %v (penyebab awal: %v)", linkID, err, cause)
		return apperror.Internal("Gagal menyimpan — hubungi pengelola untuk link baru")
	}
	if appErr, ok := apperror.As(cause); ok && appErr.Kind == apperror.KindValidation {
		return cause
	}
	if appErr, ok := apperror.As(cause); ok && (appErr.Kind == apperror.KindNotFound || appErr.Kind == apperror.KindConflict) {
		return cause
	}
	return apperror.Internal("Gagal menyimpan, silakan coba lagi")
}

// releaseQuietly memulihkan link setelah galat pra-penyimpanan (nama peran
// tak dikenal) — link hidup lagi; galat aslinya yang diteruskan.
func (s *SignatureLinkService) releaseQuietly(ctx context.Context, linkID int64, usedAt time.Time) {
	if err := s.links.ReleaseUsed(ctx, linkID, usedAt); err != nil {
		logger.Error("kompensasi link tanda tangan %d gagal: %v", linkID, err)
	}
}
