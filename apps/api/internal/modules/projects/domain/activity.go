package domain

import "time"

type ActivityType string

const (
	ActivityProjectCreated       ActivityType = "project_created"
	ActivityProjectUpdated       ActivityType = "project_updated"
	ActivityProjectStatusChanged ActivityType = "project_status_changed"
	ActivityVendorAdded          ActivityType = "vendor_added"
	// ActivityVendorOverBudget mencatat komitmen vendor yang SENGAJA
	// melampaui nilai kontrak: siapa yang menyetujui, berapa selisihnya, dan
	// alasannya. Inilah yang membedakan kontrol sungguhan dari peringatan
	// berwarna — enam bulan kemudian masih bisa dijawab siapa yang memutuskan.
	ActivityVendorOverBudget    ActivityType = "vendor_over_budget"
	ActivityVendorStatusChanged ActivityType = "vendor_status_changed"
	ActivityMilestoneUpdated    ActivityType = "milestone_updated"
	ActivityPaymentRecorded     ActivityType = "payment_recorded"
	// ActivityPaymentUpdated/ActivityPaymentDeleted are shared generically
	// across all three payment types (vendor/client/venue), same convention
	// as ActivityPaymentRecorded -- distinguished by entityType/entityLabel/
	// description at the call site, not by separate per-type constants.
	ActivityPaymentUpdated   ActivityType = "payment_updated"
	ActivityPaymentDeleted   ActivityType = "payment_deleted"
	ActivityEvidenceUploaded ActivityType = "evidence_uploaded"
	ActivityIssueCreated     ActivityType = "issue_created"
	ActivityIssueUpdated     ActivityType = "issue_updated"
	// ClientInvoice is a distinct entity, not one of the three payment types
	// ActivityPaymentUpdated/Deleted are shared across (see comment above),
	// so it gets its own constants — see PLAN.md invoice-kwitansi-client.
	ActivityInvoiceCreated      ActivityType = "invoice_created"
	ActivityInvoiceUpdated      ActivityType = "invoice_updated"
	ActivityInvoiceMarkedPaid   ActivityType = "invoice_marked_paid"
	ActivityInvoiceUnmarkedPaid ActivityType = "invoice_unmarked_paid"
	ActivityInvoiceDeleted      ActivityType = "invoice_deleted"

	// PO Paket (PLAN.md po-paket-client). activity_log.type is VARCHAR(50),
	// not an ENUM, so new kinds need no migration.
	ActivityPackageOrderApplied   ActivityType = "package_order_applied"
	ActivityPackageOrderIssued    ActivityType = "package_order_issued"
	ActivityPackageOrderRevised   ActivityType = "package_order_revised"
	ActivityPackageOrderCancelled ActivityType = "package_order_cancelled"
)

// ActivityLogEntry is append-only — see ADR-0007. Every mutating use case in
// this module appends one row; nothing ever updates or deletes an entry.
type ActivityLogEntry struct {
	ID           int64
	ProjectID    *int64
	Type         ActivityType
	ActorStaffID int64
	EntityType   string
	EntityID     string
	EntityLabel  string
	Description  string
	CreatedAt    time.Time
}
