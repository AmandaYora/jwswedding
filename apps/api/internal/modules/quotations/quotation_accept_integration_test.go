package quotations_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"jwswedding/internal/migrator"
	projectsapp "jwswedding/internal/modules/projects/application"
	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/modules/projects/infrastructure"
	"jwswedding/internal/modules/quotations/application"
	qcontracts "jwswedding/internal/modules/quotations/contracts"
	qdomain "jwswedding/internal/modules/quotations/domain"
	qinfra "jwswedding/internal/modules/quotations/infrastructure"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/database"
)

// Tes alur Accept ujung-ke-ujung lawan MySQL nyata (§9 PLAN
// penawaran-client-master): Issue → Accept (project + tagihan lahir) →
// Accept ulang idempoten → revisi harga mengalir ke project → hapus
// berjenjang + delete-impact. Pola yang sama dengan
// package_order_integration_test yang lama.
const (
	acceptTestDB       = "jwswedding_quotation_accept_test"
	acceptTestAdminDSN = "root:@tcp(127.0.0.1:3306)/"
)

type acceptFixture struct {
	quotations *application.QuotationService
	projects   *projectsapp.ProjectService
	invoices   *projectsapp.ClientInvoiceService
	db         *sql.DB
	tenantID   int64
	clientID   int64
}

type stubClientDirectory struct{}

func (stubClientDirectory) CoupleNames(_ context.Context, _, _ int64) (string, string, error) {
	return "Rara", "Dafa", nil
}
func (stubClientDirectory) CoupleNamesBatch(_ context.Context, _ int64, ids []int64) (map[int64][2]string, error) {
	out := make(map[int64][2]string, len(ids))
	for _, id := range ids {
		out[id] = [2]string{"Rara", "Dafa"}
	}
	return out, nil
}
func (stubClientDirectory) PhoneForClient(_ context.Context, _, _ int64) (string, error) {
	return "08170043310", nil
}
func (stubClientDirectory) SaveSpecimen(_ context.Context, _ int64, _ int64, _, _ string, _ []byte, _, _ string) error {
	return nil
}
func (stubClientDirectory) SpecimenImage(_ context.Context, _, _ int64) ([]byte, error) {
	return nil, nil
}
func (stubClientDirectory) SpecimenMeta(_ context.Context, _, _ int64) ([4]string, bool, error) {
	return [4]string{}, false, nil
}
func (stubClientDirectory) SignerOptions(_ context.Context, _, _ int64) ([][2]string, error) {
	return [][2]string{{"Bride", "Rara"}, {"Groom", "Dafa"}}, nil
}

// mapObjectStorage adalah ObjectStorage dalam memori untuk tes integrasi —
// TTD tidak butuh bucket sungguhan untuk mengunci alurnya.
type mapObjectStorage struct {
	mu   sync.Mutex
	docs map[string][]byte
}

func newMapObjectStorage() *mapObjectStorage {
	return &mapObjectStorage{docs: map[string][]byte{}}
}

func (m *mapObjectStorage) Save(_ context.Context, key string, data []byte, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := append([]byte(nil), data...)
	m.docs[key] = cp
	return key, nil
}

func (m *mapObjectStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.docs[key]
	if !ok {
		return nil, errors.New("object tidak ditemukan: " + key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mapObjectStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, key)
	return nil
}

// testSignaturePNG adalah PNG 1x1 valid — cukup untuk melewati
// normalizeSignatureImage (decode + re-encode nyata).
func testSignaturePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.Black)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG uji: %v", err)
	}
	return buf.Bytes()
}

func setupAcceptFixture(t *testing.T) *acceptFixture {
	t.Helper()

	admin, err := sql.Open("mysql", acceptTestAdminDSN)
	if err != nil {
		t.Skipf("driver mysql tidak bisa dibuka, skip: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("mysql lokal tidak bisa dihubungi, skip: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + acceptTestDB); err != nil {
		t.Fatalf("drop db uji: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + acceptTestDB); err != nil {
		t.Fatalf("create db uji: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("mysql", acceptTestAdminDSN)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + acceptTestDB)
	})

	url := fmt.Sprintf("mysql://root:@tcp(127.0.0.1:3306)/%s", acceptTestDB)
	if err := migrator.Up(url); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	db, err := database.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(
		`INSERT INTO project_milestone_templates (tenant_id, sort_order, name, category, days_before_event)
		 VALUES (1, 1, 'Survei Venue', 'Venue', 90), (1, 2, 'Gladi Resik', '', 1)`); err != nil {
		t.Fatalf("seed template timeline: %v", err)
	}
	res, err := db.Exec(
		`INSERT INTO clients (tenant_id, bride_name, groom_name, phone, email, notes)
		 VALUES (1, 'Rara', 'Dafa', '08170043310', '', '')`)
	if err != nil {
		t.Fatalf("seed client: %v", err)
	}
	clientID, _ := res.LastInsertId()

	activity := projectsapp.NewActivityService(infrastructure.NewMySQLActivityRepository(db))
	evidence := projectsapp.NewEvidenceService(infrastructure.NewMySQLEvidenceRepository(db), nil, nil, activity)
	clientPayments := projectsapp.NewClientPaymentService(infrastructure.NewMySQLClientPaymentRepository(db), evidence, activity)
	invoices := projectsapp.NewClientInvoiceService(infrastructure.NewMySQLClientInvoiceRepository(db), clientPayments, activity)
	projectRepo := infrastructure.NewMySQLProjectRepository(db)
	projectSvc := projectsapp.NewProjectService(projectRepo, infrastructure.NewMySQLMilestoneRepository(db),
		infrastructure.NewMySQLMilestoneTemplateRepository(db), infrastructure.NewMySQLVendorEngagementRepository(db),
		infrastructure.NewMySQLVendorMilestoneRepository(db), infrastructure.NewMySQLIssueRepository(db),
		infrastructure.NewMySQLPaymentRepository(db), infrastructure.NewMySQLVenuePaymentRepository(db),
		evidence, activity, nil)
	projectSvc.SetClientInvoiceService(invoices)
	projContracts := projectscontracts.New(projectSvc,
		projectsapp.NewVendorEngagementService(infrastructure.NewMySQLVendorEngagementRepository(db), infrastructure.NewMySQLVendorMilestoneRepository(db), activity),
		projectsapp.NewMilestoneTemplateService(infrastructure.NewMySQLMilestoneTemplateRepository(db)),
		clientPayments)

	quotSvc := application.NewQuotationService(qinfra.NewMySQLQuotationRepository(db), qinfra.NewMySQLPackageTemplateRepository(db), projContracts, newMapObjectStorage())
	quotSvc.SetClientDirectory(stubClientDirectory{})
	projectSvc.SetQuotationResolver(qcontracts.New(quotSvc))

	return &acceptFixture{quotations: quotSvc, projects: projectSvc, invoices: invoices, db: db, tenantID: 1, clientID: clientID}
}

func acceptInput() application.AcceptQuotationInput {
	return application.AcceptQuotationInput{
		ProjectName:   "Akad Rara & Dafa",
		PrepStartDate: time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
		Description:   "Deal dari penawaran",
		ActorStaffID:  1,
		ActorRole:     "Admin",
	}
}

func seedOffered(t *testing.T, f *acceptFixture, basePrice int64) int64 {
	t.Helper()
	qID := seedOfferedUnsigned(t, f, basePrice)
	// TTD WAJIB (D9): Accept menolak tanpa tanda tangan, jadi alur uji
	// meneken dulu — persis seperti jalur unggah pengelola.
	signOffered(t, f, qID)
	return qID
}

// seedOfferedUnsigned menyiapkan penawaran Ditawarkan TANPA tanda tangan —
// untuk tes yang mengunci gerbang penerbitan link / penandatanganan itu
// sendiri (keadaan "belum diteken" adalah prasyaratnya).
func seedOfferedUnsigned(t *testing.T, f *acceptFixture, basePrice int64) int64 {
	t.Helper()
	ctx := context.Background()

	tmplSvc := application.NewPackageTemplateService(qinfra.NewMySQLPackageTemplateRepository(mustDB(t, f)))
	tmpl, err := tmplSvc.Create(ctx, f.tenantID, application.PackageTemplateInput{
		Name: "Paket Silver", BasePrice: basePrice, IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create template: %v", err)
	}

	eventDate := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	view, err := f.quotations.Create(ctx, f.tenantID, 1, application.CreateQuotationInput{
		ClientID: f.clientID, EventDate: &eventDate, Pax: 800, TemplateID: tmpl.ID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.Total != basePrice {
		t.Fatalf("total awal = %d, mau %d (harga standar template)", view.Total, basePrice)
	}
	if _, err := f.quotations.ReplaceAdjustments(ctx, f.tenantID, view.Quotation.ID, []qdomain.QuotationAdjustment{
		{Description: "Add buffet", Amount: 10_000_000},
	}); err != nil {
		t.Fatalf("ReplaceAdjustments: %v", err)
	}
	issued, err := f.quotations.Issue(ctx, f.tenantID, view.Quotation.ID, 1)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if issued.Quotation.Status != "Ditawarkan" {
		t.Fatalf("status = %q, mau Ditawarkan", issued.Quotation.Status)
	}
	if issued.Quotation.PONumber == "" {
		t.Fatal("Ditawarkan tanpa nomor PO")
	}
	return view.Quotation.ID
}

// signOffered membubuhkan TTD jalur upload pada penawaran Ditawarkan.
func signOffered(t *testing.T, f *acceptFixture, qID int64) {
	t.Helper()
	if _, err := f.quotations.SignQuotation(context.Background(), f.tenantID, qID,
		"Bride", "Rara", testSignaturePNG(t), "image/png", application.SignatureChannelUpload); err != nil {
		t.Fatalf("SignQuotation: %v", err)
	}
}

func mustDB(t *testing.T, f *acceptFixture) *sql.DB {
	t.Helper()
	return f.db
}

func TestAccept_MembuatProjectDanTagihan(t *testing.T) {
	f := setupAcceptFixture(t)
	ctx := context.Background()
	qID := seedOffered(t, f, 200_000_000)

	view, projectID, err := f.quotations.Accept(ctx, f.tenantID, qID, acceptInput())
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if view.Quotation.Status != "Diterima" {
		t.Errorf("status = %q, mau Diterima", view.Quotation.Status)
	}
	if projectID == 0 {
		t.Fatal("Accept tidak mengembalikan projectID")
	}

	p, err := f.projects.Get(ctx, f.tenantID, projectID)
	if err != nil {
		t.Fatalf("baca project: %v", err)
	}
	// 200jt + 10jt adjustment.
	if p.ContractValue != 210_000_000 {
		t.Errorf("contract_value = %d, mau 210000000", p.ContractValue)
	}
	if p.QuotationID != qID || p.ClientID != f.clientID {
		t.Errorf("relasi project salah: %+v", p)
	}
	if p.Status != domain.StatusPreparation {
		t.Errorf("status project = %q, mau Preparation", p.Status)
	}

	milestones, err := f.projects.ListMilestones(ctx, f.tenantID, projectID)
	if err != nil {
		t.Fatalf("baca timeline: %v", err)
	}
	if len(milestones) != 2 {
		t.Errorf("timeline = %d baris, mau 2 dari template", len(milestones))
	}

	// Rencana termin sudah dihapus dari penawaran, jadi Accept TIDAK lagi
	// menyemai tagihan apa pun — klien mencicil sebebas yang disepakati, dan
	// setiap Tagihan diterbitkan staff sendiri di tab Pembayaran.
	invoices, err := f.invoices.List(ctx, projectID)
	if err != nil {
		t.Fatalf("baca tagihan: %v", err)
	}
	if len(invoices) != 0 {
		t.Errorf("tagihan hasil Accept = %d baris, mau 0 (tidak ada penyemaian)", len(invoices))
	}

	// Accept ulang: project yang SAMA, bukan yang kedua.
	_, projectID2, err := f.quotations.Accept(ctx, f.tenantID, qID, acceptInput())
	if err != nil {
		t.Fatalf("Accept ulang: %v", err)
	}
	if projectID2 != projectID {
		t.Errorf("Accept ulang membuat project %d, mau %d yang sama", projectID2, projectID)
	}
	var nProjects int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE tenant_id = ?`, f.tenantID).Scan(&nProjects); err != nil {
		t.Fatalf("hitung project: %v", err)
	}
	if nProjects != 1 {
		t.Errorf("projects = %d baris, mau 1", nProjects)
	}
}

// Regression untuk lubang Revisi→Draft→Hapus: status saat dihapus sudah
// Draft, tapi project-nya tetap ada — Delete harus menolak (hapus
// project-nya dulu), dan DeleteImpact harus tetap menyebut project itu
// supaya dialog bisa memblokir, bukan diam.
func TestAccept_RevisiLaluHapus_DitolakKarenaBerproject(t *testing.T) {
	f := setupAcceptFixture(t)
	ctx := context.Background()
	qID := seedOffered(t, f, 200_000_000)
	_, projectID, err := f.quotations.Accept(ctx, f.tenantID, qID, acceptInput())
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	revised, err := f.quotations.Revise(ctx, f.tenantID, qID)
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}
	if string(revised.Quotation.Status) != "Draft" {
		t.Fatalf("status revisi = %q, mau Draft", revised.Quotation.Status)
	}

	impact, err := f.quotations.DeleteImpact(ctx, f.tenantID, qID)
	if err != nil {
		t.Fatalf("DeleteImpact: %v", err)
	}
	if impact.ProjectID != projectID {
		t.Errorf("impact Draft ber-project tidak menyebut project %d: %+v", projectID, impact)
	}

	if err := f.quotations.Delete(ctx, f.tenantID, qID); err == nil {
		t.Fatalf("Delete Draft ber-project = success, mau ditolak hapus-project-dulu")
	} else {
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
			t.Fatalf("Delete Draft ber-project error = %v, mau Validation", err)
		}
	}

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE id = ?`, projectID).Scan(&n); err != nil {
		t.Fatalf("cek project: %v", err)
	}
	if n != 1 {
		t.Errorf("project sisa %d baris, mau 1 (tidak boleh ikut hilang)", n)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM quotations WHERE id = ?`, qID).Scan(&n); err != nil {
		t.Fatalf("cek penawaran: %v", err)
	}
	if n != 1 {
		t.Errorf("penawaran sisa %d baris, mau 1 (penolakan tidak boleh menghapus)", n)
	}
}

func TestAccept_RevisiHargaMengalirKeProject(t *testing.T) {
	f := setupAcceptFixture(t)
	ctx := context.Background()
	qID := seedOffered(t, f, 200_000_000)
	_, projectID, err := f.quotations.Accept(ctx, f.tenantID, qID, acceptInput())
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	// Revisi setelah Diterima: dokumen sama, nomor tetap, revision++.
	revised, err := f.quotations.Revise(ctx, f.tenantID, qID)
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}
	if revised.Quotation.Revision != 1 {
		t.Errorf("revision = %d, mau 1", revised.Quotation.Revision)
	}
	firstNumber := revised.Quotation.PONumber

	// Naikkan harga 5jt — SyncContractValue mendorong nilai kontraknya ke
	// project. Tagihan tidak ikut disentuh (lihat SyncContractValue).
	if _, err := f.quotations.ReplaceAdjustments(ctx, f.tenantID, qID, []qdomain.QuotationAdjustment{
		{Description: "Add buffet", Amount: 15_000_000},
	}); err != nil {
		t.Fatalf("ReplaceAdjustments: %v", err)
	}
	p, err := f.projects.Get(ctx, f.tenantID, projectID)
	if err != nil {
		t.Fatalf("baca project: %v", err)
	}
	if p.ContractValue != 215_000_000 {
		t.Errorf("contract_value = %d, mau 215000000 setelah revisi", p.ContractValue)
	}

	// Terbitkan ulang: kembali Diterima (project sudah ada), nomor tetap.
	reissued, err := f.quotations.Issue(ctx, f.tenantID, qID, 1)
	if err != nil {
		t.Fatalf("Issue ulang: %v", err)
	}
	if reissued.Quotation.Status != "Diterima" || reissued.Quotation.PONumber != firstNumber {
		t.Errorf("re-issue = %q %q, mau Diterima dengan nomor tetap", reissued.Quotation.Status, reissued.Quotation.PONumber)
	}
}

func TestAccept_HapusBerjenjangDanImpact(t *testing.T) {
	f := setupAcceptFixture(t)
	ctx := context.Background()
	qID := seedOffered(t, f, 200_000_000)
	_, projectID, err := f.quotations.Accept(ctx, f.tenantID, qID, acceptInput())
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	// Terbitkan satu tagihan lalu lunaskan, supaya impact menyebut angka
	// berbayar. Dibuat eksplisit di sini: Accept tidak lagi menyemai tagihan.
	inv, err := f.invoices.Create(ctx, f.tenantID, projectID, 1, projectsapp.ClientInvoiceInput{
		Type: domain.PaymentDP, Description: "DP", Amount: 20_000_000, DueDate: time.Now(),
	})
	if err != nil {
		t.Fatalf("Create tagihan: %v", err)
	}
	if _, err := f.invoices.MarkPaid(ctx, projectID, inv.ID, 1, projectsapp.ClientPaymentInput{
		Type: inv.Type, Amount: inv.Amount, PaymentDate: time.Now(), Method: "Transfer",
	}); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}

	impact, err := f.quotations.DeleteImpact(ctx, f.tenantID, qID)
	if err != nil {
		t.Fatalf("DeleteImpact: %v", err)
	}
	if impact.ProjectID != projectID || impact.ProjectName == "" {
		t.Errorf("impact tidak menyebut project: %+v", impact)
	}
	if impact.PaidInvoiceCount != 1 || impact.PaidInvoiceTotal != inv.Amount {
		t.Errorf("impact berbayar = %d/%d, mau 1/%d", impact.PaidInvoiceCount, impact.PaidInvoiceTotal, inv.Amount)
	}

	// Penawaran ber-project DITOLAK dihapus: satu-satunya jalan adalah lewat
	// penghapusan project-nya (yang ikut menyapu penawaran). Mengandalkan
	// label status pernah meloloskan yatim via Revisi→Draft→Hapus.
	if err := f.quotations.Delete(ctx, f.tenantID, qID); err == nil {
		t.Fatalf("Delete ber-project = success, mau ditolak hapus-project-dulu")
	} else {
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
			t.Fatalf("Delete ber-project error = %v, mau Validation", err)
		}
	}

	// Hapus lewat project: project + penawaran + seluruh isi hilang bersama.
	if err := f.projects.Delete(ctx, f.tenantID, projectID); err != nil {
		t.Fatalf("Delete project: %v", err)
	}
	for _, check := range []struct {
		table, where string
		args         []interface{}
	}{
		{"quotations", "id = ?", []interface{}{qID}},
		{"projects", "id = ?", []interface{}{projectID}},
		{"client_invoices", "project_id = ?", []interface{}{projectID}},
		{"client_payments", "project_id = ?", []interface{}{projectID}},
		{"project_milestones", "project_id = ?", []interface{}{projectID}},
	} {
		var n int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM `+check.table+` WHERE `+check.where, check.args...).Scan(&n); err != nil {
			t.Fatalf("cek %s: %v", check.table, err)
		}
		if n != 0 {
			t.Errorf("%s sisa %d baris setelah hapus berjenjang", check.table, n)
		}
	}
}
