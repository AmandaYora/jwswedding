package infrastructure

import (
	"context"
	"testing"

	"jwswedding/internal/modules/quotations/application"
	"jwswedding/internal/shared/pagination"
)

// Restrict aktif tapi kosong = filter Wedding Planner yang tidak cocok dengan
// penawaran apa pun — WAJIB menghasilkan daftar kosong dengan total 0, bukan
// seluruh penawaran (bug filter-bocor, PLAN
// wording-role-dan-filter-sales-wp §5.4/T11). Jalur ini hubung-singkat
// sebelum DB disentuh (`IN ()` adalah galat sintaks MySQL), jadi repo boleh
// ber-DB nil.
func TestListByTenant_RestrictAktifTapiKosong_HasilKosong(t *testing.T) {
	repo := NewMySQLQuotationRepository(nil)
	list, total, err := repo.ListByTenant(context.Background(), 1,
		application.QuotationListFilter{RestrictActive: true, RestrictIDs: []int64{}},
		pagination.Params{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListByTenant: error tak terduga: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, mau 0", total)
	}
	if len(list) != 0 {
		t.Errorf("len(list) = %d, mau 0", len(list))
	}
}

// Tanpa RestrictActive, slice kosong berarti "tanpa filter" — perilaku lama
// tetap dipertahankan (tidak boleh ikut hubung-singkat).
func TestListByTenant_TanpaRestrict_TetapMenyentuhDB(t *testing.T) {
	repo := NewMySQLQuotationRepository(nil)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("mau panic (nil DB dipakai), dapat nil — hubung-singkat bocor ke jalur tanpa filter?")
		}
	}()
	_, _, _ = repo.ListByTenant(context.Background(), 1,
		application.QuotationListFilter{},
		pagination.Params{Page: 1, Limit: 10})
}
