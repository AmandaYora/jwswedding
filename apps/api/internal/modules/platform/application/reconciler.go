package application

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"jwswedding/internal/modules/platform/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/logger"
)

const (
	// reconcileMinAge keeps a charge out of the sweep until it's had a
	// realistic chance to be paid — no point re-checking something created
	// seconds ago. Mirrors payment's own reconcileMinAge.
	reconcileMinAge = 2 * time.Minute
	// reconcileBatchLimit caps how many charges one sweep tick re-checks
	// against ElProof, so a large backlog can't turn one tick into a
	// long-running burst of outbound requests.
	reconcileBatchLimit = 50
	// reconcileConcurrency bounds how many charges are checked against
	// ElProof at once within a single sweep tick — mirrors payment's own
	// reconcileConcurrency.
	reconcileConcurrency = 10
)

// Reconciler is the safety net for a charge whose webhook was never received
// (T1) — ElProof's webhook relay is fire-and-forget, one attempt, 5s
// timeout, no retry. Without this, one missed webhook means the tenant paid
// but stays locked read-only forever.
type Reconciler struct {
	pendingCharges PendingChargeRepository
	charges        ChargeCreator
	tenants        *TenantService
	maxAge         time.Duration
}

// NewReconciler's maxAge (ELPROOF_CHARGE_MAX_AGE, default 24h) bounds how
// long a charge with a definitive "unpaid" answer from ElProof can stay
// open before being force-closed as failed (D17) — a charge whose status
// check errors is never force-closed regardless of age, since an error
// means "we don't know", not "unpaid".
func NewReconciler(pendingCharges PendingChargeRepository, charges ChargeCreator, tenants *TenantService, maxAge time.Duration) *Reconciler {
	return &Reconciler{pendingCharges: pendingCharges, charges: charges, tenants: tenants, maxAge: maxAge}
}

// StartReconciler runs ReconcilePending on a fixed interval until ctx is
// cancelled. Errors are logged, never fatal — a bad tick just retries next
// interval.
func (r *Reconciler) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checked, resolved, err := r.ReconcilePending(ctx)
				if err != nil {
					logger.Error("platform: reconciliation sweep gagal: %v", err)
					continue
				}
				if checked > 0 {
					logger.Info("platform: reconciliation sweep — %d diperiksa, %d diselesaikan", checked, resolved)
				}
			}
		}
	}()
}

// ReconcilePending re-checks every still-open charge older than
// reconcileMinAge directly against ElProof and applies a terminal outcome
// exactly the way the webhook handler would (same ApplyWebhookEvent call),
// so downstream behavior is identical regardless of how the outcome was
// discovered.
func (r *Reconciler) ReconcilePending(ctx context.Context) (checked int, resolved int, err error) {
	pending, err := r.pendingCharges.FindUnresolved(ctx, reconcileMinAge, reconcileBatchLimit)
	if err != nil {
		return 0, 0, err
	}

	sem := make(chan struct{}, reconcileConcurrency)
	var wg sync.WaitGroup
	var resolvedCount int64
	for _, charge := range pending {
		wg.Add(1)
		sem <- struct{}{}
		go func(c domain.PendingCharge) {
			defer wg.Done()
			defer func() { <-sem }()
			if r.reconcileOne(ctx, c) {
				atomic.AddInt64(&resolvedCount, 1)
			}
		}(charge)
	}
	wg.Wait()

	return len(pending), int(resolvedCount), nil
}

// reconcileOne handles a single charge, returning whether it ended up
// resolved (a terminal event was applied) this round. See D17: only a
// definitive answer from ElProof (paid/expired/failed/refund/not_found, or
// unpaid past maxAge) ever resolves a charge — a status-check error always
// leaves it open, no matter how old, since "we don't know" is not "unpaid".
func (r *Reconciler) reconcileOne(ctx context.Context, charge domain.PendingCharge) bool {
	result, err := r.charges.ChargeStatus(ctx, charge.OrderRef)
	if err != nil {
		if appErr, ok := apperror.As(err); ok && appErr.Kind == apperror.KindNotFound {
			// Ghost row from D16/T7: Pay wrote this row locally but died (or
			// got a duplicate-orderRef conflict) before/at CreateCharge, so
			// ElProof has no record of it at all. Resolve immediately —
			// there's nothing to wait for, and leaving it open would block
			// Pay with a 409 forever.
			return r.apply(ctx, charge.OrderRef, false)
		}
		logger.Error("platform: reconciliation — order_ref %q gagal dicek (%v), dibiarkan menggantung", charge.OrderRef, err)
		return false
	}

	switch result.Status {
	case "paid":
		return r.apply(ctx, charge.OrderRef, true)
	case "expired", "failed", "refund":
		return r.apply(ctx, charge.OrderRef, false)
	case "unpaid":
		if time.Since(charge.CreatedAt) > r.maxAge {
			return r.apply(ctx, charge.OrderRef, false)
		}
		return false // still legitimately open — check again next sweep
	default:
		logger.Error("platform: reconciliation — order_ref %q punya status tak dikenal %q, dibiarkan menggantung", charge.OrderRef, result.Status)
		return false
	}
}

func (r *Reconciler) apply(ctx context.Context, orderRef string, paid bool) bool {
	if err := r.tenants.ApplyWebhookEvent(ctx, orderRef, WebhookEvent{Paid: paid}); err != nil {
		logger.Error("platform: reconciliation — gagal menerapkan hasil order_ref %q: %v", orderRef, err)
		return false
	}
	return true
}
