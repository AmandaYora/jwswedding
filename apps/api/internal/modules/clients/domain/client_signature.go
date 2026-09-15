package domain

import "time"

// ClientSignatureSource adalah asal specimen: digambar klien di halaman
// publik ("draw") atau difoto lalu diunggah pengelola ("upload").
type ClientSignatureSource string

const (
	SignatureSourceDraw   ClientSignatureSource = "draw"
	SignatureSourceUpload ClientSignatureSource = "upload"
)

// ClientSignature adalah SPECIMEN tanda tangan milik satu client (TTD
// Penawaran, D6a/D6b/D12): data master seperti foto profil — selalu SATU
// baris per client (UNIQUE(client_id)), ditimpa saat ada TTD baru, boleh
// dihapus pengelola kapan saja.
//
// Ini BUKAN pembubuhan pada dokumen: dokumen yang sudah diteken memegang
// SALINANNYA SENDIRI di snapshot (D6c), jadi menghapus baris ini tidak
// pernah mengubah dokumen mana pun (D6d).
//
// Role memakai ClientRole yang sudah ada — tidak ada enum peran kedua.
// Ia hanya KETERANGAN pemilik specimen ("milik siapa"), bukan kunci.
type ClientSignature struct {
	ID         int64
	TenantID   int64
	ClientID   int64
	Role       ClientRole
	SignerName string
	StorageKey string
	Source     ClientSignatureSource
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
