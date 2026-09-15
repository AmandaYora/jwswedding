package application

import (
	"context"
	"errors"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/logger"
)

// ClientActivator adalah bentuk sempit yang dibutuhkan projects dari `clients`
// (D4): mengaktifkan/menonaktifkan akun portal mengikuti "niat admin DAN
// punya project". Interface lokal (bukan impor clients/contracts) supaya
// projects tidak pernah mengimpor clients — dipasang dari main.go lewat
// Module.SetClientActivator (two-phase).
type ClientActivator interface {
	SyncCredentialActive(ctx context.Context, tenantID, clientID int64) error
}

// QuotationResolver adalah bentuk sempit yang dibutuhkan projects dari
// `quotations`: menghapus penawaran milik project yang dihapus dan membaca
// nomor PO-nya untuk dialog delete-impact (T2.8, T3.5). Interface lokal,
// dipasang dari main.go lewat Module.SetQuotationResolver.
type QuotationResolver interface {
	DeleteQuotation(ctx context.Context, tenantID, quotationID int64) error
	PONumberForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error)
	// CompositionForQuotation memberi bahan tabel KATEGORI/PRODUK/QTY/BONUS
	// untuk PDF Tagihan. Primitif ([][4]string) karena tipe bersama apa pun
	// akan menutup lingkaran impor antara kedua modul — lihat doc comment
	// method ini di quotations/application untuk uraiannya.
	CompositionForQuotation(ctx context.Context, tenantID, quotationID int64) ([][4]string, error)
	PackageNameForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error)
	StatusForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error)
}

// QuotationUnderRevision menjawab: apakah penawaran project ini sedang
// direvisi dan BELUM dikirim ulang?
//
// Keadaan ini penting karena mengubah harga pada revisi langsung mendorong
// nilai kontrak baru ke project — sebelum klien menyetujui apa pun. Selama
// itu, Sisa Tagihan dan PDF Invoice memakai angka yang belum disepakati,
// jadi penerbitan Tagihan baru ditahan dan project-nya diberi tanda.
//
// Menelan error seperti resolver display-only lainnya: kalau tidak terjawab,
// jawab false — menahan penerbitan Tagihan karena kegagalan pembacaan jauh
// lebih mengganggu daripada satu tanda yang tidak muncul.
func (s *ProjectService) QuotationUnderRevision(ctx context.Context, tenantID, quotationID int64) bool {
	if quotationID == 0 || s.quotations == nil {
		return false
	}
	status, err := s.quotations.StatusForQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return false
	}
	return status == "Draft"
}

// PackageNameFromQuotation menjawab: apakah "Paket / Layanan" project ini
// diatur oleh penawarannya?
//
// true untuk penawaran yang punya nama sendiri — nilainya didorong tiap kali
// penawaran diedit, jadi mengetiknya manual di project hanya akan tertimpa.
// false untuk project pra-penawaran maupun penawaran pra-000065: tidak ada
// yang mendorongnya, jadi satu-satunya cara merapikannya adalah mengetiknya
// di sana.
//
// Menelan error seperti ResolvePICName: kalau tidak terjawab, jawab false —
// membuka kunci sebuah field jauh lebih ringan akibatnya daripada menggagalkan
// pembacaan detail project.
func (s *ProjectService) PackageNameFromQuotation(ctx context.Context, tenantID, quotationID int64) bool {
	if quotationID == 0 || s.quotations == nil {
		return false
	}
	name, err := s.quotations.PackageNameForQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return false
	}
	return name != ""
}

// QuotationCompositionRow adalah satu baris tabel komposisi paket milik
// penawaran, dalam bentuk bernama yang dimiliki modul ini — hasil pemetaan
// dari [4]string primitif yang menyeberangi batas modul, dilakukan sekali di
// QuotationComposition supaya indeks telanjang tidak pernah bocor ke
// pemanggil.
type QuotationCompositionRow struct {
	Category string
	Product  string
	Qty      string
	Bonus    string
}

// QuotationComposition membaca komposisi paket penawaran milik sebuah project
// untuk dicetak di PDF Tagihan.
//
// Display-only, jadi menelan error dan mengembalikan nil — sama seperti
// ResolvePONumber dan ResolvePICName. Tagihan tetap sah tanpa tabel
// komposisi; menggagalkan pencetakan tagihan gara-gara satu tabel konteks
// tidak sebanding, apalagi untuk project pra-penawaran yang memang tidak
// punya penawaran sama sekali.
func (s *ProjectService) QuotationComposition(ctx context.Context, tenantID, quotationID int64) []QuotationCompositionRow {
	if quotationID == 0 || s.quotations == nil {
		return nil
	}
	raw, err := s.quotations.CompositionForQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return nil
	}
	rows := make([]QuotationCompositionRow, 0, len(raw))
	for _, r := range raw {
		rows = append(rows, QuotationCompositionRow{Category: r[0], Product: r[1], Qty: r[2], Bonus: r[3]})
	}
	return rows
}

// SetClientActivator melengkapi two-phase wiring (lihat ClientActivator).
func (s *ProjectService) SetClientActivator(activator ClientActivator) {
	s.activator = activator
}

// SetQuotationResolver melengkapi two-phase wiring (lihat QuotationResolver).
func (s *ProjectService) SetQuotationResolver(resolver QuotationResolver) {
	s.quotations = resolver
}

// SetClientInvoiceService memasok ledger tagihan yang dibutuhkan
// SyncContractValue dan ImpactForClient. Setter (bukan konstruktor) supaya
// signature NewProjectService dan semua pemanggilnya tidak berubah.
func (s *ProjectService) SetClientInvoiceService(invoices *ClientInvoiceService) {
	s.invoices = invoices
}

// ClientHasProject menjawab dari projects.client_id — satu-satunya tempat
// relasi itu hidup (T1.7). Satu query atas SATU client (orde satuan baris).
func (s *ProjectService) ClientHasProject(ctx context.Context, tenantID, clientID int64) (bool, error) {
	list, err := s.repo.ListByClient(ctx, tenantID, clientID)
	if err != nil {
		return false, err
	}
	return len(list) > 0, nil
}

// ProjectIDsForClient mengembalikan project milik satu client — dipakai
// resolver akses portal (T1.10) dan SyncCredentialActive (D4). Butuh indeks
// projects.client_id (T1.2); tanpanya tiap request portal jadi full scan.
func (s *ProjectService) ProjectIDsForClient(ctx context.Context, tenantID, clientID int64) ([]int64, error) {
	list, err := s.repo.ListByClient(ctx, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	return ids, nil
}

// ProjectCountsForClients menjawab hitungan project per client untuk satu
// halaman daftar — satu query agregat (lihat repo CountByClients).
func (s *ProjectService) ProjectCountsForClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]int, error) {
	return s.repo.CountByClients(ctx, tenantID, clientIDs)
}

// ClientImpact adalah bentuk application-level dari contracts.ProjectImpact —
// contracts merakit DTO publiknya dari sini (preseden: CreateOwner).
type ClientImpact struct {
	ProjectCount     int
	ProjectNames     []string
	PaidInvoiceCount int
	PaidInvoiceTotal int64
}

// ImpactForClient menggabungkan hitungan project milik satu client dengan
// agregat tagihan Lunas-nya — bahan dialog konfirmasi hapus Client (D14,
// T3.5). Menyebut angka yang paling mahal kalau salah hapus: berapa tagihan
// terbayar dan totalnya.
func (s *ProjectService) ImpactForClient(ctx context.Context, tenantID, clientID int64) (ClientImpact, error) {
	var out ClientImpact
	list, err := s.repo.ListByClient(ctx, tenantID, clientID)
	if err != nil {
		return out, err
	}
	out.ProjectCount = len(list)
	for _, p := range list {
		out.ProjectNames = append(out.ProjectNames, p.Name)
		if s.invoices == nil {
			continue
		}
		invoices, err := s.invoices.List(ctx, p.ID)
		if err != nil {
			return out, err
		}
		for _, inv := range invoices {
			if inv.Status == domain.InvoicePaid {
				out.PaidInvoiceCount++
				out.PaidInvoiceTotal += inv.Amount
			}
		}
	}
	return out, nil
}

// DeleteProjectsForClient menghapus seluruh project milik satu client, satu
// per satu lewat jalur force (D14 — tanpa blokir). Dipakai penghapusan Client
// berjenjang (T3.5), BUKAN penghapusan project manual. Lintas project
// dijalankan sekuensial (bukan satu transaksi): kegagalan di tengah dicatat
// dan dilaporkan, tidak menggagalkan yang sudah terjadi.
func (s *ProjectService) DeleteProjectsForClient(ctx context.Context, tenantID, clientID int64) error {
	list, err := s.repo.ListByClient(ctx, tenantID, clientID)
	if err != nil {
		return err
	}
	var firstErr error
	for _, p := range list {
		if err := s.DeleteProjectCascade(ctx, tenantID, p.ID); err != nil {
			logger.Error("gagal menghapus project %d saat bersih-bersih client %d: %v", p.ID, clientID, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// CreateFromQuotationInput adalah seluruh bahan kelahiran project dari
// penawaran Diterima (§7). Tidak lagi membawa rencana termin: fitur itu
// dihapus (migrasi 000064), jadi project lahir tanpa tagihan dan staff
// menerbitkannya sendiri sesuai cicilan yang benar-benar disepakati.
type CreateFromQuotationInput struct {
	TenantID         int64
	QuotationID      int64
	ClientID         int64
	ProjectName      string
	BrideName        string
	GroomName        string
	EventDate        time.Time
	PrepStartDate    time.Time
	Pax              int
	VenueID          *int64
	VenueRentalPrice *int64
	VenueCharge      *int64
	PackageName      string
	ContractValue    int64
	PICStaffID       int64
	PICSalesStaffID  int64
	Description      string
	ActorStaffID     int64
}

// CreateFromQuotation adalah SATU-SATUNYA pintu kelahiran project baru (D11):
// satu transaksi berisi project + Timeline Default + tagihan (seedSchedule -
// inlined), idempoten lewat projects.quotation_id UNIQUE (T3.2) — Accept yang
// diulang setelah gagal di tengah mengembalikan project yang sudah ada,
// bukan membuat yang kedua.
//
// Panggilan balik ke modul lain (SyncCredentialActive) terjadi SETELAH commit,
// jadi tidak ada transaksi yang menganggur menunggu modul lain (§7).
func (s *ProjectService) CreateFromQuotation(ctx context.Context, input CreateFromQuotationInput) (int64, error) {
	if input.ContractValue <= 0 {
		return 0, apperror.Validation("Nilai kontrak tidak valid", map[string][]string{
			"total": {"Total penawaran harus lebih dari nol untuk menjadi project"},
		})
	}
	if input.EventDate.IsZero() {
		return 0, apperror.Validation("Tanggal acara wajib diisi", map[string][]string{
			"eventDate": {"Lengkapi tanggal acara di penawaran sebelum diterima"},
		})
	}
	if input.PrepStartDate.IsZero() {
		return 0, apperror.Validation("Tanggal Booking wajib diisi", map[string][]string{
			"prepStartDate": {"Isi Tanggal Booking dulu"},
		})
	}
	if s.invoices == nil {
		return 0, apperror.Validation("Layanan tagihan belum terpasang", nil)
	}
	if existing, err := s.repo.FindByQuotationID(ctx, input.TenantID, input.QuotationID); err != nil {
		return 0, err
	} else if existing != nil {
		return existing.ID, nil
	}

	p := &domain.Project{
		TenantID: input.TenantID, ClientID: input.ClientID, QuotationID: input.QuotationID,
		Name: input.ProjectName, BrideName: input.BrideName, GroomName: input.GroomName,
		EventDate: input.EventDate, Pax: input.Pax, PrepStartDate: input.PrepStartDate,
		PackageName: input.PackageName, ContractValue: input.ContractValue,
		Status:     domain.StatusPreparation,
		PICStaffID: input.PICStaffID, PICSalesStaffID: input.PICSalesStaffID,
		Description: input.Description,
	}
	if input.VenueID != nil && *input.VenueID > 0 {
		p.VenueID = input.VenueID
		p.VenueRentalPrice = input.VenueRentalPrice
		p.VenueCharge = input.VenueCharge
		// Nama venue untuk kolom teks dibaca lewat kontrak VenueResolver yang
		// sudah ada (ADR-0016). Venue yang sudah dihapus sejak penawaran
		// dibuat (stale id) cukup membuat kolom teks kosong — tidak
		// menggagalkan kelahiran project.
		p.Venue = s.resolveVenueName(ctx, input.TenantID, *input.VenueID)
	}

	milestones, err := s.buildSeedMilestones(ctx, input.TenantID, input.EventDate, input.PrepStartDate)
	if err != nil {
		return 0, err
	}
	if err := s.repo.CreateSeeded(ctx, p, milestones); err != nil {
		// Balapan Accept ganda: yang kalah menabrak UNIQUE(quotation_id) —
		// baca ulang; bila project-nya sudah ada, kembalikan ia (T3.2),
		// bukan error. Hanya jalur gagal yang membayar satu query ekstra.
		if existing, checkErr := s.repo.FindByQuotationID(ctx, input.TenantID, input.QuotationID); checkErr == nil && existing != nil {
			return existing.ID, nil
		}
		return 0, err
	}

	s.activity.Record(ctx, &p.ID, domain.ActivityProjectCreated, input.ActorStaffID, "project", formatID(p.ID), p.Name,
		"Project lahir dari penawaran yang diterima: "+p.Name)

	// Best-effort (D4, §7): akun portal aktif mengikuti "niat admin DAN punya
	// project" — client ini BARU SAJA punya project. Kegagalan dicatat, tidak
	// membatalkan project maupun tagihannya; akun bisa diaktifkan ulang dari
	// menu Client.
	if s.activator != nil && input.ClientID != 0 {
		if err := s.activator.SyncCredentialActive(ctx, input.TenantID, input.ClientID); err != nil {
			logger.Error("gagal mengaktifkan akun portal client %d setelah project %d lahir: %v", input.ClientID, p.ID, err)
		}
	}
	return p.ID, nil
}

// resolveVenueName membaca nama venue lewat kontrak yang sudah ada — "" bila
// venue-nya sudah dihapus (stale id) atau resolver belum dipasang.
func (s *ProjectService) resolveVenueName(ctx context.Context, tenantID, venueID int64) string {
	if s.venues == nil {
		return ""
	}
	summary, err := s.venues.GetVenueSummary(ctx, tenantID, venueID)
	if err != nil {
		return ""
	}
	return summary.Name
}

// buildSeedMilestones menyiapkan baris Timeline Default (disalin dari template
// tenant, bukan daftar hardcoded) untuk disisipkan dalam transaksi Accept.
func (s *ProjectService) buildSeedMilestones(ctx context.Context, tenantID int64, eventDate, prepStartDate time.Time) ([]domain.ProjectMilestone, error) {
	templates, err := s.milestoneTemplates.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	milestones := make([]domain.ProjectMilestone, 0, len(templates))
	for i, tmpl := range templates {
		target := eventDate.AddDate(0, 0, -tmpl.DaysBeforeEvent)
		if target.Before(prepStartDate) {
			target = prepStartDate
		}
		milestones = append(milestones, domain.ProjectMilestone{
			SortOrder: i + 1, Name: tmpl.Name, Category: tmpl.Category,
			Status: domain.MilestoneNotStarted, TargetDate: target,
		})
	}
	return milestones, nil
}

// SyncFromQuotation mendorong nilai turunan penawaran ke dalam project-nya:
// total (D15) dan nama paket. Dilewati selama penawaran belum punya project:
// ProjectIDForQuotation yang kosong berarti berhenti di situ — di fase
// penawaran belum ada nilai kontrak untuk disesuaikan.
//
// Dulu fungsi ini juga menyeimbangkan tagihan Draft (rebalanceDraftInvoices,
// T3.3): selisih harga mendarat di tagihan Draft terakhir, atau melahirkan
// tagihan "Tambahan". Itu masuk akal selama tagihan disemai sistem dari
// rencana termin. Sejak rencana termin dihapus, setiap tagihan dibuat manual
// oleh staff — dan menulis ulang nominal tagihan buatan orang secara diam-diam
// adalah hal yang berbeda sama sekali. Nilai kontrak tetap didorong; tagihannya
// urusan yang menerbitkan.
//
// packageName ikut di sini, bukan lewat panggilan kedua, supaya keduanya tidak
// mungkin berbeda cerita: revisi harga dan revisi nama tiba sebagai satu
// perubahan. "" berarti penawaran lama yang memang belum punya nama — nama
// project yang sudah ada TIDAK dikosongkan karenanya.
func (s *ProjectService) SyncFromQuotation(ctx context.Context, tenantID, projectID, total int64, packageName string) error {
	if projectID == 0 {
		return nil
	}
	p, err := s.repo.FindByID(ctx, tenantID, projectID)
	if err != nil {
		return err
	}
	if p == nil {
		return apperror.NotFound("Project tidak ditemukan")
	}
	if total < 0 {
		return apperror.Validation("Total pembayaran tidak boleh minus", map[string][]string{
			"total": {"Pengurangan harga melebihi harga paket"},
		})
	}
	changed := false
	if p.ContractValue != total {
		p.ContractValue = total
		changed = true
	}
	if packageName != "" && p.PackageName != packageName {
		p.PackageName = packageName
		changed = true
	}
	if changed {
		if err := s.repo.Update(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// ProjectDeleteImpact adalah bahan dialog hapus penawaran Diterima (T3.5):
// project yang ikut hilang + angka tagihan terbayarnya.
type ProjectDeleteImpact struct {
	ProjectID        int64
	ProjectName      string
	PaidInvoiceCount int
	PaidInvoiceTotal int64
}

func (s *ProjectService) ProjectDeleteImpact(ctx context.Context, tenantID, projectID int64) (ProjectDeleteImpact, error) {
	var out ProjectDeleteImpact
	p, err := s.Get(ctx, tenantID, projectID)
	if err != nil {
		return out, err
	}
	out.ProjectID = p.ID
	out.ProjectName = p.Name
	if s.invoices == nil {
		return out, nil
	}
	invoices, err := s.invoices.List(ctx, p.ID)
	if err != nil {
		return out, err
	}
	for _, inv := range invoices {
		if inv.Status == domain.InvoicePaid {
			out.PaidInvoiceCount++
			out.PaidInvoiceTotal += inv.Amount
		}
	}
	return out, nil
}

// ProjectIDsForQuotations memetakan penawaran Diterima ke project-nya untuk
// satu halaman daftar — satu query, bukan N+1.
func (s *ProjectService) ProjectIDsForQuotations(ctx context.Context, tenantID int64, quotationIDs []int64) (map[int64]int64, error) {
	return s.repo.ProjectIDsForQuotations(ctx, tenantID, quotationIDs)
}

// ProjectIDForQuotation mengembalikan project yang lahir dari penawaran —
// 0, nil bila penawaran belum punya project (fase penawaran).
func (s *ProjectService) ProjectIDForQuotation(ctx context.Context, tenantID, quotationID int64) (int64, error) {
	p, err := s.repo.FindByQuotationID(ctx, tenantID, quotationID)
	if err != nil {
		return 0, err
	}
	if p == nil {
		return 0, nil
	}
	return p.ID, nil
}

// PONumberForQuotation membaca nomor PO milik project untuk dialog
// delete-impact project (T3.5) — "" bila project lahir pra-penawaran,
// resolver belum dipasang, atau baris penawarannya SUDAH TIDAK ADA.
//
// Kasus terakhir itu bukan kegagalan. Sebuah project bisa menyimpan
// quotation_id yang barisnya sudah hilang (penawaran dihapus lebih dulu, atau
// data lama dari masa sebelum penghapusan berpasangan). Dulu 404-nya
// diteruskan apa adanya ke endpoint delete-impact, dan karena dialog hapus
// MENOLAK tampil bila dampaknya gagal dibaca (D14), project seperti itu tidak
// pernah bisa dihapus lewat layar sama sekali — padahal justru project yatim
// itulah yang paling perlu dihapus. Artinya di sini sederhana: tidak ada
// penawaran yang ikut terhapus, jadi tidak ada nomor untuk disebut.
//
// Hanya not-found yang ditelan. Galat lain (koneksi putus, tabel rusak) tetap
// diteruskan — melaporkannya sebagai "tidak ada penawaran" akan membuat dialog
// hapus berbohong tentang apa yang ikut hilang, dan itu persis yang D14 cegah.
func (s *ProjectService) PONumberForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	if quotationID == 0 || s.quotations == nil {
		return "", nil
	}
	number, err := s.quotations.PONumberForQuotation(ctx, tenantID, quotationID)
	if err != nil {
		if isNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return number, nil
}

// isNotFound membedakan "barisnya memang tidak ada" dari "pembacaannya gagal".
// Keduanya sampai ke sini sebagai error, tetapi hanya yang pertama boleh
// diperlakukan sebagai keadaan sah — lihat pemakaiannya di atas.
func isNotFound(err error) bool {
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr.Kind == apperror.KindNotFound
}

// ResolvePONumber adalah pasangan display-only dari PONumberForQuotation di
// atas: mengisi `poNumber` pada response detail project, yang menjadi label
// tautan "Penawaran" di header project setelah tab "Paket & PO" dihapus.
//
// Tiga keadaan, dan ketiganya berbeda — itulah sebabnya pointer, bukan string.
// nil berarti tidak ada penawaran yang bisa dituju: project pra-penawaran
// (quotation_id = 0), atau quotation_id yang tidak lagi resolve; header tidak
// memasang tautan sama sekali. Pointer ke "" berarti penawarannya ADA tapi
// tidak pernah bernomor (PO lama hasil migrasi 000061) — tautan tetap
// dipasang, labelnya apa adanya. Pointer ke "026/…" berarti ada dan bernomor.
//
// Membedakan nil dari "" itu yang penting: tanpa itu sebuah quotation_id
// menggantung tampil identik dengan PO lama tak bernomor, dan header memasang
// tautan yang 404 saat diklik. Menggabungkan keduanya persis kelemahan tab
// lama, yang baru memberi tahu "Penawaran tidak ditemukan" setelah dibuka.
//
// Error ditelan (jadi nil), persis seperti ResolvePICName — detail project
// tidak boleh gagal dimuat gara-gara label sebuah tautan. Yang mengembalikan
// error SENGAJA dipertahankan untuk dialog delete-impact: D14 mensyaratkan
// dialog itu menolak tampil ketika dampaknya gagal dibaca, jadi di sana
// kegagalan tidak boleh diam-diam hilang.
func (s *ProjectService) ResolvePONumber(ctx context.Context, tenantID, quotationID int64) *string {
	if quotationID == 0 || s.quotations == nil {
		return nil
	}
	number, err := s.quotations.PONumberForQuotation(ctx, tenantID, quotationID)
	if err != nil {
		return nil
	}
	return &number
}

// DeleteProjectCascade menghapus SATU project beserta seluruh isinya dan
// penawarannya, tanpa prasyarat arsip/batal (D14 — pengamannya dialog
// delete-impact, bukan blokir). Dipakai penghapusan project manual (Owner),
// cascade penawaran Diterima, dan cascade Client — satu mesin, tiga pintu.
func (s *ProjectService) DeleteProjectCascade(ctx context.Context, tenantID, id int64) error {
	p, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}

	evidences, err := s.evidence.List(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteCascade(ctx, tenantID, id); err != nil {
		return err
	}

	// Berjalan setelah commit — kegagalan di sini meninggalkan, paling buruk,
	// objek yatim, bukan referensi menggantung (urutan yang sama dengan Delete
	// yang lama).
	s.evidence.DeleteStorageObjects(ctx, evidences)

	if s.quotations != nil && p.QuotationID != 0 {
		// Penawaran yang barisnya memang sudah tidak ada bukan kegagalan:
		// keadaan yang dituju sudah tercapai. Mencatatnya sebagai ERROR
		// hanya melatih orang mengabaikan log error.
		if err := s.quotations.DeleteQuotation(ctx, tenantID, p.QuotationID); err != nil && !isNotFound(err) {
			logger.Error("gagal menghapus penawaran %d saat menghapus project %d: %v", p.QuotationID, id, err)
		}
	}

	if s.activator != nil && p.ClientID != 0 {
		// Best-effort: sisa project client ini mungkin nol — akun portal yang
		// tidak lagi punya project harus ikut nonaktif (D4).
		if err := s.activator.SyncCredentialActive(ctx, tenantID, p.ClientID); err != nil {
			logger.Error("gagal sinkron akun portal client %d setelah project %d dihapus: %v", p.ClientID, id, err)
		}
	}
	return nil
}
