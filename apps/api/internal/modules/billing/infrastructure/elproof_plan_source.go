package infrastructure

import (
	"context"

	"jwswedding/internal/modules/billing/domain"
	"jwswedding/internal/shared/elproofpay"
)

// ElProofPlanSource implements application.PlanRepository by delegating to
// the shared elproofpay.Client (D12) — read-through, ListPlans already
// caches for 60s on the client side, so this adapter adds no cache of its
// own.
type ElProofPlanSource struct {
	client *elproofpay.Client
}

func NewElProofPlanSource(client *elproofpay.Client) *ElProofPlanSource {
	return &ElProofPlanSource{client: client}
}

func (s *ElProofPlanSource) List(ctx context.Context) ([]domain.Plan, error) {
	plans, err := s.client.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Plan, 0, len(plans))
	for _, p := range plans {
		result = append(result, toDomainPlan(p))
	}
	return result, nil
}

func (s *ElProofPlanSource) FindByID(ctx context.Context, id int64) (*domain.Plan, error) {
	plans, err := s.client.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range plans {
		if p.ID == id {
			plan := toDomainPlan(p)
			return &plan, nil
		}
	}
	return nil, nil
}

func toDomainPlan(p elproofpay.Plan) domain.Plan {
	return domain.Plan{ID: p.ID, Name: p.Name, DurationMonths: p.DurationMonths, Price: p.Price, IsActive: p.Active}
}
