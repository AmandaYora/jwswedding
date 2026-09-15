package infrastructure

import (
	"context"
	"database/sql"

	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/utils"
)

type MySQLClientRepository struct {
	db *sql.DB
}

func NewMySQLClientRepository(db *sql.DB) *MySQLClientRepository {
	return &MySQLClientRepository{db: db}
}

const clientColumns = `id, tenant_id, bride_name, groom_name, phone, email, notes, created_at, updated_at`

func scanClient(scan func(dest ...interface{}) error) (*domain.Client, error) {
	var c domain.Client
	err := scan(&c.ID, &c.TenantID, &c.BrideName, &c.GroomName, &c.Phone, &c.Email, &c.Notes,
		&c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *MySQLClientRepository) FindByID(ctx context.Context, tenantID, id int64) (*domain.Client, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+clientColumns+` FROM clients WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	return scanClient(row.Scan)
}

// FindByIDs memuat satu halaman client sekaligus untuk CoupleNamesBatch —
// SATU query, bukan satu per id. Daftar Penawaran memanggilnya sekali per
// halaman, dan bentuk inilah yang PLAN §11 minta untuk mencegah N+1.
func (r *MySQLClientRepository) FindByIDs(ctx context.Context, tenantID int64, ids []int64) ([]domain.Client, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := append([]interface{}{tenantID}, utils.Int64Args(ids)...)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+clientColumns+` FROM clients WHERE tenant_id = ? AND id IN (`+utils.Placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Client
	for rows.Next() {
		c, err := scanClient(rows.Scan)
		if err != nil {
			return nil, err
		}
		list = append(list, *c)
	}
	return list, rows.Err()
}

// ListPaginated menopang halaman Client (daftar pasangan). Pencarian mencakup
// kedua nama + telepon — cara WO menemukan pasangan dari nomor yang menelepon.
func (r *MySQLClientRepository) ListPaginated(ctx context.Context, tenantID int64, params pagination.Params, search string) ([]domain.Client, int64, error) {
	countQuery := `SELECT COUNT(*) FROM clients WHERE tenant_id = ?`
	listQuery := `SELECT ` + clientColumns + ` FROM clients WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	if search != "" {
		countQuery += ` AND (bride_name LIKE ? OR groom_name LIKE ? OR phone LIKE ?)`
		listQuery += ` AND (bride_name LIKE ? OR groom_name LIKE ? OR phone LIKE ?)`
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, listQuery, append(args, params.Limit, params.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.Client
	for rows.Next() {
		c, err := scanClient(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *c)
	}
	return list, total, rows.Err()
}

func (r *MySQLClientRepository) Create(ctx context.Context, c *domain.Client) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO clients (tenant_id, bride_name, groom_name, phone, email, notes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.TenantID, c.BrideName, c.GroomName, c.Phone, c.Email, c.Notes,
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

func (r *MySQLClientRepository) Update(ctx context.Context, c *domain.Client) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE clients SET bride_name = ?, groom_name = ?, phone = ?, email = ?, notes = ? WHERE tenant_id = ? AND id = ?`,
		c.BrideName, c.GroomName, c.Phone, c.Email, c.Notes, c.TenantID, c.ID,
	)
	return err
}

func (r *MySQLClientRepository) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM clients WHERE tenant_id = ? AND id = ?`, tenantID, id)
	return err
}
