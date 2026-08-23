package presentation

import (
	"encoding/json"
	"net/http"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/response"
)

// listClientPayments computes each payment's evidenceComplete the same way
// the response DTO needs it (§3.4 of PLAN.md's Client Payments design):
// client_payments has no direct evidence-ID column (unlike vendor_payments),
// so completeness is a simple existence check against the polymorphic
// evidence table instead of a struct method.
func (h *Handler) listClientPayments(w http.ResponseWriter, r *http.Request, projectID int64) {
	list, err := h.clientPayments.List(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	evidences, err := h.evidence.List(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	hasEvidence := make(map[int64]bool, len(evidences))
	for _, e := range evidences {
		if e.RelatedKind == domain.RelatedClientPayment {
			hasEvidence[e.RelatedID] = true
		}
	}
	result := make([]clientPaymentResponse, 0, len(list))
	for _, p := range list {
		result = append(result, toClientPaymentResponse(p, hasEvidence[p.ID]))
	}
	response.OK(w, "ok", result)
}

type clientPaymentInputBody struct {
	Type            string `json:"type"`
	Amount          int64  `json:"amount"`
	PaymentDate     string `json:"paymentDate"`
	Method          string `json:"method"`
	ReferenceNumber string `json:"referenceNumber"`
	Notes           string `json:"notes"`
}

func (h *Handler) createClientPayment(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	var body clientPaymentInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	paymentDate, err := parseDate(body.PaymentDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"paymentDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	p, err := h.clientPayments.Create(r.Context(), projectID, claims.staffID, application.ClientPaymentInput{
		Type: domain.PaymentType(body.Type), Amount: body.Amount,
		PaymentDate: paymentDate, Method: body.Method, ReferenceNumber: body.ReferenceNumber, Notes: body.Notes,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Pembayaran client berhasil dicatat", toClientPaymentResponse(*p, false))
}

// updateClientPayment is a full-field overwrite, no role-gate — see
// updatePayment's doc comment. Guarded against changing type/amount on a
// payment that a ClientInvoice's MarkPaid produced (PLAN.md
// invoice-kwitansi-client §1.9/§4.3) — compared against the Invoice's own
// Type/Amount (not the payment's current values), which is valid because
// MarkPaid already forces them equal at creation time.
func (h *Handler) updateClientPayment(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, paymentIDRaw string) {
	paymentID, err := parseInt64(paymentIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	var body clientPaymentInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	paymentDate, err := parseDate(body.PaymentDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"paymentDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	inv, err := h.clientInvoices.FindByPaymentID(r.Context(), projectID, paymentID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if inv != nil && (body.Type != string(inv.Type) || body.Amount != inv.Amount) {
		response.Error(w, http.StatusUnprocessableEntity,
			"Pembayaran ini berasal dari Tagihan "+inv.InvoiceNumber+". Jenis dan jumlahnya mengikuti Tagihan, gunakan \"Batalkan Pelunasan\" pada tagihan tersebut untuk mengubahnya.", nil)
		return
	}
	p, err := h.clientPayments.Update(r.Context(), projectID, paymentID, claims.staffID, application.ClientPaymentInput{
		Type: domain.PaymentType(body.Type), Amount: body.Amount,
		PaymentDate: paymentDate, Method: body.Method, ReferenceNumber: body.ReferenceNumber, Notes: body.Notes,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	evidences, err := h.evidence.List(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	hasEvidence := make(map[int64]bool, len(evidences))
	for _, e := range evidences {
		if e.RelatedKind == domain.RelatedClientPayment {
			hasEvidence[e.RelatedID] = true
		}
	}
	response.OK(w, "Pembayaran client berhasil diperbarui", toClientPaymentResponse(*p, hasEvidence[p.ID]))
}

// deleteClientPayment hard-deletes the payment (and its own evidence).
// Owner-or-Admin — see deletePayment's doc comment. Guarded against deleting
// a payment a ClientInvoice's MarkPaid produced (PLAN.md
// invoice-kwitansi-client §1.9/§4.3) — without this, "Batalkan Pelunasan"
// wouldn't be the only way to unwind a mark-paid, and the Invoice would be
// left dangling at status Lunas pointing at a deleted payment.
func (h *Handler) deleteClientPayment(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, paymentIDRaw string) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner atau Admin yang dapat menghapus pembayaran secara permanen", nil)
		return
	}
	paymentID, err := parseInt64(paymentIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	inv, err := h.clientInvoices.FindByPaymentID(r.Context(), projectID, paymentID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if inv != nil {
		response.Error(w, http.StatusUnprocessableEntity,
			"Pembayaran ini berasal dari Tagihan "+inv.InvoiceNumber+". Gunakan tombol \"Batalkan Pelunasan\" pada tagihan tersebut.", nil)
		return
	}
	if err := h.clientPayments.Delete(r.Context(), projectID, paymentID, claims.staffID); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Pembayaran client berhasil dihapus", nil)
}

// downloadClientPaymentReceiptPDF streams the Kwitansi PDF, lazily assigning
// its official number on first print (EnsureReceiptNumber) — reachable by
// staff and by a `client` principal reading their own project, same as
// downloadClientInvoicePDF.
func (h *Handler) downloadClientPaymentReceiptPDF(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, paymentIDRaw string) {
	paymentID, err := parseInt64(paymentIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	p, err := h.clientPayments.EnsureReceiptNumber(r.Context(), claims.tenantID, projectID, paymentID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	project, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	inv, err := h.clientInvoices.FindByPaymentID(r.Context(), projectID, paymentID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	invoiceNumber := ""
	if inv != nil {
		invoiceNumber = inv.InvoiceNumber
	}
	profile, err := h.platform.GetTenantProfile(r.Context(), claims.tenantID)
	if err != nil {
		writeAppError(w, err)
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

	pdf, err := buildClientPaymentReceiptPDF(*project, *p, invoiceNumber, profile, logo, signature)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+p.ReceiptNumber+`.pdf"`)
	if err := pdf.Output(w); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas PDF", nil)
	}
}
