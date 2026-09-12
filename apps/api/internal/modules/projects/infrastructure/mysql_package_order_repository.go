package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"

	"jwswedding/internal/modules/projects/domain"
)

type MySQLPackageOrderRepository struct {
	db *sql.DB
}

func NewMySQLPackageOrderRepository(db *sql.DB) *MySQLPackageOrderRepository {
	return &MySQLPackageOrderRepository{db: db}
}

const packageOrderColumns = `id, project_id, po_number, number_period, number_seq, revision, base_price,
	terms_text, bonus_note, terms_plan_json, status, snapshot_json, issued_at, created_by_staff_id, created_at, updated_at`

// scanPackageOrder maps the four nullable columns onto their sentinel forms:
// po_number/number_period are "" and number_seq is 0 until the first Issue
// (D26), while terms_plan_json and snapshot_json decode to nil when absent.
func scanPackageOrder(scan func(dest ...interface{}) error) (*domain.PackageOrder, error) {
	var o domain.PackageOrder
	var poNumber, numberPeriod sql.NullString
	var numberSeq sql.NullInt64
	var termsPlan, snapshot []byte
	var issuedAt sql.NullTime

	err := scan(&o.ID, &o.ProjectID, &poNumber, &numberPeriod, &numberSeq, &o.Revision, &o.BasePrice,
		&o.TermsText, &o.BonusNote, &termsPlan, &o.Status, &snapshot, &issuedAt,
		&o.CreatedByStaffID, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	o.PONumber = poNumber.String
	o.NumberPeriod = numberPeriod.String
	o.NumberSeq = int(numberSeq.Int64)
	if issuedAt.Valid {
		o.IssuedAt = &issuedAt.Time
	}
	if len(termsPlan) > 0 {
		if err := json.Unmarshal(termsPlan, &o.TermsPlan); err != nil {
			return nil, err
		}
	}
	if len(snapshot) > 0 {
		var s domain.PackageOrderSnapshot
		if err := json.Unmarshal(snapshot, &s); err != nil {
			return nil, err
		}
		o.Snapshot = &s
	}
	return &o, nil
}

// FindByProject loads the one PO a project may have (uq_..._project). Returns
// (nil, nil) when the project has none — the empty-state path (D23), which is
// where every pre-existing project starts.
func (r *MySQLPackageOrderRepository) FindByProject(ctx context.Context, projectID int64) (*domain.PackageOrder, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+packageOrderColumns+` FROM project_package_orders WHERE project_id = ? LIMIT 1`, projectID)
	return scanPackageOrder(row.Scan)
}

func (r *MySQLPackageOrderRepository) Create(ctx context.Context, o *domain.PackageOrder) error {
	termsPlan, err := marshalOrNil(o.TermsPlan)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO project_package_orders
		 (project_id, base_price, terms_text, bonus_note, terms_plan_json, status, created_by_staff_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		o.ProjectID, o.BasePrice, o.TermsText, o.BonusNote, termsPlan, o.Status, o.CreatedByStaffID,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	o.ID = id
	return nil
}

// Update writes every mutable column. The numbering columns are included
// because Issue assigns them, but the caller is responsible for only ever
// setting them once — see PackageOrder.IsNumbered and §9 R6.
func (r *MySQLPackageOrderRepository) Update(ctx context.Context, o *domain.PackageOrder) error {
	termsPlan, err := marshalOrNil(o.TermsPlan)
	if err != nil {
		return err
	}
	var snapshot []byte
	if o.Snapshot != nil {
		if snapshot, err = json.Marshal(o.Snapshot); err != nil {
			return err
		}
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE project_package_orders SET
		   po_number = ?, number_period = ?, number_seq = ?, revision = ?, base_price = ?,
		   terms_text = ?, bonus_note = ?, terms_plan_json = ?, status = ?, snapshot_json = ?, issued_at = ?
		 WHERE id = ?`,
		nullIfEmpty(o.PONumber), nullIfEmpty(o.NumberPeriod), nullIfZero(o.NumberSeq), o.Revision, o.BasePrice,
		o.TermsText, o.BonusNote, termsPlan, o.Status, snapshot, o.IssuedAt, o.ID,
	)
	return err
}

// NextPOSequence returns MAX(number_seq)+1 for (tenant, period). Same
// same-module-join idiom as MySQLClientInvoiceRepository.NextInvoiceSequence:
// project_package_orders carries no tenant_id of its own, and `projects` is
// owned by this very module, so the join crosses no boundary.
//
// Rows still in Draft have number_seq NULL and are skipped by MAX() on their
// own — an unissued PO never consumes a number (D26).
func (r *MySQLPackageOrderRepository) NextPOSequence(ctx context.Context, tenantID int64, period string) (int, error) {
	var maxSeq sql.NullInt64
	row := r.db.QueryRowContext(ctx,
		`SELECT MAX(po.number_seq) FROM project_package_orders po
		 JOIN projects p ON p.id = po.project_id
		 WHERE p.tenant_id = ? AND po.number_period = ?`, tenantID, period)
	if err := row.Scan(&maxSeq); err != nil {
		return 0, err
	}
	return int(maxSeq.Int64) + 1, nil
}

func (r *MySQLPackageOrderRepository) ListBlocks(ctx context.Context, projectID int64) ([]domain.ProjectPackageBlock, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, category, body, qty_text, bonus_note, sort_order
		 FROM project_package_blocks WHERE project_id = ? ORDER BY sort_order, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := []domain.ProjectPackageBlock{}
	for rows.Next() {
		var b domain.ProjectPackageBlock
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.Category, &b.Body, &b.QtyText, &b.BonusNote, &b.SortOrder); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

// ReplaceBlocks rewrites a project's whole composition in one transaction —
// same replace-all reasoning as the template repository's own version.
func (r *MySQLPackageOrderRepository) ReplaceBlocks(ctx context.Context, projectID int64, blocks []domain.ProjectPackageBlock) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_package_blocks WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for i, b := range blocks {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project_package_blocks (project_id, category, body, qty_text, bonus_note, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			projectID, b.Category, b.Body, b.QtyText, b.BonusNote, i+1,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *MySQLPackageOrderRepository) ListAdjustments(ctx context.Context, projectID int64) ([]domain.ProjectPackageAdjustment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, description, amount, sort_order
		 FROM project_package_adjustments WHERE project_id = ? ORDER BY sort_order, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	adjustments := []domain.ProjectPackageAdjustment{}
	for rows.Next() {
		var a domain.ProjectPackageAdjustment
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Description, &a.Amount, &a.SortOrder); err != nil {
			return nil, err
		}
		adjustments = append(adjustments, a)
	}
	return adjustments, rows.Err()
}

func (r *MySQLPackageOrderRepository) ReplaceAdjustments(ctx context.Context, projectID int64, adjustments []domain.ProjectPackageAdjustment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_package_adjustments WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for i, a := range adjustments {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project_package_adjustments (project_id, description, amount, sort_order)
			 VALUES (?, ?, ?, ?)`,
			projectID, a.Description, a.Amount, i+1,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func marshalOrNil(plan []domain.TermPlanEntry) ([]byte, error) {
	if len(plan) == 0 {
		return nil, nil
	}
	return json.Marshal(plan)
}

// nullIfEmpty/nullIfZero keep the sentinel-in-Go, NULL-in-MySQL split honest:
// a Draft PO must write real NULLs so uq_..._number tolerates many of them
// (D26) — writing "" instead would collide on the second project.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(i int) interface{} {
	if i == 0 {
		return nil
	}
	return i
}
