package domain

// Plan mirrors one row of ElProof's subscription-plan catalog (D7,
// PLAN.md §5.2) — jwswedding no longer owns a plan catalog table, this is
// read-only data sourced live from ElProof, scoped to jwswedding's own
// appId. IsActive reflects ElProof's own `active` flag; nothing here is
// ever created/updated/toggled locally. Features is the plan's feature
// list, shown on SubscriptionPage's plan cards; may be empty.
type Plan struct {
	ID             int64
	Name           string
	DurationMonths int
	Price          int64
	IsActive       bool
	Features       []string
}
