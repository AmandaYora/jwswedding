package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"

	"jwswedding/internal/modules/projects/domain"
)

type MySQLClientPaymentRepository struct {
	db *sql.DB
}

func NewMySQLClientPaymentRepository(db *sql.DB) *MySQLClientPaymentRepository {
	return &MySQLClientPaymentRepository{db: db}
}

// clientPaymentColumns dipakai SEMUA jalur baca (ListByProject, FindByID, dan
// lewat FindByID juga EnsureReceiptNumber yang memasok struct ke PDF Kwitansi)
// — menambah kolom di sini otomatis mengaliri ketiganya.
const clientPaymentColumns = `id, project_id, type, amount, payment_date, method, reference_number, notes,
	created_by_staff_id, receipt_number, receipt_period, receipt_seq`

func scanClientPayment(scan func(dest ...interface{}) error) (*domain.ClientPayment, error) {
	var p domain.ClientPayment
	var paymentType string
	var notes, receiptNumber, receiptPeriod sql.NullString
	var receiptSeq sql.NullInt64
	err := scan(
		&p.ID, &p.ProjectID, &paymentType, &p.Amount, &p.PaymentDate, &p.Method, &p.ReferenceNumber, &notes,
		&p.CreatedByStaffID, &receiptNumber, &receiptPeriod, &receiptSeq,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Type = domain.PaymentType(paymentType)
	p.Notes = notes.String
	p.ReceiptNumber = receiptNumber.String
	p.ReceiptPeriod = receiptPeriod.String
	p.ReceiptSeq = int(receiptSeq.Int64)
	return &p, nil
}

func (r *MySQLClientPaymentRepository) ListByProject(ctx context.Context, projectID int64) ([]domain.ClientPayment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+clientPaymentColumns+` FROM client_payments WHERE project_id = ? ORDER BY payment_date DESC, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ClientPayment
	for rows.Next() {
		p, err := scanClientPayment(rows.Scan)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

func (r *MySQLClientPaymentRepository) FindByID(ctx context.Context, projectID, id int64) (*domain.ClientPayment, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+clientPaymentColumns+` FROM client_payments WHERE project_id = ? AND id = ? LIMIT 1`, projectID, id)
	return scanClientPayment(row.Scan)
}

func (r *MySQLClientPaymentRepository) Create(ctx context.Context, p *domain.ClientPayment) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO client_payments (project_id, type, amount, payment_date, method, reference_number, notes, created_by_staff_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProjectID, string(p.Type), p.Amount, p.PaymentDate, p.Method, p.ReferenceNumber, p.Notes, p.CreatedByStaffID,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func (r *MySQLClientPaymentRepository) Update(ctx context.Context, projectID, id int64, p domain.ClientPayment) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE client_payments SET type = ?, amount = ?, payment_date = ?, method = ?, reference_number = ?, notes = ?
		 WHERE project_id = ? AND id = ?`,
		string(p.Type), p.Amount, p.PaymentDate, p.Method, p.ReferenceNumber, p.Notes, projectID, id,
	)
	return err
}

func (r *MySQLClientPaymentRepository) Delete(ctx context.Context, projectID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM client_payments WHERE project_id = ? AND id = ?`, projectID, id)
	return err
}

// NextReceiptSequence returns MAX(receipt_seq)+1 for (tenant, period) — same
// MAX-based idiom as MySQLMilestoneRepository.NextSortOrder, tenant-scoped
// via the same-module join to projects (not a cross-module join —
// client_payments and projects both belong to `projects`).
func (r *MySQLClientPaymentRepository) NextReceiptSequence(ctx context.Context, tenantID int64, period string) (int, error) {
	var maxSeq sql.NullInt64
	row := r.db.QueryRowContext(ctx,
		`SELECT MAX(cp.receipt_seq) FROM client_payments cp
		 JOIN projects p ON p.id = cp.project_id
		 WHERE p.tenant_id = ? AND cp.receipt_period = ?`, tenantID, period)
	if err := row.Scan(&maxSeq); err != nil {
		return 0, err
	}
	return int(maxSeq.Int64) + 1, nil
}

// SetReceiptNumber writes the lazily-assigned Kwitansi number — translates a
// MySQL 1062 (two concurrent prints computing the same seq) into
// domain.ErrDuplicateReceiptNumber, same pattern as
// MySQLTenantRepository.Update's ErrDuplicateCustomDomain translation.
func (r *MySQLClientPaymentRepository) SetReceiptNumber(ctx context.Context, projectID, id int64, number, period string, seq int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE client_payments SET receipt_number = ?, receipt_period = ?, receipt_seq = ? WHERE project_id = ? AND id = ?`,
		number, period, seq, projectID, id,
	)
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return domain.ErrDuplicateReceiptNumber
	}
	return err
}
