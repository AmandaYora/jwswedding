package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"jwswedding/internal/modules/quotations/domain"
)

// MySQLSignatureLinkRepository menyimpan token magic link (jalur C, D8).
type MySQLSignatureLinkRepository struct {
	db *sql.DB
}

func NewMySQLSignatureLinkRepository(db *sql.DB) *MySQLSignatureLinkRepository {
	return &MySQLSignatureLinkRepository{db: db}
}

const signatureLinkColumns = `id, tenant_id, quotation_id, token_hash, expires_at, used_at, revoked_at, created_by_staff_id, created_at`

func scanSignatureLink(scan func(dest ...interface{}) error) (*domain.SignatureLink, error) {
	var l domain.SignatureLink
	var usedAt, revokedAt sql.NullTime
	err := scan(&l.ID, &l.TenantID, &l.QuotationID, &l.TokenHash, &l.ExpiresAt,
		&usedAt, &revokedAt, &l.CreatedByStaffID, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if usedAt.Valid {
		l.UsedAt = &usedAt.Time
	}
	if revokedAt.Valid {
		l.RevokedAt = &revokedAt.Time
	}
	return &l, nil
}

func (r *MySQLSignatureLinkRepository) Create(ctx context.Context, l *domain.SignatureLink) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO quotation_signature_links
		   (tenant_id, quotation_id, token_hash, expires_at, created_by_staff_id)
		 VALUES (?, ?, ?, ?, ?)`,
		l.TenantID, l.QuotationID, l.TokenHash, l.ExpiresAt.UTC(), l.CreatedByStaffID)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	l.ID = id
	return nil
}

func (r *MySQLSignatureLinkRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*domain.SignatureLink, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+signatureLinkColumns+` FROM quotation_signature_links WHERE token_hash = ? LIMIT 1`, tokenHash)
	return scanSignatureLink(row.Scan)
}

// MarkUsed menandai link terpakai — BERSYARAT: hanya bila used_at masih NULL.
// Inilah satu-satunya cara memenangkan balapan dua klik Terima (§9): nol
// baris terpengaruh = kalah balapan → panggilannya dibatalkan dengan 409.
//
// Mengembalikan stempel waktu yang ditulisnya untuk kompensasi D14
// (ReleaseUsed). Stempel dipotong ke detik (UTC): kolom TIMESTAMP tanpa
// presisi fraksional tidak menyimpan nanos, dan pagar ReleaseUsed
// membandingkan kesetaraan persis.
func (r *MySQLSignatureLinkRepository) MarkUsed(ctx context.Context, id int64) (usedAt time.Time, ok bool, err error) {
	usedAt = time.Now().UTC().Truncate(time.Second)
	res, err := r.db.ExecContext(ctx,
		`UPDATE quotation_signature_links SET used_at = ? WHERE id = ? AND used_at IS NULL`, usedAt, id)
	if err != nil {
		return time.Time{}, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return time.Time{}, false, err
	}
	return usedAt, n == 1, nil
}

// ReleaseUsed mengembalikan used_at ke NULL — kompensasi D14 bila langkah
// sesudah MarkUsed gagal. Dipagari stempel yang ditulis MarkUsed sendiri
// (WHERE id=? AND used_at=?): tanpa itu, kompensasi bisa membatalkan
// pemakaian sah milik permintaan lain yang menang balapan.
func (r *MySQLSignatureLinkRepository) ReleaseUsed(ctx context.Context, id int64, usedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE quotation_signature_links SET used_at = NULL WHERE id = ? AND used_at = ?`, id, usedAt.UTC())
	return err
}

// RevokeAllForQuotation mencabut link-link yang masih bisa dipakai milik
// penawaran ini — link lama mati saat yang baru terbit (D8).
func (r *MySQLSignatureLinkRepository) RevokeAllForQuotation(ctx context.Context, tenantID, quotationID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE quotation_signature_links SET revoked_at = NOW()
		  WHERE tenant_id = ? AND quotation_id = ? AND revoked_at IS NULL AND used_at IS NULL`,
		tenantID, quotationID)
	return err
}
