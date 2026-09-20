package domain

// Nilai-nilai enum di bawah ini tidak sekadar mencerminkan kolom ENUM di
// MySQL: sebagian juga memilih paragraf-template mana di dalam berkas .docx
// yang dipakai untuk mencetak barisnya. Karena itu menambah nilai baru di sini
// berarti menambah paragraf-template baru di rundown_template.docx juga —
// kalau tidak, renderer tidak punya cetakan untuk baris itu.

// Section membedakan dua tabel SUSUNAN ACARA.
type Section string

const (
	SectionAkad    Section = "Akad"
	SectionResepsi Section = "Resepsi"
)

// AllSections dipakai validator dan penomoran ulang per seksi.
var AllSections = []Section{SectionAkad, SectionResepsi}

func (s Section) Valid() bool {
	return s == SectionAkad || s == SectionResepsi
}

// MenuGroup adalah tiga kolom pada tabel DATA LAINNYA.
type MenuGroup string

const (
	MenuGroupStall     MenuGroup = "Stall"
	MenuGroupBuffet    MenuGroup = "Buffet"
	MenuGroupAfterAkad MenuGroup = "AfterAkad"
)

var AllMenuGroups = []MenuGroup{MenuGroupStall, MenuGroupBuffet, MenuGroupAfterAkad}

func (g MenuGroup) Valid() bool {
	switch g {
	case MenuGroupStall, MenuGroupBuffet, MenuGroupAfterAkad:
		return true
	}
	return false
}

// MenuStyle memilih paragraf-template di dalam sel DATA LAINNYA.
type MenuStyle string

const (
	MenuStyleHeading  MenuStyle = "Heading"
	MenuStyleNumbered MenuStyle = "Numbered"
	MenuStyleBullet   MenuStyle = "Bullet"
	MenuStylePlain    MenuStyle = "Plain"
)

func (s MenuStyle) Valid() bool {
	switch s {
	case MenuStyleHeading, MenuStyleNumbered, MenuStyleBullet, MenuStylePlain:
		return true
	}
	return false
}

// TemplateStyle memetakan MenuStyle ke nama style di template. 'Bullet' belum
// punya cetakan sendiri di rundown_template.docx sehingga jatuh ke 'numbered';
// dipisahkan di sini supaya penambahan cetakannya nanti cukup satu baris.
func (s MenuStyle) TemplateStyle() string {
	switch s {
	case MenuStyleHeading:
		return "heading"
	case MenuStylePlain:
		return "plain"
	case MenuStyleBullet:
		return "numbered"
	default:
		return "numbered"
	}
}

// MakeupStyle memilih paragraf-template di dalam sel LIST & RUANGAN MAKEUP.
type MakeupStyle string

const (
	MakeupStyleHeading  MakeupStyle = "Heading"
	MakeupStyleNumbered MakeupStyle = "Numbered"
	MakeupStyleDash     MakeupStyle = "Dash"
	MakeupStyleNote     MakeupStyle = "Note"
)

func (s MakeupStyle) Valid() bool {
	switch s {
	case MakeupStyleHeading, MakeupStyleNumbered, MakeupStyleDash, MakeupStyleNote:
		return true
	}
	return false
}

func (s MakeupStyle) TemplateStyle() string {
	switch s {
	case MakeupStyleHeading:
		return "heading"
	case MakeupStyleDash:
		return "dash"
	case MakeupStyleNote:
		return "note"
	default:
		return "numbered"
	}
}

// LayoutNoteKind memisahkan aturan tamu dari tabel keterangan denah.
type LayoutNoteKind string

const (
	LayoutNoteLegend LayoutNoteKind = "Legend"
	LayoutNoteRule   LayoutNoteKind = "Rule"
)

func (k LayoutNoteKind) Valid() bool {
	return k == LayoutNoteLegend || k == LayoutNoteRule
}

// SectionKey adalah nama seksi sebagaimana muncul di URL
// PUT /rundowns/{id}/sections/{section}.
type SectionKey string

const (
	SectionKeyCover        SectionKey = "cover"
	SectionKeyVendors      SectionKey = "vendors"
	SectionKeyRoles        SectionKey = "roles"
	SectionKeyCommittees   SectionKey = "committees"
	SectionKeyDataLainnya  SectionKey = "data-lainnya"
	SectionKeyMakeup       SectionKey = "makeup"
	SectionKeyAcaraAkad    SectionKey = "acara-akad"
	SectionKeyAcaraResepsi SectionKey = "acara-resepsi"
	SectionKeyLayout       SectionKey = "layout"
	SectionKeyFotoTamu     SectionKey = "foto-tamu"
	SectionKeyTamuVIP      SectionKey = "tamu-vip"
	SectionKeyPlaylist     SectionKey = "playlist"
)

// AllSectionKeys adalah dua belas kunci seksi yang dikenal API dan editor.
// Dua belas, bukan sepuluh seperti cara WO menghitung halaman: Cover berdiri
// sendiri dan SUSUNAN ACARA dipecah jadi akad dan resepsi.
var AllSectionKeys = []SectionKey{
	SectionKeyCover, SectionKeyVendors, SectionKeyRoles, SectionKeyCommittees,
	SectionKeyDataLainnya, SectionKeyMakeup, SectionKeyAcaraAkad,
	SectionKeyAcaraResepsi, SectionKeyLayout, SectionKeyFotoTamu,
	SectionKeyTamuVIP, SectionKeyPlaylist,
}

func (k SectionKey) Valid() bool {
	for _, known := range AllSectionKeys {
		if k == known {
			return true
		}
	}
	return false
}
