package presentation

// Tes TTD pada respons API (T9): signature terisi/null, dan storageKey
// TIDAK PERNAH ikut bocor ke frontend.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"jwswedding/internal/modules/quotations/application"
	"jwswedding/internal/modules/quotations/domain"
)

func signedView() *application.QuotationView {
	signedAt := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	return &application.QuotationView{
		Quotation: &domain.Quotation{
			ID: 3, TenantID: 1, ClientID: 7, PONumber: "PO/202609/0003",
			Status: domain.QuotationAccepted, BasePrice: 100_000_000, PackageName: "Silver",
			Snapshot: &domain.QuotationSnapshot{
				Current: domain.QuotationRevision{
					Revision: 0, BasePrice: 100_000_000, PackageName: "Silver",
					Signature: &domain.QuotationClientSignature{
						SignerName: "Rara", SignerRole: "Bride",
						StorageKey: "jwswedding/signature/quotation/1/3/rev0.png",
						SignedAt:   signedAt, Channel: "magic_link",
					},
				},
			},
		},
		Total: 100_000_000, ClientBride: "Rara", ClientGroom: "Dafa",
	}
}

func TestQuotationResponse_MembawaSignature(t *testing.T) {
	res := toQuotationResponse(signedView())
	if res == nil {
		t.Fatal("respons nil")
	}
	if res.Signature == nil {
		t.Fatal("signature null padahal revisi sudah diteken")
	}
	if res.Signature.SignerName != "Rara" || res.Signature.SignerRole != "Bride" ||
		res.Signature.Channel != "magic_link" || res.Signature.SignedAt == "" {
		t.Errorf("signature = %+v", res.Signature)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// storageKey milik dokumen tidak boleh keluar ke frontend.
	if strings.Contains(string(raw), "storageKey") {
		t.Errorf("respons membocorkan storageKey: %s", raw)
	}
	if strings.Contains(string(raw), "rev0.png") {
		t.Errorf("respons membocorkan kunci penyimpanan: %s", raw)
	}
}

func TestQuotationResponse_TanpaTTD_SignatureNull(t *testing.T) {
	view := signedView()
	view.Quotation.Snapshot.Current.Signature = nil

	res := toQuotationResponse(view)
	if res == nil {
		t.Fatal("respons nil")
	}
	if res.Signature != nil {
		t.Errorf("signature = %+v, mau null — D13a dan dialog Terima membaca null ini", res.Signature)
	}
}
