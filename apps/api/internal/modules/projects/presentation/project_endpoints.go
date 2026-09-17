package presentation

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/response"
)

func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}

// effectivePICFilter computes the single picStaffID/picSalesStaffID value to
// scope this request by, per role (docs/plan/revisi-putri-lanjutan/PLAN.md
// Blok I, D6): a "Staff" (Wedding Planner) or "Sales" caller is ALWAYS
// scoped to themselves and their own ?picStaffId/?picSalesStaffId query
// param is never even read — there is exactly one slot per column
// (mysql_project_repository.go's `pic_staff_id = ?` / `pic_sales_staff_id =
// ?`, already used for this same role-scoping, T-2), so there is no second
// value that could ever widen their own access by construction, not by a
// check that must be remembered. Owner/Admin may narrow the result with
// either query param — 0 is the "Belum ditugaskan" sentinel, a real filter
// value, not "no filter" (that's the param simply absent) — or see
// everything when neither is sent.
func effectivePICFilter(w http.ResponseWriter, r *http.Request, claims staffClaims) (picStaffID, picSalesStaffID *int64, ok bool) {
	if claims.role == "Staff" {
		return &claims.staffID, nil, true
	}
	if claims.role == "Sales" {
		return nil, &claims.staffID, true
	}
	if raw := r.URL.Query().Get("picStaffId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "Parameter picStaffId tidak valid", nil)
			return nil, nil, false
		}
		picStaffID = &id
	}
	if raw := r.URL.Query().Get("picSalesStaffId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "Parameter picSalesStaffId tidak valid", nil)
			return nil, nil, false
		}
		picSalesStaffID = &id
	}
	return picStaffID, picSalesStaffID, true
}

// parseEventMonthFilter reads the optional ?eventMonth=YYYY-MM query param
// (Blok I, D12/D7) — applies to every role, unlike the PIC filters above; it
// scopes ownership of nothing. A malformed value is answered 422 (same
// error shape as parseDate's callers below) rather than silently ignored:
// treating "September" as "no filter" would return every project and look
// like the filter itself is broken, not like a typo.
func parseEventMonthFilter(w http.ResponseWriter, r *http.Request) (*string, bool) {
	raw := r.URL.Query().Get("eventMonth")
	if raw == "" {
		return nil, true
	}
	if _, err := time.Parse("2006-01", raw); err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format bulan tidak valid", map[string][]string{"eventMonth": {"Gunakan format YYYY-MM"}})
		return nil, false
	}
	return &raw, true
}

// listProjects scopes the result to the caller's own PIC'd projects when
// their role is "Staff" (Wedding Planner) or "Sales" — nil/nil (Owner/Admin)
// means every project in the tenant, unchanged from before either role
// existed. Also backs Blok I's server-side filter bar (PIC/Sales/Bulan
// Event) — see effectivePICFilter/parseEventMonthFilter above for the gate.
func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request, claims staffClaims) {
	picStaffID, picSalesStaffID, ok := effectivePICFilter(w, r, claims)
	if !ok {
		return
	}
	eventMonth, ok := parseEventMonthFilter(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("all") == "true" {
		projects, err := h.projects.List(r.Context(), claims.tenantID, picStaffID, picSalesStaffID)
		if err != nil {
			writeAppError(w, err)
			return
		}
		result, err := h.toProjectResponsesWithProgress(r, claims.tenantID, projects)
		if err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "ok", result)
		return
	}
	params := pagination.FromRequest(r)
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	showArchived := r.URL.Query().Get("archived") == "true"
	projects, total, err := h.projects.ListPaginated(r.Context(), claims.tenantID, picStaffID, picSalesStaffID, params, search, status, showArchived, eventMonth)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result, err := h.toProjectResponsesWithProgress(r, claims.tenantID, projects)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OKPaginated(w, "ok", result, pagination.BuildMeta(params, total))
}

// toProjectResponsesWithProgress sets `.Progress` on every returned row using
// one ComputeProgressBatch call over exactly this page's project ids —
// PLAN.md "Performance remediation": the list endpoint now returns
// progress-complete rows in one round trip (the same contract shape GET
// /projects/{id} already has), instead of the frontend enriching each row
// with its own GET /projects/{id} call. `.PICName` is deliberately left ""
// here (unlike the single-project handlers below) -- nothing reads it off a
// list row today (the list page resolves PIC names client-side via
// /staff/summary instead), and resolving it per-row would reintroduce the
// exact N+1 this batching was written to avoid.
func (h *Handler) toProjectResponsesWithProgress(r *http.Request, tenantID int64, projects []domain.Project) ([]projectResponse, error) {
	ids := make([]int64, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	progressByID, err := h.projects.ComputeProgressBatch(r.Context(), tenantID, ids, time.Now())
	if err != nil {
		return nil, err
	}
	result := make([]projectResponse, 0, len(projects))
	for _, p := range projects {
		resp := toProjectResponse(p)
		if progress, ok := progressByID[p.ID]; ok {
			progressResp := toProgressResponse(progress)
			resp.Progress = &progressResp
		}
		result = append(result, resp)
	}
	return result, nil
}

type projectInputBody struct {
	Name      string `json:"name"`
	BrideName string `json:"brideName"`
	GroomName string `json:"groomName"`
	EventDate string `json:"eventDate"`
	// EventStartTime/EventEndTime are the project-level Jam Acara ("HH:MM"),
	// Blok A. The frontend schema always sends both keys as strings (default
	// ""); toProjectInput converts "" to a nil pointer ("Belum ditentukan").
	EventStartTime string `json:"eventStartTime"`
	EventEndTime   string `json:"eventEndTime"`
	// Pax is the guest count on the PO Paket header (blok B1). 0 = belum
	// ditentukan; guardKonteksUmum treats it as konteks umum like the fields
	// above, so a Wedding Planner may not change it.
	Pax           int    `json:"pax"`
	Venue         string `json:"venue"`
	PrepStartDate string `json:"prepStartDate"`
	PackageName   string `json:"packageName"`
	ContractValue int64  `json:"contractValue"`
	Status        string `json:"status"`
	PICStaffID    int64  `json:"picStaffId"`
	// PICSalesStaffID is the "PIC Sales" slot -- see
	// application.ProjectInput.PICSalesStaffID's doc comment for how Create
	// enforces it for a Sales caller.
	PICSalesStaffID int64  `json:"picSalesStaffId"`
	Description     string `json:"description"`
	// VenueID is only meaningful on update -- see ProjectInput.VenueID's doc
	// comment for the omitted/0/positive tri-state. create/duplicate bodies
	// never send this key, so it decodes to nil and Create simply never
	// reads it (ADR-0016: attaching a venue is a separate, post-creation
	// action).
	VenueID *int64 `json:"venueId"`
	// PackageTemplateID applies a Template Paket at creation time (D10): it
	// seeds the composition, the terms text and the payment-schedule preset
	// in one step, and writes the template's name into PackageName (D28).
	// Only read by createProject; 0 or absent means "Tanpa template", which
	// leaves the project on the empty-state path (D23).
	PackageTemplateID int64 `json:"packageTemplateId"`
	// VenueRentalPrice/VenueCharge ride along with VenueID whenever it's
	// present at all -- the per-project cost snapshot (PLAN.md "Financial
	// Calculation Correctness"). Meaningless (and ignored by
	// ProjectService.Update) when VenueID itself is nil or the detach
	// sentinel 0.
	VenueRentalPrice *int64 `json:"venueRentalPrice"`
	VenueCharge      *int64 `json:"venueCharge"`
}

// emptyStrToNilPtr maps an empty string to a nil *string, and any other value
// to a pointer to it -- used for the project-level Jam Acara (Blok A) where ""
// from the body means "Belum ditentukan" (stored NULL), not the string "".
func emptyStrToNilPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toProjectInput(body projectInputBody) (application.ProjectInput, error) {
	eventDate, err := parseDate(body.EventDate)
	if err != nil {
		return application.ProjectInput{}, err
	}
	prepStartDate, err := parseDate(body.PrepStartDate)
	if err != nil {
		return application.ProjectInput{}, err
	}
	return application.ProjectInput{
		Name: body.Name, BrideName: body.BrideName, GroomName: body.GroomName, EventDate: eventDate,
		EventStartTime: emptyStrToNilPtr(body.EventStartTime), EventEndTime: emptyStrToNilPtr(body.EventEndTime),
		Pax:   body.Pax,
		Venue: body.Venue, PrepStartDate: prepStartDate, PackageName: body.PackageName,
		ContractValue: body.ContractValue, Status: domain.ProjectStatus(body.Status),
		PICStaffID: body.PICStaffID, PICSalesStaffID: body.PICSalesStaffID, Description: body.Description, VenueID: body.VenueID,
		VenueRentalPrice: body.VenueRentalPrice, VenueCharge: body.VenueCharge,
	}, nil
}

// createProject is Owner/Admin/Sales (confirmed role rule) — a Wedding
// Planner only ever operates within a project already assigned to them by
// someone else, never creates one themselves; Sales creates a project of
// their own (see ProjectService.Create's callerRole handling for how their
// PICSalesStaffID gets forced to themselves).
// me is the Client Portal's entry point — a client principal has no other way
// to learn its own project ids (login doesn't return them, and `identity` is
// deliberately profile-agnostic per ADR-0005). Returns the client's OWN
// projects (T1.13, D5): the portal enters directly when exactly one, and
// renders a project picker when more than one.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "client" {
		response.Error(w, http.StatusForbidden, "Hanya client yang dapat mengakses endpoint ini", nil)
		return
	}
	tenantID, ok := claims.TenantIDInt()
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}
	contactID, err := strconv.ParseInt(claims.PrincipalID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Identitas client tidak valid", nil)
		return
	}
	projectIDs, ok := h.clientProjectIDs(r.Context(), tenantID, contactID)
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini belum terhubung ke project manapun", nil)
		return
	}
	// Progress ikut dibawa: portal client memakai daftar ini apa adanya ketika
	// client hanya punya satu project (kasus mayoritas) dan tidak lagi memanggil
	// detail per project, sehingga ring progress akan kosong tanpa ini.
	// ComputeProgressBatch, bukan ComputeProgress per baris — satu client bisa
	// punya beberapa project (D5) dan bentuk per-baris itu N+1.
	progressByProject, err := h.projects.ComputeProgressBatch(r.Context(), tenantID, projectIDs, time.Now())
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]projectResponse, 0, len(projectIDs))
	for _, pid := range projectIDs {
		p, err := h.projects.Get(r.Context(), tenantID, pid)
		if err != nil {
			writeAppError(w, err)
			return
		}
		resp := toProjectResponse(*p)
		if pr, ok := progressByProject[p.ID]; ok {
			progressResp := toProgressResponse(pr)
			resp.Progress = &progressResp
		}
		resp.PICName = h.projects.ResolvePICName(r.Context(), tenantID, p.PICStaffID)
		result = append(result, resp)
	}
	response.OK(w, "ok", result)
}

func (h *Handler) getProject(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	tenantID := claims.tenantID
	p, err := h.projects.Get(r.Context(), tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	progress, err := h.projects.ComputeProgress(r.Context(), tenantID, projectID, time.Now())
	if err != nil {
		writeAppError(w, err)
		return
	}
	resp := toProjectResponse(*p)
	progressResp := toProgressResponse(*progress)
	resp.Progress = &progressResp
	resp.PICName = h.projects.ResolvePICName(r.Context(), tenantID, p.PICStaffID)
	// PONumber dkk. diisi hanya untuk pemanggil yang memang boleh membuka
	// penawarannya: tautan "Penawaran" di header detail project (WO Console)
	// memakai PONumber/PackageNameFromQuotation/QuotationUnderRevision, dan kartu
	// "Purchase Order" di portal client memakai PONumber (nama berkas) +
	// QuotationPDFAvailable (tampil/sembunyi) — PLAN revisi-vendor-venue-portal
	// §1.1 poin 11 (PLAN revisi-vendor-venue-portal §4.5/F5) sengaja membalik
	// keputusan lama yang menyatakan portal tidak butuh query ini. Sekali per
	// pembukaan portal (ClientPortalLayout memanggil fetchProjectDetail sekali
	// per sesi), bukan per baris daftar: daftar project dan /projects/me tetap
	// tidak pernah mengisinya (itu akan jadi satu query per baris). Server yang
	// memutuskan, bukan UI.
	if claims.principalType == "staff" && canReadQuotation(claims.role) {
		resp.PONumber = h.projects.ResolvePONumber(r.Context(), tenantID, p.QuotationID)
		resp.PackageNameFromQuotation = h.projects.PackageNameFromQuotation(r.Context(), tenantID, p.QuotationID)
		resp.QuotationUnderRevision = h.projects.QuotationUnderRevision(r.Context(), tenantID, p.QuotationID)
	}
	if claims.principalType == "client" && p.QuotationID != 0 {
		resp.PONumber = h.projects.ResolvePONumber(r.Context(), tenantID, p.QuotationID)
		resp.QuotationPDFAvailable = h.projects.QuotationAccepted(r.Context(), tenantID, p.QuotationID)
	}
	response.OK(w, "ok", resp)
}

func (h *Handler) updateProject(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	var body projectInputBody
	if err := json.Unmarshal(data, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	// PATCH is partial by definition, but projectInputBody decodes every
	// absent key to its Go zero value — forwarding that straight into
	// ProjectService.Update silently zeroes stored figures (contractValue,
	// pax, …) the caller never meant to touch. Fill absent keys from the
	// current row first, so an omitted key always means "leave it", never
	// "reset it". venueId/venueRentalPrice/venueCharge are already
	// presence-aware (*int64, nil = omitted) and are deliberately left out
	// of the overlay. The frontend always sends the full body, so for its
	// requests this merge is a strict no-op.
	var present map[string]json.RawMessage
	if err := json.Unmarshal(data, &present); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	current, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	body = overlayMissingProjectFields(body, present, current)
	input, err := toProjectInput(body)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"eventDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	p, err := h.projects.Update(r.Context(), claims.tenantID, projectID, claims.staffID, claims.role, input)
	if err != nil {
		writeAppError(w, err)
		return
	}
	resp := toProjectResponse(*p)
	resp.PICName = h.projects.ResolvePICName(r.Context(), claims.tenantID, p.PICStaffID)
	response.OK(w, "Project berhasil diperbarui", resp)
}

// overlayMissingProjectFields backs updateProject's partial-PATCH contract
// above: every key absent from the request body is filled from the stored
// row, so only keys the caller actually sent can change anything.
func overlayMissingProjectFields(body projectInputBody, present map[string]json.RawMessage, p *domain.Project) projectInputBody {
	presentHas := func(key string) bool {
		_, ok := present[key]
		return ok
	}
	if !presentHas("name") {
		body.Name = p.Name
	}
	if !presentHas("brideName") {
		body.BrideName = p.BrideName
	}
	if !presentHas("groomName") {
		body.GroomName = p.GroomName
	}
	if !presentHas("eventDate") {
		body.EventDate = p.EventDate.Format(dateLayout)
	}
	if !presentHas("eventStartTime") {
		body.EventStartTime = derefString(p.EventStartTime)
	}
	if !presentHas("eventEndTime") {
		body.EventEndTime = derefString(p.EventEndTime)
	}
	if !presentHas("pax") {
		body.Pax = p.Pax
	}
	if !presentHas("venue") {
		body.Venue = p.Venue
	}
	if !presentHas("prepStartDate") {
		body.PrepStartDate = p.PrepStartDate.Format(dateLayout)
	}
	if !presentHas("packageName") {
		body.PackageName = p.PackageName
	}
	if !presentHas("contractValue") {
		body.ContractValue = p.ContractValue
	}
	if !presentHas("status") {
		body.Status = string(p.Status)
	}
	if !presentHas("picStaffId") {
		body.PICStaffID = p.PICStaffID
	}
	if !presentHas("picSalesStaffId") {
		body.PICSalesStaffID = p.PICSalesStaffID
	}
	if !presentHas("description") {
		body.Description = p.Description
	}
	return body
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// getProjectVenue backs GET /projects/{id}/venue (ADR-0016) -- reachable by
// both staff and client principals (resolveProjectAccess in handler.go
// already scoped access before this is called), always the public-safe
// VenueSummary shape regardless of caller. Returns `data: null` (not an
// error) when no venue is attached, so both the WO Console tab and Client
// Portal's tab can render an empty state.
func (h *Handler) getProjectVenue(w http.ResponseWriter, r *http.Request, tenantID, projectID int64) {
	summary, err := h.projects.GetVenue(r.Context(), tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if summary == nil {
		response.OK(w, "ok", nil)
		return
	}
	response.OK(w, "ok", toVenueSummaryResponse(*summary))
}

func (h *Handler) cancelProject(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	p, err := h.projects.Cancel(r.Context(), claims.tenantID, projectID, claims.staffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	resp := toProjectResponse(*p)
	resp.PICName = h.projects.ResolvePICName(r.Context(), claims.tenantID, p.PICStaffID)
	response.OK(w, "Project dibatalkan", resp)
}

// toggleArchiveProject is a single toggle (flips the current is_archived
// state), same convention as vendors'/staff's toggle-active — no body, no
// separate archive/unarchive endpoints. See ADR-0013.
func (h *Handler) toggleArchiveProject(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	current, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	p, err := h.projects.SetArchived(r.Context(), claims.tenantID, projectID, claims.staffID, !current.IsArchived)
	if err != nil {
		writeAppError(w, err)
		return
	}
	resp := toProjectResponse(*p)
	resp.PICName = h.projects.ResolvePICName(r.Context(), claims.tenantID, p.PICStaffID)
	response.OK(w, "Status arsip project diperbarui", resp)
}

// deleteProject is the hard-delete endpoint (ADR-0013) — Owner-only, checked
// inline here since this module has no other role-gating to build on (same
// idiom as platform's tenant_handler.go Owner-only checks). The
// archived-or-cancelled precondition is enforced in ProjectService.Delete
// itself, not just here, so it can never be bypassed by a differently-shaped
// request.
func (h *Handler) deleteProject(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	if claims.role != "Owner" {
		response.Error(w, http.StatusForbidden, "Hanya akun Owner yang dapat menghapus project secara permanen", nil)
		return
	}
	if err := h.projects.Delete(r.Context(), claims.tenantID, projectID); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Project berhasil dihapus permanen", nil)
}

type projectDeleteImpactResponse struct {
	ProjectID        int64  `json:"projectId"`
	ProjectName      string `json:"projectName"`
	QuotationID      int64  `json:"quotationId"`
	PONumber         string `json:"poNumber"`
	PaidInvoiceCount int    `json:"paidInvoiceCount"`
	PaidInvoiceTotal int64  `json:"paidInvoiceTotal"`
}

// projectDeleteImpact WAJIB dipanggil sebelum dialog hapus dirender (D14,
// T3.5) — menyebut nomor PO yang ikut terhapus lewat DeleteForProject.
func (h *Handler) projectDeleteImpact(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	p, err := h.projects.Get(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	poNumber, err := h.projects.PONumberForQuotation(r.Context(), claims.tenantID, p.QuotationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	impact, err := h.projects.ProjectDeleteImpact(r.Context(), claims.tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", projectDeleteImpactResponse{
		ProjectID: p.ID, ProjectName: p.Name,
		QuotationID: p.QuotationID, PONumber: poNumber,
		PaidInvoiceCount: impact.PaidInvoiceCount, PaidInvoiceTotal: impact.PaidInvoiceTotal,
	})
}

// --- Project milestones ---

func (h *Handler) listMilestones(w http.ResponseWriter, r *http.Request, tenantID, projectID int64) {
	milestones, err := h.projects.ListMilestones(r.Context(), tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]milestoneResponse, 0, len(milestones))
	for _, m := range milestones {
		result = append(result, toMilestoneResponse(m))
	}
	response.OK(w, "ok", result)
}

type milestoneInputBody struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	TargetDate string `json:"targetDate"`
}

func (h *Handler) createMilestone(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	var body milestoneInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	targetDate, err := parseDate(body.TargetDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"targetDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	m, err := h.projects.CreateMilestone(r.Context(), claims.tenantID, projectID, claims.staffID, application.MilestoneInput{Name: body.Name, Category: body.Category, TargetDate: targetDate})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Timeline berhasil ditambahkan", toMilestoneResponse(*m))
}

type milestoneUpdateBody struct {
	Category      string `json:"category"`
	Status        string `json:"status"`
	TargetDate    string `json:"targetDate"`
	CompletedDate string `json:"completedDate"`
}

func (h *Handler) updateMilestone(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64, milestoneIDRaw string) {
	milestoneID, err := parseInt64(milestoneIDRaw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID timeline tidak valid", nil)
		return
	}
	var body milestoneUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	targetDate, err := parseDate(body.TargetDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"targetDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	m, err := h.projects.UpdateMilestone(r.Context(), claims.tenantID, projectID, milestoneID, claims.staffID, application.MilestoneUpdateInput{
		Category: body.Category, Status: domain.MilestoneStatus(body.Status), TargetDate: targetDate, CompletedDate: parseOptionalDate(body.CompletedDate),
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Timeline berhasil diperbarui", toMilestoneResponse(*m))
}

type milestoneReorderBody struct {
	OrderedIDs []int64 `json:"orderedIds"`
}

func (h *Handler) reorderMilestones(w http.ResponseWriter, r *http.Request, claims staffClaims, projectID int64) {
	var body milestoneReorderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	if err := h.projects.ReorderMilestones(r.Context(), claims.tenantID, projectID, claims.staffID, body.OrderedIDs); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Urutan timeline berhasil diperbarui", nil)
}
