package presentation

import (
	"context"
	"errors"
	"net/http"
	"testing"

	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
)

// generateRepo cukup untuk satu jalur generate: memuat rundown dan
// aggregatnya, lalu mencatat evidence hasil generate.
type generateRepo struct {
	application.RundownRepository
	recorded int64
}

func (g *generateRepo) FindByID(_ context.Context, tenantID, id int64) (*domain.Rundown, error) {
	return &domain.Rundown{ID: id, TenantID: tenantID, ProjectID: 3, ProjectName: "Dinda & Reza"}, nil
}

func (g *generateRepo) LoadAggregate(_ context.Context, tenantID, id int64) (*domain.View, error) {
	r, _ := g.FindByID(context.Background(), tenantID, id)
	return &domain.View{Rundown: *r}, nil
}

func (g *generateRepo) UpdateEvidenceID(_ context.Context, _, _ int64, _ string, evidenceID int64) error {
	g.recorded = evidenceID
	return nil
}

type generateProjects struct {
	projectscontracts.Contracts
	result projectscontracts.GeneratedDocResult
	err    error
}

func (g *generateProjects) RundownProjectContext(context.Context, int64, int64) (projectscontracts.RundownProjectContext, error) {
	return projectscontracts.RundownProjectContext{ProjectID: 3, ProjectName: "Dinda & Reza"}, nil
}

func (g *generateProjects) SaveGeneratedDocument(context.Context, int64, int64, int64, projectscontracts.GeneratedDocInput) (projectscontracts.GeneratedDocResult, error) {
	return g.result, g.err
}

type busyConverter struct{}

func (busyConverter) Available() bool { return true }
func (busyConverter) ConvertToPDF(context.Context, []byte) ([]byte, error) {
	return nil, context.DeadlineExceeded
}

func generateHandler(projects *generateProjects, converter PDFConverter) (*Handler, *generateRepo) {
	repo := &generateRepo{}
	svc := application.NewRundownService(repo, projects, nil, nil)
	return NewHandler(svc, nil, projects, converter), repo
}

func TestGenerate_ArchiveHeader(t *testing.T) {
	cases := []struct {
		name     string
		projects *generateProjects
		want     string
	}{
		{"terlihat klien", &generateProjects{result: projectscontracts.GeneratedDocResult{EvidenceID: 11, ClientVisible: true}}, "shared"},
		{"privat", &generateProjects{result: projectscontracts.GeneratedDocResult{EvidenceID: 11}}, "private"},
		{"arsip gagal", &generateProjects{err: errors.New("storage mati")}, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := generateHandler(tc.projects, stubConverter{available: true})
			w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/5/generate?format=pdf", staff("Owner", "1"))
			// Arsip gagal tidak boleh menggagalkan unduhan.
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, mau 200; body %s", w.Code, w.Body.String())
			}
			if got := w.Header().Get(archiveHeader); got != tc.want {
				t.Errorf("%s = %q, mau %q", archiveHeader, got, tc.want)
			}
		})
	}
}

func TestGenerate_RecordsEvidenceForNextReplace(t *testing.T) {
	h, repo := generateHandler(&generateProjects{result: projectscontracts.GeneratedDocResult{EvidenceID: 42}},
		stubConverter{available: true})
	serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/5/generate?format=docx", staff("Owner", "1"))
	if repo.recorded != 42 {
		t.Errorf("evidence tercatat = %d, mau 42", repo.recorded)
	}
}

func TestGenerate_ConverterBusyReturns503(t *testing.T) {
	h, _ := generateHandler(&generateProjects{}, busyConverter{})
	w := serve(t, h.Item, http.MethodGet, "/api/v1/rundowns/5/generate?format=pdf", staff("Owner", "1"))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, mau 503 saat antrean PDF melewati anggaran", w.Code)
	}
}
