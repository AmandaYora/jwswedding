package presentation

import (
	"net/http"

	"jwswedding/internal/modules/billing/application"
	"jwswedding/internal/modules/billing/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/response"
)

type PlanHandler struct {
	plans *application.PlanService
}

func NewPlanHandler(plans *application.PlanService) *PlanHandler {
	return &PlanHandler{plans: plans}
}

type planResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DurationMonths int    `json:"durationMonths"`
	Price          int64  `json:"price"`
	IsActive       bool   `json:"isActive"`
}

func toPlanResponseValue(p domain.Plan) planResponse {
	return planResponse{
		ID: p.ID, Name: p.Name, DurationMonths: p.DurationMonths, Price: p.Price, IsActive: p.IsActive,
	}
}

func toPlanResponses(plans []domain.Plan) []planResponse {
	result := make([]planResponse, 0, len(plans))
	for _, p := range plans {
		result = append(result, toPlanResponseValue(p))
	}
	return result
}

// Collection is read-only now (D7: the plan catalog lives at ElProof) —
// GET only, open to any authenticated principal, same as before.
func (h *PlanHandler) Collection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "Metode HTTP tidak diizinkan untuk endpoint ini", nil)
		return
	}
	plans, err := h.plans.List(r.Context())
	if err != nil {
		writeAppError(w, err)
		return
	}
	response.OK(w, "ok", toPlanResponses(plans))
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
