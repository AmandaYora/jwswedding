package presentation

// Gerbang akses TTD pengguna (PLAN tanda-tangan-pengguna K7/T34).
//
// Yang dikunci di sini adalah keputusan AKSES, bukan alur simpan/baca yang
// sudah diuji di staff/application: seluruh permukaan /api/v1/staff tetap
// Owner-only, KECUALI jalur /staff/me/signature yang terbuka untuk semua role
// staff — dan jalur itu mengambil staffID dari klaim JWT, tidak pernah dari
// URL, sehingga tidak bisa dipakai menyentuh TTD orang lain.

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"jwswedding/internal/modules/staff/application"
	"jwswedding/internal/modules/staff/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
)

const testSecret = "staff-signature-test-secret"

// --- fakes ---

type fakeRepo struct {
	members map[int64]*domain.StaffMember
}

func newFakeRepo(members ...*domain.StaffMember) *fakeRepo {
	m := map[int64]*domain.StaffMember{}
	for _, member := range members {
		m[member.ID] = member
	}
	return &fakeRepo{members: m}
}

func (f *fakeRepo) FindByID(ctx context.Context, tenantID, id int64) (*domain.StaffMember, error) {
	member, ok := f.members[id]
	if !ok || member.TenantID != tenantID {
		return nil, nil
	}
	return member, nil
}

func (f *fakeRepo) UpdateSignature(ctx context.Context, tenantID, id int64, path *string) error {
	member, ok := f.members[id]
	if !ok || member.TenantID != tenantID {
		return apperror.NotFound("pengguna tidak ditemukan")
	}
	member.SignatureStoragePath = path
	return nil
}

func (f *fakeRepo) List(ctx context.Context, tenantID int64) ([]domain.StaffMember, error) {
	return nil, nil
}

func (f *fakeRepo) ListPaginated(ctx context.Context, tenantID int64, params pagination.Params, search, role string) ([]domain.StaffMember, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Create(ctx context.Context, member *domain.StaffMember) error { return nil }
func (f *fakeRepo) Update(ctx context.Context, member *domain.StaffMember) error { return nil }
func (f *fakeRepo) SetActive(ctx context.Context, tenantID, id int64, isActive bool) error {
	return nil
}
func (f *fakeRepo) Delete(ctx context.Context, tenantID, id int64) error { return nil }

type fakeStorage struct{ objects map[string][]byte }

func newFakeStorage() *fakeStorage { return &fakeStorage{objects: map[string][]byte{}} }

func (f *fakeStorage) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	f.objects[key] = data
	return key, nil
}

func (f *fakeStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, apperror.NotFound("objek tidak ada")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (f *fakeStorage) Delete(ctx context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

// --- helpers ---

func staffClaimsFor(role, staffID string) middleware.Claims {
	return middleware.Claims{PrincipalType: "staff", PrincipalID: staffID, TenantID: "1", Role: role}
}

func serveStaff(t *testing.T, h http.HandlerFunc, method, path, body string, claims middleware.Claims) *httptest.ResponseRecorder {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()
	middleware.RequireAuth(testSecret)(h).ServeHTTP(w, r)
	return w
}

// newSignatureTestHandler membangun handler dengan dua pengguna se-tenant:
// id 7 (Staff, punya TTD) dan id 9 (Staff lain, punya TTD juga).
func newSignatureTestHandler(t *testing.T) (*Handler, *fakeRepo) {
	t.Helper()
	pathSeven := "jwswedding/signature/staff/1/7/specimen.png"
	pathNine := "jwswedding/signature/staff/1/9/specimen.png"
	repo := newFakeRepo(
		&domain.StaffMember{ID: 7, TenantID: 1, Name: "Anisa Putri", Title: "Lead Planner", Role: domain.RoleStaff, IsActive: true, SignatureStoragePath: &pathSeven},
		&domain.StaffMember{ID: 9, TenantID: 1, Name: "Rara", Title: "Koordinator", Role: domain.RoleStaff, IsActive: true, SignatureStoragePath: &pathNine},
	)
	store := newFakeStorage()
	store.objects[pathSeven] = []byte("ttd-milik-7")
	store.objects[pathNine] = []byte("ttd-milik-9")
	signatures := application.NewStaffSignatureService(repo, store)
	return NewHandler(application.NewStaffService(repo, nil), signatures), repo
}

// --- gerbang Owner-only pada jalur {id} ---

func TestSignature_JalurIDDitolakUntukNonOwner(t *testing.T) {
	h, _ := newSignatureTestHandler(t)
	for _, role := range []string{"Admin", "Staff", "Sales"} {
		w := serveStaff(t, h.Item, http.MethodGet, "/api/v1/staff/7/signature/image", "", staffClaimsFor(role, "7"))
		if w.Code != http.StatusForbidden {
			t.Errorf("role %s: status = %d, mau 403 di jalur {id}", role, w.Code)
		}
	}
}

func TestSignature_JalurIDDiizinkanUntukOwner(t *testing.T) {
	h, _ := newSignatureTestHandler(t)
	w := serveStaff(t, h.Item, http.MethodGet, "/api/v1/staff/7/signature/image", "", staffClaimsFor("Owner", "1"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), mau 200", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "ttd-milik-7" {
		t.Errorf("body = %q, mau gambar milik staff 7", got)
	}
}

// --- jalur self-service ---

func TestSignature_JalurMeTerbukaUntukSemuaRole(t *testing.T) {
	h, _ := newSignatureTestHandler(t)
	for _, role := range []string{"Owner", "Admin", "Staff", "Sales"} {
		w := serveStaff(t, h.Item, http.MethodGet, "/api/v1/staff/me/signature/image", "", staffClaimsFor(role, "7"))
		if w.Code != http.StatusOK {
			t.Errorf("role %s: status = %d (%s), mau 200 di jalur me", role, w.Code, w.Body.String())
		}
	}
}

// Inti keamanannya: staff 7 memanggil jalur `me`, dan yang keluar HARUS
// TTD-nya sendiri — bukan milik staff 9 — berapa pun angka di URL.
func TestSignature_JalurMeSelaluMemakaiStaffIDDariKlaim(t *testing.T) {
	h, _ := newSignatureTestHandler(t)
	w := serveStaff(t, h.Item, http.MethodGet, "/api/v1/staff/me/signature/image", "", staffClaimsFor("Staff", "7"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), mau 200", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "ttd-milik-7" {
		t.Errorf("body = %q, mau TTD milik pemanggil (staff 7)", got)
	}
}

// Staff 7 tidak boleh bisa membaca TTD staff 9 lewat jalur mana pun.
func TestSignature_StaffTidakBisaMembacaTTDStaffLain(t *testing.T) {
	h, _ := newSignatureTestHandler(t)
	w := serveStaff(t, h.Item, http.MethodGet, "/api/v1/staff/9/signature/image", "", staffClaimsFor("Staff", "7"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, mau 403", w.Code)
	}
	if strings.Contains(w.Body.String(), "ttd-milik-9") {
		t.Error("TTD staff lain bocor di body respons")
	}
}

// Menimpa TTD lewat jalur `me` hanya boleh menyentuh baris pemanggilnya.
func TestSignature_PutJalurMeHanyaMenyentuhBarisSendiri(t *testing.T) {
	h, repo := newSignatureTestHandler(t)
	before := *repo.members[9].SignatureStoragePath

	body := `{"fileName":"ttd.png","mimeType":"image/png","base64Data":"` + validPNGBase64(t) + `"}`
	w := serveStaff(t, h.Item, http.MethodPut, "/api/v1/staff/me/signature", body, staffClaimsFor("Staff", "7"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), mau 200", w.Code, w.Body.String())
	}
	if *repo.members[9].SignatureStoragePath != before {
		t.Error("TTD staff lain ikut berubah lewat jalur me")
	}
	if !strings.Contains(*repo.members[7].SignatureStoragePath, "/1/7/") {
		t.Errorf("TTD pemanggil harus tersimpan di kunci miliknya, got %q", *repo.members[7].SignatureStoragePath)
	}
}

func TestSignature_PutMenolakBase64Rusak(t *testing.T) {
	h, _ := newSignatureTestHandler(t)
	body := `{"fileName":"ttd.png","mimeType":"image/png","base64Data":"!!!bukan-base64!!!"}`
	w := serveStaff(t, h.Item, http.MethodPut, "/api/v1/staff/me/signature", body, staffClaimsFor("Staff", "7"))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d (%s), mau 422", w.Code, w.Body.String())
	}
}

// Principal klien (portal) tidak punya urusan dengan permukaan staff sama
// sekali, termasuk jalur `me`.
func TestSignature_PrincipalClientDitolakDiJalurMe(t *testing.T) {
	h, _ := newSignatureTestHandler(t)
	claims := middleware.Claims{PrincipalType: "client", PrincipalID: "5", TenantID: "1", Role: "Client"}
	w := serveStaff(t, h.Item, http.MethodGet, "/api/v1/staff/me/signature/image", "", claims)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, mau 403 untuk principal client", w.Code)
	}
}

// validPNGBase64 menghasilkan PNG sungguhan ter-base64 — compress.Image
// benar-benar men-decode gambarnya, jadi byte sembarangan tidak membuktikan
// apa pun tentang jalur simpan.
func validPNGBase64(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 30, G: 41, B: 59, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
