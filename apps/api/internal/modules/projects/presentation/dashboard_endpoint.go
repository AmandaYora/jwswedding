package presentation

import (
	"net/http"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/response"
)

type dashboardIssueResponse struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"projectId"`
	ProjectName string `json:"projectName"`
	VendorID    int64  `json:"vendorId"`
	Title       string `json:"title"`
	Impact      string `json:"impact"`
	FoundDate   string `json:"foundDate"`
	Status      string `json:"status"`
}

type dashboardMilestoneResponse struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"projectId"`
	ProjectName string `json:"projectName"`
	VendorID    int64  `json:"vendorId"`
	Name        string `json:"name"`
	TargetDate  string `json:"targetDate"`
}

type dashboardPaymentResponse struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"projectId"`
	ProjectName string `json:"projectName"`
	VendorID    int64  `json:"vendorId"`
	Type        string `json:"type"`
	Amount      int64  `json:"amount"`
	PaymentDate string `json:"paymentDate"`
}

// dashboardVenuePaymentResponse has no VendorID -- a venue payment isn't
// tied to any vendor. Kept as its own response type/field (see
// domain.DashboardVenuePaymentRow's doc comment) rather than reshaping
// dashboardPaymentResponse into a source-discriminated union.
type dashboardVenuePaymentResponse struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"projectId"`
	ProjectName string `json:"projectName"`
	Type        string `json:"type"`
	Amount      int64  `json:"amount"`
	PaymentDate string `json:"paymentDate"`
}

type laggingProjectResponse struct {
	Project        projectResponse `json:"project"`
	OverallPercent int             `json:"overallPercent"`
}

type projectTrendPointResponse struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type revenueTrendPointResponse struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Total int64  `json:"total"`
}

type revenueSummaryResponse struct {
	Total         int64    `json:"total"`
	PreviousTotal int64    `json:"previousTotal"`
	DeltaPercent  *float64 `json:"deltaPercent"`
}

type dashboardResponse struct {
	TotalProjects           int                             `json:"totalProjects"`
	ActiveProjects          int                             `json:"activeProjects"`
	ActiveVendorCount       int                             `json:"activeVendorCount"`
	OpenIssues              []dashboardIssueResponse        `json:"openIssues"`
	OverdueVendorMilestones []dashboardMilestoneResponse    `json:"overdueVendorMilestones"`
	IncompletePayments      []dashboardPaymentResponse      `json:"incompletePayments"`
	IncompleteVenuePayments []dashboardVenuePaymentResponse `json:"incompleteVenuePayments"`
	NearDDayProjects        []projectResponse               `json:"nearDDayProjects"`
	LaggingProjects         []laggingProjectResponse        `json:"laggingProjects"`
	UpcomingProjects        []projectResponse               `json:"upcomingProjects"`
	RecentActivity          []activityResponse              `json:"recentActivity"`
	Revenue                 revenueSummaryResponse          `json:"revenue"`
	ProjectTrend            []projectTrendPointResponse     `json:"projectTrend"`
	RevenueTrend            []revenueTrendPointResponse     `json:"revenueTrend"`
}

func toDashboardResponse(s domain.DashboardStats) dashboardResponse {
	resp := dashboardResponse{
		TotalProjects:     s.TotalProjects,
		ActiveProjects:    s.ActiveProjects,
		ActiveVendorCount: s.ActiveVendorCount,
		// Every slice is pre-allocated (not left nil) so the JSON response
		// always has `[]`, never `null`, for an empty list — matches the
		// convention used everywhere else in this module (see listProjects
		// etc.) and avoids every frontend consumer needing a null-guard.
		OpenIssues:              make([]dashboardIssueResponse, 0, len(s.OpenIssues)),
		OverdueVendorMilestones: make([]dashboardMilestoneResponse, 0, len(s.OverdueVendorMilestones)),
		IncompletePayments:      make([]dashboardPaymentResponse, 0, len(s.IncompletePayments)),
		IncompleteVenuePayments: make([]dashboardVenuePaymentResponse, 0, len(s.IncompleteVenuePayments)),
		NearDDayProjects:        make([]projectResponse, 0, len(s.NearDDayProjects)),
		LaggingProjects:         make([]laggingProjectResponse, 0, len(s.LaggingProjects)),
		UpcomingProjects:        make([]projectResponse, 0, len(s.UpcomingProjects)),
		RecentActivity:          make([]activityResponse, 0, len(s.RecentActivity)),
		ProjectTrend:            make([]projectTrendPointResponse, 0, len(s.ProjectTrend)),
		RevenueTrend:            make([]revenueTrendPointResponse, 0, len(s.RevenueTrend)),
	}
	for _, row := range s.OpenIssues {
		resp.OpenIssues = append(resp.OpenIssues, dashboardIssueResponse{
			ID: row.Issue.ID, ProjectID: row.ProjectID, ProjectName: row.ProjectName, VendorID: row.VendorID,
			Title: row.Issue.Title, Impact: string(row.Issue.Impact), FoundDate: row.Issue.FoundDate.Format(dateLayout),
			Status: string(row.Issue.Status),
		})
	}
	for _, row := range s.OverdueVendorMilestones {
		resp.OverdueVendorMilestones = append(resp.OverdueVendorMilestones, dashboardMilestoneResponse{
			ID: row.Milestone.ID, ProjectID: row.ProjectID, ProjectName: row.ProjectName, VendorID: row.VendorID,
			Name: row.Milestone.Name, TargetDate: row.Milestone.TargetDate.Format(dateLayout),
		})
	}
	for _, row := range s.IncompletePayments {
		resp.IncompletePayments = append(resp.IncompletePayments, dashboardPaymentResponse{
			ID: row.Payment.ID, ProjectID: row.Payment.ProjectID, ProjectName: row.ProjectName, VendorID: row.VendorID,
			Type: string(row.Payment.Type), Amount: row.Payment.Amount, PaymentDate: row.Payment.PaymentDate.Format(dateLayout),
		})
	}
	for _, row := range s.IncompleteVenuePayments {
		resp.IncompleteVenuePayments = append(resp.IncompleteVenuePayments, dashboardVenuePaymentResponse{
			ID: row.Payment.ID, ProjectID: row.Payment.ProjectID, ProjectName: row.ProjectName,
			Type: string(row.Payment.Type), Amount: row.Payment.Amount, PaymentDate: row.Payment.PaymentDate.Format(dateLayout),
		})
	}
	for _, p := range s.NearDDayProjects {
		resp.NearDDayProjects = append(resp.NearDDayProjects, toProjectResponse(p))
	}
	for _, row := range s.LaggingProjects {
		resp.LaggingProjects = append(resp.LaggingProjects, laggingProjectResponse{Project: toProjectResponse(row.Project), OverallPercent: row.OverallPercent})
	}
	for _, p := range s.UpcomingProjects {
		resp.UpcomingProjects = append(resp.UpcomingProjects, toProjectResponse(p))
	}
	for _, a := range s.RecentActivity {
		resp.RecentActivity = append(resp.RecentActivity, toActivityResponse(a))
	}
	for _, pt := range s.ProjectTrend {
		resp.ProjectTrend = append(resp.ProjectTrend, projectTrendPointResponse{Key: pt.Key, Label: pt.Label, Count: pt.Count})
	}
	for _, pt := range s.RevenueTrend {
		resp.RevenueTrend = append(resp.RevenueTrend, revenueTrendPointResponse{Key: pt.Key, Label: pt.Label, Total: pt.Total})
	}
	resp.Revenue = revenueSummaryResponse{Total: s.Revenue.Total, PreviousTotal: s.Revenue.PreviousTotal, DeltaPercent: s.Revenue.DeltaPercent}
	return resp
}

// clientTimelineResponse flattens ClientTimelineRow's milestone + project
// identity into one row for the Monitoring Timeline page (PLAN.md
// mom-25082026-item-belum item 17) — same flattening convention
// dashboardMilestoneResponse already uses for the vendor-milestone sibling.
type clientTimelineResponse struct {
	ID            int64   `json:"id"`
	Order         int     `json:"order"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	TargetDate    string  `json:"targetDate"`
	CompletedDate *string `json:"completedDate"`
	ProjectID     int64   `json:"projectId"`
	ProjectName   string  `json:"projectName"`
	BrideName     string  `json:"brideName"`
	GroomName     string  `json:"groomName"`
	EventDate     string  `json:"eventDate"`
	// PICStaffID is the project's Wedding Planner (0 = "Belum ditugaskan") --
	// backs the PIC column and WP filter on Monitoring Timeline (Blok D). The
	// name is resolved client-side via /staff/summary, per MODULE_MAP.md.
	PICStaffID int64 `json:"picStaffId"`
	// PICSalesStaffID is the project's Sales PIC (0 = belum ditugaskan) --
	// backs the Sales column and Sales filter on Monitoring Timeline. The
	// name is resolved client-side via /staff/summary, per MODULE_MAP.md.
	PICSalesStaffID int64 `json:"picSalesStaffId"`
}

func toClientTimelineResponse(row domain.ClientTimelineRow) clientTimelineResponse {
	return clientTimelineResponse{
		ID: row.Milestone.ID, Order: row.Milestone.SortOrder, Name: row.Milestone.Name, Status: string(row.Milestone.Status),
		TargetDate: row.Milestone.TargetDate.Format(dateLayout), CompletedDate: formatDatePtr(row.Milestone.CompletedDate),
		ProjectID: row.ProjectID, ProjectName: row.ProjectName, BrideName: row.BrideName, GroomName: row.GroomName,
		EventDate:  row.EventDate.Format(dateLayout),
		PICStaffID: row.PICStaffID,
		PICSalesStaffID: row.PICSalesStaffID,
	}
}

// ClientTimelines is Owner/Admin/Staff (Wedding Planner) — Sales is
// rejected, unlike Dashboard's Owner/Admin-only bar (PLAN.md
// mom-25082026-item-belum item 17, D2): this page's whole point is letting
// a WP monitor Timeline Project items, scoped to their own PIC'd projects
// (DashboardService.ListClientTimelines translates the role for us) rather
// than the tenant-wide aggregation Dashboard itself stays gated behind.
func (h *Handler) ClientTimelines(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireStaff(w, r)
	if !ok {
		return
	}
	if claims.role == "Sales" {
		response.Error(w, http.StatusForbidden, "Sales tidak dapat mengakses monitoring timeline", nil)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
		return
	}
	rows, err := h.dashboard.ListClientTimelines(r.Context(), claims.tenantID, claims.role, claims.staffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]clientTimelineResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, toClientTimelineResponse(row))
	}
	response.OK(w, "ok", result)
}

// Dashboard is Owner/Admin only (confirmed role rule) — it aggregates stats
// across every project/vendor/issue in the whole tenant, which neither a
// Wedding Planner nor Sales (both scoped to only their own PIC'd projects
// everywhere else in this module) must ever see in bulk, even though
// requireStaff alone would let any staff role through.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireStaff(w, r)
	if !ok {
		return
	}
	if claims.role == "Staff" || claims.role == "Sales" {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat mengakses dashboard", nil)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
		return
	}
	upcomingMonth, ok := parseUpcomingMonth(w, r)
	if !ok {
		return
	}
	stats, err := h.dashboard.Get(r.Context(), claims.tenantID, time.Now(), upcomingMonth)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toDashboardResponse(*stats))
}

// parseUpcomingMonth reads the optional ?month=YYYY-MM query param for the
// "Acara Terdekat" filter (PLAN.md mom-25082026-item-belum item 16). Empty
// means nil (default "5 nearest upcoming" behavior). time.Parse rather than
// a regex is deliberate: a "^\d{4}-\d{2}$" pattern alone would accept
// "2027-13", which then matches no project and silently renders an empty
// list instead of a clear error -- time.Parse("2006-01", ...) rejects both
// a malformed shape and an out-of-range month (13) in one call.
func parseUpcomingMonth(w http.ResponseWriter, r *http.Request) (*time.Time, bool) {
	raw := r.URL.Query().Get("month")
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse("2006-01", raw)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Parameter month tidak valid", map[string][]string{"month": {"Gunakan format YYYY-MM"}})
		return nil, false
	}
	return &parsed, true
}
