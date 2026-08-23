// Package contracts is the ONLY package other modules (or composition-root
// code like shared/middleware's injected predicate) may import from
// `platform` — this is platform's first contracts package, needed only once
// the read-only subscription guard (D5/D10/D13) had to call into platform's
// application layer from outside the module.
package contracts

import (
	"context"

	"jwswedding/internal/modules/platform/application"
)

// Contracts exposes the single read used by the subscription guard —
// WritesAllowed evaluates subscription_expires_at directly (D10), never
// subscription_status (T3: no code path ever writes StatusExpired/
// StatusExpiringSoon, so a status-based check would never trip).
type Contracts interface {
	WritesAllowed(ctx context.Context, tenantID int64) (bool, error)
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
