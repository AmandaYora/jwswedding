package presentation

import (
	"fmt"
	"regexp"
	"strings"
)

// Bedah XML tingkat teks untuk word/document.xml.
//
// Kenapa string, bukan encoding/xml: menyerialkan ulang seluruh dokumen
// membuat prefix namespace ditulis ulang (w: -> ns1:, dst.) sementara atribut
// mc:Ignorable masih merujuk nama lama, dan Word menolak berkasnya dengan
// "The file appears to be corrupted". Ini bukan dugaan — sudah terjadi saat
// pembuktian konsep. Dengan bedah terarah, byte di luar potongan yang memang
// diganti tidak pernah tersentuh.

// scalarToken adalah bentuk penanda skalar {{key}}. Sengaja TIDAK mencakup
// {{#name}}, {{/name}}, dan {{~style}}: ketiganya penanda region/baris yang
// sudah habis dipakai expandRows/expandStyledRegion sebelum pass skalar.
var scalarToken = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}`)

// xmlEscape menyiapkan nilai yang diketik WO untuk disisipkan ke dalam w:t.
//
// Empat hal ditangani, masing-masing karena bisa merusak berkas:
//
//  1. &, <, > di-escape. Satu nama ber-"&" saja sudah cukup membuat XML-nya
//     tidak valid.
//  2. Karakter kontrol yang ILEGAL di XML 1.0 (0x00-0x08, 0x0B, 0x0C,
//     0x0E-0x1F) dibuang. Karakter semacam itu ikut terbawa saat teks ditempel
//     dari Excel atau aplikasi chat, dan satu saja cukup membuat Word menolak
//     berkasnya sebagai korup. Tab, CR, dan LF dipertahankan.
//  3. Baris baru menjadi <w:br/> DI DALAM run yang sama, bukan paragraf baru:
//     hasil visualnya sama di dalam sel tabel, tanpa harus mengkloning
//     paragraf beserta seluruh properti formatnya.
//  4. "{{" yang diketik WO dipecah ke dua w:t bertetangga di dalam run yang
//     sama. Word menyambung keduanya, jadi yang tercetak tetap "{{" persis;
//     yang berubah hanya satu hal: teksnya tidak lagi menyerupai penanda
//     template, sehingga tidak bisa tersangkut di penjaga "penanda belum
//     terisi" dan menggagalkan generate rundown itu untuk seterusnya.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = stripIllegalXMLChars(s)
	s = strings.ReplaceAll(s, "{{", `{</w:t><w:t xml:space="preserve">{`)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", `</w:t><w:br/><w:t xml:space="preserve">`)
	return s
}

func stripIllegalXMLChars(s string) string {
	if strings.IndexFunc(s, illegalInXML) < 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if illegalInXML(r) {
			return -1
		}
		return r
	}, s)
}

func illegalInXML(r rune) bool {
	return r < 0x20 && r != '\t' && r != '\n' && r != '\r'
}

// substitute mengganti {{key}} dalam SATU kali pindai.
//
// Bukan ReplaceAll per key di dalam loop map: urutan iterasi map di Go acak,
// dan nilai yang baru disisipkan ikut terbaca lagi pada putaran berikutnya —
// artinya teks yang kebetulan berisi "{{bride_name}}" kadang ikut
// tersubstitusi dan kadang tidak, tergantung urutan. Dengan satu kali pindai,
// nilai yang sudah disisipkan tidak pernah dipindai ulang dan hasilnya
// deterministik.
//
// Penanda yang TIDAK dikenal dibiarkan apa adanya, supaya penjaga di
// renderDocumentXML masih bisa menangkap placeholder template yang lupa diisi.
func substitute(xml string, values map[string]string) string {
	return scalarToken.ReplaceAllStringFunc(xml, func(token string) string {
		v, ok := values[token[2:len(token)-2]]
		if !ok {
			return token
		}
		return xmlEscape(v)
	})
}

// tagToken adalah satu tag pembuka/penutup hasil pemindaian.
type tagToken struct {
	start, end int
	closing    bool
	selfClose  bool
}

// scanTags memindai seluruh kemunculan satu nama tag, mengabaikan yang hanya
// berupa awalan nama lain (mis. <w:pPr> saat mencari <w:p>).
func scanTags(xml, tag string) []tagToken {
	var out []tagToken
	openTag, closeTag := "<"+tag, "</"+tag
	for i := 0; i < len(xml); {
		nextOpen := strings.Index(xml[i:], openTag)
		nextClose := strings.Index(xml[i:], closeTag)
		if nextOpen < 0 && nextClose < 0 {
			break
		}
		isClose := nextOpen < 0 || (nextClose >= 0 && nextClose < nextOpen)
		var at int
		if isClose {
			at = i + nextClose
		} else {
			at = i + nextOpen
		}
		nameEnd := at + len(openTag)
		if isClose {
			nameEnd = at + len(closeTag)
		}
		if nameEnd >= len(xml) {
			break
		}
		// Karakter tepat setelah nama tag harus mengakhiri nama, kalau tidak
		// ini tag lain yang namanya berawalan sama.
		if c := xml[nameEnd]; c != '>' && c != ' ' && c != '/' && c != '\t' && c != '\n' {
			i = nameEnd
			continue
		}
		gt := strings.IndexByte(xml[at:], '>')
		if gt < 0 {
			break
		}
		end := at + gt + 1
		out = append(out, tagToken{
			start: at, end: end, closing: isClose,
			selfClose: xml[end-2] == '/',
		})
		i = end
	}
	return out
}

// enclosingElement mencari elemen <tag> terdalam yang memuat posisi pos,
// dengan memperhitungkan penyarangan (w:p bisa bersarang di dalam textbox).
func enclosingElement(xml string, pos int, tag string) (start, end int, err error) {
	tokens := scanTags(xml, tag)
	var stack []int
	for _, t := range tokens {
		if t.start > pos {
			break
		}
		switch {
		case t.selfClose:
			// tidak membuka apa pun
		case t.closing:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			stack = append(stack, t.start)
		}
	}
	if len(stack) == 0 {
		return 0, 0, fmt.Errorf("tidak ada <%s> yang memuat posisi %d", tag, pos)
	}
	startIdx := stack[len(stack)-1]

	depth := 0
	for _, t := range tokens {
		if t.start < startIdx || t.selfClose {
			continue
		}
		if t.closing {
			depth--
			if depth == 0 {
				return startIdx, t.end, nil
			}
			continue
		}
		depth++
	}
	return 0, 0, fmt.Errorf("<%s> pembuka di %d tidak punya penutup", tag, startIdx)
}

// elementContaining mengembalikan potongan XML elemen <tag> yang memuat marker.
func elementContaining(xml, marker, tag string) (start, end int, body string, err error) {
	pos := strings.Index(xml, marker)
	if pos < 0 {
		return 0, 0, "", fmt.Errorf("penanda %q tidak ditemukan di template", marker)
	}
	start, end, err = enclosingElement(xml, pos, tag)
	if err != nil {
		return 0, 0, "", err
	}
	return start, end, xml[start:end], nil
}

// expandRows menggandakan satu baris tabel template menjadi satu baris per
// data. Seluruh format (border, font, perataan) ikut karena yang digandakan
// adalah baris aslinya, bukan baris yang dikarang ulang.
func expandRows(xml, name string, rows []map[string]string) (string, error) {
	openTag, closeTag := "{{#"+name+"}}", "{{/"+name+"}}"
	start, end, tmpl, err := elementContaining(xml, openTag, "w:tr")
	if err != nil {
		return "", err
	}
	if !strings.Contains(tmpl, closeTag) {
		return "", fmt.Errorf("penanda %q dan %q tidak berada di baris yang sama", openTag, closeTag)
	}
	clean := strings.NewReplacer(openTag, "", closeTag, "")
	var b strings.Builder
	for _, row := range rows {
		b.WriteString(substitute(clean.Replace(tmpl), row))
	}
	return xml[:start] + b.String() + xml[end:], nil
}

// styledLine adalah satu baris di dalam region paragraf ber-style.
type styledLine struct {
	Style string
	Text  string
}

// expandStyledRegion mengganti seluruh region -- dari paragraf penanda
// {{#name}} sampai {{/name}} -- dengan satu paragraf per baris data, memakai
// paragraf cetakan yang cocok dengan style-nya.
//
// Region ini yang membuat daftar VENDORS, tiga kolom DATA LAINNYA, isi tiap
// ruangan makeup, dan aturan tamu LAYOUT bisa panjangnya bebas tanpa
// kehilangan penomoran Word aslinya.
func expandStyledRegion(xml, name string, lines []styledLine) (string, error) {
	openTag, closeTag := "{{#"+name+"}}", "{{/"+name+"}}"
	startPos, _, _, err := elementContaining(xml, openTag, "w:p")
	if err != nil {
		return "", err
	}
	_, endPos, _, err := elementContaining(xml, closeTag, "w:p")
	if err != nil {
		return "", err
	}
	if endPos <= startPos {
		return "", fmt.Errorf("region %q: penanda penutup mendahului pembuka", name)
	}

	region := xml[startPos:endPos]
	models := map[string]string{}
	for _, t := range scanTags(region, "w:p") {
		if t.closing || t.selfClose {
			continue
		}
		s, e, err := enclosingElement(region, t.start, "w:p")
		if err != nil {
			continue
		}
		para := region[s:e]
		if i := strings.Index(para, "{{~"); i >= 0 {
			if j := strings.Index(para[i:], "}}"); j > 0 {
				models[para[i+3:i+j]] = para
			}
		}
	}
	if len(models) == 0 {
		return "", fmt.Errorf("region %q tidak punya satu pun paragraf cetakan {{~style}}", name)
	}

	var b strings.Builder
	for _, line := range lines {
		model, ok := models[line.Style]
		if !ok {
			return "", fmt.Errorf("region %q: tidak ada cetakan untuk style %q", name, line.Style)
		}
		b.WriteString(strings.Replace(model, "{{~"+line.Style+"}}", xmlEscape(line.Text), 1))
	}
	return xml[:startPos] + b.String() + xml[endPos:], nil
}
