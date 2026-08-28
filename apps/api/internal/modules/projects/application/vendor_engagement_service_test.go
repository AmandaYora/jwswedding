package application

import (
	"context"
	"testing"
	"time"

	"jwswedding/internal/modules/projects/domain"
)

// fakeVendorEngagementRepo is a minimal in-memory stand-in for
// VendorEngagementRepository -- good enough to exercise Update's control
// flow (PLAN.md mom-25082026-item-sebagian §7.1, item 12) without a real
// database. Every method beyond FindByID/Update is left unimplemented since
// Update is the only one under test here.
type fakeVendorEngagementRepo struct {
	engagement *domain.ProjectVendor
	updated    *domain.ProjectVendor
}

func (f *fakeVendorEngagementRepo) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectVendor, error) {
	panic("not implemented")
}
func (f *fakeVendorEngagementRepo) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectVendor, error) {
	panic("not implemented")
}
func (f *fakeVendorEngagementRepo) FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectVendor, error) {
	return f.engagement, nil
}
func (f *fakeVendorEngagementRepo) Create(ctx context.Context, pv *domain.ProjectVendor) error {
	panic("not implemented")
}
func (f *fakeVendorEngagementRepo) Update(ctx context.Context, pv *domain.ProjectVendor) error {
	f.updated = pv
	return nil
}
func (f *fakeVendorEngagementRepo) SetStatus(ctx context.Context, projectID, id int64, status domain.EngagementStatus) error {
	panic("not implemented")
}
func (f *fakeVendorEngagementRepo) ListByVendor(ctx context.Context, tenantID, vendorID int64) ([]domain.VendorEngagementHistoryRow, error) {
	panic("not implemented")
}

func baseEngagement() *domain.ProjectVendor {
	return &domain.ProjectVendor{
		ID: 1, ProjectID: 10, VendorID: 20, CategoryID: 3,
		Scope: "Sewa ballroom + basic lighting rigging", ContractValue: 5_000_000,
		PricingTier: domain.PricingTierAkad, EngagementStatus: domain.EngagementBooked,
		EventDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		DPAmount:  1_000_000, PICStaffID: 5, Notes: "Catatan awal",
	}
}

func baseEngagementInputFor(pv *domain.ProjectVendor) VendorEngagementInput {
	return VendorEngagementInput{
		VendorID: pv.VendorID, CategoryID: pv.CategoryID, Scope: pv.Scope,
		ContractValue: pv.ContractValue, PricingTier: pv.PricingTier, EngagementStatus: pv.EngagementStatus,
		BookingDate: pv.BookingDate, EventDate: pv.EventDate, EventStartTime: pv.EventStartTime, EventEndTime: pv.EventEndTime,
		DPAmount: pv.DPAmount, DueDate: pv.DueDate, PICStaffID: pv.PICStaffID, Notes: pv.Notes,
	}
}

func newVendorEngagementServiceForTest(pv *domain.ProjectVendor) *VendorEngagementService {
	return &VendorEngagementService{
		repo:     &fakeVendorEngagementRepo{engagement: pv},
		activity: NewActivityService(&fakeActivityRepoForProject{}),
	}
}

// A Wedding Planner's form never renders Nilai Kerja Sama at all (PLAN.md
// §12a), so its submission always carries ContractValue's zero value --
// Update must keep the stored figure instead of overwriting it with that
// zero, or an Admin-entered contract value silently disappears the moment a
// WP edits anything else on the same engagement (§12b's whole reason for
// existing).
func TestVendorEngagementUpdate_StaffContractValueZero_NilaiTersimpanTetapUtuh(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.ContractValue = 0
	input.Scope = "Scope diperbarui oleh WP"
	got, err := svc.Update(context.Background(), pv.ProjectID, pv.ID, 99, "Staff", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.ContractValue != 5_000_000 {
		t.Errorf("ContractValue = %d, want tetap 5000000 (nilai Admin tidak boleh hilang)", got.ContractValue)
	}
	if got.Scope != "Scope diperbarui oleh WP" {
		t.Errorf("Scope = %q, want %q (edit lain WP tetap tersimpan)", got.Scope, "Scope diperbarui oleh WP")
	}
}

func TestVendorEngagementUpdate_StaffDPAmountZero_NilaiTersimpanTetapUtuh(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.DPAmount = 0
	got, err := svc.Update(context.Background(), pv.ProjectID, pv.ID, 99, "Sales", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.DPAmount != 1_000_000 {
		t.Errorf("DPAmount = %d, want tetap 1000000 (nilai Admin tidak boleh hilang)", got.DPAmount)
	}
}

func TestVendorEngagementUpdate_Owner_ContractValueBerubah_Tersimpan(t *testing.T) {
	pv := baseEngagement()
	svc := newVendorEngagementServiceForTest(pv)
	input := baseEngagementInputFor(pv)
	input.ContractValue = 7_000_000
	got, err := svc.Update(context.Background(), pv.ProjectID, pv.ID, 99, "Owner", input)
	if err != nil {
		t.Fatalf("Update() error = %v, want success", err)
	}
	if got.ContractValue != 7_000_000 {
		t.Errorf("ContractValue = %d, want 7000000 (Owner boleh mengubah)", got.ContractValue)
	}
}
