// Package rundowns menyusun modul buku acara — "Panduan Acara Akad &
// Resepsi" yang dipegang tim WO di hari-H.
//
// Bergantung satu arah ke `projects` lewat contracts-nya (data prefill,
// penyaringan Wedding Planner, pengarsipan berkas hasil generate). Arah
// sebaliknya — `projects` membersihkan rundown saat project dihapus permanen —
// dijembatani setter di main.go, bukan import.
package rundowns

import (
	"database/sql"
	"net/http"
	"time"

	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/contracts"
	"jwswedding/internal/modules/rundowns/infrastructure"
	"jwswedding/internal/modules/rundowns/presentation"
	"jwswedding/internal/shared/storage"
)

// pdfTimeout adalah batas satu konversi LibreOffice. Cukup longgar untuk
// dokumen 13 halaman berisi gambar pada VPS kecil, tetapi tetap menjamin
// permintaan yang menggantung akhirnya dilepas.
const pdfTimeout = 60 * time.Second

type Module struct {
	handler   *presentation.Handler
	contracts contracts.Contracts
}

func NewModule(db *sql.DB, projects projectscontracts.Contracts, storageClient *storage.Client) *Module {
	repo := infrastructure.NewMySQLRundownRepository(db)
	templateRepo := infrastructure.NewMySQLRundownTemplateRepository(db)
	service := application.NewRundownService(repo, projects, storageClient, templateRepo)
	templates := application.NewRundownTemplateService(templateRepo, repo)
	converter := infrastructure.NewLibreOfficeConverter("", pdfTimeout)
	return &Module{
		handler:   presentation.NewHandler(service, templates, projects, converter),
		contracts: contracts.New(service),
	}
}

func (m *Module) Contracts() contracts.Contracts {
	return m.contracts
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/rundowns", authed(http.HandlerFunc(m.handler.Collection)))
	mux.Handle("/api/v1/rundowns/", authed(http.HandlerFunc(m.handler.Item)))
}
