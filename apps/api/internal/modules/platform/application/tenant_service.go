package application

import (
	"context"
	"encoding/base64"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"

	billingcontracts "jwswedding/internal/modules/billing/contracts"
	"jwswedding/internal/modules/platform/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/compress"
	"jwswedding/internal/shared/filename"
	"jwswedding/internal/shared/logger"
)

// maxLogoDecodedSize mirrors projects/application's evidence maxDecodedSize
// (ADR-0010's cap) — same 15MB ceiling, deliberately not shared across
// modules (a numeric literal, not domain logic).
const maxLogoDecodedSize = 15 * 1024 * 1024

// allowedLogoMimeTypes is a whitelist, not a pass-through like
// compress.Image's default branch — a logo that isn't one of these 3 types
// is rejected outright rather than stored byte-identical (PLAN.md
// invoice-kwitansi-client §4.6).
var allowedLogoMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// allowedSignatureMimeTypes is deliberately narrower than the logo whitelist
// — PNG only, per the user's explicit requirement (PLAN.md
// redesain-pdf-invoice-kwitansi §D4/§D10): a signature typically needs a
// transparent background, which JPEG can't carry and WEBP's alpha survives
// this app's own compress/render pipeline less predictably.
var allowedSignatureMimeTypes = map[string]bool{
	"image/png": true,
}

// ChargeResult mirrors elproofpay.ChargeResult — kept as a local type (not
// imported directly) so this module's application layer never depends on
// `internal/shared/elproofpay`'s own type identity, only on the narrow
// ChargeCreator interface below. Same idiom as the ObjectStorage interface.
type ChargeResult struct {
	OrderRef    string
	ProviderRef string
	Channel     string
	QRImageURL  string
	PayCode     string
	CheckoutURL string
	Amount      int64
	FeeAmount   int64
	ExpiresAt   time.Time
	Status      string
}

// WebhookEvent is the generic shape ElProof's webhook relay (or this
// module's own reconciler) resolves a charge outcome into — see D8: unlike
// `payment`'s old WebhookEvent, ProviderRef/Amount/PaidAt are never read by
// ApplyWebhookEvent, so they aren't carried here either (T4).
type WebhookEvent struct {
	Paid bool
}

// ChargeCreator is the narrow slice of elproofpay.Client this service needs
// to create/check a charge at ElProof — exactly the 2 methods `platform`
// ever calls. Declared locally (like ObjectStorage below) so this module
// never imports another module's infrastructure; the concrete implementation
// (platform/infrastructure.ElProofChargeClient) just delegates to
// elproofpay.Client.
type ChargeCreator interface {
	CreateSubscriptionCharge(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error)
	ChargeStatus(ctx context.Context, orderRef string) (*ChargeResult, error)
}

// TenantRepository is deliberately narrower than MySQLTenantRepository's full
// method set (List/ListPaginated/Update/UpdateLogo/SetSuspended/
// SetCredentialResetAt still exist there, just unused now that the Platform
// Console CRUD they backed is gone — D2) — this interface only declares what
// TenantService actually calls.
type TenantRepository interface {
	FindByID(ctx context.Context, id int64) (*domain.Tenant, error)
	FindByDomain(ctx context.Context, host string) (*domain.Tenant, error)
	UpdateSubscription(ctx context.Context, id int64, planID int64, status domain.SubscriptionStatus, expiresAt time.Time) error
	// Update/UpdateLogo back the self-service "Profil Usaha" page (PLAN.md
	// invoice-kwitansi-client §1.7) — both already fully implemented on
	// MySQLTenantRepository (unused since Platform Console's admin tenant
	// CRUD was cut), just re-declared here now that TenantService calls them
	// again.
	Update(ctx context.Context, tenant *domain.Tenant) error
	UpdateLogo(ctx context.Context, id int64, logoStoragePath *string) error
	// UpdateSignature backs the self-service "Profil Usaha" signature upload
	// (PLAN.md redesain-pdf-invoice-kwitansi §D4/§D7) — same shape as
	// UpdateLogo.
	UpdateSignature(ctx context.Context, id int64, signatureStoragePath *string) error
}

// ObjectStorage is the narrow slice of internal/shared/storage.Client this
// service depends on for tenant logo uploads — package-local (like
// projects/application.ObjectStorage) so this module never imports another
// module's application layer, per the modular-monolith boundary.
type ObjectStorage interface {
	Save(ctx context.Context, key string, data []byte, contentType string) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// PendingChargeRepository backs the self-service payment flow — see
// domain.PendingCharge. Create takes the plan snapshot (D8); FindByOrderRef/
// FindByTenant only ever return unresolved rows; FindUnresolved/
// ClaimUnresolved back the reconciler and webhook handler's atomic claim
// (D11).
type PendingChargeRepository interface {
	Create(ctx context.Context, orderRef string, tenantID, planID int64, planName string, planPrice int64, planDurationMonths int) error
	FindByOrderRef(ctx context.Context, orderRef string) (*domain.PendingCharge, error)
	FindByTenant(ctx context.Context, tenantID int64) ([]domain.PendingCharge, error)
	FindUnresolved(ctx context.Context, olderThan time.Duration, limit int) ([]domain.PendingCharge, error)
	ClaimUnresolved(ctx context.Context, orderRef string) (bool, error)
	Delete(ctx context.Context, orderRef string) error
}

type TenantService struct {
	repo           TenantRepository
	pendingCharges PendingChargeRepository
	billing        billingcontracts.Contracts
	charges        ChargeCreator
	storage        ObjectStorage
	buildKey       func(tenantID, projectID, category, filename string) string
}

func NewTenantService(
	repo TenantRepository,
	pendingCharges PendingChargeRepository,
	billing billingcontracts.Contracts,
	charges ChargeCreator,
	storage ObjectStorage,
	buildKey func(string, string, string, string) string,
) *TenantService {
	return &TenantService{
		repo: repo, pendingCharges: pendingCharges,
		billing: billing, charges: charges,
		storage: storage, buildKey: buildKey,
	}
}

// WritesAllowed backs the read-only subscription guard (D10, D13): it
// evaluates subscription_expires_at directly, never subscription_status —
// no code path ever writes StatusExpired/StatusExpiringSoon (T3), so a
// status-based check would never trip.
func (s *TenantService) WritesAllowed(ctx context.Context, tenantID int64) (bool, error) {
	tenant, err := s.Get(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return tenant.SubscriptionExpiresAt != nil && tenant.SubscriptionExpiresAt.After(time.Now()), nil
}

func (s *TenantService) Get(ctx context.Context, id int64) (*domain.Tenant, error) {
	tenant, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, apperror.NotFound("Tenant tidak ditemukan")
	}
	return tenant, nil
}

// GetBrandingByDomain resolves a tenant from an incoming request's Host
// header (ADR-0015) — the pre-auth counterpart to Get, used only by the
// public branding/logo endpoints, never by anything JWT-scoped.
func (s *TenantService) GetBrandingByDomain(ctx context.Context, host string) (*domain.Tenant, error) {
	tenant, err := s.repo.FindByDomain(ctx, host)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, apperror.NotFound("Domain tidak terdaftar")
	}
	return tenant, nil
}

// DownloadLogo streams a tenant's stored logo back — same byte-proxy shape
// as evidence.Download, since object storage requires backend auth and can't
// be linked to directly from the browser.
func (s *TenantService) DownloadLogo(ctx context.Context, tenantID int64) (io.ReadCloser, error) {
	tenant, err := s.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if tenant.LogoStoragePath == nil {
		return nil, apperror.NotFound("Tenant belum memiliki logo")
	}
	reader, err := s.storage.Open(ctx, *tenant.LogoStoragePath)
	if err != nil {
		return nil, apperror.Internal("Gagal mengambil logo dari object storage")
	}
	return reader, nil
}

// UpdateProfileInput carries every field the self-service "Profil Usaha" page
// (Owner-only, PATCH /tenants/me) can write — deliberately excludes
// BrandColorPreset/CustomDomain, which have no write path at all anymore
// (PLAN.md invoice-kwitansi-client §4.6).
type UpdateProfileInput struct {
	BusinessName, OwnerName, Email, Phone, City                 string
	Address, BankName, BankAccountNumber, BankAccountHolderName string
}

// UpdateProfile applies input onto the tenant's existing record and persists
// it — a full-field overwrite of exactly the 9 profile fields (5 pre-existing
// ones the Owner previously had no way to edit themselves, plus the 4 new
// ones from §1.7), never touching BrandColorPreset/CustomDomain/subscription
// state.
func (s *TenantService) UpdateProfile(ctx context.Context, id int64, input UpdateProfileInput) (*domain.Tenant, error) {
	tenant, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	tenant.BusinessName, tenant.OwnerName, tenant.Email, tenant.Phone, tenant.City =
		input.BusinessName, input.OwnerName, input.Email, input.Phone, input.City
	tenant.Address, tenant.BankName, tenant.BankAccountNumber, tenant.BankAccountHolderName =
		input.Address, input.BankName, input.BankAccountNumber, input.BankAccountHolderName
	if err := s.repo.Update(ctx, tenant); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

type UploadLogoInput struct {
	FileName, MimeType string
	Base64Data         string
}

// UploadLogo is genuinely new code (not a reuse of any prior admin-upload
// path — that one was deleted along with Platform Console), mirroring
// EvidenceService.Upload's body: decode, cap size, whitelist MIME (a real
// validator, unlike compress.Image's pass-through default), compress,
// build a storage key, save, then persist the path (PLAN.md
// invoice-kwitansi-client §4.6).
func (s *TenantService) UploadLogo(ctx context.Context, tenantID int64, input UploadLogoInput) (*domain.Tenant, error) {
	if !allowedLogoMimeTypes[input.MimeType] {
		return nil, apperror.Validation("Format logo tidak didukung", map[string][]string{
			"mimeType": {"Gunakan JPEG, PNG, atau WEBP"},
		})
	}

	decoded, err := base64.StdEncoding.DecodeString(input.Base64Data)
	if err != nil {
		return nil, apperror.Validation("Data file tidak valid", map[string][]string{"base64Data": {"Gagal membaca data file"}})
	}
	if len(decoded) == 0 {
		return nil, apperror.Validation("File kosong", map[string][]string{"base64Data": {"File tidak boleh kosong"}})
	}
	if len(decoded) > maxLogoDecodedSize {
		return nil, apperror.Validation("Ukuran file terlalu besar", map[string][]string{"base64Data": {"Maksimal 15 MB"}})
	}

	compressed, err := compress.Image(decoded, input.MimeType)
	if err != nil {
		return nil, apperror.Internal("Gagal memproses file")
	}

	// Sentinel "0" for the projectID slot — a tenant's logo belongs to no
	// single project, same convention buildKey callers elsewhere use for a
	// tenant-level (not project-level) upload.
	key := s.buildKey(
		strconv.FormatInt(tenantID, 10), "0", "logo",
		uuid.NewString()+"-"+filename.Sanitize(input.FileName),
	)
	if _, err := s.storage.Save(ctx, key, compressed, input.MimeType); err != nil {
		return nil, apperror.Internal("Gagal mengunggah file ke object storage")
	}
	if err := s.repo.UpdateLogo(ctx, tenantID, &key); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID)
}

type UploadSignatureInput struct {
	FileName, MimeType string
	Base64Data         string
}

// UploadSignature mirrors UploadLogo's body exactly (decode, cap size,
// whitelist MIME, compress, build a storage key, save, then persist the
// path), the only differences being the PNG-only whitelist and the
// "signature" storage key category (PLAN.md
// redesain-pdf-invoice-kwitansi §D4/§D10).
func (s *TenantService) UploadSignature(ctx context.Context, tenantID int64, input UploadSignatureInput) (*domain.Tenant, error) {
	if !allowedSignatureMimeTypes[input.MimeType] {
		return nil, apperror.Validation("Format tanda tangan tidak didukung", map[string][]string{
			"mimeType": {"Gunakan file PNG"},
		})
	}

	decoded, err := base64.StdEncoding.DecodeString(input.Base64Data)
	if err != nil {
		return nil, apperror.Validation("Data file tidak valid", map[string][]string{"base64Data": {"Gagal membaca data file"}})
	}
	if len(decoded) == 0 {
		return nil, apperror.Validation("File kosong", map[string][]string{"base64Data": {"File tidak boleh kosong"}})
	}
	if len(decoded) > maxLogoDecodedSize {
		return nil, apperror.Validation("Ukuran file terlalu besar", map[string][]string{"base64Data": {"Maksimal 15 MB"}})
	}

	compressed, err := compress.Image(decoded, input.MimeType)
	if err != nil {
		return nil, apperror.Internal("Gagal memproses file")
	}

	// Sentinel "0" for the projectID slot — same convention UploadLogo uses
	// for a tenant-level (not project-level) upload.
	key := s.buildKey(
		strconv.FormatInt(tenantID, 10), "0", "signature",
		uuid.NewString()+"-"+filename.Sanitize(input.FileName),
	)
	if _, err := s.storage.Save(ctx, key, compressed, input.MimeType); err != nil {
		return nil, apperror.Internal("Gagal mengunggah file ke object storage")
	}
	if err := s.repo.UpdateSignature(ctx, tenantID, &key); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID)
}

// DownloadSignature mirrors DownloadLogo's byte-proxy shape exactly.
func (s *TenantService) DownloadSignature(ctx context.Context, tenantID int64) (io.ReadCloser, error) {
	tenant, err := s.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if tenant.SignatureStoragePath == nil {
		return nil, apperror.NotFound("Tenant belum memiliki tanda tangan")
	}
	reader, err := s.storage.Open(ctx, *tenant.SignatureStoragePath)
	if err != nil {
		return nil, apperror.Internal("Gagal mengambil tanda tangan dari object storage")
	}
	return reader, nil
}

// ActivateSubscription is an ops-only manual-activation path (D2: no HTTP
// route exposes it anymore, since the Platform Console it used to serve is
// gone) — bypasses payment entirely; recorded as a "granted" transaction so
// it never counts as real revenue. See ADR (subscription bypass) and
// docs/DB_SCHEMA.md. Unlike Pay, this never touches ElProof at all — it
// activates synchronously, in this same call.
//
// If the Owner's own self-service "Bayar Sekarang" charge is still pending
// for this tenant, it's resolved here first: expired in the transaction
// ledger and cleared from pendingCharges tracking. Without this, that row
// stayed stuck at "Menunggu Pembayaran" until the payment module's
// reconciliation sweep eventually caught up with the gateway (its own
// natural charge expiry, potentially hours later) — and if the Owner
// actually completed that old payment in the meantime, the late webhook
// would still fire ApplyWebhookEvent and call UpdateSubscription a second
// time, silently double-extending the expiry this call is about to set
// (computeNewExpiry stacks on top of whatever expiry is already there).
func (s *TenantService) ActivateSubscription(ctx context.Context, tenantID int64, planID int64) (*domain.Tenant, error) {
	tenant, err := s.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	plan, err := s.billing.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	pending, err := s.pendingCharges.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, p := range pending {
		if err := s.billing.UpdateTransactionStatus(ctx, p.OrderRef, billingcontracts.StatusExpired); err != nil {
			return nil, err
		}
		if err := s.pendingCharges.Delete(ctx, p.OrderRef); err != nil {
			return nil, err
		}
	}

	if err := s.billing.RecordTransaction(ctx, billingcontracts.RecordTransactionInput{
		TenantID: tenantID, Type: txTypeFor(tenant), Amount: plan.Price,
		PaymentMethod: "Aktivasi Manual (Super Admin)", PaymentReference: generateGrantReference(),
		Status: billingcontracts.StatusGranted,
	}); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSubscription(ctx, tenantID, planID, domain.StatusActive, computeNewExpiry(tenant, plan.DurationMonths)); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID)
}

// Pay is the tenant Owner's own self-service "Bayar Sekarang" action —
// creates a real charge at ElProof's subscription-charge endpoint (QRIS by
// default; amount is derived by ElProof from planID's own catalog price)
// and returns it for the frontend to render; the subscription is NOT
// activated here. Activation only happens once ElProof's webhook confirms
// payment (see ApplyWebhookEvent below) or the reconciler catches up (T1) —
// this function only ever gets the charge started.
//
// Guard: rejects a second charge while one is already pending for this
// tenant (409), instead of silently starting a second, independent charge —
// this is real money moved at an external gateway (unlike ActivateSubscription,
// an internal admin action), so the guard lives here in the backend rather
// than relying only on the frontend disabling its button while a request is
// in flight. If both charges were ever paid, the tenant would be billed
// twice and the subscription would be double-extended (computeNewExpiry
// stacks on top of whatever expiry is already there).
//
// Order is deliberately reversed from the old payment-module-backed flow
// (D16/T7): the local pendingCharges row is written BEFORE calling ElProof.
// It is now the only local record of this charge and the reconciler's only
// input, so a process death between the two calls must never leave a charge
// at ElProof with zero local trace. If CreateSubscriptionCharge itself fails
// (network, or a 409 from a duplicate orderRef), the just-written row is
// deleted so a retried Pay never gets stuck behind its own failed attempt.
func (s *TenantService) Pay(ctx context.Context, tenantID int64, planID int64) (*ChargeResult, error) {
	tenant, err := s.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	plan, err := s.billing.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	existing, err := s.pendingCharges.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, apperror.Conflict("Anda masih memiliki tagihan yang belum diselesaikan. Selesaikan atau tunggu tagihan tersebut kedaluwarsa sebelum membuat tagihan baru.")
	}

	orderRef := generatePaymentReference()
	if err := s.pendingCharges.Create(ctx, orderRef, tenantID, planID, plan.Name, plan.Price, plan.DurationMonths); err != nil {
		return nil, err
	}

	charge, err := s.charges.CreateSubscriptionCharge(ctx, orderRef, planID, tenant.OwnerName, tenant.Email, tenant.Phone)
	if err != nil {
		if delErr := s.pendingCharges.Delete(ctx, orderRef); delErr != nil {
			logger.Error("gagal membersihkan baris pending %s setelah CreateSubscriptionCharge gagal: %v", orderRef, delErr)
		}
		return nil, err
	}

	if err := s.billing.RecordTransaction(ctx, billingcontracts.RecordTransactionInput{
		TenantID: tenantID, Type: txTypeFor(tenant), Amount: plan.Price,
		PaymentMethod: charge.Channel, PaymentReference: orderRef, Status: billingcontracts.StatusPending,
	}); err != nil {
		return nil, err
	}

	return charge, nil
}

// GetPendingCharge lets the Owner re-view a still-pending self-service
// charge after closing its QR modal (accidentally or otherwise) — instead of
// the charge's display fields (QR image, pay code, checkout URL) being lost
// forever, since they were never persisted anywhere beyond Pay's one-time
// response. Re-fetches them live from ElProof via ChargeStatus rather than
// caching a copy that could go stale. Returns nil (not an error) when
// there's nothing pending — this is a normal, common state.
func (s *TenantService) GetPendingCharge(ctx context.Context, tenantID int64) (*ChargeResult, error) {
	pending, err := s.pendingCharges.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(pending) == 0 {
		return nil, nil
	}
	p := pending[0]
	charge, err := s.charges.ChargeStatus(ctx, p.OrderRef)
	if err != nil {
		// The live ElProof call can fail for reasons that have nothing to do
		// with whether the charge is still genuinely pending (an expired
		// service token exchange, ElProof down, a transient network error) —
		// letting that failure bubble up as an opaque error here would
		// silently hide the pending-charge banner (the frontend fetch has no
		// visible failure state) and strand the Owner exactly like the bug
		// this endpoint exists to fix: unable to see or cancel their pending
		// charge. Log for diagnosis and fall back to the snapshot already
		// stored locally (D8) — no QR/pay code/checkout URL (only ElProof has
		// those), but enough for the Owner to see it's pending and use
		// "Batalkan", without a second call to ElProof's plan catalog.
		logger.Error("gagal memuat status live charge %s dari ElProof: %v", p.OrderRef, err)
		return &ChargeResult{OrderRef: p.OrderRef, Amount: p.PlanPrice, Status: "pending"}, nil
	}
	return charge, nil
}

// CancelPendingCharge is the Owner's "Batalkan" action — gives up on a
// pending self-service charge without waiting for it to expire on its own,
// freeing Pay() to start a fresh one immediately. Tripay has no cancel/void
// API, so this is bookkeeping on our side only: the transaction is marked
// StatusCancelled (distinct from StatusExpired, which means it timed out
// unattended) and its pendingCharges tracking row is deleted. If the Owner
// somehow still completes payment on the cancelled QR afterward, the late
// webhook/reconciliation result finds no tracking row for that order_ref and
// is silently ignored (see ApplyWebhookEvent's nil-pending branch) rather
// than resurrecting a subscription the Owner explicitly walked away from.
func (s *TenantService) CancelPendingCharge(ctx context.Context, tenantID int64) error {
	pending, err := s.pendingCharges.FindByTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return apperror.NotFound("Tidak ada tagihan yang menggantung untuk dibatalkan")
	}
	for _, p := range pending {
		if err := s.billing.UpdateTransactionStatus(ctx, p.OrderRef, billingcontracts.StatusCancelled); err != nil {
			return err
		}
		if err := s.pendingCharges.Delete(ctx, p.OrderRef); err != nil {
			return err
		}
	}
	return nil
}

// ApplyWebhookEvent applies a charge outcome discovered either via ElProof's
// webhook relay (platform/presentation.WebhookHandler) or this module's own
// reconciler (Reconciler.ReconcilePending, T1) — both call this same method,
// so downstream behavior is identical regardless of how the outcome was
// discovered.
//
// ClaimUnresolved (D11) makes this safe against the two ever racing on the
// same orderRef: only whichever caller wins the atomic claim proceeds, the
// loser gets a no-op. Duration comes from the snapshot stored at Pay-time
// (pending.PlanDurationMonths, D8), never from billing.GetPlan — this is
// exactly what keeps activation working even if ElProof itself is
// unreachable when the webhook/reconciler fires.
func (s *TenantService) ApplyWebhookEvent(ctx context.Context, orderRef string, event WebhookEvent) error {
	pending, err := s.pendingCharges.FindByOrderRef(ctx, orderRef)
	if err != nil {
		return err
	}
	if pending == nil {
		return nil // unknown, already-consumed, or already-resolved order_ref — idempotent no-op
	}

	won, err := s.pendingCharges.ClaimUnresolved(ctx, orderRef)
	if err != nil {
		return err
	}
	if !won {
		return nil // the webhook and the reconciler raced on this order_ref — the other one already claimed it
	}

	if !event.Paid {
		if err := s.billing.UpdateTransactionStatus(ctx, orderRef, billingcontracts.StatusExpired); err != nil {
			return err
		}
		return s.pendingCharges.Delete(ctx, orderRef)
	}

	tenant, err := s.Get(ctx, pending.TenantID)
	if err != nil {
		return err
	}

	if err := s.billing.UpdateTransactionStatus(ctx, orderRef, billingcontracts.StatusPaid); err != nil {
		return err
	}
	if err := s.repo.UpdateSubscription(ctx, pending.TenantID, pending.PlanID, domain.StatusActive, computeNewExpiry(tenant, pending.PlanDurationMonths)); err != nil {
		return err
	}
	return s.pendingCharges.Delete(ctx, orderRef)
}

func txTypeFor(tenant *domain.Tenant) billingcontracts.TransactionType {
	if tenant.SubscriptionStatus != domain.StatusPendingPayment {
		return billingcontracts.TransactionRenewal
	}
	return billingcontracts.TransactionNew
}

// computeNewExpiry extends from the tenant's existing expiry if their plan
// is still active (renewal stacks on top of remaining time), otherwise from
// now (new subscription, or reactivating a lapsed one).
func computeNewExpiry(tenant *domain.Tenant, durationMonths int) time.Time {
	wasActive := tenant.SubscriptionStatus == domain.StatusActive || tenant.SubscriptionStatus == domain.StatusExpiringSoon
	base := time.Now()
	if wasActive && tenant.SubscriptionExpiresAt != nil && tenant.SubscriptionExpiresAt.After(base) {
		base = *tenant.SubscriptionExpiresAt
	}
	return base.AddDate(0, durationMonths, 0)
}

// ParseTenantID converts a JWT tenant-id claim (string) to int64 — used by the
// presentation layer for the self-service "pay" endpoint.
func ParseTenantID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
