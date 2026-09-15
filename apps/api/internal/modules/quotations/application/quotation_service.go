package application

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	projectscontracts "jwswedding/internal/modules/projects/contracts"

	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/compress"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/storage"
)

type QuotationRepository interface {
	FindByID(ctx context.Context, tenantID, id int64) (*domain.Quotation, error)
	ListByTenant(ctx context.Context, tenantID int64, status string, params pagination.Params, search string, clientID int64) ([]domain.Quotation, int64, error)
	ListIDsByClient(ctx context.Context, tenantID, clientID int64, statuses ...string) ([]domain.Quotation, error)
	Create(ctx context.Context, o *domain.Quotation) error
	Update(ctx context.Context, o *domain.Quotation) error
	Delete(ctx context.Context, tenantID, id int64) error
	DeleteForClient(ctx context.Context, tenantID, clientID int64) error
	NextPOSequence(ctx context.Context, tenantID int64, period string) (int, error)
	ListBlocks(ctx context.Context, quotationID int64) ([]domain.QuotationBlock, error)
	ReplaceBlocks(ctx context.Context, quotationID int64, blocks []domain.QuotationBlock) error
	ListAdjustments(ctx context.Context, quotationID int64) ([]domain.QuotationAdjustment, error)
	AdjustmentTotals(ctx context.Context, quotationIDs []int64) (map[int64]int64, error)
	DistinctCategories(ctx context.Context, tenantID int64) ([]string, error)
	ReplaceAdjustments(ctx context.Context, quotationID int64, adjustments []domain.QuotationAdjustment) error
}

type QuotationService struct {
	repo      QuotationRepository
	templates PackageTemplateRepository
	projects  projectscontracts.Contracts
	clients   ClientDirectory
	venues    VenueResolver
	storage   ObjectStorage
}

// ObjectStorage adalah irisan sempit storage.Client yang dipakai modul ini
// (salinan milik dokumen + baca untuk PDF) — interface lokal mengikuti
// evidence_service.go:55 supaya bisa difake tanpa bucket sungguhan.
type ObjectStorage interface {
	Save(ctx context.Context, key string, data []byte, contentType string) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

func NewQuotationService(
	repo QuotationRepository,
	templates PackageTemplateRepository,
	projects projectscontracts.Contracts,
	storage ObjectStorage,
) *QuotationService {
	return &QuotationService{repo: repo, templates: templates, projects: projects, storage: storage}
}

// SetClientDirectory melengkapi two-phase wiring (lihat ClientDirectory).
func (s *QuotationService) SetClientDirectory(clients ClientDirectory) {
	s.clients = clients
}

// SetVenueResolver melengkapi two-phase wiring (lihat VenueResolver).
func (s *QuotationService) SetVenueResolver(venues VenueResolver) {
	s.venues = venues
}

// QuotationView adalah seluruh yang dibutuhkan editor penawaran dan PDF dalam
// satu baca: dokumen + komposisi + total turunan + nama client + nama venue +
// project yang lahir darinya (0 bila belum ada).
type QuotationView struct {
	Quotation   *domain.Quotation
	Blocks      []domain.QuotationBlock
	Adjustments []domain.QuotationAdjustment
	// Total = BasePrice + TotalAdjustments — nilai kontrak project setelah
	// Accept (D15). 0 bila penawaran tidak ada (tidak dipakai di list).
	Total       int64
	ClientBride string
	ClientGroom string
	ClientPhone string
	VenueName   string
	ProjectID   int64
}

func (s *QuotationService) Get(ctx context.Context, tenantID, id int64) (*QuotationView, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return s.loadView(ctx, tenantID, o)
}

// QuotationListItem adalah satu baris daftar Penawaran — ringan tanpa
// komposisi (dropdown Tambah Project memakai filter status + tenant yang
// sama). Nama pasangan diisi batch satu halaman (§11); ProjectID dipetakan
// batch lewat UNIQUE(quotation_id).
type QuotationListItem struct {
	Quotation   domain.Quotation
	ClientBride string
	ClientGroom string
	ProjectID   int64
	// Total = BasePrice + penyesuaian. Inilah nilai kontrak yang akan dibuat
	// Accept, jadi daftar dan dropdown harus menampilkan angka ini — bukan
	// BasePrice, yang berbeda begitu ada additional/takeout.
	Total int64
	// Signed = revisi yang berlaku sudah bertanda tangan — bahan badge daftar
	// (D13a) tanpa membuka snapshot mentah.
	Signed bool
}

func (s *QuotationService) ListPaginated(ctx context.Context, tenantID int64, status, search string, clientID int64, params pagination.Params) ([]QuotationListItem, int64, error) {
	list, total, err := s.repo.ListByTenant(ctx, tenantID, status, params, search, clientID)
	if err != nil {
		return nil, 0, err
	}
	clientIDs := make([]int64, 0, len(list))
	quotationIDs := make([]int64, 0, len(list))
	for _, o := range list {
		clientIDs = append(clientIDs, o.ClientID)
		quotationIDs = append(quotationIDs, o.ID)
	}
	var names map[int64][2]string
	if s.clients != nil {
		if names, err = s.clients.CoupleNamesBatch(ctx, tenantID, clientIDs); err != nil {
			return nil, 0, err
		}
	}
	projectIDs, err := s.projects.ProjectIDsForQuotations(ctx, tenantID, quotationIDs)
	if err != nil {
		return nil, 0, err
	}
	adjustmentTotals, err := s.repo.AdjustmentTotals(ctx, quotationIDs)
	if err != nil {
		return nil, 0, err
	}
	items := make([]QuotationListItem, 0, len(list))
	for _, o := range list {
		item := QuotationListItem{
			Quotation: o, ProjectID: projectIDs[o.ID],
			Total: o.BasePrice + adjustmentTotals[o.ID],
		}
		if o.Snapshot != nil && o.Snapshot.Current.Signature != nil {
			item.Signed = true
		}
		if n, ok := names[o.ClientID]; ok {
			item.ClientBride, item.ClientGroom = n[0], n[1]
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (s *QuotationService) loadView(ctx context.Context, tenantID int64, o *domain.Quotation) (*QuotationView, error) {
	view := &QuotationView{Quotation: o}
	var err error
	if view.Blocks, err = s.repo.ListBlocks(ctx, o.ID); err != nil {
		return nil, err
	}
	if view.Adjustments, err = s.repo.ListAdjustments(ctx, o.ID); err != nil {
		return nil, err
	}
	view.Total = o.BasePrice + domain.TotalAdjustments(view.Adjustments)
	if s.clients != nil {
		if bride, groom, err := s.clients.CoupleNames(ctx, tenantID, o.ClientID); err == nil {
			view.ClientBride, view.ClientGroom = bride, groom
		}
		if phone, err := s.clients.PhoneForClient(ctx, tenantID, o.ClientID); err == nil {
			view.ClientPhone = phone
		}
	}
	if o.VenueID != nil && s.venues != nil {
		if summary, err := s.venues.GetVenueSummary(ctx, tenantID, *o.VenueID); err == nil {
			view.VenueName = summary.Name
		}
	}
	if pid, err := s.projects.ProjectIDForQuotation(ctx, tenantID, o.ID); err == nil {
		view.ProjectID = pid
	}
	return view, nil
}

type CreateQuotationInput struct {
	ClientID  int64
	EventDate *time.Time
	Pax       int
	VenueID   *int64
	// TemplateID opsional: >0 berarti komposisi + harga awal disalin dari
	// template (peran "Harga Standar", D13); 0 berarti mulai kosong.
	TemplateID int64
	// PackageName boleh diketik pemanggil. Kosong + ada template berarti
	// nama template yang dipakai — auto-fill-nya diputuskan di SINI, bukan
	// cuma di UI, supaya penawaran yang dibuat lewat jalur lain (skrip,
	// import) tetap mendapat nama yang sama.
	PackageName string
}

// Create melahirkan penawaran Draft milik satu client (fase penawaran — belum
// punya project). Satu-satunya validasi lintas modul adalah keberadaan client
// (lewat direktori, bukan join).
func (s *QuotationService) Create(ctx context.Context, tenantID, actorStaffID int64, input CreateQuotationInput) (*QuotationView, error) {
	if _, _, err := s.requireClient(ctx, tenantID, input.ClientID); err != nil {
		return nil, err
	}
	o := &domain.Quotation{
		TenantID: tenantID, ClientID: input.ClientID, PackageName: strings.TrimSpace(input.PackageName),
		EventDate: input.EventDate, Pax: input.Pax, VenueID: input.VenueID,
		Status: domain.QuotationDraft, CreatedByStaffID: actorStaffID,
	}
	var tmpl *domain.PackageTemplate
	if input.TemplateID > 0 {
		var err error
		tmpl, err = s.templates.FindByID(ctx, tenantID, input.TemplateID)
		if err != nil {
			return nil, err
		}
		if tmpl == nil {
			return nil, apperror.NotFound("Template paket tidak ditemukan")
		}
		// D13: harga template hanya nilai awal — penawaran yang berwenang.
		o.BasePrice = tmpl.BasePrice
		o.TermsText = tmpl.DefaultTerms
		o.BonusNote = tmpl.DefaultBonusNote
		// DISALIN, bukan dirujuk: template bisa di-rename dan bisa dihapus
		// permanen, dan nama paket yang sudah diteken tidak boleh ikut
		// berubah karenanya (D22).
		if o.PackageName == "" {
			o.PackageName = tmpl.Name
		}
	}
	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}
	if tmpl != nil {
		blocks := make([]domain.QuotationBlock, 0, len(tmpl.Blocks))
		for _, b := range tmpl.Blocks {
			blocks = append(blocks, domain.QuotationBlock{
				QuotationID: o.ID, Category: b.Category, Body: b.Body,
				QtyText: b.QtyText, BonusNote: b.BonusNote, SortOrder: b.SortOrder,
			})
		}
		if err := s.repo.ReplaceBlocks(ctx, o.ID, blocks); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, tenantID, o.ID)
}

func (s *QuotationService) ReplaceBlocks(ctx context.Context, tenantID, id int64, blocks []domain.QuotationBlock) (*QuotationView, error) {
	o, err := s.requireEditable(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceBlocks(ctx, o.ID, blocks); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, o.ID)
}

func (s *QuotationService) ReplaceAdjustments(ctx context.Context, tenantID, id int64, adjustments []domain.QuotationAdjustment) (*QuotationView, error) {
	o, err := s.requireEditable(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	// Total divalidasi dari penyesuaian yang MASUK, sebelum satu baris pun
	// ditulis. Memvalidasi sesudah menulis menolak permintaan dengan 422
	// padahal nilainya sudah terlanjur tersimpan.
	total, err := validateTotal(o.BasePrice, adjustments)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceAdjustments(ctx, o.ID, adjustments); err != nil {
		return nil, err
	}
	// T3.3: penawaran yang sudah punya project mendorong total barunya ke
	// project (contract_value turunan, D15). Fase penawaran: ProjectID = 0 =
	// berhenti di situ.
	if err := s.pushDerived(ctx, tenantID, o.ID, total, o.PackageName); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, o.ID)
}

// SetHeaderInput adalah bahan SetHeader. Struct, bukan parameter posisional:
// begitu PackageName masuk, ada TIGA string bersebelahan (nama paket, S&K,
// bonus) yang tertukar tanpa keluhan compiler. Idiom yang sama sudah dipakai
// fetchProjectPage untuk alasan yang sama persis.
type SetHeaderInput struct {
	BasePrice   int64
	PackageName string
	TermsText   string
	BonusNote   string
	EventDate   *time.Time
	Pax         int
	VenueID     *int64
	ClientID    int64
}

// SetHeader memperbarui bagian penawaran di luar blok/penyesuaian: harga
// awal, nama paket, S&K, bonus, tanggal/pax/venue, dan client-nya.
func (s *QuotationService) SetHeader(ctx context.Context, tenantID, id int64, input SetHeaderInput) (*QuotationView, error) {
	basePrice, termsText, bonusNote := input.BasePrice, input.TermsText, input.BonusNote
	eventDate, pax, venueID, clientID := input.EventDate, input.Pax, input.VenueID, input.ClientID
	o, err := s.requireEditable(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if basePrice < 0 {
		return nil, apperror.Validation("Harga paket tidak boleh minus", map[string][]string{
			"basePrice": {"Harga paket tidak boleh minus"},
		})
	}
	projectID, err := s.projects.ProjectIDForQuotation(ctx, tenantID, o.ID)
	if err != nil {
		return nil, err
	}
	if clientID != 0 && clientID != o.ClientID {
		// Penawaran yang sudah jadi project tidak boleh berpindah client:
		// project menyimpan client_id dan nama mempelainya sendiri, dan akun
		// portal menggantung pada client itu. Membiarkannya berpindah membuat
		// project dan penawaran menunjuk dua client berbeda, diam-diam.
		if projectID != 0 {
			return nil, apperror.Validation("Client tidak bisa diganti — penawaran ini sudah menjadi project", map[string][]string{
				"clientId": {"Hapus project-nya dulu bila memang salah client"},
			})
		}
		if _, _, err := s.requireClient(ctx, tenantID, clientID); err != nil {
			return nil, err
		}
		o.ClientID = clientID
	}
	// Sama seperti ReplaceAdjustments: total dihitung dari harga BARU dan
	// divalidasi sebelum apa pun ditulis.
	adjustments, err := s.repo.ListAdjustments(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	total, err := validateTotal(basePrice, adjustments)
	if err != nil {
		return nil, err
	}
	o.BasePrice = basePrice
	o.PackageName = strings.TrimSpace(input.PackageName)
	o.TermsText = termsText
	o.BonusNote = bonusNote
	o.EventDate = eventDate
	o.Pax = pax
	o.VenueID = venueID
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	if err := s.projects.SyncFromQuotation(ctx, tenantID, projectID, total, o.PackageName); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, o.ID)
}

// validateTotal adalah bagian murni dari perhitungan: tidak menyentuh DB,
// sehingga bisa dipanggil SEBELUM menulis. Pemisahan inilah yang membuat
// penolakan 422 tidak pernah meninggalkan nilai yang sudah tersimpan.
func validateTotal(basePrice int64, adjustments []domain.QuotationAdjustment) (int64, error) {
	total := basePrice + domain.TotalAdjustments(adjustments)
	if total < 0 {
		return 0, apperror.Validation("Total pembayaran tidak boleh minus", map[string][]string{
			"adjustments": {"Pengurangan harga melebihi harga paket"},
		})
	}
	return total, nil
}

// pushDerived mendorong nilai turunan penawaran ke project bila penawaran
// sudah punya satu — satu-satunya penulis contract_value dan package_name
// dari sisi penawaran (D15, T3.3). projectID 0 (masih fase penawaran)
// berhenti di sisi projects.
//
// packageName dibaca dari penawarannya sendiri, bukan disodorkan pemanggil:
// pemanggilnya (ReplaceBlocks/ReplaceAdjustments) hanya mengubah komposisi
// dan harga, dan meminta mereka ikut membawa nama hanya akan menambah satu
// argumen yang bisa lupa diisi.
func (s *QuotationService) pushDerived(ctx context.Context, tenantID, quotationID, total int64, packageName string) error {
	projectID, err := s.projects.ProjectIDForQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return err
	}
	return s.projects.SyncFromQuotation(ctx, tenantID, projectID, total, packageName)
}

// Issue mengirim penawaran ke klien (D7): beri nomor (sekali, permanen) +
// bekukan snapshot. Draft tanpa project → Ditawarkan; Draft yang sudah punya
// project (re-issue setelah revisi) → langsung Diterima lagi.
func (s *QuotationService) Issue(ctx context.Context, tenantID, id, actorStaffID int64) (*QuotationView, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if o.Status != domain.QuotationDraft {
		return nil, apperror.Validation("Hanya penawaran Draft yang bisa dikirim", nil)
	}
	// Nama paket diwajibkan di sini, bukan saat Create: Draft adalah kertas
	// coretan (opsi "Mulai kosong" memang ada), Kirim adalah janji. Tanpa
	// syarat ini, Accept bisa melahirkan project tanpa nama paket — dan
	// itulah keadaan yang membuat Portal Klien menampilkan kolom kosong.
	if strings.TrimSpace(o.PackageName) == "" {
		return nil, apperror.Validation("Nama paket wajib diisi sebelum penawaran dikirim", map[string][]string{
			"packageName": {"Buka Ubah pada kartu Detail Kontrak, lalu isi Nama Paket (mis. \"Silver\")"},
		})
	}
	bride, groom, err := s.requireClient(ctx, tenantID, o.ClientID)
	if err != nil {
		return nil, err
	}
	blocks, err := s.repo.ListBlocks(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	adjustments, err := s.repo.ListAdjustments(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	// Tanggal acara dan total diwajibkan DI SINI, bukan saat Terima.
	//
	// Keduanya sudah lama dipaksakan oleh CreateFromQuotation, tetapi di sana
	// letaknya terlambat: begitu penawaran dikirim ia tidak bisa diedit lagi
	// (requireEditable), sehingga penawaran yang terlanjur terkirim tanpa
	// tanggal acara berakhir buntu — Terima menolaknya, dan dialog Terima
	// memang TIDAK punya kotak isian untuk itu karena tanggal acara milik
	// penawaran, bukan project (D16). Satu-satunya jalan keluar adalah Tarik
	// ke Draft, dan tidak ada yang memberi tahu.
	//
	// Memindahkan gerbangnya ke sini tidak menutup alur apa pun yang tadinya
	// berhasil: syarat yang sama sudah berlaku di ujung. Yang berubah hanya
	// KAPAN orang mengetahuinya — saat field-nya masih bisa diisi. Alasannya
	// sama dengan nama paket di atas: Draft kertas coretan, Kirim janji, dan
	// PO yang dikirim ke klien tanpa menyebut tanggal acara memang belum layak
	// kirim.
	if o.EventDate == nil {
		return nil, apperror.Validation("Tanggal acara wajib diisi sebelum penawaran dikirim", map[string][]string{
			"eventDate": {"Buka Ubah pada kartu Detail Kontrak, lalu isi Tanggal Acara"},
		})
	}
	if o.BasePrice+domain.TotalAdjustments(adjustments) <= 0 {
		return nil, apperror.Validation("Total penawaran harus lebih dari nol sebelum dikirim", map[string][]string{
			"total": {"Buka Ubah pada kartu Detail Kontrak, lalu isi Harga Paket"},
		})
	}

	if !o.IsNumbered() {
		period := time.Now().Format("200601")
		seq, err := s.repo.NextPOSequence(ctx, tenantID, period)
		if err != nil {
			return nil, err
		}
		o.PONumber = fmt.Sprintf("PO/%s/%04d", period, seq)
		o.NumberPeriod = period
		o.NumberSeq = seq
	}

	now := time.Now()
	eventDate := ""
	if o.EventDate != nil {
		eventDate = o.EventDate.Format("2006-01-02")
	}
	revision := domain.QuotationRevision{
		Revision: o.Revision, IssuedAt: now, BasePrice: o.BasePrice,
		PackageName: o.PackageName, TermsText: o.TermsText, BonusNote: o.BonusNote,
		Blocks: blocks, Adjustments: adjustments,
		Event: domain.QuotationEventSnapshot{
			ClientName: bride + " & " + groom, Phone: s.resolvePhone(ctx, tenantID, o.ClientID),
			EventDate: eventDate, Venue: s.resolveVenueName(ctx, tenantID, o.VenueID), Pax: o.Pax,
		},
	}
	if o.Snapshot == nil {
		o.Snapshot = &domain.QuotationSnapshot{}
	}
	o.Snapshot.Current = revision
	projectID, err := s.projects.ProjectIDForQuotation(ctx, tenantID, o.ID)
	if err != nil {
		return nil, err
	}
	if projectID != 0 {
		// Re-issue pasca-revisi: kembali Diterima, stempel terimanya ikut
		// diperbarui; nomor tetap (D8).
		o.Status = domain.QuotationAccepted
		o.AcceptedAt = &now
	} else {
		o.Status = domain.QuotationOffered
	}
	o.IssuedAt = &now
	_ = actorStaffID
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, o.ID)
}

// Withdraw menarik penawaran Ditawarkan kembali ke Draft untuk diedit — nomor
// tetap, revision bertambah (D8).
func (s *QuotationService) Withdraw(ctx context.Context, tenantID, id int64) (*QuotationView, error) {
	return s.reopen(ctx, tenantID, id, domain.QuotationOffered)
}

// Revise membuka penawaran Diterima untuk direvisi — dokumen yang SAMA:
// nomor tetap, revision bertambah (D8). Perubahan harga mengalir ke project
// lewat SyncContractValue di tiap edit (T3.3), bukan lewat Accept ulang.
func (s *QuotationService) Revise(ctx context.Context, tenantID, id int64) (*QuotationView, error) {
	return s.reopen(ctx, tenantID, id, domain.QuotationAccepted)
}

func (s *QuotationService) reopen(ctx context.Context, tenantID, id int64, from domain.QuotationStatus) (*QuotationView, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if o.Status != from {
		return nil, apperror.Validation("Status penawaran tidak memungkinkan aksi ini", nil)
	}
	// Arsipkan status bertandatangan ke histori sebelum dibuka lagi — klausul
	// pembatalan di S&K merujuk "kesepakatan awal", jadi revisi yang
	// digantikan harus tetap terbaca.
	if o.Snapshot != nil {
		o.Snapshot.History = append(o.Snapshot.History, o.Snapshot.Current)
	}
	o.Revision++
	o.Status = domain.QuotationDraft
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, o.ID)
}

type AcceptQuotationInput struct {
	ProjectName      string
	PrepStartDate    time.Time
	PICStaffID       int64
	Description      string
	VenueRentalPrice *int64
	VenueCharge      *int64
	ActorStaffID     int64
	ActorRole        string
}

// Accept menerima penawaran: project lahir otomatis (§7, D11).
// Idempoten — Accept yang diulang mengembalikan project yang sama (T3.2).
func (s *QuotationService) Accept(ctx context.Context, tenantID, id int64, input AcceptQuotationInput) (*QuotationView, int64, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, 0, err
	}
	if o.Status != domain.QuotationOffered {
		if o.Status == domain.QuotationAccepted {
			// Idempoten: sudah diterima — kembalikan project yang ada.
			if pid, err := s.projects.ProjectIDForQuotation(ctx, tenantID, o.ID); err == nil && pid != 0 {
				view, err := s.Get(ctx, tenantID, o.ID)
				return view, pid, err
			}
			// Diterima TANPA project (T2): penawaran yang ditandatangani
			// klien lewat magic link (D1) dan menunggu pengelola
			// melengkapi project — lanjut ke alur pembuatan di bawah.
		} else {
			return nil, 0, apperror.Validation("Hanya penawaran yang sudah dikirim (Ditawarkan) yang bisa diterima", nil)
		}
	}
	// TTD WAJIB (D9): penawaran tidak bisa menjadi project tanpa tanda tangan
	// klien. Diperiksa SESUDAH cabang idempoten di atas — menaruhnya lebih
	// dulu membuat setiap project lama mendadak tak terjangkau. Cukup periksa
	// snapshot: handler memanggil SignQuotation lebih dulu, jadi TTD dari
	// jalur mana pun sudah mendarat di sana.
	if o.Snapshot == nil || o.Snapshot.Current.Signature == nil {
		return nil, 0, apperror.Validation("Penawaran belum ditandatangani klien", map[string][]string{
			"signature": {"Unggah foto TTD klien, pakai TTD tersimpan, atau kirim link tanda tangan"},
		})
	}
	if !o.IsNumbered() {
		return nil, 0, apperror.Validation("Penawaran belum bernomor — kirim dulu ke klien sebelum diterima", nil)
	}
	bride, groom, err := s.requireClient(ctx, tenantID, o.ClientID)
	if err != nil {
		return nil, 0, err
	}
	blocks, err := s.repo.ListBlocks(ctx, o.ID)
	if err != nil {
		return nil, 0, err
	}
	adjustments, err := s.repo.ListAdjustments(ctx, o.ID)
	if err != nil {
		return nil, 0, err
	}
	total := o.BasePrice + domain.TotalAdjustments(adjustments)
	if total <= 0 {
		return nil, 0, apperror.Validation("Total penawaran harus lebih dari nol untuk menjadi project", map[string][]string{
			"total": {"Total penawaran harus lebih dari nol"},
		})
	}
	if o.EventDate == nil {
		return nil, 0, apperror.Validation("Tanggal acara wajib diisi", map[string][]string{
			"eventDate": {"Lengkapi tanggal acara di penawaran sebelum diterima"},
		})
	}
	if input.ProjectName == "" {
		input.ProjectName = bride + " & " + groom
	}
	picSales := int64(0)
	if input.ActorRole == "Sales" {
		picSales = input.ActorStaffID
	}
	projectRef, err := s.projects.CreateFromQuotation(ctx, projectscontracts.CreateFromQuotationInput{
		TenantID: tenantID, QuotationID: o.ID, ClientID: o.ClientID,
		ProjectName: input.ProjectName, BrideName: bride, GroomName: groom,
		EventDate: *o.EventDate, PrepStartDate: input.PrepStartDate, Pax: o.Pax,
		VenueID: o.VenueID, VenueRentalPrice: input.VenueRentalPrice, VenueCharge: input.VenueCharge,
		PackageName: packageNameFor(o, blocks), ContractValue: total,
		PICStaffID: input.PICStaffID, PICSalesStaffID: picSales,
		Description: input.Description, ActorStaffID: input.ActorStaffID,
	})
	if err != nil {
		return nil, 0, err
	}
	projectID := projectRef.ID

	now := time.Now()
	o.Status = domain.QuotationAccepted
	o.AcceptedAt = &now
	if err := s.repo.Update(ctx, o); err != nil {
		// §7, jalur gagal eksplisit: project SUDAH lahir (di dalam transaksi
		// CreateFromQuotation) tetapi penandaan Diterima gagal ditulis.
		// Accept diulang aman — cabang idempoten di atas menemukan project
		// lewat quotation_id UNIQUE dan mengembalikan project yang sama.
		// projectID ikut dikembalikan agar pemanggil bisa memulihkan.
		return nil, projectID, err
	}
	view, err := s.Get(ctx, tenantID, o.ID)
	return view, projectID, err
}

// Jalur tanda tangan klien (TTD Penawaran).
const (
	SignatureChannelMagicLink = "magic_link"
	SignatureChannelUpload    = "upload"
	SignatureChannelSpecimen  = "specimen"
)

// maxSignatureBytes adalah batas gambar TTD sebelum diproses (2 MB, §9).
const maxSignatureBytes = 2 * 1024 * 1024

// SignQuotation membubuhkan TTD klien pada revisi yang berlaku (D13).
//
// Gerbangnya bersandar pada TTD, bukan pada status: tolak bila revisi ini
// sudah bertanda tangan (satu revisi satu tanda tangan); izinkan status
// Ditawarkan (lalu status → Diterima + AcceptedAt, TANPA membuat project —
// D1) dan Diterima-yang-sudah-punya-project (revisi pasca-project T8 —
// HANYA snapshot yang disentuh: status, AcceptedAt, dan project tetap).
//
// Gambar disimpan sebagai SALINAN milik dokumen (D6c) — jangan pernah
// menyimpan kunci specimen ke snapshot. Dari jalur draw/upload, specimen di
// master ikut ditimpa (D10); kegagalannya hanya dicatat, tidak menggagalkan
// tanda tangan yang sudah sah (D14).
func (s *QuotationService) SignQuotation(ctx context.Context, tenantID, quotationID int64, role, signerName string, img []byte, mimeType, channel string) (*QuotationView, error) {
	if role != "Bride" && role != "Groom" && role != "Family Representative" {
		return nil, apperror.Validation("Peran penanda tangan tidak dikenal", map[string][]string{
			"role": {"Pilih Atas Nama yang tersedia"},
		})
	}
	signerName = strings.TrimSpace(signerName)
	if signerName == "" {
		return nil, apperror.Validation("Nama penanda tangan wajib diisi", map[string][]string{
			"signerName": {"Isi nama penanda tangan"},
		})
	}
	var specimenSource string
	switch channel {
	case SignatureChannelMagicLink:
		specimenSource = "draw"
	case SignatureChannelUpload:
		specimenSource = "upload"
	case SignatureChannelSpecimen:
		// Specimen tidak disentuh — isinya sudah sama dengan yang dibubuhkan.
	default:
		return nil, apperror.Validation("Jalur tanda tangan tidak dikenal", nil)
	}
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return nil, err
	}
	if o.Snapshot != nil && o.Snapshot.Current.Signature != nil {
		return nil, apperror.Validation("Revisi penawaran ini sudah ditandatangani — buat revisi baru untuk meneken ulang", map[string][]string{
			"signature": {"Revisi ini sudah memiliki tanda tangan"},
		})
	}
	projectID, err := s.projects.ProjectIDForQuotation(ctx, tenantID, o.ID)
	if err != nil {
		return nil, err
	}
	acceptIt := false
	switch {
	case o.Status == domain.QuotationOffered:
		acceptIt = true
	case o.Status == domain.QuotationAccepted && projectID != 0:
		// Revisi pasca-project (D13): hanya snapshot yang berubah.
	default:
		return nil, apperror.Validation("Penawaran ini sudah tidak bisa ditandatangani", nil)
	}
	if o.Snapshot == nil {
		return nil, apperror.Internal("Penawaran sudah dikirim tetapi tidak punya dokumen beku")
	}
	data, contentType, err := normalizeSignatureImage(img, mimeType)
	if err != nil {
		return nil, err
	}
	if s.storage == nil {
		return nil, apperror.Internal("Penyimpanan dokumen belum dikonfigurasi")
	}
	key := storage.BuildQuotationSignatureKey(
		strconv.FormatInt(tenantID, 10), strconv.FormatInt(o.ID, 10), o.Revision)
	if _, err := s.storage.Save(ctx, key, data, contentType); err != nil {
		return nil, err
	}
	now := time.Now()
	o.Snapshot.Current.Signature = &domain.QuotationClientSignature{
		SignerName: signerName, SignerRole: role,
		StorageKey: key, SignedAt: now, Channel: channel,
	}
	if acceptIt {
		o.Status = domain.QuotationAccepted
		o.AcceptedAt = &now
	}
	// Objek yatim bila Update gagal setelah Save berhasil — dibiarkan, sama
	// dengan urutan yang dipakai EvidenceService (§14).
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	if specimenSource != "" && s.clients != nil {
		if err := s.clients.SaveSpecimen(ctx, tenantID, o.ClientID, role, signerName, img, mimeType, specimenSource); err != nil {
			logger.Error("gagal memperbarui specimen client %d setelah penandatanganan quotation %d: %v", o.ClientID, o.ID, err)
		}
	}
	return s.Get(ctx, tenantID, o.ID)
}

// SignFromSpecimen membubuhkan specimen tersimpan ke revisi yang berlaku.
// Gerbang D12a: pemilik specimen HARUS sama dengan Atas Nama yang dipilih —
// memakai TTD orang lain di bawah nama orang ini adalah pemalsuan yang
// dilakukan sistem sendiri. Layar menyembunyikan tombolnya, tetapi gerbang
// yang hanya ada di layar bukan gerbang: API menolak dengan 422 juga.
func (s *QuotationService) SignFromSpecimen(ctx context.Context, tenantID, quotationID int64, role, signerName string) (*QuotationView, error) {
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return nil, err
	}
	if s.clients == nil {
		return nil, apperror.Internal("Direktori client belum dikonfigurasi")
	}
	meta, found, err := s.clients.SpecimenMeta(ctx, tenantID, o.ClientID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperror.NotFound("TTD tersimpan tidak ditemukan — unggah foto TTD sebagai gantinya")
	}
	if meta[0] != role {
		return nil, apperror.Validation("TTD tersimpan milik "+specimenOwnerLabel(meta)+" — unggah TTD baru atas nama yang dipilih", map[string][]string{
			"role": {"TTD tersimpan milik " + specimenOwnerLabel(meta)},
		})
	}
	img, err := s.clients.SpecimenImage(ctx, tenantID, o.ClientID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(signerName) == "" {
		signerName = meta[1]
	}
	return s.SignQuotation(ctx, tenantID, quotationID, role, signerName, img, sniffSignatureMIME(img), SignatureChannelSpecimen)
}

func specimenOwnerLabel(meta [4]string) string {
	if meta[1] != "" {
		return meta[1] + " (" + meta[0] + ")"
	}
	return meta[0]
}

// QuotationSignatureOptions adalah bahan layar penandatanganan: pilihan Atas
// Nama + keterangan specimen (bila ada).
type QuotationSignatureOptions struct {
	// Options adalah pasangan {role, name} dari master clients (T1).
	Options [][2]string
	// Specimen adalah {role, signerName, source, updatedAt}, nil bila client
	// belum punya specimen tersimpan.
	Specimen *[4]string
}

func (s *QuotationService) SignatureOptions(ctx context.Context, tenantID, quotationID int64) (*QuotationSignatureOptions, error) {
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return nil, err
	}
	if s.clients == nil {
		return nil, apperror.Internal("Direktori client belum dikonfigurasi")
	}
	options, err := s.clients.SignerOptions(ctx, tenantID, o.ClientID)
	if err != nil {
		return nil, err
	}
	out := &QuotationSignatureOptions{Options: options}
	if meta, found, err := s.clients.SpecimenMeta(ctx, tenantID, o.ClientID); err != nil {
		return nil, err
	} else if found {
		out.Specimen = &meta
	}
	return out, nil
}

// DocumentSignatureImage mengembalikan bytes salinan TTD milik dokumen untuk
// PDF — nil bila revisi yang berlaku belum diteken ATAU objeknya tidak bisa
// dibaca. Kegagalan membaca gambar TIDAK menggagalkan pencetakan: PDF tetap
// tercetak dengan kotak klien kosong (idiom ResolvePICName).
func (s *QuotationService) DocumentSignatureImage(ctx context.Context, tenantID, quotationID int64) []byte {
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil || o.Snapshot == nil || o.Snapshot.Current.Signature == nil || s.storage == nil {
		return nil
	}
	rc, err := s.storage.Open(ctx, o.Snapshot.Current.Signature.StorageKey)
	if err != nil {
		return nil
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}
	return data
}

// normalizeSignatureImage menolak selain PNG/JPEG dan yang lebih dari 2 MB,
// lalu me-re-encode lewat compress.Image — apa pun yang menumpang di luar
// piksel ikut hilang (§9). Helper milik modul ini (tidak memakai milik
// clients): presentation/application helper diduplikasi per modul mengikuti
// idiom codebase ini.
func normalizeSignatureImage(img []byte, mimeType string) (data []byte, contentType string, err error) {
	switch mimeType {
	case "image/png", "image/jpeg", "image/jpg":
	default:
		return nil, "", apperror.Validation("Jenis berkas harus gambar (PNG atau JPEG)", map[string][]string{
			"mimeType": {"Unggah foto TTD berekstensi PNG atau JPG"},
		})
	}
	if len(img) == 0 {
		return nil, "", apperror.Validation("Gambar tanda tangan kosong", map[string][]string{
			"signature": {"Bubuhkan tanda tangan dahulu"},
		})
	}
	if len(img) > maxSignatureBytes {
		return nil, "", apperror.Validation("Gambar tanda tangan terlalu besar (maksimal 2 MB)", map[string][]string{
			"signature": {"Perkecil gambarnya lalu coba lagi"},
		})
	}
	out, err := compress.Image(img, mimeType)
	if err != nil {
		return nil, "", err
	}
	if mimeType == "image/jpg" {
		mimeType = "image/jpeg"
	}
	return out, mimeType, nil
}

// sniffSignatureMIME menebak tipe bytes specimen yang dibaca dari storage —
// specimen selalu PNG/JPEG karena normalizeSignatureImage menjaganya saat
// masuk. Jatuh ke PNG bila tak dikenali (fpdfImageType yang memutuskan
// nasib akhirnya di PDF).
func sniffSignatureMIME(img []byte) string {
	switch http.DetectContentType(img) {
	case "image/jpeg":
		return "image/jpeg"
	default:
		return "image/png"
	}
}

// packageNameFor memilih nama paket yang dibawa ke project saat Accept.
//
// Penawaran yang dibuat sejak migrasi 000065 SELALU punya nama — Issue
// mewajibkannya — jadi cabang pertama inilah yang praktis selalu terpakai.
//
// Cabang kedua melayani penawaran lama saja: sebelum kolomnya ada, nama
// diturunkan dari KATEGORI BLOK PERTAMA. Perilakunya dipertahankan apa adanya
// supaya penawaran lama yang baru diterima hari ini tidak mendadak lahir
// tanpa nama. Cabang ini boleh dihapus begitu tidak ada lagi penawaran
// pra-000065 yang belum diterima.
func packageNameFor(o *domain.Quotation, blocks []domain.QuotationBlock) string {
	if name := strings.TrimSpace(o.PackageName); name != "" {
		return name
	}
	if len(blocks) == 0 {
		return ""
	}
	return blocks[0].Category
}

// Reject menandai penawaran kalah (Ditolak), Expire yang kedaluwarsa
// (Kedaluwarsa), Cancel yang dibatalkan (Dibatalkan) — ketiganya terminal dan
// tidak menyentuh project apa pun (D18: tanpa ini penawaran kalah menggantung
// selamanya dan mengotori dropdown).
func (s *QuotationService) Reject(ctx context.Context, tenantID, id int64) (*QuotationView, error) {
	return s.terminal(ctx, tenantID, id, domain.QuotationRejected)
}

func (s *QuotationService) Expire(ctx context.Context, tenantID, id int64) (*QuotationView, error) {
	return s.terminal(ctx, tenantID, id, domain.QuotationExpired)
}

func (s *QuotationService) Cancel(ctx context.Context, tenantID, id int64) (*QuotationView, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if o.Status != domain.QuotationDraft && o.Status != domain.QuotationOffered {
		return nil, apperror.Validation("Status penawaran tidak memungkinkan aksi ini", nil)
	}
	o.Status = domain.QuotationCancelled
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, o.ID)
}

func (s *QuotationService) terminal(ctx context.Context, tenantID, id int64, to domain.QuotationStatus) (*QuotationView, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if o.Status != domain.QuotationOffered {
		return nil, apperror.Validation("Hanya penawaran Ditawarkan yang bisa ditutup dengan status ini", nil)
	}
	o.Status = to
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, o.ID)
}

// Duplicate menyalin penawaran menjadi Draft baru (T3.4, pengganti duplikat
// project D12): blok, penyesuaian, S&K, rencana termin, dan harga tersalin;
// status Draft, nomor kosong, belum terhubung project.
func (s *QuotationService) Duplicate(ctx context.Context, tenantID, id, actorStaffID int64) (*QuotationView, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	clone := &domain.Quotation{
		TenantID: o.TenantID, ClientID: o.ClientID, BasePrice: o.BasePrice,
		PackageName: o.PackageName, TermsText: o.TermsText, BonusNote: o.BonusNote,
		EventDate: o.EventDate, Pax: o.Pax, VenueID: o.VenueID,
		Status: domain.QuotationDraft, CreatedByStaffID: actorStaffID,
	}
	if err := s.repo.Create(ctx, clone); err != nil {
		return nil, err
	}
	blocks, err := s.repo.ListBlocks(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	for i := range blocks {
		blocks[i].QuotationID = clone.ID
	}
	if err := s.repo.ReplaceBlocks(ctx, clone.ID, blocks); err != nil {
		return nil, err
	}
	adjustments, err := s.repo.ListAdjustments(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	for i := range adjustments {
		adjustments[i].QuotationID = clone.ID
	}
	if err := s.repo.ReplaceAdjustments(ctx, clone.ID, adjustments); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, clone.ID)
}

// DeleteImpact adalah bahan dialog konfirmasi (D14, T3.5) — dialog menolak
// tampil kalau ini gagal.
type QuotationDeleteImpact struct {
	Quotation        *domain.Quotation
	ProjectID        int64
	ProjectName      string
	PaidInvoiceCount int
	PaidInvoiceTotal int64
}

func (s *QuotationService) DeleteImpact(ctx context.Context, tenantID, id int64) (QuotationDeleteImpact, error) {
	var out QuotationDeleteImpact
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return out, err
	}
	out.Quotation = o
	// Project diselesaikan tanpa pandang status: penawaran Diterima yang
	// sempat di-Revisi kembali berstatus Draft TAPI project-nya tetap ada —
	// dialog hapus harus menyebut project itu (untuk memblokir, lihat
	// Delete), bukan diam seolah tidak ada apa-apa.
	projectID, err := s.projects.ProjectIDForQuotation(ctx, tenantID, o.ID)
	if err != nil {
		return out, err
	}
	out.ProjectID = projectID
	if projectID == 0 {
		return out, nil
	}
	impact, err := s.projects.ProjectDeleteImpact(ctx, tenantID, projectID)
	if err != nil {
		return out, err
	}
	out.ProjectName = impact.ProjectName
	out.PaidInvoiceCount = impact.PaidInvoiceCount
	out.PaidInvoiceTotal = impact.PaidInvoiceTotal
	return out, nil
}

// Delete menghapus penawaran yang BELUM menjadi project (D14 — selalu
// tersedia, tanpa pengecualian pemblokir). Penawaran yang sudah memiliki
// project — status apa pun, termasuk Draft hasil Revisi — DITOLAK dengan
// pesan hapus-project-dulu; satu-satunya jalan menghapusnya adalah lewat
// penghapusan project-nya (yang ikut menyapu penawaran via DeleteQuotation
// di bawah). Mengandalkan label status saja pernah meloloskan project yatim
// lewat Revisi→Draft→Hapus, karena status saat dihapus sudah bukan Diterima.
func (s *QuotationService) Delete(ctx context.Context, tenantID, id int64) error {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return err
	}
	projectID, err := s.projects.ProjectIDForQuotation(ctx, tenantID, o.ID)
	if err != nil {
		return err
	}
	if projectID != 0 {
		return apperror.Validation("Penawaran ini sudah memiliki project — hapus project-nya terlebih dahulu", map[string][]string{
			"projectId": {"Penawaran ini sudah memiliki project — hapus project-nya terlebih dahulu"},
		})
	}
	return s.repo.Delete(ctx, tenantID, o.ID)
}

// DeleteQuotation adalah hapus-baris repo murni — port yang dipakai
// DeleteProjectCascade milik projects (T2.8). BUKAN Delete di atas (itu akan
// loop: Delete → DeleteProjectCascade → DeleteQuotation → …).
func (s *QuotationService) DeleteQuotation(ctx context.Context, tenantID, quotationID int64) error {
	if _, err := s.requireQuotation(ctx, tenantID, quotationID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, tenantID, quotationID)
}

// CompositionForQuotation mengembalikan komposisi paket sebuah penawaran —
// bahan tabel KATEGORI/PRODUK/QTY/BONUS yang kini ikut tercetak di PDF
// Tagihan, supaya pembaca tagihan tahu paket apa yang sedang ditagihkan
// tanpa harus membuka dokumen PO-nya.
//
// Bentuk kembaliannya PRIMITIF ([][4]string: {kategori, produk, qty, bonus}),
// bukan []domain.QuotationBlock, dan itu disengaja. `projects` tidak boleh
// mengimpor modul ini; sebaliknya `quotations` sudah mengimpor
// projects/contracts (Accept). Tipe bersama apa pun di antara keduanya
// menutup lingkaran impor projects/application → quotations/contracts →
// quotations/application → projects/contracts → projects/application. Port
// QuotationResolver yang sudah ada memakai alasan yang sama — karena itu ia
// pun hanya berisi int64 dan string. Pemanggil memetakannya ke tipe bernama
// miliknya sendiri begitu diterima (ProjectService.QuotationComposition).
//
// Sumber datanya mengikuti aturan yang sama persis dengan PDF PO: penawaran
// selain Draft dibaca dari snapshot bekunya, supaya tagihan menampilkan
// paket seperti saat dokumen itu DITEKEN, bukan seperti setelah master
// datanya bergerak. Project hanya lahir dari penawaran Diterima, jadi jalur
// snapshot inilah yang praktis selalu terpakai; cabang live tetap ada supaya
// penawaran yang entah bagaimana masih Draft tidak mengembalikan kosong.
func (s *QuotationService) CompositionForQuotation(ctx context.Context, tenantID, quotationID int64) ([][4]string, error) {
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return nil, err
	}
	var blocks []domain.QuotationBlock
	if o.Status != domain.QuotationDraft && o.Snapshot != nil {
		blocks = o.Snapshot.Current.Blocks
	} else if blocks, err = s.repo.ListBlocks(ctx, o.ID); err != nil {
		return nil, err
	}
	rows := make([][4]string, 0, len(blocks))
	for _, b := range blocks {
		rows = append(rows, [4]string{b.Category, b.Body, b.QtyText, b.BonusNote})
	}
	return rows, nil
}

// StatusForQuotation memberi tahu `projects` fase penawarannya. Yang dicari
// `projects` cuma satu keadaan: penawaran milik project yang berstatus Draft
// = sedang DIREVISI dan belum dikirim ulang, sehingga nilai kontraknya belum
// disepakati ulang oleh klien.
func (s *QuotationService) StatusForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return "", err
	}
	return string(o.Status), nil
}

// PackageNameForQuotation memberi tahu `projects` apakah penawaran sebuah
// project sudah memiliki nama paket sendiri. "" berarti penawaran pra-000065
// — di situlah, dan HANYA di situ, field "Paket / Layanan" di Ubah Project
// boleh dibuka kuncinya, karena tidak ada yang mendorongnya dari sini.
func (s *QuotationService) PackageNameForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return "", err
	}
	return o.PackageName, nil
}

// PONumberForQuotation membaca nomor PO untuk dialog delete-impact project
// (T3.5) — "" bila belum bernomor.
func (s *QuotationService) PONumberForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	o, err := s.requireQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return "", err
	}
	return o.PONumber, nil
}

// ImpactForClient / DeleteForClient adalah sisi-penawaran dari hapus Client
// berjenjang (T3.5). Keduanya mencakup SELURUH penawaran milik client,
// termasuk yang sudah Diterima: penawaran Diterima yang project-nya lebih dulu
// hilang tidak muncul di sisi project mana pun, sehingga kalau dilewati di
// sini ia lolos dari dialog maupun dari penghapusan — dua-duanya dilarang D14.
// Urutan cascade (project lebih dulu) membuat yang masih punya project sudah
// terhapus sebelum sapuan ini berjalan, jadi tidak ada yang terhitung dua kali.
func (s *QuotationService) ImpactForClient(ctx context.Context, tenantID, clientID int64) (int, []string, error) {
	list, err := s.repo.ListIDsByClient(ctx, tenantID, clientID)
	if err != nil {
		return 0, nil, err
	}
	var numbers []string
	for _, o := range list {
		if o.PONumber != "" {
			numbers = append(numbers, o.PONumber)
		} else {
			numbers = append(numbers, fmt.Sprintf("Draft #%d", o.ID))
		}
	}
	return len(numbers), numbers, nil
}

func (s *QuotationService) DeleteForClient(ctx context.Context, tenantID, clientID int64) error {
	return s.repo.DeleteForClient(ctx, tenantID, clientID)
}

// Categories menopang datalist kategori pada editor penawaran: riwayat
// tenant, bukan hanya blok dokumen yang sedang dibuka — kalau dari dokumen
// sendiri, daftarnya justru kosong tepat saat penawaran baru disusun.
func (s *QuotationService) Categories(ctx context.Context, tenantID int64) ([]string, error) {
	return s.repo.DistinctCategories(ctx, tenantID)
}

func (s *QuotationService) requireQuotation(ctx context.Context, tenantID, id int64) (*domain.Quotation, error) {
	o, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, apperror.NotFound("Penawaran tidak ditemukan")
	}
	return o, nil
}

// requireEditable menolak tulis ke dokumen yang sudah dikirim — membukanya
// kembali adalah tugas Withdraw/Revise, bukan efek samping edit blok.
func (s *QuotationService) requireEditable(ctx context.Context, tenantID, id int64) (*domain.Quotation, error) {
	o, err := s.requireQuotation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if o.Status != domain.QuotationDraft {
		return nil, apperror.Validation("Penawaran sudah dikirim. Tarik kembali ke Draft dulu untuk mengubah isinya.", nil)
	}
	return o, nil
}

// requireClient memastikan client_id milik tenant ini — satu-satunya
// validasi lintas modul di jalur tulis, lewat direktori (bukan join).
func (s *QuotationService) requireClient(ctx context.Context, tenantID, clientID int64) (bride, groom string, err error) {
	if s.clients == nil {
		return "", "", apperror.Validation("Client tidak valid", map[string][]string{"clientId": {"Client tidak ditemukan"}})
	}
	bride, groom, err = s.clients.CoupleNames(ctx, tenantID, clientID)
	if err != nil {
		return "", "", err
	}
	return bride, groom, nil
}

func (s *QuotationService) resolvePhone(ctx context.Context, tenantID, clientID int64) string {
	if s.clients == nil {
		return ""
	}
	phone, err := s.clients.PhoneForClient(ctx, tenantID, clientID)
	if err != nil {
		return ""
	}
	return phone
}

func (s *QuotationService) resolveVenueName(ctx context.Context, tenantID int64, venueID *int64) string {
	if venueID == nil || s.venues == nil {
		return ""
	}
	summary, err := s.venues.GetVenueSummary(ctx, tenantID, *venueID)
	if err != nil {
		return ""
	}
	return summary.Name
}
