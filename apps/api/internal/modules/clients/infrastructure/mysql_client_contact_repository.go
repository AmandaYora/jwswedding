package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"jwswedding/internal/modules/clients/domain"
)

type MySQLClientContactRepository struct {
	db *sql.DB
}

func NewMySQLClientContactRepository(db *sql.DB) *MySQLClientContactRepository {
	return &MySQLClientContactRepository{db: db}
}

const clientContactColumns = `id, tenant_id, client_id, role, username, relation_note, name, phone, email, is_active,
	last_credential_reset_at, created_at, updated_at`

func scanClientContact(scan func(dest ...interface{}) error) (*domain.ClientContact, error) {
	var c domain.ClientContact
	var role string
	var clientID sql.NullInt64
	var lastReset sql.NullTime
	err := scan(&c.ID, &c.TenantID, &clientID, &role, &c.Username, &c.RelationNote, &c.Name, &c.Phone, &c.Email, &c.IsActive,
		&lastReset, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if clientID.Valid {
		c.ClientID = clientID.Int64
	}
	c.Role = domain.ClientRole(role)
	if lastReset.Valid {
		c.LastCredentialResetAt = &lastReset.Time
	}
	return &c, nil
}

func (r *MySQLClientContactRepository) ListByClient(ctx context.Context, tenantID, clientID int64) ([]domain.ClientContact, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+clientContactColumns+` FROM client_contacts WHERE tenant_id = ? AND client_id = ? ORDER BY id`, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ClientContact
	for rows.Next() {
		c, err := scanClientContact(rows.Scan)
		if err != nil {
			return nil, err
		}
		list = append(list, *c)
	}
	return list, rows.Err()
}

// CountByClients mengembalikan jumlah kontak per client untuk SATU halaman
// daftar — satu query agregat, bukan N+1 (§11).
func (r *MySQLClientContactRepository) CountByClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]int, error) {
	out := make(map[int64]int, len(clientIDs))
	if len(clientIDs) == 0 {
		return out, nil
	}
	query := `SELECT client_id, COUNT(*) FROM client_contacts WHERE tenant_id = ? AND client_id IN (`
	args := []interface{}{tenantID}
	for i, id := range clientIDs {
		if i > 0 {
			query += `,`
		}
		query += `?`
		args = append(args, id)
	}
	query += `) GROUP BY client_id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func (r *MySQLClientContactRepository) FindByID(ctx context.Context, tenantID, id int64) (*domain.ClientContact, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+clientContactColumns+` FROM client_contacts WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	return scanClientContact(row.Scan)
}

func (r *MySQLClientContactRepository) Create(ctx context.Context, c *domain.ClientContact) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO client_contacts (tenant_id, client_id, role, username, relation_note, name, phone, email, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.TenantID, c.ClientID, string(c.Role), c.Username, c.RelationNote, c.Name, c.Phone, c.Email, c.IsActive,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = id
	return nil
}

func (r *MySQLClientContactRepository) Update(ctx context.Context, c *domain.ClientContact) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE client_contacts SET name = ?, phone = ?, email = ?, relation_note = ? WHERE tenant_id = ? AND id = ?`,
		c.Name, c.Phone, c.Email, c.RelationNote, c.TenantID, c.ID,
	)
	return err
}

func (r *MySQLClientContactRepository) SetActive(ctx context.Context, tenantID, id int64, isActive bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE client_contacts SET is_active = ? WHERE tenant_id = ? AND id = ?`, isActive, tenantID, id)
	return err
}

func (r *MySQLClientContactRepository) SetCredentialResetAt(ctx context.Context, tenantID, id int64, when time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE client_contacts SET last_credential_reset_at = ? WHERE tenant_id = ? AND id = ?`, when, tenantID, id)
	return err
}

func (r *MySQLClientContactRepository) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM client_contacts WHERE tenant_id = ? AND id = ?`, tenantID, id)
	return err
}

// DeleteForClient menyapu seluruh kontak milik satu client (penghapusan
// Client berjenjang, T3.5) — tanpa kredensial per baris di sini; pemanggil
// (service) menonaktifkan kredensialnya lebih dulu.
func (r *MySQLClientContactRepository) DeleteForClient(ctx context.Context, tenantID, clientID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM client_contacts WHERE tenant_id = ? AND client_id = ?`, tenantID, clientID)
	return err
}
