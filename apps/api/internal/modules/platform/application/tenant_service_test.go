package application

import (
	"context"
	"sync"
	"testing"
	"time"

	billingcontracts "jwswedding/internal/modules/billing/contracts"
	"jwswedding/internal/modules/platform/domain"
	"jwswedding/internal/shared/apperror"
)

// --- fakes ---

type fakeTenantRepo struct {
	mu      sync.Mutex
	tenants map[int64]*domain.Tenant
	// updateSubscriptionCalls counts real invocations of UpdateSubscription —
	// used to assert ApplyWebhookEvent activates exactly once under a race
	// (T2/D11), not twice.
	updateSubscriptionCalls int
}

func newFakeTenantRepo(tenants ...*domain.Tenant) *fakeTenantRepo {
	m := make(map[int64]*domain.Tenant)
	for _, t := range tenants {
		m[t.ID] = t
	}
	return &fakeTenantRepo{tenants: m}
}

func (f *fakeTenantRepo) FindByID(ctx context.Context, id int64) (*domain.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[id]
	if !ok {
		return nil, nil
	}
	copy := *t
	return &copy, nil
}

func (f *fakeTenantRepo) FindByDomain(ctx context.Context, host string) (*domain.Tenant, error) {
	return nil, nil
}

func (f *fakeTenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tenants[tenant.ID]; !ok {
		return apperror.NotFound("tenant tidak ditemukan")
	}
	copy := *tenant
	f.tenants[tenant.ID] = &copy
	return nil
}

func (f *fakeTenantRepo) UpdateLogo(ctx context.Context, id int64, logoStoragePath *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[id]
	if !ok {
		return apperror.NotFound("tenant tidak ditemukan")
	}
	t.LogoStoragePath = logoStoragePath
	return nil
}

func (f *fakeTenantRepo) UpdateSubscription(ctx context.Context, id int64, planID int64, status domain.SubscriptionStatus, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[id]
	if !ok {
		return apperror.NotFound("tenant tidak ditemukan")
	}
	f.updateSubscriptionCalls++
	t.PlanID = &planID
	t.SubscriptionStatus = status
	t.SubscriptionExpiresAt = &expiresAt
	return nil
}

// fakePendingChargeRepo mirrors the atomic ClaimUnresolved contract
// (D11) with an in-memory mutex — good enough to test every consumer of
// that contract (TenantService, Reconciler) without a real MySQL instance.
type fakePendingChargeRepo struct {
	mu      sync.Mutex
	charges map[string]*domain.PendingCharge
	// createCalledWithChargeAt records, per orderRef, whether the row
	// existed in `charges` at the moment CreateCharge below was about to be
	// invoked — set from a test's ChargeCreator to verify D16/T7 ordering.
}

func newFakePendingChargeRepo() *fakePendingChargeRepo {
	return &fakePendingChargeRepo{charges: make(map[string]*domain.PendingCharge)}
}

func (f *fakePendingChargeRepo) Create(ctx context.Context, orderRef string, tenantID, planID int64, planName string, planPrice int64, planDurationMonths int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charges[orderRef] = &domain.PendingCharge{
		OrderRef: orderRef, TenantID: tenantID, PlanID: planID,
		PlanName: planName, PlanPrice: planPrice, PlanDurationMonths: planDurationMonths,
		CreatedAt: time.Now(),
	}
	return nil
}

func (f *fakePendingChargeRepo) FindByOrderRef(ctx context.Context, orderRef string) (*domain.PendingCharge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.charges[orderRef]
	if !ok || c.ResolvedAt != nil {
		return nil, nil
	}
	copy := *c
	return &copy, nil
}

func (f *fakePendingChargeRepo) FindByTenant(ctx context.Context, tenantID int64) ([]domain.PendingCharge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.PendingCharge
	for _, c := range f.charges {
		if c.TenantID == tenantID && c.ResolvedAt == nil {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (f *fakePendingChargeRepo) FindUnresolved(ctx context.Context, olderThan time.Duration, limit int) ([]domain.PendingCharge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.PendingCharge
	cutoff := time.Now().Add(-olderThan)
	for _, c := range f.charges {
		if c.ResolvedAt == nil && !c.CreatedAt.After(cutoff) {
			out = append(out, *c)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakePendingChargeRepo) ClaimUnresolved(ctx context.Context, orderRef string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.charges[orderRef]
	if !ok || c.ResolvedAt != nil {
		return false, nil
	}
	now := time.Now()
	c.ResolvedAt = &now
	return true, nil
}

func (f *fakePendingChargeRepo) Delete(ctx context.Context, orderRef string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.charges, orderRef)
	return nil
}

func (f *fakePendingChargeRepo) has(orderRef string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.charges[orderRef]
	return ok
}

// fakeBilling implements billingcontracts.Contracts.
type fakeBilling struct {
	mu               sync.Mutex
	plans            map[int64]billingcontracts.Plan
	transactions     []billingcontracts.RecordTransactionInput
	statusUpdates    map[string]billingcontracts.TransactionStatus
	getPlanCallCount int
}

func newFakeBilling(plans ...billingcontracts.Plan) *fakeBilling {
	m := make(map[int64]billingcontracts.Plan)
	for _, p := range plans {
		m[p.ID] = p
	}
	return &fakeBilling{plans: m, statusUpdates: make(map[string]billingcontracts.TransactionStatus)}
}

func (b *fakeBilling) GetPlan(ctx context.Context, id int64) (*billingcontracts.Plan, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.getPlanCallCount++
	p, ok := b.plans[id]
	if !ok {
		return nil, apperror.NotFound("paket tidak ditemukan")
	}
	return &p, nil
}

func (b *fakeBilling) RecordTransaction(ctx context.Context, input billingcontracts.RecordTransactionInput) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transactions = append(b.transactions, input)
	return nil
}

func (b *fakeBilling) UpdateTransactionStatus(ctx context.Context, paymentReference string, status billingcontracts.TransactionStatus) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.statusUpdates[paymentReference] = status
	return nil
}

// fakeCharges implements ChargeCreator with fully overridable behavior per
// test case.
type fakeCharges struct {
	createSubscriptionCharge func(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error)
	chargeStatus             func(ctx context.Context, orderRef string) (*ChargeResult, error)
}

func (f *fakeCharges) CreateSubscriptionCharge(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
	return f.createSubscriptionCharge(ctx, orderRef, planID, customerName, customerEmail, customerPhone)
}

func (f *fakeCharges) ChargeStatus(ctx context.Context, orderRef string) (*ChargeResult, error) {
	return f.chargeStatus(ctx, orderRef)
}

func newTestService(repo *fakeTenantRepo, pending *fakePendingChargeRepo, billing *fakeBilling, charges *fakeCharges) *TenantService {
	return NewTenantService(repo, pending, billing, charges, nil, func(a, b, c, d string) string { return a + b + c + d })
}

// --- T2/D11: atomic claim ---

func TestApplyWebhookEvent_WebhookLaluReconciler(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "Paket Tahunan", 2_400_000, 12)
	billing := newFakeBilling(billingcontracts.Plan{ID: 10, Name: "Paket Tahunan", DurationMonths: 12, Price: 2_400_000})
	svc := newTestService(repo, pending, billing, &fakeCharges{})

	if err := svc.ApplyWebhookEvent(context.Background(), "REF-1", WebhookEvent{Paid: true}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	// Second call simulates the reconciler discovering the same order_ref
	// independently after the webhook already won the claim.
	if err := svc.ApplyWebhookEvent(context.Background(), "REF-1", WebhookEvent{Paid: true}); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if repo.updateSubscriptionCalls != 1 {
		t.Fatalf("expected UpdateSubscription exactly once, got %d", repo.updateSubscriptionCalls)
	}
}

func TestClaimUnresolved_HanyaSatuPemenang(t *testing.T) {
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "Paket", 100, 1)

	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			won, err := pending.ClaimUnresolved(context.Background(), "REF-1")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			results <- won
		}()
	}
	wg.Wait()
	close(results)

	trueCount, falseCount := 0, 0
	for r := range results {
		if r {
			trueCount++
		} else {
			falseCount++
		}
	}
	if trueCount != 1 || falseCount != 1 {
		t.Fatalf("expected exactly one true and one false, got %d true, %d false", trueCount, falseCount)
	}
}

func TestApplyWebhookEvent_OrderRefTidakDikenal(t *testing.T) {
	repo := newFakeTenantRepo(&domain.Tenant{ID: 1})
	pending := newFakePendingChargeRepo()
	billing := newFakeBilling()
	svc := newTestService(repo, pending, billing, &fakeCharges{})

	if err := svc.ApplyWebhookEvent(context.Background(), "TIDAK-ADA", WebhookEvent{Paid: true}); err != nil {
		t.Fatalf("expected nil error for unknown order_ref, got %v", err)
	}
	if repo.updateSubscriptionCalls != 0 {
		t.Fatalf("expected no activation for unknown order_ref, got %d calls", repo.updateSubscriptionCalls)
	}
}

// --- D8: plan snapshot ---

func TestPay_MenyimpanSnapshotPaket(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	billing := newFakeBilling(billingcontracts.Plan{ID: 10, Name: "Paket Tahunan", DurationMonths: 12, Price: 2_400_000})
	charges := &fakeCharges{
		createSubscriptionCharge: func(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
			return &ChargeResult{OrderRef: orderRef, Channel: "QRIS", Amount: 2_400_000, Status: "unpaid"}, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)

	charge, err := svc.Pay(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}

	stored, err := pending.FindByOrderRef(context.Background(), charge.OrderRef)
	if err != nil || stored == nil {
		t.Fatalf("expected pending charge to be stored, err=%v", err)
	}
	if stored.PlanName != "Paket Tahunan" || stored.PlanPrice != 2_400_000 || stored.PlanDurationMonths != 12 {
		t.Errorf("unexpected snapshot: %+v", stored)
	}
}

func TestPay_MengirimIdentitasTenantKeCharge(t *testing.T) {
	tenant := &domain.Tenant{
		ID: 1, SubscriptionStatus: domain.StatusPendingPayment,
		OwnerName: "Budi Santoso", Email: "budi@example.com", Phone: "081234567890",
	}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	billing := newFakeBilling(billingcontracts.Plan{ID: 10, Name: "Paket Tahunan", DurationMonths: 12, Price: 2_400_000})
	var gotName, gotEmail, gotPhone string
	charges := &fakeCharges{
		createSubscriptionCharge: func(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
			gotName, gotEmail, gotPhone = customerName, customerEmail, customerPhone
			return &ChargeResult{OrderRef: orderRef, Channel: "QRIS", Amount: 2_400_000, Status: "unpaid"}, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)

	if _, err := svc.Pay(context.Background(), 1, 10); err != nil {
		t.Fatalf("Pay: %v", err)
	}
	if gotName != tenant.OwnerName || gotEmail != tenant.Email || gotPhone != tenant.Phone {
		t.Errorf("expected tenant identity (%q, %q, %q), got (%q, %q, %q)",
			tenant.OwnerName, tenant.Email, tenant.Phone, gotName, gotEmail, gotPhone)
	}
}

func TestApplyWebhookEvent_TidakMemanggilElProof(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "Paket Tahunan", 2_400_000, 12)
	billing := newFakeBilling() // no plan 10 registered — GetPlan would error if ever called
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			t.Fatal("ApplyWebhookEvent must never call ElProof")
			return nil, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)

	if err := svc.ApplyWebhookEvent(context.Background(), "REF-1", WebhookEvent{Paid: true}); err != nil {
		t.Fatalf("expected activation to succeed purely from the snapshot, got %v", err)
	}
	if repo.updateSubscriptionCalls != 1 {
		t.Fatalf("expected exactly one activation, got %d", repo.updateSubscriptionCalls)
	}
}

func TestApplyWebhookEvent_PakaiDurasiSnapshotBukanDurasiTerbaru(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "Paket Tahunan", 2_400_000, 12)
	billing := newFakeBilling()
	svc := newTestService(repo, pending, billing, &fakeCharges{})

	before := time.Now()
	if err := svc.ApplyWebhookEvent(context.Background(), "REF-1", WebhookEvent{Paid: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	updated := repo.tenants[1]
	if updated.SubscriptionExpiresAt == nil {
		t.Fatal("expected expiry to be set")
	}
	months := int(updated.SubscriptionExpiresAt.Sub(before).Hours() / 24 / 29) // rough lower bound
	if months < 11 {
		t.Errorf("expected expiry to advance ~12 months from the snapshot, got %v", updated.SubscriptionExpiresAt)
	}
}

func TestGetPendingCharge_FallbackPakaiSnapshotPrice(t *testing.T) {
	tenant := &domain.Tenant{ID: 1}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "Paket Tahunan", 2_400_000, 12)
	billing := newFakeBilling()
	getPlanCalled := false
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			return nil, apperror.Internal("ElProof tidak bisa dihubungi")
		},
	}
	svc := newTestService(repo, pending, billing, charges)

	result, err := svc.GetPendingCharge(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}
	if result.Amount != 2_400_000 || result.Status != "pending" {
		t.Errorf("unexpected fallback result: %+v", result)
	}
	if getPlanCalled || billing.getPlanCallCount != 0 {
		t.Error("expected fallback to never call GetPlan")
	}
}

// --- T7/D16: Pay ordering and ghost rows ---

func TestPay_BarisPendingDitulisSebelumCharge(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	billing := newFakeBilling(billingcontracts.Plan{ID: 10, Name: "P", DurationMonths: 1, Price: 100})
	var existedBeforeCreateCharge bool
	var orderRefSeen string
	charges := &fakeCharges{
		createSubscriptionCharge: func(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
			orderRefSeen = orderRef
			existedBeforeCreateCharge = pending.has(orderRef)
			return &ChargeResult{OrderRef: orderRef, Channel: "QRIS", Amount: 100, Status: "unpaid"}, nil
		},
	}
	svc := newTestService(repo, pending, billing, charges)

	if _, err := svc.Pay(context.Background(), 1, 10); err != nil {
		t.Fatalf("Pay: %v", err)
	}
	if orderRefSeen == "" {
		t.Fatal("CreateSubscriptionCharge was never called")
	}
	if !existedBeforeCreateCharge {
		t.Error("expected the pending row to already exist before CreateSubscriptionCharge was invoked (D16)")
	}
}

func TestPay_ChargeGagalMembersihkanBarisPending(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	billing := newFakeBilling(billingcontracts.Plan{ID: 10, Name: "P", DurationMonths: 1, Price: 100})
	charges := &fakeCharges{
		createSubscriptionCharge: func(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
			return nil, apperror.Internal("gagal terhubung ke ElProof")
		},
	}
	svc := newTestService(repo, pending, billing, charges)

	if _, err := svc.Pay(context.Background(), 1, 10); err == nil {
		t.Fatal("expected Pay to propagate the CreateSubscriptionCharge error")
	}

	if len(pending.charges) != 0 {
		t.Fatalf("expected the pending row to be cleaned up, still have: %+v", pending.charges)
	}

	// A retried Pay must not be blocked by the cleaned-up row.
	charges.createSubscriptionCharge = func(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
		return &ChargeResult{OrderRef: orderRef, Channel: "QRIS", Amount: 100, Status: "unpaid"}, nil
	}
	if _, err := svc.Pay(context.Background(), 1, 10); err != nil {
		t.Fatalf("expected retried Pay to succeed, got %v", err)
	}
}

func TestPay_Charge409MembersihkanBarisPending(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	billing := newFakeBilling(billingcontracts.Plan{ID: 10, Name: "P", DurationMonths: 1, Price: 100})
	charges := &fakeCharges{
		createSubscriptionCharge: func(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
			return nil, apperror.Conflict("order_ref sudah dipakai")
		},
	}
	svc := newTestService(repo, pending, billing, charges)

	_, err := svc.Pay(context.Background(), 1, 10)
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindConflict {
		t.Fatalf("expected a conflict error to propagate, got %v", err)
	}
	if len(pending.charges) != 0 {
		t.Fatalf("expected the pending row to be cleaned up after a 409, still have: %+v", pending.charges)
	}
}

func TestReconcile_404MenyelesaikanBarisHantu(t *testing.T) {
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "GHOST-1", 1, 10, "P", 100, 1)
	// Backdate creation so it clears reconcileMinAge.
	pending.charges["GHOST-1"].CreatedAt = time.Now().Add(-10 * time.Minute)
	billing := newFakeBilling()
	charges := &fakeCharges{
		chargeStatus: func(ctx context.Context, orderRef string) (*ChargeResult, error) {
			return nil, apperror.NotFound("order_ref tidak ditemukan")
		},
	}
	svc := newTestService(repo, pending, billing, charges)
	rec := NewReconciler(pending, charges, svc, 24*time.Hour)

	checked, resolved, err := rec.ReconcilePending(context.Background())
	if err != nil {
		t.Fatalf("ReconcilePending: %v", err)
	}
	if checked != 1 || resolved != 1 {
		t.Fatalf("expected 1 checked and 1 resolved immediately (no max-age wait), got checked=%d resolved=%d", checked, resolved)
	}
	if repo.updateSubscriptionCalls != 0 {
		t.Error("a 404 ghost row must never activate a subscription")
	}
	if pending.has("GHOST-1") {
		t.Error("expected the ghost row to be deleted so a retried Pay isn't blocked by 409")
	}
}

func TestApplyWebhookEvent_TanpaBarisTransaksiTetapMengaktifkan(t *testing.T) {
	// RecordTransaction having failed earlier (e.g. right after Pay's
	// CreateCharge succeeded) leaves no subscription_transactions row for
	// this order_ref — ApplyWebhookEvent must still activate: it never reads
	// billing's transaction table, only pendingCharges + the tenant repo.
	tenant := &domain.Tenant{ID: 1, SubscriptionStatus: domain.StatusPendingPayment}
	repo := newFakeTenantRepo(tenant)
	pending := newFakePendingChargeRepo()
	_ = pending.Create(context.Background(), "REF-1", 1, 10, "P", 100, 1)
	billing := newFakeBilling() // no RecordTransaction ever called for REF-1
	svc := newTestService(repo, pending, billing, &fakeCharges{})

	if err := svc.ApplyWebhookEvent(context.Background(), "REF-1", WebhookEvent{Paid: true}); err != nil {
		t.Fatalf("expected activation despite no transaction row, got %v", err)
	}
	if repo.updateSubscriptionCalls != 1 {
		t.Error("expected the subscription to still be activated")
	}
}
