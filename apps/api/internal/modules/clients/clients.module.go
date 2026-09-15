// Package clients wires the clients module — master pasangan (Client) +
// kontak/akun portal (ClientContact), tenant-scoped (PLAN
// penawaran-client-master, Fase 1).
package clients

import (
	"database/sql"
	"net/http"

	"jwswedding/internal/modules/clients/application"
	"jwswedding/internal/modules/clients/contracts"
	"jwswedding/internal/modules/clients/infrastructure"
	"jwswedding/internal/modules/clients/presentation"
	identitycontracts "jwswedding/internal/modules/identity/contracts"
	projectscontracts "jwswedding/internal/modules/projects/contracts"
	quotationscontracts "jwswedding/internal/modules/quotations/contracts"
	"jwswedding/internal/shared/storage"
)

type Module struct {
	handler   *presentation.Handler
	contracts contracts.Contracts
}

func NewModule(db *sql.DB, projects projectscontracts.Contracts, quotations quotationscontracts.Contracts, identity identitycontracts.Contracts, storageClient *storage.Client) *Module {
	repo := infrastructure.NewMySQLClientRepository(db)
	contactRepo := infrastructure.NewMySQLClientContactRepository(db)
	contacts := application.NewClientContactService(contactRepo, repo, identity)
	service := application.NewClientService(repo, contacts, projects, quotations, identity)
	contacts.SetSyncer(service)
	signatureRepo := infrastructure.NewMySQLClientSignatureRepository(db)
	signatures := application.NewClientSignatureService(signatureRepo, repo, storageClient)
	return &Module{handler: presentation.NewHandler(service, contacts, signatures), contracts: contracts.New(service, signatures)}
}

func (m *Module) Contracts() contracts.Contracts {
	return m.contracts
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/clients", authed(http.HandlerFunc(m.handler.Collection)))
	mux.Handle("/api/v1/clients/", authed(http.HandlerFunc(m.handler.Item)))
}
