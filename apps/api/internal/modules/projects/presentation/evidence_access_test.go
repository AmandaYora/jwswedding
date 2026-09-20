package presentation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
)

// These are the tests T-4 actually needs: the leak it fixes was in the HTTP
// layer (a `client` principal reaching rows the is_client_visible column was
// created to hide), so asserting on the service alone would not have caught it.
// listEvidence/downloadEvidence/listMilestoneDocuments are exercised directly
// with a hand-built staffClaims rather than through Handler.Item's dispatcher —
// resolveProjectAccess (which produces those claims) is a separate concern
// already covered by its own project-ownership checks, and going through it
// would require standing up ProjectService + ClientAccessResolver fakes just to
// reach the branch under test.

// fakeEvidenceRepoForAccess is a functional EvidenceRepository over an
// in-memory row set — only what the three handlers touch is implemented.
type fakeEvidenceRepoForAccess struct {
	rows []domain.Evidence
}

func (f *fakeEvidenceRepoForAccess) ListByProject(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	return f.rows, nil
}
func (f *fakeEvidenceRepoForAccess) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForAccess) ListByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForAccess) ListClientVisibleGeneral(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForAccess) ListClientVisibleMilestoneDocs(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	var out []domain.Evidence
	for _, e := range f.rows {
		if e.RelatedKind == domain.RelatedProjectMilestone && e.IsClientVisible {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeEvidenceRepoForAccess) FindByID(ctx context.Context, projectID, id int64) (*domain.Evidence, error) {
	for i := range f.rows {
		if f.rows[i].ID == id {
			return &f.rows[i], nil
		}
	}
	return nil, nil
}
func (f *fakeEvidenceRepoForAccess) Create(ctx context.Context, e *domain.Evidence) error {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForAccess) SetClientVisible(ctx context.Context, projectID, id int64, visible bool) error {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForAccess) DeleteByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) error {
	panic("not implemented")
}

type fakeStorageForAccess struct{}

func (f *fakeStorageForAccess) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	panic("not implemented")
}
func (f *fakeStorageForAccess) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("isi berkas")), nil
}
func (f *fakeStorageForAccess) Delete(ctx context.Context, key string) error {
	panic("not implemented")
}

// accessTestRows covers every kind that matters: the two opt-in kinds in both
// visibility states, plus the six kinds that are unconditionally client-visible
// and MUST keep flowing (Client Portal's Kendala/Pembayaran/Vendor tabs).
func accessTestRows() []domain.Evidence {
	return []domain.Evidence{
		{ID: 1, RelatedKind: domain.RelatedGeneral, IsClientVisible: true},
		{ID: 2, RelatedKind: domain.RelatedGeneral, IsClientVisible: false},
		{ID: 3, RelatedKind: domain.RelatedProjectMilestone, IsClientVisible: true},
		{ID: 4, RelatedKind: domain.RelatedProjectMilestone, IsClientVisible: false},
		{ID: 5, RelatedKind: domain.RelatedPayment, IsClientVisible: false},
		{ID: 6, RelatedKind: domain.RelatedClientPayment, IsClientVisible: false},
		{ID: 7, RelatedKind: domain.RelatedVenuePayment, IsClientVisible: false},
		{ID: 8, RelatedKind: domain.RelatedIssue, IsClientVisible: false},
		{ID: 9, RelatedKind: domain.RelatedProjectVendor, IsClientVisible: false},
		{ID: 10, RelatedKind: domain.RelatedVendorMilestone, IsClientVisible: false},
	}
}

func newHandlerForAccessTest() *Handler {
	repo := &fakeEvidenceRepoForAccess{rows: accessTestRows()}
	// activity is nil on purpose: List/Download/ListClientMilestoneDocuments
	// never record activity, so there is nothing to stub.
	return &Handler{evidence: application.NewEvidenceService(repo, &fakeStorageForAccess{}, nil, nil)}
}

// decodeEvidenceIDs pulls the `data[].id` set out of a success envelope.
func decodeEvidenceIDs(t *testing.T, body string) []int64 {
	t.Helper()
	var env struct {
		Data []evidenceResponse `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal response: %v (body=%s)", err, body)
	}
	ids := make([]int64, 0, len(env.Data))
	for _, e := range env.Data {
		ids = append(ids, e.ID)
	}
	return ids
}

func hasID(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestListEvidence_Client_MembuangGeneralDanMilestoneYangTersembunyi(t *testing.T) {
	h := newHandlerForAccessTest()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/evidence", nil)
	h.listEvidence(rec, req, staffClaims{tenantID: 1, principalType: "client"}, 1)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ids := decodeEvidenceIDs(t, rec.Body.String())
	// T-4: baris general (2) dan projectMilestone (4) yang is_client_visible=0
	// tidak boleh sampai ke client.
	if hasID(ids, 2) {
		t.Error("baris general tersembunyi (ID 2) masih terkirim ke client -- ini kebocoran T-4")
	}
	if hasID(ids, 4) {
		t.Error("lampiran timeline tersembunyi (ID 4) masih terkirim ke client")
	}
	// Yang visible tetap lewat.
	if !hasID(ids, 1) || !hasID(ids, 3) {
		t.Errorf("baris yang visible ikut terbuang: ids = %v", ids)
	}
}

// Regresi: perbaikan T-4 tidak boleh mematikan enam kind lain yang memang
// client-visible tanpa syarat -- tab Kendala/Pembayaran/Vendor mengandalkannya.
func TestListEvidence_Client_EnamKindLainTetapLewat(t *testing.T) {
	h := newHandlerForAccessTest()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/evidence", nil)
	h.listEvidence(rec, req, staffClaims{tenantID: 1, principalType: "client"}, 1)

	ids := decodeEvidenceIDs(t, rec.Body.String())
	for _, want := range []int64{5, 6, 7, 8, 9, 10} {
		if !hasID(ids, want) {
			t.Errorf("kind client-visible tanpa syarat (ID %d) terbuang -- regresi tab Kendala/Pembayaran/Vendor", want)
		}
	}
}

func TestListEvidence_Staff_TidakTerpengaruh(t *testing.T) {
	h := newHandlerForAccessTest()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/evidence", nil)
	h.listEvidence(rec, req, staffClaims{tenantID: 1, staffID: 9, role: "Owner", principalType: "staff"}, 1)

	ids := decodeEvidenceIDs(t, rec.Body.String())
	if len(ids) != len(accessTestRows()) {
		t.Errorf("staff menerima %d baris, want %d -- staf tidak boleh ikut tersaring", len(ids), len(accessTestRows()))
	}
}

func TestDownloadEvidence_Client_MilestoneTersembunyi_403(t *testing.T) {
	h := newHandlerForAccessTest()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/evidence/4/file", nil)
	h.downloadEvidence(rec, req, staffClaims{tenantID: 1, principalType: "client"}, 1, "4")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 untuk lampiran timeline yang belum dicentang", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "isi berkas") {
		t.Error("isi berkas ikut terkirim padahal ditolak")
	}
}

func TestDownloadEvidence_Client_GeneralTersembunyi_403(t *testing.T) {
	h := newHandlerForAccessTest()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/evidence/2/file", nil)
	h.downloadEvidence(rec, req, staffClaims{tenantID: 1, principalType: "client"}, 1, "2")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// Regresi tab Dokumen: dokumen general yang sudah dicentang tetap bisa diunduh
// client -- endpoint ini tidak boleh ditutup total untuk client.
func TestDownloadEvidence_Client_GeneralVisible_Berhasil(t *testing.T) {
	h := newHandlerForAccessTest()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/evidence/1/file", nil)
	h.downloadEvidence(rec, req, staffClaims{tenantID: 1, principalType: "client"}, 1, "1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "isi berkas") {
		t.Error("berkas tidak ter-stream padahal boleh diakses")
	}
}

func TestDownloadEvidence_Staff_BarisTersembunyi_Berhasil(t *testing.T) {
	h := newHandlerForAccessTest()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/evidence/4/file", nil)
	h.downloadEvidence(rec, req, staffClaims{tenantID: 1, staffID: 9, role: "Owner", principalType: "staff"}, 1, "4")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 -- staf tidak terpengaruh gerbang client", rec.Code)
	}
}

func TestListMilestoneDocuments_HanyaLampiranVisible(t *testing.T) {
	h := newHandlerForAccessTest()
	// Sama hasilnya untuk kedua tipe prinsipal -- sifat aman endpoint ini ada
	// pada APA yang dikembalikan, bukan pada SIAPA yang boleh memanggil.
	for _, principal := range []string{"client", "staff"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1/milestone-documents", nil)
		h.listMilestoneDocuments(rec, req, 1)

		if rec.Code != http.StatusOK {
			t.Fatalf("principal=%s: status = %d, want 200", principal, rec.Code)
		}
		ids := decodeEvidenceIDs(t, rec.Body.String())
		if len(ids) != 1 || ids[0] != 3 {
			t.Errorf("principal=%s: ids = %v, want hanya [3] (projectMilestone yang visible)", principal, ids)
		}
	}
}

// DeleteByID melengkapi EvidenceRepository setelah jalur "ganti dokumen hasil
// generate" ditambahkan (PLAN rundown-generator D6). Tes di berkas ini tidak
// melewatinya, jadi cukup no-op.
func (f *fakeEvidenceRepoForAccess) DeleteByID(ctx context.Context, projectID, id int64) error { return nil }
