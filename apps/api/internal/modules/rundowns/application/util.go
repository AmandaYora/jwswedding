package application

import (
	"strconv"
	"strings"
	"time"
)

// Nama hari dan bulan disalin lokal, mengikuti konvensi yang sudah ada di
// repo ini: projects/presentation/pdf_theme.go dan quotations/presentation/
// pdf_theme.go masing-masing memegang salinannya sendiri alih-alih berbagi
// satu helper di `shared` (shared hanya untuk utilitas teknis, bukan
// pelokalan tampilan). Sampul rundown butuh nama HARI juga, yang tidak
// dipunyai kedua salinan itu.
var hariIndonesia = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis",
	"Jumat", "Sabtu"}

var bulanIndonesia = [...]string{"", "Januari", "Februari", "Maret", "April",
	"Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November",
	"Desember"}

// formatEventDate menghasilkan "Sabtu, 8 Agustus 2026" — bentuk yang dipakai
// di sampul berkas contoh. Hasilnya cuma nilai awal: WO boleh menyuntingnya,
// karena kolomnya memang label teks bebas.
func formatEventDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return hariIndonesia[int(t.Weekday())] + ", " +
		strconv.Itoa(t.Day()) + " " + bulanIndonesia[int(t.Month())] + " " + strconv.Itoa(t.Year())
}

// coupleTitle menyusun judul lampiran «THE WEDDING OF DINDA & REZA» dari nama
// depan kedua mempelai — di berkas contoh yang dipakai memang nama panggilan,
// bukan nama lengkap bergelar.
func coupleTitle(bride, groom string) string {
	b, g := firstWord(bride), firstWord(groom)
	if b == "" && g == "" {
		return ""
	}
	return "“THE WEDDING OF " + strings.ToUpper(b) + " & " + strings.ToUpper(g) + "”"
}

func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
