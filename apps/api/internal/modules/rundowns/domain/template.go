package domain

import "time"

// Template adalah isi standar per tenant yang disalin ke rundown baru saat
// dibuat (PLAN rundown-ux-ideal §6.4). Hanya enam seksi yang memang sama dari
// satu acara ke acara lain; sampul, vendor, dan lampiran khas tiap pasangan.
//
// Memakai ulang tipe baris aggregate rundown supaya validasi, penomoran, dan
// editor frontend-nya sama persis.
type Template struct {
	TenantID     int64
	Roles        []Role
	Committees   []Committee
	MakeupRooms  []MakeupRoom
	ItemsAkad    []Item
	ItemsResepsi []Item
	LayoutNotes  []LayoutNote
	UpdatedAt    time.Time
}

// TemplateSectionKeys adalah seksi yang ikut template (keputusan D2).
var TemplateSectionKeys = []SectionKey{
	SectionKeyRoles, SectionKeyCommittees, SectionKeyMakeup,
	SectionKeyAcaraAkad, SectionKeyAcaraResepsi, SectionKeyLayout,
}

// InTemplate melaporkan apakah seksi ini bagian dari Template Rundown.
func (k SectionKey) InTemplate() bool {
	for _, known := range TemplateSectionKeys {
		if k == known {
			return true
		}
	}
	return false
}

// IsEmpty benar bila tidak satu pun seksi template berisi baris.
func (t *Template) IsEmpty() bool {
	return len(t.Roles) == 0 && len(t.Committees) == 0 && len(t.MakeupRooms) == 0 &&
		len(t.ItemsAkad) == 0 && len(t.ItemsResepsi) == 0 && len(t.LayoutNotes) == 0
}
