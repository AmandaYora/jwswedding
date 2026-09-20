package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"

	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/pagination"
)

type MySQLRundownRepository struct {
	db *sql.DB
}

func NewMySQLRundownRepository(db *sql.DB) *MySQLRundownRepository {
	return &MySQLRundownRepository{db: db}
}

const rundownColumns = `id, tenant_id, project_id, project_name,
	groom_name, groom_birth_order, groom_parents,
	bride_name, bride_birth_order, bride_parents,
	event_date_label, venue_label, event_time_label, couple_title,
	wo_pic_name, wo_pic_phone,
	siblings_bride, siblings_groom, souvenir_note, table_cloth_note,
	playlist_notes, layout_image_path,
	last_docx_evidence_id, last_pdf_evidence_id, created_at, updated_at`

func scanRundown(scan func(dest ...interface{}) error) (*domain.Rundown, error) {
	var r domain.Rundown
	var siblingsBride, siblingsGroom, playlistNotes sql.NullString
	err := scan(
		&r.ID, &r.TenantID, &r.ProjectID, &r.ProjectName,
		&r.GroomName, &r.GroomBirthOrder, &r.GroomParents,
		&r.BrideName, &r.BrideBirthOrder, &r.BrideParents,
		&r.EventDateLabel, &r.VenueLabel, &r.EventTimeLabel, &r.CoupleTitle,
		&r.WOPICName, &r.WOPICPhone,
		&siblingsBride, &siblingsGroom, &r.SouvenirNote, &r.TableClothNote,
		&playlistNotes, &r.LayoutImagePath,
		&r.LastDocxEvidenceID, &r.LastPdfEvidenceID, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.SiblingsBride = siblingsBride.String
	r.SiblingsGroom = siblingsGroom.String
	r.PlaylistNotes = playlistNotes.String
	return &r, nil
}

// inClause membangun "(?, ?, ...)" beserta argumennya. Pemanggil WAJIB
// memastikan ids tidak kosong: `IN ()` adalah galat sintaks di MySQL, bukan
// himpunan kosong.
func inClause(ids []int64) (string, []interface{}) {
	marks := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args[i] = id
	}
	return "(" + strings.Join(marks, ", ") + ")", args
}

// --------------------------------------------------------------- pembacaan

func (r *MySQLRundownRepository) List(ctx context.Context, tenantID int64,
	filter application.ListFilter, params pagination.Params) ([]domain.Summary, int, error) {

	where := []string{"tenant_id = ?"}
	args := []interface{}{tenantID}

	if filter.ProjectIDs != nil {
		// nil = tanpa pembatasan; slice kosong = benar-benar tidak ada project
		// yang boleh dilihat. Keduanya berbeda arti dan tidak boleh disamakan.
		if len(*filter.ProjectIDs) == 0 {
			return []domain.Summary{}, 0, nil
		}
		clause, inArgs := inClause(*filter.ProjectIDs)
		where = append(where, "project_id IN "+clause)
		args = append(args, inArgs...)
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		where = append(where, "(project_name LIKE ? OR bride_name LIKE ? OR groom_name LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	cond := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rundowns"+cond, args...).
		Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, project_name, bride_name, groom_name,
		        event_date_label, venue_label, layout_image_path, updated_at
		 FROM rundowns`+cond+` ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`,
		append(args, params.Limit, params.Offset())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []domain.Summary{}
	for rows.Next() {
		var s domain.Summary
		var layoutPath string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ProjectName, &s.BrideName,
			&s.GroomName, &s.EventDateLabel, &s.VenueLabel, &layoutPath,
			&s.UpdatedAt); err != nil {
			return nil, 0, err
		}
		s.HasLayoutImage = layoutPath != ""
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// UsedProjectIDs memberi dialog "Buat Rundown" daftar project yang sudah
// terpakai. Sengaja endpoint sendiri, bukan hasil paginasi List: MaxLimit=100
// akan memotongnya diam-diam begitu rundown melewati seratus baris.
func (r *MySQLRundownRepository) UsedProjectIDs(ctx context.Context, tenantID int64,
	scope *[]int64) ([]int64, error) {

	query := "SELECT project_id FROM rundowns WHERE tenant_id = ?"
	args := []interface{}{tenantID}
	if scope != nil {
		if len(*scope) == 0 {
			return []int64{}, nil
		}
		clause, inArgs := inClause(*scope)
		query += " AND project_id IN " + clause
		args = append(args, inArgs...)
	}
	query += " ORDER BY project_id"

	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *MySQLRundownRepository) FindByID(ctx context.Context, tenantID, id int64) (*domain.Rundown, error) {
	return scanRundown(r.db.QueryRowContext(ctx,
		"SELECT "+rundownColumns+" FROM rundowns WHERE tenant_id = ? AND id = ?",
		tenantID, id).Scan)
}

func (r *MySQLRundownRepository) FindByProject(ctx context.Context, tenantID, projectID int64) (*domain.Rundown, error) {
	return scanRundown(r.db.QueryRowContext(ctx,
		"SELECT "+rundownColumns+" FROM rundowns WHERE tenant_id = ? AND project_id = ?",
		tenantID, projectID).Scan)
}

// LoadAggregate menarik seluruh isi satu rundown: satu query untuk akar dan
// satu per tabel anak. Jumlah query tetap dua belas berapa pun banyak barisnya
// — bukan N+1.
func (r *MySQLRundownRepository) LoadAggregate(ctx context.Context, tenantID, id int64) (*domain.View, error) {
	root, err := r.FindByID(ctx, tenantID, id)
	if err != nil || root == nil {
		return nil, err
	}
	v := &domain.View{Rundown: *root}

	if err := r.each(ctx, `SELECT id, sort_order, category_label, vendor_name
		FROM rundown_vendors WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.Vendor
			if err := s(&x.ID, &x.SortOrder, &x.CategoryLabel, &x.VendorName); err != nil {
				return err
			}
			v.Vendors = append(v.Vendors, x)
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, sort_order, role_label, person_name, note
		FROM rundown_roles WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.Role
			if err := s(&x.ID, &x.SortOrder, &x.RoleLabel, &x.PersonName, &x.Note); err != nil {
				return err
			}
			v.Roles = append(v.Roles, x)
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, sort_order, role_label,
		COALESCE(person_text, ''), COALESCE(job_desc, '')
		FROM rundown_committees WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.Committee
			if err := s(&x.ID, &x.SortOrder, &x.RoleLabel, &x.PersonText, &x.JobDesc); err != nil {
				return err
			}
			v.Committees = append(v.Committees, x)
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, sort_order, group_key, style, COALESCE(content, '')
		FROM rundown_menu_items WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.MenuItem
			if err := s(&x.ID, &x.SortOrder, &x.GroupKey, &x.Style, &x.Content); err != nil {
				return err
			}
			v.MenuItems = append(v.MenuItems, x)
			return nil
		}); err != nil {
		return nil, err
	}

	roomIndex := map[int64]int{}
	if err := r.each(ctx, `SELECT id, sort_order, room_label
		FROM rundown_makeup_rooms WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.MakeupRoom
			if err := s(&x.ID, &x.SortOrder, &x.RoomLabel); err != nil {
				return err
			}
			roomIndex[x.ID] = len(v.MakeupRooms)
			v.MakeupRooms = append(v.MakeupRooms, x)
			return nil
		}); err != nil {
		return nil, err
	}
	// Satu query untuk SEMUA baris makeup, lalu dibagikan ke ruangannya di
	// memori — bukan satu query per ruangan.
	if err := r.each(ctx, `SELECT id, makeup_room_id, sort_order, style, COALESCE(content, '')
		FROM rundown_makeup_lines WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.MakeupLine
			if err := s(&x.ID, &x.RoomID, &x.SortOrder, &x.Style, &x.Content); err != nil {
				return err
			}
			if idx, ok := roomIndex[x.RoomID]; ok {
				v.MakeupRooms[idx].Lines = append(v.MakeupRooms[idx].Lines, x)
			}
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, section, sort_order, no_label, time_label,
		COALESCE(item, ''), COALESCE(pic, ''), COALESCE(note, '')
		FROM rundown_items WHERE rundown_id = ? ORDER BY section, sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.Item
			if err := s(&x.ID, &x.Section, &x.SortOrder, &x.NoLabel, &x.TimeLabel,
				&x.Item, &x.PIC, &x.Note); err != nil {
				return err
			}
			v.Items = append(v.Items, x)
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, kind, sort_order, number_label, COALESCE(content, '')
		FROM rundown_layout_notes WHERE rundown_id = ? ORDER BY kind, sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.LayoutNote
			if err := s(&x.ID, &x.Kind, &x.SortOrder, &x.NumberLabel, &x.Content); err != nil {
				return err
			}
			v.LayoutNotes = append(v.LayoutNotes, x)
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, sort_order, group_name
		FROM rundown_photo_groups WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.PhotoGroup
			if err := s(&x.ID, &x.SortOrder, &x.GroupName); err != nil {
				return err
			}
			v.PhotoGroups = append(v.PhotoGroups, x)
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, sort_order, full_name, position
		FROM rundown_vip_guests WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.VIPGuest
			if err := s(&x.ID, &x.SortOrder, &x.FullName, &x.Position); err != nil {
				return err
			}
			v.VIPGuests = append(v.VIPGuests, x)
			return nil
		}); err != nil {
		return nil, err
	}

	if err := r.each(ctx, `SELECT id, sort_order, title, artist
		FROM rundown_playlist WHERE rundown_id = ? ORDER BY sort_order, id`, id,
		func(s func(...interface{}) error) error {
			var x domain.PlaylistEntry
			if err := s(&x.ID, &x.SortOrder, &x.Title, &x.Artist); err != nil {
				return err
			}
			v.Playlist = append(v.Playlist, x)
			return nil
		}); err != nil {
		return nil, err
	}

	return v, nil
}

func (r *MySQLRundownRepository) each(ctx context.Context, query string, id int64,
	fn func(scan func(...interface{}) error) error) error {
	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
}

// --------------------------------------------------------------- penulisan

func (r *MySQLRundownRepository) Create(ctx context.Context, rd *domain.Rundown) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO rundowns (tenant_id, project_id, project_name,
			groom_name, groom_birth_order, groom_parents,
			bride_name, bride_birth_order, bride_parents,
			event_date_label, venue_label, event_time_label, couple_title,
			wo_pic_name, wo_pic_phone, siblings_bride, siblings_groom,
			souvenir_note, table_cloth_note, playlist_notes, layout_image_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rd.TenantID, rd.ProjectID, rd.ProjectName,
		rd.GroomName, rd.GroomBirthOrder, rd.GroomParents,
		rd.BrideName, rd.BrideBirthOrder, rd.BrideParents,
		rd.EventDateLabel, rd.VenueLabel, rd.EventTimeLabel, rd.CoupleTitle,
		rd.WOPICName, rd.WOPICPhone, rd.SiblingsBride, rd.SiblingsGroom,
		rd.SouvenirNote, rd.TableClothNote, rd.PlaylistNotes, rd.LayoutImagePath)
	if err != nil {
		// uq_rundowns_project, bukan pemeriksaan di service, yang benar-benar
		// menjamin satu project satu rundown: pemeriksaan itu check-then-insert
		// dan dua permintaan bersamaan bisa lolos berdua. Tanpa pemetaan ini
		// yang kedua menerima 500 "Terjadi kesalahan pada server" untuk keadaan
		// yang sebenarnya sangat bisa dijelaskan.
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return apperror.Conflict("Project ini sudah punya rundown")
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	rd.ID = id
	return nil
}

func (r *MySQLRundownRepository) UpdateProjectName(ctx context.Context, tenantID, id int64, name string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE rundowns SET project_name = ? WHERE tenant_id = ? AND id = ?", name, tenantID, id)
	return err
}

func (r *MySQLRundownRepository) UpdateEvidenceID(ctx context.Context, tenantID, id int64, format string, evidenceID int64) error {
	column := "last_docx_evidence_id"
	if format == "pdf" {
		column = "last_pdf_evidence_id"
	}
	// Nama kolom berasal dari konstanta di atas, bukan dari input pemanggil.
	_, err := r.db.ExecContext(ctx,
		"UPDATE rundowns SET "+column+" = ? WHERE tenant_id = ? AND id = ?", evidenceID, tenantID, id)
	return err
}

func (r *MySQLRundownRepository) UpdateLayoutImagePath(ctx context.Context, tenantID, id int64, path string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE rundowns SET layout_image_path = ? WHERE tenant_id = ? AND id = ?", path, tenantID, id)
	return err
}

func (r *MySQLRundownRepository) Delete(ctx context.Context, tenantID, id int64) error {
	// Tabel anak ikut terhapus lewat ON DELETE CASCADE — semuanya milik modul
	// ini sendiri, jadi tidak ada pembersihan lintas modul di sini.
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM rundowns WHERE tenant_id = ? AND id = ?", tenantID, id)
	return err
}

// batchInsert menulis banyak baris dalam SATU pernyataan INSERT.
//
// Seksi SUSUNAN ACARA yang panjang bisa puluhan baris; satu INSERT per baris
// berarti puluhan bolak-balik ke MySQL di dalam transaksi yang sama, dan
// selama itu baris akar-nya terkunci. Dipotong per 200 baris supaya satu
// pernyataan tidak pernah mendekati max_allowed_packet betapa pun panjang
// seksinya.
func batchInsert(ctx context.Context, tx *sql.Tx, table string, columns []string,
	rows [][]interface{}) error {

	if len(rows) == 0 {
		return nil
	}
	const perStatement = 200
	marks := make([]string, len(columns))
	for i := range marks {
		marks[i] = "?"
	}
	tuple := "(" + strings.Join(marks, ", ") + ")"

	for start := 0; start < len(rows); start += perStatement {
		end := start + perStatement
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]
		tuples := make([]string, len(chunk))
		args := make([]interface{}, 0, len(chunk)*len(columns))
		for i, row := range chunk {
			tuples[i] = tuple
			args = append(args, row...)
		}
		// Nama tabel dan kolom berasal dari konstanta di ReplaceSection, tidak
		// pernah dari input pemanggil; hanya nilainya yang ter-parameter.
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO "+table+" ("+strings.Join(columns, ", ")+") VALUES "+
				strings.Join(tuples, ", "), args...); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceSection menulis ulang satu seksi secara utuh: hapus baris lama,
// masukkan baris baru, dan -- untuk tiga seksi yang juga punya field di tabel
// akar -- perbarui kolom akarnya. Semua di dalam satu transaksi, dan transaksi
// itu tidak pernah keluar dari modul ini.
func (r *MySQLRundownRepository) ReplaceSection(ctx context.Context, tenantID, id int64,
	section domain.SectionKey, payload application.SectionPayload) error {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Gerbang tenant untuk SELURUH transaksi. Seluruh pernyataan di bawah
	// menyaring dengan `rundown_id = ?` saja -- itu cukup dan tetap murah
	// justru karena kepemilikannya sudah dipastikan di sini. FOR UPDATE
	// sekalian membuat dua penyimpanan seksi yang bersamaan pada rundown yang
	// sama berbaris, bukan saling menimpa di tengah jalan.
	var owned int64
	if err := tx.QueryRowContext(ctx,
		"SELECT id FROM rundowns WHERE tenant_id = ? AND id = ? FOR UPDATE",
		tenantID, id).Scan(&owned); err != nil {
		if err == sql.ErrNoRows {
			return apperror.NotFound("Rundown tidak ditemukan")
		}
		return err
	}

	clear := func(table string) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE rundown_id = ?", id)
		return err
	}

	switch section {
	case domain.SectionKeyCover:
		if _, err := tx.ExecContext(ctx,
			`UPDATE rundowns SET groom_name = ?, groom_birth_order = ?, groom_parents = ?,
			   bride_name = ?, bride_birth_order = ?, bride_parents = ?,
			   event_date_label = ?, venue_label = ?, event_time_label = ?,
			   couple_title = ?, wo_pic_name = ?, wo_pic_phone = ?
			 WHERE id = ?`,
			payload.Cover.GroomName, payload.Cover.GroomBirthOrder, payload.Cover.GroomParents,
			payload.Cover.BrideName, payload.Cover.BrideBirthOrder, payload.Cover.BrideParents,
			payload.Cover.EventDateLabel, payload.Cover.VenueLabel, payload.Cover.EventTimeLabel,
			payload.Cover.CoupleTitle, payload.Cover.WOPICName, payload.Cover.WOPICPhone,
			id); err != nil {
			return err
		}

	case domain.SectionKeyVendors:
		if err := clear("rundown_vendors"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.Vendors))
		for i, v := range payload.Vendors {
			rows = append(rows, []interface{}{id, i, v.CategoryLabel, v.VendorName})
		}
		if err := batchInsert(ctx, tx, "rundown_vendors",
			[]string{"rundown_id", "sort_order", "category_label", "vendor_name"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyRoles:
		if err := clear("rundown_roles"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.Roles))
		for i, v := range payload.Roles {
			rows = append(rows, []interface{}{id, i, v.RoleLabel, v.PersonName, v.Note})
		}
		if err := batchInsert(ctx, tx, "rundown_roles",
			[]string{"rundown_id", "sort_order", "role_label", "person_name", "note"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyCommittees:
		if err := clear("rundown_committees"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.Committees))
		for i, v := range payload.Committees {
			rows = append(rows, []interface{}{id, i, v.RoleLabel, v.PersonText, v.JobDesc})
		}
		if err := batchInsert(ctx, tx, "rundown_committees",
			[]string{"rundown_id", "sort_order", "role_label", "person_text", "job_desc"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyDataLainnya:
		if _, err := tx.ExecContext(ctx,
			`UPDATE rundowns SET siblings_bride = ?, siblings_groom = ?,
			   souvenir_note = ?, table_cloth_note = ? WHERE id = ?`,
			payload.DataLainnya.SiblingsBride, payload.DataLainnya.SiblingsGroom,
			payload.DataLainnya.SouvenirNote, payload.DataLainnya.TableClothNote,
			id); err != nil {
			return err
		}
		if err := clear("rundown_menu_items"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.MenuItems))
		for i, v := range payload.MenuItems {
			rows = append(rows, []interface{}{id, i, v.GroupKey, v.Style, v.Content})
		}
		if err := batchInsert(ctx, tx, "rundown_menu_items",
			[]string{"rundown_id", "sort_order", "group_key", "style", "content"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyMakeup:
		// rundown_makeup_lines ikut terhapus lewat FK cascade ke room, tetapi
		// dihapus eksplisit lebih dulu supaya urutannya tidak bergantung pada
		// perilaku cascade.
		if err := clear("rundown_makeup_lines"); err != nil {
			return err
		}
		if err := clear("rundown_makeup_rooms"); err != nil {
			return err
		}
		// Ruangan tetap disisipkan satu per satu: barisnya butuh LastInsertId
		// masing-masing, dan INSERT multi-baris hanya menjanjikan id baris
		// PERTAMA -- id berikutnya tidak dijamin berurutan pada
		// innodb_autoinc_lock_mode=2. Jumlah ruangan memang sedikit; yang
		// banyak adalah barisnya, dan itu yang di-batch.
		lines := make([][]interface{}, 0, 32)
		for i, room := range payload.MakeupRooms {
			res, err := tx.ExecContext(ctx,
				`INSERT INTO rundown_makeup_rooms (rundown_id, sort_order, room_label)
				 VALUES (?, ?, ?)`, id, i, room.RoomLabel)
			if err != nil {
				return err
			}
			roomID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			for j, line := range room.Lines {
				lines = append(lines, []interface{}{id, roomID, j, line.Style, line.Content})
			}
		}
		if err := batchInsert(ctx, tx, "rundown_makeup_lines",
			[]string{"rundown_id", "makeup_room_id", "sort_order", "style", "content"},
			lines); err != nil {
			return err
		}

	case domain.SectionKeyAcaraAkad, domain.SectionKeyAcaraResepsi:
		sec := domain.SectionAkad
		if section == domain.SectionKeyAcaraResepsi {
			sec = domain.SectionResepsi
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM rundown_items WHERE rundown_id = ? AND section = ?",
			id, string(sec)); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.Items))
		for i, v := range payload.Items {
			rows = append(rows, []interface{}{
				id, string(sec), i, v.NoLabel, v.TimeLabel, v.Item, v.PIC, v.Note})
		}
		if err := batchInsert(ctx, tx, "rundown_items",
			[]string{"rundown_id", "section", "sort_order", "no_label", "time_label",
				"item", "pic", "note"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyLayout:
		if err := clear("rundown_layout_notes"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.LayoutNotes))
		for i, v := range payload.LayoutNotes {
			rows = append(rows, []interface{}{id, v.Kind, i, v.NumberLabel, v.Content})
		}
		if err := batchInsert(ctx, tx, "rundown_layout_notes",
			[]string{"rundown_id", "kind", "sort_order", "number_label", "content"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyFotoTamu:
		if err := clear("rundown_photo_groups"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.PhotoGroups))
		for i, v := range payload.PhotoGroups {
			rows = append(rows, []interface{}{id, i, v.GroupName})
		}
		if err := batchInsert(ctx, tx, "rundown_photo_groups",
			[]string{"rundown_id", "sort_order", "group_name"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyTamuVIP:
		if err := clear("rundown_vip_guests"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.VIPGuests))
		for i, v := range payload.VIPGuests {
			rows = append(rows, []interface{}{id, i, v.FullName, v.Position})
		}
		if err := batchInsert(ctx, tx, "rundown_vip_guests",
			[]string{"rundown_id", "sort_order", "full_name", "position"}, rows); err != nil {
			return err
		}

	case domain.SectionKeyPlaylist:
		if _, err := tx.ExecContext(ctx,
			"UPDATE rundowns SET playlist_notes = ? WHERE id = ?",
			payload.PlaylistNotes, id); err != nil {
			return err
		}
		if err := clear("rundown_playlist"); err != nil {
			return err
		}
		rows := make([][]interface{}, 0, len(payload.Playlist))
		for i, v := range payload.Playlist {
			rows = append(rows, []interface{}{id, i, v.Title, v.Artist})
		}
		if err := batchInsert(ctx, tx, "rundown_playlist",
			[]string{"rundown_id", "sort_order", "title", "artist"}, rows); err != nil {
			return err
		}

	default:
		return fmt.Errorf("seksi tidak dikenal: %s", section)
	}

	// Menyentuh updated_at supaya urutan daftar mencerminkan suntingan terakhir
	// meski yang berubah hanya tabel anak.
	if _, err := tx.ExecContext(ctx,
		"UPDATE rundowns SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
