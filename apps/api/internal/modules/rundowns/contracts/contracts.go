// Package contracts adalah SATU-SATUNYA paket modul `rundowns` yang boleh
// diimpor modul lain.
//
// Isinya sengaja cuma satu method: `projects` perlu membersihkan buku acara
// milik project yang dihapus permanen, dan tidak perlu tahu apa pun lagi
// tentang modul ini. Arah sebaliknya (rundowns -> projects) berjalan lewat
// projects/contracts sebagai argumen konstruktor.
package contracts

import (
	"context"

	"jwswedding/internal/modules/rundowns/application"
)

type Contracts interface {
	// DeleteRundownForProject menghapus buku acara sebuah project beserta
	// denahnya di object storage. Project yang memang belum punya rundown
	// bukan kegagalan — keadaan yang dituju sudah tercapai.
	//
	// Memenuhi projects/application.RundownCleaner secara struktural;
	// `projects` tidak mengimpor paket ini, main.go yang menjembatani.
	DeleteRundownForProject(ctx context.Context, tenantID, projectID int64) error
}

type impl struct {
	rundowns *application.RundownService
}

func New(rundowns *application.RundownService) Contracts {
	return &impl{rundowns: rundowns}
}

func (c *impl) DeleteRundownForProject(ctx context.Context, tenantID, projectID int64) error {
	return c.rundowns.DeleteRundownForProject(ctx, tenantID, projectID)
}
