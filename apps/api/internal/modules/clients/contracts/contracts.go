// Package contracts is the ONLY package other modules may import from
// clients. It exists solely for Fase 6's Client Portal scoping: the
// `projects` module needs to know which single project a `client` principal
// is allowed to read, without importing clients' domain/application/
// infrastructure internals.
package contracts

import (
	"context"

	"jwswedding/internal/modules/clients/application"
	"jwswedding/internal/shared/apperror"
)

type Contracts interface {
	// ProjectIDForClient resolves the one project a client principal may
	// read. Returns a NotFound apperror if the client row doesn't exist
	// (or belongs to a different tenant) — callers should treat any error
	// here as "deny access", never as "allow".
	ProjectIDForClient(ctx context.Context, tenantID, clientID int64) (int64, error)
	// DeleteAllForProject best-effort deletes every client tied to a project
	// — called by `projects` (ADR-0013's hard delete) via the ClientCleaner
	// bridge, never awaited to block the project delete itself.
	DeleteAllForProject(ctx context.Context, tenantID, projectID int64) error
	// PhoneForProject resolves the phone number shown in the PO Paket's header
	// box (PLAN.md po-paket-client, blok B1). Returns "" — never an error —
	// when the project has no client yet or none of them recorded a number, so
	// a PO stays printable during the window before client accounts exist.
	//
	// Deliberately phone ONLY: the name on that header comes from the
	// project's own BrideName/GroomName, not from this module (PLAN.md §1.6
	// F7), so widening this to return a name would create a second, competing
	// source for a field that already has one.
	PhoneForProject(ctx context.Context, tenantID, projectID int64) (string, error)
}

type impl struct {
	clients *application.ClientService
}

func New(clients *application.ClientService) Contracts {
	return &impl{clients: clients}
}

func (c *impl) ProjectIDForClient(ctx context.Context, tenantID, clientID int64) (int64, error) {
	client, err := c.clients.Get(ctx, tenantID, clientID)
	if err != nil {
		return 0, err
	}
	if !client.IsActive {
		return 0, apperror.Forbidden("Akun client ini sudah dinonaktifkan")
	}
	return client.ProjectID, nil
}

func (c *impl) DeleteAllForProject(ctx context.Context, tenantID, projectID int64) error {
	return c.clients.DeleteAllForProject(ctx, tenantID, projectID)
}

// PhoneForProject takes the first client on the project that actually recorded
// a number. A project usually has one client account, and when it has several
// (bride's side, groom's side) any of their numbers is a valid contact for the
// document — so this prefers "a real number" over "the first row", which could
// otherwise be a representative who left the field blank.
func (c *impl) PhoneForProject(ctx context.Context, tenantID, projectID int64) (string, error) {
	list, err := c.clients.ListByProject(ctx, tenantID, projectID)
	if err != nil {
		return "", err
	}
	for _, cl := range list {
		if cl.Phone != "" {
			return cl.Phone, nil
		}
	}
	return "", nil
}
