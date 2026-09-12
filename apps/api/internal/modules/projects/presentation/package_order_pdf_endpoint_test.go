package presentation

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"jwswedding/internal/migrator"
	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/modules/projects/infrastructure"
	"jwswedding/internal/shared/database"
)

// HTTP-level test for the PO PDF endpoint, against a real database.
//
// The service layer and the PDF builder are covered elsewhere; what was never
// exercised is the HANDLER GLUE between them — fetching the view, the profile
// gate, logo/signature, the payment ledger, and writing the response. A fault
// anywhere in that chain shows up to the user only as "the button does
// nothing", which is exactly the report this test was written to answer.
const (
	pdfEpTestDB   = "jwswedding_pdf_endpoint_test"
	pdfEpAdminDSN = "root:@tcp(127.0.0.1:3306)/"
)

// stubPlatform supplies the tenant profile the PDF letterhead needs, with no
// `platform` module wired up. completeProfile=false reproduces the 422 gate.
type stubPlatform struct {
	complete bool
}

func (s stubPlatform) GetTenantProfile(ctx context.Context, tenantID int64) (platformcontracts.TenantProfile, error) {
	return platformcontracts.TenantProfile{
		BusinessName: "JWS Wedding", OwnerName: "Dimas", Phone: "0812", Email: "a@b.c",
		Address: "Jl. Melati", City: "Bandung",
		BankName: "BCA", BankAccountNumber: "123", BankAccountHolderName: "Dimas",
		AccentRGB: [3]int{150, 105, 46}, AccentDarkRGB: [3]int{107, 74, 31}, AccentSoftRGB: [3]int{244, 228, 204},
	}, nil
}

func (s stubPlatform) ProfileMissingFields(ctx context.Context, tenantID int64) ([]string, error) {
	if s.complete {
		return nil, nil
	}
	return []string{"Nama Bank", "No. Rekening"}, nil
}

// WritesAllowed backs the subscription guard. Irrelevant to a PDF download
// (a read), but part of the contract this stub has to satisfy.
func (s stubPlatform) WritesAllowed(ctx context.Context, tenantID int64) (bool, error) {
	return true, nil
}

func (s stubPlatform) GetTenantLogo(ctx context.Context, tenantID int64) ([]byte, string, bool, error) {
	return nil, "", false, nil
}

func (s stubPlatform) GetTenantSignature(ctx context.Context, tenantID int64) ([]byte, string, bool, error) {
	return nil, "", false, nil
}

func setupPDFEndpoint(t *testing.T, profileComplete bool) (*Handler, int64) {
	t.Helper()

	admin, err := sql.Open("mysql", pdfEpAdminDSN)
	if err != nil {
		t.Skipf("driver mysql tidak bisa dibuka, skip: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + pdfEpTestDB); err != nil {
		t.Fatalf("drop db uji: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + pdfEpTestDB); err != nil {
		t.Fatalf("create db uji: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", pdfEpAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + pdfEpTestDB)
	})

	url := fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", pdfEpTestDB)
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
	orders := application.NewPackageOrderService(orderRepo, templateRepo, projectRepo, invoices, activity)
	projectSvc := application.NewProjectService(projectRepo, infrastructure.NewMySQLMilestoneRepository(db),
		infrastructure.NewMySQLMilestoneTemplateRepository(db), infrastructure.NewMySQLVendorEngagementRepository(db),
		infrastructure.NewMySQLVendorMilestoneRepository(db), infrastructure.NewMySQLIssueRepository(db),
		infrastructure.NewMySQLPaymentRepository(db), infrastructure.NewMySQLVenuePaymentRepository(db),
		evidence, activity, nil)

	h := NewHandler(projectSvc, nil, nil, clientPayments, invoices, nil, nil, evidence, activity, nil,
		stubPlatform{complete: profileComplete}, orders)

	// Beri project sebuah PO Draft, sebagaimana keadaan saat tombol
	// "Pratinjau PDF" terlihat di layar.
	if _, err := orders.StartBlank(context.Background(), 1, projectID, 1); err != nil {
		t.Fatalf("StartBlank: %v", err)
	}
	if _, err := orders.ReplaceBlocks(context.Background(), 1, projectID, []domain.ProjectPackageBlock{
		{ProjectID: projectID, Category: "CATERING", Body: "BUFFET\nNasi Putih", QtyText: "700 PORSI", BonusNote: "BONUS :"},
	}); err != nil {
		t.Fatalf("ReplaceBlocks: %v", err)
	}
	return h, projectID
}

// The exact request the "Pratinjau PDF" button makes.
func TestDownloadPackageOrderPDF_DraftMengembalikanPDF(t *testing.T) {
	h, projectID := setupPDFEndpoint(t, true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/projects/%d/package-order/pdf", projectID), nil)
	h.downloadPackageOrderPDF(rec, req, staffClaims{tenantID: 1, staffID: 1, role: "Owner"}, projectID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, mau application/pdf", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Error("Content-Disposition kosong")
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("body bukan PDF, 40 byte pertama: %q", rec.Body.Bytes()[:min(40, rec.Body.Len())])
	}
	t.Logf("PDF terkirim: %d byte, disposition=%s", rec.Body.Len(), rec.Header().Get("Content-Disposition"))
}

// Profil usaha belum lengkap -> 422 dengan daftar field, BUKAN PDF rusak.
// Ini yang membedakan "tombol tidak berfungsi" karena gerbang profil dari
// karena bug sungguhan.
func TestDownloadPackageOrderPDF_ProfilBelumLengkap(t *testing.T) {
	h, projectID := setupPDFEndpoint(t, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/projects/%d/package-order/pdf", projectID), nil)
	h.downloadPackageOrderPDF(rec, req, staffClaims{tenantID: 1, staffID: 1, role: "Owner"}, projectID)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, mau 422 (body=%s)", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Profil usaha belum lengkap")) {
		t.Errorf("pesan 422 tidak menyebut profil usaha: %s", rec.Body.String())
	}
}
