package presentation

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"regexp"
	"strconv"
	"strings"

	"jwswedding/internal/modules/rundowns/domain"
)

const (
	documentEntry = "word/document.xml"
	// layoutImageEntry adalah entry PNG diagram denah akad di dalam template,
	// dan layoutImageRelID adalah r:embed yang menunjuknya. Keduanya properti
	// template, bukan konfigurasi -- kalau template dibangun ulang dengan
	// gambar di slot lain, dua konstanta ini ikut berubah.
	layoutImageEntry = "word/media/image10.png"
	layoutImageRelID = "rId12"
)

// Jumlah baris minimum tiga tabel lampiran. Berkas contoh memang mencetaknya
// dengan baris kosong berlebih supaya bisa diisi tangan di lapangan; tanpa
// padding ini tabelnya akan menyusut dan tidak lagi sama dengan aslinya.
const (
	minPhotoGroupRows = 21
	minVIPGuestRows   = 15
	minPlaylistRows   = 10
)

var leftoverPlaceholder = regexp.MustCompile(`\{\{[#/~]?[A-Za-z_]+\}\}`)

// buildRundownDocx menghasilkan berkas .docx buku acara.
//
// Seluruh entry zip template disalin apa adanya byte per byte; hanya
// word/document.xml (dan entry PNG denah, bila WO sudah mengunggahnya) yang
// ditulis ulang. Itulah yang membuat ornamen, kop monogram, bingkai, border,
// dan font tetap identik dengan berkas contoh -- semuanya tidak pernah
// digambar ulang.
func buildRundownDocx(v *domain.View, template []byte, layoutPNG []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(template), int64(len(template)))
	if err != nil {
		return nil, fmt.Errorf("template rundown tidak terbaca: %w", err)
	}

	var docXML string
	for _, f := range zr.File {
		if f.Name != documentEntry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		_, err = buf.ReadFrom(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		docXML = buf.String()
	}
	if docXML == "" {
		return nil, fmt.Errorf("template rundown tidak memuat %s", documentEntry)
	}

	docXML, err = renderDocumentXML(docXML, v, len(layoutPNG) > 0, layoutPNG)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:   f.Name,
			Method: f.Method,
		})
		if err != nil {
			return nil, err
		}
		switch {
		case f.Name == documentEntry:
			if _, err := w.Write([]byte(docXML)); err != nil {
				return nil, err
			}
		case f.Name == layoutImageEntry && len(layoutPNG) > 0:
			if _, err := w.Write(layoutPNG); err != nil {
				return nil, err
			}
		default:
			// Disalin apa adanya: ornamen, kop monogram, bingkai, font yang
			// ditanam, relasi, style. Inilah yang menjaga hasilnya identik
			// dengan berkas contoh.
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(w, rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderDocumentXML(xml string, v *domain.View, hasLayout bool, layoutPNG []byte) (string, error) {
	var err error

	// --- region paragraf ber-style ---------------------------------------
	if xml, err = expandStyledRegion(xml, "vendors", vendorLines(v.Vendors)); err != nil {
		return "", err
	}
	for name, group := range map[string]domain.MenuGroup{
		"menuStall":     domain.MenuGroupStall,
		"menuBuffet":    domain.MenuGroupBuffet,
		"menuAfterAkad": domain.MenuGroupAfterAkad,
	} {
		if xml, err = expandStyledRegion(xml, name, menuLines(v.MenuItems, group)); err != nil {
			return "", err
		}
	}
	if xml, err = expandStyledRegion(xml, "layoutRules", ruleLines(v.LayoutNotes)); err != nil {
		return "", err
	}
	if xml, err = expandStyledRegion(xml, "playlistNotes", noteLines(v.Rundown.PlaylistNotes)); err != nil {
		return "", err
	}

	// --- baris tabel ------------------------------------------------------
	// makeupRooms lebih dulu: tiap salinan barisnya masih memuat region
	// makeupLines yang baru diperluas sesudahnya, per ruangan.
	if xml, err = expandMakeupRooms(xml, v.MakeupRooms); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "roles", roleRows(v.Roles)); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "committees", committeeRows(v.Committees)); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "itemsAkad", itemRows(v.Items, domain.SectionAkad)); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "itemsResepsi", itemRows(v.Items, domain.SectionResepsi)); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "layoutLegend", legendRows(v.LayoutNotes)); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "photoGroups", photoRows(v.PhotoGroups)); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "vipGuests", vipRows(v.VIPGuests)); err != nil {
		return "", err
	}
	if xml, err = expandRows(xml, "playlist", playlistRows(v.Playlist)); err != nil {
		return "", err
	}

	// --- skalar -----------------------------------------------------------
	xml = substitute(xml, scalarValues(v.Rundown))

	if hasLayout {
		if xml, err = resizeLayoutImage(xml, layoutPNG); err != nil {
			return "", err
		}
	}

	if m := leftoverPlaceholder.FindString(xml); m != "" {
		return "", fmt.Errorf("masih ada penanda template yang belum terisi: %s", m)
	}
	return xml, nil
}

// expandMakeupRooms menggandakan baris ruangan makeup, lalu mengisi region
// makeupLines di dalam tiap salinannya. Dua tingkat, jadi tidak bisa memakai
// expandRows begitu saja.
func expandMakeupRooms(xml string, rooms []domain.MakeupRoom) (string, error) {
	openTag, closeTag := "{{#makeupRooms}}", "{{/makeupRooms}}"
	start, end, tmpl, err := elementContaining(xml, openTag, "w:tr")
	if err != nil {
		return "", err
	}
	// Penomoran tiap ruangan hanya mengulang dari 1 kalau setiap salinan baris
	// memakai numId sendiri (assignRoomNumbering). Itu bergantung pada dua
	// string yang harus ada PERSIS di baris cetakan. Kalau template dibangun
	// ulang dan bentuknya bergeser sedikit saja — `w:val="2" />` dengan spasi,
	// misalnya — penggantiannya menjadi no-op dan ruangan kedua menyambung
	// "2." tanpa satu pun tanda kesalahan. Diperiksa di sini supaya pergeseran
	// itu ketahuan sebagai galat, bukan sebagai dokumen yang diam-diam salah.
	for _, id := range []string{templateNumberedID, templateDashID} {
		if !strings.Contains(tmpl, numIDTag(id)) {
			return "", fmt.Errorf("baris cetakan ruangan makeup tidak memuat %s "+
				"— penomoran per ruangan tidak bisa dijamin mulai dari 1", numIDTag(id))
		}
	}
	clean := strings.NewReplacer(openTag, "", closeTag, "")
	var b strings.Builder
	for i, room := range rooms {
		one := substitute(clean.Replace(tmpl), map[string]string{"room_label": room.RoomLabel})
		lines := make([]styledLine, 0, len(room.Lines))
		for _, l := range room.Lines {
			lines = append(lines, styledLine{Style: l.Style.TemplateStyle(), Text: l.Content})
		}
		one, err = expandStyledRegion(one, "makeupLines", lines)
		if err != nil {
			return "", err
		}
		b.WriteString(assignRoomNumbering(one, i))
	}
	if len(rooms) == 0 {
		// Tanpa satu pun ruangan, baris cetakan tetap harus lenyap -- termasuk
		// region makeupLines di dalamnya, yang kalau tertinggal akan terbaca
		// sebagai penanda yang belum terisi.
		return xml[:start] + xml[end:], nil
	}
	return xml[:start] + b.String() + xml[end:], nil
}

// Kolam numId yang disiapkan build_template.py. Tanpa ini seluruh salinan
// baris ruangan makeup berbagi satu daftar, dan Word melanjutkan hitungannya
// (ruangan kedua mulai dari "2.") alih-alih mengulang dari "1." seperti di
// berkas contoh. Nilainya harus sama persis dengan konstanta di skrip itu.
const (
	makeupRoomPool     = 16
	numberedPoolBase   = 900
	dashPoolBase       = 920
	templateNumberedID = "2"
	templateDashID     = "17"
)

// assignRoomNumbering memberi satu salinan baris ruangan daftar penomorannya
// sendiri. Ruangan ke-17 dan seterusnya memakai slot terakhir -- penomorannya
// ikut menyambung, konsekuensi yang jauh lebih ringan daripada menolak
// menyimpan rundown yang punya banyak ruangan.
func assignRoomNumbering(row string, index int) string {
	if index >= makeupRoomPool {
		index = makeupRoomPool - 1
	}
	rep := strings.NewReplacer(
		numIDTag(templateNumberedID), numIDTag(strconv.Itoa(numberedPoolBase+index)),
		numIDTag(templateDashID), numIDTag(strconv.Itoa(dashPoolBase+index)),
	)
	return rep.Replace(row)
}

func numIDTag(id string) string { return `<w:numId w:val="` + id + `"/>` }

// scalarValues memetakan kolom tabel akar ke penanda {{key}} di template.
//
// Fungsi tersendiri, bukan map inline, karena ada tes yang memeriksa SETIAP
// kunci di sini benar-benar punya placeholder di rundown_template.docx.
// Penjaga itu ada sebabnya: versi pertama modul ini memasok "wo_pic" dan
// "wo_phone" ke template yang blok ORGANIZED BY-nya sudah telanjur terbuang
// saat template dibangun, dan "table_cloth_note" bahkan tidak pernah dipasok
// sama sekali — ketiganya diisi WO, tersimpan rapi di basis data, lalu hilang
// tanpa suara di cetakan. substitute() memang mengabaikan kunci yang tidak
// punya placeholder, jadi tidak ada yang gagal; justru itu masalahnya.
func scalarValues(r domain.Rundown) map[string]string {
	return map[string]string{
		"groom_name":        r.GroomName,
		"groom_birth_order": r.GroomBirthOrder,
		"groom_parents":     r.GroomParents,
		"bride_name":        r.BrideName,
		"bride_birth_order": r.BrideBirthOrder,
		"bride_parents":     r.BrideParents,
		"event_date_label":  r.EventDateLabel,
		"venue_label":       r.VenueLabel,
		"event_time_label":  r.EventTimeLabel,
		"couple_title":      r.CoupleTitle,
		"siblings_bride":    r.SiblingsBride,
		"siblings_groom":    r.SiblingsGroom,
		"souvenir_note":     r.SouvenirNote,
		"table_cloth_note":  r.TableClothNote,
		"wo_contact":        woContact(r.WOPICName, r.WOPICPhone),
	}
}

// woContact menyusun baris kontak di blok ORGANIZED BY: "(Amanda/0856-...)".
// Satu skalar, bukan dua, supaya kurung dan garis miringnya tidak ikut
// tercetak saat kedua kolomnya masih kosong — buku acara memang boleh
// dicetak setengah jadi (Q4), jadi keadaan itu lumrah, bukan kasus tepi.
func woContact(name, phone string) string {
	name, phone = strings.TrimSpace(name), strings.TrimSpace(phone)
	switch {
	case name == "" && phone == "":
		return ""
	case phone == "":
		return "(" + name + ")"
	case name == "":
		return "(" + phone + ")"
	}
	return "(" + name + "/" + phone + ")"
}

// ------------------------------------------------------------ penyusun data

// vendorLines mengubah daftar datar menjadi urutan baris bergaya: satu baris
// kategori, lalu satu baris per vendor di bawahnya, lalu satu baris pemisah.
// Baris kategori hanya dicetak saat kategorinya berganti -- itulah yang
// membuat "VENUE & CATERING" dengan dua vendor tampil seperti di berkas
// contoh, bukan dua kali judul kategori.
func vendorLines(vendors []domain.Vendor) []styledLine {
	var out []styledLine
	prev := ""
	for i, v := range vendors {
		if v.CategoryLabel != prev {
			if i > 0 {
				out = append(out, styledLine{Style: "spacer"})
			}
			out = append(out, styledLine{Style: "category", Text: v.CategoryLabel})
			prev = v.CategoryLabel
		}
		out = append(out, styledLine{Style: "vendor", Text: v.VendorName})
	}
	return out
}

func menuLines(items []domain.MenuItem, group domain.MenuGroup) []styledLine {
	var out []styledLine
	for _, m := range items {
		if m.GroupKey != group {
			continue
		}
		out = append(out, styledLine{Style: m.Style.TemplateStyle(), Text: m.Content})
	}
	return out
}

func ruleLines(notes []domain.LayoutNote) []styledLine {
	var out []styledLine
	for _, n := range notes {
		if n.Kind == domain.LayoutNoteRule {
			out = append(out, styledLine{Style: "rule", Text: n.Content})
		}
	}
	return out
}

func noteLines(text string) []styledLine {
	var out []styledLine
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, styledLine{Style: "note", Text: line})
	}
	return out
}

func roleRows(rows []domain.Role) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]string{
			"role_label": r.RoleLabel, "person_name": r.PersonName, "note": r.Note,
		})
	}
	return out
}

func committeeRows(rows []domain.Committee) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]string{
			"role_label": r.RoleLabel, "person_text": r.PersonText, "job_desc": r.JobDesc,
		})
	}
	return out
}

func itemRows(items []domain.Item, section domain.Section) []map[string]string {
	out := []map[string]string{}
	for _, it := range items {
		if it.Section != section {
			continue
		}
		out = append(out, map[string]string{
			"no_label": it.NoLabel, "time_label": it.TimeLabel,
			"item": it.Item, "pic": it.PIC, "note": it.Note,
		})
	}
	return out
}

func legendRows(notes []domain.LayoutNote) []map[string]string {
	out := []map[string]string{}
	for _, n := range notes {
		if n.Kind != domain.LayoutNoteLegend {
			continue
		}
		out = append(out, map[string]string{
			"number_label": n.NumberLabel, "content": n.Content,
		})
	}
	return out
}

// padded menjamin tabel lampiran tetap sepanjang aslinya: baris yang tidak
// terisi data tetap tercetak kosong bernomor, persis seperti berkas contoh
// yang memang disiapkan untuk diisi tangan di lapangan.
func padded(rows []map[string]string, min int, blank func() map[string]string) []map[string]string {
	for len(rows) < min {
		rows = append(rows, blank())
	}
	for i := range rows {
		rows[i]["number_label"] = strconv.Itoa(i + 1)
	}
	return rows
}

func photoRows(groups []domain.PhotoGroup) []map[string]string {
	rows := []map[string]string{}
	for _, g := range groups {
		rows = append(rows, map[string]string{"group_name": g.GroupName})
	}
	return padded(rows, minPhotoGroupRows, func() map[string]string {
		return map[string]string{"group_name": ""}
	})
}

func vipRows(guests []domain.VIPGuest) []map[string]string {
	rows := []map[string]string{}
	for _, g := range guests {
		rows = append(rows, map[string]string{"full_name": g.FullName, "position": g.Position})
	}
	return padded(rows, minVIPGuestRows, func() map[string]string {
		return map[string]string{"full_name": "", "position": ""}
	})
}

func playlistRows(entries []domain.PlaylistEntry) []map[string]string {
	rows := []map[string]string{}
	for _, e := range entries {
		rows = append(rows, map[string]string{"title": e.Title, "artist": e.Artist})
	}
	return padded(rows, minPlaylistRows, func() map[string]string {
		return map[string]string{"title": "", "artist": ""}
	})
}

// ------------------------------------------------------------ gambar denah

var (
	wpExtentRe = regexp.MustCompile(`<wp:extent cx="(\d+)" cy="(\d+)"`)
	aExtRe     = regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"`)
)

// resizeLayoutImage menyesuaikan tinggi bingkai gambar denah dengan rasio
// aspek PNG yang diunggah, dengan lebar dipertahankan. Tanpa ini, denah yang
// rasionya berbeda dari gambar bawaan akan tercetak gepeng -- ukuran tampil
// ditentukan wp:extent di document.xml, bukan oleh dimensi PNG-nya.
func resizeLayoutImage(xml string, png []byte) (string, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(png))
	if err != nil {
		return "", fmt.Errorf("denah akad tidak terbaca sebagai gambar: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return "", fmt.Errorf("denah akad berukuran %dx%d piksel", cfg.Width, cfg.Height)
	}

	// Mulai dari sini semuanya GALAT, bukan "kembalikan apa adanya". Ketiga
	// penanda di bawah adalah properti template: kalau salah satunya tidak
	// ketemu, template-nya sudah bergeser dari yang diharapkan renderer, dan
	// diam-diam melewatkan penyesuaian ukuran akan mencetak denah gepeng —
	// byte PNG-nya tetap ditukar di buildRundownDocx, hanya bingkainya yang
	// masih memakai rasio gambar lama. Gagal keras jauh lebih murah daripada
	// dokumen yang salah tapi kelihatan wajar.
	relMark := `r:embed="` + layoutImageRelID + `"`
	relPos := strings.Index(xml, relMark)
	if relPos < 0 {
		return "", fmt.Errorf("template tidak memuat %s (slot gambar denah bergeser?)", relMark)
	}
	start, end, err := enclosingElement(xml, relPos, "w:drawing")
	if err != nil {
		return "", fmt.Errorf("bingkai <w:drawing> gambar denah tidak terbaca: %w", err)
	}
	block := xml[start:end]

	m := wpExtentRe.FindStringSubmatch(block)
	if m == nil {
		return "", fmt.Errorf("bingkai gambar denah tidak punya <wp:extent>")
	}
	cx, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || cx <= 0 {
		return "", fmt.Errorf("lebar <wp:extent> gambar denah tidak masuk akal: %q", m[1])
	}
	cy := cx * int64(cfg.Height) / int64(cfg.Width)

	block = wpExtentRe.ReplaceAllString(block,
		fmt.Sprintf(`<wp:extent cx="%d" cy="%d"`, cx, cy))
	block = aExtRe.ReplaceAllString(block,
		fmt.Sprintf(`<a:ext cx="%d" cy="%d"`, cx, cy))
	return xml[:start] + block + xml[end:], nil
}
