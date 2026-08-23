package domain

import "errors"

// ErrDuplicateInvoiceNumber/ErrDuplicateReceiptNumber signal a unique-key
// collision (MySQL 1062) on invoice_number/receipt_number — the repository
// returns these instead of a raw driver error so the application layer can
// translate/retry without knowing MySQL error codes itself. Same pattern as
// platform/domain.ErrDuplicateCustomDomain.
var (
	ErrDuplicateInvoiceNumber = errors.New("nomor invoice sudah digunakan")
	ErrDuplicateReceiptNumber = errors.New("nomor kwitansi sudah digunakan")
)
