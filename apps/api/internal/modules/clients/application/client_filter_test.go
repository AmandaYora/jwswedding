package application

import (
	"context"
	"testing"

	projectscontracts "jwswedding/internal/modules/projects/contracts"

	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/pagination"
)

// Stub repo yang merekam filter ListPaginated (pola yang sama dengan
// quotation_filter_test di modul quotations — PLAN
// wording-role-dan-filter-sales-wp T44).
type recordedClientRepo struct {
	ClientRepository
	rows      []domain.Client
	gotFilter ClientListFilter
}

func (f *recordedClientRepo) ListPaginated(_ context.Context, _ int64, _ pagination.Params, filter ClientListFilter) ([]domain.Client, int64, error) {
	f.gotFilter = filter
	return f.rows, int64(len(f.rows)), nil
}

type stubContactRepo struct {
	ClientContactRepository
}

func (stubContactRepo) CountByClients(_ context.Context, _ int64, _ []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}

// Stub contracts yang menghitung pemanggilan ClientIDsForPIC dan
// PICsForClients (T44: PICsForClients WAJIB sekali per halaman, bukan per
// client).
type countingClientProjects struct {
	projectscontracts.Contracts
	forPICCalls int
	forPICIDs   []int64
	picsCalls   int
	pics        map[int64]projectscontracts.ClientPICs
}

func (f *countingClientProjects) ClientIDsForPIC(_ context.Context, _ int64, _, _ int64) ([]int64, error) {
	f.forPICCalls++
	return f.forPICIDs, nil
}

func (f *countingClientProjects) ProjectCountsForClients(_ context.Context, _ int64, _ []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}

func (f *countingClientProjects) PICsForClients(_ context.Context, _ int64, _ []int64) (map[int64]projectscontracts.ClientPICs, error) {
	f.picsCalls++
	return f.pics, nil
}

func newClientFilterTestService(repo *recordedClientRepo, projects *countingClientProjects) *ClientService {
	contacts := NewClientContactService(stubContactRepo{}, repo, nil)
	return NewClientService(repo, contacts, projects, nil, nil)
}

// Salah satu filter terisi WAJIB memanggil ClientIDsForPIC sekali dan
// meneruskan hasilnya sebagai pembatas aktif.
func TestClientListPaginated_FilterPIC_MemanggilContractsSekali(t *testing.T) {
	repo := &recordedClientRepo{rows: []domain.Client{{ID: 9, TenantID: 1, BrideName: "Rara", GroomName: "Dafa"}}}
	projects := &countingClientProjects{
		forPICIDs: []int64{9},
		pics:      map[int64]projectscontracts.ClientPICs{9: {PICStaffIDs: []int64{7}, PICSalesStaffIDs: []int64{3}}},
	}
	svc := newClientFilterTestService(repo, projects)

	items, _, err := svc.ListPaginated(context.Background(), 1, pagination.Params{Page: 1, Limit: 10}, "", 7, 0)
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if projects.forPICCalls != 1 {
		t.Errorf("ClientIDsForPIC dipanggil %d kali, mau 1", projects.forPICCalls)
	}
	if !repo.gotFilter.RestrictActive {
		t.Errorf("RestrictActive = false, mau true")
	}
	if projects.picsCalls != 1 {
		t.Errorf("PICsForClients dipanggil %d kali, mau 1 (satu kali per halaman)", projects.picsCalls)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, mau 1", len(items))
	}
	if len(items[0].PICStaffIDs) != 1 || items[0].PICStaffIDs[0] != 7 {
		t.Errorf("PICStaffIDs = %v, mau [7]", items[0].PICStaffIDs)
	}
	if len(items[0].PICSalesStaffIDs) != 1 || items[0].PICSalesStaffIDs[0] != 3 {
		t.Errorf("PICSalesStaffIDs = %v, mau [3]", items[0].PICSalesStaffIDs)
	}
}

// Tanpa filter = tanpa panggilan ClientIDsForPIC, tapi PICsForClients tetap
// sekali (tampilan nama D7 selalu butuh).
func TestClientListPaginated_TanpaFilter_TidakMemanggilClientIDsForPIC(t *testing.T) {
	repo := &recordedClientRepo{rows: []domain.Client{{ID: 9, TenantID: 1}}}
	projects := &countingClientProjects{pics: map[int64]projectscontracts.ClientPICs{}}
	svc := newClientFilterTestService(repo, projects)

	if _, _, err := svc.ListPaginated(context.Background(), 1, pagination.Params{Page: 1, Limit: 10}, "", 0, 0); err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if projects.forPICCalls != 0 {
		t.Errorf("ClientIDsForPIC dipanggil %d kali, mau 0", projects.forPICCalls)
	}
	if repo.gotFilter.RestrictActive {
		t.Errorf("RestrictActive = true, mau false")
	}
	if projects.picsCalls != 1 {
		t.Errorf("PICsForClients dipanggil %d kali, mau 1", projects.picsCalls)
	}
}
