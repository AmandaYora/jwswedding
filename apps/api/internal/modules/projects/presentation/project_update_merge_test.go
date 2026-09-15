package presentation

import (
	"encoding/json"
	"testing"
	"time"

	"jwswedding/internal/modules/projects/domain"
)

// Regression: PATCH /projects/{id} forwarded every absent key's Go zero
// value straight into ProjectService.Update, silently zeroing stored
// figures a partial body never meant to touch (contractValue 265jt -> 0,
// pax 500 -> 0, found live during E2E). updateProject now overlays absent
// keys from the stored row first; these tests pin that contract at the
// exact seam where the data loss happened.

func storedProjectForMergeTest() *domain.Project {
	start := "08:00"
	return &domain.Project{
		ID: 4, TenantID: 1, ClientID: 3, QuotationID: 3,
		Name: "Sinta E2E & Bima E2E", BrideName: "Sinta E2E", GroomName: "Bima E2E",
		EventDate:      time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		EventStartTime: &start, EventEndTime: nil,
		Pax: 500, Venue: "Gedung Serbaguna Nusa Indah",
		PrepStartDate: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
		PackageName:   "Paket Akad + Resepsi Premium",
		ContractValue: 265_000_000,
		Status:        domain.StatusPreparation,
		PICStaffID:    4, PICSalesStaffID: 3,
		Description: "Catatan WO",
	}
}

func decodeMergeTestBody(t *testing.T, raw string) (projectInputBody, map[string]json.RawMessage) {
	t.Helper()
	var body projectInputBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body test tidak valid: %v", err)
	}
	var present map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &present); err != nil {
		t.Fatalf("presence test tidak valid: %v", err)
	}
	return body, present
}

// Body parsial {"status": ...} — satu-satunya kasus yang live merusak data:
// semua field lain wajib dipertahankan dari baris tersimpan.
func TestOverlayMissingProjectFields_BodyParsial_FieldLainTetap(t *testing.T) {
	p := storedProjectForMergeTest()
	body, present := decodeMergeTestBody(t, `{"status":"Completed"}`)
	merged := overlayMissingProjectFields(body, present, p)

	input, err := toProjectInput(merged)
	if err != nil {
		t.Fatalf("toProjectInput() error = %v, want success (tanggal merge dari baris valid)", err)
	}
	if input.Status != domain.StatusCompleted {
		t.Errorf("Status = %q, want Completed (satu-satunya field yang dikirim)", input.Status)
	}
	if input.ContractValue != 265_000_000 {
		t.Errorf("ContractValue = %d, want 265000000 (tidak boleh ikut nol)", input.ContractValue)
	}
	if input.Pax != 500 {
		t.Errorf("Pax = %d, want 500 (tidak boleh ikut nol)", input.Pax)
	}
	if input.Name != p.Name || input.BrideName != p.BrideName || input.GroomName != p.GroomName {
		t.Errorf("identitas berubah: %+v", input)
	}
	if !input.EventDate.Equal(p.EventDate) || !input.PrepStartDate.Equal(p.PrepStartDate) {
		t.Errorf("tanggal berubah: event %v prep %v", input.EventDate, input.PrepStartDate)
	}
	if input.EventStartTime == nil || *input.EventStartTime != "08:00" {
		t.Errorf("EventStartTime = %v, want 08:00 (nil-vs-\"\" harus dipertahankan)", input.EventStartTime)
	}
	if input.EventEndTime != nil {
		t.Errorf("EventEndTime = %v, want nil", input.EventEndTime)
	}
	if input.PackageName != p.PackageName || input.Venue != p.Venue || input.Description != p.Description {
		t.Errorf("field teks berubah: %+v", input)
	}
	if input.PICStaffID != 4 || input.PICSalesStaffID != 3 {
		t.Errorf("PIC berubah: %+v", input)
	}
}

// Body penuh — perilaku lama tidak berubah: semua yang dikirim menang.
func TestOverlayMissingProjectFields_BodyPenuh_SemuaMenang(t *testing.T) {
	p := storedProjectForMergeTest()
	body, present := decodeMergeTestBody(t, `{
		"name":"Nama Baru","brideName":"B","groomName":"G",
		"eventDate":"2026-12-21","eventStartTime":"","eventEndTime":"18:00",
		"pax":600,"venue":"Hall Baru","prepStartDate":"2026-09-14",
		"packageName":"Paket Baru","contractValue":300000000,"status":"Ready",
		"picStaffId":9,"picSalesStaffId":0,"description":"Baru"
	}`)
	merged := overlayMissingProjectFields(body, present, p)

	input, err := toProjectInput(merged)
	if err != nil {
		t.Fatalf("toProjectInput() error = %v", err)
	}
	if input.Name != "Nama Baru" || input.Pax != 600 || input.ContractValue != 300_000_000 {
		t.Errorf("body penuh tidak menang penuh: %+v", input)
	}
	if input.Status != domain.StatusReady || input.PICStaffID != 9 || input.PICSalesStaffID != 0 {
		t.Errorf("status/PIC tidak menang penuh: %+v", input)
	}
	if input.EventStartTime != nil {
		t.Errorf("EventStartTime = %v, want nil (string kosong = Belum ditentukan)", input.EventStartTime)
	}
}

// Reset eksplisit tetap mungkin: key yang dikirim (walau nol) tidak
// dioverlay — hanya key yang absen yang dipertahankan.
func TestOverlayMissingProjectFields_NolEksplisit_TidakDioverlay(t *testing.T) {
	p := storedProjectForMergeTest()
	body, present := decodeMergeTestBody(t, `{"pax":0,"status":"Preparation"}`)
	merged := overlayMissingProjectFields(body, present, p)

	if merged.Pax != 0 {
		t.Errorf("Pax = %d, want 0 (nol eksplisit adalah reset yang sah, bukan absen)", merged.Pax)
	}
}

// Trio venue (presence-aware *int64) tidak tersentuh overlay dalam kondisi
// apapun — jalurnya sudah benar sebelum perbaikan ini.
func TestOverlayMissingProjectFields_TrioVenue_TidakTersentuh(t *testing.T) {
	p := storedProjectForMergeTest()
	venueID := int64(1)
	p.VenueID = &venueID
	body, present := decodeMergeTestBody(t, `{"status":"Ready"}`)
	merged := overlayMissingProjectFields(body, present, p)

	if merged.VenueID != nil {
		t.Errorf("VenueID = %v, want nil (absen = biarkan lampiran apa adanya)", merged.VenueID)
	}
}
