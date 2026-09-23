// Package contracts is the ONLY package other modules (or composition-root
// code like shared/middleware's injected predicate) may import from
// `platform` — this is platform's first contracts package, needed only once
// the read-only subscription guard (D5/D10/D13) had to call into platform's
// application layer from outside the module.
package contracts

import (
	"context"
	"io"

	"jwswedding/internal/modules/platform/application"
	"jwswedding/internal/modules/platform/domain"
	"jwswedding/internal/shared/apperror"
)

// TenantProfile is the cross-module-safe projection of domain.Tenant needed
// to render an Invoice/Kwitansi PDF (PLAN.md invoice-kwitansi-client, then
// redesain-pdf-invoice-kwitansi) — deliberately a flat struct of primitives,
// never domain.Tenant itself, so `projects` never sees platform's domain
// internals. AccentRGB/AccentDarkRGB/AccentSoftRGB are the tenant's
// brand_color_preset already resolved to RGB by domain.PresetRGB (§D8) —
// `projects` renders these directly and never needs to know what a "preset"
// is or that 21 of them exist.
type TenantProfile struct {
	BusinessName, OwnerName, Phone, Email, Address, City string
	BankName, BankAccountNumber, BankAccountHolderName   string
	AccentRGB, AccentDarkRGB, AccentSoftRGB              [3]int
}

// Contracts exposes what other modules need from `platform` without ever
// importing its application/infrastructure/domain internals —
// WritesAllowed backs the subscription guard (D5/D10/D13, evaluates
// subscription_expires_at directly, never subscription_status since no code
// path ever writes StatusExpired/StatusExpiringSoon — T3); GetTenantProfile/
// GetTenantLogo back `projects`' Invoice/Kwitansi PDF. Tanda tangannya TIDAK
// di sini: sejak PLAN tanda-tangan-pengguna, TTD adalah master data per
// pengguna dan diselesaikan lewat staff/contracts.GetSigner.
// ProfileMissingFields backs the PDF-download gate (PLAN.md
// redesain-pdf-invoice-kwitansi-v2 §6.2) — this is the ONLY way `projects`
// may learn whether a tenant's business profile is complete; it must never
// import platform/domain directly (modular-monolith boundary).
type Contracts interface {
	WritesAllowed(ctx context.Context, tenantID int64) (bool, error)
	GetTenantProfile(ctx context.Context, tenantID int64) (TenantProfile, error)
	GetTenantLogo(ctx context.Context, tenantID int64) (data []byte, contentType string, ok bool, err error)
	ProfileMissingFields(ctx context.Context, tenantID int64) ([]string, error)
}

type impl struct {
	tenants *application.TenantService
}

func New(tenants *application.TenantService) Contracts {
	return &impl{tenants: tenants}
}

func (c *impl) WritesAllowed(ctx context.Context, tenantID int64) (bool, error) {
	return c.tenants.WritesAllowed(ctx, tenantID)
}

func (c *impl) GetTenantProfile(ctx context.Context, tenantID int64) (TenantProfile, error) {
	tenant, err := c.tenants.Get(ctx, tenantID)
	if err != nil {
		return TenantProfile{}, err
	}
	accent, accentDark, accentSoft := domain.PresetRGB(tenant.BrandColorPreset)
	return TenantProfile{
		BusinessName: tenant.BusinessName, OwnerName: tenant.OwnerName, Phone: tenant.Phone, Email: tenant.Email,
		Address: tenant.Address, City: tenant.City, BankName: tenant.BankName, BankAccountNumber: tenant.BankAccountNumber,
		BankAccountHolderName: tenant.BankAccountHolderName,
		AccentRGB:             accent, AccentDarkRGB: accentDark, AccentSoftRGB: accentSoft,
	}, nil
}

// GetTenantLogo returns ok=false (never an error) when the tenant simply has
// no logo yet, so a PDF builder can render without one instead of failing —
// DownloadLogo itself distinguishes "no logo" via apperror.NotFound, this
// wrapper translates that one case into ok=false and lets every other error
// (a real storage failure) still propagate as err.
func (c *impl) GetTenantLogo(ctx context.Context, tenantID int64) (data []byte, contentType string, ok bool, err error) {
	reader, dlErr := c.tenants.DownloadLogo(ctx, tenantID)
	if dlErr != nil {
		if appErr, isApp := apperror.As(dlErr); isApp && appErr.Kind == apperror.KindNotFound {
			return nil, "", false, nil
		}
		return nil, "", false, dlErr
	}
	defer reader.Close()

	data, err = io.ReadAll(reader)
	if err != nil {
		return nil, "", false, err
	}
	return data, "", true, nil
}

func (c *impl) ProfileMissingFields(ctx context.Context, tenantID int64) ([]string, error) {
	return c.tenants.ProfileMissingFields(ctx, tenantID)
}
