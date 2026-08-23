// Package staff wires the staff module together: WO Console's own tenant-
// scoped user management (Fase 3), plus CreateOwner (via contracts, called by
// `platform`'s tenant-registration orchestration, Fase 2).
package staff

import (
	"database/sql"
	"net/http"

	identitycontracts "jwswedding/internal/modules/identity/contracts"

	"jwswedding/internal/modules/staff/application"
	"jwswedding/internal/modules/staff/contracts"
	"jwswedding/internal/modules/staff/infrastructure"
	"jwswedding/internal/modules/staff/presentation"
)

type Module struct {
	contracts contracts.Contracts
	handler   *presentation.Handler
	service   *application.StaffService
}

func NewModule(db *sql.DB, identity identitycontracts.Contracts) *Module {
	repo := infrastructure.NewMySQLStaffRepository(db)
	service := application.NewStaffService(repo, identity)
	return &Module{
		contracts: contracts.New(service),
		handler:   presentation.NewHandler(service),
		service:   service,
	}
}

func (m *Module) Contracts() contracts.Contracts {
	return m.contracts
}

// SetProjectReferenceLookup completes the two-phase wiring described on
// application.ProjectReferenceLookup — main.go calls this right after
// projectsModule is built (staff is built first, so this can't be a
// constructor argument).
func (m *Module) SetProjectReferenceLookup(lookup application.ProjectReferenceLookup) {
	m.service.SetProjectReferenceLookup(lookup)
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/staff", authed(http.HandlerFunc(m.handler.Collection)))
	mux.Handle("/api/v1/staff/summary", authed(http.HandlerFunc(m.handler.Summary)))
	mux.Handle("/api/v1/staff/", authed(http.HandlerFunc(m.handler.Item)))
}
