package application

import (
	"context"
	"strconv"
	"testing"
	"time"

	identitycontracts "jwswedding/internal/modules/identity/contracts"
	projectscontracts "jwswedding/internal/modules/projects/contracts"
	quotationscontracts "jwswedding/internal/modules/quotations/contracts"

	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/pagination"
)

// --- SyncCredentialActive (D4): tabel kebenaran is_active × punya-project. ---

type fakeMasterRepo struct {
	rows map[int64]*domain.Client
}

func (f *fakeMasterRepo) FindByID(_ context.Context, _ int64, id int64) (*domain.Client, error) {
	if c, ok := f.rows[id]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, nil
}
func (f *fakeMasterRepo) FindByIDs(_ context.Context, _ int64, ids []int64) ([]domain.Client, error) {
	var out []domain.Client
	for _, id := range ids {
		if c, ok := f.rows[id]; ok {
			out = append(out, *c)
		}
	}
	return out, nil
}
func (f *fakeMasterRepo) ListPaginated(_ context.Context, _ int64, _ pagination.Params, _ ClientListFilter) ([]domain.Client, int64, error) {
	panic("not implemented")
}
func (f *fakeMasterRepo) Create(_ context.Context, _ *domain.Client) error {
	panic("not implemented")
}
func (f *fakeMasterRepo) Update(_ context.Context, _ *domain.Client) error {
	panic("not implemented")
}
func (f *fakeMasterRepo) Delete(_ context.Context, _ int64, _ int64) error {
	panic("not implemented")
}

type fakeContactRepo struct {
	rows map[int64]*domain.ClientContact
}

func (f *fakeContactRepo) ListByClient(_ context.Context, _, _ int64) ([]domain.ClientContact, error) {
	var out []domain.ClientContact
	for _, c := range f.rows {
		out = append(out, *c)
	}
	return out, nil
}
func (f *fakeContactRepo) CountByClients(_ context.Context, _ int64, _ []int64) (map[int64]int, error) {
	panic("not implemented")
}
func (f *fakeContactRepo) FindByID(_ context.Context, _ int64, id int64) (*domain.ClientContact, error) {
	if c, ok := f.rows[id]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, nil
}
func (f *fakeContactRepo) Create(_ context.Context, _ *domain.ClientContact) error {
	panic("not implemented")
}
func (f *fakeContactRepo) Update(_ context.Context, _ *domain.ClientContact) error {
	panic("not implemented")
}
func (f *fakeContactRepo) SetActive(_ context.Context, _ int64, _ int64, _ bool) error {
	panic("not implemented")
}
func (f *fakeContactRepo) SetCredentialResetAt(_ context.Context, _ int64, _ int64, _ time.Time) error {
	panic("not implemented")
}
func (f *fakeContactRepo) Delete(_ context.Context, _ int64, _ int64) error {
	panic("not implemented")
}
func (f *fakeContactRepo) DeleteForClient(_ context.Context, _ int64, _ int64) error {
	panic("not implemented")
}

type fakeProjectsForSync struct {
	hasProject bool
}

func (f *fakeProjectsForSync) ProjectExists(_ context.Context, _, _ int64) (bool, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ProjectPICStaffID(_ context.Context, _, _ int64) (int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ProjectPICSalesStaffID(_ context.Context, _, _ int64) (int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ListVendorEngagementHistory(_ context.Context, _, _ int64) ([]projectscontracts.VendorEngagementHistoryItem, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ListAffectedProjectsForVendor(_ context.Context, _, _ int64) ([]projectscontracts.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ListAffectedProjectsForVenue(_ context.Context, _, _ int64) ([]projectscontracts.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ListAffectedProjectsForStaff(_ context.Context, _, _ int64) ([]projectscontracts.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) SeedDefaultMilestoneTemplate(_ context.Context, _ int64) error {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ClientHasProject(_ context.Context, _, _ int64) (bool, error) {
	return f.hasProject, nil
}
func (f *fakeProjectsForSync) ProjectIDsForClient(_ context.Context, _, _ int64) ([]int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ProjectCountsForClients(_ context.Context, _ int64, _ []int64) (map[int64]int, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ImpactForClient(_ context.Context, _, _ int64) (projectscontracts.ProjectImpact, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) DeleteProjectsForClient(_ context.Context, _, _ int64) error {
	panic("not implemented")
}
func (f *fakeProjectsForSync) CreateFromQuotation(_ context.Context, _ projectscontracts.CreateFromQuotationInput) (projectscontracts.ProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) SyncFromQuotation(_ context.Context, _, _, _ int64, _ string) error {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ProjectIDForQuotation(_ context.Context, _ int64, _ int64) (int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ClientPaymentLedger(_ context.Context, _, _ int64) ([]projectscontracts.ClientPaymentInfo, int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) DeleteProjectCascade(_ context.Context, _, _ int64) error {
	panic("not implemented")
}
func (f *fakeProjectsForSync) PONumberForQuotation(_ context.Context, _ int64, _ int64) (string, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ProjectDeleteImpact(_ context.Context, _, _ int64) (projectscontracts.ProjectDeleteImpact, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ProjectRefsForQuotations(_ context.Context, _ int64, _ []int64) (map[int64]projectscontracts.QuotationProjectRef, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) QuotationIDsForPICStaff(_ context.Context, _ int64, _ int64) ([]int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) ClientIDsForPIC(_ context.Context, _ int64, _, _ int64) ([]int64, error) {
	panic("not implemented")
}
func (f *fakeProjectsForSync) PICsForClients(_ context.Context, _ int64, _ []int64) (map[int64]projectscontracts.ClientPICs, error) {
	panic("not implemented")
}

type fakeQuotationsForSync struct{}

func (fakeQuotationsForSync) ImpactForClient(_ context.Context, _, _ int64) (quotationscontracts.QuotationImpact, error) {
	panic("not implemented")
}
func (fakeQuotationsForSync) DeleteForClient(_ context.Context, _, _ int64) error {
	panic("not implemented")
}
func (fakeQuotationsForSync) DeleteQuotation(_ context.Context, _, _ int64) error {
	panic("not implemented")
}
func (fakeQuotationsForSync) CompositionForQuotation(_ context.Context, _, _ int64) ([][4]string, error) {
	return nil, nil
}
func (fakeQuotationsForSync) PackageNameForQuotation(_ context.Context, _, _ int64) (string, error) {
	return "", nil
}
func (fakeQuotationsForSync) StatusForQuotation(_ context.Context, _, _ int64) (string, error) {
	return "", nil
}
func (fakeQuotationsForSync) PONumberForQuotation(_ context.Context, _, _ int64) (string, error) {
	panic("not implemented")
}

type fakeIdentityForSync struct {
	active map[string]bool
}

func (f *fakeIdentityForSync) CreateCredential(_ context.Context, _ identitycontracts.CreateCredentialInput) error {
	panic("not implemented")
}
func (f *fakeIdentityForSync) ResetPassword(_ context.Context, _ identitycontracts.PrincipalType, _ string, _ string) error {
	panic("not implemented")
}
func (f *fakeIdentityForSync) ResetPasswordByUsername(_ context.Context, _ string, _ string) error {
	panic("not implemented")
}
func (f *fakeIdentityForSync) SetActive(_ context.Context, _ identitycontracts.PrincipalType, principalID string, isActive bool) error {
	f.active[principalID] = isActive
	return nil
}
func (f *fakeIdentityForSync) UpdateRole(_ context.Context, _ identitycontracts.PrincipalType, _ string, _ string) error {
	panic("not implemented")
}

func TestSyncCredentialActive_TabelKebenaran(t *testing.T) {
	cases := []struct {
		name       string
		isActive   bool
		hasProject bool
		want       bool
	}{
		{"aktif + punya project = menyala", true, true, true},
		{"aktif + belum punya project = mati", true, false, false},
		{"nonaktif + punya project = mati", false, true, false},
		{"nonaktif + belum punya project = mati", false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			contacts := &fakeContactRepo{rows: map[int64]*domain.ClientContact{
				11: {ID: 11, TenantID: 1, ClientID: 7, IsActive: c.isActive},
			}}
			contactSvc := NewClientContactService(contacts, &fakeMasterRepo{rows: map[int64]*domain.Client{
				7: {ID: 7, TenantID: 1},
			}}, &fakeIdentityForSync{active: map[string]bool{}})
			identity := &fakeIdentityForSync{active: map[string]bool{}}
			svc := NewClientService(&fakeMasterRepo{}, contactSvc, &fakeProjectsForSync{hasProject: c.hasProject}, fakeQuotationsForSync{}, identity)

			if err := svc.SyncCredentialActive(context.Background(), 1, 7); err != nil {
				t.Fatalf("SyncCredentialActive: %v", err)
			}
			if got := identity.active[strconv.FormatInt(11, 10)]; got != c.want {
				t.Errorf("kredensial kontak 11 = %v, mau %v", got, c.want)
			}
		})
	}
}

// Tiga pelengkap projects.Contracts setelah modul `rundowns` menambahkannya
// (PLAN rundown-generator §8.1). Tes di berkas ini tidak melewatinya.
func (f *fakeProjectsForSync) RundownProjectContext(ctx context.Context, tenantID, projectID int64) (projectscontracts.RundownProjectContext, error) {
	return projectscontracts.RundownProjectContext{}, nil
}

func (f *fakeProjectsForSync) ProjectIDsForPICStaff(ctx context.Context, tenantID, picStaffID int64) ([]int64, error) {
	return nil, nil
}

func (f *fakeProjectsForSync) SaveGeneratedDocument(ctx context.Context, tenantID, projectID, actorStaffID int64, in projectscontracts.GeneratedDocInput) (projectscontracts.GeneratedDocResult, error) {
	return projectscontracts.GeneratedDocResult{}, nil
}
