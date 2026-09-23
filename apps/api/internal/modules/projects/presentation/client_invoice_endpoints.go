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

// listClientInvoices hides Draft rows from a `client` principal (PLAN.md
// po-paket-client D9). Draft is now two things at once: a bill the WO has not
// sent yet, and — since issuing a PO Paket seeds the whole payment schedule as
// Draft rows (D8) — the forward plan itself. Showing those to the client would
// present four or five instalments as if every one were already due.
//
// The filter lives here rather than in the service because only this layer
// knows which principal is asking; the WO Console must keep seeing Draft.
func (h *Handler) listClientInvoices(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	list, err := h.clientInvoices.List(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	visible := visibleInvoicesFor(claims.principalType, list)
	result := make([]clientInvoiceResponse, 0, len(visible))
	for _, inv := range visible {
		result = append(result, toClientInvoiceResponse(inv))
	}
	response.OK(w, "ok", result)
}

// visibleInvoicesFor applies D9. Extracted as a pure function because the rule
// it encodes is short but its failure mode is not: getting it wrong shows a
// client every unsent instalment as an apparently-due bill.
//
// Anything that is not a `client` principal (staff of every role) sees the
// full list unchanged — the WO Console depends on Draft rows being visible.
func visibleInvoicesFor(principalType string, list []domain.ClientInvoice) []domain.ClientInvoice {
	if principalType != "client" {
		return list
	}
	visible := make([]domain.ClientInvoice, 0, len(list))
	for _, inv := range list {
		if inv.Status == domain.InvoiceDraft {
			continue
		}
		visible = append(visible, inv)
	}
	return visible
}

type clientInvoiceInputBody struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	DueDate     string `json:"dueDate"`
}

// createClientInvoice is Owner-or-Admin only -- confirmed role rule, PLAN.md
// mom-25082026-item-sebagian §3b, same bar deleteClientInvoice already uses.
func (h *Handler) createClientInvoice(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner atau Admin yang dapat membuat Tagihan", nil)
		return
	}
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
	// Penawaran yang sedang direvisi belum dikirim ulang, jadi nilai kontrak
	// project saat ini BELUM disepakati klien — menerbitkan Tagihan di atasnya
	// berarti menagih angka yang belum ia setujui. Ditahan sampai revisinya
	// dikirim; itu satu klik di menu Penawaran, bukan jalan buntu.
	project, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if h.projects.QuotationUnderRevision(r.Context(), claims.tenantID, project.QuotationID) {
		response.Error(w, http.StatusUnprocessableEntity,
			"Penawaran project ini sedang direvisi dan belum dikirim ulang — Nilai Kontrak-nya belum disepakati klien",
			map[string][]string{"quotation": {"Kirim ulang penawarannya dulu, baru terbitkan Tagihan."}})
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

// updateClientInvoice is Owner-or-Admin only -- confirmed role rule,
// PLAN.md mom-25082026-item-sebagian §3b, same bar deleteClientInvoice
// already uses.
func (h *Handler) updateClientInvoice(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner atau Admin yang dapat mengubah Tagihan", nil)
		return
	}
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

// markClientInvoicePaid is Owner-or-Admin only -- confirmed role rule,
// PLAN.md mom-25082026-item-sebagian §3b. This gate is not optional: marking
// an Invoice "Lunas" auto-creates a linked ClientPayment (ClientInvoice's
// own MarkPaid), so leaving this route open would let a non-Owner/Admin
// caller create a client payment through a back door even after
// createClientPayment/updateClientPayment are gated -- see the sibling
// unmarkClientInvoicePaid for the matching back door on the delete side.
func (h *Handler) markClientInvoicePaid(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner atau Admin yang dapat menandai Tagihan Lunas", nil)
		return
	}
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

// unmarkClientInvoicePaid is Owner-or-Admin only -- confirmed role rule,
// PLAN.md mom-25082026-item-sebagian §3b. Mirrors markClientInvoicePaid's
// gate: UnmarkPaid deletes the linked ClientPayment, which is exactly the
// back door deleteClientPayment's own Owner-or-Admin gate exists to close.
func (h *Handler) unmarkClientInvoicePaid(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, invoiceIDRaw string) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner atau Admin yang dapat membatalkan pelunasan Tagihan", nil)
		return
	}
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
	// Blok tanda tangan memakai staff yang MENERBITKAN tagihan ini, bukan
	// pemilik usaha (PLAN tanda-tangan-pengguna). ok=false — sentinel 0 atau
	// akun yang sudah dihapus permanen — mencetak blok kosong, tidak jatuh ke
	// orang lain.
	signerName, _, signature, signerOK, err := h.staff.GetSigner(r.Context(), claims.tenantID, inv.CreatedByStaffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !signerOK {
		signerName, signature = "", nil
	}
	totalPaid, err := h.clientPayments.TotalReceived(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	// Konteks PO: keduanya display-only dan menelan errornya sendiri (lihat
	// ResolvePONumber/QuotationComposition), jadi project pra-penawaran —
	// atau penawaran yang barisnya sudah hilang — mencetak tagihan tanpa
	// tabel komposisi, bukan gagal mencetak.
	// ResolvePONumber bertri-state (nil = tidak ada penawaran yang bisa
	// dituju, "" = ada tapi belum bernomor). Di atas kertas keduanya sama
	// saja: tidak ada nomor untuk dicantumkan, jadi keduanya jatuh ke "-".
	poNumber := ""
	if n := h.projects.ResolvePONumber(r.Context(), claims.tenantID, project.QuotationID); n != nil {
		poNumber = *n
	}
	composition := h.projects.QuotationComposition(r.Context(), claims.tenantID, project.QuotationID)

	pdf, err := buildClientInvoicePDF(*project, *inv, profile, logo, signature, totalPaid, poNumber, composition, signerName)
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
