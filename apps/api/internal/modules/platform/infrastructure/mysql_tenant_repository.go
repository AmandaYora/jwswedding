package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"jwswedding/internal/modules/platform/domain"
	"jwswedding/internal/shared/pagination"
)

type MySQLTenantRepository struct {
	db *sql.DB
}

func NewMySQLTenantRepository(db *sql.DB) *MySQLTenantRepository {
	return &MySQLTenantRepository{db: db}
}

const tenantColumns = `id, business_name, owner_name, username, email, phone, city, joined_at, plan_id,
	subscription_status, subscription_expires_at, is_suspended, last_credential_reset_at,
	brand_color_preset, logo_storage_path, signature_storage_path, custom_domain, address, bank_name,
	bank_account_number, bank_account_holder_name, created_at, updated_at`

func scanTenant(scan func(dest ...interface{}) error) (*domain.Tenant, error) {
	var t domain.Tenant
	var planID sql.NullInt64
	var status string
	var expiresAt, lastReset sql.NullTime
	var logoStoragePath, signatureStoragePath, customDomain, address, bankName, bankAccountNumber, bankAccountHolderName sql.NullString

	err := scan(
		&t.ID, &t.BusinessName, &t.OwnerName, &t.Username, &t.Email, &t.Phone, &t.City, &t.JoinedAt, &planID,
		&status, &expiresAt, &t.IsSuspended, &lastReset, &t.BrandColorPreset, &logoStoragePath, &signatureStoragePath,
		&customDomain, &address, &bankName, &bankAccountNumber, &bankAccountHolderName,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	t.SubscriptionStatus = domain.SubscriptionStatus(status)
	if planID.Valid {
		t.PlanID = &planID.Int64
	}
	if expiresAt.Valid {
		t.SubscriptionExpiresAt = &expiresAt.Time
	}
	if lastReset.Valid {
		t.LastCredentialResetAt = &lastReset.Time
	}
	if logoStoragePath.Valid {
		t.LogoStoragePath = &logoStoragePath.String
	}
	if signatureStoragePath.Valid {
		t.SignatureStoragePath = &signatureStoragePath.String
	}
	if customDomain.Valid {
		t.CustomDomain = &customDomain.String
	}
	t.Address = address.String
	t.BankName = bankName.String
	t.BankAccountNumber = bankAccountNumber.String
	t.BankAccountHolderName = bankAccountHolderName.String
	return &t, nil
}

func (r *MySQLTenantRepository) List(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+tenantColumns+` FROM tenants ORDER BY joined_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		t, err := scanTenant(rows.Scan)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, *t)
	}
	return tenants, rows.Err()
}

// ListPaginated backs the real `GET /tenants` list page — List above stays
// as-is for any full-roster consumer.
func (r *MySQLTenantRepository) ListPaginated(ctx context.Context, params pagination.Params, search, status string) ([]domain.Tenant, int64, error) {
	countQuery := `SELECT COUNT(*) FROM tenants`
	listQuery := `SELECT ` + tenantColumns + ` FROM tenants`
	var args []interface{}
	var conditions []string
	if search != "" {
		conditions = append(conditions, `(business_name LIKE ? OR owner_name LIKE ? OR email LIKE ?)`)
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	if status != "" {
		conditions = append(conditions, `subscription_status = ?`)
		args = append(args, status)
	}
	if len(conditions) > 0 {
		where := ` WHERE ` + strings.Join(conditions, " AND ")
		countQuery += where
		listQuery += where
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += ` ORDER BY joined_at DESC, id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, listQuery, append(args, params.Limit, params.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		t, err := scanTenant(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		tenants = append(tenants, *t)
	}
	return tenants, total, rows.Err()
}

func (r *MySQLTenantRepository) FindByID(ctx context.Context, id int64) (*domain.Tenant, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE id = ? LIMIT 1`, id)
	return scanTenant(row.Scan)
}

// FindByDomain resolves a tenant from an incoming request's Host header — the
// pre-auth lookup ADR-0015's public branding/logo endpoints use, distinct
// from every other lookup in this repo (all JWT/ID-scoped).
func (r *MySQLTenantRepository) FindByDomain(ctx context.Context, host string) (*domain.Tenant, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE custom_domain = ? LIMIT 1`, host)
	return scanTenant(row.Scan)
}

func (r *MySQLTenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	// brand_color_preset is included here (PLAN.md redesain-pdf-invoice-kwitansi
	// §D13) -- it used to be silently omitted, so Tenant.BrandColorPreset set
	// by a caller (e.g. adminseed.Run) was a dead assignment and every new
	// tenant fell back to migration 000017's `DEFAULT 'navy'` regardless of
	// what the caller intended.
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO tenants (business_name, owner_name, username, email, phone, city, joined_at, plan_id, subscription_status, brand_color_preset)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tenant.BusinessName, tenant.OwnerName, tenant.Username, tenant.Email, tenant.Phone, tenant.City,
		tenant.JoinedAt, tenant.PlanID, string(tenant.SubscriptionStatus), tenant.BrandColorPreset,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	tenant.ID = id
	return nil
}

func (r *MySQLTenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tenants SET business_name = ?, owner_name = ?, email = ?, phone = ?, city = ?, brand_color_preset = ?,
		 custom_domain = ?, address = ?, bank_name = ?, bank_account_number = ?, bank_account_holder_name = ? WHERE id = ?`,
		tenant.BusinessName, tenant.OwnerName, tenant.Email, tenant.Phone, tenant.City, tenant.BrandColorPreset,
		tenant.CustomDomain, tenant.Address, tenant.BankName, tenant.BankAccountNumber, tenant.BankAccountHolderName,
		tenant.ID,
	)
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return domain.ErrDuplicateCustomDomain
	}
	return err
}

func (r *MySQLTenantRepository) UpdateLogo(ctx context.Context, id int64, logoStoragePath *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tenants SET logo_storage_path = ? WHERE id = ?`, logoStoragePath, id)
	return err
}

func (r *MySQLTenantRepository) UpdateSignature(ctx context.Context, id int64, signatureStoragePath *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tenants SET signature_storage_path = ? WHERE id = ?`, signatureStoragePath, id)
	return err
}

func (r *MySQLTenantRepository) SetSuspended(ctx context.Context, id int64, suspended bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tenants SET is_suspended = ? WHERE id = ?`, suspended, id)
	return err
}

func (r *MySQLTenantRepository) SetCredentialResetAt(ctx context.Context, id int64, when time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tenants SET last_credential_reset_at = ? WHERE id = ?`, when, id)
	return err
}

func (r *MySQLTenantRepository) UpdateSubscription(ctx context.Context, id int64, planID int64, status domain.SubscriptionStatus, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tenants SET plan_id = ?, subscription_status = ?, subscription_expires_at = ? WHERE id = ?`,
		planID, string(status), expiresAt, id,
	)
	return err
}
