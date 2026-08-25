package application

import "testing"

func int64Ptr(n int64) *int64 { return &n }

func TestMissingVendorImportFields(t *testing.T) {
	complete := VendorImportRow{
		Name: "Lopict Story", CategoryName: "Dekorasi", PICName: "Lutfi", Phone: "08569090284",
		City: "Kota Bogor", PriceAkad: int64Ptr(5000000), PriceAkadResepsi: int64Ptr(6000000),
	}
	if missing := missingVendorImportFields(complete); len(missing) != 0 {
		t.Errorf("baris lengkap seharusnya tidak ada kolom hilang, dapat %v", missing)
	}

	noPrice := complete
	noPrice.PriceAkad = nil
	if missing := missingVendorImportFields(noPrice); len(missing) != 1 || missing[0] != "Harga Akad" {
		t.Errorf("PriceAkad nil seharusnya menghasilkan [\"Harga Akad\"], dapat %v", missing)
	}

	zeroPrice := complete
	zeroPrice.PriceAkad = int64Ptr(0)
	if missing := missingVendorImportFields(zeroPrice); len(missing) != 1 || missing[0] != "Harga Akad" {
		t.Errorf("PriceAkad 0 seharusnya tetap dilaporkan sebagai kosong, dapat %v", missing)
	}

	cityAndPICEmpty := complete
	cityAndPICEmpty.City = ""
	cityAndPICEmpty.PICName = ""
	missing := missingVendorImportFields(cityAndPICEmpty)
	if len(missing) != 2 {
		t.Fatalf("kota+PIC kosong seharusnya menghasilkan 2 nama kolom, dapat %v", missing)
	}
	want := map[string]bool{"Kota": false, "Nama PIC": false}
	for _, m := range missing {
		if _, ok := want[m]; !ok {
			t.Errorf("kolom hilang %q tidak diharapkan", m)
		}
		want[m] = true
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("kolom hilang %q seharusnya ada di hasil", k)
		}
	}
}

func TestMissingVenueImportFields(t *testing.T) {
	complete := VenueImportRow{
		Name: "Grand Ballroom", PICName: "Sari", PhonePIC: "08123456789", City: "Kota Bandung",
	}
	if missing := missingVenueImportFields(complete); len(missing) != 0 {
		t.Errorf("baris lengkap seharusnya tidak ada kolom hilang, dapat %v", missing)
	}

	// Venue's price/charge/capacity stay optional -- unlike Vendor, a blank
	// RentalPrice must never show up in missingVenueImportFields.
	noPriceNoCapacity := complete
	noPriceNoCapacity.RentalPrice = nil
	noPriceNoCapacity.Charge = nil
	noPriceNoCapacity.Capacity = nil
	if missing := missingVenueImportFields(noPriceNoCapacity); len(missing) != 0 {
		t.Errorf("harga/kapasitas venue kosong seharusnya tidak dilaporkan sebagai wajib, dapat %v", missing)
	}

	missingRequired := complete
	missingRequired.PhonePIC = ""
	if missing := missingVenueImportFields(missingRequired); len(missing) != 1 || missing[0] != "No Tlp PIC" {
		t.Errorf("PhonePIC kosong seharusnya menghasilkan [\"No Tlp PIC\"], dapat %v", missing)
	}
}
