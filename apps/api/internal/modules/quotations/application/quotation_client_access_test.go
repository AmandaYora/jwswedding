package application

import (
	"context"
	"errors"
	"testing"

	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/apperror"
)

// fakeContactDirectory memetakan contactID portal ke master client —
// ClientIDForContact yang bisa diatur per tes (sukses, salah pemilik, galat).
type fakeContactDirectory struct {
	fakeClientDirectory
	clientID map[int64]int64
	err      error
}

func (f fakeContactDirectory) ClientIDForContact(_ context.Context, _, contactID int64) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.clientID[contactID], nil
}

func newClientAccessService(projectID int64, dir ClientDirectory) (*QuotationService, *fakeQuotationRepo, *fakeProjectsContracts) {
	repo := newFakeQuotationRepo()
	projects := &fakeProjectsContracts{projectID: projectID}
	svc := NewQuotationService(repo, fakeQuotationTemplates{}, projects, nil)
	if dir != nil {
		svc.SetClientDirectory(dir)
	}
	return svc, repo, projects
}

func seedClientQuotation(repo *fakeQuotationRepo, clientID int64, status domain.QuotationStatus) *domain.Quotation {
	return repo.seed(&domain.Quotation{
		TenantID: 1, ClientID: clientID, Status: status, BasePrice: 50_000_000,
		PackageName: "Silver", PONumber: "PO/202612/0007",
	})
}

func assertClientDenied(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("GetForClientContact seharusnya menolak, tapi lolos")
	}
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindNotFound {
		t.Fatalf("GetForClientContact err = %v, seharusnya NotFound", err)
	}
}

// TestGetForClientContact mengunci gerbang baca portal (PLAN
// revisi-vendor-venue-portal §4.3/F2 + §6): hanya PO asal project yang sudah
// Diterima, dan setiap penolakan adalah NotFound (bukan Forbidden —
// keberadaan dokumen orang lain tidak boleh bocor).
func TestGetForClientContact(t *testing.T) {
	ctx := context.Background()

	t.Run("Diterima + punya project = view kembali", func(t *testing.T) {
		dir := fakeContactDirectory{clientID: map[int64]int64{100: 7}}
		svc, repo, _ := newClientAccessService(9, dir)
		q := seedClientQuotation(repo, 7, domain.QuotationAccepted)
		view, err := svc.GetForClientContact(ctx, 1, 100, q.ID)
		if err != nil {
			t.Fatalf("GetForClientContact: %v", err)
		}
		if view.ProjectID != 9 {
			t.Errorf("view.ProjectID = %d, seharusnya 9", view.ProjectID)
		}
	})

	t.Run("kontak milik client lain = NotFound", func(t *testing.T) {
		dir := fakeContactDirectory{clientID: map[int64]int64{100: 8}}
		svc, repo, _ := newClientAccessService(9, dir)
		q := seedClientQuotation(repo, 7, domain.QuotationAccepted)
		assertClientDenied(t, func() error { _, err := svc.GetForClientContact(ctx, 1, 100, q.ID); return err }())
	})

	t.Run("kontak tak dikenal = NotFound", func(t *testing.T) {
		dir := fakeContactDirectory{clientID: map[int64]int64{}, err: errors.New("kontak tidak ditemukan")}
		svc, repo, _ := newClientAccessService(9, dir)
		q := seedClientQuotation(repo, 7, domain.QuotationAccepted)
		assertClientDenied(t, func() error { _, err := svc.GetForClientContact(ctx, 1, 100, q.ID); return err }())
	})

	t.Run("belum punya project = NotFound", func(t *testing.T) {
		dir := fakeContactDirectory{clientID: map[int64]int64{100: 7}}
		svc, repo, _ := newClientAccessService(0, dir)
		q := seedClientQuotation(repo, 7, domain.QuotationAccepted)
		assertClientDenied(t, func() error { _, err := svc.GetForClientContact(ctx, 1, 100, q.ID); return err }())
	})

	t.Run("status bukan Diterima = NotFound", func(t *testing.T) {
		for _, status := range []domain.QuotationStatus{
			domain.QuotationDraft, domain.QuotationOffered,
			domain.QuotationRejected, domain.QuotationExpired, domain.QuotationCancelled,
		} {
			dir := fakeContactDirectory{clientID: map[int64]int64{100: 7}}
			svc, repo, _ := newClientAccessService(9, dir)
			q := seedClientQuotation(repo, 7, status)
			if _, err := svc.GetForClientContact(ctx, 1, 100, q.ID); err == nil {
				t.Errorf("status %q seharusnya ditolak, tapi lolos", status)
			} else if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindNotFound {
				t.Errorf("status %q err = %v, seharusnya NotFound", status, err)
			}
		}
	})

	t.Run("penawaran tidak ada = NotFound", func(t *testing.T) {
		dir := fakeContactDirectory{clientID: map[int64]int64{100: 7}}
		svc, _, _ := newClientAccessService(9, dir)
		assertClientDenied(t, func() error { _, err := svc.GetForClientContact(ctx, 1, 100, 999); return err }())
	})

	t.Run("direktori belum terpasang = NotFound, bukan panic", func(t *testing.T) {
		svc, repo, _ := newClientAccessService(9, nil)
		q := seedClientQuotation(repo, 7, domain.QuotationAccepted)
		assertClientDenied(t, func() error { _, err := svc.GetForClientContact(ctx, 1, 100, q.ID); return err }())
	})
}
