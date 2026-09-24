package infrastructure_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"jwswedding/internal/migrator"
	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/modules/rundowns/infrastructure"
	"jwswedding/internal/shared/database"
)

// Tes Template Rundown lawan MySQL nyata (PLAN rundown-ux-ideal B18): kolom
// JSON, INSERT IGNORE + FOR UPDATE, dan ON DUPLICATE KEY UPDATE tidak bisa
// dikunci oleh tes unit. Dilewati bila MySQL lokal tidak tersedia — pola yang
// sama dengan tes integrasi di modul quotations.
const (
	templateTestAdminDSN = "root:@tcp(127.0.0.1:3306)/"
	templateTestDB       = "jwswedding_rundown_template_test"
)

func setupTemplateRepo(t *testing.T) *infrastructure.MySQLRundownTemplateRepository {
	t.Helper()
	admin, err := sql.Open("mysql", templateTestAdminDSN)
	if err != nil {
		t.Skipf("driver mysql tidak bisa dibuka, skip: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + templateTestDB); err != nil {
		t.Fatalf("drop db uji: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + templateTestDB); err != nil {
		t.Fatalf("create db uji: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", templateTestAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + templateTestDB)
	})

	url := fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", templateTestDB)
	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	db, err := database.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return infrastructure.NewMySQLRundownTemplateRepository(db)
}

func TestTemplateRepository_Integration(t *testing.T) {
	repo := setupTemplateRepo(t)
	ctx := context.Background()

	t.Run("belum ada baris", func(t *testing.T) {
		got, err := repo.Get(ctx, 1)
		if err != nil || got != nil {
			t.Fatalf("Get = %+v, %v; mau nil, nil", got, err)
		}
	})

	t.Run("dua seksi berbeda disimpan bersamaan, keduanya bertahan", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs <- repo.ReplaceSection(ctx, 1, domain.SectionKeyRoles,
				application.SectionPayload{Roles: []domain.Role{{RoleLabel: "Saksi", Note: "dua orang"}}})
		}()
		go func() {
			defer wg.Done()
			errs <- repo.ReplaceSection(ctx, 1, domain.SectionKeyAcaraAkad,
				application.SectionPayload{Items: []domain.Item{{NoLabel: "", Item: "Checking dekor"}, {NoLabel: "1", Item: "Akad"}}})
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("ReplaceSection: %v", err)
			}
		}

		got, err := repo.Get(ctx, 1)
		if err != nil || got == nil {
			t.Fatalf("Get: %+v, %v", got, err)
		}
		if len(got.Roles) != 1 || got.Roles[0].RoleLabel != "Saksi" || got.Roles[0].Note != "dua orang" {
			t.Errorf("seksi List Nama hilang atau berubah: %+v", got.Roles)
		}
		if len(got.ItemsAkad) != 2 || got.ItemsAkad[0].NoLabel != "" || got.ItemsAkad[1].NoLabel != "1" {
			t.Errorf("susunan akad hilang atau berubah: %+v", got.ItemsAkad)
		}
		if got.ItemsAkad[0].Section != domain.SectionAkad {
			t.Errorf("section = %q, mau Akad", got.ItemsAkad[0].Section)
		}
		if got.UpdatedAt.IsZero() {
			t.Error("updated_at tidak terbaca")
		}
	})

	t.Run("ReplaceAll mengganti seluruh isi", func(t *testing.T) {
		err := repo.ReplaceAll(ctx, &domain.Template{TenantID: 1,
			MakeupRooms: []domain.MakeupRoom{{RoomLabel: "Ruang CPW", Lines: []domain.MakeupLine{
				{Style: domain.MakeupStyleNumbered, Content: "Rias"}}}},
			LayoutNotes: []domain.LayoutNote{{Kind: domain.LayoutNoteRule, NumberLabel: "1", Content: "Tamu duduk"}},
		})
		if err != nil {
			t.Fatalf("ReplaceAll: %v", err)
		}
		got, _ := repo.Get(ctx, 1)
		if len(got.Roles) != 0 || len(got.ItemsAkad) != 0 {
			t.Error("seksi lama masih tersisa setelah ReplaceAll")
		}
		if len(got.MakeupRooms) != 1 || len(got.MakeupRooms[0].Lines) != 1 ||
			got.MakeupRooms[0].Lines[0].Style != domain.MakeupStyleNumbered {
			t.Errorf("ruangan makeup tidak tersimpan utuh: %+v", got.MakeupRooms)
		}
		if len(got.LayoutNotes) != 1 || got.LayoutNotes[0].Kind != domain.LayoutNoteRule {
			t.Errorf("catatan layout tidak tersimpan utuh: %+v", got.LayoutNotes)
		}
	})

	t.Run("template per tenant terpisah", func(t *testing.T) {
		got, err := repo.Get(ctx, 2)
		if err != nil || got != nil {
			t.Errorf("tenant 2 melihat template tenant 1: %+v, %v", got, err)
		}
	})
}
