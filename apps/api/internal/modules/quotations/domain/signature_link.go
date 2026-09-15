package domain

import "time"

// SignatureLinkTTL adalah umur magic link tanda tangan: 24 jam (D8).
const SignatureLinkTTL = 24 * time.Hour

// SignatureLink adalah token sekali pakai untuk menandatangani penawaran
// tanpa login (jalur C). Yang disimpan hanya SHA-256 token (token_hash,
// UNIQUE) — bocornya isi DB tidak memberikan token yang bisa dipakai (§9).
type SignatureLink struct {
	ID               int64
	TenantID         int64
	QuotationID      int64
	TokenHash        string
	ExpiresAt        time.Time
	UsedAt           *time.Time
	RevokedAt        *time.Time
	CreatedByStaffID int64
	CreatedAt        time.Time
}

// IsUsable melaporkan apakah link masih bisa dipakai: belum dipakai, belum
// dicabut, belum kedaluwarsa.
func (l *SignatureLink) IsUsable(now time.Time) bool {
	if l == nil {
		return false
	}
	if l.UsedAt != nil || l.RevokedAt != nil {
		return false
	}
	return now.Before(l.ExpiresAt)
}
