package presentation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/modules/rundowns/presentation/templates"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/response"
)

const (
	collectionPath = "/api/v1/rundowns"
	itemPrefix     = "/api/v1/rundowns/"
	// usedProjectIDsSegment harus dicocokkan sebagai literal SEBELUM segmen
	// pertama di-ParseInt sebagai {id}; kalau tidak, endpoint ini jatuh ke
	// jalur item dan gagal parse.
	usedProjectIDsSegment = "used-project-ids"
	// templateSegment juga literal: /rundowns/template adalah Template
	// Rundown tenant, bukan rundown ber-{id}.
	templateSegment = "template"

	docxMime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	pdfMime  = "application/pdf"

	// generateBudget membatasi seluruh permintaan generate — antre semaphore
	// PDF, konversi (pdfTimeout 60 dtk), dan pengarsipan. nginx di depan app
	// harus diberi proxy_read_timeout di atas angka ini (infra/nginx).
	generateBudget = 110 * time.Second

	// archiveHeader memberi tahu frontend nasib salinan di tab Dokumen.
	archiveHeader = "X-Rundown-Archive"
)

// archiveStatus adalah nilai header archiveHeader.
type archiveStatus string

const (
	archiveShared  archiveStatus = "shared"  // tersimpan dan terlihat klien
	archivePrivate archiveStatus = "private" // tersimpan, belum dibagikan
	archiveFailed  archiveStatus = "failed"  // unduhan jalan, arsip gagal
)

// PDFConverter adalah antarmuka konsumen yang dideklarasikan di sisi pemakai —
// infrastructure.LibreOfficeConverter memenuhinya secara struktural.
type PDFConverter interface {
	ConvertToPDF(ctx context.Context, docx []byte) ([]byte, error)
	Available() bool
}

type Handler struct {
	rundowns  *application.RundownService
	templates *application.RundownTemplateService
	projects  projectscontracts.Contracts
	converter PDFConverter
}

func NewHandler(rundowns *application.RundownService, templates *application.RundownTemplateService,
	projects projectscontracts.Contracts, converter PDFConverter) *Handler {
	return &Handler{rundowns: rundowns, templates: templates, projects: projects, converter: converter}
}

// Collection melayani /api/v1/rundowns.
func (h *Handler) Collection(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireStaff(w, r)
	if !ok || !requireRundownAccess(w, claims.role) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.list(w, r, claims)
	case http.MethodPost:
		h.create(w, r, claims)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Metode tidak didukung", nil)
	}
}

// Item melayani /api/v1/rundowns/... — termasuk segmen literal
// used-project-ids, yang HARUS diperiksa sebelum ParseInt.
func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireStaff(w, r)
	if !ok || !requireRundownAccess(w, claims.role) {
		return
	}
	segs := httpx.Segments(r.URL.Path, itemPrefix)
	if len(segs) == 0 {
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
		return
	}
	if segs[0] == usedProjectIDsSegment {
		if r.Method != http.MethodGet {
			response.Error(w, http.StatusMethodNotAllowed, "Metode tidak didukung", nil)
			return
		}
		h.usedProjectIDs(w, r, claims)
		return
	}
	if segs[0] == templateSegment {
		h.template(w, r, claims, segs[1:])
		return
	}

	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		response.Error(w, http.StatusNotFound, "Rundown tidak ditemukan", nil)
		return
	}
	rest := segs[1:]

	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		h.get(w, r, claims, id)
	case len(rest) == 0 && r.Method == http.MethodDelete:
		h.delete(w, r, claims, id)
	case len(rest) == 2 && rest[0] == "sections" && r.Method == http.MethodPut:
		h.replaceSection(w, r, claims, id, domain.SectionKey(rest[1]))
	case len(rest) == 1 && rest[0] == "layout-image" && r.Method == http.MethodPost:
		h.uploadLayoutImage(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "generate" && r.Method == http.MethodGet:
		h.generate(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "project-prefill" && r.Method == http.MethodGet:
		h.projectPrefill(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "save-as-template" && r.Method == http.MethodPost:
		h.saveAsTemplate(w, r, claims, id)
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

// scopeFor menyusun batas project yang boleh dilihat pemanggil. nil berarti
// tanpa batas; slice kosong berarti Wedding Planner yang belum memegang satu
// project pun — dan hasilnya memang harus kosong.
func (h *Handler) scopeFor(ctx context.Context, claims staffClaims) (*[]int64, error) {
	if !isWeddingPlanner(claims.role) {
		return nil, nil
	}
	ids, err := h.projects.ProjectIDsForPICStaff(ctx, claims.tenantID, claims.staffID)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []int64{}
	}
	return &ids, nil
}

// authorize memastikan pemanggil boleh menyentuh rundown ini. Untuk Wedding
// Planner, project pemiliknya harus project yang dia pegang.
func (h *Handler) authorize(ctx context.Context, claims staffClaims, rundownID int64) error {
	projectID, err := h.rundowns.ProjectIDOf(ctx, claims.tenantID, rundownID)
	if err != nil {
		return err
	}
	if !isWeddingPlanner(claims.role) {
		return nil
	}
	pic, err := h.projects.ProjectPICStaffID(ctx, claims.tenantID, projectID)
	if err != nil {
		// Rundown yatim: project-nya sudah dihapus tetapi pembersihannya
		// (best-effort) gagal. 404, bukan 500 — Owner/Admin masih bisa
		// membukanya dan menghapus barisnya secara manual.
		if _, ok := apperror.As(err); ok {
			return apperror.NotFound("Rundown tidak ditemukan")
		}
		return err
	}
	if pic != claims.staffID {
		return apperror.Forbidden("Rundown ini bukan milik project yang Anda pegang")
	}
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, claims staffClaims) {
	scope, err := h.scopeFor(r.Context(), claims)
	if err != nil {
		writeAppError(w, err)
		return
	}
	params := pagination.FromRequest(r)
	rows, total, err := h.rundowns.List(r.Context(), claims.tenantID,
		application.ListFilter{ProjectIDs: scope, Search: r.URL.Query().Get("search")}, params)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OKPaginated(w, "Daftar rundown", toSummaryDTOs(rows),
		pagination.BuildMeta(params, int64(total)))
}

func (h *Handler) usedProjectIDs(w http.ResponseWriter, r *http.Request, claims staffClaims) {
	scope, err := h.scopeFor(r.Context(), claims)
	if err != nil {
		writeAppError(w, err)
		return
	}
	ids, err := h.rundowns.UsedProjectIDs(r.Context(), claims.tenantID, scope)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Project yang sudah punya rundown", ids)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, claims staffClaims) {
	var req createRundownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	if req.ProjectID <= 0 {
		response.Error(w, http.StatusBadRequest, "Project wajib dipilih",
			map[string][]string{"projectId": {"Project wajib dipilih"}})
		return
	}
	// Wedding Planner hanya boleh membuat rundown untuk project yang dia
	// pegang — diperiksa sebelum apa pun ditulis.
	if isWeddingPlanner(claims.role) {
		pic, err := h.projects.ProjectPICStaffID(r.Context(), claims.tenantID, req.ProjectID)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if pic != claims.staffID {
			writeAppError(w, apperror.Forbidden("Project ini bukan project yang Anda pegang"))
			return
		}
	}

	view, err := h.rundowns.CreateFromProject(r.Context(), claims.tenantID, req.toInput())
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Rundown dibuat", toViewDTO(view))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	if err := h.authorize(r.Context(), claims, id); err != nil {
		writeAppError(w, err)
		return
	}
	view, err := h.rundowns.Get(r.Context(), claims.tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Detail rundown", toViewDTO(view))
}

func (h *Handler) replaceSection(w http.ResponseWriter, r *http.Request, claims staffClaims,
	id int64, section domain.SectionKey) {

	if err := h.authorize(r.Context(), claims, id); err != nil {
		writeAppError(w, err)
		return
	}
	var req sectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	view, err := h.rundowns.ReplaceSection(r.Context(), claims.tenantID, id, section, req.toPayload())
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Seksi rundown disimpan", toViewDTO(view))
}

func (h *Handler) uploadLayoutImage(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	if err := h.authorize(r.Context(), claims, id); err != nil {
		writeAppError(w, err)
		return
	}
	var req layoutImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Base64Data)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Data file tidak valid",
			map[string][]string{"base64Data": {"Gagal membaca data file"}})
		return
	}
	if err := h.rundowns.SaveLayoutImage(r.Context(), claims.tenantID, id, data); err != nil {
		writeAppError(w, err)
		return
	}
	view, err := h.rundowns.Get(r.Context(), claims.tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Denah akad disimpan", toViewDTO(view))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	if !requireOwnerOrAdmin(w, claims.role) {
		return
	}
	if err := h.authorize(r.Context(), claims, id); err != nil {
		writeAppError(w, err)
		return
	}
	if err := h.rundowns.Delete(r.Context(), claims.tenantID, id); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Rundown dihapus", nil)
}

// template melayani /rundowns/template (GET) dan
// /rundowns/template/sections/{key} (PUT). Template berlaku se-tenant dan
// terbuka untuk setiap role yang boleh membuka Rundown (keputusan D4) — gerbang
// requireRundownAccess sudah dijalankan Item.
func (h *Handler) template(w http.ResponseWriter, r *http.Request, claims staffClaims, rest []string) {
	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		t, err := h.templates.Get(r.Context(), claims.tenantID)
		if err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "Template rundown", toTemplateDTO(t))
	case len(rest) == 2 && rest[0] == "sections" && r.Method == http.MethodPut:
		var req sectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
			return
		}
		t, err := h.templates.ReplaceSection(r.Context(), claims.tenantID,
			domain.SectionKey(rest[1]), req.toPayload())
		if err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "Seksi template disimpan", toTemplateDTO(t))
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

// projectPrefill memberi usulan isian sampul dari data project terkini untuk
// tombol "Tarik ulang dari project". Tidak menulis apa pun.
func (h *Handler) projectPrefill(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	if err := h.authorize(r.Context(), claims, id); err != nil {
		writeAppError(w, err)
		return
	}
	cover, err := h.rundowns.ProjectPrefill(r.Context(), claims.tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Data project terkini", toCoverPrefillDTO(cover))
}

// saveAsTemplate menjadikan isi rundown ini sebagai Template Rundown tenant.
func (h *Handler) saveAsTemplate(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	if err := h.authorize(r.Context(), claims, id); err != nil {
		writeAppError(w, err)
		return
	}
	t, err := h.templates.SaveFromRundown(r.Context(), claims.tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Template rundown diperbarui", toTemplateDTO(t))
}

// generate menyusun dokumen, mengunduhkannya, dan menyimpan salinannya ke tab
// Dokumen project. Urutannya disengaja: unduhan tidak boleh gagal hanya karena
// penyimpanan salinan gagal.
func (h *Handler) generate(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "docx"
	}
	if format != "docx" && format != "pdf" {
		response.Error(w, http.StatusBadRequest, "Format tidak dikenal",
			map[string][]string{"format": {"Hanya docx atau pdf"}})
		return
	}
	if format == "pdf" && !h.converter.Available() {
		response.Error(w, http.StatusServiceUnavailable,
			"Konversi PDF tidak tersedia di lingkungan ini. Unduh DOCX sebagai gantinya.", nil)
		return
	}
	if err := h.authorize(r.Context(), claims, id); err != nil {
		writeAppError(w, err)
		return
	}

	// Anggaran waktu sendiri untuk endpoint ini: WriteTimeout global server
	// (30 dtk, cmd/server/main.go) lebih pendek dari satu konversi PDF yang
	// sah (pdfTimeout) ditambah antrean semaphore-nya. Tanpa perpanjangan ini
	// respons terpotong di tengah jalan sementara server masih bekerja.
	ctx, cancel := context.WithTimeout(r.Context(), generateBudget)
	defer cancel()
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(generateBudget + 10*time.Second)); err != nil {
		logger.Error("gagal memperpanjang batas tulis generate rundown %d: %v", id, err)
	}

	view, err := h.rundowns.Get(ctx, claims.tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}

	// Gagal membaca denah tidak menggagalkan generate: template memakai
	// gambar placeholder bawaannya dan sisa dokumen tetap benar.
	layoutPNG, err := h.rundowns.LayoutImage(ctx, claims.tenantID, id)
	if err != nil {
		logger.Error("gagal membaca denah rundown %d: %v", id, err)
		layoutPNG = nil
	}

	docx, err := buildRundownDocx(view, templates.RundownDocx, layoutPNG)
	if err != nil {
		logger.Error("gagal menyusun dokumen rundown %d: %v", id, err)
		response.Error(w, http.StatusInternalServerError, "Gagal menyusun dokumen rundown", nil)
		return
	}

	payload, mime, ext := docx, docxMime, "docx"
	if format == "pdf" {
		pdf, err := h.converter.ConvertToPDF(ctx, docx)
		if err != nil {
			logger.Error("gagal mengonversi rundown %d ke PDF: %v", id, err)
			// Hanya penantian semaphore yang mengembalikan ctx.Err() mentah;
			// timeout konversi sendiri dibungkus tanpa %w dan jatuh ke 500 di
			// bawah. Jadi 503 di sini memang berarti "antre terlalu lama".
			if errors.Is(err, context.DeadlineExceeded) {
				response.Error(w, http.StatusServiceUnavailable,
					"Server sedang membuat PDF lain. Coba lagi sebentar lagi, atau unduh DOCX.", nil)
				return
			}
			response.Error(w, http.StatusInternalServerError,
				"Gagal membuat PDF. Unduh DOCX sebagai gantinya.", nil)
			return
		}
		payload, mime, ext = pdf, pdfMime, "pdf"
	}

	archive := h.archiveGenerated(ctx, claims, view, payload, mime, ext)

	filename := documentFilename(view, ext)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set(archiveHeader, string(archive))
	if _, err := w.Write(payload); err != nil {
		logger.Error("gagal mengirim berkas rundown %d: %v", id, err)
	}
}

// archiveGenerated menyimpan salinan ke tab Dokumen project. Seluruhnya
// best-effort: kegagalan di sini hanya dicatat, unduhan tetap diteruskan —
// WO yang sedang butuh berkasnya di lapangan tidak boleh dihalangi oleh
// kegagalan pengarsipan. Hasilnya dilaporkan lewat header X-Rundown-Archive
// supaya WO tahu berkasnya tersimpan di mana dan apakah klien melihatnya.
func (h *Handler) archiveGenerated(ctx context.Context, claims staffClaims,
	view *domain.View, payload []byte, mime, ext string) archiveStatus {

	replaceID := view.Rundown.LastDocxEvidenceID
	if ext == "pdf" {
		replaceID = view.Rundown.LastPdfEvidenceID
	}
	saved, err := h.projects.SaveGeneratedDocument(ctx, claims.tenantID,
		view.Rundown.ProjectID, claims.staffID, projectscontracts.GeneratedDocInput{
			Name:              "Rundown " + view.Rundown.ProjectName,
			FileName:          documentFilename(view, ext),
			MimeType:          mime,
			Data:              payload,
			ReplaceEvidenceID: replaceID,
		})
	if err != nil {
		logger.Error("gagal mengarsipkan rundown %d ke dokumen project: %v", view.Rundown.ID, err)
		return archiveFailed
	}
	if err := h.rundowns.RecordGeneratedEvidence(ctx, claims.tenantID,
		view.Rundown.ID, ext, saved.EvidenceID); err != nil {
		// Berkasnya sudah tersimpan; yang gagal hanya catatan untuk
		// menggantikannya di generate berikutnya.
		logger.Error("gagal mencatat dokumen hasil generate rundown %d: %v", view.Rundown.ID, err)
	}
	if saved.ClientVisible {
		return archiveShared
	}
	return archivePrivate
}

func documentFilename(view *domain.View, ext string) string {
	name := strings.TrimSpace(view.Rundown.ProjectName)
	if name == "" {
		name = "Rundown-" + strconv.FormatInt(view.Rundown.ID, 10)
	}
	name = strings.NewReplacer("/", "-", "\\", "-", `"`, "", ":", "-").Replace(name)
	return "Rundown - " + name + "." + ext
}
