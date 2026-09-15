// Package projects wires the largest module in the system: the project
// lifecycle and its 7 sub-entities (milestones, vendor engagements, vendor
// milestones, payments, issues, evidence, activity log) — see MODULE_MAP.md.
package projects

import (
	"database/sql"
	"net/http"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/projects/infrastructure"
	"jwswedding/internal/modules/projects/presentation"
	"jwswedding/internal/shared/storage"
)

type Module struct {
	handler                  *presentation.Handler
	milestoneTemplateHandler *presentation.MilestoneTemplateHandler
	contracts                contracts.Contracts
	projectService           *application.ProjectService
}

func NewModule(db *sql.DB, storageClient *storage.Client, staff application.StaffNameResolver, platform platformcontracts.Contracts) *Module {
	projectRepo := infrastructure.NewMySQLProjectRepository(db)
	milestoneRepo := infrastructure.NewMySQLMilestoneRepository(db)
	milestoneTemplateRepo := infrastructure.NewMySQLMilestoneTemplateRepository(db)
	vendorEngagementRepo := infrastructure.NewMySQLVendorEngagementRepository(db)
	vendorMilestoneRepo := infrastructure.NewMySQLVendorMilestoneRepository(db)
	paymentRepo := infrastructure.NewMySQLPaymentRepository(db)
	clientPaymentRepo := infrastructure.NewMySQLClientPaymentRepository(db)
	clientInvoiceRepo := infrastructure.NewMySQLClientInvoiceRepository(db)
	venuePaymentRepo := infrastructure.NewMySQLVenuePaymentRepository(db)
	issueRepo := infrastructure.NewMySQLIssueRepository(db)
	evidenceRepo := infrastructure.NewMySQLEvidenceRepository(db)
	activityRepo := infrastructure.NewMySQLActivityRepository(db)
	dashboardRepo := infrastructure.NewMySQLDashboardRepository(db)

	activityService := application.NewActivityService(activityRepo)
	evidenceService := application.NewEvidenceService(evidenceRepo, storageClient, storage.BuildKey, activityService)
	projectService := application.NewProjectService(projectRepo, milestoneRepo, milestoneTemplateRepo, vendorEngagementRepo, vendorMilestoneRepo, issueRepo, paymentRepo, venuePaymentRepo, evidenceService, activityService, staff)
	milestoneTemplateService := application.NewMilestoneTemplateService(milestoneTemplateRepo)
	vendorEngagementService := application.NewVendorEngagementService(vendorEngagementRepo, vendorMilestoneRepo, activityService)
	paymentService := application.NewPaymentService(paymentRepo, evidenceService, activityService)
	clientPaymentService := application.NewClientPaymentService(clientPaymentRepo, evidenceService, activityService)
	clientInvoiceService := application.NewClientInvoiceService(clientInvoiceRepo, clientPaymentService, activityService)
	venuePaymentService := application.NewVenuePaymentService(venuePaymentRepo, evidenceService, activityService)
	issueService := application.NewIssueService(issueRepo, vendorMilestoneRepo, activityService)
	dashboardService := application.NewDashboardService(projectService, dashboardRepo, evidenceService)
	// Ledger tagihan untuk SyncContractValue + ImpactForClient (T3.3, T3.5) —
	// setter supaya konstruktor ProjectService tidak berubah.
	projectService.SetClientInvoiceService(clientInvoiceService)
	// Gerbang komitmen biaya vendor membaca anggaran dari ProjectService —
	// satu modul, jadi ini perakitan biasa, bukan lintas batas. Setter karena
	// keduanya saling membutuhkan pada waktu yang berbeda.
	vendorEngagementService.SetBudgetReader(projectService)

	handler := presentation.NewHandler(projectService, vendorEngagementService, paymentService, clientPaymentService, clientInvoiceService, venuePaymentService, issueService, evidenceService, activityService, dashboardService, platform)

	return &Module{
		handler:                  handler,
		milestoneTemplateHandler: presentation.NewMilestoneTemplateHandler(milestoneTemplateService),
		contracts:                contracts.New(projectService, vendorEngagementService, milestoneTemplateService, clientPaymentService),
		projectService:           projectService,
	}
}

func (m *Module) Contracts() contracts.Contracts {
	return m.contracts
}

// SetClientAccessResolver completes Fase 6's client-portal scoping. It must
// be called from main.go after clients.NewModule exists — see the
// presentation.ClientAccessResolver doc comment for why this can't just be
// a constructor parameter (clients.NewModule itself needs projects.Contracts()
// already built, so the two modules can't be constructed in either order
// with a direct constructor dependency both ways).
func (m *Module) SetClientAccessResolver(resolver presentation.ClientAccessResolver) {
	m.handler.SetClientAccessResolver(resolver)
}

// SetClientActivator memasok port aktivasi akun portal (D4) — dipanggil dari
// main.go setelah clientsModule ada.
func (m *Module) SetClientActivator(activator application.ClientActivator) {
	m.projectService.SetClientActivator(activator)
}

// SetQuotationResolver memasok port penawaran (T2.8, T3.5) — dipanggil dari
// main.go setelah quotationsModule ada.
func (m *Module) SetQuotationResolver(resolver application.QuotationResolver) {
	m.projectService.SetQuotationResolver(resolver)
}

// SetVenueResolver completes the same two-phase wiring, for resolving a
// project's attached venue_id into display data (ADR-0016) — see
// application.VenueResolver's doc comment.
func (m *Module) SetVenueResolver(resolver application.VenueResolver) {
	m.projectService.SetVenueResolver(resolver)
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/projects", authed(http.HandlerFunc(m.handler.Collection)))
	mux.Handle("/api/v1/projects/", authed(http.HandlerFunc(m.handler.Item)))
	mux.Handle("/api/v1/dashboard", authed(http.HandlerFunc(m.handler.Dashboard)))
	mux.Handle("/api/v1/client-timelines", authed(http.HandlerFunc(m.handler.ClientTimelines)))
	mux.Handle("/api/v1/milestone-templates", authed(http.HandlerFunc(m.milestoneTemplateHandler.Collection)))
	mux.Handle("/api/v1/milestone-templates/", authed(http.HandlerFunc(m.milestoneTemplateHandler.Item)))
}
