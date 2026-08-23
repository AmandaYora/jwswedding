package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"jwswedding/internal/modules/platform/domain"
)

type MySQLPendingChargeRepository struct {
	db *sql.DB
}

func NewMySQLPendingChargeRepository(db *sql.DB) *MySQLPendingChargeRepository {
	return &MySQLPendingChargeRepository{db: db}
}

// Create persists the plan snapshot (name/price/durationMonths) alongside the
// order_ref — D8, so the activation path (webhook or reconciler) never has to
// call ElProof's plan catalog again.
func (r *MySQLPendingChargeRepository) Create(ctx context.Context, orderRef string, tenantID, planID int64, planName string, planPrice int64, planDurationMonths int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pending_subscription_charges (order_ref, tenant_id, plan_id, plan_name, plan_price, plan_duration_months) VALUES (?, ?, ?, ?, ?, ?)`,
		orderRef, tenantID, planID, planName, planPrice, planDurationMonths,
	)
	return err
}

const pendingChargeColumns = `order_ref, tenant_id, plan_id, plan_name, plan_price, plan_duration_months, created_at, resolved_at`

func scanPendingCharge(row interface{ Scan(...any) error }) (*domain.PendingCharge, error) {
	var p domain.PendingCharge
	err := row.Scan(&p.OrderRef, &p.TenantID, &p.PlanID, &p.PlanName, &p.PlanPrice, &p.PlanDurationMonths, &p.CreatedAt, &p.ResolvedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByOrderRef only ever returns an unresolved row — a resolved charge is
// no longer "pending" from any caller's perspective (D11).
func (r *MySQLPendingChargeRepository) FindByOrderRef(ctx context.Context, orderRef string) (*domain.PendingCharge, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+pendingChargeColumns+` FROM pending_subscription_charges WHERE order_ref = ? AND resolved_at IS NULL LIMIT 1`,
		orderRef,
	)
	return scanPendingCharge(row)
}

// FindByTenant backs ActivateSubscription/Pay's guard against a second charge
// while one is already open — Create has no uniqueness guard against a
// tenant having more than one in flight, so this returns every unresolved
// match rather than assuming at most one.
func (r *MySQLPendingChargeRepository) FindByTenant(ctx context.Context, tenantID int64) ([]domain.PendingCharge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+pendingChargeColumns+` FROM pending_subscription_charges WHERE tenant_id = ? AND resolved_at IS NULL`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PendingCharge
	for rows.Next() {
		p, err := scanPendingCharge(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

// FindUnresolved is the reconciler's sweep query (T1): every charge still
// unclaimed, created more than olderThan ago, oldest first, capped at limit.
func (r *MySQLPendingChargeRepository) FindUnresolved(ctx context.Context, olderThan time.Duration, limit int) ([]domain.PendingCharge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+pendingChargeColumns+` FROM pending_subscription_charges
		 WHERE resolved_at IS NULL AND created_at <= ?
		 ORDER BY created_at ASC LIMIT ?`,
		time.Now().Add(-olderThan), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PendingCharge
	for rows.Next() {
		p, err := scanPendingCharge(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

// ClaimUnresolved atomically marks one order_ref resolved — mirrors
// payment's ClaimUnresolved/`payment_charge_dispatch` pattern (D11). Returns
// true only for the caller that actually won the race; a webhook and the
// reconciler landing on the same order_ref at the same time can never both
// activate the subscription.
func (r *MySQLPendingChargeRepository) ClaimUnresolved(ctx context.Context, orderRef string) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE pending_subscription_charges SET resolved_at = NOW() WHERE order_ref = ? AND resolved_at IS NULL`,
		orderRef,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *MySQLPendingChargeRepository) Delete(ctx context.Context, orderRef string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM pending_subscription_charges WHERE order_ref = ?`, orderRef)
	return err
}
