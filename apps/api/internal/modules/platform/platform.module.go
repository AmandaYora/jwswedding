// Package platform wires the platform module: tenant lifecycle and the
// read-only subscription guard's own contract. The Platform Console (7
// pages of multi-tenant admin) is gone (D2) — jwswedding is single-tenant,
// seeded via internal/adminseed, not self-registered through this module.
package platform

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	billingcontracts "jwswedding/internal/modules/billing/contracts"
	"jwswedding/internal/modules/platform/application"
	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/platform/infrastructure"
	"jwswedding/internal/modules/platform/presentation"
	"jwswedding/internal/shared/elproofpay"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/storage"
)

type Module struct {
	tenantHandler      *presentation.TenantHandler
	webhookHandler     *presentation.WebhookHandler
	loginSlidesHandler *presentation.LoginSlidesHandler
	tenantService      *application.TenantService
	reconciler         *application.Reconciler
}

func NewModule(
	db *sql.DB,
	billing billingcontracts.Contracts,
	elproofClient *elproofpay.Client,
	chargeMaxAge time.Duration,
	storageClient *storage.Client,
) *Module {
	tenantRepo := infrastructure.NewMySQLTenantRepository(db)
	pendingChargeRepo := infrastructure.NewMySQLPendingChargeRepository(db)
	charges := infrastructure.NewElProofChargeClient(elproofClient)

	tenantService := application.NewTenantService(
		tenantRepo, pendingChargeRepo, billing, charges, storageClient, storage.BuildKey,
	)
	reconciler := application.NewReconciler(pendingChargeRepo, charges, tenantService, chargeMaxAge)

	return &Module{
		tenantHandler:      presentation.NewTenantHandler(tenantService),
		webhookHandler:     presentation.NewWebhookHandler(elproofClient, tenantService),
		loginSlidesHandler: presentation.NewLoginSlidesHandler(storageClient),
		tenantService:      tenantService,
		reconciler:         reconciler,
	}
}

// Contracts exposes WritesAllowed to shared/middleware's subscription guard
// — platform's first contracts package (D5/D10/D13).
func (m *Module) Contracts() platformcontracts.Contracts {
	return platformcontracts.New(m.tenantService)
}

// StartReconciler runs the reconciliation sweep (T1) on a fixed interval
// until ctx is cancelled.
func (m *Module) StartReconciler(ctx context.Context, interval time.Duration) {
	m.reconciler.Start(ctx, interval)
}

// SiteMeta is the minimal, primitive-typed slice of a tenant's branding
// needed to render a request's <title>/Open Graph tags at HTML-serve time —
// deliberately not domain.Tenant itself, same primitive-typed-bridge
// reasoning as StaffNameResolver (projects/application): main.go's
// spaFileServer is composition-root code, not another module, but there's
// still no reason for it to see this module's domain internals for a need
// this narrow.
type SiteMeta struct {
	BusinessName string
	HasLogo      bool
}

// SiteMetaForHost resolves a tenant's own display name for injecting into
// index.html's <title>/Open Graph tags at serve time (spaFileServer in
// main.go) — same Host-based lookup as PublicBranding (ADR-0015), reused
// here so a shared link's preview card (WhatsApp/Telegram/etc., which never
// execute the SPA's JS) shows the tenant's real name instead of the
// platform's own. ok=false for the platform's own domain or any unmatched
// Host, exactly like PublicBranding's 404.
func (m *Module) SiteMetaForHost(ctx context.Context, host string) (meta SiteMeta, ok bool) {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	tenant, err := m.tenantService.GetBrandingByDomain(ctx, strings.ToLower(host))
	if err != nil {
		return SiteMeta{}, false
	}
	return SiteMeta{BusinessName: tenant.BusinessName, HasLogo: tenant.LogoStoragePath != nil}, true
}

// RegisterPublicRoutes registers this module's pre-auth endpoints — Host-header-resolved
// tenant branding for a custom domain's login page (ADR-0015), plus
// ElProof's webhook relay (PAYMENT_INTEGRATION_GUIDE.md §8) — never wrapped
// in the `authed` middleware, since neither a browser session nor ElProof's
// server-to-server call carries one.
func (m *Module) RegisterPublicRoutes(mux *http.ServeMux) {
	mux.Handle("/api/v1/public/branding", httpx.Method(http.MethodGet, m.tenantHandler.PublicBranding))
	mux.Handle("/api/v1/public/logo", httpx.Method(http.MethodGet, m.tenantHandler.PublicLogo))
	mux.Handle("/api/v1/public/app-icon/", httpx.Method(http.MethodGet, m.tenantHandler.PublicAppIcon))
	mux.Handle("/api/v1/public/login-slides", httpx.Method(http.MethodGet, m.loginSlidesHandler.List))
	mux.Handle("/api/v1/public/login-slides/", httpx.Method(http.MethodGet, m.loginSlidesHandler.Slide))
	mux.Handle("/webhooks/elproof-payment", httpx.Method(http.MethodPost, m.webhookHandler.Receive))
	// PWA manifest — registered at the site root, not under /api/v1/, so its
	// implicit scope covers "/" (docs/plan/ikon-homescreen-pwa/PLAN.md §3.3
	// A1). Still safe to register alongside these pre-auth API routes: Go's
	// ServeMux picks the most specific pattern regardless of registration
	// order, so this never shadows spaFileServer's own "/" catch-all in
	// main.go, and vice versa.
	mux.Handle("/manifest.webmanifest", httpx.Method(http.MethodGet, m.tenantHandler.PublicManifest))
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/tenants/", authed(http.HandlerFunc(m.tenantHandler.Item)))
	mux.Handle("/api/v1/subscriptions/pay", authed(httpx.Method(http.MethodPost, m.tenantHandler.Pay)))
	mux.Handle("/api/v1/subscriptions/pending-charge", authed(httpx.Method(http.MethodGet, m.tenantHandler.PendingCharge)))
	mux.Handle("/api/v1/subscriptions/pending-charge/cancel", authed(httpx.Method(http.MethodPost, m.tenantHandler.CancelPendingCharge)))
}
