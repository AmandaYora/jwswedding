// Package billing wires the billing module: the subscription plan catalog
// (read-only, sourced from ElProof — D7) and the subscription transaction
// ledger. See ADR-0008, docs/API_CONTRACT.md.
package billing

import (
	"database/sql"
	"net/http"

	"jwswedding/internal/modules/billing/application"
	"jwswedding/internal/modules/billing/contracts"
	"jwswedding/internal/modules/billing/infrastructure"
	"jwswedding/internal/modules/billing/presentation"
	"jwswedding/internal/shared/elproofpay"
	"jwswedding/internal/shared/httpx"
)

type Module struct {
	planHandler        *presentation.PlanHandler
	transactionHandler *presentation.TransactionHandler
	contracts          contracts.Contracts
}

func NewModule(db *sql.DB, elproofClient *elproofpay.Client) *Module {
	planRepo := infrastructure.NewElProofPlanSource(elproofClient)
	transactionRepo := infrastructure.NewMySQLTransactionRepository(db)

	planService := application.NewPlanService(planRepo)
	transactionService := application.NewTransactionService(transactionRepo)

	return &Module{
		planHandler:        presentation.NewPlanHandler(planService),
		transactionHandler: presentation.NewTransactionHandler(transactionService),
		contracts:          contracts.New(planService, transactionService),
	}
}

func (m *Module) Contracts() contracts.Contracts {
	return m.contracts
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/plans", authed(http.HandlerFunc(m.planHandler.Collection)))
	mux.Handle("/api/v1/subscription-transactions", authed(httpx.Method(http.MethodGet, m.transactionHandler.List)))
}
