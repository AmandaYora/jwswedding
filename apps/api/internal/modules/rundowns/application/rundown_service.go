package application

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // mendaftarkan dekoder JPEG untuk image.DecodeConfig
	"image/png"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/compress"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/pagination"
)

// maxLayoutImageSize membatasi PNG denah akad. Jauh di bawah batas evidence
// (15 MB) karena yang diharapkan adalah satu diagram denah, bukan foto.
const maxLayoutImageSize = 5 * 1024 * 1024

type RundownService struct {
	repo      RundownRepository
	projects  projectscontracts.Contracts
	storage   ObjectStorage
	templates TemplateRepository
}

func NewRundownService(repo RundownRepository, projects projectscontracts.Contracts, storage ObjectStorage,
	templates TemplateRepository) *RundownService {
	return &RundownService{repo: repo, projects: projects, storage: storage, templates: templates}
}

func (s *RundownService) List(ctx context.Context, tenantID int64, filter ListFilter,
	params pagination.Params) ([]domain.Summary, int, error) {
	return s.repo.List(ctx, tenantID, filter, params)
}

func (s *RundownService) UsedProjectIDs(ctx context.Context, tenantID int64, scope *[]int64) ([]int64, error) {
	return s.repo.UsedProjectIDs(ctx, tenantID, scope)
}

// Get memuat aggregate utuh dan sekalian menyegarkan snapshot nama project.
// Penyegaran dijalankan best-effort: project yang sudah dihapus tidak boleh
// membuat buku acaranya mustahil dibuka (rundown yatim masih harus bisa
// dilihat dan dihapus Owner/Admin — PLAN §9.1 poin 4).
func (s *RundownService) Get(ctx context.Context, tenantID, id int64) (*domain.View, error) {
	v, err := s.repo.LoadAggregate(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, apperror.NotFound("Rundown tidak ditemukan")
	}
	if pc, err := s.projects.RundownProjectContext(ctx, tenantID, v.Rundown.ProjectID); err == nil {
		if pc.ProjectName != v.Rundown.ProjectName {
			if err := s.repo.UpdateProjectName(ctx, tenantID, v.Rundown.ID, pc.ProjectName); err != nil {
				logger.Error("gagal menyegarkan nama project pada rundown %d: %v", v.Rundown.ID, err)
			} else {
				v.Rundown.ProjectName = pc.ProjectName
			}
		}
	}
	return v, nil
}

// ProjectIDOf menjawab project pemilik sebuah rundown — dipakai gerbang
// otorisasi di presentation sebelum aggregate penuh dimuat.
func (s *RundownService) ProjectIDOf(ctx context.Context, tenantID, id int64) (int64, error) {
	r, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return 0, err
	}
	if r == nil {
		return 0, apperror.NotFound("Rundown tidak ditemukan")
	}
	return r.ProjectID, nil
}

// CreateInput adalah payload "Buat Rundown". Field bernama *Prefill dipasok
// frontend dari store-nya sendiri (nama vendor/kategori milik modul `vendors`
// — MODULE_MAP.md); sisanya diisi server dari project.
type CreateInput struct {
	ProjectID      int64
	VendorPrefill  []domain.Vendor
	WOPICName      string
	WOPICPhone     string
	EventTimeLabel string
	// UseTemplate menyalin enam seksi Template Rundown tenant ke rundown baru.
	UseTemplate bool
}

// CoverPrefill adalah bagian sampul yang bisa diturunkan dari project. Dipakai
// saat membuat rundown DAN saat WO menarik ulang data project, supaya kedua
// jalur itu pasti menghasilkan nilai yang sama.
type CoverPrefill struct {
	GroomName      string
	BrideName      string
	EventDateLabel string
	VenueLabel     string
	EventTimeLabel string
	CoupleTitle    string
}

// coverFromProject menurunkan isian sampul dari konteks project.
func coverFromProject(pc projectscontracts.RundownProjectContext) CoverPrefill {
	timeLabel := ""
	if pc.EventStartTime != "" {
		timeLabel = pc.EventStartTime
		if pc.EventEndTime != "" {
			timeLabel += " - " + pc.EventEndTime
		}
		timeLabel += " wib"
	}
	return CoverPrefill{
		GroomName:      strings.ToUpper(pc.GroomName),
		BrideName:      strings.ToUpper(pc.BrideName),
		EventDateLabel: formatEventDate(pc.EventDate),
		VenueLabel:     pc.Venue,
		EventTimeLabel: timeLabel,
		CoupleTitle:    coupleTitle(pc.BrideName, pc.GroomName),
	}
}

// ProjectPrefill memberi usulan isian sampul dari data project TERKINI, untuk
// tombol "Tarik ulang dari project". Tidak menulis apa pun: WO yang memilih
// field mana yang diterapkan.
func (s *RundownService) ProjectPrefill(ctx context.Context, tenantID, id int64) (CoverPrefill, error) {
	r, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return CoverPrefill{}, err
	}
	if r == nil {
		return CoverPrefill{}, apperror.NotFound("Rundown tidak ditemukan")
	}
	pc, err := s.projects.RundownProjectContext(ctx, tenantID, r.ProjectID)
	if err != nil {
		if appErr, ok := apperror.As(err); ok && appErr.Kind == apperror.KindNotFound {
			return CoverPrefill{}, apperror.NotFound("Project rundown ini sudah tidak ada")
		}
		return CoverPrefill{}, err
	}
	return coverFromProject(pc), nil
}

// CreateFromProject melahirkan buku acara dari sebuah project. Field yang
// sudah ada di project langsung terpakai; sisanya dibiarkan kosong untuk
// dilengkapi WO — atau diisi dari Template Rundown bila diminta.
func (s *RundownService) CreateFromProject(ctx context.Context, tenantID int64, in CreateInput) (*domain.View, error) {
	// Gerbang keberadaan project sekaligus sumber prefill — NotFound bila
	// project tidak ada atau milik tenant lain.
	pc, err := s.projects.RundownProjectContext(ctx, tenantID, in.ProjectID)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByProject(ctx, tenantID, in.ProjectID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperror.Conflict("Project ini sudah punya rundown")
	}

	cover := coverFromProject(pc)
	if t := strings.TrimSpace(in.EventTimeLabel); t != "" {
		cover.EventTimeLabel = t
	}

	r := &domain.Rundown{
		TenantID:       tenantID,
		ProjectID:      pc.ProjectID,
		ProjectName:    pc.ProjectName,
		GroomName:      cover.GroomName,
		BrideName:      cover.BrideName,
		EventDateLabel: cover.EventDateLabel,
		VenueLabel:     cover.VenueLabel,
		EventTimeLabel: cover.EventTimeLabel,
		CoupleTitle:    cover.CoupleTitle,
		WOPICName:      in.WOPICName,
		WOPICPhone:     in.WOPICPhone,
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}

	if len(in.VendorPrefill) > 0 {
		if err := s.repo.ReplaceSection(ctx, tenantID, r.ID, domain.SectionKeyVendors,
			SectionPayload{Vendors: in.VendorPrefill}); err != nil {
			// Buku acaranya sudah lahir; daftar vendor bisa diisi ulang dari
			// editor. Menggagalkan pembuatan di sini justru meninggalkan
			// project tanpa rundown padahal barisnya sudah tercipta.
			logger.Error("gagal menulis prefill vendor rundown %d: %v", r.ID, err)
		}
	}
	if in.UseTemplate {
		s.applyTemplate(ctx, tenantID, r.ID)
	}
	return s.Get(ctx, tenantID, r.ID)
}

// applyTemplate menyalin seksi Template Rundown yang berisi ke rundown baru.
// Best-effort, sama seperti prefill vendor: buku acaranya sudah lahir, dan
// seksi yang gagal tersalin masih bisa diisi dari editor.
func (s *RundownService) applyTemplate(ctx context.Context, tenantID, rundownID int64) {
	if s.templates == nil {
		return
	}
	t, err := s.templates.Get(ctx, tenantID)
	if err != nil {
		logger.Error("gagal membaca template untuk rundown %d: %v", rundownID, err)
		return
	}
	if t == nil {
		return
	}
	for _, sec := range templateSections(t) {
		if sec.empty {
			continue
		}
		if err := s.repo.ReplaceSection(ctx, tenantID, rundownID, sec.key, sec.payload); err != nil {
			logger.Error("gagal menyalin seksi template %s ke rundown %d: %v", sec.key, rundownID, err)
		}
	}
}

// templateSection adalah satu seksi Template Rundown dalam bentuk payload
// PUT seksi — bentuk yang sama yang ditulis ReplaceSection repository.
type templateSection struct {
	key     domain.SectionKey
	payload SectionPayload
	empty   bool
}

func templateSections(t *domain.Template) []templateSection {
	return []templateSection{
		{domain.SectionKeyRoles, SectionPayload{Roles: t.Roles}, len(t.Roles) == 0},
		{domain.SectionKeyCommittees, SectionPayload{Committees: t.Committees}, len(t.Committees) == 0},
		{domain.SectionKeyMakeup, SectionPayload{MakeupRooms: t.MakeupRooms}, len(t.MakeupRooms) == 0},
		{domain.SectionKeyAcaraAkad, SectionPayload{Items: t.ItemsAkad}, len(t.ItemsAkad) == 0},
		{domain.SectionKeyAcaraResepsi, SectionPayload{Items: t.ItemsResepsi}, len(t.ItemsResepsi) == 0},
		{domain.SectionKeyLayout, SectionPayload{LayoutNotes: t.LayoutNotes}, len(t.LayoutNotes) == 0},
	}
}

// ReplaceSection menulis ulang satu seksi. Untuk dua seksi SUSUNAN ACARA,
// penomoran `no_label` diisi ulang di sini — server yang memegang otoritasnya,
// bukan payload klien, supaya cacat penomoran 1-2-3-3-3 di berkas contoh tidak
// mungkin terulang.
func (s *RundownService) ReplaceSection(ctx context.Context, tenantID, id int64,
	section domain.SectionKey, payload SectionPayload) (*domain.View, error) {

	if !section.Valid() {
		return nil, apperror.Validation("Seksi tidak dikenal",
			map[string][]string{"section": {"Nilai seksi tidak valid"}})
	}
	if _, err := s.ProjectIDOf(ctx, tenantID, id); err != nil {
		return nil, err
	}
	if err := validateSection(section, &payload); err != nil {
		return nil, err
	}
	if section == domain.SectionKeyAcaraAkad || section == domain.SectionKeyAcaraResepsi {
		renumberItems(payload.Items)
	}
	if section == domain.SectionKeyLayout {
		renumberLegend(payload.LayoutNotes)
	}
	if err := s.repo.ReplaceSection(ctx, tenantID, id, section, payload); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, id)
}

// SaveLayoutImage menyimpan denah akad. Entry gambar di dalam template adalah
// PNG, jadi yang DISIMPAN selalu PNG — menukar byte format lain akan tidak
// cocok dengan content-type yang sudah tercatat di berkas .docx. JPEG (format
// denah yang lazim dikirim venue) diterima dan dikonversi ke PNG di sini.
func (s *RundownService) SaveLayoutImage(ctx context.Context, tenantID, id int64, data []byte) error {
	r, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if r == nil {
		return apperror.NotFound("Rundown tidak ditemukan")
	}
	if len(data) == 0 {
		return apperror.Validation("File kosong",
			map[string][]string{"base64Data": {"File tidak boleh kosong"}})
	}
	if len(data) > maxLayoutImageSize {
		return apperror.Validation("Ukuran file terlalu besar",
			map[string][]string{"base64Data": {"Maksimal 5 MB"}})
	}
	// Batas 5 MB di atas berlaku pada berkas masukan; hasil konversi JPEG
	// sudah dikecilkan ke sisi terpanjang 2000 px oleh compress.ToPNG.
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	invalid := apperror.Validation("Format gambar harus PNG atau JPG",
		map[string][]string{"base64Data": {"Denah akad harus berupa berkas PNG atau JPG"}})
	if err != nil {
		return invalid
	}
	switch format {
	case "png":
		if _, err := png.Decode(bytes.NewReader(data)); err != nil {
			return invalid
		}
	case "jpeg":
		converted, err := compress.ToPNG(data)
		if err != nil {
			return invalid
		}
		data = converted
	default:
		return invalid
	}

	key := fmt.Sprintf("rundowns/%s/%s/layout-%s.png",
		strconv.FormatInt(tenantID, 10), strconv.FormatInt(id, 10), uuid.NewString())
	if _, err := s.storage.Save(ctx, key, data, "image/png"); err != nil {
		return apperror.Internal("Gagal mengunggah denah ke object storage")
	}
	old := r.LayoutImagePath
	if err := s.repo.UpdateLayoutImagePath(ctx, tenantID, id, key); err != nil {
		return err
	}
	if old != "" && old != key {
		// Setelah kolom menunjuk objek baru — gagal di sini hanya menyisakan
		// objek yatim, bukan referensi menggantung.
		if err := s.storage.Delete(ctx, old); err != nil {
			logger.Error("gagal menghapus denah lama %s: %v", old, err)
		}
	}
	return nil
}

// LayoutImage membaca PNG denah milik rundown. Mengembalikan nil tanpa galat
// bila belum ada denah yang diunggah — template memakai gambar placeholder
// bawaannya dan generate tetap berjalan.
func (s *RundownService) LayoutImage(ctx context.Context, tenantID, id int64) ([]byte, error) {
	r, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil || r.LayoutImagePath == "" {
		return nil, nil
	}
	rc, err := s.storage.Open(ctx, r.LayoutImagePath)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// RecordGeneratedEvidence mencatat dokumen hasil generate terakhir per format,
// supaya generate berikutnya menggantikannya alih-alih menumpuk.
func (s *RundownService) RecordGeneratedEvidence(ctx context.Context, tenantID, id int64,
	format string, evidenceID int64) error {
	if _, err := s.ProjectIDOf(ctx, tenantID, id); err != nil {
		return err
	}
	return s.repo.UpdateEvidenceID(ctx, tenantID, id, format, evidenceID)
}

func (s *RundownService) Delete(ctx context.Context, tenantID, id int64) error {
	r, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if r == nil {
		return apperror.NotFound("Rundown tidak ditemukan")
	}
	if r.LayoutImagePath != "" {
		if err := s.storage.Delete(ctx, r.LayoutImagePath); err != nil {
			logger.Error("gagal menghapus denah rundown %d: %v", id, err)
		}
	}
	return s.repo.Delete(ctx, tenantID, id)
}

// DeleteRundownForProject memenuhi projects/application.RundownCleaner:
// dipanggil saat sebuah project dihapus permanen. Project tanpa rundown bukan
// kegagalan — keadaan yang dituju memang sudah tercapai.
func (s *RundownService) DeleteRundownForProject(ctx context.Context, tenantID, projectID int64) error {
	r, err := s.repo.FindByProject(ctx, tenantID, projectID)
	if err != nil {
		return err
	}
	if r == nil {
		return nil
	}
	return s.Delete(ctx, tenantID, r.ID)
}

// ----------------------------------------------------------------- bantuan

// renumberItems menomori ulang baris SUSUNAN ACARA 1..n. Baris yang ditandai
// "tanpa nomor" (NoLabel "-") dilewati dan dikosongkan, supaya baris pembuka
// seperti "04.00 - 14.00 Checking Dekor" tetap tampil tanpa nomor.
func renumberItems(items []domain.Item) {
	n := 0
	for i := range items {
		if strings.TrimSpace(items[i].NoLabel) == "-" {
			items[i].NoLabel = ""
			continue
		}
		n++
		items[i].NoLabel = strconv.Itoa(n)
	}
}

// renumberLegend menomori ulang tabel keterangan denah; aturan tamu (Rule)
// dinomori terpisah karena tercetak sebagai daftar bernomor sendiri.
func renumberLegend(notes []domain.LayoutNote) {
	legend, rule := 0, 0
	for i := range notes {
		if notes[i].Kind == domain.LayoutNoteRule {
			rule++
			notes[i].NumberLabel = strconv.Itoa(rule)
			continue
		}
		legend++
		notes[i].NumberLabel = strconv.Itoa(legend)
	}
}

// checkLen menjaga panjang nilai terhadap lebar kolom VARCHAR-nya.
//
// Batas yang sama memang sudah ada di skema Zod editor, tetapi itu hanya
// berlaku untuk yang lewat editor. Tanpa pemeriksaan di sini, payload yang
// lebih panjang sampai ke MySQL dalam mode ketat dan ditolak sebagai galat
// 1406 — terbaca pengguna sebagai 500 "Terjadi kesalahan pada server", padahal
// yang terjadi cuma satu kolom kepanjangan dan itu sepenuhnya bisa dijelaskan.
//
// RuneCount, bukan len(): VARCHAR(150) di MySQL berarti 150 KARAKTER, dan satu
// karakter non-ASCII memakan lebih dari satu byte.
func checkLen(fields map[string][]string, key, value string, max int) {
	if utf8.RuneCountInString(value) > max {
		fields[key] = []string{fmt.Sprintf("Maksimal %d karakter", max)}
	}
}

func validateSection(section domain.SectionKey, p *SectionPayload) error {
	fields := map[string][]string{}
	switch section {
	case domain.SectionKeyCover:
		c := p.Cover
		for key, pair := range map[string]struct {
			val string
			max int
		}{
			"cover.groomName": {c.GroomName, 255}, "cover.groomBirthOrder": {c.GroomBirthOrder, 255},
			"cover.groomParents": {c.GroomParents, 255}, "cover.brideName": {c.BrideName, 255},
			"cover.brideBirthOrder": {c.BrideBirthOrder, 255}, "cover.brideParents": {c.BrideParents, 255},
			"cover.eventDateLabel": {c.EventDateLabel, 255}, "cover.venueLabel": {c.VenueLabel, 255},
			"cover.eventTimeLabel": {c.EventTimeLabel, 255}, "cover.coupleTitle": {c.CoupleTitle, 255},
			"cover.woPicName": {c.WOPICName, 100}, "cover.woPicPhone": {c.WOPICPhone, 100},
		} {
			checkLen(fields, key, pair.val, pair.max)
		}

	case domain.SectionKeyVendors:
		for i, v := range p.Vendors {
			checkLen(fields, fmt.Sprintf("vendors[%d].categoryLabel", i), v.CategoryLabel, 100)
			checkLen(fields, fmt.Sprintf("vendors[%d].vendorName", i), v.VendorName, 150)
		}

	case domain.SectionKeyRoles:
		for i, v := range p.Roles {
			checkLen(fields, fmt.Sprintf("roles[%d].roleLabel", i), v.RoleLabel, 150)
			checkLen(fields, fmt.Sprintf("roles[%d].personName", i), v.PersonName, 150)
			checkLen(fields, fmt.Sprintf("roles[%d].note", i), v.Note, 255)
		}

	case domain.SectionKeyCommittees:
		for i, v := range p.Committees {
			checkLen(fields, fmt.Sprintf("committees[%d].roleLabel", i), v.RoleLabel, 150)
		}

	case domain.SectionKeyDataLainnya:
		checkLen(fields, "dataLainnya.souvenirNote", p.DataLainnya.SouvenirNote, 255)
		checkLen(fields, "dataLainnya.tableClothNote", p.DataLainnya.TableClothNote, 255)
		for i, m := range p.MenuItems {
			if !m.GroupKey.Valid() {
				fields[fmt.Sprintf("menuItems[%d].groupKey", i)] = []string{"Kolom menu tidak dikenal"}
			}
			if !m.Style.Valid() {
				fields[fmt.Sprintf("menuItems[%d].style", i)] = []string{"Gaya baris tidak dikenal"}
			}
		}

	case domain.SectionKeyMakeup:
		for i, room := range p.MakeupRooms {
			checkLen(fields, fmt.Sprintf("makeupRooms[%d].roomLabel", i), room.RoomLabel, 150)
			for j, line := range room.Lines {
				if !line.Style.Valid() {
					fields[fmt.Sprintf("makeupRooms[%d].lines[%d].style", i, j)] = []string{"Gaya baris tidak dikenal"}
				}
			}
		}

	case domain.SectionKeyAcaraAkad, domain.SectionKeyAcaraResepsi:
		// no_label tidak diperiksa: isinya ditimpa renumberItems.
		for i, v := range p.Items {
			checkLen(fields, fmt.Sprintf("items[%d].timeLabel", i), v.TimeLabel, 50)
		}

	case domain.SectionKeyLayout:
		// number_label juga ditimpa server (renumberLegend).
		for i, n := range p.LayoutNotes {
			if !n.Kind.Valid() {
				fields[fmt.Sprintf("layoutNotes[%d].kind", i)] = []string{"Jenis catatan tidak dikenal"}
			}
		}

	case domain.SectionKeyFotoTamu:
		for i, g := range p.PhotoGroups {
			checkLen(fields, fmt.Sprintf("photoGroups[%d].groupName", i), g.GroupName, 255)
		}

	case domain.SectionKeyTamuVIP:
		for i, g := range p.VIPGuests {
			checkLen(fields, fmt.Sprintf("vipGuests[%d].fullName", i), g.FullName, 150)
			checkLen(fields, fmt.Sprintf("vipGuests[%d].position", i), g.Position, 150)
		}

	case domain.SectionKeyPlaylist:
		for i, e := range p.Playlist {
			checkLen(fields, fmt.Sprintf("playlist[%d].title", i), e.Title, 255)
			checkLen(fields, fmt.Sprintf("playlist[%d].artist", i), e.Artist, 150)
		}
	}
	if len(fields) > 0 {
		return apperror.Validation("Data seksi tidak valid", fields)
	}
	return nil
}
