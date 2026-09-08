package presentation

import (
	"encoding/json"
	"io"
	"net/http"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/response"
)

// clientHiddenEvidence reports whether an evidence row must NOT reach a client
// principal: one of the two opt-in kinds (general, projectMilestone) that was
// never marked client-visible. Every other kind is unconditionally
// client-visible and passes through — deliberate, and relied on by Client
// Portal's Kendala/Pembayaran/Vendor tabs (T-4, Blok E). Mirrors the
// application layer's clientVisibilityApplies + is_client_visible test, kept
// here because the HTTP layer is where the principal type is known.
func clientHiddenEvidence(e domain.Evidence) bool {
	if e.IsClientVisible {
		return false
	}
	return e.RelatedKind == domain.RelatedGeneral || e.RelatedKind == domain.RelatedProjectMilestone
}

// listEvidence is reachable by both staff and client principals (both pass
// resolveProjectAccess). For a client it drops the two opt-in kinds that were
// not marked client-visible — the T-4 fix: before this, a `general` document
// hidden with is_client_visible=0 still leaked to the client through this route.
func (h *Handler) listEvidence(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	list, err := h.evidence.List(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]evidenceResponse, 0, len(list))
	for _, e := range list {
		if claims.principalType == "client" && clientHiddenEvidence(e) {
			continue
		}
		result = append(result, toEvidenceResponse(e))
	}
	response.OK(w, "ok", result)
}

// listDocuments backs GET /projects/{id}/documents — Client Portal's own
// "Dokumen" tab. Reachable by staff too; the safety property is that this
// endpoint ALWAYS only returns general-kind, client-visible documents,
// unconditionally, regardless of who's calling — see
// EvidenceService.ListClientDocuments.
func (h *Handler) listDocuments(w http.ResponseWriter, r *http.Request, projectID int64) {
	list, err := h.evidence.ListClientDocuments(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]evidenceResponse, 0, len(list))
	for _, e := range list {
		result = append(result, toEvidenceResponse(e))
	}
	response.OK(w, "ok", result)
}

// listMilestoneDocuments backs GET /projects/{id}/milestone-documents — the
// timeline lampiran a client may see in Client Portal (Blok E). Reachable by
// staff too; the safety property is that it ALWAYS returns only
// projectMilestone-kind, client-visible rows, unconditionally, regardless of
// who's calling — see EvidenceService.ListClientMilestoneDocuments.
func (h *Handler) listMilestoneDocuments(w http.ResponseWriter, r *http.Request, projectID int64) {
	list, err := h.evidence.ListClientMilestoneDocuments(r.Context(), projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]evidenceResponse, 0, len(list))
	for _, e := range list {
		result = append(result, toEvidenceResponse(e))
	}
	response.OK(w, "ok", result)
}

type evidenceUploadBody struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	FileName        string `json:"fileName"`
	MimeType        string `json:"mimeType"`
	Base64Data      string `json:"base64Data"`
	DocumentDate    string `json:"documentDate"`
	Description     string `json:"description"`
	RelatedKind     string `json:"relatedKind"`
	RelatedID       int64  `json:"relatedId"`
	IsClientVisible bool   `json:"isClientVisible"`
}

func (h *Handler) uploadEvidence(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	var body evidenceUploadBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	e, err := h.evidence.Upload(r.Context(), claims.tenantID, projectID, claims.staffID, application.UploadEvidenceInput{
		Name: body.Name, Type: domain.EvidenceType(body.Type), FileName: body.FileName, MimeType: body.MimeType,
		Base64Data: body.Base64Data, DocumentDate: parseOptionalDate(body.DocumentDate), Description: body.Description,
		RelatedKind: domain.EvidenceRelatedKind(body.RelatedKind), RelatedID: body.RelatedID,
		IsClientVisible: body.IsClientVisible,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Evidence berhasil diunggah", toEvidenceResponse(*e))
}

// toggleEvidenceClientVisible flips the client visibility of one of the two
// opt-in kinds — `general` (project documents) and `projectMilestone`
// (timeline lampiran, Blok E) — without re-uploading the file. See
// EvidenceService.ToggleClientVisible for why it is rejected for every other
// RelatedKind. No claims parameter: this route is POST, which
// resolveProjectAccess already refuses for a client principal, so only staff
// ever reach it.
func (h *Handler) toggleEvidenceClientVisible(w http.ResponseWriter, r *http.Request, projectID int64, evidenceIDRaw string) {
	evidenceID, err := parseInt64(evidenceIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	e, err := h.evidence.ToggleClientVisible(r.Context(), projectID, evidenceID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Visibilitas dokumen diperbarui", toEvidenceResponse(*e))
}

// downloadEvidence streams one evidence file inline. It is deliberately NOT
// closed off to clients — EvidenceViewerModal uses it for every Client Portal
// tab — but a client is denied (403) exactly the rows listEvidence would have
// hidden from them: an opt-in kind (general, projectMilestone) not marked
// client-visible (T-4/Blok E). Staff are unaffected.
func (h *Handler) downloadEvidence(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, evidenceIDRaw string) {
	evidenceID, err := parseInt64(evidenceIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	e, reader, err := h.evidence.Download(r.Context(), projectID, evidenceID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	defer reader.Close()

	if claims.principalType == "client" && clientHiddenEvidence(*e) {
		response.Error(w, http.StatusForbidden, "Lampiran ini tidak dapat diakses", nil)
		return
	}

	w.Header().Set("Content-Disposition", `inline; filename="`+e.FileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}
