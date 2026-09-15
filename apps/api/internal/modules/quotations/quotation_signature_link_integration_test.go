package quotations_test

// Tes repo link lawan MySQL nyata: balapan MarkUsed dan pagar stempel
// ReleaseUsed (D14) adalah klausa WHERE — hanya database sungguhan yang bisa
// membuktikan perilakunya. Skip otomatis tanpa MySQL lokal, mengikuti idiom
// berkas integrasi lainnya.

import (
	"context"
	"testing"
	"time"

	"jwswedding/internal/modules/quotations/application"
	qinfra "jwswedding/internal/modules/quotations/infrastructure"
)

func latestLinkID(t *testing.T, f *acceptFixture, qID int64) int64 {
	t.Helper()
	var id int64
	if err := f.db.QueryRow(
		`SELECT id FROM quotation_signature_links WHERE quotation_id = ? ORDER BY id DESC LIMIT 1`, qID).Scan(&id); err != nil {
		t.Fatalf("baca link: %v", err)
	}
	return id
}

func linkUsedAt(t *testing.T, f *acceptFixture, id int64) *time.Time {
	t.Helper()
	// Query mentah supaya yang diperiksa nilai kolomnya, bukan salinan struct.
	var raw *time.Time
	if err := f.db.QueryRow(`SELECT used_at FROM quotation_signature_links WHERE id = ?`, id).Scan(&raw); err != nil {
		t.Fatalf("baca used_at: %v", err)
	}
	return raw
}

func TestSignatureLink_MarkUsed_HanyaSekali(t *testing.T) {
	f := setupAcceptFixture(t)
	ctx := context.Background()
	repo := qinfra.NewMySQLSignatureLinkRepository(f.db)
	svc := application.NewSignatureLinkService(repo, f.quotations)
	qID := seedOfferedUnsigned(t, f, 200_000_000)

	if _, _, err := svc.Issue(ctx, f.tenantID, qID, 1); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	linkID := latestLinkID(t, f, qID)

	// Dua klik Terima bersamaan: tepat satu yang menang.
	firstStamp, firstOK, err := repo.MarkUsed(ctx, linkID)
	if err != nil || !firstOK {
		t.Fatalf("MarkUsed pertama = %v %v, mau menang", firstOK, err)
	}
	if _, secondOK, err := repo.MarkUsed(ctx, linkID); err != nil || secondOK {
		t.Fatalf("MarkUsed kedua = %v %v, mau kalah (409 di service)", secondOK, err)
	}

	// Pagar stempel: kompensasi dengan stempel ORANG LAIN tidak menyentuh
	// baris — pemakaian sah milik pemenang balapan tidak bisa dibatalkan.
	if err := repo.ReleaseUsed(ctx, linkID, firstStamp.Add(time.Hour)); err != nil {
		t.Fatalf("ReleaseUsed stempel asing: %v", err)
	}
	if got := linkUsedAt(t, f, linkID); got == nil {
		t.Fatal("used_at ikut terhapus oleh kompensasi berstempel asing")
	}

	// Stempel sendiri memulihkan.
	if err := repo.ReleaseUsed(ctx, linkID, firstStamp); err != nil {
		t.Fatalf("ReleaseUsed: %v", err)
	}
	if got := linkUsedAt(t, f, linkID); got != nil {
		t.Fatalf("used_at = %v, mau kembali NULL", got)
	}
}

func TestSignatureLink_Issue_MencabutLinkLama_DB(t *testing.T) {
	f := setupAcceptFixture(t)
	ctx := context.Background()
	repo := qinfra.NewMySQLSignatureLinkRepository(f.db)
	svc := application.NewSignatureLinkService(repo, f.quotations)
	qID := seedOfferedUnsigned(t, f, 200_000_000)

	if _, _, err := svc.Issue(ctx, f.tenantID, qID, 1); err != nil {
		t.Fatalf("Issue pertama: %v", err)
	}
	firstID := latestLinkID(t, f, qID)
	if _, _, err := svc.Issue(ctx, f.tenantID, qID, 1); err != nil {
		t.Fatalf("Issue kedua: %v", err)
	}

	var revoked *time.Time
	if err := f.db.QueryRow(`SELECT revoked_at FROM quotation_signature_links WHERE id = ?`, firstID).Scan(&revoked); err != nil {
		t.Fatalf("baca revoked_at: %v", err)
	}
	if revoked == nil {
		t.Error("link lama tidak dicabut saat link baru terbit (D8)")
	}
}
