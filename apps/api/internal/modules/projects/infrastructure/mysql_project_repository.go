package infrastructure

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/pagination"
	"jwswedding/internal/shared/utils"
)

type MySQLProjectRepository struct {
	db *sql.DB
}

func NewMySQLProjectRepository(db *sql.DB) *MySQLProjectRepository {
	return &MySQLProjectRepository{db: db}
}

const projectColumns = `id, tenant_id, client_id, quotation_id, name, bride_name, groom_name, event_date, event_start_time, event_end_time, pax, venue, venue_id, venue_rental_price, venue_charge,
	prep_start_date, package_name, contract_value, status, pic_staff_id, pic_sales_staff_id, description, is_archived, created_at, updated_at`

func scanProject(scan func(dest ...interface{}) error) (*domain.Project, error) {
	var p domain.Project
	var status string
	var description sql.NullString
	var clientID, quotationID sql.NullInt64
	var venueID sql.NullInt64
	var venueRentalPrice, venueCharge sql.NullInt64
	var eventStartTime, eventEndTime sql.NullString
	err := scan(&p.ID, &p.TenantID, &clientID, &quotationID, &p.Name, &p.BrideName, &p.GroomName, &p.EventDate, &eventStartTime, &eventEndTime, &p.Pax, &p.Venue, &venueID, &venueRentalPrice, &venueCharge,
		&p.PrepStartDate, &p.PackageName, &p.ContractValue, &status, &p.PICStaffID, &p.PICSalesStaffID, &description, &p.IsArchived, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Status = domain.ProjectStatus(status)
	p.Description = description.String
	if clientID.Valid {
		p.ClientID = clientID.Int64
	}
	if quotationID.Valid {
		p.QuotationID = quotationID.Int64
	}
	if venueID.Valid {
		p.VenueID = &venueID.Int64
	}
	if venueRentalPrice.Valid {
		p.VenueRentalPrice = &venueRentalPrice.Int64
	}
	if venueCharge.Valid {
		p.VenueCharge = &venueCharge.Int64
	}
	// TIME columns arrive as "15:04:05"; normalize to the "HH:MM" shape the
	// domain/API boundary carries -- same treatment as project_vendors' Jam Acara.
	if eventStartTime.Valid {
		v := timeToHHMM(eventStartTime.String)
		p.EventStartTime = &v
	}
	if eventEndTime.Valid {
		v := timeToHHMM(eventEndTime.String)
		p.EventEndTime = &v
	}
	return &p, nil
}

// picStaffID, when non-nil, scopes the result to a single PIC's own projects
// — the Wedding Planner ("Staff" role) role restriction; picSalesStaffID,
// when non-nil, scopes it to a single Sales staff member's own projects
// instead (mutually exclusive with picStaffID in practice -- a caller is
// either role). nil/nil means every project in the tenant (Owner/Admin, and
// internal callers like Dashboard).
func (r *MySQLProjectRepository) List(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64) ([]domain.Project, error) {
	query := `SELECT ` + projectColumns + ` FROM projects WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	if picStaffID != nil {
		query += ` AND pic_staff_id = ?`
		args = append(args, *picStaffID)
	}
	if picSalesStaffID != nil {
		query += ` AND pic_sales_staff_id = ?`
		args = append(args, *picSalesStaffID)
	}
	query += ` ORDER BY event_date DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	return projects, rows.Err()
}

// ListByVenueID backs the hard-delete "impact" endpoint for Venue (PLAN.md) —
// every project this venue is currently attached to (projects.venue_id has no
// SQL FK, cross-module by design, so this is the one place that can answer
// "what still points at this venue").
func (r *MySQLProjectRepository) ListByVenueID(ctx context.Context, tenantID, venueID int64) ([]domain.ProjectRef, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM projects WHERE tenant_id = ? AND venue_id = ?`, tenantID, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []domain.ProjectRef
	for rows.Next() {
		var ref domain.ProjectRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// ListByStaffPIC backs the hard-delete "impact" endpoint for Staff (PLAN.md)
// — every project this staff member is a live PIC assignment on, across the
// four columns that represent an active "responsible person" role
// (projects.pic_staff_id, project_vendors.pic_staff_id,
// vendor_milestones.pic_staff_id, vendor_issues.pic_staff_id). Deliberately
// excludes activity_log.actor_staff_id/evidence.uploaded_by_staff_id — those
// are immutable historical audit trail, not a live assignment, so a staff
// member who merely triggered one of those years ago must not block their
// own deletion (see PLAN.md's reasoning). UNION already de-duplicates a
// project referenced through more than one of the four columns.
func (r *MySQLProjectRepository) ListByStaffPIC(ctx context.Context, tenantID, staffID int64) ([]domain.ProjectRef, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.name FROM projects p WHERE p.tenant_id = ? AND p.pic_staff_id = ?
		 UNION
		 SELECT p.id, p.name FROM project_vendors pv JOIN projects p ON p.id = pv.project_id
		   WHERE p.tenant_id = ? AND pv.pic_staff_id = ?
		 UNION
		 SELECT p.id, p.name FROM vendor_milestones vm
		   JOIN project_vendors pv ON pv.id = vm.project_vendor_id
		   JOIN projects p ON p.id = pv.project_id
		   WHERE p.tenant_id = ? AND vm.pic_staff_id = ?
		 UNION
		 SELECT p.id, p.name FROM vendor_issues vi JOIN projects p ON p.id = vi.project_id
		   WHERE p.tenant_id = ? AND vi.pic_staff_id = ?`,
		tenantID, staffID, tenantID, staffID, tenantID, staffID, tenantID, staffID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []domain.ProjectRef
	for rows.Next() {
		var ref domain.ProjectRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// CountAll backs the dashboard's TotalProjects stat — a plain COUNT(*)
// instead of loading every row just to take len() (PLAN.md "Performance
// remediation").
func (r *MySQLProjectRepository) CountAll(ctx context.Context, tenantID int64) (int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE tenant_id = ?`, tenantID).Scan(&total)
	return total, err
}

// ListForDashboard backs every other dashboard computation (trends,
// near-D-day, upcoming, lagging) — bounded to rows that could actually
// matter for any of them, instead of List's entire unbounded tenant history
// (PLAN.md "Performance remediation"): a project still open (non-terminal
// status) regardless of age, or a project whose created_at/event_date falls
// within the trend window (`since`, the earliest month ProjectTrend/
// RevenueTrend can ever display). A terminal-status project older than
// `since` on both dates can't affect any of the four computations, so it's
// excluded at the query level rather than fetched and then ignored in Go.
func (r *MySQLProjectRepository) ListForDashboard(ctx context.Context, tenantID int64, since time.Time) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+projectColumns+` FROM projects
		 WHERE tenant_id = ? AND (status NOT IN ('Completed', 'Cancelled') OR created_at >= ? OR event_date >= ?)
		 ORDER BY event_date DESC, id DESC`,
		tenantID, since, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	return projects, rows.Err()
}

// eventMonthRange converts a validated "YYYY-MM" string into the half-open
// [start, end) bounds of that calendar month, as plain "YYYY-MM-DD" date
// strings.
//
// Date STRINGS, not time.Time, deliberately: go-sql-driver writes a time.Time
// argument as v.In(cfg.Loc), and this pool is opened with loc=Local (see
// shared/database.Open). A time.Time parsed from "2006-01" carries UTC
// midnight, so on a UTC+7 host the driver would put "2026-09-01 07:00:00" on
// the wire -- and against a DATE column (implicitly 00:00:00) that silently
// drops every event on the 1st of the filtered month while pulling in the 1st
// of the next one. A date string is compared by MySQL as a DATE, with no
// timezone conversion anywhere in the path.
func eventMonthRange(eventMonth string) (start, end string, err error) {
	t, err := time.Parse("2006-01", eventMonth)
	if err != nil {
		return "", "", err
	}
	return t.Format("2006-01-02"), t.AddDate(0, 1, 0).Format("2006-01-02"), nil
}

// ListPaginated backs the real `GET /projects` list page — List above stays
// as-is for dashboard/global-search consumers that need the full roster
// (including archived projects; this method's showArchived split doesn't
// apply there — see ProjectService.ListPaginated's doc comment).
//
// eventMonth (docs/plan/revisi-putri-lanjutan/PLAN.md Blok I, D12/D7), when
// non-nil, is a validated "YYYY-MM" string scoping the result to that
// calendar month of event_date. picStaffID/picSalesStaffID need no companion
// parameter to support this feature (D6/T-2): the slots below already exist
// for role-scoping, and the presentation layer fills them with the right
// value per role/query-param.
func (r *MySQLProjectRepository) ListPaginated(ctx context.Context, tenantID int64, picStaffID, picSalesStaffID *int64, params pagination.Params, search, status string, showArchived bool, eventMonth *string) ([]domain.Project, int64, error) {
	countQuery := `SELECT COUNT(*) FROM projects WHERE tenant_id = ? AND is_archived = ?`
	listQuery := `SELECT ` + projectColumns + ` FROM projects WHERE tenant_id = ? AND is_archived = ?`
	args := []interface{}{tenantID, showArchived}
	if picStaffID != nil {
		countQuery += ` AND pic_staff_id = ?`
		listQuery += ` AND pic_staff_id = ?`
		args = append(args, *picStaffID)
	}
	if picSalesStaffID != nil {
		countQuery += ` AND pic_sales_staff_id = ?`
		listQuery += ` AND pic_sales_staff_id = ?`
		args = append(args, *picSalesStaffID)
	}
	var conditions []string
	if search != "" {
		conditions = append(conditions, `(name LIKE ? OR bride_name LIKE ? OR groom_name LIKE ? OR venue LIKE ?)`)
		like := "%" + search + "%"
		args = append(args, like, like, like, like)
	}
	if status != "" {
		conditions = append(conditions, `status = ?`)
		args = append(args, status)
	}
	if eventMonth != nil {
		// Half-open range, not YEAR()=? AND MONTH()=? -- the latter wraps the
		// column in a function and disables any index on event_date (§8).
		start, end, err := eventMonthRange(*eventMonth)
		if err != nil {
			return nil, 0, err
		}
		conditions = append(conditions, `event_date >= ? AND event_date < ?`)
		args = append(args, start, end)
	}
	if len(conditions) > 0 {
		where := ` AND ` + strings.Join(conditions, " AND ")
		countQuery += where
		listQuery += where
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += ` ORDER BY event_date DESC, id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, listQuery, append(args, params.Limit, params.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		projects = append(projects, *p)
	}
	return projects, total, rows.Err()
}

func (r *MySQLProjectRepository) FindByID(ctx context.Context, tenantID, id int64) (*domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	return scanProject(row.Scan)
}

func (r *MySQLProjectRepository) Create(ctx context.Context, p *domain.Project) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO projects (tenant_id, client_id, quotation_id, name, bride_name, groom_name, event_date, event_start_time, event_end_time, pax, venue, prep_start_date,
		 package_name, contract_value, status, pic_staff_id, pic_sales_staff_id, description)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.TenantID, nullInt(p.ClientID), nullInt(p.QuotationID), p.Name, p.BrideName, p.GroomName, p.EventDate, p.EventStartTime, p.EventEndTime, p.Pax, p.Venue, p.PrepStartDate,
		p.PackageName, p.ContractValue, string(p.Status), p.PICStaffID, p.PICSalesStaffID, p.Description,
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

func (r *MySQLProjectRepository) Update(ctx context.Context, p *domain.Project) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, bride_name = ?, groom_name = ?, event_date = ?, event_start_time = ?, event_end_time = ?, pax = ?, venue = ?, venue_id = ?, venue_rental_price = ?,
		 venue_charge = ?, prep_start_date = ?, package_name = ?, contract_value = ?, status = ?, pic_staff_id = ?, pic_sales_staff_id = ?, description = ?
		 WHERE tenant_id = ? AND id = ?`,
		p.Name, p.BrideName, p.GroomName, p.EventDate, p.EventStartTime, p.EventEndTime, p.Pax, p.Venue, p.VenueID, p.VenueRentalPrice,
		p.VenueCharge, p.PrepStartDate, p.PackageName, p.ContractValue, string(p.Status), p.PICStaffID, p.PICSalesStaffID, p.Description, p.TenantID, p.ID,
	)
	return err
}

// nullInt maps the 0 sentinel ("belum ada relasi") to SQL NULL — client_id
// and quotation_id are NULL-able precisely so pre-cutover rows and
// not-yet-linked rows stay distinguishable from a real id.
func nullInt(id int64) interface{} {
	if id == 0 {
		return nil
	}
	return id
}

// ListByClient backs ProjectDirectory (T1.7): every project of one client —
// the only join-free way to answer "client ini punya project apa saja".
func (r *MySQLProjectRepository) ListByClient(ctx context.Context, tenantID, clientID int64) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE tenant_id = ? AND client_id = ? ORDER BY event_date DESC, id DESC`, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	return projects, rows.Err()
}

// FindByQuotationID backs Accept idempotency (T3.2): a retry after a partial
// failure finds the already-born project instead of making a second one.
func (r *MySQLProjectRepository) FindByQuotationID(ctx context.Context, tenantID, quotationID int64) (*domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE tenant_id = ? AND quotation_id = ? LIMIT 1`, tenantID, quotationID)
	return scanProject(row.Scan)
}

// CountByClients menjawab hitungan project per client untuk satu halaman
// daftar Client — satu query agregat, bukan N+1.
func (r *MySQLProjectRepository) CountByClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]int, error) {
	out := make(map[int64]int, len(clientIDs))
	if len(clientIDs) == 0 {
		return out, nil
	}
	query := `SELECT client_id, COUNT(*) FROM projects WHERE tenant_id = ? AND client_id IN (`
	args := []interface{}{tenantID}
	for i, id := range clientIDs {
		if i > 0 {
			query += `,`
		}
		query += `?`
		args = append(args, id)
	}
	query += `) GROUP BY client_id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// ProjectRefsForQuotations memetakan penawaran ke project-nya beserta PIC
// Wedding Planner-nya untuk satu halaman daftar Penawaran — satu query atas
// UNIQUE(quotation_id), bukan N+1 (PLAN wording-role-dan-filter-sales-wp §5.3).
func (r *MySQLProjectRepository) ProjectRefsForQuotations(ctx context.Context, tenantID int64, quotationIDs []int64) (map[int64]domain.QuotationProjectRef, error) {
	out := make(map[int64]domain.QuotationProjectRef, len(quotationIDs))
	if len(quotationIDs) == 0 {
		return out, nil
	}
	query := `SELECT quotation_id, id, pic_staff_id FROM projects WHERE tenant_id = ? AND quotation_id IN (`
	args := []interface{}{tenantID}
	for i, id := range quotationIDs {
		if i > 0 {
			query += `,`
		}
		query += `?`
		args = append(args, id)
	}
	query += `)`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var qid, pid, pic int64
		if err := rows.Scan(&qid, &pid, &pic); err != nil {
			return nil, err
		}
		out[qid] = domain.QuotationProjectRef{ProjectID: pid, PICStaffID: pic}
	}
	return out, rows.Err()
}

// QuotationIDsForPICStaff menjawab penawaran mana yang sudah melahirkan
// project yang dipegang satu Wedding Planner — dasar filter WP di daftar
// Penawaran (PLAN wording-role-dan-filter-sales-wp §5.3). quotation_id
// NULLABLE sehingga baris tanpa penawaran wajib dikecualikan eksplisit.
func (r *MySQLProjectRepository) QuotationIDsForPICStaff(ctx context.Context, tenantID, picStaffID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT quotation_id FROM projects WHERE tenant_id = ? AND pic_staff_id = ? AND quotation_id IS NOT NULL`, tenantID, picStaffID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var qid int64
		if err := rows.Scan(&qid); err != nil {
			return nil, err
		}
		out = append(out, qid)
	}
	return out, rows.Err()
}

// ClientIDsForPIC menjawab client mana yang punya project yang dipegang PIC
// tertentu — dasar filter Sales/WP di daftar Client (PLAN
// wording-role-dan-filter-sales-wp §5.3). 0 berarti "tidak memfilter" untuk
// slot itu; keduanya terisi berarti AND.
func (r *MySQLProjectRepository) ClientIDsForPIC(ctx context.Context, tenantID, picStaffID, picSalesStaffID int64) ([]int64, error) {
	query := `SELECT DISTINCT client_id FROM projects WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	if picStaffID != 0 {
		query += ` AND pic_staff_id = ?`
		args = append(args, picStaffID)
	}
	if picSalesStaffID != 0 {
		query += ` AND pic_sales_staff_id = ?`
		args = append(args, picSalesStaffID)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ProjectIDsForPICStaff menjawab project mana saja yang dipegang satu Wedding
// Planner — dasar penyaringan daftar Rundown (PLAN rundown-generator §9.1).
// Sepadan dengan QuotationIDsForPICStaff, hanya memproyeksikan project-nya
// sendiri. Beralas idx_projects_tenant dan kolom pic_staff_id.
func (r *MySQLProjectRepository) ProjectIDsForPICStaff(ctx context.Context, tenantID, picStaffID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM projects WHERE tenant_id = ? AND pic_staff_id = ? ORDER BY id`,
		tenantID, picStaffID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// PICsForClients menjawab himpunan PIC berbeda dari seluruh project milik
// tiap client pada satu halaman daftar Client — satu query, bukan N+1 (PLAN
// wording-role-dan-filter-sales-wp §5.3, pola yang sama dengan
// CountByClients di atas).
func (r *MySQLProjectRepository) PICsForClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]domain.ClientPICs, error) {
	out := make(map[int64]domain.ClientPICs, len(clientIDs))
	if len(clientIDs) == 0 {
		return out, nil
	}
	query := `SELECT DISTINCT client_id, pic_staff_id, pic_sales_staff_id FROM projects WHERE tenant_id = ? AND client_id IN (`
	args := []interface{}{tenantID}
	for i, id := range clientIDs {
		if i > 0 {
			query += `,`
		}
		query += `?`
		args = append(args, id)
	}
	query += `)`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seenWP := make(map[int64]map[int64]bool, len(clientIDs))
	seenSales := make(map[int64]map[int64]bool, len(clientIDs))
	for rows.Next() {
		var clientID, pic, picSales int64
		if err := rows.Scan(&clientID, &pic, &picSales); err != nil {
			return nil, err
		}
		entry := out[clientID]
		if seenWP[clientID] == nil {
			seenWP[clientID] = make(map[int64]bool)
		}
		if seenSales[clientID] == nil {
			seenSales[clientID] = make(map[int64]bool)
		}
		if !seenWP[clientID][pic] {
			seenWP[clientID][pic] = true
			entry.PICStaffIDs = append(entry.PICStaffIDs, pic)
		}
		if !seenSales[clientID][picSales] {
			seenSales[clientID][picSales] = true
			entry.PICSalesStaffIDs = append(entry.PICSalesStaffIDs, picSales)
		}
		out[clientID] = entry
	}
	return out, rows.Err()
}

func (r *MySQLProjectRepository) SetStatus(ctx context.Context, tenantID, id int64, status domain.ProjectStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE projects SET status = ? WHERE tenant_id = ? AND id = ?`, string(status), tenantID, id)
	return err
}

func (r *MySQLProjectRepository) SetArchived(ctx context.Context, tenantID, id int64, archived bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE projects SET is_archived = ? WHERE tenant_id = ? AND id = ?`, archived, tenantID, id)
	return err
}

// CreateSeeded menyisipkan project + Timeline Default dalam SATU transaksi
// (jalur Accept, T3.2). Bentuk INSERT tiap tabel disalin dari Create milik
// repo masing-masing (lihat Create di atas dan sisipan milestone di bawah) —
// bila salah satunya berubah, perbarui juga di sini; TestAccept_* mengunci
// perilakunya dari sisi service.
//
// Tagihan TIDAK lagi ikut disemai: rencana termin di penawaran dihapus, jadi
// tidak ada jadwal yang bisa diterjemahkan menjadi tagihan. Project lahir
// tanpa tagihan sama sekali, dan staff menerbitkannya sendiri sesuai cicilan
// yang benar-benar disepakati.
//
// Penomoran milestone dihitung pemanggil SEBELUM transaksi — di dalam satu
// Accept sekuensial tidak ada penulis lain yang bisa menyela, dan tabrakan
// sisa (pekerja konkuren) ditangani idempotensi FindByQuotationID di sisi
// service.
func (r *MySQLProjectRepository) CreateSeeded(ctx context.Context, p *domain.Project, milestones []domain.ProjectMilestone) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`INSERT INTO projects (tenant_id, client_id, quotation_id, name, bride_name, groom_name, event_date, event_start_time, event_end_time, pax, venue, venue_id, venue_rental_price, venue_charge, prep_start_date,
		 package_name, contract_value, status, pic_staff_id, pic_sales_staff_id, description)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.TenantID, nullInt(p.ClientID), nullInt(p.QuotationID), p.Name, p.BrideName, p.GroomName, p.EventDate, p.EventStartTime, p.EventEndTime, p.Pax, p.Venue, p.VenueID, p.VenueRentalPrice, p.VenueCharge, p.PrepStartDate,
		p.PackageName, p.ContractValue, string(p.Status), p.PICStaffID, p.PICSalesStaffID, p.Description,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id

	for i := range milestones {
		m := &milestones[i]
		m.ProjectID = id
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project_milestones (project_id, sort_order, name, category, status, target_date) VALUES (?, ?, ?, ?, ?, ?)`,
			id, m.SortOrder, m.Name, m.Category, string(m.Status), m.TargetDate,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteCascade permanently removes a project and every row across this
// module's 8 sub-entity tables that references it, in one transaction —
// see ADR-0013. Deletion order matters: vendor_payments references evidence
// (invoice_evidence_id/proof_evidence_id), so it must go first; everything
// that references project_vendors must go before project_vendors itself;
// everything goes before the projects row. client_payments has its own
// direct FK straight to projects (no project_vendor_id, unlike
// vendor_payments) so it has no ordering dependency on anything else here —
// it just needs to go before the final projects delete. Evidence's
// object-storage files are NOT touched here — the caller
// (ProjectService.Delete) deletes those only after this transaction
// commits, so a storage failure never leaves a dangling DB reference (see
// its own doc comment).
func (r *MySQLProjectRepository) DeleteCascade(ctx context.Context, tenantID, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_log WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM vendor_payments WHERE project_id = ?`, id); err != nil {
		return err
	}
	// client_invoices has fk_client_invoices_project (PLAN.md
	// invoice-kwitansi-client) -- must go before client_payments below, and
	// before the final projects delete, or DELETE FROM projects fails FK 1451
	// for any project with at least one invoice.
	if _, err := tx.ExecContext(ctx, `DELETE FROM client_invoices WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM client_payments WHERE project_id = ?`, id); err != nil {
		return err
	}
	// venue_payments has fk_venue_payments_project
	// (000030_create_venue_payments_table.up.sql:21) but was never registered
	// here -- pre-existing bug (PLAN.md invoice-kwitansi-client §1.12), fixed
	// alongside this cascade while touching this exact function for the
	// Invoice feature above.
	if _, err := tx.ExecContext(ctx, `DELETE FROM venue_payments WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM vendor_issues WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE vm FROM vendor_milestones vm INNER JOIN project_vendors pv ON vm.project_vendor_id = pv.id WHERE pv.project_id = ?`,
		id,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM evidence WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_vendors WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_milestones WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE tenant_id = ? AND id = ?`, tenantID, id); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Project milestones ---

type MySQLMilestoneRepository struct {
	db *sql.DB
}

func NewMySQLMilestoneRepository(db *sql.DB) *MySQLMilestoneRepository {
	return &MySQLMilestoneRepository{db: db}
}

const milestoneColumns = `id, project_id, sort_order, name, category, status, target_date, completed_date`

func scanMilestone(scan func(dest ...interface{}) error) (*domain.ProjectMilestone, error) {
	var m domain.ProjectMilestone
	var status string
	var completedDate sql.NullTime
	err := scan(&m.ID, &m.ProjectID, &m.SortOrder, &m.Name, &m.Category, &status, &m.TargetDate, &completedDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Status = domain.MilestoneStatus(status)
	if completedDate.Valid {
		m.CompletedDate = &completedDate.Time
	}
	return &m, nil
}

func (r *MySQLMilestoneRepository) ListByProject(ctx context.Context, projectID int64) ([]domain.ProjectMilestone, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+milestoneColumns+` FROM project_milestones WHERE project_id = ? ORDER BY sort_order`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var milestones []domain.ProjectMilestone
	for rows.Next() {
		m, err := scanMilestone(rows.Scan)
		if err != nil {
			return nil, err
		}
		milestones = append(milestones, *m)
	}
	return milestones, rows.Err()
}

// ListByProjects backs ComputeProgressBatch (PLAN.md "Performance
// remediation"): one query across every id in projectIDs instead of one
// query per project.
func (r *MySQLMilestoneRepository) ListByProjects(ctx context.Context, projectIDs []int64) ([]domain.ProjectMilestone, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	query := `SELECT ` + milestoneColumns + ` FROM project_milestones WHERE project_id IN (` + utils.Placeholders(len(projectIDs)) + `) ORDER BY project_id, sort_order`
	rows, err := r.db.QueryContext(ctx, query, utils.Int64Args(projectIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var milestones []domain.ProjectMilestone
	for rows.Next() {
		m, err := scanMilestone(rows.Scan)
		if err != nil {
			return nil, err
		}
		milestones = append(milestones, *m)
	}
	return milestones, rows.Err()
}

func (r *MySQLMilestoneRepository) FindByID(ctx context.Context, projectID, id int64) (*domain.ProjectMilestone, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+milestoneColumns+` FROM project_milestones WHERE project_id = ? AND id = ? LIMIT 1`, projectID, id)
	return scanMilestone(row.Scan)
}

func (r *MySQLMilestoneRepository) Create(ctx context.Context, m *domain.ProjectMilestone) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO project_milestones (project_id, sort_order, name, category, status, target_date) VALUES (?, ?, ?, ?, ?, ?)`,
		m.ProjectID, m.SortOrder, m.Name, m.Category, string(m.Status), m.TargetDate,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	m.ID = id
	return nil
}

func (r *MySQLMilestoneRepository) Update(ctx context.Context, m *domain.ProjectMilestone) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE project_milestones SET name = ?, category = ?, status = ?, target_date = ?, completed_date = ? WHERE id = ?`,
		m.Name, m.Category, string(m.Status), m.TargetDate, m.CompletedDate, m.ID,
	)
	return err
}

func (r *MySQLMilestoneRepository) NextSortOrder(ctx context.Context, projectID int64) (int, error) {
	var maxOrder sql.NullInt64
	row := r.db.QueryRowContext(ctx, `SELECT MAX(sort_order) FROM project_milestones WHERE project_id = ?`, projectID)
	if err := row.Scan(&maxOrder); err != nil {
		return 0, err
	}
	return int(maxOrder.Int64) + 1, nil
}

// Reorder rewrites sort_order to match each ID's position in orderedIDs
// (1-based) — the application layer has already validated orderedIDs is an
// exact permutation of this project's milestone IDs.
func (r *MySQLMilestoneRepository) Reorder(ctx context.Context, projectID int64, orderedIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE project_milestones SET sort_order = ? WHERE id = ? AND project_id = ?`,
			i+1, id, projectID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}
