package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
)

type memTemplates struct{ t *domain.Template }

func (m *memTemplates) Get(context.Context, int64) (*domain.Template, error) { return m.t, nil }
func (m *memTemplates) ReplaceSection(_ context.Context, tenantID int64, _ domain.SectionKey, p application.SectionPayload) error {
	m.t = &domain.Template{TenantID: tenantID, Roles: p.Roles}
	return nil
}
func (m *memTemplates) ReplaceAll(_ context.Context, t *domain.Template) error {
	m.t = t
	return nil
}

func templateHandler() *Handler {
	return NewHandler(nil, application.NewRundownTemplateService(&memTemplates{}, nil), &stubProjects{},
		stubConverter{available: true})
}

// Template berlaku untuk semua yang boleh membuka Rundown (keputusan D4),
// dan tetap tertutup untuk Sales.
func TestTemplateRoutes_RequireRundownAccess(t *testing.T) {
	h := templateHandler()
	for _, role := range []string{"Owner", "Admin", "Staff"} {
		if w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/template", staff(role, "7")); w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, mau 200", role, w.Code)
		}
	}
	if w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/template", staff("Sales", "7")); w.Code != http.StatusForbidden {
		t.Errorf("Sales: status = %d, mau 403", w.Code)
	}
}

// "template" harus dikenali sebelum ParseInt — kalau tidak, jatuh ke jalur
// {id} dan dijawab 404 "Rundown tidak ditemukan".
func TestTemplateSegmentBeforeParseInt(t *testing.T) {
	w := serve(t, templateHandler().Item, http.MethodGet, "/api/v1/rundowns/template", staff("Owner", "1"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200; body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data templateDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Roles == nil || body.Data.UpdatedAt != nil {
		t.Errorf("template kosong harus berupa array kosong tanpa updatedAt, dapat %+v", body.Data)
	}
}

func TestTemplateRoutes_RejectNonTemplateSection(t *testing.T) {
	w := serve(t, templateHandler().Item, http.MethodPut, "/api/v1/rundowns/template/sections/cover", staff("Owner", "1"))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, mau 422 untuk seksi di luar template", w.Code)
	}
}

// Baris tanpa nomor disimpan "" (dokumen mencetak sel kosong) dan harus
// kembali ke frontend sebagai penanda "-", supaya penyimpanan berikutnya tidak
// diam-diam memberinya nomor.
func TestToAcaraItem_RestoresNoNumberMarker(t *testing.T) {
	if got := toAcaraItem(domain.Item{NoLabel: ""}).NoLabel; got != noNumberMarker {
		t.Errorf("baris tanpa nomor = %q, mau %q", got, noNumberMarker)
	}
	if got := toAcaraItem(domain.Item{NoLabel: "3"}).NoLabel; got != "3" {
		t.Errorf("baris bernomor = %q, mau \"3\"", got)
	}
}
