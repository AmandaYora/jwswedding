package presentation

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/quotations/application"
	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/response"
)

type Handler struct {
	quotations *application.QuotationService
	platform   platformcontracts.Contracts
	projects   projectscontracts.Contracts
	links      *application.SignatureLinkService
	// staff memasok nama + jabatan + TTD penerbit dokumen untuk kolom WO pada
	// blok tanda tangan PDF (PLAN tanda-tangan-pengguna).
	staff application.StaffSignerResolver
}

func NewHandler(quotations *application.QuotationService, platform platformcontracts.Contracts, projects projectscontracts.Contracts, links *application.SignatureLinkService, staff application.StaffSignerResolver) *Handler {
	return &Handler{quotations: quotations, platform: platform, projects: projects, links: links, staff: staff}
}

type quotationBlockBody struct {
	Category  string `json:"category"`
	Body      string `json:"body"`
	QtyText   string `json:"qtyText"`
	BonusNote string `json:"bonusNote"`
}

// packageBlockBody adalah bentuk yang dipakai template handler (pindahan dari
// projects) — alias ke bentuk penawaran supaya satu PUT shape berlaku di
// kedua editor.
type packageBlockBody = quotationBlockBody

type quotationAdjustmentBody struct {
	Description string `json:"description"`
	// Amount BERTANDA — negatif adalah takeout/cashback. Frontend mengirim
	// tandanya; tidak ada kunci "kind" terpisah.
	Amount int64 `json:"amount"`
}

type quotationResponse struct {
	ID         int64  `json:"id"`
	ClientID   int64  `json:"clientId"`
	ClientName string `json:"clientName"`
	// PONumber "" sampai dikirim ke klien (D7), permanen setelahnya.
	PONumber  string `json:"poNumber"`
	Revision  int    `json:"revision"`
	Status    string `json:"status"`
	BasePrice int64  `json:"basePrice"`
	// PackageName adalah nama komersial paket ("Silver") — inilah yang jadi
	// "Paket / Layanan" di project dan baris "Paket" di PDF Invoice. ""
	// hanya pada penawaran pra-000065; Issue mewajibkannya.
	PackageName string  `json:"packageName"`
	TermsText   string  `json:"termsText"`
	BonusNote   string  `json:"bonusNote"`
	EventDate   *string `json:"eventDate"`
	Pax         int     `json:"pax"`
	VenueID     *int64  `json:"venueId"`
	VenueName   string  `json:"venueName"`
	IssuedAt    *string `json:"issuedAt"`
	AcceptedAt  *string `json:"acceptedAt"`
	ProjectID   int64   `json:"projectId"`

	Blocks      []quotationBlockResponse      `json:"blocks"`
	Adjustments []quotationAdjustmentResponse `json:"adjustments"`
	// TotalAdjustments/Total menghindarkan frontend menghitung ulang apa yang
	// backend memang harus hitung untuk contract_value (D15).
	TotalAdjustments int64 `json:"totalAdjustments"`
	Total            int64 `json:"total"`
	// Signature adalah ringkasan pembubuhan pada revisi yang berlaku (T9,
	// §6.5) — null bila belum diteken. BUKAN snapshot_json mentah, dan TANPA
	// storageKey: gambarnya disajikan lewat endpoint tersendiri.
	Signature *quotationSignatureResponse `json:"signature"`
}

// quotationSignatureResponse adalah yang dibawa respons API tentang TTD:
// siapa, kapan, lewat jalur apa. Kunci penyimpanan tidak pernah keluar —
// idiom yang sama dipakai evidence.
type quotationSignatureResponse struct {
	SignerName string `json:"signerName"`
	SignerRole string `json:"signerRole"`
	SignedAt   string `json:"signedAt"`
	Channel    string `json:"channel"`
}

func toSignatureResponse(sig *domain.QuotationClientSignature) *quotationSignatureResponse {
	if sig == nil {
		return nil
	}
	return &quotationSignatureResponse{
		SignerName: sig.SignerName, SignerRole: sig.SignerRole,
		SignedAt: sig.SignedAt.Format(time.RFC3339), Channel: sig.Channel,
	}
}

type quotationBlockResponse struct {
	ID        int64  `json:"id"`
	Category  string `json:"category"`
	Body      string `json:"body"`
	QtyText   string `json:"qtyText"`
	BonusNote string `json:"bonusNote"`
	SortOrder int    `json:"sortOrder"`
}

// packageBlockResponse adalah nama yang dipakai template handler (pindahan
// dari projects) — alias supaya kedua editor berbagi satu shape.
type packageBlockResponse = quotationBlockResponse

type quotationAdjustmentResponse struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	SortOrder   int    `json:"sortOrder"`
}

type quotationListItemResponse struct {
	ID         int64  `json:"id"`
	ClientID   int64  `json:"clientId"`
	ClientName string `json:"clientName"`
	PONumber   string `json:"poNumber"`
	Revision   int    `json:"revision"`
	Status     string `json:"status"`
	BasePrice  int64  `json:"basePrice"`
	// Total = basePrice + penyesuaian; nilai kontrak yang benar-benar dibuat
	// saat penawaran diterima.
	Total      int64   `json:"total"`
	EventDate  *string `json:"eventDate"`
	ProjectID  int64   `json:"projectId"`
	// SalesStaffID adalah pembuat penawaran — dasar filter Sales (D3).
	SalesStaffID int64 `json:"salesStaffId"`
	// PICStaffID adalah Wedding Planner dari project hasil Accept (0 = belum
	// ada project / belum ditugaskan) — dasar tampilan nama WP di kartu (D7).
	PICStaffID int64   `json:"picStaffId"`
	IssuedAt   *string `json:"issuedAt"`
	AcceptedAt *string `json:"acceptedAt"`
	// Signed = revisi yang berlaku sudah bertanda tangan (T9, D13a).
	Signed bool `json:"signed"`
}

func formatDatePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(dateLayout)
	return &s
}

func toQuotationResponse(view *application.QuotationView) *quotationResponse {
	if view == nil || view.Quotation == nil {
		return nil
	}
	o := view.Quotation
	clientName := strings.TrimSpace(view.ClientBride + " & " + view.ClientGroom)
	if view.ClientBride == "" && view.ClientGroom == "" {
		clientName = ""
	}
	out := &quotationResponse{
		ID: o.ID, ClientID: o.ClientID, ClientName: clientName,
		PONumber: o.PONumber, Revision: o.Revision, Status: string(o.Status),
		BasePrice: o.BasePrice, PackageName: o.PackageName, TermsText: o.TermsText, BonusNote: o.BonusNote,
		EventDate: formatDatePtr(o.EventDate), Pax: o.Pax,
		VenueID: o.VenueID, VenueName: view.VenueName,
		IssuedAt: formatDatePtr(o.IssuedAt), AcceptedAt: formatDatePtr(o.AcceptedAt),
		ProjectID:        view.ProjectID,
		Blocks:           make([]quotationBlockResponse, 0, len(view.Blocks)),
		Adjustments:      make([]quotationAdjustmentResponse, 0, len(view.Adjustments)),
		TotalAdjustments: domain.TotalAdjustments(view.Adjustments),
		Total:            view.Total,
	}
	if o.Snapshot != nil {
		out.Signature = toSignatureResponse(o.Snapshot.Current.Signature)
	}
	for _, b := range view.Blocks {
		out.Blocks = append(out.Blocks, quotationBlockResponse{
			ID: b.ID, Category: b.Category, Body: b.Body, QtyText: b.QtyText,
			BonusNote: b.BonusNote, SortOrder: b.SortOrder,
		})
	}
	for _, a := range view.Adjustments {
		out.Adjustments = append(out.Adjustments, quotationAdjustmentResponse{
			ID: a.ID, Description: a.Description, Amount: a.Amount, SortOrder: a.SortOrder,
		})
	}
	return out
}

func (h *Handler) writeQuotation(w http.ResponseWriter, view *application.QuotationView, err error) {
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toQuotationResponse(view))
}

// Collection melayani /api/v1/quotations — daftar (filter status, cari nomor,
// saring client untuk dropdown Tambah Project) + buat Draft baru.
func (h *Handler) Collection(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireStaff(w, r)
	if !ok {
		return
	}
	if !requireQuotationManager(w, claims.role) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.list(w, r, claims)
	case http.MethodPost:
		h.create(w, r, claims)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, claims staffClaims) {
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	var clientID int64
	if raw := r.URL.Query().Get("clientId"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "clientId tidak valid", nil)
			return
		}
		clientID = parsed
	}
	var salesStaffID int64
	if raw := r.URL.Query().Get("salesStaffId"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "salesStaffId tidak valid", nil)
			return
		}
		salesStaffID = parsed
	}
	var picStaffID int64
	if raw := r.URL.Query().Get("picStaffId"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "picStaffId tidak valid", nil)
			return
		}
		picStaffID = parsed
	}
	params := pagination.FromRequest(r)
	items, total, err := h.quotations.ListPaginated(r.Context(), claims.tenantID, status, search, clientID, salesStaffID, picStaffID, params)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]quotationListItemResponse, 0, len(items))
	for _, item := range items {
		clientName := strings.TrimSpace(item.ClientBride + " & " + item.ClientGroom)
		result = append(result, quotationListItemResponse{
			ID: item.Quotation.ID, ClientID: item.Quotation.ClientID, ClientName: clientName,
			PONumber: item.Quotation.PONumber, Revision: item.Quotation.Revision,
			Status: string(item.Quotation.Status), BasePrice: item.Quotation.BasePrice,
			Total:     item.Total,
			EventDate: formatDatePtr(item.Quotation.EventDate),
			ProjectID: item.ProjectID,
			SalesStaffID: item.SalesStaffID, PICStaffID: item.PICStaffID,
			IssuedAt:  formatDatePtr(item.Quotation.IssuedAt), AcceptedAt: formatDatePtr(item.Quotation.AcceptedAt),
			Signed: item.Signed,
		})
	}
	response.OKPaginated(w, "ok", result, pagination.BuildMeta(params, total))
}

type createQuotationBody struct {
	ClientID   int64   `json:"clientId"`
	EventDate  *string `json:"eventDate"`
	Pax        int     `json:"pax"`
	VenueID    *int64  `json:"venueId"`
	TemplateID int64   `json:"templateId"`
	// Kosong + templateId terisi berarti nama template yang dipakai — lihat
	// CreateQuotationInput.PackageName.
	PackageName string `json:"packageName"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, claims staffClaims) {
	var body createQuotationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	if body.ClientID == 0 {
		response.Error(w, http.StatusUnprocessableEntity, "Client wajib dipilih", map[string][]string{"clientId": {"Pilih client dari daftar"}})
		return
	}
	var eventDate *time.Time
	if body.EventDate != nil && *body.EventDate != "" {
		parsed, err := time.Parse(dateLayout, *body.EventDate)
		if err != nil {
			response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"eventDate": {"Gunakan format YYYY-MM-DD"}})
			return
		}
		eventDate = &parsed
	}
	view, err := h.quotations.Create(r.Context(), claims.tenantID, claims.staffID, application.CreateQuotationInput{
		ClientID: body.ClientID, EventDate: eventDate, Pax: body.Pax,
		VenueID: body.VenueID, TemplateID: body.TemplateID, PackageName: body.PackageName,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Penawaran berhasil dibuat", toQuotationResponse(view))
}

// Item melayani /api/v1/quotations/... — baca, sunting Draft, alur fase
// (kirim/tarik/revisi/terima/tolak/kedaluwarsa/batal), duplikat, PDF, dan
// dampak hapus (T2.7, T3.1).
//
// Satu celah sempit untuk prinsipal `client` (portal) dibuka SEBELUM gerbang
// staff — preseden: venue_handler.downloadAttachment (PLAN
// revisi-vendor-venue-portal §4.5/F3). Selain celah itu, modul ini tetap
// staff-only.
func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	if c, ok := middleware.FromContext(r.Context()); ok && c.PrincipalType == "client" {
		h.clientItem(w, r)
		return
	}
	claims, ok := requireStaff(w, r)
	if !ok {
		return
	}
	if !requireQuotationManager(w, claims.role) {
		return
	}
	segments := httpx.Segments(r.URL.Path, "/api/v1/quotations/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Penawaran tidak ditemukan", nil)
		return
	}
	// Sub-resource tingkat koleksi, bukan satu penawaran — harus ditangani
	// sebelum ParseInt, yang akan menolaknya sebagai "ID tidak valid".
	if len(segments) == 1 && segments[0] == "categories" && r.Method == http.MethodGet {
		categories, err := h.quotations.Categories(r.Context(), claims.tenantID)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if categories == nil {
			categories = []string{}
		}
		response.OK(w, "ok", categories)
		return
	}
	id, err := strconv.ParseInt(segments[0], 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID penawaran tidak valid", nil)
		return
	}
	rest := segments[1:]

	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		view, err := h.quotations.Get(r.Context(), claims.tenantID, id)
		h.writeQuotation(w, view, err)
	case len(rest) == 0 && r.Method == http.MethodPatch:
		h.updateHeader(w, r, claims, id)
	case len(rest) == 0 && r.Method == http.MethodDelete:
		h.delete(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "delete-impact" && r.Method == http.MethodGet:
		h.deleteImpact(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "blocks" && r.Method == http.MethodPut:
		h.replaceBlocks(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "adjustments" && r.Method == http.MethodPut:
		h.replaceAdjustments(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "issue" && r.Method == http.MethodPost:
		view, err := h.quotations.Issue(r.Context(), claims.tenantID, id, claims.staffID)
		h.writeQuotation(w, view, err)
	case len(rest) == 1 && rest[0] == "withdraw" && r.Method == http.MethodPost:
		view, err := h.quotations.Withdraw(r.Context(), claims.tenantID, id)
		h.writeQuotation(w, view, err)
	case len(rest) == 1 && rest[0] == "revise" && r.Method == http.MethodPost:
		view, err := h.quotations.Revise(r.Context(), claims.tenantID, id)
		h.writeQuotation(w, view, err)
	case len(rest) == 1 && rest[0] == "accept" && r.Method == http.MethodPost:
		h.accept(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "reject" && r.Method == http.MethodPost:
		view, err := h.quotations.Reject(r.Context(), claims.tenantID, id)
		h.writeQuotation(w, view, err)
	case len(rest) == 1 && rest[0] == "expire" && r.Method == http.MethodPost:
		view, err := h.quotations.Expire(r.Context(), claims.tenantID, id)
		h.writeQuotation(w, view, err)
	case len(rest) == 1 && rest[0] == "cancel" && r.Method == http.MethodPost:
		view, err := h.quotations.Cancel(r.Context(), claims.tenantID, id)
		h.writeQuotation(w, view, err)
	case len(rest) == 1 && rest[0] == "duplicate" && r.Method == http.MethodPost:
		view, err := h.quotations.Duplicate(r.Context(), claims.tenantID, id, claims.staffID)
		if err != nil {
			writeAppError(w, err)
			return
		}
		response.Created(w, "Penawaran berhasil diduplikasi", toQuotationResponse(view))
	case len(rest) == 1 && rest[0] == "pdf" && r.Method == http.MethodGet:
		h.downloadPDF(w, r, claims.tenantID, id)
	case len(rest) == 1 && rest[0] == "signature-link" && r.Method == http.MethodPost:
		h.createSignatureLink(w, r, claims, id)
	case len(rest) == 1 && rest[0] == "signature-options" && r.Method == http.MethodGet:
		h.signatureOptions(w, r, claims, id)
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

type quotationHeaderBody struct {
	BasePrice   int64  `json:"basePrice"`
	PackageName string `json:"packageName"`
	TermsText   string `json:"termsText"`
	BonusNote   string `json:"bonusNote"`
	// RawMessage, bukan *string: dengan *string, JSON `null` (yang dikirim
	// editor saat tanggal dikosongkan) tidak terbedakan dari kunci yang memang
	// tidak dikirim, sehingga tanggal lama selalu dibaca ulang dan tanggal acara
	// tidak pernah bisa dihapus.
	EventDate json.RawMessage `json:"eventDate"`
	Pax       int             `json:"pax"`
	VenueID   *int64          `json:"venueId"`
	ClientID  int64           `json:"clientId"`
}

func (h *Handler) updateHeader(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	var body quotationHeaderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	// Tiga keadaan yang berbeda, dan ketiganya harus dibedakan:
	//   kunci tidak dikirim -> pertahankan yang tersimpan
	//   null atau ""        -> kosongkan tanggalnya
	//   "YYYY-MM-DD"        -> pakai nilai itu
	var eventDate *time.Time
	switch {
	case len(body.EventDate) == 0:
		if view, err := h.quotations.Get(r.Context(), claims.tenantID, id); err == nil && view.Quotation != nil {
			eventDate = view.Quotation.EventDate
		}
	case string(body.EventDate) == "null":
		eventDate = nil
	default:
		var raw string
		if err := json.Unmarshal(body.EventDate, &raw); err != nil {
			response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"eventDate": {"Gunakan format YYYY-MM-DD"}})
			return
		}
		if raw != "" {
			parsed, err := time.Parse(dateLayout, raw)
			if err != nil {
				response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"eventDate": {"Gunakan format YYYY-MM-DD"}})
				return
			}
			eventDate = &parsed
		}
	}
	view, err := h.quotations.SetHeader(r.Context(), claims.tenantID, id, application.SetHeaderInput{
		BasePrice: body.BasePrice, PackageName: body.PackageName, TermsText: body.TermsText,
		BonusNote: body.BonusNote, EventDate: eventDate, Pax: body.Pax,
		VenueID: body.VenueID, ClientID: body.ClientID,
	})
	h.writeQuotation(w, view, err)
}

func (h *Handler) replaceBlocks(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	var body []quotationBlockBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
		return
	}
	blocks := make([]domain.QuotationBlock, 0, len(body))
	for _, b := range body {
		blocks = append(blocks, domain.QuotationBlock{
			QuotationID: id, Category: b.Category, Body: b.Body,
			QtyText: b.QtyText, BonusNote: b.BonusNote,
		})
	}
	view, err := h.quotations.ReplaceBlocks(r.Context(), claims.tenantID, id, blocks)
	h.writeQuotation(w, view, err)
}

func (h *Handler) replaceAdjustments(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	var body []quotationAdjustmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body tidak valid", nil)
		return
	}
	adjustments := make([]domain.QuotationAdjustment, 0, len(body))
	for _, a := range body {
		adjustments = append(adjustments, domain.QuotationAdjustment{
			QuotationID: id, Description: a.Description, Amount: a.Amount,
		})
	}
	view, err := h.quotations.ReplaceAdjustments(r.Context(), claims.tenantID, id, adjustments)
	h.writeQuotation(w, view, err)
}

type acceptQuotationBody struct {
	// T3.7 — dialog Tambah Project: Nama Project (prefill nama pasangan),
	// Tanggal Booking, PIC Wedding Planner (opsional), Catatan (opsional).
	// Harga venue (snapshot) dibaca frontend dari master venue.
	ProjectName      string `json:"projectName"`
	PrepStartDate    string `json:"prepStartDate"`
	PICStaffID       int64  `json:"picStaffId"`
	Notes            string `json:"notes"`
	VenueRentalPrice *int64 `json:"venueRentalPrice"`
	VenueCharge      *int64 `json:"venueCharge"`
	// Signature opsional (TTD Penawaran, D10): TTD unggahan (role +
	// base64Data) atau pakai ulang specimen (useSpecimen). Bila tidak ada
	// dan revisi yang berlaku belum berTTD, Accept menolak 422 (D9) —
	// kecuali penawarannya memang sudah diteken lewat magic link.
	Signature *acceptSignatureBody `json:"signature"`
}

// acceptSignatureBody adalah satu dari dua cara pengelola membubuhkan TTD:
// foto TTD dari klien (base64Data + mimeType) atau specimen tersimpan
// (useSpecimen — pemiliknya wajib sama dengan role, D12a).
type acceptSignatureBody struct {
	Role        string `json:"role"`
	SignerName  string `json:"signerName"`
	MimeType    string `json:"mimeType"`
	Base64Data  string `json:"base64Data"`
	UseSpecimen bool   `json:"useSpecimen"`
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	var body acceptQuotationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	if body.PrepStartDate == "" {
		response.Error(w, http.StatusUnprocessableEntity, "Tanggal Booking wajib diisi", map[string][]string{"prepStartDate": {"Isi Tanggal Booking dulu"}})
		return
	}
	prepStartDate, err := time.Parse(dateLayout, body.PrepStartDate)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format tanggal tidak valid", map[string][]string{"prepStartDate": {"Gunakan format YYYY-MM-DD"}})
		return
	}
	// TTD lebih dulu, project kemudian: SignQuotation menaruh salinan milik
	// dokumen di snapshot, Accept lalu memeriksa snapshot itu (D9). Bila
	// Accept gagal sesudahnya, penawaran berhenti di Diterima tanpa project
	// — keadaan yang sah sejak D1, bukan kerusakan (§14): percobaan
	// berikutnya melewati bagian TTD dan langsung mengulang pembuatan
	// project. TTD tidak pernah di-rollback: tanda tangan klien adalah fakta
	// yang sudah terjadi.
	if body.Signature != nil {
		if err := h.signAcceptSignature(r, claims.tenantID, id, body.Signature); err != nil {
			writeAppError(w, err)
			return
		}
	}
	view, projectID, err := h.quotations.Accept(r.Context(), claims.tenantID, id, application.AcceptQuotationInput{
		ProjectName: body.ProjectName, PrepStartDate: prepStartDate, PICStaffID: body.PICStaffID,
		Description: body.Notes, VenueRentalPrice: body.VenueRentalPrice, VenueCharge: body.VenueCharge,
		ActorStaffID: claims.staffID, ActorRole: claims.role,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Penawaran diterima — project berhasil dibuat", map[string]interface{}{
		"quotation": toQuotationResponse(view),
		"projectId": projectID,
	})
}

// signAcceptSignature membubuhkan TTD dari body accept sebelum Accept
// berjalan — satu dari dua cara pengelola (D10): foto TTD yang dikirim
// klien, atau specimen tersimpan.
func (h *Handler) signAcceptSignature(r *http.Request, tenantID, id int64, sig *acceptSignatureBody) error {
	if sig.UseSpecimen {
		_, err := h.quotations.SignFromSpecimen(r.Context(), tenantID, id, sig.Role, sig.SignerName)
		return err
	}
	img, err := base64.StdEncoding.DecodeString(sig.Base64Data)
	if err != nil {
		return apperror.Validation("Gambar TTD tidak bisa dibaca", map[string][]string{
			"signature": {"Gambar TTD tidak bisa dibaca"},
		})
	}
	_, err = h.quotations.SignQuotation(r.Context(), tenantID, id, sig.Role, sig.SignerName, img, sig.MimeType, application.SignatureChannelUpload)
	return err
}

type quotationDeleteImpactResponse struct {
	Quotation        *quotationResponse `json:"quotation"`
	ProjectID        int64              `json:"projectId"`
	ProjectName      string             `json:"projectName"`
	PaidInvoiceCount int                `json:"paidInvoiceCount"`
	PaidInvoiceTotal int64              `json:"paidInvoiceTotal"`
}

func (h *Handler) deleteImpact(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	impact, err := h.quotations.DeleteImpact(r.Context(), claims.tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	view := &application.QuotationView{Quotation: impact.Quotation}
	if full, err := h.quotations.Get(r.Context(), claims.tenantID, id); err == nil {
		view = full
	}
	response.OK(w, "ok", quotationDeleteImpactResponse{
		Quotation:        toQuotationResponse(view),
		ProjectID:        impact.ProjectID,
		ProjectName:      impact.ProjectName,
		PaidInvoiceCount: impact.PaidInvoiceCount,
		PaidInvoiceTotal: impact.PaidInvoiceTotal,
	})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	if err := h.quotations.Delete(r.Context(), claims.tenantID, id); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Penawaran berhasil dihapus permanen", nil)
}

// signerOptionResponse adalah satu pilihan Atas Nama + keterangan specimen
// tersimpan (bila ada) — dipakai endpoint staff maupun halaman publik.
type signerOptionResponse struct {
	Role string `json:"role"`
	Name string `json:"name"`
}

type signatureSpecimenResponse struct {
	Role       string `json:"role"`
	SignerName string `json:"signerName"`
	Source     string `json:"source"`
	UpdatedAt  string `json:"updatedAt"`
}

func toSignatureOptionsResponse(options *application.QuotationSignatureOptions) (opts []signerOptionResponse, specimen *signatureSpecimenResponse) {
	opts = make([]signerOptionResponse, 0, len(options.Options))
	for _, o := range options.Options {
		opts = append(opts, signerOptionResponse{Role: o[0], Name: o[1]})
	}
	if options.Specimen != nil {
		m := options.Specimen
		specimen = &signatureSpecimenResponse{Role: m[0], SignerName: m[1], Source: m[2], UpdatedAt: m[3]}
	}
	return opts, specimen
}

// createSignatureLink menerbitkan magic link 24 jam (jalur C). Link lama
// yang masih hidup ikut mati (D8) — frontend menyebutkannya di tombolnya.
func (h *Handler) createSignatureLink(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	token, expiresAt, err := h.links.Issue(r.Context(), claims.tenantID, id, claims.staffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Link tanda tangan berhasil dibuat — berlaku 24 jam", map[string]interface{}{
		"token":     token,
		"expiresAt": expiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) signatureOptions(w http.ResponseWriter, r *http.Request, claims staffClaims, id int64) {
	options, err := h.quotations.SignatureOptions(r.Context(), claims.tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	opts, specimen := toSignatureOptionsResponse(options)
	response.OK(w, "ok", map[string]interface{}{"options": opts, "specimen": specimen})
}

// quotationLedger membaca ledger hidup untuk blok "Pembayaran Diterima" —
// kosong (bukan error) selama penawaran belum punya project.
func (h *Handler) quotationLedger(w http.ResponseWriter, r *http.Request, tenantID, projectID int64) ([]projectscontracts.ClientPaymentInfo, int64, error) {
	if projectID == 0 {
		return nil, 0, nil
	}
	ledger, total, err := h.projects.ClientPaymentLedger(r.Context(), tenantID, projectID)
	if err != nil {
		writeAppError(w, err)
		return nil, 0, err
	}
	return ledger, total, nil
}

// clientItem melayani satu-satunya jalur portal client ke modul ini: GET
// /api/v1/quotations/{id}/pdf — unduh PO milik project-nya sendiri yang sudah
// Diterima (PLAN revisi-vendor-venue-portal §1.1 poin 11, PLAN revisi-vendor-
// venue-portal §4.5/F3). Selain kombinasi itu — termasuk GET dokumennya
// sendiri — menjawab 404, agar permukaan modul bagi client tidak lebih dari
// satu endpoint unduh. Otorisasi penuh ada di GetForClientContact (setiap
// kegagalan = NotFound, agar keberadaan dokumen orang lain tidak bocor).
func (h *Handler) clientItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "client" {
		response.Error(w, http.StatusForbidden, "Tidak terautentikasi", nil)
		return
	}
	tenantID, ok := claims.TenantIDInt()
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return
	}
	contactID, err := strconv.ParseInt(claims.PrincipalID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Identitas client tidak valid", nil)
		return
	}
	segments := httpx.Segments(r.URL.Path, "/api/v1/quotations/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Penawaran tidak ditemukan", nil)
		return
	}
	id, err := strconv.ParseInt(segments[0], 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID penawaran tidak valid", nil)
		return
	}
	rest := segments[1:]
	if len(rest) != 1 || rest[0] != "pdf" || r.Method != http.MethodGet {
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
		return
	}
	view, err := h.quotations.GetForClientContact(r.Context(), tenantID, contactID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	h.serveQuotationPDF(w, r, tenantID, id, view, true)
}

// downloadPDF mengunduh dokumen "PURCHASE ORDER" (D6) — blok pembayaran hidup
// hanya bila penawaran sudah punya project (pra-Accept: kosong, bukan error).
func (h *Handler) downloadPDF(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	view, err := h.quotations.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	h.serveQuotationPDF(w, r, tenantID, id, view, false)
}

// serveQuotationPDF adalah satu badan render PDF yang dipakai jalur staff
// (downloadPDF) dan jalur klien (clientItem): gerbang profil usaha,
// logo/tanda tangan tenant, ledger hidup, snapshot revisi yang berlaku, dan
// penamaan berkas. forClient=true memilih pesan netral saat profil usaha
// belum lengkap (lihat requireCompleteProfile).
func (h *Handler) serveQuotationPDF(w http.ResponseWriter, r *http.Request, tenantID, id int64, view *application.QuotationView, forClient bool) {
	profile, ok := requireCompleteProfile(h, w, r, tenantID, forClient)
	if !ok {
		return
	}
	logo, _, hasLogo, err := h.platform.GetTenantLogo(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !hasLogo {
		logo = nil
	}
	// Kolom WO pada blok tanda tangan = staff yang MENERBITKAN penawaran ini
	// (PLAN tanda-tangan-pengguna). Revisi/duplikasi memakai penerbitnya
	// sendiri, karena Duplicate mengisi CreatedByStaffID dengan actor baru.
	woSignerName, woSignerTitle, signature, signerOK, err := h.staff.GetSigner(r.Context(), tenantID, view.Quotation.CreatedByStaffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !signerOK {
		woSignerName, woSignerTitle, signature = "", "", nil
	}
	clientName := strings.TrimSpace(view.ClientBride + " & " + view.ClientGroom)
	payments, totalPaid, err := h.quotationLedger(w, r, tenantID, view.ProjectID)
	if err != nil {
		return
	}
	var eventDate time.Time
	if view.Quotation.EventDate != nil {
		eventDate = *view.Quotation.EventDate
	}
	data := buildQuotationPrintData(view, eventDate, clientName, view.ClientPhone, view.VenueName, payments, totalPaid)
	// TTD klien dibaca dari snapshot revisi yang berlaku — salinan milik
	// dokumen ini (D6c). Nama di bawah garis = penanda tangannya. Kegagalan
	// membaca gambar tidak menggagalkan pencetakan: DocumentSignatureImage
	// mengembalikan nil dan kotaknya tercetak kosong.
	if view.Quotation.Snapshot != nil && view.Quotation.Snapshot.Current.Signature != nil {
		data.ClientSignerName = view.Quotation.Snapshot.Current.Signature.SignerName
		data.ClientSignature = h.quotations.DocumentSignatureImage(r.Context(), tenantID, id)
	}

	pdf, err := buildQuotationPDF(data, eventDate, profile, logo, signature, woSignerName, woSignerTitle)
	if err != nil {
		writeAppError(w, err)
		return
	}
	// Draft belum bernomor (D7) — nama berkas jatuh ke tanggal, bukan "-.pdf".
	filename := view.Quotation.PONumber
	if filename == "" {
		filename = "Penawaran-Draft-" + time.Now().Format("20060102")
	}
	filename = strings.NewReplacer("/", "-", "\\", "-", `"`, "").Replace(filename)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.pdf"`)
	if err := pdf.Output(w); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas PDF", nil)
	}
}
