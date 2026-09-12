package projects_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"jwswedding/internal/migrator"
	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/modules/projects/infrastructure"
	"jwswedding/internal/shared/database"
)

// End-to-end integration test for the PO Paket flow against a REAL database.
//
// Every other test for this feature exercises pure functions; the repository
// SQL itself had never executed once. This runs the whole chain — template →
// apply → edit → issue → revise → cancel → hard delete — so a broken column
// name, a wrong sentinel, or an FK ordering mistake surfaces here rather than
// in production.
//
// Uses a throwaway database created and dropped around the run, the same idiom
// adminseed_test.go established, so local dev data is never touched.
const (
	poTestDB       = "jwswedding_po_integration_test"
	poTestAdminDSN = "root:@tcp(127.0.0.1:3306)/"
)

type poFixture struct {
	db        *sql.DB
	templates *application.PackageTemplateService
	orders    *application.PackageOrderService
	invoices  *application.ClientInvoiceService
	projects  *infrastructure.MySQLProjectRepository
	tenantID  int64
	projectID int64
}

func setupPOFixture(t *testing.T) *poFixture {
	t.Helper()

	admin, err := sql.Open("mysql", poTestAdminDSN)
	if err != nil {
		t.Skipf("driver mysql tidak bisa dibuka, skip: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + poTestDB); err != nil {
		t.Fatalf("drop db uji: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + poTestDB); err != nil {
		t.Fatalf("create db uji: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", poTestAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + poTestDB)
	})

	url := fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", poTestDB)
	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	db, err := database.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	res, err := db.Exec(
		`INSERT INTO projects (tenant_id, name, bride_name, groom_name, event_date, pax, venue, prep_start_date,
		 package_name, contract_value, status, pic_staff_id, pic_sales_staff_id, description)
		 VALUES (1, 'Akad Rara & Dafa', 'Rara', 'Dafa', '2026-09-19', 800, 'KLINK TOWER', '2025-10-25',
		 '', 213500000, 'Draft', 0, 0, '')`)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	projectID, _ := res.LastInsertId()

	projectRepo := infrastructure.NewMySQLProjectRepository(db)
	activity := application.NewActivityService(infrastructure.NewMySQLActivityRepository(db))
	evidence := application.NewEvidenceService(infrastructure.NewMySQLEvidenceRepository(db), nil, nil, activity)
	clientPayments := application.NewClientPaymentService(infrastructure.NewMySQLClientPaymentRepository(db), evidence, activity)
	invoices := application.NewClientInvoiceService(infrastructure.NewMySQLClientInvoiceRepository(db), clientPayments, activity)
	templateRepo := infrastructure.NewMySQLPackageTemplateRepository(db)
	orderRepo := infrastructure.NewMySQLPackageOrderRepository(db)

	return &poFixture{
		db:        db,
		templates: application.NewPackageTemplateService(templateRepo),
		orders:    application.NewPackageOrderService(orderRepo, templateRepo, projectRepo, invoices, activity),
		invoices:  invoices,
		projects:  projectRepo,
		tenantID:  1,
		projectID: projectID,
	}
}

func pct(v float64) *float64 { return &v }
func fix(v int64) *int64     { return &v }

func (f *poFixture) seedTemplate(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	tmpl, err := f.templates.Create(ctx, f.tenantID, application.PackageTemplateInput{
		Name: "Paket Platinum 800 Pax", BasePrice: 150_000_000,
		DefaultTerms: "A. TAHAP PEMBAYARAN", DefaultBonusNote: "BONUS TAMBAHAN :", IsActive: true,
	})
	if err != nil {
		t.Fatalf("buat template: %v", err)
	}
	blocks := []domain.PackageTemplateBlock{
		{Category: "CATERING", Body: "BUFFET\nNasi Putih", QtyText: "700 PORSI", BonusNote: "BONUS :\nAfter Akad"},
		{Category: "CATERING", Body: "STALL / GUBUKAN\nPilihan Asia", QtyText: "150 PORSI", BonusNote: "BONUS :\nIce Cream"},
		{Category: "DEKORASI", Body: "Pelaminan Nasional", QtyText: "10-12 METER", BonusNote: ""},
	}
	if err := f.templates.ReplaceBlocks(ctx, f.tenantID, tmpl.ID, blocks); err != nil {
		t.Fatalf("simpan blok template: %v", err)
	}
	terms := []domain.PackageTemplateTerm{
		{Label: "Down Payment", Type: domain.PaymentDP, FixedAmount: fix(10_000_000), DaysBeforeEvent: 330},
		{Label: "Pembayaran 1", Type: domain.PaymentTermin, Percent: pct(30), DaysBeforeEvent: 210},
		{Label: "Pelunasan", Type: domain.PaymentPelunasan, Percent: pct(100), DaysBeforeEvent: 30},
	}
	if err := f.templates.ReplaceTerms(ctx, f.tenantID, tmpl.ID, terms); err != nil {
		t.Fatalf("simpan termin template: %v", err)
	}
	return tmpl.ID
}

func TestPackageOrder_AlurPenuh(t *testing.T) {
	f := setupPOFixture(t)
	ctx := context.Background()
	templateID := f.seedTemplate(t)

	// --- Apply: D23, nilai kontrak yang sudah ada menang atas harga daftar ---
	view, err := f.orders.ApplyTemplate(ctx, f.tenantID, f.projectID, templateID, 1)
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if view.Order.BasePrice != 213_500_000 {
		t.Errorf("basePrice = %d, mau 213500000 (contract_value project menang atas basePrice template 150jt, D23)", view.Order.BasePrice)
	}
	if len(view.Blocks) != 3 {
		t.Fatalf("blok tersalin = %d, mau 3", len(view.Blocks))
	}
	if len(view.Order.TermsPlan) != 3 {
		t.Fatalf("termsPlan tersalin = %d, mau 3 (D22)", len(view.Order.TermsPlan))
	}
	if view.Order.IsNumbered() {
		t.Error("PO Draft sudah bernomor — nomor harus NULL sampai terbit (D26)")
	}
	// PackageName ikut terisi (D28), kalau tidak header project jadi kosong.
	p, _ := f.projects.FindByID(ctx, f.tenantID, f.projectID)
	if p.PackageName != "Paket Platinum 800 Pax" {
		t.Errorf("packageName = %q, mau nama template (D28)", p.PackageName)
	}

	// --- Apply kedua ditolak oleh UNIQUE, bukan 500 ---
	if _, err := f.orders.ApplyTemplate(ctx, f.tenantID, f.projectID, templateID, 1); err == nil {
		t.Error("ApplyTemplate kedua kali berhasil — uq_project_package_orders_project seharusnya menolaknya")
	}

	// --- Penyesuaian bertanda + recompute contract_value (D2/D15) ---
	view, err = f.orders.ReplaceAdjustments(ctx, f.tenantID, f.projectID, []domain.ProjectPackageAdjustment{
		{ProjectID: f.projectID, Description: "Takeout busana", Amount: -1_500_000},
		{ProjectID: f.projectID, Description: "Add 100 buffet", Amount: 9_900_000},
	})
	if err != nil {
		t.Fatalf("ReplaceAdjustments: %v", err)
	}
	const wantTotal int64 = 213_500_000 - 1_500_000 + 9_900_000
	if view.Total != wantTotal {
		t.Errorf("total = %d, mau %d", view.Total, wantTotal)
	}
	p, _ = f.projects.FindByID(ctx, f.tenantID, f.projectID)
	if p.ContractValue != wantTotal {
		t.Errorf("projects.contract_value = %d, mau %d — recompute D15 tidak jalan, margin & outstanding akan melenceng", p.ContractValue, wantTotal)
	}

	// --- Issue: penomoran, snapshot, penyemaian tagihan ---
	view, err = f.orders.Issue(ctx, f.tenantID, f.projectID, 1, "Rara & Dafa", "08170043310")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !view.Order.IsNumbered() {
		t.Fatal("setelah Issue PO belum bernomor")
	}
	firstNumber := view.Order.PONumber
	if view.Order.Status != domain.PackageOrderIssued || view.Order.IssuedAt == nil {
		t.Errorf("status/issuedAt setelah Issue: %v / %v", view.Order.Status, view.Order.IssuedAt)
	}
	if view.Order.Snapshot == nil || len(view.Order.Snapshot.Current.Blocks) != 3 {
		t.Error("snapshot tidak dibekukan dengan benar (D6)")
	}
	if view.Order.Snapshot != nil && view.Order.Snapshot.Current.Event.Pax != 800 {
		t.Errorf("pax pada snapshot = %d, mau 800", view.Order.Snapshot.Current.Event.Pax)
	}

	invoices, err := f.invoices.List(ctx, f.projectID)
	if err != nil {
		t.Fatalf("List invoice: %v", err)
	}
	if len(invoices) != 3 {
		t.Fatalf("tagihan tersemai = %d, mau 3 (D8)", len(invoices))
	}
	var seeded int64
	for _, inv := range invoices {
		if inv.Status != domain.InvoiceDraft {
			t.Errorf("tagihan %s berstatus %s, mau Draft", inv.InvoiceNumber, inv.Status)
		}
		seeded += inv.Amount
	}
	if seeded != wantTotal {
		t.Errorf("jumlah tagihan tersemai = %d, mau %d — cicilan wajib berjumlah persis sama dengan total kontrak", seeded, wantTotal)
	}
	// Jatuh tempo diturunkan dari tanggal acara, jadi mustahil jatuh sesudahnya.
	eventDate := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	for _, inv := range invoices {
		if inv.DueDate.After(eventDate) {
			t.Errorf("tagihan %s jatuh tempo %s — setelah hari H", inv.InvoiceNumber, inv.DueDate.Format("2006-01-02"))
		}
	}

	// --- Menulis saat sudah terbit ditolak ---
	if _, err := f.orders.ReplaceAdjustments(ctx, f.tenantID, f.projectID, nil); err == nil {
		t.Error("penyesuaian bisa diubah saat PO sudah Terbit — harus lewat revisi")
	}

	// --- Revise lalu Issue ulang: nomor WAJIB sama (D26/G2) ---
	if _, err := f.orders.Revise(ctx, f.tenantID, f.projectID, 1); err != nil {
		t.Fatalf("Revise: %v", err)
	}
	view, err = f.orders.Issue(ctx, f.tenantID, f.projectID, 1, "Rara & Dafa", "08170043310")
	if err != nil {
		t.Fatalf("Issue ulang: %v", err)
	}
	if view.Order.PONumber != firstNumber {
		t.Errorf("nomor PO berubah %q -> %q setelah revisi — klien akan memegang dua dokumen bernomor beda untuk satu kontrak", firstNumber, view.Order.PONumber)
	}
	if view.Order.Revision != 1 {
		t.Errorf("revision = %d, mau 1", view.Order.Revision)
	}
	if view.Order.Snapshot == nil || len(view.Order.Snapshot.History) != 1 {
		t.Error("snapshot lama tidak diarsipkan ke history (D30)")
	}
	// Terbit ulang tidak boleh menyemai tagihan kedua kali.
	if again, _ := f.invoices.List(ctx, f.projectID); len(again) != 3 {
		t.Errorf("tagihan setelah terbit ulang = %d, mau tetap 3", len(again))
	}

	// --- Cancel hanya dari Terbit (D29) ---
	if _, err := f.orders.Cancel(ctx, f.tenantID, f.projectID, 1); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := f.orders.Cancel(ctx, f.tenantID, f.projectID, 1); err == nil {
		t.Error("Cancel kedua kali berhasil — hanya PO Terbit yang boleh dibatalkan")
	}

	// --- Hard delete project dengan PO terpasang: regresi FK 1451 ---
	if err := f.projects.DeleteCascade(ctx, f.tenantID, f.projectID); err != nil {
		t.Fatalf("DeleteCascade dengan PO terpasang gagal (FK 1451?): %v", err)
	}
}

// StartBlank is the other half of D23: a project with no template must keep
// the contract value it already had.
func TestPackageOrder_StartBlankMengadopsiNilaiKontrak(t *testing.T) {
	f := setupPOFixture(t)
	ctx := context.Background()

	view, err := f.orders.StartBlank(ctx, f.tenantID, f.projectID, 1)
	if err != nil {
		t.Fatalf("StartBlank: %v", err)
	}
	if view.Order.BasePrice != 213_500_000 {
		t.Errorf("basePrice = %d, mau 213500000 — nilai kontrak project lama tidak boleh dinolkan (D23)", view.Order.BasePrice)
	}
	if len(view.Blocks) != 0 {
		t.Errorf("blok = %d, mau 0", len(view.Blocks))
	}
}

// A takeout larger than the package must be refused rather than writing a
// negative contract value.
func TestPackageOrder_TotalNegatifDitolak(t *testing.T) {
	f := setupPOFixture(t)
	ctx := context.Background()
	if _, err := f.orders.StartBlank(ctx, f.tenantID, f.projectID, 1); err != nil {
		t.Fatalf("StartBlank: %v", err)
	}
	_, err := f.orders.ReplaceAdjustments(ctx, f.tenantID, f.projectID, []domain.ProjectPackageAdjustment{
		{ProjectID: f.projectID, Description: "Takeout berlebihan", Amount: -999_000_000},
	})
	if err == nil {
		t.Fatal("penyesuaian yang membuat total negatif diterima")
	}
	p, _ := f.projects.FindByID(ctx, f.tenantID, f.projectID)
	if p.ContractValue < 0 {
		t.Errorf("contract_value jadi negatif: %d", p.ContractValue)
	}
}
