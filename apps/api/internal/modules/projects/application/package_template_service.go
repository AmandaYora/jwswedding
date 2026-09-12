package application

import (
	"context"
	"strconv"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
)

type PackageTemplateRepository interface {
	List(ctx context.Context, tenantID int64, activeOnly bool) ([]domain.PackageTemplate, error)
	// FindByID must return the template WITH its blocks and terms loaded —
	// ApplyTemplate copies from exactly this shape, so a header-only load here
	// would silently produce an empty composition.
	FindByID(ctx context.Context, tenantID, id int64) (*domain.PackageTemplate, error)
	Create(ctx context.Context, t *domain.PackageTemplate) error
	Update(ctx context.Context, t *domain.PackageTemplate) error
	Delete(ctx context.Context, tenantID, id int64) error
	NextSortOrder(ctx context.Context, tenantID int64) (int, error)
	ReplaceBlocks(ctx context.Context, templateID int64, blocks []domain.PackageTemplateBlock) error
	ReplaceTerms(ctx context.Context, templateID int64, terms []domain.PackageTemplateTerm) error
}

type PackageTemplateService struct {
	repo PackageTemplateRepository
}

func NewPackageTemplateService(repo PackageTemplateRepository) *PackageTemplateService {
	return &PackageTemplateService{repo: repo}
}

func (s *PackageTemplateService) List(ctx context.Context, tenantID int64, activeOnly bool) ([]domain.PackageTemplate, error) {
	return s.repo.List(ctx, tenantID, activeOnly)
}

func (s *PackageTemplateService) Get(ctx context.Context, tenantID, id int64) (*domain.PackageTemplate, error) {
	t, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, apperror.NotFound("Template paket tidak ditemukan")
	}
	return t, nil
}

type PackageTemplateInput struct {
	Name             string
	BasePrice        int64
	DefaultTerms     string
	DefaultBonusNote string
	IsActive         bool
}

func (s *PackageTemplateService) Create(ctx context.Context, tenantID int64, input PackageTemplateInput) (*domain.PackageTemplate, error) {
	if err := validateTemplateInput(input); err != nil {
		return nil, err
	}
	order, err := s.repo.NextSortOrder(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	t := &domain.PackageTemplate{
		TenantID: tenantID, Name: input.Name, BasePrice: input.BasePrice,
		DefaultTerms: input.DefaultTerms, DefaultBonusNote: input.DefaultBonusNote,
		IsActive: input.IsActive, SortOrder: order,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *PackageTemplateService) Update(ctx context.Context, tenantID, id int64, input PackageTemplateInput) (*domain.PackageTemplate, error) {
	if err := validateTemplateInput(input); err != nil {
		return nil, err
	}
	t, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	t.Name = input.Name
	t.BasePrice = input.BasePrice
	t.DefaultTerms = input.DefaultTerms
	t.DefaultBonusNote = input.DefaultBonusNote
	t.IsActive = input.IsActive
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Delete is a hard delete, cascading to blocks and terms via the FK. Safe for
// the same reason ProjectMilestoneTemplate's is: a template row is only ever
// copied into a project at ApplyTemplate time and never referenced again
// (D22), so removing it orphans nothing — no project's agreement changes.
func (s *PackageTemplateService) Delete(ctx context.Context, tenantID, id int64) error {
	if _, err := s.Get(ctx, tenantID, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, tenantID, id)
}

// ReplaceBlocks rewrites a template's composition. The Get call is not
// redundant with the repo write: it is what scopes the templateID to this
// tenant before the repo (which keys on template_id alone) touches anything.
func (s *PackageTemplateService) ReplaceBlocks(ctx context.Context, tenantID, id int64, blocks []domain.PackageTemplateBlock) error {
	t, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	for i, b := range blocks {
		if b.Category == "" {
			return apperror.Validation("Kategori wajib diisi", map[string][]string{
				"blocks": {"Rincian ke-" + strconv.Itoa(i+1) + " belum diberi kategori"},
			})
		}
	}
	return s.repo.ReplaceBlocks(ctx, t.ID, blocks)
}

func (s *PackageTemplateService) ReplaceTerms(ctx context.Context, tenantID, id int64, terms []domain.PackageTemplateTerm) error {
	t, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := validateTerms(terms); err != nil {
		return err
	}
	return s.repo.ReplaceTerms(ctx, t.ID, terms)
}

func validateTemplateInput(input PackageTemplateInput) error {
	if input.Name == "" {
		return apperror.Validation("Nama paket wajib diisi", map[string][]string{
			"name": {"Nama paket wajib diisi"},
		})
	}
	if input.BasePrice < 0 {
		return apperror.Validation("Harga paket tidak boleh negatif", map[string][]string{
			"basePrice": {"Harga paket tidak boleh negatif"},
		})
	}
	return nil
}

// validateTerms enforces the two invariants the seeding math in
// PackageOrderService depends on: a term is exactly one of percentage-based
// or fixed-amount, and its type is one a Tagihan can actually carry. Tambahan
// is excluded deliberately — it is born from a revision absorbing a change
// (D24), never from a preset — and Refund is not a bill at all.
func validateTerms(terms []domain.PackageTemplateTerm) error {
	for i, t := range terms {
		pos := "Tahap pembayaran ke-" + strconv.Itoa(i+1)
		switch t.Type {
		case domain.PaymentDP, domain.PaymentTermin, domain.PaymentPelunasan:
		default:
			return apperror.Validation("Jenis tahap pembayaran belum dipilih", map[string][]string{
				"terms": {pos + " belum dipilih jenisnya"},
			})
		}
		hasPercent := t.Percent != nil
		hasFixed := t.FixedAmount != nil
		if hasPercent == hasFixed {
			return apperror.Validation("Isi persentase atau nominal, jangan keduanya", map[string][]string{
				"terms": {pos + ": isi persentase saja atau nominal saja"},
			})
		}
		if hasPercent && (*t.Percent <= 0 || *t.Percent > 100) {
			return apperror.Validation("Persentase tidak valid", map[string][]string{
				"terms": {pos + ": persentase harus antara 1 dan 100"},
			})
		}
		if hasFixed && *t.FixedAmount <= 0 {
			return apperror.Validation("Nominal tidak valid", map[string][]string{
				"terms": {pos + ": nominal harus lebih dari nol"},
			})
		}
	}
	return nil
}
