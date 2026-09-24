package application

import (
	"context"
	"io"

	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/pagination"
)

// ListFilter menyaring daftar rundown.
//
// ProjectIDs bertipe pointer karena nil dan slice kosong berbeda arti, dan
// menyamakan keduanya adalah kebocoran hak akses: nil = tanpa pembatasan
// (Owner/Admin), slice kosong = Wedding Planner yang belum memegang satu
// project pun, sehingga hasilnya WAJIB kosong.
type ListFilter struct {
	ProjectIDs *[]int64
	Search     string
}

// CoverPayload adalah seksi `cover` — seluruhnya kolom di tabel akar.
type CoverPayload struct {
	GroomName       string
	GroomBirthOrder string
	GroomParents    string
	BrideName       string
	BrideBirthOrder string
	BrideParents    string
	EventDateLabel  string
	VenueLabel      string
	EventTimeLabel  string
	CoupleTitle     string
	WOPICName       string
	WOPICPhone      string
}

// DataLainnyaPayload adalah bagian seksi `data-lainnya` yang tinggal di tabel
// akar; baris menunya sendiri ada di SectionPayload.MenuItems.
type DataLainnyaPayload struct {
	SiblingsBride  string
	SiblingsGroom  string
	SouvenirNote   string
	TableClothNote string
}

// SectionPayload adalah isi satu PUT seksi. Hanya field yang relevan dengan
// seksi bersangkutan yang dibaca repository — sisanya diabaikan.
type SectionPayload struct {
	Cover       CoverPayload
	DataLainnya DataLainnyaPayload

	Vendors     []domain.Vendor
	Roles       []domain.Role
	Committees  []domain.Committee
	MenuItems   []domain.MenuItem
	MakeupRooms []domain.MakeupRoom
	Items       []domain.Item
	LayoutNotes []domain.LayoutNote
	PhotoGroups []domain.PhotoGroup
	VIPGuests   []domain.VIPGuest
	Playlist    []domain.PlaylistEntry

	PlaylistNotes string
}

type RundownRepository interface {
	List(ctx context.Context, tenantID int64, filter ListFilter, params pagination.Params) ([]domain.Summary, int, error)
	UsedProjectIDs(ctx context.Context, tenantID int64, scope *[]int64) ([]int64, error)
	Create(ctx context.Context, r *domain.Rundown) error
	FindByID(ctx context.Context, tenantID, id int64) (*domain.Rundown, error)
	FindByProject(ctx context.Context, tenantID, projectID int64) (*domain.Rundown, error)
	LoadAggregate(ctx context.Context, tenantID, id int64) (*domain.View, error)
	// Empat method di bawah ikut membawa tenantID meski pemanggilnya SELALU
	// sudah memuat rundown-nya lewat FindByID yang ter-scope tenant. Bukan
	// pemeriksaan ganda yang sia-sia: `WHERE id = ?` saja membuat satu
	// pemanggil baru yang lupa memuat dulu bisa menulis ke tenant lain tanpa
	// hambatan apa pun, dan biaya menutupnya di sini nol.
	ReplaceSection(ctx context.Context, tenantID, id int64, section domain.SectionKey, payload SectionPayload) error
	UpdateProjectName(ctx context.Context, tenantID, id int64, name string) error
	UpdateEvidenceID(ctx context.Context, tenantID, id int64, format string, evidenceID int64) error
	UpdateLayoutImagePath(ctx context.Context, tenantID, id int64, path string) error
	Delete(ctx context.Context, tenantID, id int64) error
}

// TemplateRepository menyimpan Template Rundown per tenant. Get mengembalikan
// nil tanpa galat bila tenant belum pernah menyimpan template.
type TemplateRepository interface {
	Get(ctx context.Context, tenantID int64) (*domain.Template, error)
	// ReplaceSection mengganti SATU seksi dalam satu transaksi yang mengunci
	// baris template, supaya dua orang yang menyunting seksi berbeda tidak
	// saling menimpa.
	ReplaceSection(ctx context.Context, tenantID int64, section domain.SectionKey, payload SectionPayload) error
	// ReplaceAll mengganti seluruh isi template sekaligus ("Jadikan Template").
	ReplaceAll(ctx context.Context, t *domain.Template) error
}

// ObjectStorage adalah irisan sempit internal/shared/storage.Client yang
// dipakai modul ini — hanya untuk PNG denah akad.
type ObjectStorage interface {
	Save(ctx context.Context, key string, data []byte, contentType string) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
