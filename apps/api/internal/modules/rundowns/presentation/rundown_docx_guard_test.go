package presentation

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/modules/rundowns/presentation/templates"
)

// templateDocumentXML membaca word/document.xml milik template apa adanya.
func templateDocumentXML(t *testing.T) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(templates.RundownDocx),
		int64(len(templates.RundownDocx)))
	if err != nil {
		t.Fatalf("template bukan zip yang sah: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != documentEntry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("buka %s: %v", documentEntry, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("baca %s: %v", documentEntry, err)
		}
		return string(b)
	}
	t.Fatalf("template tidak memuat %s", documentEntry)
	return ""
}

// Penjaga arah "data -> template".
//
// substitute() mengabaikan kunci yang tidak punya placeholder — tanpa suara,
// tanpa galat. Itulah yang dulu membuat wo_pic, wo_phone, dan table_cloth_note
// hilang dari cetakan meski diisi WO dan tersimpan rapi di basis data: blok
// ORGANIZED BY terbuang saat template dibangun, dan tidak ada apa pun yang
// memberi tahu. Tes ini yang memberi tahu.
//
// Kalau tes ini jatuh, ada DUA kemungkinan dan keduanya harus diperiksa:
// placeholder-nya hilang dari build_template.py, atau kuncinya memang tidak
// lagi dipakai dan harus dicabut dari scalarValues.
func TestScalarValues_EveryKeyHasPlaceholderInTemplate(t *testing.T) {
	xml := templateDocumentXML(t)
	for key := range scalarValues(domain.Rundown{}) {
		if !strings.Contains(xml, "{{"+key+"}}") {
			t.Errorf("scalarValues memasok %q tetapi template tidak punya {{%s}} — "+
				"nilainya akan hilang dari cetakan tanpa galat apa pun", key, key)
		}
	}
}

// Penjaga arah sebaliknya, "template -> data": setiap penanda skalar di
// template harus ada yang mengisinya. Penanda baris/region punya penjaganya
// sendiri (expandRows/expandStyledRegion gagal kalau penandanya tidak ketemu),
// jadi yang diperiksa di sini hanya yang tersisa setelah render penuh —
// itu dijamin oleh TestBuildRundownDocx_NoLeftoverPlaceholder. Yang ini
// melengkapi dengan memastikan tidak ada penanda skalar yatim yang kebetulan
// tertutupi karena barisnya ikut terhapus saat datanya kosong.
func TestTemplate_EveryScalarPlaceholderIsFed(t *testing.T) {
	xml := templateDocumentXML(t)
	fed := scalarValues(domain.Rundown{})
	// Kunci yang diisi per-baris oleh expandRows/expandStyledRegion, bukan
	// oleh scalarValues. Didaftar eksplisit supaya penanda baru yang tidak
	// diisi siapa pun tetap ketahuan.
	rowFed := map[string]bool{
		"role_label": true, "person_name": true, "note": true,
		"person_text": true, "job_desc": true,
		"room_label": true, "no_label": true, "time_label": true,
		"item": true, "pic": true,
		"number_label": true, "content": true, "group_name": true,
		"full_name": true, "position": true, "title": true, "artist": true,
	}
	for _, m := range scalarToken.FindAllStringSubmatch(xml, -1) {
		key := m[1]
		if _, ok := fed[key]; ok {
			continue
		}
		if rowFed[key] {
			continue
		}
		t.Errorf("template punya {{%s}} tetapi tidak ada yang mengisinya", key)
	}
}

// Regresi temuan 1: blok ORGANIZED BY pernah terbuang seluruhnya dari
// template, sehingga nama dan nomor PIC WO tidak punya tempat tercetak.
func TestBuildRundownDocx_OrganizedByBlockIsRendered(t *testing.T) {
	doc := string(entries(t, buildSample(t))[documentEntry])
	for _, want := range []string{"ORGANIZED BY", "JWS WEDDING", "(Amanda/0856-9360-1526)"} {
		if !strings.Contains(doc, want) {
			t.Errorf("blok ORGANIZED BY tidak lengkap: %q tidak ada di hasil", want)
		}
	}
}

// woContact tidak boleh mencetak kurung dan garis miring untuk kolom yang
// masih kosong — buku acara memang boleh dicetak setengah jadi (Q4).
func TestWOContact_OmitsPunctuationWhenEmpty(t *testing.T) {
	cases := []struct{ name, phone, want string }{
		{"", "", ""},
		{"Amanda", "", "(Amanda)"},
		{"", "0856", "(0856)"},
		{" Amanda ", " 0856 ", "(Amanda/0856)"},
	}
	for _, c := range cases {
		if got := woContact(c.name, c.phone); got != c.want {
			t.Errorf("woContact(%q, %q) = %q, mau %q", c.name, c.phone, got, c.want)
		}
	}
}

// Regresi temuan 2: table_cloth_note tersimpan di basis data dan ada isiannya
// di editor, tetapi tidak pernah sampai ke cetakan.
func TestBuildRundownDocx_TableClothNoteIsRendered(t *testing.T) {
	doc := string(entries(t, buildSample(t))[documentEntry])
	if !strings.Contains(doc, "Table Cloth") {
		t.Error("label Table Cloth tidak ada di hasil")
	}
	if !strings.Contains(doc, "meja VIP 4, reguler 500 porsi") {
		t.Error("isi table_cloth_note tidak tercetak")
	}
}

func TestXMLEscape_DropsIllegalControlCharacters(t *testing.T) {
	// 0x0B (vertical tab) lazim terbawa saat teks ditempel dari Excel.
	got := xmlEscape("Ibu\x0bAyu\x00 & Bapak\x1fBagas\tOK")
	for _, bad := range []string{"\x0b", "\x00", "\x1f"} {
		if strings.Contains(got, bad) {
			t.Fatalf("karakter kontrol ilegal masih tersisa: %q", got)
		}
	}
	if !strings.Contains(got, "&amp;") {
		t.Errorf("ampersand tidak di-escape: %q", got)
	}
	if !strings.Contains(got, "\t") {
		t.Errorf("tab seharusnya dipertahankan: %q", got)
	}
}

// Teks yang kebetulan berbentuk penanda template tidak boleh ikut
// tersubstitusi, dan tidak boleh menggagalkan generate. Sebelum diperbaiki,
// keduanya bisa terjadi: yang pertama tergantung urutan iterasi map (acak),
// yang kedua membuat rundown itu gagal di-generate untuk SETERUSNYA.
func TestSubstitute_UserTypedPlaceholderIsInertAndDeterministic(t *testing.T) {
	values := map[string]string{
		"bride_name": "{{groom_name}}",
		"groom_name": "BAGAS",
	}
	tpl := `<w:t>{{bride_name}}</w:t><w:t>{{groom_name}}</w:t>`
	first := substitute(tpl, values)
	for i := 0; i < 50; i++ {
		if got := substitute(tpl, values); got != first {
			t.Fatalf("hasil substitusi tidak deterministik:\n%s\n%s", first, got)
		}
	}
	if strings.Contains(first, "{{groom_name}}") {
		t.Errorf("teks pengguna yang berbentuk penanda masih utuh sebagai penanda: %q", first)
	}
	if !strings.Contains(first, "BAGAS") {
		t.Errorf("penanda asli template tidak tersubstitusi: %q", first)
	}
}

func TestBuildRundownDocx_UserTypedPlaceholderDoesNotFailGenerate(t *testing.T) {
	v := sampleView()
	v.Rundown.SouvenirNote = "300 pcs {{bride_name}} {{tidak_dikenal}}"
	v.Roles[0].Note = "lihat {{catatan}}"
	out, err := buildRundownDocx(v, templates.RundownDocx, nil)
	if err != nil {
		t.Fatalf("teks pengguna berbentuk penanda seharusnya tidak menggagalkan generate: %v", err)
	}
	doc := string(entries(t, out)[documentEntry])
	if !strings.Contains(doc, "tidak_dikenal") {
		t.Error("teks pengguna hilang dari hasil")
	}
}

// Kalau baris cetakan ruangan makeup kehilangan numId-nya, penomoran tiap
// ruangan diam-diam menyambung alih-alih mengulang dari 1. Harus galat.
func TestExpandMakeupRooms_MissingNumIDIsAnError(t *testing.T) {
	xml := templateDocumentXML(t)
	broken := strings.Replace(xml, numIDTag(templateNumberedID),
		`<w:numId w:val="`+templateNumberedID+`" />`, 1)
	if broken == xml {
		t.Fatalf("template tidak memuat %s — prasyarat tes ini hilang", numIDTag(templateNumberedID))
	}
	if _, err := expandMakeupRooms(broken, sampleView().MakeupRooms); err == nil {
		t.Error("pergeseran bentuk numId seharusnya galat, bukan diam-diam no-op")
	}
}

// Penyesuaian ukuran bingkai denah tidak boleh gagal diam-diam: byte PNG-nya
// tetap ditukar, jadi melewatkan langkah ini mencetak denah gepeng.
func TestResizeLayoutImage_MissingMarkerIsAnError(t *testing.T) {
	png := testPNG(t, 800, 400)
	xml := templateDocumentXML(t)
	broken := strings.ReplaceAll(xml, `r:embed="`+layoutImageRelID+`"`, `r:embed="rIdX"`)
	if _, err := resizeLayoutImage(broken, png); err == nil {
		t.Error("slot gambar denah yang bergeser seharusnya galat, bukan no-op")
	}
	if _, err := resizeLayoutImage(xml, png); err != nil {
		t.Errorf("template utuh seharusnya berhasil: %v", err)
	}
}
