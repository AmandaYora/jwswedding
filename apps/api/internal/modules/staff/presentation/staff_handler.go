package presentation

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"jwswedding/internal/modules/staff/application"
	"jwswedding/internal/modules/staff/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/response"
)

type Handler struct {
	staff      *application.StaffService
	signatures *application.StaffSignatureService
}

func NewHandler(staff *application.StaffService, signatures *application.StaffSignatureService) *Handler {
	return &Handler{staff: staff, signatures: signatures}
}

type staffResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Initials string `json:"initials"`
	Role     string `json:"role"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	IsActive bool   `json:"isActive"`
	// HasSignature hanya menyatakan ADA/TIDAK — kunci object storage-nya tidak
	// pernah keluar lewat API. Gambarnya diambil terpisah lewat
	// .../signature/image, supaya daftar Pengguna tidak menarik satu gambar
	// per baris (PLAN tanda-tangan-pengguna A7).
	HasSignature bool `json:"hasSignature"`
}

func toStaffResponse(m domain.StaffMember) staffResponse {
	return staffResponse{
		ID: m.ID, Name: m.Name, Title: m.Title, Initials: m.Initials,
		Role: string(m.Role), Username: m.Username, Email: m.Email, Phone: m.Phone, IsActive: m.IsActive,
		HasSignature: m.SignatureStoragePath != nil,
	}
}

// requireOwnerTenant gates the entire Pengguna (staff directory) surface to
// Owner only — confirmed business rule: Admin has no access to User
// management at all, not even to view the list or edit their own row via
// this endpoint (Owner-editing-Owner's own row, per StaffService.Update's
// isSelf check, still works fine since only Owner ever reaches this handler
// at all now).
func requireOwnerTenant(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" || !claims.HasRole("Owner") {
		response.Error(w, http.StatusForbidden, "Hanya Owner yang dapat mengakses pengelolaan pengguna", nil)
		return 0, false
	}
	tenantID, ok := claims.TenantIDInt()
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return 0, false
	}
	return tenantID, true
}

// requireTenant is the plain "any authenticated staff" gate — used only by
// Summary below, since PIC-name resolution (project PIC pickers, milestone/
// vendor-engagement/issue/activity "assigned to" labels throughout the
// `projects` module) is needed by every role, not just Owner.
func requireTenant(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" {
		response.Error(w, http.StatusForbidden, "Hanya staff WO yang dapat mengakses endpoint ini", nil)
		return 0, false
	}
	tenantID, ok := claims.TenantIDInt()
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return 0, false
	}
	return tenantID, true
}

type staffSummaryResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Role  string `json:"role"`
}

// Summary backs every PIC picker/label across the `projects` module —
// deliberately excludes username/email/phone/isActive (Pengguna's
// management-only fields), reachable by any staff role regardless of the
// Owner-only gate on the rest of this module (see requireOwnerTenant).
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := requireTenant(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
		return
	}
	members, err := h.staff.List(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]staffSummaryResponse, 0, len(members))
	for _, m := range members {
		result = append(result, staffSummaryResponse{ID: m.ID, Name: m.Name, Title: m.Title, Role: string(m.Role)})
	}
	response.OK(w, "ok", result)
}

func (h *Handler) Collection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := requireOwnerTenant(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("all") == "true" {
			members, err := h.staff.List(r.Context(), tenantID)
			if err != nil {
				writeAppError(w, err)
				return
			}
			result := make([]staffResponse, 0, len(members))
			for _, m := range members {
				result = append(result, toStaffResponse(m))
			}
			response.OK(w, "ok", result)
			return
		}
		params := pagination.FromRequest(r)
		search := r.URL.Query().Get("search")
		role := r.URL.Query().Get("role")
		members, total, err := h.staff.ListPaginated(r.Context(), tenantID, params, search, role)
		if err != nil {
			writeAppError(w, err)
			return
		}
		result := make([]staffResponse, 0, len(members))
		for _, m := range members {
			result = append(result, toStaffResponse(m))
		}
		response.OKPaginated(w, "ok", result, pagination.BuildMeta(params, total))
	case http.MethodPost:
		h.create(w, r, tenantID)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
	}
}

type staffInputBody struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Role     string `json:"role"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, tenantID int64) {
	var body staffInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	member, err := h.staff.Create(r.Context(), tenantID, application.StaffInput{
		Name: body.Name, Title: body.Title, Role: domain.StaffRole(body.Role),
		Username: body.Username, Password: body.Password, Email: body.Email, Phone: body.Phone,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Pengguna berhasil ditambahkan", toStaffResponse(*member))
}

func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	segments := httpx.Segments(r.URL.Path, "/api/v1/staff/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Pengguna tidak ditemukan", nil)
		return
	}

	// Jalur "me" dicabang SEBELUM gerbang Owner-only dan sebelum parsing {id}:
	// setiap pengguna berhak mengurus TTD-nya sendiri, sementara sisa modul ini
	// tetap Owner-only (PLAN tanda-tangan-pengguna K7).
	if segments[0] == "me" {
		h.mySignatureRoutes(w, r, segments)
		return
	}

	tenantID, ok := requireOwnerTenant(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(segments[0], 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}

	switch {
	case len(segments) == 1 && r.Method == http.MethodPatch:
		h.update(w, r, tenantID, id)
	case len(segments) == 2 && segments[1] == "toggle-active" && r.Method == http.MethodPost:
		h.toggleActive(w, r, tenantID, id)
	case len(segments) == 2 && segments[1] == "delete-impact" && r.Method == http.MethodGet:
		h.deleteImpact(w, r, tenantID, id)
	case len(segments) == 2 && segments[1] == "signature" && r.Method == http.MethodPut:
		h.putSignature(w, r, tenantID, id)
	case len(segments) == 2 && segments[1] == "signature" && r.Method == http.MethodDelete:
		h.deleteSignature(w, r, tenantID, id)
	case len(segments) == 3 && segments[1] == "signature" && segments[2] == "image" && r.Method == http.MethodGet:
		h.getSignatureImage(w, r, tenantID, id)
	case len(segments) == 1 && r.Method == http.MethodDelete:
		if err := h.staff.Delete(r.Context(), tenantID, id); err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "Pengguna berhasil dihapus", nil)
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

// mySignatureRoutes melayani /api/v1/staff/me/signature[/image] — TTD milik
// pemanggil sendiri, terbuka untuk semua role staff.
//
// staffID SELALU diambil dari klaim JWT, tidak pernah dari URL: itulah yang
// membuat jalur ini tidak bisa dipakai membaca atau menimpa TTD orang lain.
func (h *Handler) mySignatureRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	tenantID, ok := requireTenant(w, r)
	if !ok {
		return
	}
	claims, _ := middleware.FromContext(r.Context())
	if claims == nil {
		response.Error(w, http.StatusForbidden, "Hanya staff WO yang dapat mengakses endpoint ini", nil)
		return
	}
	staffID, err := strconv.ParseInt(claims.PrincipalID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke pengguna manapun", nil)
		return
	}

	switch {
	case len(segments) == 2 && segments[1] == "signature" && r.Method == http.MethodGet:
		h.getOwnSignatureMeta(w, r, tenantID, staffID)
	case len(segments) == 2 && segments[1] == "signature" && r.Method == http.MethodPut:
		h.putSignature(w, r, tenantID, staffID)
	case len(segments) == 2 && segments[1] == "signature" && r.Method == http.MethodDelete:
		h.deleteSignature(w, r, tenantID, staffID)
	case len(segments) == 3 && segments[1] == "signature" && segments[2] == "image" && r.Method == http.MethodGet:
		h.getSignatureImage(w, r, tenantID, staffID)
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

type staffSignatureUploadBody struct {
	FileName   string `json:"fileName"`
	MimeType   string `json:"mimeType"`
	Base64Data string `json:"base64Data"`
}

type staffSignatureMetaResponse struct {
	HasSignature bool `json:"hasSignature"`
}

// getOwnSignatureMeta memberi halaman "Tanda Tangan Saya" cukup informasi untuk
// memutuskan menampilkan pratinjau atau keadaan kosong, tanpa menarik gambarnya.
func (h *Handler) getOwnSignatureMeta(w http.ResponseWriter, r *http.Request, tenantID, staffID int64) {
	member, err := h.staff.Get(r.Context(), tenantID, staffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", staffSignatureMetaResponse{HasSignature: member.SignatureStoragePath != nil})
}

func (h *Handler) putSignature(w http.ResponseWriter, r *http.Request, tenantID, staffID int64) {
	var body staffSignatureUploadBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	img, err := base64.StdEncoding.DecodeString(body.Base64Data)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Data gambar tidak valid", map[string][]string{
			"base64Data": {"Gagal membaca data gambar tanda tangan"},
		})
		return
	}
	if err := h.signatures.SaveSignature(r.Context(), tenantID, staffID, img, body.MimeType); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tanda tangan berhasil disimpan", staffSignatureMetaResponse{HasSignature: true})
}

func (h *Handler) getSignatureImage(w http.ResponseWriter, r *http.Request, tenantID, staffID int64) {
	data, contentType, ok, err := h.signatures.SignatureImage(r.Context(), tenantID, staffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !ok {
		response.Error(w, http.StatusNotFound, "Pengguna ini belum memiliki tanda tangan", nil)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `inline; filename="signature.png"`)
	// Aset privat per pengguna — jangan sampai tersimpan di cache bersama.
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) deleteSignature(w http.ResponseWriter, r *http.Request, tenantID, staffID int64) {
	if err := h.signatures.DeleteSignature(r.Context(), tenantID, staffID); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Tanda tangan berhasil dihapus", staffSignatureMetaResponse{HasSignature: false})
}

type staffDeleteImpactResponse struct {
	AffectedProjects []staffDeleteImpactProjectRef `json:"affectedProjects"`
}

type staffDeleteImpactProjectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// deleteImpact backs the hard-delete confirmation dialog (PLAN.md) -- names
// exactly which projects still have this staff member as a live PIC
// assignment.
func (h *Handler) deleteImpact(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	if _, err := h.staff.Get(r.Context(), tenantID, id); err != nil {
		writeAppError(w, err)
		return
	}
	refs, err := h.staff.ListAffectedProjects(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]staffDeleteImpactProjectRef, 0, len(refs))
	for _, ref := range refs {
		result = append(result, staffDeleteImpactProjectRef{ID: strconv.FormatInt(ref.ID, 10), Name: ref.Name})
	}
	response.OK(w, "ok", staffDeleteImpactResponse{AffectedProjects: result})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var body staffInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	claims, _ := middleware.FromContext(r.Context())
	isSelf := claims != nil && claims.PrincipalID == strconv.FormatInt(id, 10)
	member, err := h.staff.Update(r.Context(), tenantID, id, isSelf, application.StaffInput{
		Name: body.Name, Title: body.Title, Role: domain.StaffRole(body.Role), Email: body.Email, Phone: body.Phone,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Pengguna berhasil diperbarui", toStaffResponse(*member))
}

func (h *Handler) toggleActive(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	current, err := h.staff.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	member, err := h.staff.SetActive(r.Context(), tenantID, id, !current.IsActive)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Status pengguna diperbarui", toStaffResponse(*member))
}

func writeAppError(w http.ResponseWriter, err error) {
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
