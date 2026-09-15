package infrastructure

import (
	"context"
	"database/sql"

	"jwswedding/internal/modules/quotations/domain"
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

// List returns one summary row per template: the header plus how many blocks
// and terms sit behind it. Never the composition itself — that is what
// FindByID is for, and PackageTemplateSummary has no field to put it in.
//
// The counts ride along as correlated subqueries rather than a second round
// trip: both child tables are indexed on template_id and a tenant sells a
// handful of tiers. Carrying the count with the row is what lets the list be
// honest about a composition it is deliberately not shipping.
func (r *MySQLPackageTemplateRepository) List(ctx context.Context, tenantID int64, activeOnly bool) ([]domain.PackageTemplateSummary, error) {
	query := `SELECT t.id, t.tenant_id, t.name, t.base_price, t.default_terms, t.default_bonus_note,
	                 t.is_active, t.sort_order,
	                 (SELECT COUNT(*) FROM package_template_blocks b WHERE b.template_id = t.id)
	          FROM package_templates t WHERE t.tenant_id = ?`
	if activeOnly {
		query += ` AND t.is_active = 1`
	}
	query += ` ORDER BY t.sort_order, t.id`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []domain.PackageTemplateSummary{}
	for rows.Next() {
		var t domain.PackageTemplateSummary
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.BasePrice, &t.DefaultTerms,
			&t.DefaultBonusNote, &t.IsActive, &t.SortOrder, &t.BlockCount); err != nil {
			return nil, err
		}
		summaries = append(summaries, t)
	}
	return summaries, rows.Err()
}

// FindByID loads the header plus its blocks — this is the shape a new
// penawaran copies from, so a partial load here would silently produce an
// empty composition.
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
