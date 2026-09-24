package application

import (
	"context"

	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/apperror"
)

// RundownTemplateService mengelola Template Rundown per tenant: isi standar
// enam seksi yang disalin ke rundown baru (PLAN rundown-ux-ideal §6.4).
type RundownTemplateService struct {
	templates TemplateRepository
	rundowns  RundownRepository
}

func NewRundownTemplateService(templates TemplateRepository, rundowns RundownRepository) *RundownTemplateService {
	return &RundownTemplateService{templates: templates, rundowns: rundowns}
}

// Get mengembalikan template tenant. Tenant yang belum pernah menyimpan
// template mendapat template kosong, bukan 404 — "belum ada isi" adalah
// keadaan normal yang ditangani editor dengan ajakan mengisi.
func (s *RundownTemplateService) Get(ctx context.Context, tenantID int64) (*domain.Template, error) {
	t, err := s.templates.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return &domain.Template{TenantID: tenantID}, nil
	}
	return t, nil
}

// ReplaceSection mengganti satu seksi template dengan aturan validasi dan
// penomoran yang sama persis dengan seksi rundown.
func (s *RundownTemplateService) ReplaceSection(ctx context.Context, tenantID int64,
	section domain.SectionKey, payload SectionPayload) (*domain.Template, error) {

	if !section.InTemplate() {
		return nil, apperror.Validation("Seksi ini tidak termasuk Template Rundown",
			map[string][]string{"section": {"Seksi tidak termasuk template"}})
	}
	if err := validateSection(section, &payload); err != nil {
		return nil, err
	}
	if section == domain.SectionKeyAcaraAkad || section == domain.SectionKeyAcaraResepsi {
		renumberItems(payload.Items)
	}
	if section == domain.SectionKeyLayout {
		renumberLegend(payload.LayoutNotes)
	}
	if err := s.templates.ReplaceSection(ctx, tenantID, section, payload); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID)
}

// SaveFromRundown menjadikan isi sebuah rundown sebagai template ("Jadikan
// Template"). Nama orang dikosongkan — template berisi PERAN, bukan orang
// yang kebetulan mengisinya di acara itu. Pemanggil wajib sudah memastikan
// hak akses ke rundown ini.
func (s *RundownTemplateService) SaveFromRundown(ctx context.Context, tenantID, rundownID int64) (*domain.Template, error) {
	v, err := s.rundowns.LoadAggregate(ctx, tenantID, rundownID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, apperror.NotFound("Rundown tidak ditemukan")
	}

	t := &domain.Template{TenantID: tenantID, MakeupRooms: v.MakeupRooms, LayoutNotes: v.LayoutNotes}
	for _, r := range v.Roles {
		r.PersonName = ""
		t.Roles = append(t.Roles, r)
	}
	for _, c := range v.Committees {
		c.PersonText = ""
		t.Committees = append(t.Committees, c)
	}
	for _, it := range v.Items {
		if it.Section == domain.SectionResepsi {
			t.ItemsResepsi = append(t.ItemsResepsi, it)
			continue
		}
		t.ItemsAkad = append(t.ItemsAkad, it)
	}

	if err := s.templates.ReplaceAll(ctx, t); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID)
}
