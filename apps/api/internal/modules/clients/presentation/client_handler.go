package presentation

import (
	"encoding/json"
	"net/http"
	"strconv"

	"jwswedding/internal/modules/clients/application"
	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/response"
)

type Handler struct {
	clients    *application.ClientService
	contacts   *application.ClientContactService
	signatures *application.ClientSignatureService
}

func NewHandler(clients *application.ClientService, contacts *application.ClientContactService, signatures *application.ClientSignatureService) *Handler {
	return &Handler{clients: clients, contacts: contacts, signatures: signatures}
}

type clientResponse struct {
	ID           int64  `json:"id"`
	BrideName    string `json:"brideName"`
	GroomName    string `json:"groomName"`
	DisplayName  string `json:"displayName"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Notes        string `json:"notes"`
	ContactCount int    `json:"contactCount"`
	ProjectCount int    `json:"projectCount"`
	// PICStaffIDs adalah himpunan Wedding Planner dari seluruh project milik
	// client ini — dasar tampilan nama WP di kartu (D7).
	PICStaffIDs []int64 `json:"picStaffIds"`
	// PICSalesStaffIDs adalah himpunan Sales dari seluruh project milik
	// client ini — dasar tampilan nama Sales di kartu (D7).
	PICSalesStaffIDs []int64 `json:"picSalesStaffIds"`
}

func toClientResponse(item application.ClientListItem) clientResponse {
	return clientResponse{
		ID: item.Client.ID, BrideName: item.Client.BrideName, GroomName: item.Client.GroomName,
		DisplayName: item.Client.DisplayName(), Phone: item.Client.Phone, Email: item.Client.Email,
		Notes: item.Client.Notes, ContactCount: item.ContactCount, ProjectCount: item.ProjectCount,
		PICStaffIDs: item.PICStaffIDs, PICSalesStaffIDs: item.PICSalesStaffIDs,
	}
}

type contactResponse struct {
	ID                    int64   `json:"id"`
	ClientID              int64   `json:"clientId"`
	Role                  string  `json:"role"`
	Username              string  `json:"username"`
	RelationNote          string  `json:"relationNote"`
	Name                  string  `json:"name"`
	Phone                 string  `json:"phone"`
	Email                 string  `json:"email"`
	IsActive              bool    `json:"isActive"`
	LastCredentialResetAt *string `json:"lastCredentialResetAt"`
}

func toContactResponse(c domain.ClientContact) contactResponse {
	var lastReset *string
	if c.LastCredentialResetAt != nil {
		s := c.LastCredentialResetAt.Format("2006-01-02")
		lastReset = &s
	}
	return contactResponse{
		ID: c.ID, ClientID: c.ClientID, Role: string(c.Role), Username: c.Username, RelationNote: c.RelationNote,
		Name: c.Name, Phone: c.Phone, Email: c.Email, IsActive: c.IsActive, LastCredentialResetAt: lastReset,
	}
}

func requireTenant(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Tidak terautentikasi", nil)
		return 0, false
	}
	tenantID, ok := claims.TenantIDInt()
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return 0, false
	}
	return tenantID, true
}

func requireStaff(w http.ResponseWriter, r *http.Request) (int64, string, bool) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" {
		response.Error(w, http.StatusForbidden, "Hanya staff WO yang dapat melakukan aksi ini", nil)
		return 0, "", false
	}
	staffID, err := strconv.ParseInt(claims.PrincipalID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Identitas staff tidak valid", nil)
		return 0, "", false
	}
	return staffID, claims.Role, true
}

// requireManagerOrClientPIC: Owner/Admin bebas; Staff/Sales hanya untuk client
// yang punya ≥1 project PIC mereka (aturan yang sama dengan gerbang lama,
// digeser dari project ke master).
func (h *Handler) requireManagerOrClientPIC(w http.ResponseWriter, r *http.Request, tenantID, clientID int64) bool {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" {
		response.Error(w, http.StatusForbidden, "Hanya staff WO yang dapat melakukan aksi ini", nil)
		return false
	}
	if claims.HasRole("Owner", "Admin") {
		return true
	}
	staffID, err := strconv.ParseInt(claims.PrincipalID, 10, 64)
	if err == nil && h.clients.VerifyClientAccess(r.Context(), tenantID, clientID, staffID, claims.Role) == nil {
		return true
	}
	response.Error(w, http.StatusForbidden, "Anda tidak memiliki akses untuk melakukan aksi ini", nil)
	return false
}

// Collection melayani /api/v1/clients — CRUD master pasangan (T1.11).
// Daftar + create: Owner/Admin/Sales (Sales membuat prospek penawaran);
// Wedding Planner ("Staff") tidak membuka menu Client.
func (h *Handler) Collection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := requireTenant(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.list(w, r, tenantID)
	case http.MethodPost:
		h.create(w, r, tenantID)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, tenantID int64) {
	if _, role, ok := requireStaff(w, r); !ok {
		return
	} else if role == "Staff" {
		response.Error(w, http.StatusForbidden, "Hanya Owner, Admin, atau Sales yang dapat mengakses menu ini", nil)
		return
	}
	params := pagination.FromRequest(r)
	search := r.URL.Query().Get("search")
	var picStaffID int64
	if raw := r.URL.Query().Get("picStaffId"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "picStaffId tidak valid", nil)
			return
		}
		picStaffID = parsed
	}
	var picSalesStaffID int64
	if raw := r.URL.Query().Get("picSalesStaffId"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "picSalesStaffId tidak valid", nil)
			return
		}
		picSalesStaffID = parsed
	}
	items, total, err := h.clients.ListPaginated(r.Context(), tenantID, params, search, picStaffID, picSalesStaffID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]clientResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toClientResponse(item))
	}
	response.OKPaginated(w, "ok", result, pagination.BuildMeta(params, total))
}

type createClientBody struct {
	BrideName string `json:"brideName"`
	GroomName string `json:"groomName"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Notes     string `json:"notes"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, tenantID int64) {
	staffID, role, ok := requireStaff(w, r)
	if !ok {
		return
	}
	if role == "Staff" {
		response.Error(w, http.StatusForbidden, "Hanya Owner, Admin, atau Sales yang dapat menambah client", nil)
		return
	}
	var body createClientBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	_ = staffID
	c, err := h.clients.Create(r.Context(), tenantID, application.CreateClientInput{
		BrideName: body.BrideName, GroomName: body.GroomName,
		Phone: body.Phone, Email: body.Email, Notes: body.Notes,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Client berhasil ditambahkan", toClientResponse(application.ClientListItem{Client: *c}))
}

// Item melayani /api/v1/clients/... — satu client, kontaknya, dan dampak
// hapusnya (T1.11, T3.5). Seluruh subtree tulis butuh Owner/Admin/Sales,
// atau Staff/Sales dalam lingkup PIC (lihat requireManagerOrClientPIC).
func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := requireTenant(w, r)
	if !ok {
		return
	}
	segments := httpx.Segments(r.URL.Path, "/api/v1/clients/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Client tidak ditemukan", nil)
		return
	}
	id, err := strconv.ParseInt(segments[0], 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	rest := segments[1:]

	// Cabang kontak di-scope ke client induknya; cabang master ke client itu
	// sendiri. Satu gerbang untuk semuanya.
	if !h.requireManagerOrClientPIC(w, r, tenantID, id) {
		return
	}

	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		h.get(w, r, tenantID, id)
	case len(rest) == 0 && r.Method == http.MethodPatch:
		h.update(w, r, tenantID, id)
	case len(rest) == 0 && r.Method == http.MethodDelete:
		h.delete(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "delete-impact" && r.Method == http.MethodGet:
		h.deleteImpact(w, r, tenantID, id)
	// TTD Penawaran (D, D12): specimen tunggal — tanpa segmen {role}, karena
	// hasilnya selalu nol atau satu baris.
	case len(rest) == 1 && rest[0] == "signature" && r.Method == http.MethodGet:
		h.getSignature(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "signature" && r.Method == http.MethodDelete:
		h.deleteSignature(w, r, tenantID, id)
	case len(rest) == 2 && rest[0] == "signature" && rest[1] == "image" && r.Method == http.MethodGet:
		h.signatureImage(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "contacts" && r.Method == http.MethodGet:
		h.listContacts(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "contacts" && r.Method == http.MethodPost:
		h.createContact(w, r, tenantID, id)
	case len(rest) == 2 && rest[0] == "contacts" && r.Method == http.MethodPatch:
		h.updateContact(w, r, tenantID, id, rest[1])
	case len(rest) == 2 && rest[0] == "contacts" && r.Method == http.MethodDelete:
		h.deleteContact(w, r, tenantID, id, rest[1])
	case len(rest) == 3 && rest[0] == "contacts" && rest[2] == "toggle-active" && r.Method == http.MethodPost:
		h.toggleContactActive(w, r, tenantID, id, rest[1])
	case len(rest) == 3 && rest[0] == "contacts" && rest[2] == "reset-credential" && r.Method == http.MethodPost:
		h.resetContactCredential(w, r, tenantID, id, rest[1])
	case len(rest) == 3 && rest[0] == "contacts" && rest[2] == "replace-representative" && r.Method == http.MethodPost:
		h.replaceRepresentative(w, r, tenantID, id, rest[1])
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	c, err := h.clients.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	contacts, err := h.contacts.ListByClient(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	contactResps := make([]contactResponse, 0, len(contacts))
	for _, contact := range contacts {
		contactResps = append(contactResps, toContactResponse(contact))
	}
	response.OK(w, "ok", map[string]interface{}{"client": toClientResponse(application.ClientListItem{Client: *c, ContactCount: len(contacts)}), "contacts": contactResps})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var body createClientBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	c, err := h.clients.Update(r.Context(), tenantID, id, application.UpdateClientInput{
		BrideName: body.BrideName, GroomName: body.GroomName,
		Phone: body.Phone, Email: body.Email, Notes: body.Notes,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Client berhasil diperbarui", toClientResponse(application.ClientListItem{Client: *c}))
}

type deleteImpactResponse struct {
	Client           clientResponse `json:"client"`
	ContactCount     int            `json:"contactCount"`
	ProjectCount     int            `json:"projectCount"`
	ProjectNames     []string       `json:"projectNames"`
	PaidInvoiceCount int            `json:"paidInvoiceCount"`
	PaidInvoiceTotal int64          `json:"paidInvoiceTotal"`
	QuotationCount   int            `json:"quotationCount"`
	QuotationNumbers []string       `json:"quotationNumbers"`
}

// deleteImpact WAJIB dipanggil sebelum dialog hapus dirender (D14, T3.5) —
// dialog menolak tampil kalau panggilan ini gagal, supaya tidak pernah ada
// konfirmasi yang berbohong.
func (h *Handler) deleteImpact(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	impact, err := h.clients.DeleteImpact(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", deleteImpactResponse{
		Client:           toClientResponse(application.ClientListItem{Client: impact.Client, ContactCount: impact.ContactCount, ProjectCount: impact.ProjectCount}),
		ContactCount:     impact.ContactCount,
		ProjectCount:     impact.ProjectCount,
		ProjectNames:     impact.ProjectNames,
		PaidInvoiceCount: impact.PaidInvoiceCount,
		PaidInvoiceTotal: impact.PaidInvoiceTotal,
		QuotationCount:   impact.QuotationCount,
		QuotationNumbers: impact.QuotationNumbers,
	})
}

// delete selalu tersedia tanpa pengecualian pemblokir (D14) — selalu lewat
// dialog konfirmasi yang datanya berasal dari delete-impact di atas.
func (h *Handler) delete(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	if err := h.clients.Delete(r.Context(), tenantID, id); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Client berhasil dihapus permanen", nil)
}

func (h *Handler) listContacts(w http.ResponseWriter, r *http.Request, tenantID, clientID int64) {
	list, err := h.contacts.ListByClient(r.Context(), tenantID, clientID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]contactResponse, 0, len(list))
	for _, c := range list {
		result = append(result, toContactResponse(c))
	}
	response.OK(w, "ok", result)
}

type createContactBody struct {
	Role         string `json:"role"`
	RelationNote string `json:"relationNote"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Password     string `json:"password"`
}

func (h *Handler) createContact(w http.ResponseWriter, r *http.Request, tenantID, clientID int64) {
	var body createContactBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	c, err := h.contacts.Create(r.Context(), tenantID, application.CreateContactInput{
		ClientID: clientID, Role: domain.ClientRole(body.Role), RelationNote: body.RelationNote,
		Name: body.Name, Phone: body.Phone, Username: body.Username, Email: body.Email, Password: body.Password,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Kontak berhasil ditambahkan", toContactResponse(*c))
}

func (h *Handler) parseContactID(w http.ResponseWriter, raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID kontak tidak valid", nil)
		return 0, false
	}
	return id, true
}

// requireContactOfClient menolak kontak yang bukan milik parent client di
// path — tanpa ini /clients/1/contacts/999 bisa menyentuh kontak milik
// client 2 selama penelepon punya akses ke client 1.
func (h *Handler) requireContactOfClient(w http.ResponseWriter, r *http.Request, tenantID, clientID int64, raw string) (int64, bool) {
	id, ok := h.parseContactID(w, raw)
	if !ok {
		return 0, false
	}
	c, err := h.contacts.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return 0, false
	}
	if c.ClientID != clientID {
		response.Error(w, http.StatusNotFound, "Kontak tidak ditemukan", nil)
		return 0, false
	}
	return id, true
}

type contactBody struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

func (h *Handler) updateContact(w http.ResponseWriter, r *http.Request, tenantID, clientID int64, raw string) {
	id, ok := h.requireContactOfClient(w, r, tenantID, clientID, raw)
	if !ok {
		return
	}
	var body contactBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	c, err := h.contacts.UpdateContact(r.Context(), tenantID, id, application.UpdateContactInput{Name: body.Name, Phone: body.Phone, Email: body.Email})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Kontak client berhasil diperbarui", toContactResponse(*c))
}

func (h *Handler) deleteContact(w http.ResponseWriter, r *http.Request, tenantID, clientID int64, raw string) {
	id, ok := h.requireContactOfClient(w, r, tenantID, clientID, raw)
	if !ok {
		return
	}
	if err := h.contacts.Delete(r.Context(), tenantID, id); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Kontak berhasil dihapus", nil)
}

func (h *Handler) toggleContactActive(w http.ResponseWriter, r *http.Request, tenantID, clientID int64, raw string) {
	id, ok := h.requireContactOfClient(w, r, tenantID, clientID, raw)
	if !ok {
		return
	}
	current, err := h.contacts.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	c, err := h.contacts.SetActive(r.Context(), tenantID, id, !current.IsActive)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Status kontak diperbarui", toContactResponse(*c))
}

type resetCredentialBody struct {
	Password string `json:"password"`
}

func (h *Handler) resetContactCredential(w http.ResponseWriter, r *http.Request, tenantID, clientID int64, raw string) {
	id, ok := h.requireContactOfClient(w, r, tenantID, clientID, raw)
	if !ok {
		return
	}
	var body resetCredentialBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	c, err := h.contacts.ResetCredential(r.Context(), tenantID, id, body.Password)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Kredensial kontak berhasil direset", toContactResponse(*c))
}

type replaceRepresentativeBody struct {
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	RelationNote string `json:"relationNote"`
}

func (h *Handler) replaceRepresentative(w http.ResponseWriter, r *http.Request, tenantID, clientID int64, raw string) {
	id, ok := h.requireContactOfClient(w, r, tenantID, clientID, raw)
	if !ok {
		return
	}
	var body replaceRepresentativeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	c, err := h.contacts.ReplaceRepresentative(r.Context(), tenantID, id,
		application.UpdateContactInput{Name: body.Name, Phone: body.Phone, Email: body.Email}, body.RelationNote)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Perwakilan keluarga berhasil diganti", toContactResponse(*c))
}

// clientSignatureResponse adalah pratinjau specimen tunggal (D12): milik
// siapa (role + nama), kapan diperbarui, dan dari jalur mana. Gambarnya
// diambil lewat .../signature/image; storage_key tidak pernah keluar.
type clientSignatureResponse struct {
	Role       string `json:"role"`
	SignerName string `json:"signerName"`
	Source     string `json:"source"`
	UpdatedAt  string `json:"updatedAt"`
}

func (h *Handler) getSignature(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	spec, err := h.signatures.Specimen(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if spec == nil {
		response.OK(w, "ok", nil)
		return
	}
	response.OK(w, "ok", clientSignatureResponse{
		Role: string(spec.Role), SignerName: spec.SignerName,
		Source: string(spec.Source), UpdatedAt: spec.UpdatedAt.Format("2006-01-02"),
	})
}

func (h *Handler) signatureImage(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	data, err := h.signatures.SpecimenImage(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Content-Disposition", `inline; filename="specimen.png"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) deleteSignature(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	if err := h.signatures.DeleteSpecimen(r.Context(), tenantID, id); err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "TTD tersimpan berhasil dihapus", nil)
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
