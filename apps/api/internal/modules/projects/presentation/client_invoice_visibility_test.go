package presentation

import (
	"testing"

	"jwswedding/internal/modules/projects/domain"
)

// D9. Issuing a PO seeds the whole payment plan as Draft invoices, so without
// this filter a client opening their portal sees four or five instalments that
// look due today. The WO Console must keep seeing them.
func TestVisibleInvoicesFor(t *testing.T) {
	all := []domain.ClientInvoice{
		{InvoiceNumber: "INV/1", Status: domain.InvoiceDraft},
		{InvoiceNumber: "INV/2", Status: domain.InvoiceSent},
		{InvoiceNumber: "INV/3", Status: domain.InvoicePaid},
		{InvoiceNumber: "INV/4", Status: domain.InvoiceDraft},
		{InvoiceNumber: "INV/5", Status: domain.InvoiceCancelled},
	}

	t.Run("client tidak melihat Draft", func(t *testing.T) {
		got := visibleInvoicesFor("client", all)
		if len(got) != 3 {
			t.Fatalf("client melihat %d tagihan, mau 3 (Draft tersaring)", len(got))
		}
		for _, inv := range got {
			if inv.Status == domain.InvoiceDraft {
				t.Errorf("tagihan Draft %s bocor ke client — seluruh rencana termin tampak jatuh tempo", inv.InvoiceNumber)
			}
		}
	})

	// Setiap principal non-client adalah staff; tidak ada peran staff yang
	// boleh kehilangan baris Draft-nya.
	for _, principal := range []string{"staff", ""} {
		t.Run("principal "+principal+" melihat semua", func(t *testing.T) {
			if got := visibleInvoicesFor(principal, all); len(got) != len(all) {
				t.Errorf("principal %q melihat %d tagihan, mau %d", principal, len(got), len(all))
			}
		})
	}

	t.Run("daftar kosong", func(t *testing.T) {
		if got := visibleInvoicesFor("client", nil); len(got) != 0 {
			t.Errorf("daftar kosong menghasilkan %d baris", len(got))
		}
	})
}
