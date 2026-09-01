package application

import (
	"context"
	"sort"
	"time"

	"jwswedding/internal/modules/projects/domain"
)

// DashboardRepository holds the tenant-wide (cross-project) queries the
// dashboard needs — these join within the module's own tables (projects +
// its sub-entities), which is allowed; nothing here reaches into another
// module.
type DashboardRepository interface {
	CountActiveVendors(ctx context.Context, tenantID int64) (int, error)
	ListOpenIssues(ctx context.Context, tenantID int64) ([]domain.DashboardIssueRow, error)
	ListOverdueVendorMilestones(ctx context.Context, tenantID int64, asOf time.Time) ([]domain.DashboardMilestoneRow, error)
	ListPaymentCandidates(ctx context.Context, tenantID int64) ([]domain.DashboardPaymentRow, error)
	ListVenuePaymentCandidates(ctx context.Context, tenantID int64) ([]domain.DashboardVenuePaymentRow, error)
	ListRecentActivity(ctx context.Context, tenantID int64, limit int) ([]domain.ActivityLogEntry, error)
	// ListClientTimelines backs the standalone Monitoring Timeline page
	// (PLAN.md mom-25082026-item-belum item 17) -- see the infrastructure
	// implementation's doc comment for picStaffID's nil-means-everyone
	// convention.
	ListClientTimelines(ctx context.Context, tenantID int64, picStaffID *int64) ([]domain.ClientTimelineRow, error)
}

type DashboardService struct {
	projects *ProjectService
	repo     DashboardRepository
	evidence *EvidenceService
}

func NewDashboardService(projects *ProjectService, repo DashboardRepository, evidence *EvidenceService) *DashboardService {
	return &DashboardService{projects: projects, repo: repo, evidence: evidence}
}

const trendMonthsBack = 12

// Get computes the full dashboard aggregation. upcomingMonth is nil for the
// default "5 nearest upcoming events" behavior; a non-nil pointer (already
// parsed by the presentation layer -- see dashboard_endpoint.go) switches
// UpcomingProjects to every open project whose EventDate falls in that exact
// calendar month, unbounded (PLAN.md mom-25082026-item-belum item 16) --
// capping at 5 there would hide events the caller explicitly asked to see.
func (s *DashboardService) Get(ctx context.Context, tenantID int64, asOf time.Time, upcomingMonth *time.Time) (*domain.DashboardStats, error) {
	totalProjects, err := s.projects.CountAll(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Bounded to the trend window (see ListForDashboard's doc comment)
	// instead of List's entire unbounded tenant history — PLAN.md
	// "Performance remediation". trendMonthsBack-1 months back from the
	// current month's start is the earliest point buildProjectTrend/
	// buildRevenueTrend can ever plot.
	trendWindowStart := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(trendMonthsBack - 1), 0)
	allProjects, err := s.projects.ListForDashboard(ctx, tenantID, trendWindowStart)
	if err != nil {
		return nil, err
	}

	stats := &domain.DashboardStats{TotalProjects: int(totalProjects)}

	var upcoming []domain.Project
	var openProjectIDs []int64
	for _, p := range allProjects {
		isOpenProject := p.Status != domain.StatusCompleted && p.Status != domain.StatusCancelled
		if p.Status == domain.StatusPreparation || p.Status == domain.StatusReady {
			stats.ActiveProjects++
		}
		if isOpenProject && domain.IsNearDDay(p.EventDate, asOf) {
			stats.NearDDayProjects = append(stats.NearDDayProjects, p)
		}
		if isOpenProject && matchesUpcoming(p.EventDate, asOf, upcomingMonth) {
			upcoming = append(upcoming, p)
		}
		if isOpenProject {
			openProjectIDs = append(openProjectIDs, p.ID)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].EventDate.Before(upcoming[j].EventDate) })
	// Capped at 5 only for the default "nearest upcoming" mode -- an
	// explicit month filter means the caller wants everything in that month,
	// capping it would hide events they specifically asked for.
	if upcomingMonth == nil && len(upcoming) > 5 {
		upcoming = upcoming[:5]
	}
	stats.UpcomingProjects = upcoming

	// One batched progress computation over every open project instead of a
	// ComputeProgress call per project inside the loop above — PLAN.md
	// "Performance remediation".
	progressByID, err := s.projects.ComputeProgressBatch(ctx, tenantID, openProjectIDs, asOf)
	if err != nil {
		return nil, err
	}
	for _, p := range allProjects {
		isOpenProject := p.Status != domain.StatusCompleted && p.Status != domain.StatusCancelled
		if !isOpenProject {
			continue
		}
		progress := progressByID[p.ID]
		if domain.IsLagging(p.PrepStartDate, p.EventDate, asOf, progress.OverallPercent) {
			stats.LaggingProjects = append(stats.LaggingProjects, domain.LaggingProjectRow{Project: p, OverallPercent: progress.OverallPercent})
		}
	}

	activeVendorCount, err := s.repo.CountActiveVendors(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	stats.ActiveVendorCount = activeVendorCount

	openIssues, err := s.repo.ListOpenIssues(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	stats.OpenIssues = openIssues

	overdue, err := s.repo.ListOverdueVendorMilestones(ctx, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	stats.OverdueVendorMilestones = overdue

	paymentCandidates, err := s.repo.ListPaymentCandidates(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	hasInvoice, hasProof, err := s.evidenceLookup(ctx, paymentProjectIDs(paymentCandidates), domain.RelatedPayment)
	if err != nil {
		return nil, err
	}
	for _, row := range paymentCandidates {
		if !domain.IsPaymentEvidenceComplete(row.Payment.Type, row.Payment.ID, hasInvoice, hasProof) {
			stats.IncompletePayments = append(stats.IncompletePayments, row)
		}
	}

	venuePaymentCandidates, err := s.repo.ListVenuePaymentCandidates(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	vHasInvoice, vHasProof, err := s.evidenceLookup(ctx, venuePaymentProjectIDs(venuePaymentCandidates), domain.RelatedVenuePayment)
	if err != nil {
		return nil, err
	}
	for _, row := range venuePaymentCandidates {
		if !domain.IsPaymentEvidenceComplete(row.Payment.Type, row.Payment.ID, vHasInvoice, vHasProof) {
			stats.IncompleteVenuePayments = append(stats.IncompleteVenuePayments, row)
		}
	}

	recentActivity, err := s.repo.ListRecentActivity(ctx, tenantID, 8)
	if err != nil {
		return nil, err
	}
	stats.RecentActivity = recentActivity

	stats.ProjectTrend = buildProjectTrend(allProjects, asOf, trendMonthsBack)
	stats.RevenueTrend = buildRevenueTrend(allProjects, asOf, trendMonthsBack)
	stats.Revenue = buildRevenueSummary(stats.RevenueTrend)

	return stats, nil
}

// ListClientTimelines backs the standalone Monitoring Timeline page
// (PLAN.md mom-25082026-item-belum item 17). callerRole == "Staff" scopes
// the result to the Wedding Planner's own PIC'd projects (D2, mirroring the
// scoping convention project_endpoints.go's listProjects already uses for
// the Project list); Owner/Admin see every project's timelines. This
// method has no role gate of its own -- Sales is rejected in the
// presentation layer (dashboard_endpoint.go's ClientTimelines handler)
// before this is ever called, same division of responsibility Dashboard's
// own Owner/Admin gate already has.
func (s *DashboardService) ListClientTimelines(ctx context.Context, tenantID int64, callerRole string, callerStaffID int64) ([]domain.ClientTimelineRow, error) {
	var picStaffID *int64
	if callerRole == "Staff" {
		picStaffID = &callerStaffID
	}
	return s.repo.ListClientTimelines(ctx, tenantID, picStaffID)
}

// evidenceLookup fetches evidence per distinct project id (not per payment)
// -- a small, bounded number of calls for what's meant to stay a curated
// dashboard widget, not an exhaustive report -- and cross-references it
// against a single relatedKind. Shared by both the vendor and venue
// incomplete-payments passes in Get() above; `kind` is what keeps the two
// lookups from ever mixing (see PaymentEvidenceStatus's own doc comment).
func (s *DashboardService) evidenceLookup(ctx context.Context, projectIDs []int64, kind domain.EvidenceRelatedKind) (hasInvoice, hasProof map[int64]bool, err error) {
	hasInvoice = make(map[int64]bool)
	hasProof = make(map[int64]bool)
	seen := make(map[int64]bool)
	for _, projectID := range projectIDs {
		if seen[projectID] {
			continue
		}
		seen[projectID] = true
		evidences, err := s.evidence.List(ctx, projectID)
		if err != nil {
			return nil, nil, err
		}
		inv, prf := domain.PaymentEvidenceStatus(evidences, kind)
		for id, v := range inv {
			hasInvoice[id] = v
		}
		for id, v := range prf {
			hasProof[id] = v
		}
	}
	return hasInvoice, hasProof, nil
}

func paymentProjectIDs(rows []domain.DashboardPaymentRow) []int64 {
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.Payment.ProjectID
	}
	return ids
}

func venuePaymentProjectIDs(rows []domain.DashboardVenuePaymentRow) []int64 {
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.Payment.ProjectID
	}
	return ids
}

var monthsID = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func monthKeyLabel(year int, month time.Month) (string, string) {
	key := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
	label := monthsID[month-1] + " " + time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Format("06")
	return key, label
}

func buildProjectTrend(projects []domain.Project, asOf time.Time, monthsBack int) []domain.ProjectTrendPoint {
	points := make([]domain.ProjectTrendPoint, monthsBack)
	index := make(map[string]int, monthsBack)
	cursor := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := monthsBack - 1; i >= 0; i-- {
		m := cursor.AddDate(0, -i, 0)
		key, label := monthKeyLabel(m.Year(), m.Month())
		pointIndex := monthsBack - 1 - i
		points[pointIndex] = domain.ProjectTrendPoint{Key: key, Label: label}
		index[key] = pointIndex
	}
	for _, p := range projects {
		key := p.CreatedAt.Format("2006-01")
		if i, ok := index[key]; ok {
			points[i].Count++
		}
	}
	return points
}

func buildRevenueTrend(projects []domain.Project, asOf time.Time, monthsBack int) []domain.RevenueTrendPoint {
	points := make([]domain.RevenueTrendPoint, monthsBack)
	index := make(map[string]int, monthsBack)
	cursor := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := monthsBack - 1; i >= 0; i-- {
		m := cursor.AddDate(0, -i, 0)
		key, label := monthKeyLabel(m.Year(), m.Month())
		pointIndex := monthsBack - 1 - i
		points[pointIndex] = domain.RevenueTrendPoint{Key: key, Label: label}
		index[key] = pointIndex
	}
	for _, p := range projects {
		if p.Status == domain.StatusCancelled {
			continue
		}
		key := p.EventDate.Format("2006-01")
		if i, ok := index[key]; ok {
			points[i].Total += p.ContractValue
		}
	}
	return points
}

func buildRevenueSummary(revenueTrend []domain.RevenueTrendPoint) domain.RevenueSummary {
	if len(revenueTrend) < 2 {
		return domain.RevenueSummary{}
	}
	previous := revenueTrend[len(revenueTrend)-2]
	current := revenueTrend[len(revenueTrend)-1]
	summary := domain.RevenueSummary{Total: current.Total, PreviousTotal: previous.Total}
	if previous.Total != 0 {
		delta := (float64(current.Total-previous.Total) / float64(previous.Total)) * 100
		summary.DeltaPercent = &delta
	}
	return summary
}

// matchesUpcoming decides whether eventDate belongs in UpcomingProjects.
// upcomingMonth == nil: default mode, same-or-after asOf's calendar day
// (the original "nearest upcoming" rule, unchanged). upcomingMonth != nil:
// pure calendar-month match against eventDate, deliberately NOT also
// requiring same-or-after asOf -- a caller who explicitly picked, say, the
// current month wants every event that month including ones earlier in the
// month than asOf's day, not just the ones still ahead of today.
func matchesUpcoming(eventDate, asOf time.Time, upcomingMonth *time.Time) bool {
	if upcomingMonth == nil {
		return daysBetweenPublic(asOf, eventDate) >= 0
	}
	return eventDate.Year() == upcomingMonth.Year() && eventDate.Month() == upcomingMonth.Month()
}

// daysBetweenPublic avoids exporting the domain package's private helper —
// this file needs the same day-diff, so it's re-derived from stdlib directly.
func daysBetweenPublic(from, to time.Time) int {
	f := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	t := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(t.Sub(f).Hours() / 24)
}
