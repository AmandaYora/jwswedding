package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
)

// Claims disuntikkan lewat JWT sungguhan melalui RequireAuth, bukan dengan
// menambah helper ekspor di `middleware` -- memperlebar API produksi demi tes
// justru membuat jalur injeksi claims bisa dipakai di luar tes.
const testSecret = "rundown-test-secret"

// Handler tests di sini sengaja hanya menguji GERBANG: role, scoping Wedding
// Planner, dan routing. Alur yang menyentuh repository sudah diuji di lapisan
// application; yang tidak boleh lolos dari sini adalah akses yang salah.

type stubProjects struct {
	projectscontracts.Contracts
	picStaffID int64
	picErr     error
	picIDs     []int64
}

func (s *stubProjects) ProjectPICStaffID(_ context.Context, _, _ int64) (int64, error) {
	return s.picStaffID, s.picErr
}

func (s *stubProjects) ProjectIDsForPICStaff(_ context.Context, _, _ int64) ([]int64, error) {
	return s.picIDs, nil
}

type stubConverter struct{ available bool }

func (s stubConverter) Available() bool { return s.available }
func (s stubConverter) ConvertToPDF(context.Context, []byte) ([]byte, error) {
	return []byte("%PDF-1.4"), nil
}

func serve(t *testing.T, h http.HandlerFunc, method, path string, claims middleware.Claims) *httptest.ResponseRecorder {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(w, r)
	return w
}

func staff(role, staffID string) middleware.Claims {
	return middleware.Claims{PrincipalType: "staff", PrincipalID: staffID, TenantID: "1", Role: role}
}

func newTestHandler(projects *stubProjects) *Handler {
	// RundownService tidak dipakai jalur yang diuji di berkas ini: seluruh
	// permintaan berhenti di gerbang role/format sebelum menyentuhnya.
	return NewHandler(nil, projects, stubConverter{available: true})
}

func TestCollection_SalesForbidden(t *testing.T) {
	h := newTestHandler(&stubProjects{})
	w := serve(t, h.Collection, http.MethodGet, "/api/v1/rundowns", staff("Sales", "7"))
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, mau 403 untuk Sales", w.Code)
	}
}

func TestCollection_ClientPrincipalForbidden(t *testing.T) {
	h := newTestHandler(&stubProjects{})
	w := serve(t, h.Collection, http.MethodGet, "/api/v1/rundowns",
		middleware.Claims{PrincipalType: "client", PrincipalID: "3", TenantID: "1"})
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, mau 403 untuk prinsipal client", w.Code)
	}
}

func TestItem_DeleteRequiresOwnerOrAdmin(t *testing.T) {
	h := newTestHandler(&stubProjects{picStaffID: 7})
	// Wedding Planner yang MEMANG PIC project ini pun tidak boleh menghapus.
	w := serve(t, h.Item, http.MethodDelete, "/api/v1/rundowns/5", staff("Staff", "7"))
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, mau 403: Wedding Planner tidak boleh menghapus rundown", w.Code)
	}
}

// stubRepo hanya mengisi satu method; sisanya diwarisi dari interface yang
// disematkan, dan akan panic kalau ada jalur tak terduga memakainya.
type stubRepo struct {
	application.RundownRepository
	used []int64
}

func (s *stubRepo) UsedProjectIDs(context.Context, int64, *[]int64) ([]int64, error) {
	return s.used, nil
}

func (s *stubRepo) List(context.Context, int64, application.ListFilter, pagination.Params) ([]domain.Summary, int, error) {
	return []domain.Summary{}, 0, nil
}

// Segmen literal harus dikenali sebelum ParseInt, kalau tidak endpoint ini
// jatuh ke jalur item dan membalas 404.
func TestItem_UsedProjectIDsIsNotParsedAsID(t *testing.T) {
	svc := application.NewRundownService(&stubRepo{used: []int64{42, 43}}, &stubProjects{}, nil)
	h := NewHandler(svc, &stubProjects{}, stubConverter{available: true})

	w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/used-project-ids", staff("Owner", "1"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 -- used-project-ids ter-parse sebagai {id}", w.Code)
	}
	var body struct {
		Data []int64 `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("respons bukan JSON: %v", err)
	}
	if len(body.Data) != 2 || body.Data[0] != 42 {
		t.Errorf("data = %v, mau [42 43]", body.Data)
	}
}

func TestItem_UnknownGenerateFormatIsRejected(t *testing.T) {
	h := newTestHandler(&stubProjects{})
	w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/5/generate?format=xlsx", staff("Owner", "1"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400 untuk format tidak dikenal", w.Code)
	}
	var body struct {
		Success bool `json:"success"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Success {
		t.Error("respons galat seharusnya success=false")
	}
}

// Tanpa LibreOffice, permintaan PDF dijawab jelas -- bukan 500 dari exec yang
// membingungkan. Dev di Windows umumnya tidak memasangnya.
func TestItem_PDFUnavailableIsExplicit(t *testing.T) {
	h := NewHandler(nil, &stubProjects{}, stubConverter{available: false})
	w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/5/generate?format=pdf", staff("Owner", "1"))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, mau 503 saat konversi PDF tidak tersedia", w.Code)
	}
}

func TestItem_UnknownSubpathIs404(t *testing.T) {
	h := newTestHandler(&stubProjects{})
	w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/5/tidak-ada", staff("Owner", "1"))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, mau 404", w.Code)
	}
}

func TestScopeFor_WeddingPlannerGetsEmptySliceNotNil(t *testing.T) {
	h := newTestHandler(&stubProjects{picIDs: nil})
	scope, err := h.scopeFor(context.Background(), staffClaims{tenantID: 1, staffID: 7, role: "Staff"})
	if err != nil {
		t.Fatalf("scopeFor: %v", err)
	}
	if scope == nil {
		t.Fatal("Wedding Planner tanpa project harus dapat slice KOSONG, bukan nil (nil = tanpa batas)")
	}
	if len(*scope) != 0 {
		t.Errorf("scope = %v, mau kosong", *scope)
	}
}

func TestScopeFor_OwnerIsUnscoped(t *testing.T) {
	h := newTestHandler(&stubProjects{picIDs: []int64{1, 2}})
	scope, err := h.scopeFor(context.Background(), staffClaims{tenantID: 1, staffID: 1, role: "Owner"})
	if err != nil {
		t.Fatalf("scopeFor: %v", err)
	}
	if scope != nil {
		t.Errorf("Owner seharusnya tanpa pembatasan (nil), dapat %v", *scope)
	}
}
