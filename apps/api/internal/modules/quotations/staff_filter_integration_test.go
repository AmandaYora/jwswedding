package quotations_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"jwswedding/internal/migrator"
	projectsinfra "jwswedding/internal/modules/projects/infrastructure"
	"jwswedding/internal/shared/database"

	qapp "jwswedding/internal/modules/quotations/application"
	qinfra "jwswedding/internal/modules/quotations/infrastructure"
	capp "jwswedding/internal/modules/clients/application"
	cinfra "jwswedding/internal/modules/clients/infrastructure"
	"jwswedding/internal/shared/pagination"
)

// Tes filter Sales + Wedding Planner lawan MySQL nyata (PLAN
// wording-role-dan-filter-sales-wp T41/T44-repo/T46): semantik SQL yang
// tidak bisa dikunci test unit — penyempitan count+list oleh SalesStaffID,
// pengabaian baris quotation_id NULL, peng-AND-an kedua slot PIC, dan
// pembatas ID di repo client.
//
// Seed (tenant 1): WP id 10 memegang p1 (client1, quotation q1 buatan sales
// 11) + p2 (client1, quotation q2 buatan sales 12) + p3 (client2, TANPA
// quotation — project lama pra-penawaran).
const staffFilterTestDB = "jwswedding_staff_filter_test"

type staffFilterFixture struct {
	db       *sql.DB
	tenantID int64
	client1  int64
	client2  int64
	q1       int64
	q2       int64
	q3       int64
}

func setupStaffFilterFixture(t *testing.T) *staffFilterFixture {
	t.Helper()

	admin, err := sql.Open("mysql", acceptTestAdminDSN)
	if err != nil {
		t.Skipf("driver mysql tidak bisa dibuka, skip: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + staffFilterTestDB); err != nil {
		t.Fatalf("drop db uji: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + staffFilterTestDB); err != nil {
		t.Fatalf("create db uji: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", acceptTestAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + staffFilterTestDB)
	})

	url := fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", staffFilterTestDB)
	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	db, err := database.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	mustExec := func(query string, args ...interface{}) int64 {
		t.Helper()
		res, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			t.Fatalf("seed (%s): %v", query, err)
		}
		id, _ := res.LastInsertId()
		return id
	}

	mustExec(`INSERT INTO staff_members (id, tenant_id, name, title, initials, role, email, phone)
		VALUES (10, 1, 'WP Satu', 'Koordinator', 'WS', 'Staff', '', ''),
		       (11, 1, 'Sales Satu', 'Sales Lapangan', 'SS', 'Sales', '', ''),
		       (12, 1, 'Sales Dua', 'Sales Lapangan', 'SD', 'Sales', '', '')`)
	client1 := mustExec(`INSERT INTO clients (tenant_id, bride_name, groom_name, phone, email, notes)
		VALUES (1, 'Rara', 'Dafa', '', '', '')`)
	client2 := mustExec(`INSERT INTO clients (tenant_id, bride_name, groom_name, phone, email, notes)
		VALUES (1, 'Sinta', 'Bimo', '', '', '')`)
	q1 := mustExec(`INSERT INTO quotations (tenant_id, client_id, status, base_price, terms_text, bonus_note, created_by_staff_id)
		VALUES (1, ?, 'Ditawarkan', 100, '', '', 11)`, client1)
	q2 := mustExec(`INSERT INTO quotations (tenant_id, client_id, status, base_price, terms_text, bonus_note, created_by_staff_id)
		VALUES (1, ?, 'Ditawarkan', 200, '', '', 12)`, client1)
	q3 := mustExec(`INSERT INTO quotations (tenant_id, client_id, status, base_price, terms_text, bonus_note, created_by_staff_id)
		VALUES (1, ?, 'Draft', 300, '', '', 11)`, client2)
	mustExec(`INSERT INTO projects (tenant_id, client_id, quotation_id, name, bride_name, groom_name, event_date, venue, prep_start_date, package_name, pic_staff_id, pic_sales_staff_id)
		VALUES (1, ?, ?, 'Akad Rara & Dafa', 'Rara', 'Dafa', '2026-09-19', '', '2026-01-01', '', 10, 11)`, client1, q1)
	mustExec(`INSERT INTO projects (tenant_id, client_id, quotation_id, name, bride_name, groom_name, event_date, venue, prep_start_date, package_name, pic_staff_id, pic_sales_staff_id)
		VALUES (1, ?, ?, 'Resepsi Rara & Dafa', 'Rara', 'Dafa', '2026-09-20', '', '2026-01-01', '', 10, 12)`, client1, q2)
	mustExec(`INSERT INTO projects (tenant_id, client_id, quotation_id, name, bride_name, groom_name, event_date, venue, prep_start_date, package_name, pic_staff_id, pic_sales_staff_id)
		VALUES (1, ?, NULL, 'Lama Sinta & Bimo', 'Sinta', 'Bimo', '2025-05-01', '', '2025-01-01', '', 10, 0)`, client2)

	return &staffFilterFixture{db: db, tenantID: 1, client1: client1, client2: client2, q1: q1, q2: q2, q3: q3}
}

// T41: SalesStaffID mempersempit countQuery MAUPUN listQuery (meta.total
// harus cocok dengan isi halaman).
func TestStaffFilter_SalesMempersempitCountDanList(t *testing.T) {
	fx := setupStaffFilterFixture(t)
	repo := qinfra.NewMySQLQuotationRepository(fx.db)
	params := pagination.Params{Page: 1, Limit: 10}

	list, total, err := repo.ListByTenant(context.Background(), fx.tenantID,
		qapp.QuotationListFilter{SalesStaffID: 11}, params)
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, mau 2 (q1+q3 buatan sales 11)", total)
	}
	if len(list) != 2 {
		t.Errorf("len(list) = %d, mau 2", len(list))
	}
	for _, o := range list {
		if o.CreatedByStaffID != 11 {
			t.Errorf("quotation %d created_by = %d, mau 11", o.ID, o.CreatedByStaffID)
		}
	}
}

// T41: pembatas ID hasil filter WP mempersempit daftar (jalur
// RestrictIDs non-kosong).
func TestStaffFilter_RestrictIDsMempersempit(t *testing.T) {
	fx := setupStaffFilterFixture(t)
	repo := qinfra.NewMySQLQuotationRepository(fx.db)

	list, total, err := repo.ListByTenant(context.Background(), fx.tenantID,
		qapp.QuotationListFilter{RestrictActive: true, RestrictIDs: []int64{fx.q2}},
		pagination.Params{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != fx.q2 {
		t.Errorf("total/list = %d/%v, mau 1/[q2]", total, list)
	}
}

// T46: QuotationIDsForPICStaff mengabaikan baris ber-quotation_id NULL
// (project lama pra-penawaran) — tanpa IS NOT NULL, baris itu bocor sebagai
// ID 0 dan meracuni klausa IN.
func TestStaffFilter_QuotationIDsForPICStaff_AbaikanNull(t *testing.T) {
	fx := setupStaffFilterFixture(t)
	repo := projectsinfra.NewMySQLProjectRepository(fx.db)

	ids, err := repo.QuotationIDsForPICStaff(context.Background(), fx.tenantID, 10)
	if err != nil {
		t.Fatalf("QuotationIDsForPICStaff: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ids = %v, mau 2 (q1+q2, tanpa NULL)", ids)
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id == 0 {
			t.Errorf("ID 0 bocor — baris quotation_id NULL ikut")
		}
		seen[id] = true
	}
	if !seen[fx.q1] || !seen[fx.q2] {
		t.Errorf("ids = %v, mau memuat q1=%d dan q2=%d", ids, fx.q1, fx.q2)
	}
}

// T46: ClientIDsForPIC meng-AND-kan kedua slot saat keduanya terisi.
func TestStaffFilter_ClientIDsForPIC_MengANDkan(t *testing.T) {
	fx := setupStaffFilterFixture(t)
	repo := projectsinfra.NewMySQLProjectRepository(fx.db)
	ctx := context.Background()

	// WP saja → kedua client (p1/p2 milik client1, p3 milik client2).
	ids, err := repo.ClientIDsForPIC(ctx, fx.tenantID, 10, 0)
	if err != nil {
		t.Fatalf("ClientIDsForPIC: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("WP saja: ids = %v, mau 2 client", ids)
	}

	// Sales 12 saja → hanya client1 (p2).
	ids, err = repo.ClientIDsForPIC(ctx, fx.tenantID, 0, 12)
	if err != nil {
		t.Fatalf("ClientIDsForPIC: %v", err)
	}
	if len(ids) != 1 || ids[0] != fx.client1 {
		t.Errorf("sales saja: ids = %v, mau [%d]", ids, fx.client1)
	}

	// WP 10 AND sales 12 → hanya client1 (p2 cocok keduanya; p3 cocok WP
	// tapi sales-nya 0, bukan 12).
	ids, err = repo.ClientIDsForPIC(ctx, fx.tenantID, 10, 12)
	if err != nil {
		t.Fatalf("ClientIDsForPIC: %v", err)
	}
	if len(ids) != 1 || ids[0] != fx.client1 {
		t.Errorf("AND: ids = %v, mau [%d]", ids, fx.client1)
	}
}

// ProjectRefsForQuotations memetakan project + PIC WP-nya (dasar tampilan
// nama WP di kartu Penawaran, D7).
func TestStaffFilter_ProjectRefs_MemuatPIC(t *testing.T) {
	fx := setupStaffFilterFixture(t)
	repo := projectsinfra.NewMySQLProjectRepository(fx.db)

	refs, err := repo.ProjectRefsForQuotations(context.Background(), fx.tenantID, []int64{fx.q1, fx.q3})
	if err != nil {
		t.Fatalf("ProjectRefsForQuotations: %v", err)
	}
	ref, ok := refs[fx.q1]
	if !ok || ref.ProjectID == 0 {
		t.Fatalf("refs = %v, mau memuat q1", refs)
	}
	if ref.PICStaffID != 10 {
		t.Errorf("q1.PICStaffID = %d, mau 10", ref.PICStaffID)
	}
	if _, ok := refs[fx.q3]; ok {
		t.Errorf("q3 belum punya project tapi ikut terpetakan: %v", refs[fx.q3])
	}
}

// PICsForClients menjawab himpunan PIC per client dalam satu query (dasar
// tampilan nama di kartu Client, D7).
func TestStaffFilter_PICsForClients_HimpunanBerbeda(t *testing.T) {
	fx := setupStaffFilterFixture(t)
	repo := projectsinfra.NewMySQLProjectRepository(fx.db)

	got, err := repo.PICsForClients(context.Background(), fx.tenantID, []int64{fx.client1, fx.client2})
	if err != nil {
		t.Fatalf("PICsForClients: %v", err)
	}
	c1 := got[fx.client1]
	if len(c1.PICStaffIDs) != 1 || c1.PICStaffIDs[0] != 10 {
		t.Errorf("client1 WP = %v, mau [10]", c1.PICStaffIDs)
	}
	if len(c1.PICSalesStaffIDs) != 2 {
		t.Errorf("client1 sales = %v, mau 2 (11+12)", c1.PICSalesStaffIDs)
	}
	c2 := got[fx.client2]
	if len(c2.PICStaffIDs) != 1 || c2.PICStaffIDs[0] != 10 {
		t.Errorf("client2 WP = %v, mau [10]", c2.PICStaffIDs)
	}
}

// T44-repo: pembatas ID di repo client mempersempit count dan list.
func TestStaffFilter_ClientRepo_PembatasMempersempit(t *testing.T) {
	fx := setupStaffFilterFixture(t)
	repo := cinfra.NewMySQLClientRepository(fx.db)

	list, total, err := repo.ListPaginated(context.Background(), fx.tenantID,
		pagination.Params{Page: 1, Limit: 10},
		capp.ClientListFilter{RestrictActive: true, RestrictIDs: []int64{fx.client2}})
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != fx.client2 {
		t.Errorf("total/list = %d/%v, mau 1/[client2]", total, list)
	}
}
