package application

import (
	"context"

	"jwswedding/internal/modules/billing/domain"
	"jwswedding/internal/shared/apperror"
)

// PlanRepository is now read-only (D7: the plan catalog moved to ElProof) —
// Create/Update/SetActive/ListPaginated are gone along with the local
// `subscription_plans`/`plan_features` tables backing them.
type PlanRepository interface {
	List(ctx context.Context) ([]domain.Plan, error)
	FindByID(ctx context.Context, id int64) (*domain.Plan, error)
}

type PlanService struct {
	repo PlanRepository
}

func NewPlanService(repo PlanRepository) *PlanService {
	return &PlanService{repo: repo}
}

func (s *PlanService) List(ctx context.Context) ([]domain.Plan, error) {
	return s.repo.List(ctx)
}

func (s *PlanService) Get(ctx context.Context, id int64) (*domain.Plan, error) {
	plan, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, apperror.NotFound("Paket tidak ditemukan")
	}
	return plan, nil
}
