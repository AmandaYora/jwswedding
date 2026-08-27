package presentation

import (
	"encoding/json"
	"net/http"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/response"
)

type clientInvoiceResponse struct {
	ID               int64  `json:"id"`
	InvoiceNumber    string `json:"invoiceNumber"`
	Type             string `json:"type"`
	Description      string `json:"description"`
	Amount           int64  `json:"amount"`
	DueDate          string `json:"dueDate"`
	Status           string `json:"status"`
	ClientPaymentID  int64  `json:"clientPaymentId"`
	CreatedByStaffID int64  `json:"createdByStaffId"`
	CreatedAt        string `json:"createdAt"`
}

func toClientInvoiceResponse(inv domain.ClientInvoice) clientInvoiceResponse {
	return clientInvoiceResponse{
		ID: inv.ID, InvoiceNumber: inv.InvoiceNumber, Type: string(inv.Type), Description: inv.Description,
		Amount: inv.Amount, DueDate: inv.DueDate.Format(dateLayout), Status: string(inv.Status),
		ClientPaymentID: inv.ClientPaymentID, CreatedByStaffID: inv.CreatedByStaffID,
		CreatedAt: inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *Handler) listClientInvoices(w http.ResponseWriter, r *http.Request, projectID int64) {
	list, err := h.clientInvoices.List(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]clientInvoiceResponse, 0, len(list))
	for _, inv := range list {
		result = append(result, toClientInvoiceResponse(inv))
	}
	response.OK(w, "ok", result)
}

type clientInvoiceInputBody struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	DueDate     string `json:"dueDate"`
}

func (h *Handler) createClientInvoice(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	var body clientInvoiceInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	dueDate, err := parseDate(body.DueDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"dueDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	inv, err := h.clientInvoices.Create(r.Context(), claims.tenantID, projectID, claims.staffID, application.ClientInvoiceInput{
		Type: domain.PaymentType(body.Type), Description: body.Description, Amount: body.Amount, DueDate: dueDate,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Tagihan berhasil dibuat", toClientInvoiceResponse(*inv))
}

type updateClientInvoiceBody struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	DueDate     string `json:"dueDate"`
	Status      string `json:"status"`
}

func (h *Handler) updateClientInvoice(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	invoiceID, err := parseInt64(invoiceIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	var body updateClientInvoiceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	dueDate, err := parseDate(body.DueDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"dueDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	inv, err := h.clientInvoices.Update(r.Context(), projectID, invoiceID, claims.staffID, application.ClientInvoiceInput{
		Type: domain.PaymentType(body.Type), Description: body.Description, Amount: body.Amount, DueDate: dueDate,
	}, domain.InvoiceStatus(body.Status))
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tagihan berhasil diperbarui", toClientInvoiceResponse(*inv))
}

// deleteClientInvoice is gated Owner-or-Admin explicitly (not the
// role != "Staff" pattern client_payment_endpoints.go/venue_payment
// endpoints use, which currently also lets "Sales" through -- see PLAN.md
// invoice-kwitansi-client §4.3/§4.7's note on that pre-existing gap).
func (h *Handler) deleteClientInvoice(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner atau Admin yang dapat menghapus Tagihan", nil)
		return
	}
	invoiceID, err := parseInt64(invoiceIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	if err := h.clientInvoices.Delete(r.Context(), projectID, invoiceID, claims.staffID); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tagihan berhasil dihapus", nil)
}

type markInvoicePaidBody struct {
	PaymentDate     string `json:"paymentDate"`
	Method          string `json:"method"`
	ReferenceNumber string `json:"referenceNumber"`
	Notes           string `json:"notes"`
}

func (h *Handler) markClientInvoicePaid(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	invoiceID, err := parseInt64(invoiceIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	var body markInvoicePaidBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	paymentDate, err := parseDate(body.PaymentDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"paymentDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	inv, err := h.clientInvoices.MarkPaid(r.Context(), projectID, invoiceID, claims.staffID, application.ClientPaymentInput{
		PaymentDate: paymentDate, Method: body.Method, ReferenceNumber: body.ReferenceNumber, Notes: body.Notes,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tagihan ditandai Lunas", toClientInvoiceResponse(*inv))
}

func (h *Handler) unmarkClientInvoicePaid(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	invoiceID, err := parseInt64(invoiceIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	inv, err := h.clientInvoices.UnmarkPaid(r.Context(), projectID, invoiceID, claims.staffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Pelunasan Tagihan dibatalkan", toClientInvoiceResponse(*inv))
}

// downloadClientInvoicePDF streams the Invoice PDF -- no role/principal-type
// check beyond resolveProjectAccess (every GET under /projects/{id}/... is
// already open to staff and to a `client` principal reading their own
// project), same as every other read endpoint in this switch. Invoices
// intentionally aren't hidden from Client Portal at the PDF-download level;
// §1.6's restriction is enforced by the frontend never surfacing a link to
// this endpoint there.
func (h *Handler) downloadClientInvoicePDF(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	invoiceID, err := parseInt64(invoiceIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	inv, err := h.clientInvoices.Get(r.Context(), projectID, invoiceID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	project, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	// Gate: reject with 422 before touching logo/signature/totals if the
	// tenant's business profile isn't complete enough to print (PLAN.md
	// redesain-pdf-invoice-kwitansi-v2 §6.2) — the frontend already hides
	// the print action behind IncompleteProfileDialog when it knows the
	// profile is incomplete, but this is the backend's own enforcement of
	// the same rule (K4), reached whenever that frontend state is stale or
	// the endpoint is called directly.
	profile, ok := h.requireCompleteProfile(w, r, claims.tenantID)
	if !ok {
		return
	}
	logo, _, hasLogo, err := h.platform.GetTenantLogo(r.Context(), claims.tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !hasLogo {
		logo = nil
	}
	signature, _, hasSignature, err := h.platform.GetTenantSignature(r.Context(), claims.tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !hasSignature {
		signature = nil
	}
	totalPaid, err := h.clientPayments.TotalReceived(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	pdf, err := buildClientInvoicePDF(*project, *inv, profile, logo, signature, totalPaid)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+inv.InvoiceNumber+`.pdf"`)
	if err := pdf.Output(w); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas PDF", nil)
	}
}
