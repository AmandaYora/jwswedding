package presentation

import (
	"net/http"
	"strconv"

	"jwswedding/internal/modules/billing/application"
	"jwswedding/internal/modules/billing/domain"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/response"
)

type TransactionHandler struct {
	transactions *application.TransactionService
}

func NewTransactionHandler(transactions *application.TransactionService) *TransactionHandler {
	return &TransactionHandler{transactions: transactions}
}

type transactionResponse struct {
	ID               int64   `json:"id"`
	TenantID         int64   `json:"tenantId"`
	Type             string  `json:"type"`
	Amount           int64   `json:"amount"`
	PaymentMethod    string  `json:"paymentMethod"`
	PaymentReference string  `json:"paymentReference"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"createdAt"`
	PaidAt           *string `json:"paidAt"`
}

func toTransactionResponse(t domain.Transaction) transactionResponse {
	var paidAt *string
	if t.PaidAt != nil {
		s := t.PaidAt.Format("2006-01-02T15:04:05Z07:00")
		paidAt = &s
	}
	return transactionResponse{
		ID: t.ID, TenantID: t.TenantID, Type: string(t.Type), Amount: t.Amount,
		PaymentMethod: t.PaymentMethod, PaymentReference: t.PaymentReference, Status: string(t.Status),
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), PaidAt: paidAt,
	}
}

// List is Owner-only (billing/transaction history is part of the
// "Langganan" menu, which the confirmed role rule locks to Owner in full,
// not just the write actions already gated elsewhere in
// `platform/tenant_handler.go`) and always scoped to the caller's own
// tenant from the JWT claim — no platform_admin branch anymore (D2), so a
// `?tenantId=` query param is never honored.
func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Tidak terautentikasi", nil)
		return
	}
	if claims.PrincipalType == "staff" && !claims.HasRole("Owner") {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat mengakses riwayat transaksi langganan", nil)
		return
	}

	id, err := strconv.ParseInt(claims.TenantID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}
	tenantID := &id

	if r.URL.Query().Get("all") == "true" {
		transactions, err := h.transactions.List(r.Context(), tenantID)
		if err != nil {
			writeAppError(w, err)
			return
		}
		result := make([]transactionResponse, 0, len(transactions))
		for _, t := range transactions {
			result = append(result, toTransactionResponse(t))
		}
		response.OK(w, "ok", result)
		return
	}

	params := pagination.FromRequest(r)
	status := r.URL.Query().Get("status")
	transactions, total, err := h.transactions.ListPaginated(r.Context(), tenantID, params, status)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]transactionResponse, 0, len(transactions))
	for _, t := range transactions {
		result = append(result, toTransactionResponse(t))
	}
	response.OKPaginated(w, "ok", result, pagination.BuildMeta(params, total))
}
