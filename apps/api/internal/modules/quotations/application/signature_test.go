package application

// Tes TTD Penawaran (§13 PLAN ttd-penawaran): salin-saat-meneken (D6c),
// gerbang D13, TTD wajib (D9), kompensasi link (D14), dan perilaku magic
// link — tanpa database, lewat fake yang meniru semantik kuncinya
// (MarkUsed bersyarat, UNIQUE specimen).

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"
	"time"

	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

// --- Fake penyimpanan objek dalam memori ---

type fakeObjectStorage struct {
	docs    map[string][]byte
	deleted []string
	saveErr error
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{docs: map[string][]byte{}}
}

func (f *fakeObjectStorage) Save(_ context.Context, key string, data []byte, _ string) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.docs[key] = append([]byte(nil), data...)
	return key, nil
}

func (f *fakeObjectStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := f.docs[key]
	if !ok {
		return nil, errors.New("object tidak ditemukan: " + key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeObjectStorage) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	delete(f.docs, key)
	return nil
}

// --- Fake direktori client yang bisa diatur ---

type stubDirectory struct {
	bride, groom string
	meta         [4]string
	hasSpecimen  bool
	image        []byte
	imageErr     error
	saved        int
	saveErr      error
}

func newStubDirectory() *stubDirectory {
	return &stubDirectory{bride: "Rara", groom: "Dafa"}
}

func (s *stubDirectory) CoupleNames(_ context.Context, _, _ int64) (string, string, error) {
	return s.bride, s.groom, nil
}

func (s *stubDirectory) CoupleNamesBatch(_ context.Context, _ int64, ids []int64) (map[int64][2]string, error) {
	out := make(map[int64][2]string, len(ids))
	for _, id := range ids {
		out[id] = [2]string{s.bride, s.groom}
	}
	return out, nil
}

func (s *stubDirectory) PhoneForClient(_ context.Context, _, _ int64) (string, error) {
	return "08170043310", nil
}

func (s *stubDirectory) SaveSpecimen(_ context.Context, _ int64, _ int64, _, _ string, _ []byte, _, _ string) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved++
	return nil
}

func (s *stubDirectory) SpecimenImage(_ context.Context, _, _ int64) ([]byte, error) {
	if s.imageErr != nil {
		return nil, s.imageErr
	}
	if !s.hasSpecimen {
		return nil, apperror.NotFound("TTD tersimpan tidak ditemukan")
	}
	return s.image, nil
}

func (s *stubDirectory) SpecimenMeta(_ context.Context, _, _ int64) ([4]string, bool, error) {
	return s.meta, s.hasSpecimen, nil
}

func (s *stubDirectory) SignerOptions(_ context.Context, _, _ int64) ([][2]string, error) {
	return [][2]string{{"Bride", s.bride}, {"Groom", s.groom}}, nil
}

// --- Fake repo link dengan semantik MarkUsed bersyarat ---

type fakeLinkRepo struct {
	rows   map[int64]*domain.SignatureLink
	byHash map[string]*domain.SignatureLink
	nextID int64
}

func newFakeLinkRepo() *fakeLinkRepo {
	return &fakeLinkRepo{rows: map[int64]*domain.SignatureLink{}, byHash: map[string]*domain.SignatureLink{}, nextID: 1}
}

func (f *fakeLinkRepo) Create(_ context.Context, l *domain.SignatureLink) error {
	l.ID = f.nextID
	f.nextID++
	cp := *l
	f.rows[l.ID] = &cp
	f.byHash[l.TokenHash] = &cp
	return nil
}

func (f *fakeLinkRepo) FindByTokenHash(_ context.Context, hash string) (*domain.SignatureLink, error) {
	if l, ok := f.byHash[hash]; ok {
		cp := *l
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeLinkRepo) MarkUsed(_ context.Context, id int64) (time.Time, bool, error) {
	l, ok := f.rows[id]
	if !ok {
		return time.Time{}, false, nil
	}
	if l.UsedAt != nil {
		return time.Time{}, false, nil
	}
	now := time.Now().UTC().Truncate(time.Second)
	l.UsedAt = &now
	f.byHash[l.TokenHash] = l
	return now, true, nil
}

func (f *fakeLinkRepo) ReleaseUsed(_ context.Context, id int64, usedAt time.Time) error {
	l, ok := f.rows[id]
	if !ok {
		return nil
	}
	if l.UsedAt == nil || !l.UsedAt.Equal(usedAt) {
		return nil
	}
	l.UsedAt = nil
	return nil
}

func (f *fakeLinkRepo) RevokeAllForQuotation(_ context.Context, _, quotationID int64) error {
	now := time.Now()
	for _, l := range f.rows {
		if l.QuotationID == quotationID && l.RevokedAt == nil && l.UsedAt == nil {
			l.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeLinkRepo) usable(id int64) bool {
	l, ok := f.rows[id]
	return ok && l.IsUsable(time.Now())
}

// --- Perakitan ---

type signFixture struct {
	svc      *QuotationService
	repo     *fakeQuotationRepo
	projects *fakeProjectsContracts
	storage  *fakeObjectStorage
	clients  *stubDirectory
	links    *fakeLinkRepo
	linkSvc  *SignatureLinkService
}

func newSignFixture() *signFixture {
	repo := newFakeQuotationRepo()
	projects := &fakeProjectsContracts{}
	storage := newFakeObjectStorage()
	clients := newStubDirectory()
	svc := NewQuotationService(repo, fakeQuotationTemplates{}, projects, storage)
	svc.SetClientDirectory(clients)
	links := newFakeLinkRepo()
	return &signFixture{
		svc: svc, repo: repo, projects: projects,
		storage: storage, clients: clients,
		links: links, linkSvc: NewSignatureLinkService(links, svc),
	}
}

func seedSignable(repo *fakeQuotationRepo, status domain.QuotationStatus) *domain.Quotation {
	eventDate := time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)
	return repo.seed(&domain.Quotation{
		TenantID: 1, ClientID: 7, Status: status, BasePrice: 100_000_000, PackageName: "Silver",
		PONumber: "PO/202612/0001", EventDate: &eventDate,
		Snapshot: &domain.QuotationSnapshot{
			Current: domain.QuotationRevision{
				Revision: 0, IssuedAt: time.Now(), BasePrice: 100_000_000, PackageName: "Silver",
			},
		},
	})
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.Black)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG uji: %v", err)
	}
	return buf.Bytes()
}

func asAppError(t *testing.T, err error) *apperror.AppError {
	t.Helper()
	if err == nil {
		t.Fatal("error nil, mau AppError")
	}
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v (%T), mau *apperror.AppError", err, err)
	}
	return appErr
}

// --- D6c: salinan milik dokumen ---

func TestSignQuotation_MenyimpanSalinanMilikDokumen(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	img := testPNG(t)

	view, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", img, "image/png", SignatureChannelUpload)
	if err != nil {
		t.Fatalf("SignQuotation: %v", err)
	}
	sig := view.Quotation.Snapshot.Current.Signature
	if sig == nil {
		t.Fatal("snapshot tidak berTTD setelah SignQuotation")
	}
	if sig.SignerName != "Rara" || sig.SignerRole != "Bride" || sig.Channel != SignatureChannelUpload {
		t.Errorf("signature = %+v, mau Rara/Bride/upload", sig)
	}
	wantPrefix := "jwswedding/signature/quotation/1/"
	if len(sig.StorageKey) < len(wantPrefix) || sig.StorageKey[:len(wantPrefix)] != wantPrefix {
		t.Errorf("storageKey = %q, mau berawalan %q (salinan milik dokumen)", sig.StorageKey, wantPrefix)
	}
	specimenKey := "jwswedding/signature/client/1/7/specimen.png"
	if sig.StorageKey == specimenKey {
		t.Errorf("storageKey menunjuk specimen — dokumen akan ikut berubah saat specimen ditimpa/dihapus (D6c)")
	}
	if view.Quotation.Status != domain.QuotationAccepted {
		t.Errorf("status = %q, mau Diterima", view.Quotation.Status)
	}
	if view.Quotation.AcceptedAt == nil {
		t.Error("AcceptedAt nil setelah diteken dari Ditawarkan")
	}
	if f.clients.saved != 1 {
		t.Errorf("SaveSpecimen dipanggil %d kali, mau 1 (jalur upload memperbarui master, D10)", f.clients.saved)
	}
}

func TestSignQuotation_SudahBerTTD_Ditolak(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	o.Snapshot.Current.Signature = &domain.QuotationClientSignature{SignerName: "Rara"}
	f.repo.rows[o.ID] = o

	_, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Groom", "Dafa", testPNG(t), "image/png", SignatureChannelUpload)
	appErr := asAppError(t, err)
	if appErr.Kind != apperror.KindValidation || appErr.Fields["signature"] == nil {
		t.Errorf("error = %v, mau Validation ber-field signature", err)
	}
}

func TestSignQuotation_Draft_Ditolak(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationDraft)

	_, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelUpload)
	if appErr := asAppError(t, err); appErr.Kind != apperror.KindValidation {
		t.Errorf("error = %v, mau Validation", err)
	}
}

func TestSignQuotation_TolakJenisBerkasLain(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	_, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", []byte("%PDF-1.4"), "application/pdf", SignatureChannelUpload)
	if appErr := asAppError(t, err); appErr.Kind != apperror.KindValidation {
		t.Errorf("error = %v, mau Validation", err)
	}
	if len(f.storage.docs) != 0 {
		t.Error("storage tersentuh padahal jenis berkas ditolak")
	}
}

func TestSignQuotation_TolakTerlaluBesar(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	big := make([]byte, maxSignatureBytes+1)
	_, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", big, "image/png", SignatureChannelUpload)
	if appErr := asAppError(t, err); appErr.Kind != apperror.KindValidation {
		t.Errorf("error = %v, mau Validation", err)
	}
}

// --- D13: gerbang bersandar pada TTD, bukan status ---

func TestSign_RevisiPascaProject_DiizinkanTanpaMenyentuhStatusDanProject(t *testing.T) {
	f := newSignFixture()
	acceptedAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	o := seedSignable(f.repo, domain.QuotationAccepted)
	o.AcceptedAt = &acceptedAt
	f.repo.rows[o.ID] = o
	f.projects.projectID = 55

	view, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Groom", "Dafa", testPNG(t), "image/png", SignatureChannelUpload)
	if err != nil {
		t.Fatalf("SignQuotation revisi pasca-project: %v", err)
	}
	if view.Quotation.Status != domain.QuotationAccepted {
		t.Errorf("status = %q, mau tetap Diterima", view.Quotation.Status)
	}
	if !view.Quotation.AcceptedAt.Equal(acceptedAt) {
		t.Errorf("AcceptedAt bergeser menjadi %v, mau tetap %v", view.Quotation.AcceptedAt, acceptedAt)
	}
	if view.ProjectID != 55 {
		t.Errorf("projectID = %d, mau tetap 55", view.ProjectID)
	}
	if view.Quotation.Snapshot.Current.Signature == nil {
		t.Error("snapshot tidak berTTD setelah diteken")
	}
	if f.projects.created != 0 {
		t.Error("project baru lahir saat menandatangani revisi — meneken bukan menerima (D13)")
	}
}

func TestSign_RevisiSudahBerTTD_Ditolak422(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationAccepted)
	o.Snapshot.Current.Signature = &domain.QuotationClientSignature{SignerName: "Rara"}
	f.repo.rows[o.ID] = o
	f.projects.projectID = 55

	_, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelUpload)
	appErr := asAppError(t, err)
	if appErr.Kind != apperror.KindValidation {
		t.Errorf("error = %v, mau 422 Validation", err)
	}
}

// --- D9 + T2: gerbang Accept ---

func acceptInputForSign() AcceptQuotationInput {
	return AcceptQuotationInput{
		ProjectName: "Akad Rara & Dafa", PrepStartDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		ActorStaffID: 1, ActorRole: "Admin",
	}
}

func TestAccept_TanpaTTD_Ditolak422(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	_, _, err := f.svc.Accept(context.Background(), 1, o.ID, acceptInputForSign())
	appErr := asAppError(t, err)
	if appErr.Kind != apperror.KindValidation || appErr.Fields["signature"] == nil {
		t.Errorf("error = %v, mau Validation ber-field signature (D9)", err)
	}
	if f.projects.created != 0 {
		t.Error("project lahir padahal Accept ditolak")
	}
}

func TestAccept_DiterimaTanpaProject_BolehMembuatProject(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	// Alur D1: klien meneken lewat magic link → Diterima + TTD, project
	// belum lahir. Pengelola menyusul lewat Accept (T2): cabang idempoten
	// tidak menemukan project, lalu jatuh ke pembuatan — bukan 422.
	if _, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelMagicLink); err != nil {
		t.Fatalf("SignQuotation: %v", err)
	}
	if stored := f.repo.rows[o.ID]; stored.Status != domain.QuotationAccepted {
		t.Fatalf("status = %q, mau Diterima (D1)", stored.Status)
	}
	_, projectID, err := f.svc.Accept(context.Background(), 1, o.ID, acceptInputForSign())
	if err != nil {
		t.Fatalf("Accept Diterima-tanpa-project: %v", err)
	}
	if projectID == 0 {
		t.Error("project tidak lahir dari Diterima-tanpa-project")
	}
}

func TestAccept_DiterimaDenganProject_TanpaTTD_TetapIdempoten(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationAccepted)
	f.projects.projectID = 77

	// Project lama tanpa TTD (pra-fitur) tidak boleh tersandera aturan baru:
	// cabang idempoten mengembalikan project yang sama TANPA memeriksa TTD.
	_, projectID, err := f.svc.Accept(context.Background(), 1, o.ID, acceptInputForSign())
	if err != nil {
		t.Fatalf("Accept idempoten: %v", err)
	}
	if projectID != 77 {
		t.Errorf("projectID = %d, mau 77 yang sama", projectID)
	}
	if f.projects.created != 0 {
		t.Error("project kedua lahir dari Accept idempoten")
	}
}

func TestAccept_TTDDariUnggahan_Lolos(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	if _, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelUpload); err != nil {
		t.Fatalf("SignQuotation: %v", err)
	}
	if _, projectID, err := f.svc.Accept(context.Background(), 1, o.ID, acceptInputForSign()); err != nil {
		t.Fatalf("Accept: %v", err)
	} else if projectID == 0 {
		t.Error("project tidak lahir")
	}
}

func TestAccept_TTDDariSpecimen_Lolos(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	f.clients.hasSpecimen = true
	f.clients.meta = [4]string{"Bride", "Rara", "draw", "2026-09-10"}
	f.clients.image = testPNG(t)

	view, err := f.svc.SignFromSpecimen(context.Background(), 1, o.ID, "Bride", "")
	if err != nil {
		t.Fatalf("SignFromSpecimen: %v", err)
	}
	if view.Quotation.Snapshot.Current.Signature == nil {
		t.Fatal("snapshot tidak berTTD")
	}
	if view.Quotation.Snapshot.Current.Signature.Channel != SignatureChannelSpecimen {
		t.Errorf("channel = %q, mau specimen", view.Quotation.Snapshot.Current.Signature.Channel)
	}
	if f.clients.saved != 0 {
		t.Error("specimen disentuh padahal isinya sudah sama")
	}
	if _, projectID, err := f.svc.Accept(context.Background(), 1, o.ID, acceptInputForSign()); err != nil {
		t.Fatalf("Accept: %v", err)
	} else if projectID == 0 {
		t.Error("project tidak lahir")
	}
}

func TestAccept_TTDDariSnapshot_Lolos(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	// TTD dari jalur mana pun sudah mendarat di snapshot (mis. magic link
	// lebih dulu) — Accept tidak butuh TTD baru di body-nya.
	if _, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelMagicLink); err != nil {
		t.Fatalf("SignQuotation: %v", err)
	}
	if _, _, err := f.svc.Accept(context.Background(), 1, o.ID, acceptInputForSign()); err != nil {
		t.Fatalf("Accept: %v", err)
	}
}

func TestAccept_PakaiSpecimen_PemilikBerbeda_Ditolak422(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	f.clients.hasSpecimen = true
	f.clients.meta = [4]string{"Groom", "Dafa", "upload", "2026-09-10"}
	f.clients.image = testPNG(t)

	// Memakai TTD milik Dafa di bawah nama Rara = pemalsuan oleh sistem (D12a).
	_, err := f.svc.SignFromSpecimen(context.Background(), 1, o.ID, "Bride", "")
	appErr := asAppError(t, err)
	if appErr.Kind != apperror.KindValidation || appErr.Fields["role"] == nil {
		t.Errorf("error = %v, mau Validation ber-field role (D12a)", err)
	}
	stored := f.repo.rows[o.ID]
	if stored.Snapshot.Current.Signature != nil {
		t.Error("snapshot ikut berTTD padahal pemakaian ditolak")
	}
}

func TestSignFromSpecimen_TanpaSpecimen_404(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	_, err := f.svc.SignFromSpecimen(context.Background(), 1, o.ID, "Bride", "")
	if appErr := asAppError(t, err); appErr.Kind != apperror.KindNotFound {
		t.Errorf("error = %v, mau 404 — dialog meminta unggah foto", err)
	}
}

// --- D12 + D6c: specimen tergantikan, dokumen lama utuh ---

func TestSaveSpecimenLintasPeran_TidakMengubahSnapshotLama(t *testing.T) {
	f := newSignFixture()
	first := seedSignable(f.repo, domain.QuotationOffered)
	second := seedSignable(f.repo, domain.QuotationOffered)
	second.PONumber = "PO/202612/0002"
	f.repo.rows[second.ID] = second

	if _, err := f.svc.SignQuotation(context.Background(), 1, first.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelUpload); err != nil {
		t.Fatalf("teken pertama: %v", err)
	}
	firstKey := f.repo.rows[first.ID].Snapshot.Current.Signature.StorageKey

	// Mempelai pria meneken PO-002 sesudahnya: specimen tergantikan (D12),
	// tetapi PO-001 tetap menampilkan tanda tangan yang benar (D6c).
	if _, err := f.svc.SignQuotation(context.Background(), 1, second.ID, "Groom", "Dafa", testPNG(t), "image/png", SignatureChannelUpload); err != nil {
		t.Fatalf("teken kedua: %v", err)
	}
	kept := f.repo.rows[first.ID].Snapshot.Current.Signature
	if kept == nil || kept.SignerName != "Rara" || kept.StorageKey != firstKey {
		t.Errorf("snapshot PO-001 berubah menjadi %+v — salinan dokumen ikut tertimpa specimen (D6c)", kept)
	}
}

// --- D5/T4: arsip saat revisi, revisi baru kosong ---

func TestRevise_TTDTerarsipDanRevisiBaruKosong(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	if _, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelUpload); err != nil {
		t.Fatalf("SignQuotation: %v", err)
	}
	revised, err := f.svc.Revise(context.Background(), 1, o.ID)
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}
	// TTD terarsip ke History (klausul pembatalan merujuk kesepakatan awal)...
	if len(revised.Quotation.Snapshot.History) != 1 || revised.Quotation.Snapshot.History[0].Signature == nil {
		t.Fatalf("history = %+v, mau 1 revisi berTTD", revised.Quotation.Snapshot.History)
	}
	// ...lalu Issue membangun revisi baru dari nol: Signature otomatis nil
	// (D5) — tanpa kode penyalinan apa pun di reopen/Issue.
	f.projects.projectID = 0
	reissued, err := f.svc.Issue(context.Background(), 1, o.ID, 1)
	if err != nil {
		t.Fatalf("Issue ulang: %v", err)
	}
	if reissued.Quotation.Snapshot.Current.Signature != nil {
		t.Error("revisi baru membawa TTD lama — harus teken ulang (D5)")
	}
}

// --- Magic link: Issue, Resolve, Accept, Reject ---

func issueLink(t *testing.T, f *signFixture, qID int64) string {
	t.Helper()
	token, _, err := f.linkSvc.Issue(context.Background(), 1, qID, 9)
	if err != nil {
		t.Fatalf("Issue link: %v", err)
	}
	if token == "" {
		t.Fatal("token kosong")
	}
	return token
}

func TestSignatureLink_Issue_MencabutLinkLama(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)

	first := issueLink(t, f, o.ID)
	second := issueLink(t, f, o.ID)
	if first == second {
		t.Fatal("dua Issue menghasilkan token yang sama")
	}
	resolved, err := f.linkSvc.Resolve(context.Background(), second)
	if err != nil {
		t.Fatalf("Resolve link baru: %v", err)
	}
	if resolved.Link.QuotationID != o.ID {
		t.Errorf("link menunjuk quotation %d, mau %d", resolved.Link.QuotationID, o.ID)
	}
	// Link lama mati saat yang baru terbit (D8).
	if _, err := f.linkSvc.Resolve(context.Background(), first); err == nil {
		t.Error("link lama masih berlaku setelah link baru terbit (D8)")
	} else if appErr := asAppError(t, err); appErr.Kind != apperror.KindNotFound {
		t.Errorf("error link lama = %v, mau 404", err)
	}
}

func TestSignatureLink_Resolve_Penolakan(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	token := issueLink(t, f, o.ID)

	cases := map[string]func(){
		"Kedaluwarsa": func() {
			for _, l := range f.links.rows {
				past := time.Now().Add(-time.Hour)
				l.ExpiresAt = past
			}
		},
		"SudahDipakai": func() {
			for _, l := range f.links.rows {
				now := time.Now()
				l.UsedAt = &now
			}
		},
		"Dicabut": func() {
			for _, l := range f.links.rows {
				now := time.Now()
				l.RevokedAt = &now
			}
		},
	}
	for name, sabotage := range cases {
		t.Run(name, func(t *testing.T) {
			sabotage()
			_, err := f.linkSvc.Resolve(context.Background(), token)
			if appErr := asAppError(t, err); appErr.Kind != apperror.KindNotFound {
				t.Errorf("error = %v, mau 404 Link tidak berlaku", err)
			}
		})
		// Kembalikan untuk kasus berikutnya.
		for _, l := range f.links.rows {
			l.UsedAt = nil
			l.RevokedAt = nil
			l.ExpiresAt = time.Now().Add(time.Hour)
		}
	}
	if _, err := f.linkSvc.Resolve(context.Background(), "token-salah"); err == nil {
		t.Error("token salah lolos Resolve")
	} else if appErr := asAppError(t, err); appErr.Kind != apperror.KindNotFound {
		t.Errorf("error token salah = %v, mau 404", err)
	}
}

func TestSignatureLink_Accept_TidakMembuatProject(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	token := issueLink(t, f, o.ID)

	if err := f.linkSvc.AcceptViaLink(context.Background(), token, "Bride", testPNG(t), "image/png"); err != nil {
		t.Fatalf("AcceptViaLink: %v", err)
	}
	stored := f.repo.rows[o.ID]
	if stored.Status != domain.QuotationAccepted {
		t.Errorf("status = %q, mau Diterima", stored.Status)
	}
	if stored.Snapshot.Current.Signature == nil || stored.Snapshot.Current.Signature.Channel != SignatureChannelMagicLink {
		t.Errorf("signature = %+v, mau ber-channel magic_link", stored.Snapshot.Current.Signature)
	}
	// D1: project BELUM lahir — pengelola menyusul mengisi Booking & PIC.
	if f.projects.created != 0 {
		t.Error("project lahir dari magic link — melanggar D1")
	}
	if pid, _ := f.projects.ProjectIDForQuotation(context.Background(), 1, o.ID); pid != 0 {
		t.Errorf("ProjectIDForQuotation = %d, mau 0", pid)
	}
	// Link yang sama tidak bisa dipakai dua kali.
	if err := f.linkSvc.AcceptViaLink(context.Background(), token, "Bride", testPNG(t), "image/png"); err == nil {
		t.Error("link terpakai dua kali")
	}
}

func TestSignatureLink_Accept_MenimpaSpecimen(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	token := issueLink(t, f, o.ID)

	if err := f.linkSvc.AcceptViaLink(context.Background(), token, "Groom", testPNG(t), "image/png"); err != nil {
		t.Fatalf("AcceptViaLink: %v", err)
	}
	if f.clients.saved != 1 {
		t.Errorf("SaveSpecimen dipanggil %d kali, mau 1 — jalur magic link ikut memperbarui master (D10)", f.clients.saved)
	}
}

func TestSignatureLink_Reject_MenjadiDitolak(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	token := issueLink(t, f, o.ID)

	if err := f.linkSvc.RejectViaLink(context.Background(), token); err != nil {
		t.Fatalf("RejectViaLink: %v", err)
	}
	if stored := f.repo.rows[o.ID]; stored.Status != domain.QuotationRejected {
		t.Errorf("status = %q, mau Ditolak (T3)", stored.Status)
	}
}

func TestSignatureLink_Issue_RevisiPascaProject_Diizinkan(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationAccepted)
	f.projects.projectID = 55

	if issueLink(t, f, o.ID) == "" {
		t.Fatal("Issue link revisi pasca-project ditolak — D13 berlaku sama di jalur magic link")
	}
}

// --- D14: kompensasi link ---

func TestAcceptViaLink_SignGagal_LinkDipulihkan(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	token := issueLink(t, f, o.ID)
	f.storage.saveErr = errors.New("storage tak terjangkau")

	err := f.linkSvc.AcceptViaLink(context.Background(), token, "Bride", testPNG(t), "image/png")
	if err == nil {
		t.Fatal("AcceptViaLink berhasil padahal penyimpanan gagal")
	}
	// Link hidup lagi: klien cukup menekan Terima sekali lagi.
	var linkID int64
	for id := range f.links.rows {
		linkID = id
	}
	if !f.links.usable(linkID) {
		t.Error("link tidak dipulihkan setelah SignQuotation gagal (D14) — klien jalan buntu")
	}
	if stored := f.repo.rows[o.ID]; stored.Snapshot.Current.Signature != nil {
		t.Error("snapshot berTTD padahal penyimpanan gagal")
	}
}

func TestAcceptViaLink_SaveSpecimenGagal_TTDTetapTersimpan(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	token := issueLink(t, f, o.ID)
	f.clients.saveErr = errors.New("modul clients gangguan")

	// Specimen hanya kemudahan: kegagalannya tidak membatalkan tanda tangan
	// yang sudah sah, dan tidak memicu kompensasi (D14).
	if err := f.linkSvc.AcceptViaLink(context.Background(), token, "Bride", testPNG(t), "image/png"); err != nil {
		t.Fatalf("AcceptViaLink: %v", err)
	}
	if stored := f.repo.rows[o.ID]; stored.Snapshot.Current.Signature == nil {
		t.Error("tanda tangan hilang karena SaveSpecimen gagal")
	}
}

// --- T9: respons membawa signature ---

func TestQuotationList_MembawaSigned(t *testing.T) {
	f := newSignFixture()
	o := seedSignable(f.repo, domain.QuotationOffered)
	if _, err := f.svc.SignQuotation(context.Background(), 1, o.ID, "Bride", "Rara", testPNG(t), "image/png", SignatureChannelUpload); err != nil {
		t.Fatalf("SignQuotation: %v", err)
	}

	items, _, err := f.svc.ListPaginated(context.Background(), 1, "", "", 0, 0, 0, pagination.Params{})
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if len(items) != 1 || !items[0].Signed {
		t.Errorf("items = %+v, mau 1 baris Signed=true (T9)", items)
	}
}
