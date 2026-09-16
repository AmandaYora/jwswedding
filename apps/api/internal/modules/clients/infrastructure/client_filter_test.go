package infrastructure

import (
	"context"
	"testing"

	"jwswedding/internal/modules/clients/application"
	"jwswedding/internal/shared/pagination"
)

// Pembatas ID aktif tapi kosong = filter Sales/WP yang tidak cocok dengan
// client apa pun — WAJIB menghasilkan daftar kosong dengan total 0, bukan
// seluruh client (bug filter-bocor, PLAN
// wording-role-dan-filter-sales-wp §5.5/T15, aturan identik T11). Jalur ini
// hubung-singkat sebelum DB disentuh, jadi repo boleh ber-DB nil.
func TestListPaginated_PembatasAktifTapiKosong_HasilKosong(t *testing.T) {
	repo := NewMySQLClientRepository(nil)
	list, total, err := repo.ListPaginated(context.Background(), 1,
		pagination.Params{Page: 1, Limit: 10},
		application.ClientListFilter{RestrictActive: true, RestrictIDs: []int64{}})
	if err != nil {
		t.Fatalf("ListPaginated: error tak terduga: %v", err)
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
func TestListPaginated_TanpaPembatas_TetapMenyentuhDB(t *testing.T) {
	repo := NewMySQLClientRepository(nil)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("mau panic (nil DB dipakai), dapat nil — hubung-singkat bocor ke jalur tanpa filter?")
		}
	}()
	_, _, _ = repo.ListPaginated(context.Background(), 1,
		pagination.Params{Page: 1, Limit: 10},
		application.ClientListFilter{})
}
