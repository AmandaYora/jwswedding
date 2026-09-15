package infrastructure

import (
	"context"
	"database/sql"

	"jwswedding/internal/modules/clients/domain"
)

// MySQLClientSignatureRepository menyimpan specimen TTD (TTD Penawaran,
// D6b/D12): selalu maksimum SATU baris per client. Tidak ada ListByClient —
// hasilnya selalu nol atau satu baris, jadi FindByClient sudah cukup.
type MySQLClientSignatureRepository struct {
	db *sql.DB
}

func NewMySQLClientSignatureRepository(db *sql.DB) *MySQLClientSignatureRepository {
	return &MySQLClientSignatureRepository{db: db}
}

const clientSignatureColumns = `id, tenant_id, client_id, role, signer_name, storage_key, source, created_at, updated_at`

func scanClientSignature(scan func(dest ...interface{}) error) (*domain.ClientSignature, error) {
	var s domain.ClientSignature
	var role, source string
	err := scan(&s.ID, &s.TenantID, &s.ClientID, &role, &s.SignerName, &s.StorageKey, &source,
		&s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Role = domain.ClientRole(role)
	s.Source = domain.ClientSignatureSource(source)
	return &s, nil
}

// Upsert menimpa specimen milik client — INSERT biasa bila belum ada,
// UPDATE bila sudah (UNIQUE(client_id)). Siapa pun pemilik lamanya, ia
// tergantikan (D12): specimen baru selalu menang.
func (r *MySQLClientSignatureRepository) Upsert(ctx context.Context, s *domain.ClientSignature) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO client_signatures (tenant_id, client_id, role, signer_name, storage_key, source)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id), role = VALUES(role),
		   signer_name = VALUES(signer_name), storage_key = VALUES(storage_key), source = VALUES(source)`,
		s.TenantID, s.ClientID, string(s.Role), s.SignerName, s.StorageKey, string(s.Source))
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil && id != 0 {
		s.ID = id
		return nil
	}
	// Baris lama yang tertimpa: ambil ID-nya supaya pemanggil tetap memegang
	// identitas yang benar.
	existing, err := r.FindByClient(ctx, s.TenantID, s.ClientID)
	if err != nil {
		return err
	}
	if existing != nil {
		s.ID = existing.ID
	}
	return nil
}

func (r *MySQLClientSignatureRepository) FindByClient(ctx context.Context, tenantID, clientID int64) (*domain.ClientSignature, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+clientSignatureColumns+` FROM client_signatures WHERE tenant_id = ? AND client_id = ? LIMIT 1`,
		tenantID, clientID)
	return scanClientSignature(row.Scan)
}

// Delete menghapus baris specimen dan mengembalikan storage_key-nya supaya
// service bisa menghapus objeknya. "" bila tidak ada baris — bukan error.
func (r *MySQLClientSignatureRepository) Delete(ctx context.Context, tenantID, clientID int64) (string, error) {
	existing, err := r.FindByClient(ctx, tenantID, clientID)
	if err != nil {
		return "", err
	}
	if existing == nil {
		return "", nil
	}
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM client_signatures WHERE tenant_id = ? AND client_id = ?`, tenantID, clientID); err != nil {
		return "", err
	}
	return existing.StorageKey, nil
}
