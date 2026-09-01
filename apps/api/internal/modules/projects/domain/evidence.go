package domain

import "time"

type EvidenceType string

const (
	EvidenceQuotation        EvidenceType = "Quotation"
	EvidenceInvoice          EvidenceType = "Invoice"
	EvidenceContract         EvidenceType = "Contract"
	EvidenceTransferProof    EvidenceType = "Transfer Proof"
	EvidenceReceipt          EvidenceType = "Receipt"
	EvidencePurchaseOrder    EvidenceType = "Purchase Order"
	EvidencePhoto            EvidenceType = "Photo"
	EvidenceDocument         EvidenceType = "Document"
	EvidenceScreenshot       EvidenceType = "Screenshot"
	EvidenceMinutesOfMeeting EvidenceType = "Minutes of Meeting"
	EvidenceOther            EvidenceType = "Other"
	// EvidenceBookingProof is bukti booked on a project_vendors engagement --
	// see PLAN.md revisi-timeline-vendor-role-sales.
	EvidenceBookingProof EvidenceType = "Booking Proof"
)

type EvidenceRelatedKind string

const (
	RelatedVendorMilestone EvidenceRelatedKind = "vendorMilestone"
	RelatedPayment         EvidenceRelatedKind = "payment"
	RelatedProjectVendor   EvidenceRelatedKind = "projectVendor"
	RelatedIssue           EvidenceRelatedKind = "issue"
	RelatedClientPayment   EvidenceRelatedKind = "clientPayment"
	RelatedVenuePayment    EvidenceRelatedKind = "venuePayment"
	// RelatedGeneral is a project-level document with no specific
	// vendor/payment/vendor-engagement/issue to attach to (rundown, buku
	// acara, teks juru bicara, banquet order, rekap order, rekap dekor,
	// etc. -- see PLAN.md). RelatedID is meaningless for this kind (always
	// the sentinel 0, mirroring Project.VenueID's own "0 = none" convention
	// -- AUTO_INCREMENT never starts at 0), and IsClientVisible below is
	// only ever read/enforced for this one kind.
	RelatedGeneral EvidenceRelatedKind = "general"
	// RelatedProjectMilestone attaches evidence to any Timeline item
	// (project_milestones) -- general capability, not restricted to any
	// specific milestone name (PLAN.md revisi-timeline-vendor-role-sales).
	RelatedProjectMilestone EvidenceRelatedKind = "projectMilestone"
)

type Evidence struct {
	ID                int64
	ProjectID         int64
	Name              string
	Type              EvidenceType
	StoragePath       string
	FileName          string
	DocumentDate      *time.Time
	UploadedAt        time.Time
	Description       string
	UploadedByStaffID int64
	RelatedKind       EvidenceRelatedKind
	RelatedID         int64
	// IsClientVisible controls whether a `general`-kind document appears in
	// Client Portal's own "Dokumen" tab (GET /projects/{id}/documents).
	// Meaningless for every other RelatedKind -- their evidence stays
	// unconditionally visible to a client the moment it's attached to
	// something inside their own project, same as before this field
	// existed; this is a narrower, opt-in exception, not a broader gate.
	// Defaults false (safe-by-default): a staff member must explicitly opt
	// a document in before a client can see it.
	IsClientVisible bool
}

// allowedUploadMimeTypes is the allowlist an evidence upload's mime type
// must match (PLAN.md mom-25082026-item-belum item 7) -- deliberately wider
// than "just images and PDF" but stops short of "everything": html, svg,
// javascript, and executables are excluded on purpose. Evidence is served
// with Content-Disposition: inline from this application's own origin (see
// presentation.downloadEvidence), so an uploaded .html or .svg would run as
// a script under this app's own origin for whoever opens it -- staff and,
// for client-visible documents, the client too. This is the sole
// authoritative check; the frontend's <input accept=...> is only a dialog
// filter and is not trusted.
var allowedUploadMimeTypes = map[string]bool{
	// Gambar
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	// Dokumen
	"application/pdf": true,
	// Office
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	// Teks
	"text/plain": true,
	"text/csv":   true,
	// Arsip
	"application/zip":              true,
	"application/x-rar-compressed": true,
	"application/x-7z-compressed":  true,
}

// IsAllowedUploadMimeType reports whether mime is on the evidence upload
// allowlist -- see allowedUploadMimeTypes' doc comment for what's excluded
// and why.
func IsAllowedUploadMimeType(mime string) bool {
	return allowedUploadMimeTypes[mime]
}
