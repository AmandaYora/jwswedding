package application

import (
	"testing"

	"jwswedding/internal/modules/projects/domain"
)

func percentOf(v float64) *float64 { return &v }
func fixedOf(v int64) *int64       { return &v }

// planDPPercentPelunasan is the schedule the source document (Invoice-PO.pdf)
// actually carries: a flat DP, then 30%, then 50%, then the balance.
func planDPPercentPelunasan() []domain.TermPlanEntry {
	return []domain.TermPlanEntry{
		{Sequence: 1, Label: "Down Payment", Type: domain.PaymentDP, FixedAmount: fixedOf(10_000_000), DaysBeforeEvent: 330},
		{Sequence: 2, Label: "Pembayaran 1", Type: domain.PaymentTermin, Percent: percentOf(30), DaysBeforeEvent: 210},
		{Sequence: 3, Label: "Pembayaran 2", Type: domain.PaymentTermin, Percent: percentOf(50), DaysBeforeEvent: 60},
		{Sequence: 4, Label: "Pelunasan", Type: domain.PaymentPelunasan, Percent: percentOf(100), DaysBeforeEvent: 30},
	}
}

func sum(values []int64) int64 {
	var total int64
	for _, v := range values {
		total += v
	}
	return total
}

// The single most important property of the schedule: it must add up to the
// contract total exactly. A PO whose instalments do not sum to its own
// "TOTAL PEMBAYARAN" is indefensible in front of a client, and 30%+50% of an
// odd total is exactly where the rupiah would go missing.
func TestComputeScheduleAmounts_SumsToTotalExactly(t *testing.T) {
	plan := planDPPercentPelunasan()
	totals := []int64{
		225_600_000, // the source document's own total
		1,
		999_999_999,
		100_000_003, // deliberately not divisible by 10 or 3
		10_000_000,  // equal to the fixed DP alone
	}
	for _, total := range totals {
		amounts := ComputeScheduleAmounts(total, plan)
		if got := sum(amounts); got != total {
			t.Errorf("total %d: jumlah termin = %d, mau %d (selisih %d) -- skema cicilan wajib berjumlah tepat sama dengan total",
				total, got, total, got-total)
		}
	}
}

func TestComputeScheduleAmounts_SourceDocumentNumbers(t *testing.T) {
	plan := planDPPercentPelunasan()
	amounts := ComputeScheduleAmounts(225_600_000, plan)

	want := []int64{
		10_000_000,  // DP, nominal tetap
		67_680_000,  // 30% dari 225.600.000
		112_800_000, // 50% dari 225.600.000
		35_120_000,  // sisa
	}
	for i := range want {
		if amounts[i] != want[i] {
			t.Errorf("termin ke-%d = %d, mau %d", i+1, amounts[i], want[i])
		}
	}
}

// A shrinking total must never produce a negative bill. Adjustments are edited
// live during negotiation, so this path has to degrade quietly instead of
// hard-failing the edit.
func TestComputeScheduleAmounts_OverCommittedTotalClampsToZero(t *testing.T) {
	plan := []domain.TermPlanEntry{
		{Sequence: 1, Label: "DP", Type: domain.PaymentDP, FixedAmount: fixedOf(10_000_000), DaysBeforeEvent: 300},
		{Sequence: 2, Label: "Pelunasan", Type: domain.PaymentPelunasan, Percent: percentOf(100), DaysBeforeEvent: 30},
	}
	amounts := ComputeScheduleAmounts(5_000_000, plan)
	if amounts[1] != 0 {
		t.Errorf("sisa = %d, mau 0 -- termin tidak boleh bernilai negatif", amounts[1])
	}
}

func TestComputeScheduleAmounts_EmptyPlan(t *testing.T) {
	if got := ComputeScheduleAmounts(100, nil); len(got) != 0 {
		t.Errorf("plan kosong menghasilkan %d termin, mau 0", len(got))
	}
}

// Takeout lines are stored as negative amounts (D2), so the total is a plain
// SUM. These are the exact ADDITIONAL rows from the source document, which
// footed to Rp12.100.000 there.
func TestTotalAdjustments_SignedSumMatchesSourceDocument(t *testing.T) {
	adjustments := []domain.ProjectPackageAdjustment{
		{Description: "Takeout Busana akad Resepsi 1jt, busana ortu 500k", Amount: -1_500_000},
		{Description: "Cashback Stall Kekinian", Amount: -2_000_000},
		{Description: "Takeout souvenir 1jt, Busana among 250k", Amount: -1_250_000},
		{Description: "Upgrade Mua bride 1jt", Amount: 1_000_000},
		{Description: "Add 1 ekor kambing guling", Amount: 2_650_000},
		{Description: "Add overtime Ruang Rias 1 jam", Amount: 650_000},
		{Description: "Add 100 buffet x 99k", Amount: 9_900_000},
		{Description: "Add 1 ekor kambing guling", Amount: 2_650_000},
	}
	if got := domain.TotalAdjustments(adjustments); got != 12_100_000 {
		t.Errorf("TotalAdjustments = %d, mau 12100000 (nilai ADDITIONAL & TAKEOUT di Invoice-PO.pdf)", got)
	}
}

// The whole B6 arithmetic of the source document, end to end.
func TestPackageTotal_MatchesSourceDocument(t *testing.T) {
	const basePrice int64 = 213_500_000
	adjustments := []domain.ProjectPackageAdjustment{
		{Amount: -1_500_000}, {Amount: -2_000_000}, {Amount: -1_250_000}, {Amount: 1_000_000},
		{Amount: 2_650_000}, {Amount: 650_000}, {Amount: 9_900_000}, {Amount: 2_650_000},
	}
	total := basePrice + domain.TotalAdjustments(adjustments)
	if total != 225_600_000 {
		t.Fatalf("TOTAL PEMBAYARAN = %d, mau 225600000", total)
	}

	// Ledger realisasi dari dokumen sumber.
	var received int64 = 10_000_000 + 50_000_000 + 128_125_000 + 21_625_000 + 2_630_000
	if remaining := total - received; remaining != 13_220_000 {
		t.Errorf("SISA PEMBAYARAN = %d, mau 13220000", remaining)
	}
}

// IsNumbered is what stops a re-issue after Revise from consuming a second PO
// number (D26) — the guard that keeps one contract to one number.
func TestPackageOrder_IsNumbered(t *testing.T) {
	draft := &domain.PackageOrder{}
	if draft.IsNumbered() {
		t.Error("PO Draft dianggap sudah bernomor -- Issue akan melewati penomoran dan menyimpan po_number kosong")
	}
	issued := &domain.PackageOrder{PONumber: "PO/202609/0001"}
	if !issued.IsNumbered() {
		t.Error("PO terbit dianggap belum bernomor -- Issue ulang akan memberi nomor baru untuk kontrak yang sama")
	}
}
