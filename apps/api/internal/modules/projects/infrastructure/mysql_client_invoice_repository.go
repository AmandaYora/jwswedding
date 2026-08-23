package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"

	"jwswedding/internal/modules/projects/domain"
)

type MySQLClientInvoiceRepository struct {
	db *sql.DB
}

func NewMySQLClientInvoiceRepository(db *sql.DB) *MySQLClientInvoiceRepository {
	return &MySQLClientInvoiceRepository{db: db}
}

const clientInvoiceColumns = `id, project_id, invoice_number, number_period, number_seq, type, description,
	amount, due_date, status, client_payment_id, created_by_staff_id, created_at, updated_at`

func scanClientInvoice(scan func(dest ...interface{}) error) (*domain.ClientInvoice, error) {
	var inv domain.ClientInvoice
	var invType, status string
	err := scan(
		&inv.ID, &inv.ProjectID, &inv.InvoiceNumber, &inv.NumberPeriod, &inv.NumberSeq, &invType, &inv.Description,
		&inv.Amount, &inv.DueDate, &status, &inv.ClientPaymentID, &inv.CreatedByStaffID, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	inv.Type = domain.PaymentType(invType)
	inv.Status = domain.InvoiceStatus(status)
	return &inv, nil
}

func (r *MySQLClientInvoiceRepository) ListByProject(ctx context.Context, projectID int64) ([]domain.ClientInvoice, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+clientInvoiceColumns+` FROM client_invoices WHERE project_id = ? ORDER BY due_date DESC, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ClientInvoice
	for rows.Next() {
		inv, err := scanClientInvoice(rows.Scan)
		if err != nil {
			return nil, err
		}
		list = append(list, *inv)
	}
	return list, rows.Err()
}

func (r *MySQLClientInvoiceRepository) FindByID(ctx context.Context, projectID, id int64) (*domain.ClientInvoice, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+clientInvoiceColumns+` FROM client_invoices WHERE project_id = ? AND id = ? LIMIT 1`, projectID, id)
	return scanClientInvoice(row.Scan)
}

// FindByClientPaymentID backs two callers (PLAN.md invoice-kwitansi-client
// §4.2): resolving an Invoice's number for the Kwitansi PDF, and the guard
// against deleting/editing a ClientPayment that a Tagihan is tracking as its
// pelunasan. Uses idx_client_invoices_payment.
func (r *MySQLClientInvoiceRepository) FindByClientPaymentID(ctx context.Context, projectID, clientPaymentID int64) (*domain.ClientInvoice, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+clientInvoiceColumns+` FROM client_invoices WHERE project_id = ? AND client_payment_id = ? LIMIT 1`,
		projectID, clientPaymentID)
	return scanClientInvoice(row.Scan)
}

func (r *MySQLClientInvoiceRepository) Create(ctx context.Context, inv *domain.ClientInvoice) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO client_invoices
		 (project_id, invoice_number, number_period, number_seq, type, description, amount, due_date, status, client_payment_id, created_by_staff_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ProjectID, inv.InvoiceNumber, inv.NumberPeriod, inv.NumberSeq, string(inv.Type), inv.Description,
		inv.Amount, inv.DueDate, string(inv.Status), inv.ClientPaymentID, inv.CreatedByStaffID,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return domain.ErrDuplicateInvoiceNumber
		}
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	inv.ID = id
	return nil
}

// Update overwrites every mutable field — invoice_number/number_period/
// number_seq are never touched here (ClientInvoiceService.Update never
// changes them; MarkPaid/UnmarkPaid update status/client_payment_id via this
// same method, passing the rest unchanged).
func (r *MySQLClientInvoiceRepository) Update(ctx context.Context, projectID, id int64, inv domain.ClientInvoice) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE client_invoices SET type = ?, description = ?, amount = ?, due_date = ?, status = ?, client_payment_id = ?
		 WHERE project_id = ? AND id = ?`,
		string(inv.Type), inv.Description, inv.Amount, inv.DueDate, string(inv.Status), inv.ClientPaymentID, projectID, id,
	)
	return err
}

func (r *MySQLClientInvoiceRepository) Delete(ctx context.Context, projectID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM client_invoices WHERE project_id = ? AND id = ?`, projectID, id)
	return err
}

// NextInvoiceSequence returns MAX(number_seq)+1 for (tenant, period) — see
// MySQLClientPaymentRepository.NextReceiptSequence for the identical idiom
// and same-module-join reasoning.
func (r *MySQLClientInvoiceRepository) NextInvoiceSequence(ctx context.Context, tenantID int64, period string) (int, error) {
	var maxSeq sql.NullInt64
	row := r.db.QueryRowContext(ctx,
		`SELECT MAX(ci.number_seq) FROM client_invoices ci
		 JOIN projects p ON p.id = ci.project_id
		 WHERE p.tenant_id = ? AND ci.number_period = ?`, tenantID, period)
	if err := row.Scan(&maxSeq); err != nil {
		return 0, err
	}
	return int(maxSeq.Int64) + 1, nil
}
