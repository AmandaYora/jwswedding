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
	packageTemplateHandler   *presentation.PackageTemplateHandler
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
	packageTemplateRepo := infrastructure.NewMySQLPackageTemplateRepository(db)
	packageOrderRepo := infrastructure.NewMySQLPackageOrderRepository(db)

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
	packageTemplateService := application.NewPackageTemplateService(packageTemplateRepo)
	// packageOrderService takes projectRepo directly (as its narrow
	// PackageOrderProjectStore) rather than projectService: the only project
	// state it writes is contract_value and package_name, and going through
	// ProjectService.Update would drag in RBAC and konteks-umum validation
	// meant for a user editing the form, not for a derived recompute.
	packageOrderService := application.NewPackageOrderService(packageOrderRepo, packageTemplateRepo, projectRepo, clientInvoiceService, activityService)

	handler := presentation.NewHandler(projectService, vendorEngagementService, paymentService, clientPaymentService, clientInvoiceService, venuePaymentService, issueService, evidenceService, activityService, dashboardService, platform, packageOrderService)

	return &Module{
		handler:                  handler,
		milestoneTemplateHandler: presentation.NewMilestoneTemplateHandler(milestoneTemplateService),
		packageTemplateHandler:   presentation.NewPackageTemplateHandler(packageTemplateService),
		contracts:                contracts.New(projectService, vendorEngagementService, milestoneTemplateService),
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

// SetClientContactResolver is the twin of SetClientAccessResolver, for the
// phone number on the PO Paket header (PLAN.md po-paket-client, blok B1).
// main.go passes the same clients.Contracts object into both.
func (m *Module) SetClientContactResolver(resolver presentation.ClientContactResolver) {
	m.handler.SetClientContactResolver(resolver)
}

// SetClientCleaner completes the same two-phase wiring as
// SetClientAccessResolver above, for ProjectService.Delete's (ADR-0013)
// best-effort client cleanup — see application.ClientCleaner's doc comment.
func (m *Module) SetClientCleaner(cleaner application.ClientCleaner) {
	m.projectService.SetClientCleaner(cleaner)
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
	mux.Handle("/api/v1/package-templates", authed(http.HandlerFunc(m.packageTemplateHandler.Collection)))
	mux.Handle("/api/v1/package-templates/", authed(http.HandlerFunc(m.packageTemplateHandler.Item)))
}
