package adminseed

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"jwswedding/internal/migrator"
	"jwswedding/internal/shared/database"
)

// testDBName is a dedicated, disposable database — never the developer's own
// jwswedding_db — created and dropped around each test run so this never
// touches real local dev data.
const testDBName = "jwswedding_adminseed_test"

const testAdminDSN = "root:@tcp(127.0.0.1:3306)/"

// setupTestDB skips the test entirely (not a failure) when no local MySQL is
// reachable — these are integration tests against real SQL (adminseed.Run
// takes a raw *sql.DB and issues real UPDATE/INSERT statements), not unit
// tests with fakes, so they can only run where a database exists.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	admin, err := sql.Open("mysql", testAdminDSN)
	if err != nil {
		t.Skipf("mysql driver tidak bisa dibuka, skip test integrasi: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip test integrasi: %v", err)
	}

	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + testDBName); err != nil {
		t.Fatalf("drop test db: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + testDBName); err != nil {
		t.Fatalf("create test db: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", testAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + testDBName)
	})

	testURL := fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", testDBName)
	if err := migrator.Up(testURL); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	db, err := database.Open(testURL)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seededCustomDomain(t *testing.T, db *sql.DB) sql.NullString {
	t.Helper()
	var customDomain sql.NullString
	if err := db.QueryRow("SELECT custom_domain FROM tenants LIMIT 1").Scan(&customDomain); err != nil {
		t.Fatalf("query custom_domain: %v", err)
	}
	return customDomain
}

func TestAdminSeed_MengisiCustomDomain(t *testing.T) {
	db := setupTestDB(t)
	if err := os.Unsetenv("SEED_TENANT_CUSTOM_DOMAIN"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	customDomain := seededCustomDomain(t, db)
	if !customDomain.Valid || customDomain.String != "journey.jwswedding.com" {
		t.Errorf("expected custom_domain = journey.jwswedding.com, got %+v", customDomain)
	}
}

func TestAdminSeed_CustomDomainKosongTetapNull(t *testing.T) {
	db := setupTestDB(t)
	if err := os.Setenv("SEED_TENANT_CUSTOM_DOMAIN", ""); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer os.Unsetenv("SEED_TENANT_CUSTOM_DOMAIN")

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	customDomain := seededCustomDomain(t, db)
	if customDomain.Valid {
		t.Errorf("expected custom_domain NULL when SEED_TENANT_CUSTOM_DOMAIN is explicitly empty, got %q", customDomain.String)
	}
}

func seededPlanID(t *testing.T, db *sql.DB) sql.NullInt64 {
	t.Helper()
	var planID sql.NullInt64
	if err := db.QueryRow("SELECT plan_id FROM tenants LIMIT 1").Scan(&planID); err != nil {
		t.Fatalf("query plan_id: %v", err)
	}
	return planID
}

// TestAdminSeed_MengisiPlanID locks §6.1 A4 of
// docs/plan/konsolidasi-plan-pasca-standalone/PLAN.md: a freshly seeded
// tenant's plan_id used to stay NULL (no code path ever set it), so
// SubscriptionPage never showed "Paket Aktif Anda" in a fresh dev
// environment even though the tenant's subscription itself is active.
func TestAdminSeed_MengisiPlanID(t *testing.T) {
	db := setupTestDB(t)
	if err := os.Unsetenv("SEED_TENANT_PLAN_ID"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	planID := seededPlanID(t, db)
	if !planID.Valid || planID.Int64 != 2 {
		t.Errorf("expected plan_id = 2 (default), got %+v", planID)
	}
}

func TestAdminSeed_PlanIDKosongTetapNull(t *testing.T) {
	db := setupTestDB(t)
	if err := os.Setenv("SEED_TENANT_PLAN_ID", ""); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer os.Unsetenv("SEED_TENANT_PLAN_ID")

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	planID := seededPlanID(t, db)
	if planID.Valid {
		t.Errorf("expected plan_id NULL when SEED_TENANT_PLAN_ID is explicitly empty, got %+v", planID.Int64)
	}
}

// TestAdminSeed_TruncateMembersihkanSemuaTabel locks T8/D8 from
// docs/plan/migrasi-data-jws/PLAN.md: truncateAll's table list previously
// missed venues/client_payments/venue_payments/project_milestone_templates
// (inherited from ElProof, never updated when those tables were added) —
// Run() reported success while leaving all four full of rows pointing at a
// tenant/projects it had just wiped. Seed dummy rows into each, run Run(),
// and assert all four are empty afterward.
func TestAdminSeed_TruncateMembersihkanSemuaTabel(t *testing.T) {
	db := setupTestDB(t)

	seedRes, err := db.Exec(
		`INSERT INTO tenants (business_name, owner_name, username, email, phone, city, joined_at, subscription_status)
		 VALUES ('Dummy Tenant', 'Dummy Owner', 'dummy-tenant', 'dummy@example.com', '-', '-', NOW(), 'active')`,
	)
	if err != nil {
		t.Fatalf("insert dummy tenant: %v", err)
	}
	tenantID, err := seedRes.LastInsertId()
	if err != nil {
		t.Fatalf("dummy tenant id: %v", err)
	}

	projectRes, err := db.Exec(
		`INSERT INTO projects (tenant_id, name, bride_name, groom_name, event_date, venue, prep_start_date, package_name, pic_staff_id)
		 VALUES (?, 'Dummy Project', 'Bride', 'Groom', CURDATE(), 'Dummy Venue', CURDATE(), 'Dummy Package', 1)`,
		tenantID,
	)
	if err != nil {
		t.Fatalf("insert dummy project: %v", err)
	}
	projectID, err := projectRes.LastInsertId()
	if err != nil {
		t.Fatalf("dummy project id: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO venues (tenant_id, name, pic_name, phone_pic) VALUES (?, 'Dummy Venue', 'PIC', '-')`, tenantID,
	); err != nil {
		t.Fatalf("insert dummy venue: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO project_milestone_templates (tenant_id, sort_order, name, days_before_event) VALUES (?, 1, 'Dummy Milestone', 0)`, tenantID,
	); err != nil {
		t.Fatalf("insert dummy project_milestone_template: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO client_payments (project_id, type, amount, payment_date, method, reference_number)
		 VALUES (?, 'DP', 100, CURDATE(), 'Transfer', 'REF-1')`, projectID,
	); err != nil {
		t.Fatalf("insert dummy client_payment: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO venue_payments (project_id, type, amount, payment_date, method, reference_number)
		 VALUES (?, 'DP', 100, CURDATE(), 'Transfer', 'REF-1')`, projectID,
	); err != nil {
		t.Fatalf("insert dummy venue_payment: %v", err)
	}
	// client_invoices missed truncateAll a second time (§3.6 of
	// docs/plan/konsolidasi-plan-pasca-standalone/PLAN.md) when it was added
	// after this test was written — dummy row proves the fix, not just the
	// completeness check below.
	if _, err := db.Exec(
		`INSERT INTO client_invoices (project_id, invoice_number, number_period, number_seq, type, amount, due_date, created_by_staff_id)
		 VALUES (?, 'INV-TEST-202608-0001', '202608', 1, 'DP', 100, CURDATE(), 1)`, projectID,
	); err != nil {
		t.Fatalf("insert dummy client_invoice: %v", err)
	}

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, table := range []string{"venues", "client_payments", "venue_payments", "project_milestone_templates", "client_invoices"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("expected %s to be empty after Run, got %d row(s) — T8/D8 regression", table, count)
		}
	}
}

// TestAdminSeed_TruncateTablesLengkap adalah gerbang kelengkapan struktural
// yang mencegah kelas bug T8/D8 kambuh untuk ketiga kalinya (lihat komentar
// di atas truncateTables di adminseed.go, dan
// docs/plan/konsolidasi-plan-pasca-standalone/PLAN.md §3.6/§6.1 A2) —
// truncateTables dibandingkan langsung terhadap information_schema.TABLES,
// bukan terhadap daftar nama hardcoded lain, supaya migrasi baru yang lupa
// didaftarkan gagal di sini tanpa perlu menulis test baru setiap kali.
func TestAdminSeed_TruncateTablesLengkap(t *testing.T) {
	db := setupTestDB(t)

	rows, err := db.Query(
		`SELECT TABLE_NAME FROM information_schema.TABLES
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'`,
	)
	if err != nil {
		t.Fatalf("query information_schema.TABLES: %v", err)
	}
	defer rows.Close()

	truncated := make(map[string]bool, len(truncateTables))
	for _, table := range truncateTables {
		truncated[table] = true
	}

	var missing []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		// schema_migrations belongs to golang-migrate, not app data —
		// truncating it would corrupt migration state, not clean it.
		if name == "schema_migrations" {
			continue
		}
		if !truncated[name] {
			missing = append(missing, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate information_schema.TABLES: %v", err)
	}

	if len(missing) > 0 {
		t.Errorf(
			"truncateTables (adminseed.go) tertinggal %d tabel yang ada di skema tapi tidak pernah di-truncate: %v — tambahkan ke truncateTables",
			len(missing), missing,
		)
	}
}
