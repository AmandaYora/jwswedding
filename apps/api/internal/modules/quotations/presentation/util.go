package presentation

import (
	"net/http"
	"strconv"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/response"
)

const dateLayout = "2006-01-02"

// staffClaims is the narrow, parsed set of claims every handler in this
// module needs — same shape as projects' own, duplicated deliberately (the
// codebase duplicates these guards per module rather than importing them
// cross-module).
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

// requireQuotationManager gates every penawaran endpoint (T4.4):
// Sales/Admin/Owner mengelola penawaran dan menimpa harga. Wedding Planner
// ("Staff") tidak membuka menu Penawaran — penawaran pra-project tidak punya
// PIC untuk di-scope, dan angkanya sensitif margin.
func requireQuotationManager(w http.ResponseWriter, role string) bool {
	if role == "Owner" || role == "Admin" || role == "Sales" {
		return true
	}
	response.Error(w, http.StatusForbidden, "Hanya Owner, Admin, atau Sales yang dapat mengelola penawaran", nil)
	return false
}

func isOwnerOrAdmin(role string) bool {
	return role == "Owner" || role == "Admin"
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
	logger.Error("unhandled error: %v", err)
	response.Error(w, status, "Terjadi kesalahan pada server", nil)
}

// requireCompleteProfile enforces the letterhead completeness gate shared by
// every document PDF — a document carrying the WO's letterhead must not
// print without the profile behind that letterhead being filled in.
func requireCompleteProfile(h *Handler, w http.ResponseWriter, r *http.Request, tenantID int64) (platformcontracts.TenantProfile, bool) {
	profile, err := h.platform.GetTenantProfile(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return platformcontracts.TenantProfile{}, false
	}
	missing, err := h.platform.ProfileMissingFields(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return platformcontracts.TenantProfile{}, false
	}
	if len(missing) > 0 {
		response.Error(w, http.StatusUnprocessableEntity,
			"Profil usaha belum lengkap. Lengkapi Profil Usaha sebelum mencetak dokumen.",
			map[string][]string{"profile": missing})
		return platformcontracts.TenantProfile{}, false
	}
	return profile, true
}
