package migrator_test

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"jwswedding/internal/migrator"
)

// Tes data cutover penawaran-client-master (R1, R3, D3, T1.3, T2.2):
// seed skema PRA-cutover (000055) seperti produksi hari ini — project +
// kontak portal + PO Terbit — lalu jalankan cutover (000056-000062) dan
// pastikan datanya pindah dengan benar, BUKAN cuma skemanya.
//
// D3 dikunci di sini: id baris kontak TIDAK boleh berubah (kredensial portal
// = PrincipalID id itu), dan setiap project_package_orders lama wajib punya
// quotations padanan + projects.quotation_id terisi (§9).
const (
	cutoverDB       = "jwswedding_cutover_test"
	cutoverAdminDSN = "root:@tcp(127.0.0.1:3306)/"
)

func setupCutoverDB(t *testing.T) (url string, db *sql.DB) {
	t.Helper()

	admin, err := sql.Open("mysql", cutoverAdminDSN)
	if err != nil {
		t.Skipf("driver mysql tidak bisa dibuka, skip: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + cutoverDB); err != nil {
		t.Fatalf("drop db uji: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + cutoverDB); err != nil {
		t.Fatalf("create db uji: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", cutoverAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + cutoverDB)
	})
	url = fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", cutoverDB)
	db, err = sql.Open("mysql", cutoverAdminDSN+cutoverDB)
	if err != nil {
		t.Fatalf("open db uji: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return url, db
}

func seedPreCutover(t *testing.T, db *sql.DB) (projectID, contactID int64) {
	t.Helper()

	res, err := db.Exec(
		`INSERT INTO projects (tenant_id, name, bride_name, groom_name, event_date, venue, prep_start_date,
		 package_name, contract_value, status, pic_staff_id, pic_sales_staff_id, pax, is_archived)
		 VALUES (1, 'Akad Rara & Dafa', 'Rara', 'Dafa', '2026-09-19', 'KLINK TOWER', '2025-10-25',
		 'Paket Silver', 213500000, 'Preparation', 0, 0, 800, 0)`)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	projectID, _ = res.LastInsertId()

	res, err = db.Exec(
		`INSERT INTO clients (tenant_id, project_id, role, relation_note, name, phone, email, is_active)
		 VALUES (1, ?, 'Bride', '', 'Rara', '08170043310', 'rara@example.com', 1)`, projectID)
	if err != nil {
		t.Fatalf("seed kontak: %v", err)
	}
	contactID, _ = res.LastInsertId()

	if _, err := db.Exec(
		`INSERT INTO project_package_orders (project_id, po_number, number_period, number_seq, revision,
		 base_price, terms_text, bonus_note, terms_plan_json, status, issued_at, created_by_staff_id)
		 VALUES (?, 'PO/202510/0003', '202510', 3, 0, 213500000, 'S&K', 'Bonus',
		 '[{"sequence":1,"label":"DP","type":"DP","fixedAmount":10000000,"daysBeforeEvent":330}]',
		 'Terbit', '2025-10-25 10:00:00', 1)`, projectID); err != nil {
		t.Fatalf("seed PO: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO project_package_blocks (project_id, category, body, qty_text, bonus_note, sort_order)
		 VALUES (?, 'CATERING', 'BUFFET', '700 PORSI', 'BONUS', 1)`, projectID); err != nil {
		t.Fatalf("seed blok: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO project_package_adjustments (project_id, description, amount, sort_order)
		 VALUES (?, 'Add 100 buffet', 9900000, 1)`, projectID); err != nil {
		t.Fatalf("seed adjustment: %v", err)
	}
	return projectID, contactID
}

func TestCutover_PenawaranClientMaster_MemindahkanData(t *testing.T) {
	url, db := setupCutoverDB(t)

	if err := migrator.To(url, 55); err != nil {
		t.Fatalf("migrate ke 000055: %v", err)
	}
	projectID, contactID := seedPreCutover(t, db)

	if err := migrator.Up(url); err != nil {
		t.Fatalf("cutover 000056-000062: %v", err)
	}

	// 1. Master pasangan lahir dari nama mempelai project.
	var clientID int64
	var bride, groom, phone string
	if err := db.QueryRow(`SELECT id, bride_name, groom_name, phone FROM clients LIMIT 1`).
		Scan(&clientID, &bride, &groom, &phone); err != nil {
		t.Fatalf("baca clients: %v", err)
	}
	if bride != "Rara" || groom != "Dafa" {
		t.Errorf("clients = %q & %q, mau Rara & Dafa", bride, groom)
	}
	if phone != "08170043310" {
		t.Errorf("telepon master = %q, mau dari kontak pertama", phone)
	}

	// 2. D3: id baris kontak TETAP, kini menunjuk master.
	var gotContactID, gotClientID int64
	if err := db.QueryRow(`SELECT id, client_id FROM client_contacts LIMIT 1`).
		Scan(&gotContactID, &gotClientID); err != nil {
		t.Fatalf("baca client_contacts: %v", err)
	}
	if gotContactID != contactID {
		t.Errorf("id kontak berubah %d -> %d — kredensial portal putus (D3)", contactID, gotContactID)
	}
	if gotClientID != clientID {
		t.Errorf("client_contacts.client_id = %d, mau %d", gotClientID, clientID)
	}

	// 3. Project menunjuk master + penawarannya.
	var gotClient, gotQuotation sql.NullInt64
	if err := db.QueryRow(`SELECT client_id, quotation_id FROM projects WHERE id = ?`, projectID).
		Scan(&gotClient, &gotQuotation); err != nil {
		t.Fatalf("baca projects: %v", err)
	}
	if !gotClient.Valid || gotClient.Int64 != clientID {
		t.Errorf("projects.client_id = %v, mau %d", gotClient, clientID)
	}
	if !gotQuotation.Valid {
		t.Fatal("projects.quotation_id NULL — PO lama tidak tertaut balik")
	}

	// 4. PO lama menjadi penawaran Diterima, nomor + harga utuh.
	var qID int64
	var status, poNumber string
	var basePrice int64
	if err := db.QueryRow(`SELECT id, status, po_number, base_price FROM quotations LIMIT 1`).
		Scan(&qID, &status, &poNumber, &basePrice); err != nil {
		t.Fatalf("baca quotations: %v", err)
	}
	if qID != gotQuotation.Int64 {
		t.Errorf("quotations.id = %d, projects.quotation_id = %d — tidak sinkron", qID, gotQuotation.Int64)
	}
	if status != "Diterima" {
		t.Errorf("status = %q, mau Diterima", status)
	}
	if poNumber != "PO/202510/0003" {
		t.Errorf("po_number = %q, nomor lama harus utuh", poNumber)
	}
	if basePrice != 213500000 {
		t.Errorf("base_price = %d, mau 213500000", basePrice)
	}
	var nBlocks, nAdj int
	if err := db.QueryRow(`SELECT COUNT(*) FROM quotation_blocks WHERE quotation_id = ?`, qID).Scan(&nBlocks); err != nil {
		t.Fatalf("hitung blok: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM quotation_adjustments WHERE quotation_id = ?`, qID).Scan(&nAdj); err != nil {
		t.Fatalf("hitung adjustment: %v", err)
	}
	if nBlocks != 1 || nAdj != 1 {
		t.Errorf("blok/adjustment = %d/%d, mau 1/1", nBlocks, nAdj)
	}

	// 5. Tabel lama dibuang di ujung rantai.
	for _, table := range []string{"project_package_orders", "project_package_blocks", "project_package_adjustments"} {
		var n int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`,
			cutoverDB, table).Scan(&n); err != nil {
			t.Fatalf("cek tabel %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("tabel lama %s masih ada setelah 000062", table)
		}
	}
}
