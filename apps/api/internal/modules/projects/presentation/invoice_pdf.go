package presentation

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/terbilang"
)

// formatRupiah renders a non-negative amount as "Rp 2.500.000" — dot
// thousands separator, id-ID convention (mirrors apps/web's
// formatCurrency), no shared Go equivalent existed in this codebase yet.
func formatRupiah(amount int64) string {
	s := strconv.FormatInt(amount, 10)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return sign + "Rp " + string(out)
}

// fpdfImageType maps sniffed content bytes to one of fpdf's supported embed
// types (jpg/png/gif) — fpdf doesn't support WEBP, one of the 3 MIME types
// TenantService.UploadLogo's whitelist accepts (PLAN.md
// invoice-kwitansi-client §4.6), so a WEBP (or otherwise undecodable) logo
// simply isn't embedded rather than failing the whole PDF.
func fpdfImageType(data []byte) (tp string, ok bool) {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "jpg", true
	case "image/png":
		return "png", true
	case "image/gif":
		return "gif", true
	default:
		return "", false
	}
}

// newDocument starts an A4 page and draws the shared kop surat (logo if
// embeddable + business name/address/phone/email) plus the document title —
// every builder below starts from its return value already positioned below
// the kop, ready for its own body.
func newDocument(profile platformcontracts.TenantProfile, logo []byte, title string) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 16, 18)
	pdf.AddPage()

	textX := 18.0
	if tp, ok := fpdfImageType(logo); ok {
		info := pdf.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: tp}, bytes.NewReader(logo))
		if pdf.Err() {
			// Sniffed as jpg/png/gif but fpdf still couldn't decode it (e.g.
			// truncated/corrupt bytes) — degrade to no logo rather than
			// failing the whole PDF, same "logo is optional" contract as a
			// tenant with no logo at all.
			pdf.ClearError()
		} else if info != nil {
			pdf.ImageOptions("logo", 18, 14, 20, 0, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
			textX = 42
		}
	}

	pdf.SetXY(textX, 14)
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(190-textX+18, 6, profile.BusinessName, "", 2, "L", false, 0, "")
	pdf.SetX(textX)
	pdf.SetFont("Arial", "", 9)
	if profile.Address != "" {
		pdf.CellFormat(190-textX+18, 5, profile.Address, "", 2, "L", false, 0, "")
		pdf.SetX(textX)
	}
	contact := profile.Phone
	if profile.Email != "" {
		if contact != "" {
			contact += " · "
		}
		contact += profile.Email
	}
	if contact != "" {
		pdf.CellFormat(190-textX+18, 5, contact, "", 2, "L", false, 0, "")
	}

	pdf.SetY(38)
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(18, pdf.GetY(), 192, pdf.GetY())
	pdf.Ln(6)

	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 8, title, "", 2, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Arial", "", 10)
	return pdf
}

// row prints "label: value" as a two-column line — the layout every field in
// both documents below uses.
func row(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(45, 6, label, "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, value, "", 2, "L", false, 0, "")
}

// buildClientInvoicePDF renders an Invoice/Tagihan — PLAN.md
// invoice-kwitansi-client §4.5. Not itemized (a single Type+Description+
// Amount line, §2 Out of Scope's "no line items" decision).
func buildClientInvoicePDF(project domain.Project, inv domain.ClientInvoice, profile platformcontracts.TenantProfile, logo []byte) (*fpdf.Fpdf, error) {
	pdf := newDocument(profile, logo, "INVOICE / TAGIHAN")

	row(pdf, "No. Invoice", inv.InvoiceNumber)
	row(pdf, "Tanggal Terbit", inv.CreatedAt.Format(dateLayout))
	row(pdf, "Jatuh Tempo", inv.DueDate.Format(dateLayout))
	if inv.Status == domain.InvoiceCancelled {
		pdf.SetTextColor(200, 0, 0)
		row(pdf, "Status", "DIBATALKAN")
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.Ln(4)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, "Kepada Yth.", "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, project.BrideName+" & "+project.GroomName, "", 2, "L", false, 0, "")
	pdf.Ln(2)

	row(pdf, "Acara", project.Name)
	row(pdf, "Tanggal Acara", project.EventDate.Format(dateLayout))
	row(pdf, "Paket", project.PackageName)
	pdf.Ln(4)

	pdf.SetFillColor(240, 240, 240)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(45, 8, "Jenis", "1", 0, "L", true, 0, "")
	pdf.CellFormat(90, 8, "Keterangan", "1", 0, "L", true, 0, "")
	pdf.CellFormat(0, 8, "Jumlah", "1", 2, "R", true, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(45, 8, string(inv.Type), "1", 0, "L", false, 0, "")
	pdf.CellFormat(90, 8, inv.Description, "1", 0, "L", false, 0, "")
	pdf.CellFormat(0, 8, formatRupiah(inv.Amount), "1", 2, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(135, 8, "Total", "1", 0, "R", false, 0, "")
	pdf.CellFormat(0, 8, formatRupiah(inv.Amount), "1", 2, "R", false, 0, "")
	pdf.Ln(8)

	if profile.BankName != "" || profile.BankAccountNumber != "" {
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(0, 6, "Pembayaran ditransfer ke:", "", 2, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		row(pdf, "Bank", profile.BankName)
		row(pdf, "No. Rekening", profile.BankAccountNumber)
		row(pdf, "Atas Nama", profile.BankAccountHolderName)
	}

	return pdf, nil
}

// buildClientPaymentReceiptPDF renders a Kwitansi — PLAN.md
// invoice-kwitansi-client §4.5. invoiceNumber is "" when this payment was
// recorded manually, never through a ClientInvoice.
func buildClientPaymentReceiptPDF(project domain.Project, p domain.ClientPayment, invoiceNumber string, profile platformcontracts.TenantProfile, logo []byte) (*fpdf.Fpdf, error) {
	pdf := newDocument(profile, logo, "KWITANSI")

	row(pdf, "No. Kwitansi", p.ReceiptNumber)
	row(pdf, "Tanggal", p.PaymentDate.Format(dateLayout))
	pdf.Ln(4)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, "Telah terima dari", "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, project.BrideName+" & "+project.GroomName, "", 2, "L", false, 0, "")
	pdf.Ln(2)

	untuk := "Pembayaran " + string(p.Type)
	if invoiceNumber != "" {
		untuk += " (Tagihan " + invoiceNumber + ")"
	}
	if p.Notes != "" {
		untuk += " — " + p.Notes
	}
	row(pdf, "Untuk Pembayaran", untuk)
	pdf.Ln(2)

	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 10, formatRupiah(p.Amount), "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "I", 10)
	pdf.CellFormat(0, 6, "Terbilang: "+terbilang.Rupiah(p.Amount), "", 2, "L", false, 0, "")
	pdf.Ln(4)

	row(pdf, "Metode", p.Method)
	row(pdf, "No. Referensi", p.ReferenceNumber)
	pdf.Ln(16)

	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, "Diterima oleh,", "", 2, "L", false, 0, "")
	pdf.Ln(16)
	pdf.CellFormat(0, 6, profile.OwnerName, "", 2, "L", false, 0, "")

	return pdf, nil
}
