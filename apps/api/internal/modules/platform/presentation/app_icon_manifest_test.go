package presentation

// Tes gerbang PublicAppIcon dan PublicManifest (PLAN
// docs/plan/ikon-homescreen-pwa/PLAN.md T7/T8) — yang dikunci di sini adalah
// KEPUTUSAN AKSES/BENTUK RESPONS, bukan render gambarnya sendiri (sudah
// diuji tuntas di internal/shared/appicon).

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jwswedding/internal/modules/platform/application"
	"jwswedding/internal/modules/platform/domain"
)

// --- fakes ---

// fakeTenantRepo implements application.TenantRepository with just enough
// behavior for GetBrandingByDomain/Get — the only two lookups PublicAppIcon
// and PublicManifest ever trigger.
type fakeTenantRepo struct {
	byID map[int64]*domain.Tenant
}

func (f *fakeTenantRepo) FindByID(ctx context.Context, id int64) (*domain.Tenant, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (f *fakeTenantRepo) FindByDomain(ctx context.Context, host string) (*domain.Tenant, error) {
	for _, t := range f.byID {
		if t.CustomDomain != nil && *t.CustomDomain == host {
			return t, nil
		}
	}
	return nil, nil
}

// Sisa TenantRepository tidak dipakai jalur ikon/manifest — cukup dipenuhi
// agar interface-nya terpuaskan.
func (f *fakeTenantRepo) UpdateSubscription(ctx context.Context, id int64, planID int64, status domain.SubscriptionStatus, expiresAt time.Time) error {
	panic("tidak dipakai jalur ikon/manifest")
}
func (f *fakeTenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	panic("tidak dipakai jalur ikon/manifest")
}
func (f *fakeTenantRepo) UpdateLogo(ctx context.Context, id int64, logoStoragePath *string) error {
	panic("tidak dipakai jalur ikon/manifest")
}

// notFoundErr is a minimal error type for the storage-miss case below —
// distinct from apperror so this test file has zero dependency on that
// package's internals.
type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "objek tidak ditemukan" }

// fakeIconStorage implements application.ObjectStorage — Open mengembalikan
// byte logo yang disimpan, Save tidak pernah dipanggil jalur ini.
type fakeIconStorage struct {
	objects map[string][]byte
}

func (f *fakeIconStorage) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	panic("tidak dipakai jalur ikon/manifest")
}

func (f *fakeIconStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, &notFoundErr{}
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// validLogoPNG membuat PNG sungguhan sebagai isi logo — appicon.Square
// benar-benar men-decode-nya, jadi byte sembarangan gagal tanpa membuktikan
// apa pun.
func validLogoPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 40, B: 60, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png logo: %v", err)
	}
	return buf.Bytes()
}

// newIconTestHandler membangun TenantHandler dengan TenantService sungguhan
// di atas fakes minimal — pendingCharges/billing/charges/buildKey nil-safe
// karena jalur ikon/manifest (Get/GetBrandingByDomain/DownloadLogo/AppIcon)
// tidak pernah menyentuhnya.
func newIconTestHandler(repo *fakeTenantRepo, storage *fakeIconStorage) *TenantHandler {
	svc := application.NewTenantService(repo, nil, nil, nil, storage, nil)
	return NewTenantHandler(svc)
}

func tenantWithDomain(id int64, businessName, customDomain, preset string, logoPath *string) *domain.Tenant {
	domainCopy := customDomain
	return &domain.Tenant{
		ID: id, BusinessName: businessName, BrandColorPreset: preset,
		LogoStoragePath: logoPath, CustomDomain: &domainCopy,
	}
}

func newHostedRequest(method, path, host string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Host = host
	return r
}

// --- PublicAppIcon ---

func TestPublicAppIcon_UkuranDiLuarDaftarDitolak404(t *testing.T) {
	logoKey := "logo/1.png"
	tenant := tenantWithDomain(1, "JWS Wedding", "journey.jwswedding.com", "navy", &logoKey)
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{1: tenant}}
	storage := &fakeIconStorage{objects: map[string][]byte{logoKey: validLogoPNG(t)}}
	h := newIconTestHandler(repo, storage)

	for _, size := range []string{"64", "1024", "0", "-192", "193"} {
		w := httptest.NewRecorder()
		h.PublicAppIcon(w, newHostedRequest(http.MethodGet, "/api/v1/public/app-icon/"+size, "journey.jwswedding.com"))
		if w.Code != http.StatusNotFound {
			t.Errorf("size=%q: status = %d, mau 404", size, w.Code)
		}
	}
}

func TestPublicAppIcon_SegmenBukanAngkaDitolak404(t *testing.T) {
	logoKey := "logo/1.png"
	tenant := tenantWithDomain(1, "JWS Wedding", "journey.jwswedding.com", "navy", &logoKey)
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{1: tenant}}
	storage := &fakeIconStorage{objects: map[string][]byte{logoKey: validLogoPNG(t)}}
	h := newIconTestHandler(repo, storage)

	w := httptest.NewRecorder()
	h.PublicAppIcon(w, newHostedRequest(http.MethodGet, "/api/v1/public/app-icon/besar", "journey.jwswedding.com"))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, mau 404", w.Code)
	}
}

func TestPublicAppIcon_SegmenKosongDitolak404(t *testing.T) {
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{}}
	storage := &fakeIconStorage{objects: map[string][]byte{}}
	h := newIconTestHandler(repo, storage)

	w := httptest.NewRecorder()
	h.PublicAppIcon(w, newHostedRequest(http.MethodGet, "/api/v1/public/app-icon/", "journey.jwswedding.com"))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, mau 404", w.Code)
	}
}

func TestPublicAppIcon_HostTakDikenalDitolak404(t *testing.T) {
	logoKey := "logo/1.png"
	tenant := tenantWithDomain(1, "JWS Wedding", "journey.jwswedding.com", "navy", &logoKey)
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{1: tenant}}
	storage := &fakeIconStorage{objects: map[string][]byte{logoKey: validLogoPNG(t)}}
	h := newIconTestHandler(repo, storage)

	w := httptest.NewRecorder()
	h.PublicAppIcon(w, newHostedRequest(http.MethodGet, "/api/v1/public/app-icon/192", "localhost"))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, mau 404 untuk Host tak dikenal", w.Code)
	}
}

func TestPublicAppIcon_UkuranSahMenghasilkan200PNG(t *testing.T) {
	logoKey := "logo/1.png"
	tenant := tenantWithDomain(1, "JWS Wedding", "journey.jwswedding.com", "navy", &logoKey)
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{1: tenant}}
	storage := &fakeIconStorage{objects: map[string][]byte{logoKey: validLogoPNG(t)}}
	h := newIconTestHandler(repo, storage)

	for _, size := range []string{"180", "192", "512"} {
		w := httptest.NewRecorder()
		h.PublicAppIcon(w, newHostedRequest(http.MethodGet, "/api/v1/public/app-icon/"+size, "journey.jwswedding.com"))
		if w.Code != http.StatusOK {
			t.Fatalf("size=%s: status = %d (%s), mau 200", size, w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/png" {
			t.Errorf("size=%s: Content-Type = %q, mau image/png", size, ct)
		}
		if cc := w.Header().Get("Cache-Control"); cc == "" {
			t.Errorf("size=%s: Cache-Control kosong, mau ada nilainya", size)
		}
		if w.Body.Len() == 0 {
			t.Errorf("size=%s: body kosong", size)
		}
	}
}

// --- PublicManifest ---

func TestPublicManifest_JSONValidDanFieldDasarBenar(t *testing.T) {
	logoKey := "logo/1.png"
	tenant := tenantWithDomain(1, "JWS Wedding", "journey.jwswedding.com", "navy", &logoKey)
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{1: tenant}}
	storage := &fakeIconStorage{objects: map[string][]byte{logoKey: validLogoPNG(t)}}
	h := newIconTestHandler(repo, storage)

	w := httptest.NewRecorder()
	h.PublicManifest(w, newHostedRequest(http.MethodGet, "/manifest.webmanifest", "journey.jwswedding.com"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), mau 200", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("Content-Type = %q, mau application/manifest+json", ct)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body bukan JSON valid: %v\nbody: %s", err, w.Body.String())
	}
	if got["name"] != "JWS Wedding" {
		t.Errorf("name = %v, mau %q", got["name"], "JWS Wedding")
	}
	if got["short_name"] != "JWS Wedding" {
		t.Errorf("short_name = %v, mau %q", got["short_name"], "JWS Wedding")
	}
	if got["start_url"] != "/" {
		t.Errorf("start_url = %v, mau \"/\"", got["start_url"])
	}
	if got["scope"] != "/" {
		t.Errorf("scope = %v, mau \"/\"", got["scope"])
	}
	if got["display"] != "standalone" {
		t.Errorf("display = %v, mau \"standalone\"", got["display"])
	}
	icons, ok := got["icons"].([]interface{})
	if !ok || len(icons) != 2 {
		t.Fatalf("icons = %v, mau 2 entri (tenant punya logo)", got["icons"])
	}
}

func TestPublicManifest_NamaUsahaBertandaKutipTetapJSONValid(t *testing.T) {
	logoKey := "logo/1.png"
	malicious := "Toko \"Bunga\" & Kembang\\Indah"
	tenant := tenantWithDomain(1, malicious, "journey.jwswedding.com", "navy", &logoKey)
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{1: tenant}}
	storage := &fakeIconStorage{objects: map[string][]byte{logoKey: validLogoPNG(t)}}
	h := newIconTestHandler(repo, storage)

	w := httptest.NewRecorder()
	h.PublicManifest(w, newHostedRequest(http.MethodGet, "/manifest.webmanifest", "journey.jwswedding.com"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200", w.Code)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("nama usaha bertanda kutip/backslash merusak JSON: %v\nbody: %s", err, w.Body.String())
	}
	if got["name"] != malicious {
		t.Errorf("name = %v, mau %q (utuh, ter-escape lewat encoding/json)", got["name"], malicious)
	}
}

func TestPublicManifest_TenantTanpaLogoTidakPunyaKunciIcons(t *testing.T) {
	tenant := tenantWithDomain(1, "JWS Wedding", "journey.jwswedding.com", "navy", nil)
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{1: tenant}}
	storage := &fakeIconStorage{objects: map[string][]byte{}}
	h := newIconTestHandler(repo, storage)

	w := httptest.NewRecorder()
	h.PublicManifest(w, newHostedRequest(http.MethodGet, "/manifest.webmanifest", "journey.jwswedding.com"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 walau belum punya logo", w.Code)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body bukan JSON valid: %v", err)
	}
	if _, ok := got["icons"]; ok {
		t.Errorf("kunci \"icons\" tidak boleh ada saat tenant belum punya logo, got: %v", got["icons"])
	}
	if got["name"] != "JWS Wedding" {
		t.Errorf("name tetap harus ada walau tanpa logo, got %v", got["name"])
	}
	if got["display"] != "standalone" {
		t.Errorf("display tetap harus \"standalone\" walau tanpa logo, got %v", got["display"])
	}
}

func TestPublicManifest_HostTakDikenalDitolak404(t *testing.T) {
	repo := &fakeTenantRepo{byID: map[int64]*domain.Tenant{}}
	storage := &fakeIconStorage{objects: map[string][]byte{}}
	h := newIconTestHandler(repo, storage)

	w := httptest.NewRecorder()
	h.PublicManifest(w, newHostedRequest(http.MethodGet, "/manifest.webmanifest", "localhost"))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, mau 404 untuk Host tak dikenal", w.Code)
	}
}
