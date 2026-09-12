package presentation

import (
	"encoding/json"
	"net/http"
	"strconv"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/response"
)

// isOwnerTemplateManager gates every WRITE on a Template Paket.
//
// Owner-only, deliberately stricter than the Owner-or-Admin bar a PO itself
// uses (D18): a template is tenant master data sitting under Pengaturan, and
// its direct analog there — Timeline Default — is already Owner-only
// (requireOwnerForTemplates in milestone_template_handler.go). Defining what
// the WO sells is a different act from applying it to one deal.
//
// Reads stay open to every staff role on purpose: "Sales" may create a
// project (createProject only rejects "Staff") and therefore needs the
// template picker to be readable.
func isOwnerTemplateManager(role string) bool {
	return role == "Owner"
}

// PackageTemplateHandler serves /api/v1/package-templates — the tenant's
// reusable sales packages (Pengaturan → Template Paket). Separate handler
// rather than part of the projects subtree because these are tenant master
// data, not project data; same split MilestoneTemplateHandler already uses.
type PackageTemplateHandler struct {
	templates *application.PackageTemplateService
}

func NewPackageTemplateHandler(templates *application.PackageTemplateService) *PackageTemplateHandler {
	return &PackageTemplateHandler{templates: templates}
}

type packageTemplateResponse struct {
	ID               int64                         `json:"id"`
	Name             string                        `json:"name"`
	BasePrice        int64                         `json:"basePrice"`
	DefaultTerms     string                        `json:"defaultTerms"`
	DefaultBonusNote string                        `json:"defaultBonusNote"`
	IsActive         bool                          `json:"isActive"`
	SortOrder        int                           `json:"sortOrder"`
	Blocks           []packageBlockResponse        `json:"blocks"`
	Terms            []packageTemplateTermResponse `json:"terms"`
}

type packageTemplateTermResponse struct {
	ID              int64    `json:"id"`
	Sequence        int      `json:"sequence"`
	Label           string   `json:"label"`
	Type            string   `json:"type"`
	Percent         *float64 `json:"percent"`
	FixedAmount     *int64   `json:"fixedAmount"`
	DaysBeforeEvent int      `json:"daysBeforeEvent"`
}

func toPackageTemplateResponse(t domain.PackageTemplate) packageTemplateResponse {
	out := packageTemplateResponse{
		ID: t.ID, Name: t.Name, BasePrice: t.BasePrice,
		DefaultTerms: t.DefaultTerms, DefaultBonusNote: t.DefaultBonusNote,
		IsActive: t.IsActive, SortOrder: t.SortOrder,
		Blocks: make([]packageBlockResponse, 0, len(t.Blocks)),
		Terms:  make([]packageTemplateTermResponse, 0, len(t.Terms)),
	}
	for _, b := range t.Blocks {
		out.Blocks = append(out.Blocks, packageBlockResponse{
			ID: b.ID, Category: b.Category, Body: b.Body, QtyText: b.QtyText,
			BonusNote: b.BonusNote, SortOrder: b.SortOrder,
		})
	}
	for _, term := range t.Terms {
		out.Terms = append(out.Terms, packageTemplateTermResponse{
			ID: term.ID, Sequence: term.Sequence, Label: term.Label, Type: string(term.Type),
			Percent: term.Percent, FixedAmount: term.FixedAmount, DaysBeforeEvent: term.DaysBeforeEvent,
		})
	}
	return out
}

type packageTemplateBody struct {
	Name             string `json:"name"`
	BasePrice        int64  `json:"basePrice"`
	DefaultTerms     string `json:"defaultTerms"`
	DefaultBonusNote string `json:"defaultBonusNote"`
	IsActive         bool   `json:"isActive"`
}

func (b packageTemplateBody) toInput() application.PackageTemplateInput {
	return application.PackageTemplateInput{
		Name: b.Name, BasePrice: b.BasePrice, DefaultTerms: b.DefaultTerms,
		DefaultBonusNote: b.DefaultBonusNote, IsActive: b.IsActive,
	}
}

func (h *PackageTemplateHandler) Collection(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireStaff(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		// activeOnly=true serves the project-create picker, which must not
		// offer a retired package; the settings page passes nothing and sees
		// everything.
		activeOnly := r.URL.Query().Get("activeOnly") == "true"
		list, err := h.templates.List(r.Context(), claims.tenantID, activeOnly)
		if err != nil {
			writeAppError(w, err)
			return
		}
		result := make([]packageTemplateResponse, 0, len(list))
		for _, t := range list {
			result = append(result, toPackageTemplateResponse(t))
		}
		response.OK(w, "ok", result)
	case http.MethodPost:
		if !isOwnerTemplateManager(claims.role) {
			response.Error(w, http.StatusForbidden, "Hanya Owner yang dapat mengelola template paket", nil)
			return
		}
		var body packageTemplateBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
			return
		}
		t, err := h.templates.Create(r.Context(), claims.tenantID, body.toInput())
		if err != nil {
			writeAppError(w, err)
			return
		}
		response.Created(w, "Template paket dibuat", toPackageTemplateResponse(*t))
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
	}
}

func (h *PackageTemplateHandler) Item(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireStaff(w, r)
	if !ok {
		return
	}
	segments := httpx.Segments(r.URL.Path, "/api/v1/package-templates/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Template paket tidak ditemukan", nil)
		return
	}
	id, err := strconv.ParseInt(segments[0], 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID template tidak valid", nil)
		return
	}
	rest := segments[1:]

	if r.Method != http.MethodGet && !isOwnerTemplateManager(claims.role) {
		response.Error(w, http.StatusForbidden, "Hanya Owner yang dapat mengelola template paket", nil)
		return
	}

	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		t, err := h.templates.Get(r.Context(), claims.tenantID, id)
		if err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "ok", toPackageTemplateResponse(*t))
	case len(rest) == 0 && r.Method == http.MethodPatch:
		var body packageTemplateBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
			return
		}
		t, err := h.templates.Update(r.Context(), claims.tenantID, id, body.toInput())
		if err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "Template paket diperbarui", toPackageTemplateResponse(*t))
	case len(rest) == 0 && r.Method == http.MethodDelete:
		if err := h.templates.Delete(r.Context(), claims.tenantID, id); err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "Template paket dihapus", nil)
	case len(rest) == 1 && rest[0] == "blocks" && r.Method == http.MethodPut:
		var body []packageBlockBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
			return
		}
		blocks := make([]domain.PackageTemplateBlock, 0, len(body))
		for _, b := range body {
			blocks = append(blocks, domain.PackageTemplateBlock{
				Category: b.Category, Body: b.Body, QtyText: b.QtyText, BonusNote: b.BonusNote,
			})
		}
		if err := h.templates.ReplaceBlocks(r.Context(), claims.tenantID, id, blocks); err != nil {
			writeAppError(w, err)
			return
		}
		h.respondWithTemplate(w, r, claims.tenantID, id)
	case len(rest) == 1 && rest[0] == "terms" && r.Method == http.MethodPut:
		var body []packageTemplateTermBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
			return
		}
		terms := make([]domain.PackageTemplateTerm, 0, len(body))
		for _, t := range body {
			terms = append(terms, domain.PackageTemplateTerm{
				Label: t.Label, Type: domain.PaymentType(t.Type), Percent: t.Percent,
				FixedAmount: t.FixedAmount, DaysBeforeEvent: t.DaysBeforeEvent,
			})
		}
		if err := h.templates.ReplaceTerms(r.Context(), claims.tenantID, id, terms); err != nil {
			writeAppError(w, err)
			return
		}
		h.respondWithTemplate(w, r, claims.tenantID, id)
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

type packageTemplateTermBody struct {
	Label           string   `json:"label"`
	Type            string   `json:"type"`
	Percent         *float64 `json:"percent"`
	FixedAmount     *int64   `json:"fixedAmount"`
	DaysBeforeEvent int      `json:"daysBeforeEvent"`
}

// respondWithTemplate re-reads after a child-collection write so the client
// gets back the same full shape GET returns, rather than having to refetch.
func (h *PackageTemplateHandler) respondWithTemplate(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	t, err := h.templates.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Template paket diperbarui", toPackageTemplateResponse(*t))
}
