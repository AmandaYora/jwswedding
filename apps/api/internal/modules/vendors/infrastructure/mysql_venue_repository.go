package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"jwswedding/internal/modules/vendors/application"
	"jwswedding/internal/modules/vendors/domain"
	"jwswedding/internal/shared/pagination"
)

type MySQLVenueRepository struct {
	db *sql.DB
}

func NewMySQLVenueRepository(db *sql.DB) *MySQLVenueRepository {
	return &MySQLVenueRepository{db: db}
}

const venueColumns = `id, tenant_id, name, pic_name, phone_pic, phone_venue, email, address, city,
	rental_price, charge, capacity, facilities, social_media, notes, attachment_path,
	attachment_mime_type, is_active, created_at, updated_at`

// scanVenue is a direct nullable-column mapping, same style as scanVendor --
// facilities/social_media are plain TEXT now (not JSON), so they scan exactly
// like notes, no marshal/unmarshal code needed anywhere in this file.
func scanVenue(scan func(dest ...interface{}) error) (*domain.Venue, error) {
	var v domain.Venue
	var phoneVenue, email, address, city, notes sql.NullString
	var rentalPrice, charge, capacity sql.NullInt64
	var facilities, socialMedia sql.NullString
	var attachmentPath, attachmentMimeType sql.NullString

	err := scan(
		&v.ID, &v.TenantID, &v.Name, &v.PICName, &v.PhonePIC, &phoneVenue, &email, &address, &city,
		&rentalPrice, &charge, &capacity, &facilities, &socialMedia, &notes, &attachmentPath,
		&attachmentMimeType, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	v.PhoneVenue = nullStringPtr(phoneVenue)
	v.Email = nullStringPtr(email)
	v.Address = nullStringPtr(address)
	v.City = nullStringPtr(city)
	v.RentalPrice = nullInt64Ptr(rentalPrice)
	v.Charge = nullInt64Ptr(charge)
	v.Capacity = nullIntPtr(capacity)
	v.Facilities = nullStringPtr(facilities)
	v.SocialMedia = nullStringPtr(socialMedia)
	v.Notes = notes.String
	v.AttachmentPath = nullStringPtr(attachmentPath)
	v.AttachmentMimeType = nullStringPtr(attachmentMimeType)
	return &v, nil
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func nullInt64Ptr(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

func nullIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

func (r *MySQLVenueRepository) List(ctx context.Context, tenantID int64) ([]domain.Venue, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+venueColumns+` FROM venues WHERE tenant_id = ? ORDER BY id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var venues []domain.Venue
	for rows.Next() {
		v, err := scanVenue(rows.Scan)
		if err != nil {
			return nil, err
		}
		venues = append(venues, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachCategories(ctx, tenantID, venues); err != nil {
		return nil, err
	}
	return venues, nil
}

// venueIDsOf collects IDs for the single IN-query category load below.
func venueIDsOf(venues []domain.Venue) []int64 {
	ids := make([]int64, 0, len(venues))
	for _, v := range venues {
		ids = append(ids, v.ID)
	}
	return ids
}

// loadCategories fetches categories for a page of venues in ONE query —
// never one per venue. Empty input returns an empty map without querying.
// Rows arrive ordered by (venue_id, id); the INSERT order preserves the
// canonical order NormalizeVenueCategories sorted into.
func (r *MySQLVenueRepository) loadCategories(ctx context.Context, tenantID int64, venueIDs []int64) (map[int64][]string, error) {
	out := make(map[int64][]string)
	if len(venueIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(venueIDs))
	args := make([]interface{}, 0, len(venueIDs)+1)
	args = append(args, tenantID)
	for _, id := range venueIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT venue_id, category FROM venue_categories WHERE tenant_id = ? AND venue_id IN (`+strings.Join(placeholders, ", ")+`) ORDER BY venue_id, id`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var venueID int64
		var category string
		if err := rows.Scan(&venueID, &category); err != nil {
			return nil, err
		}
		out[venueID] = append(out[venueID], category)
	}
	return out, rows.Err()
}

// attachCategories fills Categories on every venue in place via one
// loadCategories call.
func (r *MySQLVenueRepository) attachCategories(ctx context.Context, tenantID int64, venues []domain.Venue) error {
	cats, err := r.loadCategories(ctx, tenantID, venueIDsOf(venues))
	if err != nil {
		return err
	}
	for i := range venues {
		if c, ok := cats[venues[i].ID]; ok {
			venues[i].Categories = c
		}
	}
	return nil
}

// ListPaginated backs the real `GET /venues` list page -- List above stays
// as-is for the venue-picker (attach-to-project) dropdown and the bulk
// import's prefetch, both of which need the full roster in one call.
func (r *MySQLVenueRepository) ListPaginated(ctx context.Context, tenantID int64, filter application.VenueListFilter, params pagination.Params) ([]domain.Venue, int64, error) {
	countQuery := `SELECT COUNT(*) FROM venues WHERE tenant_id = ?`
	listQuery := `SELECT ` + venueColumns + ` FROM venues WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	r.applyVenueFilters(&countQuery, &listQuery, &args, filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += ` ORDER BY id LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, listQuery, append(args, params.Limit, params.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var venues []domain.Venue
	for rows.Next() {
		v, err := scanVenue(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		venues = append(venues, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := r.attachCategories(ctx, tenantID, venues); err != nil {
		return nil, 0, err
	}
	return venues, total, nil
}

// applyVenueFilters appends the shared search/city/category/tier/capacity
// predicates to both the COUNT and the page query (same args order for both).
// Category uses EXISTS — not a JOIN — so a multi-category venue is counted
// and listed exactly once.
func (r *MySQLVenueRepository) applyVenueFilters(countQuery, listQuery *string, args *[]interface{}, filter application.VenueListFilter) {
	if filter.Search != "" {
		*countQuery += ` AND (name LIKE ? OR pic_name LIKE ? OR email LIKE ?)`
		*listQuery += ` AND (name LIKE ? OR pic_name LIKE ? OR email LIKE ?)`
		like := "%" + filter.Search + "%"
		*args = append(*args, like, like, like)
	}
	if filter.City != "" {
		*countQuery += ` AND city = ?`
		*listQuery += ` AND city = ?`
		*args = append(*args, filter.City)
	}
	if filter.Category != "" {
		*countQuery += ` AND EXISTS (SELECT 1 FROM venue_categories vc WHERE vc.tenant_id = venues.tenant_id AND vc.venue_id = venues.id AND vc.category = ?)`
		*listQuery += ` AND EXISTS (SELECT 1 FROM venue_categories vc WHERE vc.tenant_id = venues.tenant_id AND vc.venue_id = venues.id AND vc.category = ?)`
		*args = append(*args, filter.Category)
	}
	if filter.PriceTier != "" {
		if min, max, ok := domain.VenuePriceTierRange(filter.PriceTier); ok {
			*countQuery += ` AND rental_price IS NOT NULL AND rental_price >= ?`
			*listQuery += ` AND rental_price IS NOT NULL AND rental_price >= ?`
			*args = append(*args, min)
			if max > 0 {
				*countQuery += ` AND rental_price <= ?`
				*listQuery += ` AND rental_price <= ?`
				*args = append(*args, max)
			}
		}
	}
	if filter.CapacityMin != nil {
		*countQuery += ` AND capacity IS NOT NULL AND capacity >= ?`
		*listQuery += ` AND capacity IS NOT NULL AND capacity >= ?`
		*args = append(*args, *filter.CapacityMin)
	}
}

// ListFiltered backs Export -- same filters as ListPaginated, but
// unpaginated (Export always returns the whole matching set).
func (r *MySQLVenueRepository) ListFiltered(ctx context.Context, tenantID int64, filter application.VenueListFilter) ([]domain.Venue, error) {
	countQuery := `SELECT COUNT(*) FROM venues WHERE tenant_id = ?`
	query := `SELECT ` + venueColumns + ` FROM venues WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	// Reuse the shared predicate builder; the COUNT side is discarded.
	r.applyVenueFilters(&countQuery, &query, &args, filter)
	query += ` ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var venues []domain.Venue
	for rows.Next() {
		v, err := scanVenue(rows.Scan)
		if err != nil {
			return nil, err
		}
		venues = append(venues, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachCategories(ctx, tenantID, venues); err != nil {
		return nil, err
	}
	return venues, nil
}

func (r *MySQLVenueRepository) FindByID(ctx context.Context, tenantID, id int64) (*domain.Venue, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+venueColumns+` FROM venues WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	venue, err := scanVenue(row.Scan)
	if err != nil || venue == nil {
		return venue, err
	}
	cats, err := r.loadCategories(ctx, tenantID, []int64{venue.ID})
	if err != nil {
		return nil, err
	}
	venue.Categories = cats[venue.ID]
	return venue, nil
}

// replaceCategoriesTx rewrites one venue's labels: DELETE all, then a single
// multi-row INSERT. An empty cats stops after the DELETE — assembling
// `INSERT ... VALUES` with zero rows is a SQL syntax error, not a no-op,
// and a categoriless venue is legitimate (D-5: empty Import cells land here).
func replaceCategoriesTx(ctx context.Context, tx *sql.Tx, tenantID, venueID int64, cats []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM venue_categories WHERE tenant_id = ? AND venue_id = ?`, tenantID, venueID); err != nil {
		return err
	}
	if len(cats) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(`INSERT INTO venue_categories (tenant_id, venue_id, category) VALUES `)
	args := make([]interface{}, 0, len(cats)*3)
	for i, c := range cats {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(`(?, ?, ?)`)
		args = append(args, tenantID, venueID, c)
	}
	_, err := tx.ExecContext(ctx, sb.String(), args...)
	return err
}

func (r *MySQLVenueRepository) Create(ctx context.Context, venue *domain.Venue) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO venues (tenant_id, name, pic_name, phone_pic, phone_venue, email, address, city,
		 rental_price, charge, capacity, facilities, social_media, notes, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		venue.TenantID, venue.Name, venue.PICName, venue.PhonePIC, venue.PhoneVenue, venue.Email, venue.Address, venue.City,
		venue.RentalPrice, venue.Charge, venue.Capacity, venue.Facilities, venue.SocialMedia, venue.Notes, venue.IsActive,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	venue.ID = id
	if err := replaceCategoriesTx(ctx, tx, venue.TenantID, venue.ID, venue.Categories); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// CreateBatch inserts multiple venues in one multi-row INSERT statement --
// used only by the bulk-import flow (§7 of PLAN.md's Revisi 2), which has
// already chunked the caller's slice to a safe size before calling this.
// IDs are not read back into the caller's slice: the import flow has no
// further use for them once committed.
func (r *MySQLVenueRepository) CreateBatch(ctx context.Context, venues []domain.Venue) error {
	if len(venues) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(`INSERT INTO venues (tenant_id, name, pic_name, phone_pic, phone_venue, email, address, city,
		 rental_price, charge, capacity, facilities, social_media, notes, is_active) VALUES `)
	args := make([]interface{}, 0, len(venues)*15)
	for i, v := range venues {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(`(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		args = append(args, v.TenantID, v.Name, v.PICName, v.PhonePIC, v.PhoneVenue, v.Email, v.Address, v.City,
			v.RentalPrice, v.Charge, v.Capacity, v.Facilities, v.SocialMedia, v.Notes, v.IsActive)
	}
	_, err := r.db.ExecContext(ctx, sb.String(), args...)
	return err
}

func (r *MySQLVenueRepository) Update(ctx context.Context, venue *domain.Venue) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	_, err = tx.ExecContext(ctx,
		`UPDATE venues SET name = ?, pic_name = ?, phone_pic = ?, phone_venue = ?, email = ?, address = ?, city = ?,
		 rental_price = ?, charge = ?, capacity = ?, facilities = ?, social_media = ?, notes = ?
		 WHERE tenant_id = ? AND id = ?`,
		venue.Name, venue.PICName, venue.PhonePIC, venue.PhoneVenue, venue.Email, venue.Address, venue.City,
		venue.RentalPrice, venue.Charge, venue.Capacity, venue.Facilities, venue.SocialMedia, venue.Notes,
		venue.TenantID, venue.ID,
	)
	if err != nil {
		return err
	}
	if err := replaceCategoriesTx(ctx, tx, venue.TenantID, venue.ID, venue.Categories); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// SetCategoriesBatch writes categories for many venues at once (the import
// insert path): one DELETE for the venue set, then one multi-row INSERT.
// Venues mapped to an empty/nil slice are cleared by the DELETE. An empty
// map is a no-op without querying.
func (r *MySQLVenueRepository) SetCategoriesBatch(ctx context.Context, tenantID int64, venueCategories map[int64][]string) error {
	if len(venueCategories) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(venueCategories))
	for id := range venueCategories {
		ids = append(ids, id)
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, tenantID)
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM venue_categories WHERE tenant_id = ? AND venue_id IN (`+strings.Join(placeholders, ", ")+`)`, args...); err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString(`INSERT INTO venue_categories (tenant_id, venue_id, category) VALUES `)
	var vals []interface{}
	first := true
	for venueID, cats := range venueCategories {
		for _, c := range cats {
			if !first {
				sb.WriteString(", ")
			}
			first = false
			sb.WriteString(`(?, ?, ?)`)
			vals = append(vals, tenantID, venueID, c)
		}
	}
	if len(vals) > 0 {
		if _, err := tx.ExecContext(ctx, sb.String(), vals...); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (r *MySQLVenueRepository) SetActive(ctx context.Context, tenantID, id int64, isActive bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE venues SET is_active = ? WHERE tenant_id = ? AND id = ?`, isActive, tenantID, id)
	return err
}

func (r *MySQLVenueRepository) UpdateAttachment(ctx context.Context, tenantID, id int64, path, mimeType *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE venues SET attachment_path = ?, attachment_mime_type = ? WHERE tenant_id = ? AND id = ?`, path, mimeType, tenantID, id)
	return err
}

// Delete permanently removes a venue -- a deliberate, guarded exception to
// this codebase's soft-state convention (PLAN.md's hard-delete plan:
// informed-consent, not a blocking precondition). Never touches
// `projects.venue_id`, which has no SQL FK to this table (cross-module by
// design) -- left to degrade gracefully via ProjectService.GetVenue's
// graceful-not-found handling, exactly the tradeoff the confirmation dialog
// exists to make explicit.
func (r *MySQLVenueRepository) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM venues WHERE tenant_id = ? AND id = ?`, tenantID, id)
	return err
}
