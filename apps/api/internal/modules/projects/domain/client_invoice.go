package domain

import "time"

type InvoiceStatus string

const (
	InvoiceDraft     InvoiceStatus = "Draft"
	InvoiceSent      InvoiceStatus = "Terkirim"
	InvoicePaid      InvoiceStatus = "Lunas"
	InvoiceCancelled InvoiceStatus = "Dibatalkan"
)

// ClientInvoice is a Tagihan (bill) the WO issues to a client, distinct from
// ClientPayment (the money actually received) — see PLAN.md
// invoice-kwitansi-client §1.2. Marking one "Lunas" auto-creates a linked
// ClientPayment (ClientInvoiceService.MarkPaid) so "Total Diterima"/"Sisa
// Tagihan" keep computing from ClientPayment alone, never from Invoice.
type ClientInvoice struct {
	ID            int64
	ProjectID     int64
	InvoiceNumber string
	// NumberPeriod/NumberSeq back MAX(number_seq)+1 numbering, scoped per
	// (tenant, period) — see NextInvoiceSequence. NumberPeriod is "YYYYMM".
	NumberPeriod string
	NumberSeq    int
	// Type reuses PaymentType — PaymentRefund is rejected at the service
	// layer (an Invoice is a bill, never a refund).
	Type        PaymentType
	Description string
	Amount      int64
	DueDate     time.Time
	Status      InvoiceStatus
	// ClientPaymentID is the sentinel-0 link to the ClientPayment created by
	// MarkPaid — 0 means "belum tertaut", same convention as
	// Project.PICSalesStaffID. Deliberately not a SQL foreign key: UnmarkPaid
	// deletes the linked ClientPayment and resets this to 0 in the same
	// application-level flow, so an FK constraint would only get in the way.
	ClientPaymentID  int64
	CreatedByStaffID int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
