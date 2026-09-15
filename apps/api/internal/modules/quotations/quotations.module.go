// Package quotations wires the quotations module — Penawaran sebagai PO
// pra-deal (PLAN penawaran-client-master, D9/T2.6): dokumen + komposisi +
// penyesuaian + Template Paket + PDF, pindahan dari projects.
package quotations

import (
	"database/sql"
	"net/http"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/quotations/application"
	"jwswedding/internal/modules/quotations/contracts"
	"jwswedding/internal/modules/quotations/infrastructure"
	"jwswedding/internal/modules/quotations/presentation"
	vendorscontracts "jwswedding/internal/modules/vendors/contracts"
	"jwswedding/internal/shared/storage"
)

type Module struct {
	handler                *presentation.Handler
	publicHandler          *presentation.PublicHandler
	packageTemplateHandler *presentation.PackageTemplateHandler
	contracts              contracts.Contracts
	quotationService       *application.QuotationService
}

func NewModule(db *sql.DB, platform platformcontracts.Contracts, projects projectscontracts.Contracts, vendors vendorscontracts.Contracts, storageClient *storage.Client) *Module {
	quotationRepo := infrastructure.NewMySQLQuotationRepository(db)
	templateRepo := infrastructure.NewMySQLPackageTemplateRepository(db)
	templateService := application.NewPackageTemplateService(templateRepo)
	quotationService := application.NewQuotationService(quotationRepo, templateRepo, projects, storageClient)
	linkRepo := infrastructure.NewMySQLSignatureLinkRepository(db)
	linkService := application.NewSignatureLinkService(linkRepo, quotationService)
	// vendors.Contracts satisfies application's VenueResolver structurally
	// (same method, same DTO) — no adapter needed, unlike staffNameResolver.
	quotationService.SetVenueResolver(vendors)
	return &Module{
		handler:                presentation.NewHandler(quotationService, platform, projects, linkService),
		publicHandler:          presentation.NewPublicHandler(linkService),
		packageTemplateHandler: presentation.NewPackageTemplateHandler(templateService),
		contracts:              contracts.New(quotationService),
		quotationService:       quotationService,
	}
}

func (m *Module) Contracts() contracts.Contracts {
	return m.contracts
}

// SetClientDirectory completes the two-phase wiring (see
// application.ClientDirectory): clients is built after quotations (it needs
// quotations.Contracts for its own cascade), so this can't be a constructor
// argument.
func (m *Module) SetClientDirectory(directory application.ClientDirectory) {
	m.quotationService.SetClientDirectory(directory)
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/quotations", authed(http.HandlerFunc(m.handler.Collection)))
	mux.Handle("/api/v1/quotations/", authed(http.HandlerFunc(m.handler.Item)))
	// Template Paket keeps its URLs (T2.4) — only the serving module moves.
	mux.Handle("/api/v1/package-templates", authed(http.HandlerFunc(m.packageTemplateHandler.Collection)))
	mux.Handle("/api/v1/package-templates/", authed(http.HandlerFunc(m.packageTemplateHandler.Item)))
}

// RegisterPublicRoutes mendaftarkan endpoint magic link tanda tangan TANPA
// auth (jalur C) — berpola platform.module.go:104. wrap adalah middleware
// pemanggil (rate limit, T5); main.go yang memasoknya.
func (m *Module) RegisterPublicRoutes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/public/quotation-signature/",
		wrap(http.HandlerFunc(m.publicHandler.Collection)))
}
