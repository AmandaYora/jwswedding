package presentation

import (
	"encoding/json"
	"strings"
	"testing"
)

// /staff/summary WAJIB memuat `role` (dasar penyaringan dropdown per role,
// D6) dan TETAP tidak memuat field sensitif manajemen Pengguna — username,
// email, phone, isActive (PLAN wording-role-dan-filter-sales-wp T45, §5.2).
func TestStaffSummaryResponse_MemuatRoleTanpaFieldSensitif(t *testing.T) {
	raw, err := json.Marshal(staffSummaryResponse{ID: 7, Name: "Rara", Title: "Koordinator", Role: "Staff"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["role"] != "Staff" {
		t.Errorf("role = %v, mau \"Staff\"", decoded["role"])
	}
	for _, sensitif := range []string{"username", "email", "phone", "isActive"} {
		if _, ada := decoded[sensitif]; ada {
			t.Errorf("field sensitif %q ikut terserialisasi", sensitif)
		}
	}
	if !strings.Contains(string(raw), `"title":"Koordinator"`) {
		t.Errorf("title hilang dari respons: %s", raw)
	}
}
