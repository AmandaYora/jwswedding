package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"

	"jwswedding/internal/modules/quotations/domain"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/utils"
)

type MySQLQuotationRepository struct {
	db *sql.DB
}

func NewMySQLQuotationRepository(db *sql.DB) *MySQLQuotationRepository {
	return &MySQLQuotationRepository{db: db}
}

const quotationColumns = `id, tenant_id, client_id, po_number, number_period, number_seq, revision, base_price,
	package_name, terms_text, bonus_note, status, event_date, pax, venue_id, snapshot_json, issued_at, accepted_at,
	created_by_staff_id, created_at, updated_at`

// scanQuotation memetakan kolom NULL-able ke bentuk sentinel: po_number/
// number_period "" dan number_seq 0 sampai Issue pertama; event_date/venue/
// snapshot/issued/accepted nil bila belum ada.
func scanQuotation(scan func(dest ...interface{}) error) (*domain.Quotation, error) {
	var o domain.Quotation
	var poNumber, numberPeriod sql.NullString
	var numberSeq sql.NullInt64
	var snapshot []byte
	var status string
	var eventDate sql.NullTime
	var venueID sql.NullInt64
	var issuedAt, acceptedAt sql.NullTime

	err := scan(&o.ID, &o.TenantID, &o.ClientID, &poNumber, &numberPeriod, &numberSeq, &o.Revision, &o.BasePrice,
		&o.PackageName, &o.TermsText, &o.BonusNote, &status, &eventDate, &o.Pax, &venueID, &snapshot, &issuedAt, &acceptedAt,
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
	o.Status = domain.QuotationStatus(status)
	if eventDate.Valid {
		o.EventDate = &eventDate.Time
	}
	if venueID.Valid {
		o.VenueID = &venueID.Int64
	}
	if issuedAt.Valid {
		o.IssuedAt = &issuedAt.Time
	}
	if acceptedAt.Valid {
		o.AcceptedAt = &acceptedAt.Time
	}
	if len(snapshot) > 0 {
		var s domain.QuotationSnapshot
		if err := json.Unmarshal(snapshot, &s); err != nil {
			return nil, err
		}
		o.Snapshot = &s
	}
	return &o, nil
}

func (r *MySQLQuotationRepository) FindByID(ctx context.Context, tenantID, id int64) (*domain.Quotation, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+quotationColumns+` FROM quotations WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	return scanQuotation(row.Scan)
}

// ListByTenant menopang halaman daftar Penawaran: filter status + cari nomor
// + saring client (dropdown Tambah Project: client + Ditawarkan). Pencarian
// nama client TIDAK di sini — frontend memakainya dari CoupleNamesBatch satu
// halaman (§11). Paginasi via shared/pagination.
func (r *MySQLQuotationRepository) ListByTenant(ctx context.Context, tenantID int64, status string, params pagination.Params, search string, clientID int64) ([]domain.Quotation, int64, error) {
	countQuery := `SELECT COUNT(*) FROM quotations WHERE tenant_id = ?`
	listQuery := `SELECT ` + quotationColumns + ` FROM quotations WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	if status != "" {
		countQuery += ` AND status = ?`
		listQuery += ` AND status = ?`
		args = append(args, status)
	}
	if clientID != 0 {
		countQuery += ` AND client_id = ?`
		listQuery += ` AND client_id = ?`
		args = append(args, clientID)
	}
	if search != "" {
		countQuery += ` AND po_number LIKE ?`
		listQuery += ` AND po_number LIKE ?`
		args = append(args, "%"+search+"%")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, listQuery, append(args, params.Limit, params.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []domain.Quotation{}
	for rows.Next() {
		o, err := scanQuotation(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *o)
	}
	return list, total, rows.Err()
}

// ListIDsByClient menopang QuotationCleaner (impact + sapu) dan dialog Tambah
// Project (dropdown penawaran Ditawarkan per client).
func (r *MySQLQuotationRepository) ListIDsByClient(ctx context.Context, tenantID, clientID int64, statuses ...string) ([]domain.Quotation, error) {
	query := `SELECT ` + quotationColumns + ` FROM quotations WHERE tenant_id = ? AND client_id = ?`
	args := []interface{}{tenantID, clientID}
	if len(statuses) > 0 {
		query += ` AND status IN (`
		for i, st := range statuses {
			if i > 0 {
				query += `,`
			}
			query += `?`
			args = append(args, st)
		}
		query += `)`
	}
	query += ` ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []domain.Quotation{}
	for rows.Next() {
		o, err := scanQuotation(rows.Scan)
		if err != nil {
			return nil, err
		}
		list = append(list, *o)
	}
	return list, rows.Err()
}

func (r *MySQLQuotationRepository) Create(ctx context.Context, o *domain.Quotation) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO quotations
		 (tenant_id, client_id, base_price, package_name, terms_text, bonus_note, status, event_date, pax, venue_id, created_by_staff_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.TenantID, o.ClientID, o.BasePrice, o.PackageName, o.TermsText, o.BonusNote, o.Status, o.EventDate, o.Pax, o.VenueID, o.CreatedByStaffID,
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

// Update menulis seluruh kolom mutable. Kolom penomoran disertakan karena
// Issue yang mengisinya — pemanggil bertanggung jawab hanya mengisinya sekali
// (lihat Quotation.IsNumbered).
func (r *MySQLQuotationRepository) Update(ctx context.Context, o *domain.Quotation) error {
	var snapshot []byte
	var err error
	if o.Snapshot != nil {
		if snapshot, err = json.Marshal(o.Snapshot); err != nil {
			return err
		}
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE quotations SET
		   client_id = ?, po_number = ?, number_period = ?, number_seq = ?, revision = ?, base_price = ?,
		   package_name = ?, terms_text = ?, bonus_note = ?, status = ?, event_date = ?, pax = ?, venue_id = ?,
		   snapshot_json = ?, issued_at = ?, accepted_at = ?
		 WHERE tenant_id = ? AND id = ?`,
		o.ClientID, nullIfEmpty(o.PONumber), nullIfEmpty(o.NumberPeriod), nullIfZero(o.NumberSeq), o.Revision, o.BasePrice,
		o.PackageName, o.TermsText, o.BonusNote, o.Status, o.EventDate, o.Pax, o.VenueID,
		snapshot, o.IssuedAt, o.AcceptedAt, o.TenantID, o.ID,
	)
	return err
}

// Delete menghapus satu penawaran — blok + penyesuaian ikut lewat FK CASCADE
// (satu modul, FK nyata). Dipakai hapus langsung (belum Diterima) maupun
// cascade project/client.
func (r *MySQLQuotationRepository) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM quotations WHERE tenant_id = ? AND id = ?`, tenantID, id)
	return err
}

// DeleteForClient menyapu SELURUH penawaran milik satu client (penghapusan
// Client berjenjang, T3.5) — satu pernyataan, CASCADE membersihkan anaknya.
//
// Tanpa filter status: yang masih punya project sudah lenyap lebih dulu lewat
// cascade project, jadi yang tersisa di sini justru yang tidak punya rumah
// lain — termasuk penawaran Diterima yang project-nya sudah dihapus. Filter
// `status <> 'Diterima'` akan meninggalkan baris yatim itu selamanya.
func (r *MySQLQuotationRepository) DeleteForClient(ctx context.Context, tenantID, clientID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM quotations WHERE tenant_id = ? AND client_id = ?`, tenantID, clientID)
	return err
}

// DistinctCategories mengumpulkan kategori blok yang sudah pernah dipakai
// tenant ini — sumber datalist kategori (T4.3/D19). Join ke `quotations` masih
// di dalam modul yang sama, jadi bukan join lintas modul. DISTINCT atas kolom
// pendek dengan filter tenant, dipanggil sekali saat editor dibuka.
func (r *MySQLQuotationRepository) DistinctCategories(ctx context.Context, tenantID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT b.category FROM quotation_blocks b
		 JOIN quotations q ON q.id = b.quotation_id
		 WHERE q.tenant_id = ? AND b.category <> '' ORDER BY b.category`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AdjustmentTotals menjumlahkan penyesuaian untuk SEKUMPULAN penawaran dalam
// satu query GROUP BY — dipakai daftar Penawaran supaya bisa menampilkan total
// (harga awal + penyesuaian) tanpa satu query per baris.
func (r *MySQLQuotationRepository) AdjustmentTotals(ctx context.Context, quotationIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(quotationIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT quotation_id, COALESCE(SUM(amount), 0) FROM quotation_adjustments
		 WHERE quotation_id IN (`+utils.Placeholders(len(quotationIDs))+`) GROUP BY quotation_id`,
		utils.Int64Args(quotationIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, sum int64
		if err := rows.Scan(&id, &sum); err != nil {
			return nil, err
		}
		out[id] = sum
	}
	return out, rows.Err()
}

// NextPOSequence mengembalikan MAX(number_seq)+1 per (tenant, period) —
// langsung dari kolom tenant_id milik tabel ini (tanpa join: dulu butuh join
// ke projects karena tabel lama tidak punya tenant_id). Baris Draft bernomor
// NULL dan dilewati MAX dengan sendirinya — penawaran yang tidak closing
// tidak menghabiskan nomor sampai dikirim (D7).
func (r *MySQLQuotationRepository) NextPOSequence(ctx context.Context, tenantID int64, period string) (int, error) {
	var maxSeq sql.NullInt64
	row := r.db.QueryRowContext(ctx,
		`SELECT MAX(number_seq) FROM quotations WHERE tenant_id = ? AND number_period = ?`, tenantID, period)
	if err := row.Scan(&maxSeq); err != nil {
		return 0, err
	}
	return int(maxSeq.Int64) + 1, nil
}

func (r *MySQLQuotationRepository) ListBlocks(ctx context.Context, quotationID int64) ([]domain.QuotationBlock, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, quotation_id, category, body, qty_text, bonus_note, sort_order
		 FROM quotation_blocks WHERE quotation_id = ? ORDER BY sort_order, id`, quotationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := []domain.QuotationBlock{}
	for rows.Next() {
		var b domain.QuotationBlock
		if err := rows.Scan(&b.ID, &b.QuotationID, &b.Category, &b.Body, &b.QtyText, &b.BonusNote, &b.SortOrder); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

// ReplaceBlocks menimpa seluruh komposisi dalam satu transaksi.
func (r *MySQLQuotationRepository) ReplaceBlocks(ctx context.Context, quotationID int64, blocks []domain.QuotationBlock) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM quotation_blocks WHERE quotation_id = ?`, quotationID); err != nil {
		return err
	}
	for i, b := range blocks {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO quotation_blocks (quotation_id, category, body, qty_text, bonus_note, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			quotationID, b.Category, b.Body, b.QtyText, b.BonusNote, i+1,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *MySQLQuotationRepository) ListAdjustments(ctx context.Context, quotationID int64) ([]domain.QuotationAdjustment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, quotation_id, description, amount, sort_order
		 FROM quotation_adjustments WHERE quotation_id = ? ORDER BY sort_order, id`, quotationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	adjustments := []domain.QuotationAdjustment{}
	for rows.Next() {
		var a domain.QuotationAdjustment
		if err := rows.Scan(&a.ID, &a.QuotationID, &a.Description, &a.Amount, &a.SortOrder); err != nil {
			return nil, err
		}
		adjustments = append(adjustments, a)
	}
	return adjustments, rows.Err()
}

func (r *MySQLQuotationRepository) ReplaceAdjustments(ctx context.Context, quotationID int64, adjustments []domain.QuotationAdjustment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM quotation_adjustments WHERE quotation_id = ?`, quotationID); err != nil {
		return err
	}
	for i, a := range adjustments {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO quotation_adjustments (quotation_id, description, amount, sort_order)
			 VALUES (?, ?, ?, ?)`,
			quotationID, a.Description, a.Amount, i+1,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// nullIfEmpty/nullIfZero menjaga split sentinel-di-Go, NULL-di-MySQL: Draft
// menulis NULL nyata supaya UNIQUE(po_number) menoleransi banyak baris (D7)
// — menulis "" justru bertabrakan di penawaran kedua.
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
