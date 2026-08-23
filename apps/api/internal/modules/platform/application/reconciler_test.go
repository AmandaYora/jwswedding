package application

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	billingcontracts "jwswedding/internal/modules/billing/contracts"
	"jwswedding/internal/modules/platform/domain"
	"jwswedding/internal/shared/apperror"
)

func backdate(pending *fakePendingChargeRepo, orderRef string, age time.Duration) {
	pending.charges[orderRef].CreatedAt = time.Now().Add(-age)
}

func TestReconcile_PaidMengaktifkanLangganan(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "P", 100, 1)
	backdate(pending, "REF-1", 10*time.Minute)
	billing := newFakeBilling()
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			return &ChargeResult{OrderRef: orderRef, Status: "paid"}, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)
	rec := NewReconciler(pending, charges, svc, 24*time.Hour)

	_, resolved, err := rec.ReconcilePending(context.Background())
	if err != nil || resolved != 1 {
		t.Fatalf("expected 1 resolved, got resolved=%d err=%v", resolved, err)
	}
	if repo.updateSubscriptionCalls != 1 {
		t.Error("expected subscription to be activated via webhook-equivalent path even though no webhook ever arrived")
	}
}

func TestReconcile_TerminalBelumLunas(t *testing.T) {
	for _, status := range []string{"expired", "failed", "refund"} {
		t.Run(status, func(t *testing.T) {
			tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
			repo := newFakeTenantRepo(tenant)
			pending := newFakePendingChargeRepo()
			_ = pending.Create(context.Background(), "REF-1", 1, 10, "P", 100, 1)
			backdate(pending, "REF-1", 10*time.Minute)
			billing := newFakeBilling()
			charges := &fakeCharges{
				chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
					return &ChargeResult{OrderRef: orderRef, Status: status}, nil
				},
			}
			svc := newTestService(repo, pending, billing, charges)
			rec := NewReconciler(pending, charges, svc, 24*time.Hour)

			_, resolved, err := rec.ReconcilePending(context.Background())
			if err != nil || resolved != 1 {
				t.Fatalf("expected resolved=1, got %d, err=%v", resolved, err)
			}
			if billing.statusUpdates["REF-1"] != billingcontracts.StatusExpired {
				t.Errorf("expected transaction marked expired, got %v", billing.statusUpdates["REF-1"])
			}
			if pending.has("REF-1") {
				t.Error("expected the pending row to be resolved/deleted")
			}
		})
	}
}

func TestReconcile_UnpaidDibiarkan(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "P", 100, 1)
	backdate(pending, "REF-1", 10*time.Minute)
	billing := newFakeBilling()
	var checkCount int32
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			atomic.AddInt32(&checkCount, 1)
			return &ChargeResult{OrderRef: orderRef, Status: "unpaid"}, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)
	rec := NewReconciler(pending, charges, svc, 24*time.Hour)

	for i := 0; i < 3; i++ {
		if _, _, err := rec.ReconcilePending(context.Background()); err != nil {
			t.Fatalf("sweep %d: %v", i, err)
		}
	}

	if !pending.has("REF-1") {
		t.Error("expected a young unpaid charge to remain open across several sweeps")
	}
	if atomic.LoadInt32(&checkCount) != 3 {
		t.Errorf("expected 3 status checks (one per sweep), got %d", checkCount)
	}
}

func TestReconcile_UnpaidLewatMaxAgeDipaksaSelesai(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "P", 100, 1)
	backdate(pending, "REF-1", 25*time.Hour)
	billing := newFakeBilling()
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			return &ChargeResult{OrderRef: orderRef, Status: "unpaid"}, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)
	rec := NewReconciler(pending, charges, svc, 24*time.Hour)

	_, resolved, err := rec.ReconcilePending(context.Background())
	if err != nil || resolved != 1 {
		t.Fatalf("expected the stale unpaid charge to be force-closed, got resolved=%d err=%v", resolved, err)
	}
	if pending.has("REF-1") {
		t.Error("expected the pending row to be resolved so Pay isn't blocked by 409 forever")
	}
}

// TestReconcile_GalatTidakPernahDipaksaSelesai locks D17: a charge whose
// status check always errors must NEVER be force-closed, no matter how
// old — an error means "we don't know", not "unpaid".
func TestReconcile_GalatTidakPernahDipaksaSelesai(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "P", 100, 1)
	backdate(pending, "REF-1", 30*24*time.Hour) // far older than any max-age
	billing := newFakeBilling()
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			return nil, apperror.Internal("ElProof sedang down")
		},
	}
	svc := newTestService(repo, pending, billing, charges)
	rec := NewReconciler(pending, charges, svc, 24*time.Hour)

	for i := 0; i < 5; i++ {
		if _, _, err := rec.ReconcilePending(context.Background()); err != nil {
			t.Fatalf("sweep %d: %v", i, err)
		}
	}

	if !pending.has("REF-1") {
		t.Fatal("a status-check error must never force-close a charge, regardless of age")
	}
	if repo.updateSubscriptionCalls != 0 {
		t.Error("expected no activation from a charge that was never resolved")
	}
}

func TestReconcile_GalatLaluPulihTetapMengaktifkan(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "P", 100, 1)
	backdate(pending, "REF-1", 10*time.Minute)
	billing := newFakeBilling()
	var attempt int32
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			n := atomic.AddInt32(&attempt, 1)
			if n <= 2 {
				return nil, apperror.Internal("ElProof sedang down")
			}
			return &ChargeResult{OrderRef: orderRef, Status: "paid"}, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)
	rec := NewReconciler(pending, charges, svc, 24*time.Hour)

	for i := 0; i < 3; i++ {
		if _, _, err := rec.ReconcilePending(context.Background()); err != nil {
			t.Fatalf("sweep %d: %v", i, err)
		}
	}

	if repo.updateSubscriptionCalls != 1 {
		t.Fatalf("expected the subscription to activate once ElProof recovered, got %d activations", repo.updateSubscriptionCalls)
	}
}

func TestReconcile_MelewatiChargeBaru(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-FRESH", 1, 10, "P", 100, 1)
	// Not backdated — created "just now", inside reconcileMinAge (2 minutes).
	billing := newFakeBilling()
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			t.Fatal("a charge younger than reconcileMinAge must never be checked")
			return nil, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)
	rec := NewReconciler(pending, charges, svc, 24*time.Hour)

	checked, _, err := rec.ReconcilePending(context.Background())
	if err != nil {
		t.Fatalf("ReconcilePending: %v", err)
	}
	if checked != 0 {
		t.Fatalf("expected a fresh charge to be excluded from the sweep, got checked=%d", checked)
	}
}
