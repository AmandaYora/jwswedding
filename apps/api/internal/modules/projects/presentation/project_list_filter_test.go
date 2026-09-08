package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/pagination"
)

// D6 (docs/plan/revisi-putri-lanjutan/PLAN.md) is a security gate introduced
// BY the filter feature itself (T-2): without it, a Wedding Planner could
// send ?picStaffId=<someone else's id> and try to widen their own scoping.
// These tests exercise Handler.listProjects end-to-end (through the real
// application.ProjectService, against an in-memory ProjectRepository) rather
// than a unit test of a helper in isolation, since the leak this guards
// against is specifically an HTTP-layer one -- a query param reaching a
// place it must never reach.

// fakeProjectRepoForFilterTest is a tiny in-memory ProjectRepository. It
// replicates just enough of ListPaginated/List's real WHERE semantics
// (tenant scoping + the two PIC slots + the eventMonth range) to prove the
// gate, without a real database.
type fakeProjectRepoForFilterTest struct {
	rows []domain.Project
}

func (f *fakeProjectRepoForFilterTest) matches(p domain.Project, picStaffID, picSalesStaffID *int64, eventMonth *string) bool {
	if picStaffID != nil && p.PICStaffID != *picStaffID {
		return false
	}
	if picSalesStaffID != nil && p.PICSalesStaffID != *picSalesStaffID {
		return false
	}
	if eventMonth != nil && p.EventDate.Format("2006-01") != *eventMonth {
		return false
	}
	return true
}

func (f *fakeProjectRepoForFilterTest) List(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64) ([]domain.Project, error) {
	var out []domain.Project
	for _, p := range f.rows {
		if p.TenantID == tenantID && f.matches(p, picStaffID, picSalesStaffID, nil) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProjectRepoForFilterTest) CountAll(ctx context.Context, tenantID int64) (int64, error) {
	panic("not implemented")
}

func (f *fakeProjectRepoForFilterTest) ListForDashboard(ctx context.Context, tenantID int64, since time.Time) ([]domain.Project, error) {
	panic("not implemented")
}

func (f *fakeProjectRepoForFilterTest) ListPaginated(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64, params pagination.Params, search, status string, showArchived bool, eventMonth *string) ([]domain.Project, int64, error) {
	var out []domain.Project
	for _, p := range f.rows {
		if p.TenantID == tenantID && p.IsArchived == showArchived && f.matches(p, picStaffID, picSalesStaffID, eventMonth) {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeProjectRepoForFilterTest) ListByVenueID(ctx context.Context, tenantID, venueID int64) ([]domain.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectRepoForFilterTest) ListByStaffPIC(ctx context.Context, tenantID, staffID int64) ([]domain.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectRepoForFilterTest) FindByID(ctx context.Context, tenantID, id int64) (*domain.Project, error) {
	panic("not implemented")
}
func (f *fakeProjectRepoForFilterTest) Create(ctx context.Context, p *domain.Project) error {
	panic("not implemented")
}
func (f *fakeProjectRepoForFilterTest) Update(ctx context.Context, p *domain.Project) error {
	panic("not implemented")
}
func (f *fakeProjectRepoForFilterTest) SetStatus(ctx context.Context, tenantID, id int64, status domain.ProjectStatus) error {
	panic("not implemented")
}
func (f *fakeProjectRepoForFilterTest) SetArchived(ctx context.Context, tenantID, id int64, archived bool) error {
	panic("not implemented")
}
func (f *fakeProjectRepoForFilterTest) DeleteCascade(ctx context.Context, tenantID, id int64) error {
	panic("not implemented")
}

// --- Minimal empty-returning sub-repositories, just enough to let
// ComputeProgressBatch run to completion for whatever page of projects the
// tests above return (§8: it's called unconditionally by
// toProjectResponsesWithProgress for any non-empty page). ---

type emptyMilestoneRepoForFilterTest struct{}

func (emptyMilestoneRepoForFilterTest) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectMilestone, error) {
	panic("not implemented")
}
func (emptyMilestoneRepoForFilterTest) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectMilestone, error) {
	return nil, nil
}
func (emptyMilestoneRepoForFilterTest) FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectMilestone, error) {
	panic("not implemented")
}
func (emptyMilestoneRepoForFilterTest) Create(ctx context.Context, m *domain.ProjectMilestone) error {
	panic("not implemented")
}
func (emptyMilestoneRepoForFilterTest) Update(ctx context.Context, m *domain.ProjectMilestone) error {
	panic("not implemented")
}
func (emptyMilestoneRepoForFilterTest) NextSortOrder(ctx context.Context, projectID int64) (int, error) {
	panic("not implemented")
}
func (emptyMilestoneRepoForFilterTest) Reorder(ctx context.Context, projectID int64, orderedIDs []int64) error {
	panic("not implemented")
}

type emptyVendorEngagementRepoForFilterTest struct{}

func (emptyVendorEngagementRepoForFilterTest) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectVendor, error) {
	panic("not implemented")
}
func (emptyVendorEngagementRepoForFilterTest) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectVendor, error) {
	return nil, nil
}
func (emptyVendorEngagementRepoForFilterTest) FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectVendor, error) {
	panic("not implemented")
}
func (emptyVendorEngagementRepoForFilterTest) Create(ctx context.Context, pv *domain.ProjectVendor) error {
	panic("not implemented")
}
func (emptyVendorEngagementRepoForFilterTest) Update(ctx context.Context, pv *domain.ProjectVendor) error {
	panic("not implemented")
}
func (emptyVendorEngagementRepoForFilterTest) SetStatus(ctx context.Context, projectID, id int64, status domain.EngagementStatus) error {
	panic("not implemented")
}
func (emptyVendorEngagementRepoForFilterTest) ListByVendor(ctx context.Context, tenantID, vendorID int64) ([]domain.VendorEngagementHistoryRow, error) {
	panic("not implemented")
}

type emptyVendorMilestoneRepoForFilterTest struct{}

func (emptyVendorMilestoneRepoForFilterTest) ListByProjectVendor(ctx context.Context, projectVendorID int64) ([]domain.VendorMilestone, error) {
	panic("not implemented")
}
func (emptyVendorMilestoneRepoForFilterTest) ListByProjectVendors(ctx context.Context, projectVendorIDs []int64) ([]domain.VendorMilestone, error) {
	return nil, nil
}
func (emptyVendorMilestoneRepoForFilterTest) FindByID(ctx context.Context, projectVendorID, id int64) (*domain.VendorMilestone, error) {
	panic("not implemented")
}
func (emptyVendorMilestoneRepoForFilterTest) Create(ctx context.Context, m *domain.VendorMilestone) error {
	panic("not implemented")
}
func (emptyVendorMilestoneRepoForFilterTest) Update(ctx context.Context, m *domain.VendorMilestone) error {
	panic("not implemented")
}
func (emptyVendorMilestoneRepoForFilterTest) NextSortOrder(ctx context.Context, projectVendorID int64) (int, error) {
	panic("not implemented")
}

type emptyIssueRepoForFilterTest struct{}

func (emptyIssueRepoForFilterTest) ListByProject(ctx context.Context, projectID int64) ([]domain.VendorIssue, error) {
	panic("not implemented")
}
func (emptyIssueRepoForFilterTest) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.VendorIssue, error) {
	return nil, nil
}
func (emptyIssueRepoForFilterTest) FindByID(ctx context.Context, projectID, id int64) (*domain.VendorIssue, error) {
	panic("not implemented")
}
func (emptyIssueRepoForFilterTest) Create(ctx context.Context, issue *domain.VendorIssue) error {
	panic("not implemented")
}
func (emptyIssueRepoForFilterTest) Update(ctx context.Context, issue *domain.VendorIssue) error {
	panic("not implemented")
}

type emptyPaymentRepoForFilterTest struct{}

func (emptyPaymentRepoForFilterTest) ListByProject(ctx context.Context, projectID int64) ([]domain.VendorPayment, error) {
	panic("not implemented")
}
func (emptyPaymentRepoForFilterTest) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.VendorPayment, error) {
	return nil, nil
}
func (emptyPaymentRepoForFilterTest) FindByID(ctx context.Context, projectID, id int64) (*domain.VendorPayment, error) {
	panic("not implemented")
}
func (emptyPaymentRepoForFilterTest) Create(ctx context.Context, p *domain.VendorPayment) error {
	panic("not implemented")
}
func (emptyPaymentRepoForFilterTest) Update(ctx context.Context, projectID, id int64, p domain.VendorPayment) error {
	panic("not implemented")
}
func (emptyPaymentRepoForFilterTest) Delete(ctx context.Context, projectID, id int64) error {
	panic("not implemented")
}

type emptyVenuePaymentRepoForFilterTest struct{}

func (emptyVenuePaymentRepoForFilterTest) ListByProject(ctx context.Context, projectID int64) ([]domain.VenuePayment, error) {
	panic("not implemented")
}
func (emptyVenuePaymentRepoForFilterTest) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.VenuePayment, error) {
	return nil, nil
}
func (emptyVenuePaymentRepoForFilterTest) FindByID(ctx context.Context, projectID, id int64) (*domain.VenuePayment, error) {
	panic("not implemented")
}
func (emptyVenuePaymentRepoForFilterTest) Create(ctx context.Context, p *domain.VenuePayment) error {
	panic("not implemented")
}
func (emptyVenuePaymentRepoForFilterTest) Update(ctx context.Context, projectID, id int64, p domain.VenuePayment) error {
	panic("not implemented")
}
func (emptyVenuePaymentRepoForFilterTest) Delete(ctx context.Context, projectID, id int64) error {
	panic("not implemented")
}

type emptyEvidenceRepoForFilterTest struct{}

func (emptyEvidenceRepoForFilterTest) ListByProject(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (emptyEvidenceRepoForFilterTest) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.Evidence, error) {
	return nil, nil
}
func (emptyEvidenceRepoForFilterTest) ListByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (emptyEvidenceRepoForFilterTest) ListClientVisibleGeneral(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (emptyEvidenceRepoForFilterTest) ListClientVisibleMilestoneDocs(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (emptyEvidenceRepoForFilterTest) FindByID(ctx context.Context, projectID, id int64) (*domain.Evidence, error) {
	panic("not implemented")
}
func (emptyEvidenceRepoForFilterTest) Create(ctx context.Context, e *domain.Evidence) error {
	panic("not implemented")
}
func (emptyEvidenceRepoForFilterTest) SetClientVisible(ctx context.Context, projectID, id int64, visible bool) error {
	panic("not implemented")
}
func (emptyEvidenceRepoForFilterTest) DeleteByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) error {
	panic("not implemented")
}

// filterTestRows is shared by every test below, tenant 1 throughout:
//   - id 1: PIC Staff 10 ("Andi"), event 2026-09-15
//   - id 2: PIC Staff 20 ("Budi"), event 2026-10-01
//   - id 3: PIC Sales 30 ("Citra"), no PIC Staff, event 2026-09-20
//   - id 4: no PIC Staff, no PIC Sales ("Belum ditugaskan" both slots), event 2026-11-05
func filterTestRows() []domain.Project {
	mk := func(id, picStaffID, picSalesStaffID int64, eventDate string) domain.Project {
		d, _ := time.Parse("2006-01-02", eventDate)
		return domain.Project{ID: id, TenantID: 1, Name: "Project", EventDate: d, PrepStartDate: d, Status: domain.StatusPreparation, PICStaffID: picStaffID, PICSalesStaffID: picSalesStaffID}
	}
	return []domain.Project{
		mk(1, 10, 0, "2026-09-15"),
		mk(2, 20, 0, "2026-10-01"),
		mk(3, 0, 30, "2026-09-20"),
		mk(4, 0, 0, "2026-11-05"),
	}
}

func newHandlerForFilterTest(rows []domain.Project) *Handler {
	repo := &fakeProjectRepoForFilterTest{rows: rows}
	evidence := application.NewEvidenceService(emptyEvidenceRepoForFilterTest{}, nil, nil, nil)
	svc := application.NewProjectService(
		repo,
		emptyMilestoneRepoForFilterTest{},
		nil, // milestoneTemplates -- never touched by listProjects
		emptyVendorEngagementRepoForFilterTest{},
		emptyVendorMilestoneRepoForFilterTest{},
		emptyIssueRepoForFilterTest{},
		emptyPaymentRepoForFilterTest{},
		emptyVenuePaymentRepoForFilterTest{},
		evidence,
		nil, // activity -- listProjects never records activity
		nil, // staff (StaffNameResolver) -- PICName is left "" on list rows regardless
	)
	return &Handler{projects: svc}
}

// decodeProjectIDs pulls data[].id out of a (possibly paginated) success envelope.
func decodeProjectIDs(t *testing.T, body string) []int64 {
	t.Helper()
	var env struct {
		Data []projectResponse `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal response: %v (body=%s)", err, body)
	}
	ids := make([]int64, 0, len(env.Data))
	for _, p := range env.Data {
		ids = append(ids, p.ID)
	}
	return ids
}

func sameIDs(got []int64, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[int64]bool, len(got))
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			return false
		}
	}
	return true
}

// Role Staff (WP) sending ?picStaffId=<another staff's id> must still only
// receive their own PIC'd projects -- the query param is never read for this
// role, so there is nothing for it to override (D6/T-2).
func TestListProjects_Staff_QueryParamPicStaffIdDiabaikan(t *testing.T) {
	h := newHandlerForFilterTest(filterTestRows())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?picStaffId=20", nil)
	h.listProjects(rec, req, staffClaims{tenantID: 1, staffID: 10, role: "Staff"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	ids := decodeProjectIDs(t, rec.Body.String())
	if !sameIDs(ids, []int64{1}) {
		t.Errorf("Staff 10 dengan ?picStaffId=20 menerima ids = %v, want [1] -- query param seharusnya diabaikan", ids)
	}
}

// Role Sales sending ?picSalesStaffId=<another sales staff's id> must still
// only receive their own PIC Sales'd projects.
func TestListProjects_Sales_QueryParamPicSalesStaffIdDiabaikan(t *testing.T) {
	h := newHandlerForFilterTest(filterTestRows())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?picSalesStaffId=999", nil)
	h.listProjects(rec, req, staffClaims{tenantID: 1, staffID: 30, role: "Sales"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	ids := decodeProjectIDs(t, rec.Body.String())
	if !sameIDs(ids, []int64{3}) {
		t.Errorf("Sales 30 dengan ?picSalesStaffId=999 menerima ids = %v, want [3] -- query param seharusnya diabaikan", ids)
	}
}

// Role Owner sending ?picStaffId=<id> DOES get narrowed -- the gate is loose
// for Owner/Admin, not dead for everyone.
func TestListProjects_Owner_PicStaffIdMenyempitkan(t *testing.T) {
	h := newHandlerForFilterTest(filterTestRows())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?picStaffId=20", nil)
	h.listProjects(rec, req, staffClaims{tenantID: 1, staffID: 1, role: "Owner"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	ids := decodeProjectIDs(t, rec.Body.String())
	if !sameIDs(ids, []int64{2}) {
		t.Errorf("Owner dengan ?picStaffId=20 menerima ids = %v, want [2]", ids)
	}
}

// Role Owner sending ?picStaffId=0 gets ONLY unassigned projects -- 0 is the
// "Belum ditugaskan" sentinel, a real filter value, not "no filter".
func TestListProjects_Owner_PicStaffIdSentinelNolBelumDitugaskan(t *testing.T) {
	h := newHandlerForFilterTest(filterTestRows())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?picStaffId=0", nil)
	h.listProjects(rec, req, staffClaims{tenantID: 1, staffID: 1, role: "Owner"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	ids := decodeProjectIDs(t, rec.Body.String())
	if !sameIDs(ids, []int64{3, 4}) {
		t.Errorf("Owner dengan ?picStaffId=0 menerima ids = %v, want [3 4] (belum ditugaskan PIC WP)", ids)
	}
}

// eventMonth is NOT part of the ownership gate -- it applies to every role,
// Staff included.
func TestListProjects_Staff_EventMonthTetapMenyaring(t *testing.T) {
	h := newHandlerForFilterTest(filterTestRows())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?eventMonth=2026-09", nil)
	h.listProjects(rec, req, staffClaims{tenantID: 1, staffID: 10, role: "Staff"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	ids := decodeProjectIDs(t, rec.Body.String())
	if !sameIDs(ids, []int64{1}) {
		t.Errorf("Staff 10 dengan ?eventMonth=2026-09 menerima ids = %v, want [1]", ids)
	}
}

// A malformed eventMonth is answered 422, never silently ignored -- ignoring
// it would return every project and look like the filter itself is broken.
func TestListProjects_EventMonthFormatSalah_422(t *testing.T) {
	h := newHandlerForFilterTest(filterTestRows())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?eventMonth=September", nil)
	h.listProjects(rec, req, staffClaims{tenantID: 1, staffID: 1, role: "Owner"})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body=%s)", rec.Code, rec.Body.String())
	}
}

// Regression: with none of the new query params sent at all, every role's
// result is identical to today's behavior -- Staff/Sales still self-scoped,
// Owner/Admin still see the whole tenant.
func TestListProjects_TanpaParameterBaru_RegresiPerRole(t *testing.T) {
	rows := filterTestRows()
	cases := []struct {
		name  string
		claim staffClaims
		want  []int64
	}{
		{"Staff", staffClaims{tenantID: 1, staffID: 10, role: "Staff"}, []int64{1}},
		{"Sales", staffClaims{tenantID: 1, staffID: 30, role: "Sales"}, []int64{3}},
		{"Owner", staffClaims{tenantID: 1, staffID: 1, role: "Owner"}, []int64{1, 2, 3, 4}},
		{"Admin", staffClaims{tenantID: 1, staffID: 2, role: "Admin"}, []int64{1, 2, 3, 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHandlerForFilterTest(rows)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
			h.listProjects(rec, req, tc.claim)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
			}
			ids := decodeProjectIDs(t, rec.Body.String())
			if !sameIDs(ids, tc.want) {
				t.Errorf("role=%s ids = %v, want %v", tc.name, ids, tc.want)
			}
		})
	}
}
