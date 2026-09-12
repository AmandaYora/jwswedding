package presentation

import (
	"encoding/json"
	"net/http"
	"strings"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/response"
)

type packageBlockBody struct {
	Category  string `json:"category"`
	Body      string `json:"body"`
	QtyText   string `json:"qtyText"`
	BonusNote string `json:"bonusNote"`
}

type packageAdjustmentBody struct {
	Description string `json:"description"`
	// Amount is SIGNED — negative is a takeout/cashback (D2). The frontend
	// sends the sign; there is no separate "kind" key to keep in step with it.
	Amount int64 `json:"amount"`
}

type packageOrderResponse struct {
	// PONumber is "" until the PO is first issued (D26), never a placeholder.
	PONumber    string                  `json:"poNumber"`
	Revision    int                     `json:"revision"`
	Status      string                  `json:"status"`
	BasePrice   int64                   `json:"basePrice"`
	TermsText   string                  `json:"termsText"`
	BonusNote   string                  `json:"bonusNote"`
	IssuedAt    *string                 `json:"issuedAt"`
	Blocks      []packageBlockResponse  `json:"blocks"`
	Adjustments []packageAdjustResponse `json:"adjustments"`
	TermsPlan   []packageTermResponse   `json:"termsPlan"`
	// TotalAdjustments/Total spare the frontend from recomputing what the
	// backend already had to compute to write contract_value (D15).
	TotalAdjustments int64 `json:"totalAdjustments"`
	Total            int64 `json:"total"`
}

type packageBlockResponse struct {
	ID        int64  `json:"id"`
	Category  string `json:"category"`
	Body      string `json:"body"`
	QtyText   string `json:"qtyText"`
	BonusNote string `json:"bonusNote"`
	SortOrder int    `json:"sortOrder"`
}

type packageAdjustResponse struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	SortOrder   int    `json:"sortOrder"`
}

type packageTermResponse struct {
	Sequence        int      `json:"sequence"`
	Label           string   `json:"label"`
	Type            string   `json:"type"`
	Percent         *float64 `json:"percent"`
	FixedAmount     *int64   `json:"fixedAmount"`
	DaysBeforeEvent int      `json:"daysBeforeEvent"`
	// Amount is this term's share of the CURRENT total, computed the same way
	// Issue will seed it. It is what the "TAHAP PEMBAYARAN" block shows while
	// the PO is still Draft (D27), when no invoice exists yet.
	Amount int64 `json:"amount"`
}

// toPackageOrderView returns nil when the project has no PO — the frontend's
// empty state (D23), not an error.
func toPackageOrderView(view *application.PackageOrderView) *packageOrderResponse {
	if view == nil || view.Order == nil {
		return nil
	}
	o := view.Order
	out := &packageOrderResponse{
		PONumber: o.PONumber, Revision: o.Revision, Status: string(o.Status),
		BasePrice: o.BasePrice, TermsText: o.TermsText, BonusNote: o.BonusNote,
		Blocks:           make([]packageBlockResponse, 0, len(view.Blocks)),
		Adjustments:      make([]packageAdjustResponse, 0, len(view.Adjustments)),
		TermsPlan:        make([]packageTermResponse, 0, len(o.TermsPlan)),
		TotalAdjustments: domain.TotalAdjustments(view.Adjustments),
		Total:            view.Total,
	}
	if o.IssuedAt != nil {
		formatted := o.IssuedAt.Format(dateLayout)
		out.IssuedAt = &formatted
	}
	for _, b := range view.Blocks {
		out.Blocks = append(out.Blocks, packageBlockResponse{
			ID: b.ID, Category: b.Category, Body: b.Body, QtyText: b.QtyText,
			BonusNote: b.BonusNote, SortOrder: b.SortOrder,
		})
	}
	for _, a := range view.Adjustments {
		out.Adjustments = append(out.Adjustments, packageAdjustResponse{
			ID: a.ID, Description: a.Description, Amount: a.Amount, SortOrder: a.SortOrder,
		})
	}
	amounts := application.ComputeScheduleAmounts(view.Total, o.TermsPlan)
	for i, t := range o.TermsPlan {
		out.TermsPlan = append(out.TermsPlan, packageTermResponse{
			Sequence: t.Sequence, Label: t.Label, Type: string(t.Type),
			Percent: t.Percent, FixedAmount: t.FixedAmount,
			DaysBeforeEvent: t.DaysBeforeEvent, Amount: amounts[i],
		})
	}
	return out
}

func (h *Handler) writePackageOrder(w http.ResponseWriter, view *application.PackageOrderView, err error) {
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toPackageOrderView(view))
}

// getPackageOrder is open to every principal that can read the project,
// clients included — the PO is the document they signed.
func (h *Handler) getPackageOrder(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	view, err := h.packageOrders.Get(r.Context(), claims.tenantID, projectID)
	h.writePackageOrder(w, view, err)
}

type applyTemplateBody struct {
	TemplateID int64 `json:"templateId"`
}

// applyPackageTemplate and every other write below is Owner-or-Admin (D18):
// a PO carries the project's contract value, so it sits behind the same bar
// createClientInvoice and the payment ledgers already use.
func (h *Handler) applyPackageTemplate(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat mengatur PO Paket", nil)
		return
	}
	var body applyTemplateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
		return
	}
	view, err := h.packageOrders.ApplyTemplate(r.Context(), claims.tenantID, projectID, body.TemplateID, claims.staffID)
	h.writePackageOrder(w, view, err)
}

func (h *Handler) startBlankPackageOrder(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat mengatur PO Paket", nil)
		return
	}
	view, err := h.packageOrders.StartBlank(r.Context(), claims.tenantID, projectID, claims.staffID)
	h.writePackageOrder(w, view, err)
}

type packageHeaderBody struct {
	BasePrice int64  `json:"basePrice"`
	TermsText string `json:"termsText"`
	BonusNote string `json:"bonusNote"`
}

func (h *Handler) updatePackageHeader(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat mengatur PO Paket", nil)
		return
	}
	var body packageHeaderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
		return
	}
	view, err := h.packageOrders.SetHeader(r.Context(), claims.tenantID, projectID, body.BasePrice, body.TermsText, body.BonusNote)
	h.writePackageOrder(w, view, err)
}

func (h *Handler) replacePackageBlocks(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat mengatur PO Paket", nil)
		return
	}
	var body []packageBlockBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
		return
	}
	blocks := make([]domain.ProjectPackageBlock, 0, len(body))
	for _, b := range body {
		blocks = append(blocks, domain.ProjectPackageBlock{
			ProjectID: projectID, Category: b.Category, Body: b.Body,
			QtyText: b.QtyText, BonusNote: b.BonusNote,
		})
	}
	view, err := h.packageOrders.ReplaceBlocks(r.Context(), claims.tenantID, projectID, blocks)
	h.writePackageOrder(w, view, err)
}

func (h *Handler) replacePackageAdjustments(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat mengatur PO Paket", nil)
		return
	}
	var body []packageAdjustmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
		return
	}
	adjustments := make([]domain.ProjectPackageAdjustment, 0, len(body))
	for _, a := range body {
		adjustments = append(adjustments, domain.ProjectPackageAdjustment{
			ProjectID: projectID, Description: a.Description, Amount: a.Amount,
		})
	}
	view, err := h.packageOrders.ReplaceAdjustments(r.Context(), claims.tenantID, projectID, adjustments)
	h.writePackageOrder(w, view, err)
}

// issuePackageOrder resolves the client phone here rather than inside the
// service so `projects` keeps its cross-module lookups at the edge — the
// service itself takes plain strings and knows nothing about `clients`.
func (h *Handler) issuePackageOrder(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat menerbitkan PO Paket", nil)
		return
	}
	project, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	phone := h.resolveClientPhone(r.Context(), claims.tenantID, projectID)
	clientName := project.BrideName + " & " + project.GroomName
	view, issueErr := h.packageOrders.Issue(r.Context(), claims.tenantID, projectID, claims.staffID, clientName, phone)
	h.writePackageOrder(w, view, issueErr)
}

func (h *Handler) revisePackageOrder(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat merevisi PO Paket", nil)
		return
	}
	view, err := h.packageOrders.Revise(r.Context(), claims.tenantID, projectID, claims.staffID)
	h.writePackageOrder(w, view, err)
}

// downloadPackageOrderPDF renders the PO document itself.
//
// Open to every principal that can read the project, clients included — it is
// the contract they signed. Writes are gated elsewhere; this is a read.
func (h *Handler) downloadPackageOrderPDF(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	view, err := h.packageOrders.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if view.Order == nil {
		response.Error(w, http.StatusNotFound, "Project ini belum punya PO Paket", nil)
		return
	}
	project, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	// Same completeness gate the Invoice/Kwitansi PDFs use — a document
	// carrying the WO's letterhead must not print without the profile behind
	// that letterhead being filled in.
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
	payments, err := h.clientPayments.List(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	totalPaid, err := h.clientPayments.TotalReceived(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	clientName := project.BrideName + " & " + project.GroomName
	phone := h.resolveClientPhone(r.Context(), claims.tenantID, projectID)
	data := buildPackageOrderPrintData(view, *project, clientName, phone, payments, totalPaid)

	pdf, err := buildPackageOrderPDF(data, *project, profile, logo, signature)
	if err != nil {
		writeAppError(w, err)
		return
	}
	// A Draft has no number yet (D26), so the file falls back to the project
	// name rather than being called "-.pdf".
	filename := view.Order.PONumber
	if filename == "" {
		filename = "PO-Draft-" + project.Name
	}
	filename = strings.NewReplacer("/", "-", "\\", "-", `"`, "").Replace(filename)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.pdf"`)
	if err := pdf.Output(w); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas PDF", nil)
	}
}

func (h *Handler) cancelPackageOrder(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if !isOwnerOrAdmin(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat membatalkan PO Paket", nil)
		return
	}
	view, err := h.packageOrders.Cancel(r.Context(), claims.tenantID, projectID, claims.staffID)
	h.writePackageOrder(w, view, err)
}
