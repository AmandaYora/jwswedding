package presentation

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestParseVendorImportRow_SelFormatMataUang reproduces the exact failure
// mode reported against real uploaded files (see PLAN.md
// "perbaikan-import-bulk-vendor" S2): a price cell formatted as Rupiah
// accounting currency in Excel, read back through excelize's f.Rows()
// (which returns the cell's *display* text, not its raw numeric value) and
// then through parseVendorImportRow -- the same round-trip Import() does.
func TestParseVendorImportRow_SelFormatMataUang(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)

	accountingStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: strPtr(`_-"Rp"* #,##0_-;_-"Rp"* -#,##0_-;_-"Rp"* "-"_-;_-@_-`)})
	if err != nil {
		t.Fatal(err)
	}

	row := []interface{}{"Lopict Story", "Dekorasi", "Lutfi", 8569090284, "", "lopictstory", "Kota Bogor", "Kota Bogor", 5500000, 6000000, "catatan"}
	for i, v := range row {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	priceStart, _ := excelize.CoordinatesToCellName(9, 2)
	priceEnd, _ := excelize.CoordinatesToCellName(10, 2)
	if err := f.SetCellStyle(sheet, priceStart, priceEnd, accountingStyle); err != nil {
		t.Fatal(err)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	g, err := excelize.OpenReader(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()

	rows, err := g.GetRows(g.GetSheetName(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 {
		t.Fatalf("baris data tidak terbaca, dapat %d baris", len(rows))
	}

	got := parseVendorImportRow(rows[1])
	if len(got.ParseIssues) != 0 {
		t.Fatalf("ParseIssues seharusnya kosong, dapat: %v", got.ParseIssues)
	}
	if got.PriceAkad == nil || *got.PriceAkad != 5500000 {
		t.Errorf("PriceAkad = %v, seharusnya 5500000", got.PriceAkad)
	}
	if got.PriceAkadResepsi == nil || *got.PriceAkadResepsi != 6000000 {
		t.Errorf("PriceAkadResepsi = %v, seharusnya 6000000", got.PriceAkadResepsi)
	}
	if got.Phone != "08569090284" {
		t.Errorf("Phone = %q, seharusnya %q (angka 0 di depan harus pulih)", got.Phone, "08569090284")
	}
}

// TestParseVendorImportRow_SelTidakTerbaca locks that a cell text the parser
// genuinely can't interpret as a number is reported by name in ParseIssues
// instead of silently becoming nil -- the exact defect this fix replaces
// (see PLAN.md "perbaikan-import-bulk-vendor" S2).
func TestParseVendorImportRow_SelTidakTerbaca(t *testing.T) {
	cells := []string{"Lopict Story", "Dekorasi", "Lutfi", "8569090284", "", "", "Kota Bogor", "", "nego", "6000000", ""}
	got := parseVendorImportRow(cells)

	if got.PriceAkad != nil {
		t.Errorf("PriceAkad seharusnya nil untuk sel yang tidak terbaca, dapat %v", *got.PriceAkad)
	}
	if len(got.ParseIssues) != 1 {
		t.Fatalf("ParseIssues seharusnya berisi 1 pesan, dapat %v", got.ParseIssues)
	}
	if !strings.Contains(got.ParseIssues[0], "Harga Akad") {
		t.Errorf("ParseIssues[0] = %q, seharusnya menyebut \"Harga Akad\"", got.ParseIssues[0])
	}
	if !strings.Contains(got.ParseIssues[0], "nego") {
		t.Errorf("ParseIssues[0] = %q, seharusnya menyebut isi sel \"nego\"", got.ParseIssues[0])
	}
}

func strPtr(s string) *string { return &s }
