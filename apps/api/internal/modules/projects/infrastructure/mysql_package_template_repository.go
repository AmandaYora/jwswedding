package infrastructure

import (
	"context"
	"database/sql"

	"jwswedding/internal/modules/projects/domain"
)

type MySQLPackageTemplateRepository struct {
	db *sql.DB
}

func NewMySQLPackageTemplateRepository(db *sql.DB) *MySQLPackageTemplateRepository {
	return &MySQLPackageTemplateRepository{db: db}
}

const packageTemplateColumns = `id, tenant_id, name, base_price, default_terms, default_bonus_note, is_active, sort_order, created_at, updated_at`

func scanPackageTemplate(scan func(dest ...interface{}) error) (*domain.PackageTemplate, error) {
	var t domain.PackageTemplate
	err := scan(&t.ID, &t.TenantID, &t.Name, &t.BasePrice, &t.DefaultTerms, &t.DefaultBonusNote,
		&t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns header rows only — Blocks/Terms stay nil. The template picker
// and the settings list both only need the header, and a tenant's full
// composition is large enough (7-9 blocks x N tiers) that loading it for a
// dropdown would be wasteful.
func (r *MySQLPackageTemplateRepository) List(ctx context.Context, tenantID int64, activeOnly bool) ([]domain.PackageTemplate, error) {
	query := `SELECT ` + packageTemplateColumns + ` FROM package_templates WHERE tenant_id = ?`
	if activeOnly {
		query += ` AND is_active = 1`
	}
	query += ` ORDER BY sort_order, id`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []domain.PackageTemplate
	for rows.Next() {
		t, err := scanPackageTemplate(rows.Scan)
		if err != nil {
			return nil, err
		}
		templates = append(templates, *t)
	}
	return templates, rows.Err()
}

// FindByID loads the header plus its blocks and terms — this is the shape
// ApplyTemplate copies from, so a partial load here would silently produce
// an empty composition.
func (r *MySQLPackageTemplateRepository) FindByID(ctx context.Context, tenantID, id int64) (*domain.PackageTemplate, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+packageTemplateColumns+` FROM package_templates WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	t, err := scanPackageTemplate(row.Scan)
	if err != nil || t == nil {
		return nil, err
	}
	if t.Blocks, err = r.listBlocks(ctx, t.ID); err != nil {
		return nil, err
	}
	if t.Terms, err = r.listTerms(ctx, t.ID); err != nil {
		return nil, err
	}
	return t, nil
}

func (r *MySQLPackageTemplateRepository) listBlocks(ctx context.Context, templateID int64) ([]domain.PackageTemplateBlock, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, template_id, category, body, qty_text, bonus_note, sort_order
		 FROM package_template_blocks WHERE template_id = ? ORDER BY sort_order, id`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := []domain.PackageTemplateBlock{}
	for rows.Next() {
		var b domain.PackageTemplateBlock
		if err := rows.Scan(&b.ID, &b.TemplateID, &b.Category, &b.Body, &b.QtyText, &b.BonusNote, &b.SortOrder); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

func (r *MySQLPackageTemplateRepository) listTerms(ctx context.Context, templateID int64) ([]domain.PackageTemplateTerm, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, template_id, sequence, label, type, percent, fixed_amount, days_before_event
		 FROM package_template_terms WHERE template_id = ? ORDER BY sequence, id`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	terms := []domain.PackageTemplateTerm{}
	for rows.Next() {
		var t domain.PackageTemplateTerm
		var percent sql.NullFloat64
		var fixed sql.NullInt64
		if err := rows.Scan(&t.ID, &t.TemplateID, &t.Sequence, &t.Label, &t.Type, &percent, &fixed, &t.DaysBeforeEvent); err != nil {
			return nil, err
		}
		if percent.Valid {
			t.Percent = &percent.Float64
		}
		if fixed.Valid {
			t.FixedAmount = &fixed.Int64
		}
		terms = append(terms, t)
	}
	return terms, rows.Err()
}

func (r *MySQLPackageTemplateRepository) Create(ctx context.Context, t *domain.PackageTemplate) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO package_templates (tenant_id, name, base_price, default_terms, default_bonus_note, is_active, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.TenantID, t.Name, t.BasePrice, t.DefaultTerms, t.DefaultBonusNote, t.IsActive, t.SortOrder,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

func (r *MySQLPackageTemplateRepository) Update(ctx context.Context, t *domain.PackageTemplate) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE package_templates SET name = ?, base_price = ?, default_terms = ?, default_bonus_note = ?, is_active = ?
		 WHERE tenant_id = ? AND id = ?`,
		t.Name, t.BasePrice, t.DefaultTerms, t.DefaultBonusNote, t.IsActive, t.TenantID, t.ID,
	)
	return err
}

// Delete relies on ON DELETE CASCADE for blocks and terms (migration 000052)
// — safe here because both are children of this one aggregate inside the same
// module, never cross-module references.
func (r *MySQLPackageTemplateRepository) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM package_templates WHERE tenant_id = ? AND id = ?`, tenantID, id)
	return err
}

func (r *MySQLPackageTemplateRepository) NextSortOrder(ctx context.Context, tenantID int64) (int, error) {
	var maxOrder sql.NullInt64
	row := r.db.QueryRowContext(ctx, `SELECT MAX(sort_order) FROM package_templates WHERE tenant_id = ?`, tenantID)
	if err := row.Scan(&maxOrder); err != nil {
		return 0, err
	}
	return int(maxOrder.Int64) + 1, nil
}

// ReplaceBlocks rewrites a template's whole block list in one transaction.
// Replace-all rather than per-row diffing: the editor submits the entire
// composition each save, blocks carry no cross-references, and sort_order is
// positional — so a diff would add moving parts without buying anything.
func (r *MySQLPackageTemplateRepository) ReplaceBlocks(ctx context.Context, templateID int64, blocks []domain.PackageTemplateBlock) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM package_template_blocks WHERE template_id = ?`, templateID); err != nil {
		return err
	}
	for i, b := range blocks {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO package_template_blocks (template_id, category, body, qty_text, bonus_note, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			templateID, b.Category, b.Body, b.QtyText, b.BonusNote, i+1,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplaceTerms rewrites a template's payment schedule preset — same
// replace-all reasoning as ReplaceBlocks.
func (r *MySQLPackageTemplateRepository) ReplaceTerms(ctx context.Context, templateID int64, terms []domain.PackageTemplateTerm) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM package_template_terms WHERE template_id = ?`, templateID); err != nil {
		return err
	}
	for i, t := range terms {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO package_template_terms (template_id, sequence, label, type, percent, fixed_amount, days_before_event)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			templateID, i+1, t.Label, t.Type, t.Percent, t.FixedAmount, t.DaysBeforeEvent,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}
