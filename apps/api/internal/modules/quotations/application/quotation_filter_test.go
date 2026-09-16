package application

import (
	"context"
	"testing"

	projectscontracts "jwswedding/internal/modules/projects/contracts"

	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/pagination"
)

// Stub repo yang merekam filter ListByTenant — membuktikan pemisahan
// RestrictActive dari RestrictIDs sampai ke repository (PLAN
// wording-role-dan-filter-sales-wp T42). Interface di-embed supaya hanya
// metode yang dipakai jalur ListPaginated yang perlu didefinisikan.
type recordedFilterRepo struct {
	QuotationRepository
	rows      []domain.Quotation
	gotFilter QuotationListFilter
}

func (f *recordedFilterRepo) ListByTenant(_ context.Context, _ int64, filter QuotationListFilter, _ pagination.Params) ([]domain.Quotation, int64, error) {
	f.gotFilter = filter
	return f.rows, int64(len(f.rows)), nil
}

func (f *recordedFilterRepo) AdjustmentTotals(_ context.Context, _ []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

// Stub contracts yang menghitung pemanggilan QuotationIDsForPICStaff (T42).
type countingProjectsContracts struct {
	projectscontracts.Contracts
	restrictCalls int
	restrictIDs   []int64
	refs          map[int64]projectscontracts.QuotationProjectRef
}

func (f *countingProjectsContracts) QuotationIDsForPICStaff(_ context.Context, _ int64, _ int64) ([]int64, error) {
	f.restrictCalls++
	return f.restrictIDs, nil
}

func (f *countingProjectsContracts) ProjectRefsForQuotations(_ context.Context, _ int64, ids []int64) (map[int64]projectscontracts.QuotationProjectRef, error) {
	out := make(map[int64]projectscontracts.QuotationProjectRef, len(ids))
	for _, id := range ids {
		if r, ok := f.refs[id]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func newFilterTestService(repo *recordedFilterRepo, projects *countingProjectsContracts) *QuotationService {
	svc := NewQuotationService(repo, nil, projects, nil)
	return svc
}

// picStaffID != 0 WAJIB memanggil QuotationIDsForPICStaff tepat sekali dan
// meneruskan hasilnya sebagai pembatas aktif (sebelum paginasi, supaya
// meta.total tetap benar).
func TestListPaginated_FilterWP_MemanggilContractsSekali(t *testing.T) {
	repo := &recordedFilterRepo{rows: []domain.Quotation{{ID: 11, ClientID: 5, CreatedByStaffID: 3}}}
	projects := &countingProjectsContracts{restrictIDs: []int64{11}}
	svc := newFilterTestService(repo, projects)

	items, total, err := svc.ListPaginated(context.Background(), 1, "", "", 0, 0, 7, pagination.Params{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if projects.restrictCalls != 1 {
		t.Errorf("QuotationIDsForPICStaff dipanggil %d kali, mau 1", projects.restrictCalls)
	}
	if !repo.gotFilter.RestrictActive {
		t.Errorf("RestrictActive = false, mau true")
	}
	if len(repo.gotFilter.RestrictIDs) != 1 || repo.gotFilter.RestrictIDs[0] != 11 {
		t.Errorf("RestrictIDs = %v, mau [11]", repo.gotFilter.RestrictIDs)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("items/total = %d/%d, mau 1/1", len(items), total)
	}
	// SalesStaffID berasal dari pembuat penawaran (D3), PICStaffID dari
	// project hasil Accept (D7).
	if items[0].SalesStaffID != 3 {
		t.Errorf("SalesStaffID = %d, mau 3 (CreatedByStaffID)", items[0].SalesStaffID)
	}
}

// picStaffID == 0 TIDAK BOLEH memanggil contracts sama sekali (tanpa filter
// = tanpa query tambahan).
func TestListPaginated_TanpaFilterWP_TidakMemanggilContracts(t *testing.T) {
	repo := &recordedFilterRepo{rows: []domain.Quotation{{ID: 11, ClientID: 5, CreatedByStaffID: 3}}}
	projects := &countingProjectsContracts{refs: map[int64]projectscontracts.QuotationProjectRef{11: {ProjectID: 99, PICStaffID: 7}}}
	svc := newFilterTestService(repo, projects)

	items, _, err := svc.ListPaginated(context.Background(), 1, "", "", 0, 0, 0, pagination.Params{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if projects.restrictCalls != 0 {
		t.Errorf("QuotationIDsForPICStaff dipanggil %d kali, mau 0", projects.restrictCalls)
	}
	if repo.gotFilter.RestrictActive {
		t.Errorf("RestrictActive = true, mau false")
	}
	if len(items) != 1 || items[0].ProjectID != 99 || items[0].PICStaffID != 7 {
		t.Errorf("items = %+v, mau ProjectID=99 PICStaffID=7 dari ProjectRefs", items)
	}
}
