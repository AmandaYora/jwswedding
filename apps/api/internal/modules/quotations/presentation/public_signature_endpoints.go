package presentation

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"jwswedding/internal/modules/quotations/application"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/response"
)

// maxPublicBodyBytes membatasi body endpoint publik (2 MB, §9): foto dari
// kamera bisa besar sebelum dikompres; PNG hasil signature_pad hanya
// ±10–30 KB.
const maxPublicBodyBytes = 2 * 1024 * 1024

// PublicHandler melayani magic link tanda tangan TANPA login (jalur C) —
// permukaan tulis tanpa autentikasi pertama di aplikasi ini (§9). Tenant
// selalu diambil dari baris token, tidak pernah dari header atau body.
type PublicHandler struct {
	links *application.SignatureLinkService
}

func NewPublicHandler(links *application.SignatureLinkService) *PublicHandler {
	return &PublicHandler{links: links}
}

// Collection melayani /api/v1/public/quotation-signature/... — GET dokumen,
// POST .../accept, POST .../reject.
func (h *PublicHandler) Collection(w http.ResponseWriter, r *http.Request) {
	segments := httpx.Segments(r.URL.Path, "/api/v1/public/quotation-signature/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Link tidak berlaku", nil)
		return
	}
	token := segments[0]
	rest := segments[1:]
	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		h.resolve(w, r, token)
	case len(rest) == 1 && rest[0] == "accept" && r.Method == http.MethodPost:
		h.accept(w, r, token)
	case len(rest) == 1 && rest[0] == "reject" && r.Method == http.MethodPost:
		h.reject(w, r, token)
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

// resolve mengembalikan seluruh isi PURCHASE ORDER + pilihan Atas Nama untuk
// halaman publik. Tanpa sumber daya eksternal apa pun, supaya token tidak
// bocor lewat header Referer (§9).
func (h *PublicHandler) resolve(w http.ResponseWriter, r *http.Request, token string) {
	resolved, err := h.links.Resolve(r.Context(), token)
	if err != nil {
		writePublicError(w, err)
		return
	}
	opts, specimen := toSignatureOptionsResponse(resolved.Options)
	response.OK(w, "ok", map[string]interface{}{
		"quotation": toQuotationResponse(resolved.View),
		"options":   opts,
		"specimen":  specimen,
	})
}

type publicAcceptBody struct {
	Role       string `json:"role"`
	MimeType   string `json:"mimeType"`
	Base64Data string `json:"base64Data"`
}

func (h *PublicHandler) accept(w http.ResponseWriter, r *http.Request, token string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPublicBodyBytes)
	var body publicAcceptBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			response.Error(w, http.StatusRequestEntityTooLarge, "Gambar terlalu besar (maksimal 2 MB)", nil)
			return
		}
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	img, err := base64.StdEncoding.DecodeString(body.Base64Data)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Gambar tanda tangan tidak bisa dibaca", map[string][]string{
			"signature": {"Gambar tanda tangan tidak bisa dibaca"},
		})
		return
	}
	if err := h.links.AcceptViaLink(r.Context(), token, body.Role, img, body.MimeType); err != nil {
		writePublicError(w, err)
		return
	}
	response.OK(w, "Terima kasih, penawaran telah disetujui", nil)
}

func (h *PublicHandler) reject(w http.ResponseWriter, r *http.Request, token string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPublicBodyBytes)
	if err := h.links.RejectViaLink(r.Context(), token); err != nil {
		writePublicError(w, err)
		return
	}
	response.OK(w, "Penawaran telah ditolak", nil)
}

func writePublicError(w http.ResponseWriter, err error) {
	status := apperror.HTTPStatus(err)
	if appErr, ok := apperror.As(err); ok {
		if appErr.Kind == apperror.KindValidation {
			response.Error(w, status, appErr.Message, appErr.Fields)
			return
		}
		response.Error(w, status, appErr.Message, nil)
		return
	}
	response.Error(w, status, "Terjadi kesalahan pada server", nil)
}
