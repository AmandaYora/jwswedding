package presentation

import (
	"strconv"
	"time"

	"jwswedding/internal/modules/rundowns/application"
	"jwswedding/internal/modules/rundowns/domain"
)

// ID dikirim sebagai string, mengikuti kebiasaan modul lain di repo ini:
// frontend memperlakukan id sebagai string di seluruh store-nya.

type createRundownRequest struct {
	ProjectID      int64        `json:"projectId"`
	WOPICName      string       `json:"woPicName"`
	WOPICPhone     string       `json:"woPicPhone"`
	EventTimeLabel string       `json:"eventTimeLabel"`
	Vendors        []vendorItem `json:"vendors"`
	// UseTemplate menyalin isi Template Rundown ke rundown baru.
	UseTemplate bool `json:"useTemplate"`
}

func (r createRundownRequest) toInput() application.CreateInput {
	vendors := make([]domain.Vendor, 0, len(r.Vendors))
	for i, v := range r.Vendors {
		vendors = append(vendors, domain.Vendor{
			SortOrder: i, CategoryLabel: v.CategoryLabel, VendorName: v.VendorName,
		})
	}
	return application.CreateInput{
		ProjectID:      r.ProjectID,
		VendorPrefill:  vendors,
		WOPICName:      r.WOPICName,
		WOPICPhone:     r.WOPICPhone,
		EventTimeLabel: r.EventTimeLabel,
		UseTemplate:    r.UseTemplate,
	}
}

type layoutImageRequest struct {
	Base64Data string `json:"base64Data"`
}

type vendorItem struct {
	CategoryLabel string `json:"categoryLabel"`
	VendorName    string `json:"vendorName"`
}

type roleItem struct {
	RoleLabel  string `json:"roleLabel"`
	PersonName string `json:"personName"`
	Note       string `json:"note"`
}

type committeeItem struct {
	RoleLabel  string `json:"roleLabel"`
	PersonText string `json:"personText"`
	JobDesc    string `json:"jobDesc"`
}

type menuItem struct {
	GroupKey string `json:"groupKey"`
	Style    string `json:"style"`
	Content  string `json:"content"`
}

type makeupLineItem struct {
	Style   string `json:"style"`
	Content string `json:"content"`
}

type makeupRoomItem struct {
	RoomLabel string           `json:"roomLabel"`
	Lines     []makeupLineItem `json:"lines"`
}

type acaraItem struct {
	NoLabel   string `json:"noLabel"`
	TimeLabel string `json:"timeLabel"`
	Item      string `json:"item"`
	PIC       string `json:"pic"`
	Note      string `json:"note"`
}

type layoutNoteItem struct {
	Kind        string `json:"kind"`
	NumberLabel string `json:"numberLabel"`
	Content     string `json:"content"`
}

type photoGroupItem struct {
	GroupName string `json:"groupName"`
}

type vipGuestItem struct {
	FullName string `json:"fullName"`
	Position string `json:"position"`
}

type playlistItem struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

type coverPayload struct {
	GroomName       string `json:"groomName"`
	GroomBirthOrder string `json:"groomBirthOrder"`
	GroomParents    string `json:"groomParents"`
	BrideName       string `json:"brideName"`
	BrideBirthOrder string `json:"brideBirthOrder"`
	BrideParents    string `json:"brideParents"`
	EventDateLabel  string `json:"eventDateLabel"`
	VenueLabel      string `json:"venueLabel"`
	EventTimeLabel  string `json:"eventTimeLabel"`
	CoupleTitle     string `json:"coupleTitle"`
	WOPICName       string `json:"woPicName"`
	WOPICPhone      string `json:"woPicPhone"`
}

type dataLainnyaPayload struct {
	SiblingsBride  string `json:"siblingsBride"`
	SiblingsGroom  string `json:"siblingsGroom"`
	SouvenirNote   string `json:"souvenirNote"`
	TableClothNote string `json:"tableClothNote"`
}

// sectionRequest adalah amplop satu PUT seksi. Hanya field yang relevan
// dengan seksi bersangkutan yang dibaca; sisanya diabaikan, sehingga frontend
// boleh mengirim bentuk yang sama untuk semua seksi.
type sectionRequest struct {
	Cover       coverPayload       `json:"cover"`
	DataLainnya dataLainnyaPayload `json:"dataLainnya"`

	Vendors     []vendorItem     `json:"vendors"`
	Roles       []roleItem       `json:"roles"`
	Committees  []committeeItem  `json:"committees"`
	MenuItems   []menuItem       `json:"menuItems"`
	MakeupRooms []makeupRoomItem `json:"makeupRooms"`
	Items       []acaraItem      `json:"items"`
	LayoutNotes []layoutNoteItem `json:"layoutNotes"`
	PhotoGroups []photoGroupItem `json:"photoGroups"`
	VIPGuests   []vipGuestItem   `json:"vipGuests"`
	Playlist    []playlistItem   `json:"playlist"`

	PlaylistNotes string `json:"playlistNotes"`
}

func (s sectionRequest) toPayload() application.SectionPayload {
	p := application.SectionPayload{
		Cover: application.CoverPayload{
			GroomName: s.Cover.GroomName, GroomBirthOrder: s.Cover.GroomBirthOrder,
			GroomParents: s.Cover.GroomParents, BrideName: s.Cover.BrideName,
			BrideBirthOrder: s.Cover.BrideBirthOrder, BrideParents: s.Cover.BrideParents,
			EventDateLabel: s.Cover.EventDateLabel, VenueLabel: s.Cover.VenueLabel,
			EventTimeLabel: s.Cover.EventTimeLabel, CoupleTitle: s.Cover.CoupleTitle,
			WOPICName: s.Cover.WOPICName, WOPICPhone: s.Cover.WOPICPhone,
		},
		DataLainnya: application.DataLainnyaPayload{
			SiblingsBride: s.DataLainnya.SiblingsBride,
			SiblingsGroom: s.DataLainnya.SiblingsGroom,
			SouvenirNote:  s.DataLainnya.SouvenirNote,
			TableClothNote: s.DataLainnya.TableClothNote,
		},
		PlaylistNotes: s.PlaylistNotes,
	}
	for i, v := range s.Vendors {
		p.Vendors = append(p.Vendors, domain.Vendor{
			SortOrder: i, CategoryLabel: v.CategoryLabel, VendorName: v.VendorName})
	}
	for i, v := range s.Roles {
		p.Roles = append(p.Roles, domain.Role{
			SortOrder: i, RoleLabel: v.RoleLabel, PersonName: v.PersonName, Note: v.Note})
	}
	for i, v := range s.Committees {
		p.Committees = append(p.Committees, domain.Committee{
			SortOrder: i, RoleLabel: v.RoleLabel, PersonText: v.PersonText, JobDesc: v.JobDesc})
	}
	for i, v := range s.MenuItems {
		p.MenuItems = append(p.MenuItems, domain.MenuItem{
			SortOrder: i, GroupKey: domain.MenuGroup(v.GroupKey),
			Style: domain.MenuStyle(v.Style), Content: v.Content})
	}
	for i, room := range s.MakeupRooms {
		r := domain.MakeupRoom{SortOrder: i, RoomLabel: room.RoomLabel}
		for j, l := range room.Lines {
			r.Lines = append(r.Lines, domain.MakeupLine{
				SortOrder: j, Style: domain.MakeupStyle(l.Style), Content: l.Content})
		}
		p.MakeupRooms = append(p.MakeupRooms, r)
	}
	for i, v := range s.Items {
		p.Items = append(p.Items, domain.Item{
			SortOrder: i, NoLabel: v.NoLabel, TimeLabel: v.TimeLabel,
			Item: v.Item, PIC: v.PIC, Note: v.Note})
	}
	for i, v := range s.LayoutNotes {
		p.LayoutNotes = append(p.LayoutNotes, domain.LayoutNote{
			SortOrder: i, Kind: domain.LayoutNoteKind(v.Kind),
			NumberLabel: v.NumberLabel, Content: v.Content})
	}
	for i, v := range s.PhotoGroups {
		p.PhotoGroups = append(p.PhotoGroups, domain.PhotoGroup{
			SortOrder: i, GroupName: v.GroupName})
	}
	for i, v := range s.VIPGuests {
		p.VIPGuests = append(p.VIPGuests, domain.VIPGuest{
			SortOrder: i, FullName: v.FullName, Position: v.Position})
	}
	for i, v := range s.Playlist {
		p.Playlist = append(p.Playlist, domain.PlaylistEntry{
			SortOrder: i, Title: v.Title, Artist: v.Artist})
	}
	return p
}

// ------------------------------------------------------------------ keluaran

type summaryDTO struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"projectId"`
	ProjectName    string    `json:"projectName"`
	BrideName      string    `json:"brideName"`
	GroomName      string    `json:"groomName"`
	EventDateLabel string    `json:"eventDateLabel"`
	VenueLabel     string    `json:"venueLabel"`
	HasLayoutImage bool      `json:"hasLayoutImage"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func toSummaryDTOs(rows []domain.Summary) []summaryDTO {
	out := make([]summaryDTO, 0, len(rows))
	for _, s := range rows {
		out = append(out, summaryDTO{
			ID: strconv.FormatInt(s.ID, 10), ProjectID: strconv.FormatInt(s.ProjectID, 10),
			ProjectName: s.ProjectName, BrideName: s.BrideName, GroomName: s.GroomName,
			EventDateLabel: s.EventDateLabel, VenueLabel: s.VenueLabel,
			HasLayoutImage: s.HasLayoutImage, UpdatedAt: s.UpdatedAt,
		})
	}
	return out
}

type viewDTO struct {
	ID             string             `json:"id"`
	ProjectID      string             `json:"projectId"`
	ProjectName    string             `json:"projectName"`
	Cover          coverPayload       `json:"cover"`
	DataLainnya    dataLainnyaPayload `json:"dataLainnya"`
	Vendors        []vendorItem       `json:"vendors"`
	Roles          []roleItem         `json:"roles"`
	Committees     []committeeItem    `json:"committees"`
	MenuItems      []menuItem         `json:"menuItems"`
	MakeupRooms    []makeupRoomItem   `json:"makeupRooms"`
	ItemsAkad      []acaraItem        `json:"itemsAkad"`
	ItemsResepsi   []acaraItem        `json:"itemsResepsi"`
	LayoutNotes    []layoutNoteItem   `json:"layoutNotes"`
	PhotoGroups    []photoGroupItem   `json:"photoGroups"`
	VIPGuests      []vipGuestItem     `json:"vipGuests"`
	Playlist       []playlistItem     `json:"playlist"`
	PlaylistNotes  string             `json:"playlistNotes"`
	HasLayoutImage bool               `json:"hasLayoutImage"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

func toViewDTO(v *domain.View) viewDTO {
	r := v.Rundown
	out := viewDTO{
		ID: strconv.FormatInt(r.ID, 10), ProjectID: strconv.FormatInt(r.ProjectID, 10),
		ProjectName: r.ProjectName,
		Cover: coverPayload{
			GroomName: r.GroomName, GroomBirthOrder: r.GroomBirthOrder, GroomParents: r.GroomParents,
			BrideName: r.BrideName, BrideBirthOrder: r.BrideBirthOrder, BrideParents: r.BrideParents,
			EventDateLabel: r.EventDateLabel, VenueLabel: r.VenueLabel,
			EventTimeLabel: r.EventTimeLabel, CoupleTitle: r.CoupleTitle,
			WOPICName: r.WOPICName, WOPICPhone: r.WOPICPhone,
		},
		DataLainnya: dataLainnyaPayload{
			SiblingsBride: r.SiblingsBride, SiblingsGroom: r.SiblingsGroom,
			SouvenirNote: r.SouvenirNote, TableClothNote: r.TableClothNote,
		},
		PlaylistNotes:  r.PlaylistNotes,
		HasLayoutImage: r.LayoutImagePath != "",
		UpdatedAt:      r.UpdatedAt,
		// Slice kosong, bukan nil: frontend selalu menerima array sehingga
		// tidak perlu menjaga-jaga terhadap null di setiap seksi.
		Vendors: []vendorItem{}, Roles: []roleItem{}, Committees: []committeeItem{},
		MenuItems: []menuItem{}, MakeupRooms: []makeupRoomItem{},
		ItemsAkad: []acaraItem{}, ItemsResepsi: []acaraItem{},
		LayoutNotes: []layoutNoteItem{}, PhotoGroups: []photoGroupItem{},
		VIPGuests: []vipGuestItem{}, Playlist: []playlistItem{},
	}
	for _, x := range v.Vendors {
		out.Vendors = append(out.Vendors, vendorItem{CategoryLabel: x.CategoryLabel, VendorName: x.VendorName})
	}
	out.Roles = toRoleItems(v.Roles)
	out.Committees = toCommitteeItems(v.Committees)
	for _, x := range v.MenuItems {
		out.MenuItems = append(out.MenuItems, menuItem{
			GroupKey: string(x.GroupKey), Style: string(x.Style), Content: x.Content})
	}
	out.MakeupRooms = toMakeupRoomItems(v.MakeupRooms)
	for _, x := range v.Items {
		if x.Section == domain.SectionResepsi {
			out.ItemsResepsi = append(out.ItemsResepsi, toAcaraItem(x))
			continue
		}
		out.ItemsAkad = append(out.ItemsAkad, toAcaraItem(x))
	}
	out.LayoutNotes = toLayoutNoteItems(v.LayoutNotes)
	for _, x := range v.PhotoGroups {
		out.PhotoGroups = append(out.PhotoGroups, photoGroupItem{GroupName: x.GroupName})
	}
	for _, x := range v.VIPGuests {
		out.VIPGuests = append(out.VIPGuests, vipGuestItem{FullName: x.FullName, Position: x.Position})
	}
	for _, x := range v.Playlist {
		out.Playlist = append(out.Playlist, playlistItem{Title: x.Title, Artist: x.Artist})
	}
	return out
}

// noNumberMarker adalah penanda "baris tanpa nomor" di kolom No SUSUNAN ACARA
// (NO_NUMBER_MARKER di frontend).
const noNumberMarker = "-"

// toAcaraItem mengembalikan penanda tanpa-nomor ke frontend. renumberItems
// menyimpan baris tanpa nomor sebagai "" (supaya dokumen mencetak sel kosong),
// dan setiap baris bernomor pasti berangka setelah penomoran ulang — jadi ""
// di sini selalu berarti "tanpa nomor". Tanpa pemetaan ini, frontend menerima
// "" dan penyimpanan berikutnya diam-diam memberi baris itu nomor.
func toAcaraItem(x domain.Item) acaraItem {
	no := x.NoLabel
	if no == "" {
		no = noNumberMarker
	}
	return acaraItem{NoLabel: no, TimeLabel: x.TimeLabel, Item: x.Item, PIC: x.PIC, Note: x.Note}
}

func toRoleItems(rows []domain.Role) []roleItem {
	out := []roleItem{}
	for _, x := range rows {
		out = append(out, roleItem{RoleLabel: x.RoleLabel, PersonName: x.PersonName, Note: x.Note})
	}
	return out
}

func toCommitteeItems(rows []domain.Committee) []committeeItem {
	out := []committeeItem{}
	for _, x := range rows {
		out = append(out, committeeItem{RoleLabel: x.RoleLabel, PersonText: x.PersonText, JobDesc: x.JobDesc})
	}
	return out
}

func toMakeupRoomItems(rooms []domain.MakeupRoom) []makeupRoomItem {
	out := []makeupRoomItem{}
	for _, room := range rooms {
		item := makeupRoomItem{RoomLabel: room.RoomLabel, Lines: []makeupLineItem{}}
		for _, l := range room.Lines {
			item.Lines = append(item.Lines, makeupLineItem{Style: string(l.Style), Content: l.Content})
		}
		out = append(out, item)
	}
	return out
}

func toAcaraItems(rows []domain.Item) []acaraItem {
	out := []acaraItem{}
	for _, x := range rows {
		out = append(out, toAcaraItem(x))
	}
	return out
}

func toLayoutNoteItems(rows []domain.LayoutNote) []layoutNoteItem {
	out := []layoutNoteItem{}
	for _, x := range rows {
		out = append(out, layoutNoteItem{Kind: string(x.Kind), NumberLabel: x.NumberLabel, Content: x.Content})
	}
	return out
}

// templateDTO adalah bentuk Template Rundown di API. Nama field-nya sama
// dengan viewDTO supaya frontend bisa memakai editor seksi yang sama.
type templateDTO struct {
	Roles        []roleItem       `json:"roles"`
	Committees   []committeeItem  `json:"committees"`
	MakeupRooms  []makeupRoomItem `json:"makeupRooms"`
	ItemsAkad    []acaraItem      `json:"itemsAkad"`
	ItemsResepsi []acaraItem      `json:"itemsResepsi"`
	LayoutNotes  []layoutNoteItem `json:"layoutNotes"`
	// UpdatedAt nil = template belum pernah disimpan.
	UpdatedAt *time.Time `json:"updatedAt"`
}

func toTemplateDTO(t *domain.Template) templateDTO {
	out := templateDTO{
		Roles: toRoleItems(t.Roles), Committees: toCommitteeItems(t.Committees),
		MakeupRooms: toMakeupRoomItems(t.MakeupRooms),
		ItemsAkad:   toAcaraItems(t.ItemsAkad), ItemsResepsi: toAcaraItems(t.ItemsResepsi),
		LayoutNotes: toLayoutNoteItems(t.LayoutNotes),
	}
	if !t.UpdatedAt.IsZero() {
		updated := t.UpdatedAt
		out.UpdatedAt = &updated
	}
	return out
}

// coverPrefillDTO adalah usulan isian sampul dari data project terkini.
type coverPrefillDTO struct {
	GroomName      string `json:"groomName"`
	BrideName      string `json:"brideName"`
	EventDateLabel string `json:"eventDateLabel"`
	VenueLabel     string `json:"venueLabel"`
	EventTimeLabel string `json:"eventTimeLabel"`
	CoupleTitle    string `json:"coupleTitle"`
}

func toCoverPrefillDTO(c application.CoverPrefill) map[string]coverPrefillDTO {
	return map[string]coverPrefillDTO{"cover": {
		GroomName: c.GroomName, BrideName: c.BrideName, EventDateLabel: c.EventDateLabel,
		VenueLabel: c.VenueLabel, EventTimeLabel: c.EventTimeLabel, CoupleTitle: c.CoupleTitle,
	}}
}
