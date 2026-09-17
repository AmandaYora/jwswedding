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

func indexExists(t *testing.T, url, table, index string) bool {
	t.Helper()
	db, err := sql.Open("mysql", roundTripAdminDSN+roundTripDB)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	var n int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND INDEX_NAME = ?`,
		roundTripDB, table, index).Scan(&n)
	if err != nil {
		t.Fatalf("cek index %s.%s: %v", table, index, err)
	}
	return n > 0
}

// TestMigrations_UpDownUp rolls the PO Paket migrations (000052-000055), the
// penawaran-client-master migrations (000056-000063), 000064 (drop of the
// payment-schedule feature), 000065 (quotations.package_name), the TTD
// Penawaran migrations (000066 client_signatures, 000067
// quotation_signature_links), 000068 (tiga index filter Sales/WP, ADR-0033),
// 000069 (venue_categories label table, PLAN revisi-vendor-venue-portal),
// and 000070 (restrict cities to Jabodetabek+Bali, down = intentional no-op)
// back one step at a time and then forward again.
//
// The step count below is coupled to the chain length: every new migration
// appended after 000070 must extend it (and the table assertions), or the
// rollback lands short and old tables "remain" — exactly the false failure
// adding 000069/000070 first produced here.
//
// The rollback order matters and is the part most likely to break: the three
// project_package_* tables carry FKs to `projects`, the quotation_* tables
// carry FKs to `quotations`, the template tables carry FKs to each other, and
// 000059/000062 drop columns other steps added — so a DROP in the wrong order
// fails on a constraint.
func TestMigrations_UpDownUp(t *testing.T) {
	url := setupRoundTripDB(t)

	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up awal: %v", err)
	}
	for _, table := range []string{
		"package_templates", "package_template_blocks",
		"clients", "client_contacts", "quotations", "quotation_blocks", "quotation_adjustments",
	} {
		if !tableExists(t, url, table) {
			t.Fatalf("setelah up, tabel %s tidak ada", table)
		}
	}
	// 000064 membuang fitur rencana termin sepenuhnya.
	if tableExists(t, url, "package_template_terms") {
		t.Fatal("setelah up, tabel package_template_terms masih ada")
	}
	if columnExists(t, url, "quotations", "terms_plan_json") {
		t.Fatal("setelah up, kolom quotations.terms_plan_json masih ada")
	}
	// 000065: nama paket jadi milik penawaran.
	if !columnExists(t, url, "quotations", "package_name") {
		t.Fatal("setelah up, kolom quotations.package_name tidak ada")
	}
	// 000066/000067 (TTD Penawaran): specimen + link tanda tangan.
	for _, table := range []string{"client_signatures", "quotation_signature_links"} {
		if !tableExists(t, url, table) {
			t.Fatalf("setelah up, tabel %s tidak ada", table)
		}
	}
	// 000068 (ADR-0033): tiga index filter Sales/WP.
	for _, idx := range [][2]string{
		{"quotations", "idx_quotations_tenant_created_by"},
		{"projects", "idx_projects_tenant_pic_sales"},
		{"projects", "idx_projects_tenant_client"},
	} {
		if !indexExists(t, url, idx[0], idx[1]) {
			t.Fatalf("setelah up, index %s.%s tidak ada", idx[0], idx[1])
		}
	}
	// 000069 (PLAN revisi-vendor-venue-portal): label kategori venue.
	if !tableExists(t, url, "venue_categories") {
		t.Fatal("setelah up, tabel venue_categories tidak ada")
	}
	// 000062 dropped the old composition tables at the end of the chain.
	for _, table := range []string{
		"project_package_blocks", "project_package_adjustments", "project_package_orders",
	} {
		if tableExists(t, url, table) {
			t.Fatalf("setelah up, tabel lama %s masih ada", table)
		}
	}
	for _, col := range [][2]string{
		{"projects", "pax"},
		{"projects", "client_id"},
		{"projects", "quotation_id"},
		{"client_contacts", "client_id"},
	} {
		if !columnExists(t, url, col[0], col[1]) {
			t.Fatalf("setelah up, kolom %s.%s tidak ada", col[0], col[1])
		}
	}
	if columnExists(t, url, "client_contacts", "project_id") {
		t.Fatal("setelah up, kolom client_contacts.project_id masih ada")
	}

	// Roll back the nineteen steps: 000070 down to 000052. (000070's own down
	// is an intentional no-op — its city-NULLing is unrecoverable by design,
	// see A6/A7 — so no data assertion covers it on this empty schema.)
	for i := 0; i < 19; i++ {
		if err := migrator.Down(url); err != nil {
			t.Fatalf("migrate down langkah ke-%d: %v", i+1, err)
		}
	}

	// 000068 ikut turun pertama: ketiga index-nya wajib hilang.
	for _, idx := range [][2]string{
		{"quotations", "idx_quotations_tenant_created_by"},
		{"projects", "idx_projects_tenant_pic_sales"},
		{"projects", "idx_projects_tenant_client"},
	} {
		if indexExists(t, url, idx[0], idx[1]) {
			t.Errorf("setelah down, index %s.%s masih ada", idx[0], idx[1])
		}
	}

	if columnExists(t, url, "projects", "pax") {
		t.Error("setelah down, kolom projects.pax masih ada")
	}
	if columnExists(t, url, "projects", "client_id") {
		t.Error("setelah down, kolom projects.client_id masih ada")
	}
	if columnExists(t, url, "projects", "quotation_id") {
		t.Error("setelah down, kolom projects.quotation_id masih ada")
	}
	if columnExists(t, url, "quotations", "package_name") {
		t.Error("setelah down, kolom quotations.package_name masih ada")
	}
	for _, table := range []string{
		"package_templates", "package_template_blocks", "package_template_terms",
		"project_package_blocks", "project_package_adjustments", "project_package_orders",
		"quotations", "quotation_blocks", "quotation_adjustments", "client_contacts",
		"client_signatures", "quotation_signature_links", "venue_categories",
	} {
		if tableExists(t, url, table) {
			t.Errorf("setelah down, tabel %s masih ada", table)
		}
	}
	// 000056's down renames client_contacts back to clients — the original
	// table must be back.
	if !tableExists(t, url, "clients") {
		t.Error("setelah down, tabel clients tidak kembali")
	}
	if !columnExists(t, url, "clients", "project_id") {
		t.Error("setelah down, kolom clients.project_id tidak kembali")
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
	// 000068 ikut naik lagi: ketiga index-nya wajib kembali.
	for _, idx := range [][2]string{
		{"quotations", "idx_quotations_tenant_created_by"},
		{"projects", "idx_projects_tenant_pic_sales"},
		{"projects", "idx_projects_tenant_client"},
	} {
		if !indexExists(t, url, idx[0], idx[1]) {
			t.Errorf("setelah up ulang, index %s.%s tidak kembali", idx[0], idx[1])
		}
	}
	if !columnExists(t, url, "projects", "pax") {
		t.Error("setelah up ulang, kolom projects.pax tidak kembali")
	}
	if !columnExists(t, url, "projects", "client_id") {
		t.Error("setelah up ulang, kolom projects.client_id tidak kembali")
	}
	if !columnExists(t, url, "projects", "quotation_id") {
		t.Error("setelah up ulang, kolom projects.quotation_id tidak kembali")
	}
	if !tableExists(t, url, "quotations") {
		t.Error("setelah up ulang, quotations tidak kembali")
	}
	if !tableExists(t, url, "client_contacts") {
		t.Error("setelah up ulang, client_contacts tidak kembali")
	}
	if !tableExists(t, url, "venue_categories") {
		t.Error("setelah up ulang, venue_categories tidak kembali")
	}
	for _, table := range []string{
		"project_package_blocks", "project_package_adjustments", "project_package_orders",
	} {
		if tableExists(t, url, table) {
			t.Errorf("setelah up ulang, tabel lama %s kembali ada", table)
		}
	}
}

// TestMigrations_RollbackDenganData menutup celah yang membuat rollback
// 000061 lolos selama ini: TestMigrations_UpDownUp menjalankan turun-naik pada
// skema KOSONG, sehingga `DELETE FROM quotations` tidak pernah menabrak satu
// baris anak pun.
//
// Dengan data, urutannya jadi menentukan: 000063.down mencabut ON DELETE
// CASCADE lebih dulu, jadi 000061.down yang mengandalkan cascade akan mati
// dengan FK 1451 — persis di jalur mundur yang paling dibutuhkan kalau cutover
// harus dibatalkan.
func TestMigrations_RollbackDenganData(t *testing.T) {
	url := setupRoundTripDB(t)

	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up awal: %v", err)
	}

	db, err := sql.Open("mysql", roundTripDSN())
	if err != nil {
		t.Fatalf("buka db uji: %v", err)
	}
	defer db.Close()

	res, err := db.Exec(`INSERT INTO quotations
		(tenant_id, client_id, revision, status, base_price, terms_text, bonus_note, pax, created_by_staff_id)
		VALUES (1, 1, 0, 'Ditawarkan', 150000000, 'S&K', 'Bonus', 700, 1)`)
	if err != nil {
		t.Fatalf("seed quotation: %v", err)
	}
	quotationID, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO quotation_blocks
		(quotation_id, category, body, qty_text, bonus_note, sort_order)
		VALUES (?, 'CATERING', 'BUFFET', '700 PORSI', '', 1)`, quotationID); err != nil {
		t.Fatalf("seed blok: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO quotation_adjustments
		(quotation_id, description, amount, sort_order)
		VALUES (?, 'Takeout busana', -1500000, 1)`, quotationID); err != nil {
		t.Fatalf("seed penyesuaian: %v", err)
	}

	// Turun melewati 000063 (cabut cascade), 000062 (bangun ulang tabel lama),
	// lalu 000061 (buang hasil salinan) — dengan tabel yang berisi.
	if err := migrator.To(url, 60); err != nil {
		t.Fatalf("rollback ke 000060 dengan data gagal: %v", err)
	}

	var sisa int
	if err := db.QueryRow(`SELECT COUNT(*) FROM quotations`).Scan(&sisa); err != nil {
		t.Fatalf("hitung quotations: %v", err)
	}
	if sisa != 0 {
		t.Errorf("quotations tersisa %d setelah rollback, mau 0", sisa)
	}
	var anak int
	if err := db.QueryRow(`SELECT COUNT(*) FROM quotation_blocks`).Scan(&anak); err != nil {
		t.Fatalf("hitung quotation_blocks: %v", err)
	}
	if anak != 0 {
		t.Errorf("quotation_blocks tersisa %d setelah rollback, mau 0", anak)
	}

	// Dan masih bisa maju lagi setelahnya.
	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up kembali setelah rollback: %v", err)
	}
}

func roundTripDSN() string {
	return fmt.Sprintf("root:@tcp(127.0.0.1:3306)/%s?multiStatements=true&parseTime=true", roundTripDB)
}
