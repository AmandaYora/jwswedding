package application

import (
	"context"
	"testing"
	"time"

	"jwswedding/internal/modules/projects/domain"
)

// matchesUpcoming is tested directly rather than through DashboardService.Get
// end to end -- Get pulls in ComputeProgressBatch, which alone needs working
// fakes for milestones/vendorEngagements/vendorMilestones/issues/payments/
// venuePayments (see ProjectService.ComputeProgressBatch). Building that
// whole dependency graph just to exercise a two-branch date filter is
// scaffolding disproportionate to what's actually new; matchesUpcoming is
// the real decision logic PLAN.md mom-25082026-item-belum item 16
// introduces, and it's a pure function -- testing it directly gives the
// same confidence with none of that setup.

func TestMatchesUpcoming_NilMonth_PerilakuLama(t *testing.T) {
	asOf := time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC)
	future := time.Date(2027, 6, 20, 0, 0, 0, 0, time.UTC)
	past := time.Date(2027, 6, 10, 0, 0, 0, 0, time.UTC)
	if !matchesUpcoming(future, asOf, nil) {
		t.Error("matchesUpcoming(future, asOf, nil) = false, want true")
	}
	if matchesUpcoming(past, asOf, nil) {
		t.Error("matchesUpcoming(past, asOf, nil) = true, want false (mode default menyaring yang sudah lewat)")
	}
}

func TestMatchesUpcoming_BulanCocok_TermasukYangSudahLewatBulanItu(t *testing.T) {
	asOf := time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC)
	month := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	earlierThisMonth := time.Date(2027, 6, 5, 0, 0, 0, 0, time.UTC)
	// Sengaja menguji bagian yang berbeda dari mode default: tanggal di
	// bulan yang dipilih tapi sudah lewat hari ini tetap harus cocok, karena
	// pengguna secara eksplisit meminta "bulan ini", bukan "sisa hari ini".
	if !matchesUpcoming(earlierThisMonth, asOf, &month) {
		t.Error("matchesUpcoming(earlierThisMonth, asOf, &month) = false, want true -- filter bulan tidak boleh ikut mensyaratkan >= asOf")
	}
}

func TestMatchesUpcoming_BulanTidakCocok_False(t *testing.T) {
	asOf := time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC)
	month := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	otherMonth := time.Date(2027, 7, 5, 0, 0, 0, 0, time.UTC)
	if matchesUpcoming(otherMonth, asOf, &month) {
		t.Error("matchesUpcoming(otherMonth, asOf, &month) = true, want false")
	}
}

func TestMatchesUpcoming_BulanCocokLokasiBeda(t *testing.T) {
	// Regresi kelas timezone yang sama seperti guardKonteksUmum's sameCalendarDate
	// (PLAN.md mom-25082026-item-sebagian) -- eventDate dari database membawa
	// lokasi manapun yang diset DATABASE_URL's loc= (proyek ini: "Local"),
	// sedangkan upcomingMonth hasil parse selalu UTC. matchesUpcoming memakai
	// .Year()/.Month(), yang membaca komponen kalender dari lokasi masing-
	// masing nilai sendiri (sama seperti .Date()), bukan mengonversi ke UTC
	// dulu -- jadi harus tetap cocok meski lokasinya berbeda.
	jakarta := time.FixedZone("WIB", 7*60*60)
	eventDate := time.Date(2027, 6, 5, 0, 0, 0, 0, jakarta)
	month := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC)
	if !matchesUpcoming(eventDate, asOf, &month) {
		t.Error("matchesUpcoming dengan lokasi berbeda = false, want true")
	}
}

// --- ListClientTimelines role scoping (D2) ---

// fakeDashboardRepoForClientTimelines is a minimal in-memory stand-in for
// DashboardRepository -- only ListClientTimelines matters here, and it
// records the picStaffID it was called with so the role->scope translation
// (D2) is something a test verifies, not just prose.
type fakeDashboardRepoForClientTimelines struct {
	calledWithPicStaffID *int64
	calledAtLeastOnce    bool
	// rows, when set, is what ListClientTimelines hands back -- lets a test
	// assert the new PICStaffID column (D1/D2) survives the service layer.
	rows []domain.ClientTimelineRow
}

func (f *fakeDashboardRepoForClientTimelines) CountActiveVendors(ctx context.Context, tenantID int64) (int, error) {
	panic("not implemented")
}
func (f *fakeDashboardRepoForClientTimelines) ListOpenIssues(ctx context.Context, tenantID int64) ([]domain.DashboardIssueRow, error) {
	panic("not implemented")
}
func (f *fakeDashboardRepoForClientTimelines) ListOverdueVendorMilestones(ctx context.Context, tenantID int64, asOf time.Time) ([]domain.DashboardMilestoneRow, error) {
	panic("not implemented")
}
func (f *fakeDashboardRepoForClientTimelines) ListPaymentCandidates(ctx context.Context, tenantID int64) ([]domain.DashboardPaymentRow, error) {
	panic("not implemented")
}
func (f *fakeDashboardRepoForClientTimelines) ListVenuePaymentCandidates(ctx context.Context, tenantID int64) ([]domain.DashboardVenuePaymentRow, error) {
	panic("not implemented")
}
func (f *fakeDashboardRepoForClientTimelines) ListRecentActivity(ctx context.Context, tenantID int64, limit int) ([]domain.ActivityLogEntry, error) {
	panic("not implemented")
}
func (f *fakeDashboardRepoForClientTimelines) ListClientTimelines(ctx context.Context, tenantID int64, picStaffID *int64) ([]domain.ClientTimelineRow, error) {
	f.calledAtLeastOnce = true
	f.calledWithPicStaffID = picStaffID
	return f.rows, nil
}

func TestListClientTimelines_Staff_MeneruskanPicStaffIDNonNil(t *testing.T) {
	repo := &fakeDashboardRepoForClientTimelines{}
	svc := &DashboardService{repo: repo}
	if _, err := svc.ListClientTimelines(context.Background(), 1, "Staff", 42); err != nil {
		t.Fatalf("ListClientTimelines() error = %v", err)
	}
	if repo.calledWithPicStaffID == nil {
		t.Fatal("picStaffID = nil, want &42 untuk callerRole Staff (D2)")
	}
	if *repo.calledWithPicStaffID != 42 {
		t.Errorf("picStaffID = %d, want 42", *repo.calledWithPicStaffID)
	}
}

func TestListClientTimelines_OwnerAdmin_MeneruskanPicStaffIDNil(t *testing.T) {
	for _, role := range []string{"Owner", "Admin"} {
		repo := &fakeDashboardRepoForClientTimelines{}
		svc := &DashboardService{repo: repo}
		if _, err := svc.ListClientTimelines(context.Background(), 1, role, 42); err != nil {
			t.Fatalf("ListClientTimelines() error = %v", err)
		}
		if repo.calledWithPicStaffID != nil {
			t.Errorf("role=%s: picStaffID = %v, want nil (Owner/Admin melihat semua project, D2)", role, *repo.calledWithPicStaffID)
		}
	}
}

// PICStaffID (Blok D) harus utuh sampai pemanggil -- kolom ini yang menyuplai
// kolom PIC dan filter WP di Monitoring Timeline.
func TestListClientTimelines_MengembalikanPICStaffID(t *testing.T) {
	repo := &fakeDashboardRepoForClientTimelines{rows: []domain.ClientTimelineRow{
		{ProjectID: 10, ProjectName: "A", PICStaffID: 42},
		{ProjectID: 11, ProjectName: "B", PICStaffID: 0}, // belum ditugaskan
	}}
	svc := &DashboardService{repo: repo}
	got, err := svc.ListClientTimelines(context.Background(), 1, "Owner", 0)
	if err != nil {
		t.Fatalf("ListClientTimelines() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	if got[0].PICStaffID != 42 || got[1].PICStaffID != 0 {
		t.Errorf("PICStaffID = %d, %d; want 42, 0", got[0].PICStaffID, got[1].PICStaffID)
	}
}
