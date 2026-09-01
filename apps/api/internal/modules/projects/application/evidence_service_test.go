package application

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
)

// fakeEvidenceRepoForUpload is a minimal in-memory stand-in for
// EvidenceRepository -- only Create matters for Upload's own tests.
type fakeEvidenceRepoForUpload struct {
	created *domain.Evidence
}

func (f *fakeEvidenceRepoForUpload) ListByProject(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForUpload) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForUpload) ListByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForUpload) ListClientVisibleGeneral(ctx context.Context, projectID int64) ([]domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForUpload) FindByID(ctx context.Context, projectID, id int64) (*domain.Evidence, error) {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForUpload) Create(ctx context.Context, e *domain.Evidence) error {
	f.created = e
	return nil
}
func (f *fakeEvidenceRepoForUpload) SetClientVisible(ctx context.Context, projectID, id int64, visible bool) error {
	panic("not implemented")
}
func (f *fakeEvidenceRepoForUpload) DeleteByRelated(ctx context.Context, kind domain.EvidenceRelatedKind, relatedID int64) error {
	panic("not implemented")
}

// fakeObjectStorage counts Save calls so a rejected upload can be proven to
// have never reached the storage layer (PLAN.md mom-25082026-item-belum
// item 7's whole point: the allowlist check runs before any storage I/O).
type fakeObjectStorage struct {
	saveCalls int
}

func (f *fakeObjectStorage) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	f.saveCalls++
	return key, nil
}
func (f *fakeObjectStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	panic("not implemented")
}
func (f *fakeObjectStorage) Delete(ctx context.Context, key string) error {
	panic("not implemented")
}

func fakeBuildKey(tenantID, projectID, category, filename string) string {
	return tenantID + "/" + projectID + "/" + category + "/" + filename
}

func newEvidenceServiceForTest() (*EvidenceService, *fakeEvidenceRepoForUpload, *fakeObjectStorage) {
	repo := &fakeEvidenceRepoForUpload{}
	storage := &fakeObjectStorage{}
	svc := NewEvidenceService(repo, storage, fakeBuildKey, NewActivityService(&fakeActivityRepoForProject{}))
	return svc, repo, storage
}

func TestHasForRelated_AdaBaris_True(t *testing.T) {
	repo := &fakeEvidenceRepoForGuard{hasEvidence: true}
	svc := NewEvidenceService(repo, nil, nil, NewActivityService(&fakeActivityRepoForProject{}))
	has, err := svc.HasForRelated(context.Background(), domain.RelatedProjectMilestone, 1)
	if err != nil {
		t.Fatalf("HasForRelated() error = %v", err)
	}
	if !has {
		t.Error("HasForRelated() = false, want true")
	}
}

func TestHasForRelated_Kosong_False(t *testing.T) {
	repo := &fakeEvidenceRepoForGuard{hasEvidence: false}
	svc := NewEvidenceService(repo, nil, nil, NewActivityService(&fakeActivityRepoForProject{}))
	has, err := svc.HasForRelated(context.Background(), domain.RelatedProjectMilestone, 1)
	if err != nil {
		t.Fatalf("HasForRelated() error = %v", err)
	}
	if has {
		t.Error("HasForRelated() = true, want false")
	}
}

func TestUpload_TolakHTML_SebelumMenyentuhStorage(t *testing.T) {
	svc, _, storage := newEvidenceServiceForTest()
	_, err := svc.Upload(context.Background(), 1, 10, 99, UploadEvidenceInput{
		Name: "berkas jahat", FileName: "jahat.html", MimeType: "text/html",
		Base64Data:  base64.StdEncoding.EncodeToString([]byte("<script>alert(1)</script>")),
		RelatedKind: domain.RelatedProjectMilestone, RelatedID: 1,
	})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
		t.Fatalf("Upload() error = %v, want Validation (mimeType)", err)
	}
	if storage.saveCalls != 0 {
		t.Errorf("storage.Save dipanggil %d kali, want 0 -- allowlist harus menolak sebelum storage disentuh", storage.saveCalls)
	}
}

func TestUpload_TolakSVG_SebelumMenyentuhStorage(t *testing.T) {
	svc, _, storage := newEvidenceServiceForTest()
	_, err := svc.Upload(context.Background(), 1, 10, 99, UploadEvidenceInput{
		Name: "logo", FileName: "logo.svg", MimeType: "image/svg+xml",
		Base64Data:  base64.StdEncoding.EncodeToString([]byte("<svg onload=\"alert(1)\"></svg>")),
		RelatedKind: domain.RelatedProjectMilestone, RelatedID: 1,
	})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
		t.Fatalf("Upload() error = %v, want Validation (mimeType)", err)
	}
	if storage.saveCalls != 0 {
		t.Errorf("storage.Save dipanggil %d kali, want 0 -- allowlist harus menolak sebelum storage disentuh", storage.saveCalls)
	}
}

func TestUpload_TerimaPDF(t *testing.T) {
	svc, repo, storage := newEvidenceServiceForTest()
	_, err := svc.Upload(context.Background(), 1, 10, 99, UploadEvidenceInput{
		Name: "Kontrak", FileName: "kontrak.pdf", MimeType: "application/pdf",
		Base64Data:  base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 isi dummy")),
		RelatedKind: domain.RelatedProjectMilestone, RelatedID: 1,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v, want success", err)
	}
	if storage.saveCalls != 1 || repo.created == nil {
		t.Errorf("upload PDF tidak tersimpan: saveCalls=%d created=%v", storage.saveCalls, repo.created)
	}
}

func TestUpload_TerimaPNG(t *testing.T) {
	svc, _, storage := newEvidenceServiceForTest()
	// Byte acak (bukan PNG asli) sengaja dipakai -- compress.Image sudah
	// meneruskan data yang gagal di-decode apa adanya (lihat doc comment
	// recompressPNG), jadi ini menguji penerimaan MIME type, bukan
	// pemrosesan gambar sungguhan.
	_, err := svc.Upload(context.Background(), 1, 10, 99, UploadEvidenceInput{
		Name: "Screenshot", FileName: "ss.png", MimeType: "image/png",
		Base64Data:  base64.StdEncoding.EncodeToString([]byte("bukan png asli")),
		RelatedKind: domain.RelatedProjectMilestone, RelatedID: 1,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v, want success", err)
	}
	if storage.saveCalls != 1 {
		t.Errorf("storage.Save dipanggil %d kali, want 1", storage.saveCalls)
	}
}

func TestUpload_TerimaSpreadsheet(t *testing.T) {
	svc, _, storage := newEvidenceServiceForTest()
	_, err := svc.Upload(context.Background(), 1, 10, 99, UploadEvidenceInput{
		Name: "Rekap Vendor", FileName: "rekap.xlsx",
		MimeType:    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Base64Data:  base64.StdEncoding.EncodeToString([]byte("isi dummy xlsx")),
		RelatedKind: domain.RelatedProjectMilestone, RelatedID: 1,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v, want success", err)
	}
	if storage.saveCalls != 1 {
		t.Errorf("storage.Save dipanggil %d kali, want 1", storage.saveCalls)
	}
}
