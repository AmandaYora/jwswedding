package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
)

// MySQLRundownTemplateRepository menyimpan Template Rundown sebagai satu
// dokumen JSON per tenant (migrasi 000075).
type MySQLRundownTemplateRepository struct {
	db *sql.DB
}

func NewMySQLRundownTemplateRepository(db *sql.DB) *MySQLRundownTemplateRepository {
	return &MySQLRundownTemplateRepository{db: db}
}

// templateDocVersion ditulis ke setiap dokumen supaya perubahan bentuk di masa
// depan bisa dimigrasikan dengan membaca versinya, bukan menebak.
const templateDocVersion = 1

// templateDoc adalah bentuk JSON yang tersimpan. Sengaja struct lokal ber-tag
// json, bukan domain.Template langsung: nama field Go boleh berganti tanpa
// merusak dokumen yang sudah tersimpan. ID dan SortOrder tidak disimpan —
// urutan array adalah urutannya.
type templateDoc struct {
	V            int              `json:"v"`
	Roles        []templateRole   `json:"roles"`
	Committees   []templateCommit `json:"committees"`
	MakeupRooms  []templateRoom   `json:"makeupRooms"`
	ItemsAkad    []templateItem   `json:"itemsAkad"`
	ItemsResepsi []templateItem   `json:"itemsResepsi"`
	LayoutNotes  []templateLayout `json:"layoutNotes"`
}

type templateRole struct {
	RoleLabel  string `json:"roleLabel"`
	PersonName string `json:"personName"`
	Note       string `json:"note"`
}

type templateCommit struct {
	RoleLabel  string `json:"roleLabel"`
	PersonText string `json:"personText"`
	JobDesc    string `json:"jobDesc"`
}

type templateRoom struct {
	RoomLabel string         `json:"roomLabel"`
	Lines     []templateLine `json:"lines"`
}

type templateLine struct {
	Style   string `json:"style"`
	Content string `json:"content"`
}

type templateItem struct {
	NoLabel   string `json:"noLabel"`
	TimeLabel string `json:"timeLabel"`
	Item      string `json:"item"`
	PIC       string `json:"pic"`
	Note      string `json:"note"`
}

type templateLayout struct {
	Kind        string `json:"kind"`
	NumberLabel string `json:"numberLabel"`
	Content     string `json:"content"`
}

func (r *MySQLRundownTemplateRepository) Get(ctx context.Context, tenantID int64) (*domain.Template, error) {
	var raw []byte
	var updated time.Time
	err := r.db.QueryRowContext(ctx,
		"SELECT payload, updated_at FROM rundown_templates WHERE tenant_id = ?", tenantID).
		Scan(&raw, &updated)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc templateDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("template rundown tenant %d tidak terbaca: %w", tenantID, err)
	}
	t := docToTemplate(doc)
	t.TenantID = tenantID
	t.UpdatedAt = updated
	return t, nil
}

// ReplaceSection mengganti satu seksi. Baris template dikunci (FOR UPDATE)
// selama baca-ubah-tulis, sehingga dua penyuntingan seksi berbeda yang
// bersamaan berbaris, bukan saling menimpa.
func (r *MySQLRundownTemplateRepository) ReplaceSection(ctx context.Context, tenantID int64,
	section domain.SectionKey, payload application.SectionPayload) error {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// INSERT IGNORE menjamin barisnya ada sebelum dikunci; uq_rundown_templates_tenant
	// membuat pemanggil kedua yang bersamaan cukup mengabaikannya.
	if _, err := tx.ExecContext(ctx,
		"INSERT IGNORE INTO rundown_templates (tenant_id, payload) VALUES (?, ?)",
		tenantID, fmt.Sprintf(`{"v":%d}`, templateDocVersion)); err != nil {
		return err
	}
	var raw []byte
	if err := tx.QueryRowContext(ctx,
		"SELECT payload FROM rundown_templates WHERE tenant_id = ? FOR UPDATE", tenantID).
		Scan(&raw); err != nil {
		return err
	}
	var doc templateDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("template rundown tenant %d tidak terbaca: %w", tenantID, err)
	}

	t := docToTemplate(doc)
	switch section {
	case domain.SectionKeyRoles:
		t.Roles = payload.Roles
	case domain.SectionKeyCommittees:
		t.Committees = payload.Committees
	case domain.SectionKeyMakeup:
		t.MakeupRooms = payload.MakeupRooms
	case domain.SectionKeyAcaraAkad:
		t.ItemsAkad = payload.Items
	case domain.SectionKeyAcaraResepsi:
		t.ItemsResepsi = payload.Items
	case domain.SectionKeyLayout:
		t.LayoutNotes = payload.LayoutNotes
	default:
		return fmt.Errorf("seksi %s tidak termasuk template", section)
	}

	encoded, err := json.Marshal(templateToDoc(t))
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE rundown_templates SET payload = ? WHERE tenant_id = ?", encoded, tenantID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *MySQLRundownTemplateRepository) ReplaceAll(ctx context.Context, t *domain.Template) error {
	encoded, err := json.Marshal(templateToDoc(t))
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO rundown_templates (tenant_id, payload) VALUES (?, ?)
		 ON DUPLICATE KEY UPDATE payload = VALUES(payload)`, t.TenantID, encoded)
	return err
}

func docToTemplate(doc templateDoc) *domain.Template {
	t := &domain.Template{}
	for i, x := range doc.Roles {
		t.Roles = append(t.Roles, domain.Role{SortOrder: i, RoleLabel: x.RoleLabel, PersonName: x.PersonName, Note: x.Note})
	}
	for i, x := range doc.Committees {
		t.Committees = append(t.Committees, domain.Committee{SortOrder: i, RoleLabel: x.RoleLabel, PersonText: x.PersonText, JobDesc: x.JobDesc})
	}
	for i, room := range doc.MakeupRooms {
		m := domain.MakeupRoom{SortOrder: i, RoomLabel: room.RoomLabel}
		for j, l := range room.Lines {
			m.Lines = append(m.Lines, domain.MakeupLine{SortOrder: j, Style: domain.MakeupStyle(l.Style), Content: l.Content})
		}
		t.MakeupRooms = append(t.MakeupRooms, m)
	}
	t.ItemsAkad = docItems(doc.ItemsAkad, domain.SectionAkad)
	t.ItemsResepsi = docItems(doc.ItemsResepsi, domain.SectionResepsi)
	for i, x := range doc.LayoutNotes {
		t.LayoutNotes = append(t.LayoutNotes, domain.LayoutNote{SortOrder: i, Kind: domain.LayoutNoteKind(x.Kind), NumberLabel: x.NumberLabel, Content: x.Content})
	}
	return t
}

func docItems(rows []templateItem, section domain.Section) []domain.Item {
	var out []domain.Item
	for i, x := range rows {
		out = append(out, domain.Item{Section: section, SortOrder: i, NoLabel: x.NoLabel,
			TimeLabel: x.TimeLabel, Item: x.Item, PIC: x.PIC, Note: x.Note})
	}
	return out
}

func templateToDoc(t *domain.Template) templateDoc {
	// Slice kosong, bukan nil: dokumen tersimpan selalu berisi array, jadi
	// pembacanya tidak perlu membedakan null dari kosong.
	doc := templateDoc{V: templateDocVersion,
		Roles: []templateRole{}, Committees: []templateCommit{}, MakeupRooms: []templateRoom{},
		ItemsAkad: []templateItem{}, ItemsResepsi: []templateItem{}, LayoutNotes: []templateLayout{}}
	for _, x := range t.Roles {
		doc.Roles = append(doc.Roles, templateRole{RoleLabel: x.RoleLabel, PersonName: x.PersonName, Note: x.Note})
	}
	for _, x := range t.Committees {
		doc.Committees = append(doc.Committees, templateCommit{RoleLabel: x.RoleLabel, PersonText: x.PersonText, JobDesc: x.JobDesc})
	}
	for _, room := range t.MakeupRooms {
		m := templateRoom{RoomLabel: room.RoomLabel, Lines: []templateLine{}}
		for _, l := range room.Lines {
			m.Lines = append(m.Lines, templateLine{Style: string(l.Style), Content: l.Content})
		}
		doc.MakeupRooms = append(doc.MakeupRooms, m)
	}
	for _, x := range t.ItemsAkad {
		doc.ItemsAkad = append(doc.ItemsAkad, templateItem{NoLabel: x.NoLabel, TimeLabel: x.TimeLabel, Item: x.Item, PIC: x.PIC, Note: x.Note})
	}
	for _, x := range t.ItemsResepsi {
		doc.ItemsResepsi = append(doc.ItemsResepsi, templateItem{NoLabel: x.NoLabel, TimeLabel: x.TimeLabel, Item: x.Item, PIC: x.PIC, Note: x.Note})
	}
	for _, x := range t.LayoutNotes {
		doc.LayoutNotes = append(doc.LayoutNotes, templateLayout{Kind: string(x.Kind), NumberLabel: x.NumberLabel, Content: x.Content})
	}
	return doc
}
