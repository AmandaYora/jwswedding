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
	"jwswedding/internal/shared/apperror"
)

// TenantProfile is the cross-module-safe projection of domain.Tenant needed
// to render an Invoice/Kwitansi PDF kop surat (PLAN.md invoice-kwitansi-client)
// — deliberately a flat struct of primitives, never domain.Tenant itself, so
// `projects` never sees platform's domain internals.
type TenantProfile struct {
	BusinessName, OwnerName, Phone, Email, Address     string
	BankName, BankAccountNumber, BankAccountHolderName string
}

// Contracts exposes what other modules need from `platform` without ever
// importing its application/infrastructure/domain internals —
// WritesAllowed backs the subscription guard (D5/D10/D13, evaluates
// subscription_expires_at directly, never subscription_status since no code
// path ever writes StatusExpired/StatusExpiringSoon — T3); GetTenantProfile/
// GetTenantLogo back `projects`' Invoice/Kwitansi PDF kop surat.
type Contracts interface {
	WritesAllowed(ctx context.Context, tenantID int64) (bool, error)
	GetTenantProfile(ctx context.Context, tenantID int64) (TenantProfile, error)
	GetTenantLogo(ctx context.Context, tenantID int64) (data []byte, contentType string, ok bool, err error)
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
	return TenantProfile{
		BusinessName: tenant.BusinessName, OwnerName: tenant.OwnerName, Phone: tenant.Phone, Email: tenant.Email,
		Address: tenant.Address, BankName: tenant.BankName, BankAccountNumber: tenant.BankAccountNumber,
		BankAccountHolderName: tenant.BankAccountHolderName,
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
