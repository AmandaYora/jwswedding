package application

import (
	"context"
	"io"
	"testing"

	"jwswedding/internal/modules/projects/domain"
)

// fakeGeneratedRepo melayani jalur SaveGeneratedDocument saja: FindByID,
// DeleteByID, dan Create nyata; sisanya diwarisi dari interface yang
// disematkan dan akan panic bila tersentuh.
type fakeGeneratedRepo struct {
	EvidenceRepository
	old     *domain.Evidence
	deleted int64
	created *domain.Evidence
}

func (f *fakeGeneratedRepo) FindByID(_ context.Context, _, id int64) (*domain.Evidence, error) {
	if f.old != nil && f.old.ID == id {
		return f.old, nil
	}
	return nil, nil
}

func (f *fakeGeneratedRepo) DeleteByID(_ context.Context, _, id int64) error {
	f.deleted = id
	return nil
}

func (f *fakeGeneratedRepo) Create(_ context.Context, e *domain.Evidence) error {
	f.created = e
	return nil
}

type noopStorage struct{}

func (noopStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	return key, nil
}
func (noopStorage) Open(context.Context, string) (io.ReadCloser, error) { return nil, nil }
func (noopStorage) Delete(context.Context, string) error                { return nil }

func generatedInput() UploadEvidenceInput {
	return UploadEvidenceInput{
		Name: "Rundown X", Type: domain.EvidenceDocument, FileName: "Rundown - X.pdf",
		MimeType: "application/pdf", RelatedKind: domain.RelatedGeneral,
	}
}

func newGeneratedService(repo *fakeGeneratedRepo) *EvidenceService {
	return NewEvidenceService(repo, noopStorage{}, fakeBuildKey, NewActivityService(&fakeActivityRepoForProject{}))
}

// Rundown yang sudah dibagikan ke klien tidak boleh hilang dari Client Portal
// hanya karena WO generate ulang (PLAN rundown-ux-ideal temuan #2).
func TestSaveGeneratedDocument_InheritsClientVisibility(t *testing.T) {
	repo := &fakeGeneratedRepo{old: &domain.Evidence{ID: 5, RelatedKind: domain.RelatedGeneral, IsClientVisible: true}}
	svc := newGeneratedService(repo)

	e, err := svc.SaveGeneratedDocument(context.Background(), 1, 10, 99, generatedInput(), []byte("%PDF-1.4"), 5)
	if err != nil {
		t.Fatalf("SaveGeneratedDocument() error = %v", err)
	}
	if !e.IsClientVisible || !repo.created.IsClientVisible {
		t.Error("versi baru privat, padahal versi lama terlihat oleh klien")
	}
	if repo.deleted != 5 {
		t.Errorf("dokumen lama tidak dihapus: deleted = %d, mau 5", repo.deleted)
	}
}

func TestSaveGeneratedDocument_InheritsPrivate(t *testing.T) {
	repo := &fakeGeneratedRepo{old: &domain.Evidence{ID: 5, RelatedKind: domain.RelatedGeneral, IsClientVisible: false}}
	e, err := newGeneratedService(repo).SaveGeneratedDocument(context.Background(), 1, 10, 99, generatedInput(), []byte("%PDF-1.4"), 5)
	if err != nil {
		t.Fatalf("SaveGeneratedDocument() error = %v", err)
	}
	if e.IsClientVisible {
		t.Error("versi baru terbuka untuk klien, padahal versi lama privat")
	}
}

// Dokumen lama yang sudah dihapus WO tidak ditemukan: versi baru tetap privat.
func TestSaveGeneratedDocument_NoOldDocStaysPrivate(t *testing.T) {
	repo := &fakeGeneratedRepo{}
	e, err := newGeneratedService(repo).SaveGeneratedDocument(context.Background(), 1, 10, 99, generatedInput(), []byte("%PDF-1.4"), 5)
	if err != nil {
		t.Fatalf("SaveGeneratedDocument() error = %v", err)
	}
	if e.IsClientVisible {
		t.Error("tanpa dokumen lama, versi baru harus privat")
	}
	if repo.deleted != 0 {
		t.Errorf("tidak ada yang boleh dihapus: deleted = %d", repo.deleted)
	}
}
