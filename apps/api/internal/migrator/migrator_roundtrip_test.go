package migrator_test

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"jwswedding/internal/migrator"
)

// Round-trip test for the migration set, against a throwaway database created
// and dropped around the run so it never touches local dev data — the same
// idiom adminseed_test.go already uses.
//
// This exists because a `.down.sql` file is normally never executed: `migrate
// up` is all CI and deployment ever run, so a broken rollback stays invisible
// until the one moment it is needed most. Rolling the newest migrations back
// and forward here is the only thing that proves they are reversible.
const (
	roundTripDB       = "jwswedding_migrate_roundtrip_test"
	roundTripAdminDSN = "root:@tcp(127.0.0.1:3306)/"
)

func setupRoundTripDB(t *testing.T) string {
	t.Helper()

	admin, err := sql.Open("mysql", roundTripAdminDSN)
	if err != nil {
		t.Skipf("driver mysql tidak bisa dibuka, skip: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + roundTripDB); err != nil {
		t.Fatalf("drop db uji: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + roundTripDB); err != nil {
		t.Fatalf("create db uji: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", roundTripAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + roundTripDB)
	})
	return fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", roundTripDB)
}

func tableExists(t *testing.T, url, table string) bool {
	t.Helper()
	db, err := sql.Open("mysql", roundTripAdminDSN+roundTripDB)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	var n int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`,
		roundTripDB, table).Scan(&n)
	if err != nil {
		t.Fatalf("cek tabel %s: %v", table, err)
	}
	return n > 0
}

func columnExists(t *testing.T, url, table, column string) bool {
	t.Helper()
	db, err := sql.Open("mysql", roundTripAdminDSN+roundTripDB)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	var n int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		roundTripDB, table, column).Scan(&n)
	if err != nil {
		t.Fatalf("cek kolom %s.%s: %v", table, column, err)
	}
	return n > 0
}

// TestMigrations_UpDownUp rolls the four PO Paket migrations (000052-000055)
// back one step at a time and then forward again.
//
// The rollback order matters and is the part most likely to break: the three
// project_package_* tables carry FKs to `projects`, and the template tables
// carry FKs to each other, so a DROP in the wrong order fails on a constraint.
func TestMigrations_UpDownUp(t *testing.T) {
	url := setupRoundTripDB(t)

	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up awal: %v", err)
	}
	for _, table := range []string{
		"package_templates", "package_template_blocks", "package_template_terms",
		"project_package_blocks", "project_package_adjustments", "project_package_orders",
	} {
		if !tableExists(t, url, table) {
			t.Fatalf("setelah up, tabel %s tidak ada", table)
		}
	}
	if !columnExists(t, url, "projects", "pax") {
		t.Fatal("setelah up, kolom projects.pax tidak ada")
	}

	// Roll back the four PO Paket steps: 000055, 000054, 000053, 000052.
	for i := 0; i < 4; i++ {
		if err := migrator.Down(url); err != nil {
			t.Fatalf("migrate down langkah ke-%d: %v", i+1, err)
		}
	}

	if columnExists(t, url, "projects", "pax") {
		t.Error("setelah down, kolom projects.pax masih ada")
	}
	for _, table := range []string{
		"package_templates", "package_template_blocks", "package_template_terms",
		"project_package_blocks", "project_package_adjustments", "project_package_orders",
	} {
		if tableExists(t, url, table) {
			t.Errorf("setelah down, tabel %s masih ada", table)
		}
	}
	// The pre-existing schema must survive the rollback untouched — a down
	// migration that takes a neighbouring table with it is worse than one that
	// fails outright.
	for _, table := range []string{"projects", "client_invoices", "client_payments", "project_milestone_templates"} {
		if !tableExists(t, url, table) {
			t.Errorf("setelah down, tabel lama %s ikut terhapus", table)
		}
	}

	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up ulang setelah down: %v", err)
	}
	if !columnExists(t, url, "projects", "pax") {
		t.Error("setelah up ulang, kolom projects.pax tidak kembali")
	}
	if !tableExists(t, url, "project_package_orders") {
		t.Error("setelah up ulang, project_package_orders tidak kembali")
	}
}
