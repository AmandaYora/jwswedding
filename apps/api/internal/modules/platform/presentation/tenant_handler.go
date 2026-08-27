package presentation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"jwswedding/internal/modules/platform/application"
	"jwswedding/internal/modules/platform/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/response"
)

type TenantHandler struct {
	tenants *application.TenantService
}

func NewTenantHandler(tenants *application.TenantService) *TenantHandler {
	return &TenantHandler{tenants: tenants}
}

type tenantResponse struct {
	ID                    int64   `json:"id"`
	BusinessName          string  `json:"businessName"`
	OwnerName             string  `json:"ownerName"`
	Username              string  `json:"username"`
	Email                 string  `json:"email"`
	Phone                 string  `json:"phone"`
	City                  string  `json:"city"`
	JoinedAt              string  `json:"joinedAt"`
	PlanID                *int64  `json:"planId"`
	SubscriptionStatus    string  `json:"subscriptionStatus"`
	SubscriptionExpiresAt *string `json:"subscriptionExpiresAt"`
	IsSuspended           bool    `json:"isSuspended"`
	LastCredentialResetAt *string `json:"lastCredentialResetAt"`
	BrandColorPreset      string  `json:"brandColorPreset"`
	HasLogo               bool    `json:"hasLogo"`
	HasSignature          bool    `json:"hasSignature"`
	CustomDomain          *string `json:"customDomain"`
	// Address/BankName/BankAccountNumber/BankAccountHolderName back the
	// "Profil Usaha" page and the Invoice/Kwitansi PDF kop surat (PLAN.md
	// invoice-kwitansi-client §1.7).
	Address               string `json:"address"`
	BankName              string `json:"bankName"`
	BankAccountNumber     string `json:"bankAccountNumber"`
	BankAccountHolderName string `json:"bankAccountHolderName"`
	// ProfileComplete/MissingProfileFields back the "Profil Usaha" page's own
	// completeness banner (PLAN.md redesain-pdf-invoice-kwitansi-v2 §6.3.7) —
	// computed the same way, and from the same domain.ProfileMissingFields,
	// as the PDF-download gate itself, so this page never drifts from what
	// actually blocks printing.
	ProfileComplete      bool     `json:"profileComplete"`
	MissingProfileFields []string `json:"missingProfileFields"`
}

// brandingResponse is the minimal, non-sensitive branding shape the
// UNAUTHENTICATED PublicBranding endpoint (ADR-0015) returns — deliberately
// carries no profile-completeness fields (T4: an unknown Host header must
// never leak which of a tenant's profile fields are missing).
type brandingResponse struct {
	BusinessName     string `json:"businessName"`
	BrandColorPreset string `json:"brandColorPreset"`
	HasLogo          bool   `json:"hasLogo"`
}

// myBrandingResponse is myBranding's own response shape — brandingResponse's
// fields (embedded, so the two can never drift apart if a plain branding
// field is ever added) plus the profile-completeness gate (PLAN.md
// redesain-pdf-invoice-kwitansi-v2 §6.2/T4) — used ONLY by the
// authenticated self-service myBranding handler, never by the pre-auth
// PublicBranding one.
type myBrandingResponse struct {
	brandingResponse
	ProfileComplete      bool     `json:"profileComplete"`
	MissingProfileFields []string `json:"missingProfileFields"`
}

func dateOrNil(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func toTenantResponse(t domain.Tenant) tenantResponse {
	missing := domain.ProfileMissingFields(t)
	return tenantResponse{
		ID: t.ID, BusinessName: t.BusinessName, OwnerName: t.OwnerName, Username: t.Username,
		Email: t.Email, Phone: t.Phone, City: t.City, JoinedAt: t.JoinedAt.Format("2006-01-02"),
		PlanID: t.PlanID, SubscriptionStatus: string(t.SubscriptionStatus),
		SubscriptionExpiresAt: dateOrNil(t.SubscriptionExpiresAt), IsSuspended: t.IsSuspended,
		LastCredentialResetAt: dateOrNil(t.LastCredentialResetAt),
		BrandColorPreset:      t.BrandColorPreset, HasLogo: t.LogoStoragePath != nil,
		HasSignature: t.SignatureStoragePath != nil,
		CustomDomain: t.CustomDomain,
		Address: t.Address, BankName: t.BankName, BankAccountNumber: t.BankAccountNumber,
		BankAccountHolderName: t.BankAccountHolderName,
		ProfileComplete:       len(missing) == 0, MissingProfileFields: missing,
	}
}

func toBrandingResponse(t domain.Tenant) brandingResponse {
	return brandingResponse{
		BusinessName: t.BusinessName, BrandColorPreset: t.BrandColorPreset, HasLogo: t.LogoStoragePath != nil,
	}
}

// toMyBrandingResponse builds on toBrandingResponse plus the profile-
// completeness gate — used only by the authenticated myBranding handler
// (PLAN.md redesain-pdf-invoice-kwitansi-v2 §6.2, T4). Building on top of
// toBrandingResponse (rather than duplicating its field assignments) means
// a future field added there is automatically picked up here too.
func toMyBrandingResponse(t domain.Tenant) myBrandingResponse {
	missing := domain.ProfileMissingFields(t)
	return myBrandingResponse{
		brandingResponse: toBrandingResponse(t),
		ProfileComplete:  len(missing) == 0, MissingProfileFields: missing,
	}
}

type chargeResponse struct {
	OrderRef    string `json:"orderRef"`
	ProviderRef string `json:"providerRef"`
	Channel     string `json:"channel"`
	QRImageURL  string `json:"qrImageUrl"`
	PayCode     string `json:"payCode"`
	CheckoutURL string `json:"checkoutUrl"`
	Amount      int64  `json:"amount"`
	FeeAmount   int64  `json:"feeAmount"`
	ExpiresAt   string `json:"expiresAt"`
	Status      string `json:"status"`
}

func toChargeResponse(c application.ChargeResult) chargeResponse {
	return chargeResponse{
		OrderRef: c.OrderRef, ProviderRef: c.ProviderRef, Channel: c.Channel,
		QRImageURL: c.QRImageURL, PayCode: c.PayCode, CheckoutURL: c.CheckoutURL,
		Amount: c.Amount, FeeAmount: c.FeeAmount, ExpiresAt: c.ExpiresAt.Format(time.RFC3339), Status: c.Status,
	}
}

func (h *TenantHandler) Item(w http.ResponseWriter, r *http.Request) {
	segments := httpx.Segments(r.URL.Path, "/api/v1/tenants/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Tenant tidak ditemukan", nil)
		return
	}

	// "me" is the authenticated principal's own self-service read + partial
	// write — the only path this handler serves at all now that Platform
	// Console CRUD is gone (D2). /me itself (full tenant incl. subscription,
	// GET/PATCH), /me/logo's PUT, and /me/signature's PUT stay Owner-only;
	// /me/branding and GET /me/logo are open to any tenant-scoped principal
	// (any staff role, or client), since branding must render for everyone
	// inside WO Console / Client Portal, not just the Owner. GET
	// /me/signature follows the same "any tenant-scoped principal" rule as
	// GET /me/logo — the Kwitansi PDF it feeds is reachable from Client
	// Portal too (PLAN.md redesain-pdf-invoice-kwitansi §D4/§D7).
	if segments[0] == "me" {
		switch {
		case len(segments) == 1 && r.Method == http.MethodGet:
			h.me(w, r)
		case len(segments) == 1 && r.Method == http.MethodPatch:
			h.updateMyProfile(w, r)
		case len(segments) == 2 && segments[1] == "branding" && r.Method == http.MethodGet:
			h.myBranding(w, r)
		case len(segments) == 2 && segments[1] == "logo" && r.Method == http.MethodGet:
			h.myLogo(w, r)
		case len(segments) == 2 && segments[1] == "logo" && r.Method == http.MethodPut:
			h.uploadMyLogo(w, r)
		case len(segments) == 2 && segments[1] == "signature" && r.Method == http.MethodGet:
			h.mySignature(w, r)
		case len(segments) == 2 && segments[1] == "signature" && r.Method == http.MethodPut:
			h.uploadMySignature(w, r)
		default:
			response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
		}
		return
	}

	// Platform Console (tenant CRUD, logo admin upload, suspension toggle,
	// credential reset, manual subscription activation) is cut entirely —
	// D2. Anything other than "me" 404s.
	response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
}

// myBranding is the self-service read powering WO Console/Client Portal
// theming — open to any authenticated tenant-scoped principal (unlike me,
// which stays Owner-only since it also carries subscription/billing data).
func (h *TenantHandler) myBranding(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := selfTenantID(r)
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}
	tenant, err := h.tenants.Get(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toMyBrandingResponse(*tenant))
}

// myLogo streams the caller's own tenant's logo — same auth scoping as
// myBranding above.
func (h *TenantHandler) myLogo(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := selfTenantID(r)
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}
	streamFile(w, r, "logo", tenantID, h.tenants.DownloadLogo)
}

// mySignature streams the caller's own tenant's signature — same auth
// scoping as myLogo above (PLAN.md redesain-pdf-invoice-kwitansi §D4/§D7).
func (h *TenantHandler) mySignature(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := selfTenantID(r)
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}
	streamFile(w, r, "signature", tenantID, h.tenants.DownloadSignature)
}

// hostWithoutPort strips an optional ":port" suffix from r.Host — a custom
// domain is stored/compared as a bare hostname, but a dev/local request's
// Host header often includes a port (e.g. "localhost:8080"). Lowercased to
// match how TenantService.Update normalizes custom_domain before saving it —
// defense-in-depth alongside whatever collation the tenants table happens to
// use, not a substitute for it.
func hostWithoutPort(host string) string {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return strings.ToLower(host)
}

// PublicBranding is the pre-auth counterpart to myBranding (ADR-0015) — it
// resolves the tenant from the request's Host header instead of a JWT claim,
// so LoginPage can render a tenant's own branding before anyone logs in. A
// Host that matches no tenant's custom_domain (e.g. localhost during dev, or
// any Host other than the tenant's own configured custom_domain) 404s via
// writeAppError — the frontend treats that as "no custom branding, use the
// default look".
func (h *TenantHandler) PublicBranding(w http.ResponseWriter, r *http.Request) {
	tenant, err := h.tenants.GetBrandingByDomain(r.Context(), hostWithoutPort(r.Host))
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toBrandingResponse(*tenant))
}

// PublicLogo streams the logo for the tenant matching the request's Host
// header — same Host-based resolution and same "unknown domain" 404 as
// PublicBranding above, unauthenticated like it.
func (h *TenantHandler) PublicLogo(w http.ResponseWriter, r *http.Request) {
	tenant, err := h.tenants.GetBrandingByDomain(r.Context(), hostWithoutPort(r.Host))
	if err != nil {
		writeAppError(w, err)
		return
	}
	streamFile(w, r, "logo", tenant.ID, h.tenants.DownloadLogo)
}

// selfTenantID resolves the calling principal's own tenant from the JWT
// claim — never a request parameter. Any tenant-bound principal (staff of
// any role, or client) qualifies.
func selfTenantID(r *http.Request) (int64, bool) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok {
		return 0, false
	}
	return claims.TenantIDInt()
}

// me is the tenant Owner's self-service read of their own tenant record
// (plan, subscription status/expiry) — scoped from the JWT claim, never a
// request parameter. Powers the WO Console's own "Langganan" page.
func (h *TenantHandler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || claims.Role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengakses data ini", nil)
		return
	}
	tenantID, err := application.ParseTenantID(claims.TenantID)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}
	tenant, err := h.tenants.Get(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toTenantResponse(*tenant))
}

type updateProfileBody struct {
	BusinessName          string `json:"businessName"`
	OwnerName             string `json:"ownerName"`
	Email                 string `json:"email"`
	Phone                 string `json:"phone"`
	City                  string `json:"city"`
	Address               string `json:"address"`
	BankName              string `json:"bankName"`
	BankAccountNumber     string `json:"bankAccountNumber"`
	BankAccountHolderName string `json:"bankAccountHolderName"`
}

// updateMyProfile is the self-service "Profil Usaha" write (PLAN.md
// invoice-kwitansi-client §1.7) — Owner-only, same gate as me().
func (h *TenantHandler) updateMyProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || claims.Role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengubah profil usaha", nil)
		return
	}
	tenantID, err := application.ParseTenantID(claims.TenantID)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}

	var body updateProfileBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	tenant, err := h.tenants.UpdateProfile(r.Context(), tenantID, application.UpdateProfileInput{
		BusinessName: body.BusinessName, OwnerName: body.OwnerName, Email: body.Email, Phone: body.Phone, City: body.City,
		Address: body.Address, BankName: body.BankName, BankAccountNumber: body.BankAccountNumber,
		BankAccountHolderName: body.BankAccountHolderName,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Profil usaha berhasil disimpan", toTenantResponse(*tenant))
}

type logoUploadBody struct {
	FileName   string `json:"fileName"`
	MimeType   string `json:"mimeType"`
	Base64Data string `json:"base64Data"`
}

// uploadMyLogo is the self-service logo upload (PLAN.md
// invoice-kwitansi-client §1.7) — Owner-only, same gate as me()/
// updateMyProfile. TenantService.UploadLogo is genuinely new code, not a
// reuse of any prior admin-upload path (deleted with Platform Console).
func (h *TenantHandler) uploadMyLogo(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || claims.Role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengubah logo usaha", nil)
		return
	}
	tenantID, err := application.ParseTenantID(claims.TenantID)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}

	var body logoUploadBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	tenant, err := h.tenants.UploadLogo(r.Context(), tenantID, application.UploadLogoInput{
		FileName: body.FileName, MimeType: body.MimeType, Base64Data: body.Base64Data,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Logo usaha berhasil diperbarui", toTenantResponse(*tenant))
}

type signatureUploadBody struct {
	FileName   string `json:"fileName"`
	MimeType   string `json:"mimeType"`
	Base64Data string `json:"base64Data"`
}

// uploadMySignature mirrors uploadMyLogo exactly (PLAN.md
// redesain-pdf-invoice-kwitansi §D4/§D7) — Owner-only, same gate.
func (h *TenantHandler) uploadMySignature(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || claims.Role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengubah tanda tangan", nil)
		return
	}
	tenantID, err := application.ParseTenantID(claims.TenantID)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}

	var body signatureUploadBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	tenant, err := h.tenants.UploadSignature(r.Context(), tenantID, application.UploadSignatureInput{
		FileName: body.FileName, MimeType: body.MimeType, Base64Data: body.Base64Data,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tanda tangan berhasil diperbarui", toTenantResponse(*tenant))
}

// streamFile is streamLogo generalized to also serve the signature asset
// (PLAN.md redesain-pdf-invoice-kwitansi §D4/§D7) — download is
// TenantService.DownloadLogo or .DownloadSignature, whichever the caller
// bound; name only affects the Content-Disposition filename.
func streamFile(w http.ResponseWriter, r *http.Request, name string, tenantID int64, download func(ctx context.Context, tenantID int64) (io.ReadCloser, error)) {
	reader, err := download(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

type payBody struct {
	PlanID int64 `json:"planId"`
}

// Pay is the tenant Owner's own self-service subscription payment — scoped to
// their own tenant via the JWT claim, never a request parameter.
func (h *TenantHandler) Pay(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || claims.Role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengelola langganan", nil)
		return
	}
	tenantID, err := application.ParseTenantID(claims.TenantID)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}

	var body payBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	charge, err := h.tenants.Pay(r.Context(), tenantID, body.PlanID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tagihan pembayaran dibuat, selesaikan lewat QRIS di bawah", toChargeResponse(*charge))
}

func (h *TenantHandler) PendingCharge(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || claims.Role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengelola langganan", nil)
		return
	}
	tenantID, err := application.ParseTenantID(claims.TenantID)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}

	charge, err := h.tenants.GetPendingCharge(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if charge == nil {
		response.OK(w, "ok", nil)
		return
	}
	response.OK(w, "ok", toChargeResponse(*charge))
}

func (h *TenantHandler) CancelPendingCharge(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || claims.Role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengelola langganan", nil)
		return
	}
	tenantID, err := application.ParseTenantID(claims.TenantID)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}

	if err := h.tenants.CancelPendingCharge(r.Context(), tenantID); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tagihan pembayaran dibatalkan", nil)
}

func writeAppError(w http.ResponseWriter, err error) {
	status := apperror.HTTPStatus(err)
	if appErr, ok := apperror.As(err); ok {
		if appErr.Kind == apperror.KindValidation {
			response.Error(w, status, appErr.Message, appErr.Fields)
			return
		}
		response.Error(w, status, appErr.Message, nil)
		return
	}
	logger.Error("unhandled error: %v", err)
	response.Error(w, status, "Terjadi kesalahan pada server", nil)
}
