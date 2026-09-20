package presentation

import (
	"net/http"
	"strconv"

	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/response"
)

// staffClaims adalah irisan sempit klaim JWT yang dibutuhkan modul ini.
// Bentuknya sama dengan milik `projects` dan `quotations`, dan memang
// diduplikasi: repo ini menaruh penjaga role per modul alih-alih mengimpornya
// lintas modul.
type staffClaims struct {
	tenantID int64
	staffID  int64
	role     string
}

func requireStaff(w http.ResponseWriter, r *http.Request) (staffClaims, bool) {
	claims, ok := middleware.FromContext(r.Context())
	if !ok || claims.PrincipalType != "staff" {
		response.Error(w, http.StatusForbidden, "Hanya staff WO yang dapat mengakses endpoint ini", nil)
		return staffClaims{}, false
	}
	tenantID, ok := claims.TenantIDInt()
	if !ok {
		response.Error(w, http.StatusForbidden, "Akun ini tidak terikat ke tenant manapun", nil)
		return staffClaims{}, false
	}
	staffID, err := strconv.ParseInt(claims.PrincipalID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusForbidden, "Identitas staff tidak valid", nil)
		return staffClaims{}, false
	}
	return staffClaims{tenantID: tenantID, staffID: staffID, role: claims.Role}, true
}

// requireRundownAccess: Owner, Admin, dan Wedding Planner boleh membuka menu
// Rundown. Sales TIDAK — perannya berhenti di tahap pra-deal, sementara buku
// acara adalah dokumen operasional hari-H.
//
// Perhatikan ini BERBEDA dari menu Penawaran, yang justru menutup Wedding
// Planner dan membuka Sales. Jangan disalin begitu saja dari sana.
func requireRundownAccess(w http.ResponseWriter, role string) bool {
	switch role {
	case "Owner", "Admin", "Staff":
		return true
	}
	response.Error(w, http.StatusForbidden,
		"Hanya Owner, Admin, atau Wedding Planner yang dapat mengakses rundown", nil)
	return false
}

// requireOwnerOrAdmin menjaga penghapusan rundown. Wedding Planner mengelola
// isi buku acara project yang dipegangnya, tetapi tidak menghapusnya —
// mengikuti preseden hard-delete Owner-or-Admin di ADR-0019.
func requireOwnerOrAdmin(w http.ResponseWriter, role string) bool {
	if role == "Owner" || role == "Admin" {
		return true
	}
	response.Error(w, http.StatusForbidden,
		"Hanya Owner atau Admin yang dapat menghapus rundown", nil)
	return false
}

func isWeddingPlanner(role string) bool { return role == "Staff" }

// writeAppError memetakan galat domain ke status HTTP, berpola sama dengan
// quotations/presentation/util.go:61.
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
	logger.Error("rundowns: %v", err)
	response.Error(w, status, "Terjadi kesalahan pada server", nil)
}
