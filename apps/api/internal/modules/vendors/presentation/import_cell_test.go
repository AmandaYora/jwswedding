package presentation

import "testing"

func TestParseNumberCell_FormatTampilanExcel(t *testing.T) {
	cases := []struct {
		raw     string
		wantNil bool
		want    int64
		wantErr bool
	}{
		{raw: "5,000,000", want: 5000000},
		{raw: " Rp5,500,000 ", want: 5500000},
		{raw: "Rp1,400,000", want: 1400000},
		{raw: "5.000.000", want: 5000000},
		{raw: "5,000,000.00", want: 5000000},
		{raw: "2000000.0", want: 2000000},
		{raw: "", wantNil: true},
		{raw: "nego", wantErr: true},
		{raw: "5jt", wantErr: true},
		{raw: "-", wantErr: true},
	}
	for _, c := range cases {
		got, err := parseNumberCell(c.raw)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseNumberCell(%q) seharusnya menghasilkan error, tapi tidak", c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseNumberCell(%q) tidak seharusnya error, dapat: %v", c.raw, err)
			continue
		}
		if c.wantNil {
			if got != nil {
				t.Errorf("parseNumberCell(%q) = %d, seharusnya nil", c.raw, *got)
			}
			continue
		}
		if got == nil {
			t.Errorf("parseNumberCell(%q) = nil, seharusnya %d", c.raw, c.want)
			continue
		}
		if *got != c.want {
			t.Errorf("parseNumberCell(%q) = %d, seharusnya %d", c.raw, *got, c.want)
		}
	}
}

func TestParseNumberCell_NilaiNegatifDitolak(t *testing.T) {
	if _, err := parseNumberCell("-5000"); err == nil {
		t.Error(`parseNumberCell("-5000") seharusnya menghasilkan error karena kolom harga UNSIGNED di database`)
	}
}

func TestNormalizePhoneCell(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "8569090284", want: "08569090284"},
		{raw: "628123456789", want: "08123456789"},
		{raw: "08123456789", want: "08123456789"},
		{raw: "+62 812-3456", want: "+62 812-3456"},
		{raw: "", want: ""},
	}
	for _, c := range cases {
		got := normalizePhoneCell(c.raw)
		if got != c.want {
			t.Errorf("normalizePhoneCell(%q) = %q, seharusnya %q", c.raw, got, c.want)
		}
	}
}
