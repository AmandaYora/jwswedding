package presentation

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/modules/rundowns/presentation/templates"
)

// sampleView memuat tiap seksi dengan jumlah baris yang SENGAJA berbeda dari
// berkas contoh, supaya tes ini benar-benar menguji panjang tabel yang
// berubah-ubah, bukan kebetulan sama.
func sampleView() *domain.View {
	return &domain.View{
		Rundown: domain.Rundown{
			ID: 7, TenantID: 1, ProjectID: 42, ProjectName: "Ayu & Bagas",
			GroomName: "BAGAS ARYA PRATAMA, S.Kom.", GroomBirthOrder: "Putra ke-1 dari 2 bersaudara",
			GroomParents: "Bapak Slamet Riyadi & Ibu Sulastri",
			BrideName:    "AYU LESTARI WIJAYA, S.E.", BrideBirthOrder: "Putri ke-2 dari 4 bersaudara",
			BrideParents:   "Bapak Hartono & Ibu Wulandari",
			EventDateLabel: "Minggu, 14 Juni 2026", VenueLabel: "Grand Mercure Kemayoran",
			EventTimeLabel: "15.00 - 22.00 wib",
			CoupleTitle:    "“THE WEDDING OF AYU & BAGAS”",
			WOPICName:      "Amanda", WOPICPhone: "0856-9360-1526",
			SiblingsBride: "Ripta Oktavianti\nReznia Febianty",
			SiblingsGroom: "Chairunissa Fadilla",
			SouvenirNote:   "300 pcs / pakai kupon",
			TableClothNote: "meja VIP 4, reguler 500 porsi",
			PlaylistNotes: "1. Boleh open mic/Ya\n2. Boleh dangdut/Ya",
		},
		// Dua kategori, salah satunya dengan dua vendor -- menguji
		// pengelompokan vendorLines.
		Vendors: []domain.Vendor{
			{SortOrder: 0, CategoryLabel: "VENUE & CATERING", VendorName: "GRAND MERCURE KEMAYORAN"},
			{SortOrder: 1, CategoryLabel: "VENUE & CATERING", VendorName: "DAPUR NUSANTARA BY JWS"},
			{SortOrder: 2, CategoryLabel: "DEKORASI", VendorName: "LOTUS DECOR BY JWS"},
		},
		Roles: []domain.Role{
			{RoleLabel: "Wali Nikah CPW", PersonName: "Bapak Hartono", Note: "Ayah CPW"},
			{RoleLabel: "Saksi CPW", PersonName: "Bapak Rizal", Note: "membawa fotokopi KTP"},
		},
		Committees: []domain.Committee{
			{RoleLabel: "Ketua Panitia", PersonText: "Tante Devi\n085817547153",
				JobDesc: "Mengkoordinir seluruh bidang.\nJembatan tim WO dengan keluarga."},
		},
		MenuItems: []domain.MenuItem{
			{GroupKey: domain.MenuGroupStall, Style: domain.MenuStyleHeading, Content: "Stall"},
			{GroupKey: domain.MenuGroupStall, Style: domain.MenuStyleNumbered, Content: "Iga Bakar 100 porsi"},
			{GroupKey: domain.MenuGroupBuffet, Style: domain.MenuStyleHeading, Content: "Catering buffet 600 porsi"},
			{GroupKey: domain.MenuGroupBuffet, Style: domain.MenuStyleNumbered, Content: "Nasi Putih"},
			{GroupKey: domain.MenuGroupAfterAkad, Style: domain.MenuStyleHeading, Content: "Makanan After Akad"},
			{GroupKey: domain.MenuGroupAfterAkad, Style: domain.MenuStylePlain, Content: "Air mineral"},
		},
		MakeupRooms: []domain.MakeupRoom{
			{RoomLabel: "Ruang Makeup kamar Hotel", Lines: []domain.MakeupLine{
				{Style: domain.MakeupStyleHeading, Content: "List Makeup by Rins Makeup (Retouch):"},
				{Style: domain.MakeupStyleNumbered, Content: "Ayu"},
				{Style: domain.MakeupStyleDash, Content: "Makeup start jam 12.00 WIB"},
				{Style: domain.MakeupStyleNote, Content: "Note:"},
			}},
			{RoomLabel: "Ruang Makeup Keluarga", Lines: []domain.MakeupLine{
				{Style: domain.MakeupStyleHeading, Content: "List Makeup by team JWS (No retouch):"},
				{Style: domain.MakeupStyleNumbered, Content: "Ibu CPW"},
			}},
		},
		Items: []domain.Item{
			{Section: domain.SectionAkad, NoLabel: "", TimeLabel: "04.00 - 14.00", Item: "Checking Dekor", PIC: "WO"},
			{Section: domain.SectionAkad, NoLabel: "1", TimeLabel: "15.00", Item: "Clear area Akad Nikah", PIC: "WO", Note: "MC standby"},
			{Section: domain.SectionAkad, NoLabel: "2", TimeLabel: "15.20", Item: "Soft Opening MC", PIC: "WO", Note: "MC"},
			{Section: domain.SectionResepsi, NoLabel: "1", TimeLabel: "18.30", Item: "MC Grand Opening", PIC: "WO & MC"},
			{Section: domain.SectionResepsi, NoLabel: "2", TimeLabel: "18.45 - 19.00", Item: "Kirab Resepsi", PIC: "Dokumentasi WO", Note: "A Thousand Years"},
		},
		LayoutNotes: []domain.LayoutNote{
			{Kind: domain.LayoutNoteRule, NumberLabel: "1", Content: "Non-aktifkan nada dering HP."},
			{Kind: domain.LayoutNoteRule, NumberLabel: "2", Content: "Anak kecil harap dikondisikan."},
			{Kind: domain.LayoutNoteLegend, NumberLabel: "1", Content: "Calon Pengantin Pria (CPP)"},
			{Kind: domain.LayoutNoteLegend, NumberLabel: "2", Content: "Calon Pengantin Wanita (CPW)"},
		},
		PhotoGroups: []domain.PhotoGroup{
			{GroupName: "Keluarga Inti"}, {GroupName: "Keluarga Besar CPP"},
		},
		VIPGuests: []domain.VIPGuest{
			{FullName: "Bapak Camat", Position: "Camat Kemayoran"},
		},
		Playlist: []domain.PlaylistEntry{
			{Title: "Sambung", Artist: "Pamungkas"},
			{Title: "Best Part", Artist: "Daniel Caesar"},
		},
	}
}

func buildSample(t *testing.T) []byte {
	t.Helper()
	out, err := buildRundownDocx(sampleView(), templates.RundownDocx, nil)
	if err != nil {
		t.Fatalf("buildRundownDocx: %v", err)
	}
	return out
}

func entries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("hasil bukan zip yang sah: %v", err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("buka %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("baca %s: %v", f.Name, err)
		}
		out[f.Name] = b
	}
	return out
}

// Tes fidelity yang sesungguhnya: semua yang bukan document.xml harus identik
// byte per byte dengan template. Kalau tes ini jatuh, ada yang mulai
// menggambar ulang bagian dokumen alih-alih menyalinnya.
func TestBuildRundownDocx_NonDocumentEntriesAreByteIdentical(t *testing.T) {
	got := entries(t, buildSample(t))
	want := entries(t, templates.RundownDocx)

	if len(got) != len(want) {
		t.Fatalf("jumlah entry berubah: %d -> %d", len(want), len(got))
	}
	for name, wantBytes := range want {
		gotBytes, ok := got[name]
		if !ok {
			t.Errorf("entry hilang dari hasil: %s", name)
			continue
		}
		if name == "word/document.xml" {
			continue
		}
		if !bytes.Equal(gotBytes, wantBytes) {
			t.Errorf("entry %s berubah (%d -> %d byte); hanya document.xml yang boleh ditulis ulang",
				name, len(wantBytes), len(gotBytes))
		}
	}
}

// Menjaga agar tidak ada yang diam-diam memakai encoding/xml: serializer
// generik menulis ulang prefix namespace dan Word menolak berkasnya.
func TestBuildRundownDocx_NoNamespacePrefixRewrite(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])
	if !strings.Contains(doc, "<w:document") {
		t.Error("elemen akar <w:document> hilang")
	}
	if !strings.Contains(doc, "w14:paraId") {
		t.Error("atribut w14:paraId hilang -- prefix namespace kemungkinan ditulis ulang")
	}
	for _, bad := range []string{"ns0:", "ns1:", "ns2:"} {
		if strings.Contains(doc, bad) {
			t.Errorf("prefix namespace ditulis ulang menjadi %s", bad)
		}
	}
}

func TestBuildRundownDocx_NoLeftoverPlaceholder(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])
	if m := regexp.MustCompile(`\{\{[^}]*\}\}`).FindString(doc); m != "" {
		t.Errorf("masih ada penanda template: %s", m)
	}
}

func TestBuildRundownDocx_WellFormedXML(t *testing.T) {
	// Hanya validasi bentuk. Parser TIDAK dipakai untuk menulis -- lihat
	// docx_surgery.go.
	doc := entries(t, buildSample(t))["word/document.xml"]
	dec := newDecoder(doc)
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("document.xml tidak well-formed: %v", err)
		}
	}
}

func countTableRows(t *testing.T, doc, marker string) int {
	t.Helper()
	// Hitung <w:tr> di dalam tabel yang memuat marker kolom headernya.
	idx := strings.Index(doc, marker)
	if idx < 0 {
		t.Fatalf("penanda %q tidak ditemukan di hasil", marker)
	}
	start, end, err := enclosingElement(doc, idx, "w:tbl")
	if err != nil {
		t.Fatalf("tabel pemuat %q tidak ketemu: %v", marker, err)
	}
	return strings.Count(doc[start:end], "<w:tr>") + strings.Count(doc[start:end], "<w:tr ")
}

func TestBuildRundownDocx_RowCountsFollowData(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])

	// header + 2 baris data
	if got := countTableRows(t, doc, "NAMA PIC"); got != 3 {
		t.Errorf("LIST NAMA: baris = %d, mau 3 (1 header + 2 data)", got)
	}
	// header + 3 baris akad
	if got := countTableRows(t, doc, "Clear area Akad Nikah"); got != 4 {
		t.Errorf("SUSUNAN AKAD: baris = %d, mau 4 (1 header + 3 data)", got)
	}
}

// Tiga tabel lampiran tetap sepanjang aslinya walau datanya sedikit -- inilah
// wujud keputusan "seksi kosong tetap dicetak sebagai kerangka".
func TestBuildRundownDocx_BlankRowPadding(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])
	for _, tc := range []struct {
		name, marker string
		want         int
	}{
		{"LIST FOTO TAMU", "Nama Keluarga/Group/Instansi", minPhotoGroupRows + 1},
		{"LIST TAMU VIP", "JABATAN", minVIPGuestRows + 1},
		{"PLAYLIST", "Judul Lagu", minPlaylistRows + 1},
	} {
		if got := countTableRows(t, doc, tc.marker); got != tc.want {
			t.Errorf("%s: baris = %d, mau %d (header + padding)", tc.name, got, tc.want)
		}
	}
}

// Satu kategori dengan dua vendor harus tampil sebagai SATU judul kategori.
func TestBuildRundownDocx_VendorCategoryPrintedOncePerGroup(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])
	if n := strings.Count(doc, "VENUE &amp; CATERING"); n != 1 {
		t.Errorf("judul kategori tercetak %d kali, mau 1", n)
	}
	for _, v := range []string{"GRAND MERCURE KEMAYORAN", "DAPUR NUSANTARA BY JWS", "LOTUS DECOR BY JWS"} {
		if !strings.Contains(doc, v) {
			t.Errorf("vendor %q tidak tercetak", v)
		}
	}
}

// Nilai yang diketik WO tidak boleh merusak XML.
func TestBuildRundownDocx_EscapesMarkupInUserText(t *testing.T) {
	v := sampleView()
	v.Roles[0].PersonName = `Bapak <b>"Andi" & Rekan</b>`
	out, err := buildRundownDocx(v, templates.RundownDocx, nil)
	if err != nil {
		t.Fatalf("buildRundownDocx: %v", err)
	}
	doc := string(entries(t, out)["word/document.xml"])
	if strings.Contains(doc, "<b>") {
		t.Error("markup dari input WO bocor mentah ke dalam XML")
	}
	if !strings.Contains(doc, "&lt;b&gt;") {
		t.Error("markup dari input WO tidak ter-escape")
	}
}

// Baris baru menjadi <w:br/> di dalam sel, bukan hilang.
func TestBuildRundownDocx_MultilineBecomesLineBreak(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])
	if !strings.Contains(doc, "Mengkoordinir seluruh bidang.") ||
		!strings.Contains(doc, "Jembatan tim WO dengan keluarga.") {
		t.Fatal("teks multi-baris tidak lengkap tercetak")
	}
	if !strings.Contains(doc, `</w:t><w:br/><w:t xml:space="preserve">`) {
		t.Error("baris baru tidak menjadi <w:br/>")
	}
}

// Denah dengan rasio berbeda harus mengubah tinggi bingkai, bukan tercetak
// gepeng.
func TestBuildRundownDocx_LayoutImageAspectRatio(t *testing.T) {
	png := testPNG(t, 800, 400)
	out, err := buildRundownDocx(sampleView(), templates.RundownDocx, png)
	if err != nil {
		t.Fatalf("buildRundownDocx: %v", err)
	}
	e := entries(t, out)
	if !bytes.Equal(e[layoutImageEntry], png) {
		t.Error("byte PNG denah tidak tergantikan")
	}
	doc := string(e["word/document.xml"])
	relPos := strings.Index(doc, `r:embed="`+layoutImageRelID+`"`)
	if relPos < 0 {
		t.Fatal("relasi gambar denah hilang")
	}
	s, en, err := enclosingElement(doc, relPos, "w:drawing")
	if err != nil {
		t.Fatalf("blok drawing denah tidak ketemu: %v", err)
	}
	m := wpExtentRe.FindStringSubmatch(doc[s:en])
	if m == nil {
		t.Fatal("wp:extent hilang")
	}
	cx, cy := m[1], m[2]
	// 800x400 -> cy harus separuh cx.
	if cx != "6343650" {
		t.Errorf("lebar berubah: %s", cx)
	}
	if cy != "3171825" {
		t.Errorf("tinggi = %s, mau 3171825 (setengah lebar, mengikuti rasio 800x400)", cy)
	}
	if !strings.Contains(doc[s:en], `<a:ext cx="6343650" cy="3171825"`) {
		t.Error("a:ext tidak ikut disesuaikan -- gambar akan tetap gepeng di Word")
	}
}

// Rundown yang sama sekali kosong tetap harus menghasilkan dokumen yang sah:
// itulah wujud "seksi yang belum diisi tetap dicetak sebagai kerangka".
func TestBuildRundownDocx_EmptyRundownStillRenders(t *testing.T) {
	out, err := buildRundownDocx(&domain.View{}, templates.RundownDocx, nil)
	if err != nil {
		t.Fatalf("rundown kosong gagal dirender: %v", err)
	}
	doc := string(entries(t, out)["word/document.xml"])
	if m := regexp.MustCompile(`\{\{[^}]*\}\}`).FindString(doc); m != "" {
		t.Errorf("masih ada penanda template: %s", m)
	}
}

// Menyimpan hasil render ke berkas untuk pemeriksaan mata, hanya kalau
// RUNDOWN_DOCX_OUT diisi. Bukan assertion -- alat bantu, bukan tes.
func TestBuildRundownDocx_WriteSampleWhenRequested(t *testing.T) {
	dir := os.Getenv("RUNDOWN_DOCX_OUT")
	if dir == "" {
		t.Skip("RUNDOWN_DOCX_OUT tidak diisi")
	}
	path := filepath.Join(dir, "rundown-sample.docx")
	if err := os.WriteFile(path, buildSample(t), 0o644); err != nil {
		t.Fatalf("tulis %s: %v", path, err)
	}
	t.Logf("contoh hasil render ditulis ke %s", path)
}

// Tiap ruangan makeup harus punya daftar penomorannya sendiri, kalau tidak
// Word melanjutkan hitungan dan ruangan kedua mulai dari "2.".
func TestBuildRundownDocx_MakeupRoomsGetOwnNumbering(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])
	for i := 0; i < 2; i++ {
		want := `<w:numId w:val="` + strconv.Itoa(numberedPoolBase+i) + `"/>`
		if !strings.Contains(doc, want) {
			t.Errorf("ruangan ke-%d tidak memakai daftar penomorannya sendiri (%s)", i+1, want)
		}
	}
	if strings.Contains(doc, `<w:numId w:val="`+templateNumberedID+`"/>`) {
		t.Error("masih ada baris makeup yang memakai numId template -- penomoran akan menyambung")
	}
}

// Sorotan kuning/hijau di berkas contoh adalah coretan kerja, bukan desain.
func TestBuildRundownDocx_NoDraftHighlights(t *testing.T) {
	doc := string(entries(t, buildSample(t))["word/document.xml"])
	if strings.Contains(doc, "<w:highlight") {
		t.Error("sorotan warna dari berkas contoh masih tercetak")
	}
}
