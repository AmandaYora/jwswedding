package presentation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jwswedding/internal/modules/quotations/application"
)

// salesStaffId/picStaffId non-numerik WAJIB 400 dengan pesan yang sama
// gayanya dengan clientId (PLAN wording-role-dan-filter-sales-wp T43).
// Parse gagal SEBELUM service disentuh, jadi service boleh ber-repo nil.
func TestList_ParamStaffTidakValid_400(t *testing.T) {
	h := NewHandler(application.NewQuotationService(nil, nil, nil, nil), nil, nil, nil)
	claims := staffClaims{tenantID: 1, staffID: 1, role: "Owner"}

	for _, tc := range []struct {
		name  string
		query string
		pesan string
	}{
		{"sales", "salesStaffId=bukan-angka", "salesStaffId tidak valid"},
		{"pic", "picStaffId=bukan-angka", "picStaffId tidak valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/quotations?"+tc.query, nil)
			rec := httptest.NewRecorder()
			h.list(rec, req, claims)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, mau 400", rec.Code)
			}
			if body := rec.Body.String(); !strings.Contains(body, tc.pesan) {
				t.Errorf("body %q tidak memuat %q", body, tc.pesan)
			}
		})
	}
}
