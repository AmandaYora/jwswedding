package presentation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/vendors/application"
	"jwswedding/internal/modules/vendors/domain"
	"jwswedding/internal/shared/httpx"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/response"
)

// requireStaffTenant is requireTenant's stricter sibling: `client` principals
// are otherwise tenant-scoped just like staff (requireTenant lets both
// through), but venue records carry commercial data (rental price, charge,
// PIC contact) never meant for a client to read directly — see ADR-0016's
// Client Portal field-visibility decision. The attachment GET route
// deliberately stays on requireTenant below (see downloadAttachment) since
// it's the one piece of content ADR-0016 says clients should conditionally see.
func requireStaffTenant(w http.ResponseWriter, r *http.Request) (int64, bool) {
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

// requireManagerRole is the extra gate every Vendor/Venue **write** action
// needs on top of requireStaffTenant — confirmed role rule: a Wedding
// Planner keeps read access (list/get/attachment-download) so the vendor-
// and venue-pickers inside their own project keep working, but every
// mutation (create/update/toggle-active/upload/template/import) is
// Owner/Admin only. Assumes requireStaffTenant already ran and succeeded
// (so claims is guaranteed present) — called only from write branches below.
func requireManagerRole(w http.ResponseWriter, r *http.Request) bool {
	claims, _ := middleware.FromContext(r.Context())
	if !claims.HasRole("Owner", "Admin") {
		response.Error(w, http.StatusForbidden, "Hanya Owner atau Admin yang dapat melakukan aksi ini", nil)
		return false
	}
	return true
}

type VenueHandler struct {
	venues   *application.VenueService
	projects projectscontracts.Contracts
}

func NewVenueHandler(venues *application.VenueService, projects projectscontracts.Contracts) *VenueHandler {
	return &VenueHandler{venues: venues, projects: projects}
}

type venueResponse struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	PICName           string   `json:"picName"`
	PhonePIC          string   `json:"phonePic"`
	PhoneVenue        *string  `json:"phoneVenue"`
	Email             *string  `json:"email"`
	Address           *string  `json:"address"`
	City              *string  `json:"city"`
	Categories        []string `json:"categories"`
	PriceTier         string   `json:"priceTier"`
	RentalPrice       *int64   `json:"rentalPrice"`
	Charge            *int64   `json:"charge"`
	Capacity          *int     `json:"capacity"`
	Facilities        *string  `json:"facilities"`
	SocialMedia       *string  `json:"socialMedia"`
	Notes             string   `json:"notes"`
	HasAttachment     bool     `json:"hasAttachment"`
	AttachmentIsImage bool     `json:"attachmentIsImage"`
	IsActive          bool     `json:"isActive"`
	CreatedAt         string   `json:"createdAt"`
}

func toVenueResponse(v domain.Venue) venueResponse {
	attachmentIsImage := v.AttachmentMimeType != nil && strings.HasPrefix(*v.AttachmentMimeType, "image/")
	categories := v.Categories
	if categories == nil {
		categories = []string{}
	}
	return venueResponse{
		ID: v.ID, Name: v.Name, PICName: v.PICName, PhonePIC: v.PhonePIC, PhoneVenue: v.PhoneVenue,
		Email: v.Email, Address: v.Address, City: v.City, Categories: categories,
		PriceTier: domain.VenuePriceTierFor(v.RentalPrice),
		RentalPrice: v.RentalPrice, Charge: v.Charge,
		Capacity: v.Capacity, Facilities: v.Facilities, SocialMedia: v.SocialMedia, Notes: v.Notes,
		HasAttachment: v.AttachmentPath != nil, AttachmentIsImage: attachmentIsImage, IsActive: v.IsActive,
		CreatedAt: v.CreatedAt.Format("2006-01-02"),
	}
}

// parseVenueListFilter reads the GET /venues list filters (search, city,
// category, priceTier, capacityMin) into one VenueListFilter. Unknown
// category/tier or a non-integer capacityMin → 400; empty means "no filter"
// and is never an error. A mistyped param is never silently ignored: that
// would display the full list as if it were the filtered result.
func parseVenueListFilter(w http.ResponseWriter, r *http.Request) (application.VenueListFilter, bool) {
	filter := application.VenueListFilter{
		Search:   r.URL.Query().Get("search"),
		City:     r.URL.Query().Get("city"),
		Category: r.URL.Query().Get("category"),
		PriceTier: r.URL.Query().Get("priceTier"),
	}
	if filter.Category != "" && !domain.IsValidVenueCategory(filter.Category) {
		response.Error(w, http.StatusBadRequest, "category tidak valid", nil)
		return filter, false
	}
	if filter.PriceTier != "" && !domain.IsValidVenuePriceTier(filter.PriceTier) {
		response.Error(w, http.StatusBadRequest, "priceTier tidak valid", nil)
		return filter, false
	}
	if raw := r.URL.Query().Get("capacityMin"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			response.Error(w, http.StatusBadRequest, "capacityMin tidak valid", nil)
			return filter, false
		}
		filter.CapacityMin = &n
	}
	return filter, true
}

func (h *VenueHandler) Collection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := requireStaffTenant(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("all") == "true" {
			venues, err := h.venues.List(r.Context(), tenantID)
			if err != nil {
				writeAppError(w, err)
				return
			}
			result := make([]venueResponse, 0, len(venues))
			for _, v := range venues {
				result = append(result, toVenueResponse(v))
			}
			response.OK(w, "ok", result)
			return
		}
		params := pagination.FromRequest(r)
		filter, ok := parseVenueListFilter(w, r)
		if !ok {
			return
		}
		venues, total, err := h.venues.ListPaginated(r.Context(), tenantID, filter, params)
		if err != nil {
			writeAppError(w, err)
			return
		}
		result := make([]venueResponse, 0, len(venues))
		for _, v := range venues {
			result = append(result, toVenueResponse(v))
		}
		response.OKPaginated(w, "ok", result, pagination.BuildMeta(params, total))
	case http.MethodPost:
		if !requireManagerRole(w, r) {
			return
		}
		h.create(w, r, tenantID)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
	}
}

// Export streams every venue matching the same filters Collection's list view
// accepts (search/city/category/priceTier/capacityMin) as an .xlsx workbook,
// unpaginated -- mirrors Vendor's Export exactly. Column order matches
// venueTemplateHeaders plus an appended Status column, so the file
// round-trips through Import unmodified.
func (h *VenueHandler) Export(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := requireStaffTenant(w, r)
	if !ok {
		return
	}
	if !requireManagerRole(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
		return
	}

	filter, ok := parseVenueListFilter(w, r)
	if !ok {
		return
	}
	venues, err := h.venues.Export(r.Context(), tenantID, filter)
	if err != nil {
		writeAppError(w, err)
		return
	}

	f := buildVenueExportWorkbook(venues)
	defer f.Close()
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="venues-export.xlsx"`)
	if _, err := f.WriteTo(w); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas export", nil)
	}
}

func intPtrValue(n *int) interface{} {
	if n == nil {
		return ""
	}
	return *n
}

func buildVenueExportWorkbook(venues []domain.Venue) *excelize.File {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := append(append([]string{}, venueTemplateHeaders...), "Status")
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, header)
	}
	for rowIdx, v := range venues {
		row := rowIdx + 2
		values := []interface{}{
			v.Name, v.PICName, v.PhonePIC, stringPtrValue(v.PhoneVenue), stringPtrValue(v.Email),
			stringPtrValue(v.Address), stringPtrValue(v.City), strings.Join(v.Categories, ", "),
			int64PtrValue(v.RentalPrice),
			int64PtrValue(v.Charge), intPtrValue(v.Capacity), stringPtrValue(v.Facilities),
			stringPtrValue(v.SocialMedia), v.Notes, activeStatusLabel(v.IsActive),
		}
		for i, val := range values {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			f.SetCellValue(sheet, cell, val)
		}
	}
	return f
}

type venueInputBody struct {
	Name        string   `json:"name"`
	PICName     string   `json:"picName"`
	PhonePIC    string   `json:"phonePic"`
	PhoneVenue  string   `json:"phoneVenue"`
	Email       string   `json:"email"`
	Address     string   `json:"address"`
	City        string   `json:"city"`
	Categories  []string `json:"categories"`
	RentalPrice *int64   `json:"rentalPrice"`
	Charge      *int64   `json:"charge"`
	Capacity    *int     `json:"capacity"`
	Facilities  string   `json:"facilities"`
	SocialMedia string   `json:"socialMedia"`
	Notes       string   `json:"notes"`
}

func toVenueInput(body venueInputBody) application.VenueInput {
	return application.VenueInput{
		Name: body.Name, PICName: body.PICName, PhonePIC: body.PhonePIC, PhoneVenue: body.PhoneVenue,
		Email: body.Email, Address: body.Address, City: body.City, Categories: body.Categories,
		RentalPrice: body.RentalPrice,
		Charge: body.Charge, Capacity: body.Capacity, Facilities: body.Facilities, SocialMedia: body.SocialMedia,
		Notes: body.Notes,
	}
}

func (h *VenueHandler) create(w http.ResponseWriter, r *http.Request, tenantID int64) {
	var body venueInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	venue, err := h.venues.Create(r.Context(), tenantID, toVenueInput(body))
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.Created(w, "Venue berhasil ditambahkan", toVenueResponse(*venue))
}

func (h *VenueHandler) Item(w http.ResponseWriter, r *http.Request) {
	segments := httpx.Segments(r.URL.Path, "/api/v1/venues/")
	if len(segments) == 0 {
		response.Error(w, http.StatusNotFound, "Venue tidak ditemukan", nil)
		return
	}
	id, err := strconv.ParseInt(segments[0], 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID tidak valid", nil)
		return
	}
	rest := segments[1:]

	// GET .../attachment is the one route reachable by a `client` principal
	// (requireTenant, not requireStaffTenant) -- downloadAttachment itself
	// enforces the mime-type gate below, so a client can't fetch a
	// document-type attachment even by calling this endpoint directly.
	var tenantID int64
	var ok bool
	if len(rest) == 1 && rest[0] == "attachment" && r.Method == http.MethodGet {
		tenantID, ok = requireTenant(w, r)
	} else {
		tenantID, ok = requireStaffTenant(w, r)
	}
	if !ok {
		return
	}

	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		h.get(w, r, tenantID, id)
	case len(rest) == 0 && r.Method == http.MethodPatch:
		if !requireManagerRole(w, r) {
			return
		}
		h.update(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "toggle-active" && r.Method == http.MethodPost:
		if !requireManagerRole(w, r) {
			return
		}
		h.toggleActive(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "attachment" && r.Method == http.MethodPut:
		if !requireManagerRole(w, r) {
			return
		}
		h.uploadAttachment(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "attachment" && r.Method == http.MethodGet:
		h.downloadAttachment(w, r, tenantID, id)
	case len(rest) == 1 && rest[0] == "delete-impact" && r.Method == http.MethodGet:
		if !requireOwnerRole(w, r) {
			return
		}
		h.deleteImpact(w, r, tenantID, id)
	case len(rest) == 0 && r.Method == http.MethodDelete:
		if !requireOwnerRole(w, r) {
			return
		}
		if err := h.venues.Delete(r.Context(), tenantID, id); err != nil {
			writeAppError(w, err)
			return
		}
		response.OK(w, "Venue berhasil dihapus", nil)
	default:
		response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
	}
}

// deleteImpact backs the hard-delete confirmation dialog (PLAN.md) -- names
// exactly which projects would lose this venue's data source.
func (h *VenueHandler) deleteImpact(w http.ResponseWriter, r *http.Request, tenantID, venueID int64) {
	if _, err := h.venues.Get(r.Context(), tenantID, venueID); err != nil {
		writeAppError(w, err)
		return
	}
	refs, err := h.projects.ListAffectedProjectsForVenue(r.Context(), tenantID, venueID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]deleteImpactProjectRef, 0, len(refs))
	for _, ref := range refs {
		result = append(result, deleteImpactProjectRef{ID: strconv.FormatInt(ref.ID, 10), Name: ref.Name})
	}
	response.OK(w, "ok", deleteImpactResponse{AffectedProjects: result})
}

func (h *VenueHandler) get(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	venue, err := h.venues.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toVenueResponse(*venue))
}

func (h *VenueHandler) update(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var body venueInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	venue, err := h.venues.Update(r.Context(), tenantID, id, toVenueInput(body))
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Venue berhasil diperbarui", toVenueResponse(*venue))
}

func (h *VenueHandler) toggleActive(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	current, err := h.venues.Get(r.Context(), tenantID, id)
	if err != nil {
		writeAppError(w, err)
		return
	}
	venue, err := h.venues.SetActive(r.Context(), tenantID, id, !current.IsActive)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Status venue diperbarui", toVenueResponse(*venue))
}

type venueAttachmentUploadBody struct {
	FileName   string `json:"fileName"`
	MimeType   string `json:"mimeType"`
	Base64Data string `json:"base64Data"`
}

func (h *VenueHandler) uploadAttachment(w http.ResponseWriter, r *http.Request, tenantID, venueID int64) {
	var body venueAttachmentUploadBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	venue, err := h.venues.UploadAttachment(r.Context(), tenantID, venueID, application.UploadVenueAttachmentInput{
		FileName: body.FileName, MimeType: body.MimeType, Base64Data: body.Base64Data,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Lampiran venue berhasil diunggah", toVenueResponse(*venue))
}

func (h *VenueHandler) downloadAttachment(w http.ResponseWriter, r *http.Request, tenantID, venueID int64) {
	venue, reader, err := h.venues.DownloadAttachment(r.Context(), tenantID, venueID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	defer reader.Close()

	// requireTenant let both `staff` and `client` principals reach this
	// handler -- the mime-type gate here is the real boundary for `client`
	// (ADR-0016): a document-type attachment is never served to a client,
	// even though they know a valid venue ID, only an image is.
	if claims, ok := middleware.FromContext(r.Context()); ok && claims.PrincipalType == "client" {
		isImage := venue.AttachmentMimeType != nil && strings.HasPrefix(*venue.AttachmentMimeType, "image/")
		if !isImage {
			response.Error(w, http.StatusForbidden, "Lampiran ini tidak dapat diakses", nil)
			return
		}
	}

	w.Header().Set("Content-Disposition", `inline; filename="attachment"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

// --- Bulk import (Excel) ---

// venueTemplateHeaders is both the template's header row (Template) and the
// authoritative column order the import parser (Import) reads by index --
// keeping them as one slice means the two can never drift apart. Never edit
// these strings themselves; templateHeaderLabel appends the required-column
// marker separately when writing the printed header cell.
var venueTemplateHeaders = []string{
	"Nama Venue", "Nama PIC", "No Tlp PIC", "No Tlp Venue", "Email",
	"Alamat", "Kota", "Kategori", "Harga Sewa", "Charge", "Kapasitas", "Fasilitas", "Sosial Media", "Catatan",
}

// venueRequiredImportHeaders/venueTextImportHeaders feed styleTemplateSheet's
// templateSheetSpec -- see vendorRequiredImportHeaders/vendorTextImportHeaders
// in vendor_handler.go for the same pattern. Both phone columns are locked
// to Text format since either can be typed as an all-digit cell.
var (
	venueRequiredImportHeaders = []string{"Nama Venue", "Nama PIC", "No Tlp PIC", "Kota"}
	venueTextImportHeaders     = []string{"No Tlp PIC", "No Tlp Venue"}
)

func (h *VenueHandler) Template(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireStaffTenant(w, r); !ok {
		return
	}
	if !requireManagerRole(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
		return
	}

	requiredSet := make(map[string]struct{}, len(venueRequiredImportHeaders))
	for _, h := range venueRequiredImportHeaders {
		requiredSet[h] = struct{}{}
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	for i, header := range venueTemplateHeaders {
		_, required := requiredSet[header]
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, templateHeaderLabel(header, required))
	}
	// Kota must be one of domain.AllowedCities (same rule Create/Update
	// enforce via domain.IsValidCity) -- a dropdown here catches that in
	// Excel itself, not just after uploading. Kategori is reference-only
	// (D-4): its values are multi-select comma-separated, which an Excel
	// dropdown cannot validate, so the list is written to "Referensi" purely
	// as a lookup and the backend validates per row.
	if err := addTemplateDropdowns(f, sheet, venueTemplateHeaders, []templateDropdown{
		{header: "Kota", values: domain.AllowedCities},
		{header: "Kategori", values: domain.AllowedVenueCategories, referenceOnly: true},
	}); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas template", nil)
		return
	}
	spec := templateSheetSpec{
		currencyHeaders: []string{"Harga Sewa", "Charge"},
		textHeaders:     venueTextImportHeaders,
		requiredHeaders: venueRequiredImportHeaders,
	}
	if err := styleTemplateSheet(f, sheet, venueTemplateHeaders, spec); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas template", nil)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="template-venue.xlsx"`)
	if _, err := f.WriteTo(w); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat berkas template", nil)
	}
}

type venueImportRequestBody struct {
	Base64Data string `json:"base64Data"`
}

type venueImportRowErrorBody struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type venueImportResultBody struct {
	InsertedCount int                       `json:"insertedCount"`
	UpdatedCount  int                       `json:"updatedCount"`
	Errors        []venueImportRowErrorBody `json:"errors"`
}

func toVenueImportResultBody(result application.VenueImportResult) venueImportResultBody {
	errs := make([]venueImportRowErrorBody, 0, len(result.Errors))
	for _, e := range result.Errors {
		errs = append(errs, venueImportRowErrorBody{Row: e.Row, Message: e.Message})
	}
	return venueImportResultBody{InsertedCount: result.InsertedCount, UpdatedCount: result.UpdatedCount, Errors: errs}
}

// parseVenueImportRow reads one spreadsheet row's cells in venueTemplateHeaders'
// exact column order. A numeric cell that doesn't parse (parseNumberCell) is
// recorded into ParseIssues naming the column and the raw text, rather than
// silently becoming nil -- the service layer's required-field check
// (name/picName/phonePic/city) still gates which columns must be non-empty,
// but a parse failure on an optional column must still surface as an error,
// not disappear. PhonePIC/PhoneVenue are restored via normalizePhoneCell
// since Excel's default General format drops a leading "0".
func parseVenueImportRow(cells []string) application.VenueImportRow {
	get := func(i int) string {
		if i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	var issues []string
	parsePrice := func(column string, i int) *int64 {
		raw := get(i)
		n, err := parseNumberCell(raw)
		if err != nil {
			issues = append(issues, column+": "+err.Error())
			return nil
		}
		return n
	}
	parseCapacity := func(i int) *int {
		raw := get(i)
		n, err := parseNumberCell(raw)
		if err != nil {
			issues = append(issues, "Kapasitas: "+err.Error())
			return nil
		}
		if n == nil {
			return nil
		}
		if *n > math.MaxInt32 {
			issues = append(issues, fmt.Sprintf("Kapasitas: nilai %q terlalu besar", raw))
			return nil
		}
		capacity := int(*n)
		return &capacity
	}

	rentalPrice := parsePrice("Harga Sewa", 8)
	charge := parsePrice("Charge", 9)
	capacity := parseCapacity(10)

	// "Kategori" (column 7) holds comma-separated fixed values; the service
	// layer normalizes and rejects unknown ones per row. Empty clears (D-5).
	var categories []string
	if raw := get(7); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
	}

	return application.VenueImportRow{
		Name: get(0), PICName: get(1), PhonePIC: normalizePhoneCell(get(2)), PhoneVenue: normalizePhoneCell(get(3)), Email: get(4),
		Address: get(5), City: get(6), Categories: categories, RentalPrice: rentalPrice, Charge: charge,
		Capacity: capacity, Facilities: get(11), SocialMedia: get(12), Notes: get(13), ParseIssues: issues,
	}
}

func (h *VenueHandler) Import(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := requireStaffTenant(w, r)
	if !ok {
		return
	}
	if !requireManagerRole(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
		return
	}

	var body venueImportRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(body.Base64Data)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Data berkas tidak valid", nil)
		return
	}

	f, err := excelize.OpenReader(strings.NewReader(string(decoded)))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Berkas Excel tidak valid", nil)
		return
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rowIter, err := f.Rows(sheet)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Berkas Excel tidak valid", nil)
		return
	}
	defer rowIter.Close()

	var rows []application.VenueImportRow
	// Baris header divalidasi, bukan dilewati: parser di bawah membaca sel
	// berdasarkan INDEKS, jadi berkas dari template versi lama (sebelum kolom
	// "Kategori" disisipkan) akan menggeser setiap kolom sesudahnya — dan
	// sebagian barisnya tersimpan diam-diam dengan data salah kolom. Lihat
	// validateImportHeaderRow.
	if !rowIter.Next() {
		response.Error(w, http.StatusBadRequest, "Berkas kosong — gunakan berkas hasil unduh Template", nil)
		return
	}
	headerCells, err := rowIter.Columns()
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Gagal membaca baris header pada berkas Excel", nil)
		return
	}
	if err := validateImportHeaderRow(headerCells, venueTemplateHeaders); err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "Format berkas tidak sesuai: "+err.Error(), nil)
		return
	}
	for rowIter.Next() {
		cells, err := rowIter.Columns()
		if err != nil {
			response.Error(w, http.StatusBadRequest, "Gagal membaca salah satu baris pada berkas Excel", nil)
			return
		}
		if len(cells) == 0 || strings.TrimSpace(strings.Join(cells, "")) == "" {
			continue // fully blank row (e.g. trailing rows) -- not a data row
		}
		if len(rows) >= 1000 {
			// The service layer re-checks and rejects at exactly this cap --
			// stop reading further rows here too so a pathological file with
			// far more than 1000 rows doesn't keep streaming needlessly.
			rows = append(rows, parseVenueImportRow(cells))
			break
		}
		rows = append(rows, parseVenueImportRow(cells))
	}

	result, err := h.venues.ImportVenues(r.Context(), tenantID, rows)
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "Impor venue selesai diproses", toVenueImportResultBody(result))
}
