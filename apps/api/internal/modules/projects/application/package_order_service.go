package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
)

type PackageOrderRepository interface {
	// FindByProject returns (nil, nil) when the project has no PO — the
	// empty-state path (D23), where every pre-existing project starts.
	FindByProject(ctx context.Context, projectID int64) (*domain.PackageOrder, error)
	Create(ctx context.Context, o *domain.PackageOrder) error
	Update(ctx context.Context, o *domain.PackageOrder) error
	NextPOSequence(ctx context.Context, tenantID int64, period string) (int, error)
	ListBlocks(ctx context.Context, projectID int64) ([]domain.ProjectPackageBlock, error)
	ReplaceBlocks(ctx context.Context, projectID int64, blocks []domain.ProjectPackageBlock) error
	ListAdjustments(ctx context.Context, projectID int64) ([]domain.ProjectPackageAdjustment, error)
	ReplaceAdjustments(ctx context.Context, projectID int64, adjustments []domain.ProjectPackageAdjustment) error
}

// PackageOrderProjectStore is the narrow slice of ProjectRepository this
// service needs. Declared separately rather than taking the whole
// ProjectRepository so it is obvious at a glance that the only project state
// this service ever writes is ContractValue (D15) and PackageName (D28).
type PackageOrderProjectStore interface {
	FindByID(ctx context.Context, tenantID, id int64) (*domain.Project, error)
	Update(ctx context.Context, p *domain.Project) error
}

type PackageOrderService struct {
	repo      PackageOrderRepository
	templates PackageTemplateRepository
	projects  PackageOrderProjectStore
	invoices  *ClientInvoiceService
	activity  *ActivityService
}

func NewPackageOrderService(
	repo PackageOrderRepository,
	templates PackageTemplateRepository,
	projects PackageOrderProjectStore,
	invoices *ClientInvoiceService,
	activity *ActivityService,
) *PackageOrderService {
	return &PackageOrderService{repo: repo, templates: templates, projects: projects, invoices: invoices, activity: activity}
}

// PackageOrderView is everything the "Paket & PO" tab and the PDF need in one
// read. Order is nil when the project has no PO yet — that is not an error,
// it is the empty state (D23).
type PackageOrderView struct {
	Order       *domain.PackageOrder
	Blocks      []domain.ProjectPackageBlock
	Adjustments []domain.ProjectPackageAdjustment
	// Total is BasePrice + TotalAdjustments, i.e. the project's own
	// ContractValue once RecomputeContractValue has run (D15). 0 when there
	// is no PO.
	Total int64
}

func (s *PackageOrderService) Get(ctx context.Context, tenantID, projectID int64) (*PackageOrderView, error) {
	if _, err := s.requireProject(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	order, err := s.repo.FindByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	view := &PackageOrderView{Order: order}
	if order == nil {
		return view, nil
	}
	if view.Blocks, err = s.repo.ListBlocks(ctx, projectID); err != nil {
		return nil, err
	}
	if view.Adjustments, err = s.repo.ListAdjustments(ctx, projectID); err != nil {
		return nil, err
	}
	view.Total = order.BasePrice + domain.TotalAdjustments(view.Adjustments)
	return view, nil
}

// ApplyTemplate copies a template onto a project and creates its Draft PO.
//
// Two rules here are easy to get backwards:
//
//   - BasePrice takes the project's EXISTING ContractValue whenever that is
//     above zero (D23), and only falls back to the template's list price
//     otherwise. This is what lets a project that predates this feature adopt
//     a template without having its agreed value overwritten — and on the
//     create path it means a manually typed, negotiated value wins over the
//     list price, which is the intent.
//   - The payment schedule is COPIED into the PO's own TermsPlan (D22). Issue
//     reads only from there, never back into package_template_terms, so
//     editing a template months later can never alter a signed contract.
func (s *PackageOrderService) ApplyTemplate(ctx context.Context, tenantID, projectID, templateID, actorStaffID int64) (*PackageOrderView, error) {
	project, err := s.requireProject(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireNoOrder(ctx, projectID); err != nil {
		return nil, err
	}
	tmpl, err := s.templates.FindByID(ctx, tenantID, templateID)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, apperror.NotFound("Template paket tidak ditemukan")
	}

	basePrice := tmpl.BasePrice
	if project.ContractValue > 0 {
		basePrice = project.ContractValue
	}

	order := &domain.PackageOrder{
		ProjectID: projectID, BasePrice: basePrice,
		TermsText: tmpl.DefaultTerms, BonusNote: tmpl.DefaultBonusNote,
		TermsPlan: termsPlanFromTemplate(tmpl.Terms),
		Status:    domain.PackageOrderDraft, CreatedByStaffID: actorStaffID,
	}
	if err := s.repo.Create(ctx, order); err != nil {
		return nil, err
	}

	blocks := make([]domain.ProjectPackageBlock, 0, len(tmpl.Blocks))
	for _, b := range tmpl.Blocks {
		blocks = append(blocks, domain.ProjectPackageBlock{
			ProjectID: projectID, Category: b.Category, Body: b.Body,
			QtyText: b.QtyText, BonusNote: b.BonusNote, SortOrder: b.SortOrder,
		})
	}
	if err := s.repo.ReplaceBlocks(ctx, projectID, blocks); err != nil {
		return nil, err
	}

	// D28 — package_name still backs the project header's "Paket / Layanan"
	// field; without this it would go blank once the free-text input is
	// replaced by the template picker.
	project.PackageName = tmpl.Name
	project.ContractValue = basePrice
	if err := s.projects.Update(ctx, project); err != nil {
		return nil, err
	}

	s.activity.Record(ctx, &projectID, domain.ActivityPackageOrderApplied, actorStaffID,
		"package_order", formatID(order.ID), tmpl.Name, "Template paket diterapkan: "+tmpl.Name)

	return s.Get(ctx, tenantID, projectID)
}

// StartBlank creates an empty Draft PO for a project with no template — the
// "Mulai Kosong" path, and the other half of D23: an existing project's
// ContractValue becomes the starting BasePrice rather than being zeroed.
func (s *PackageOrderService) StartBlank(ctx context.Context, tenantID, projectID, actorStaffID int64) (*PackageOrderView, error) {
	project, err := s.requireProject(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireNoOrder(ctx, projectID); err != nil {
		return nil, err
	}
	order := &domain.PackageOrder{
		ProjectID: projectID, BasePrice: project.ContractValue,
		Status: domain.PackageOrderDraft, CreatedByStaffID: actorStaffID,
	}
	if err := s.repo.Create(ctx, order); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityPackageOrderApplied, actorStaffID,
		"package_order", formatID(order.ID), project.Name, "PO Paket dimulai tanpa template")
	return s.Get(ctx, tenantID, projectID)
}

func (s *PackageOrderService) ReplaceBlocks(ctx context.Context, tenantID, projectID int64, blocks []domain.ProjectPackageBlock) (*PackageOrderView, error) {
	if _, err := s.requireEditableOrder(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceBlocks(ctx, projectID, blocks); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, projectID)
}

func (s *PackageOrderService) ReplaceAdjustments(ctx context.Context, tenantID, projectID int64, adjustments []domain.ProjectPackageAdjustment) (*PackageOrderView, error) {
	if _, err := s.requireEditableOrder(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceAdjustments(ctx, projectID, adjustments); err != nil {
		return nil, err
	}
	if err := s.RecomputeContractValue(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, projectID)
}

// SetHeader updates the parts of the PO that are not blocks or adjustments:
// the base price, the terms text and the bonus note.
func (s *PackageOrderService) SetHeader(ctx context.Context, tenantID, projectID int64, basePrice int64, termsText, bonusNote string) (*PackageOrderView, error) {
	order, err := s.requireEditableOrder(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if basePrice < 0 {
		return nil, apperror.Validation("Harga paket tidak boleh negatif", map[string][]string{
			"basePrice": {"Harga paket tidak boleh negatif"},
		})
	}
	order.BasePrice = basePrice
	order.TermsText = termsText
	order.BonusNote = bonusNote
	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}
	if err := s.RecomputeContractValue(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, projectID)
}

// RecomputeContractValue makes projects.contract_value the derived total
// (D15) and rebalances the still-unbilled schedule (D24).
//
// EVERY write path that touches blocks, adjustments or base price must end
// here. Skipping it leaves margin, outstanding and the "Nilai Kontrak" stat
// silently wrong, because all three read contract_value — see PLAN.md §9 R2.
func (s *PackageOrderService) RecomputeContractValue(ctx context.Context, tenantID, projectID int64) error {
	project, err := s.requireProject(ctx, tenantID, projectID)
	if err != nil {
		return err
	}
	order, err := s.repo.FindByProject(ctx, projectID)
	if err != nil {
		return err
	}
	// A project without a PO keeps its manually-entered contract value — this
	// feature is opt-in per project and must not touch the rest.
	if order == nil {
		return nil
	}
	adjustments, err := s.repo.ListAdjustments(ctx, projectID)
	if err != nil {
		return err
	}
	total := order.BasePrice + domain.TotalAdjustments(adjustments)
	if total < 0 {
		return apperror.Validation("Total paket tidak boleh negatif", map[string][]string{
			"adjustments": {"Penyesuaian membuat total pembayaran menjadi negatif"},
		})
	}
	if project.ContractValue != total {
		project.ContractValue = total
		if err := s.projects.Update(ctx, project); err != nil {
			return err
		}
	}
	return s.rebalanceDraftInvoices(ctx, projectID, total)
}

// rebalanceDraftInvoices implements D24. Invoices already sent or paid are
// untouchable, so the difference between the new total and what is already
// committed has to land on the schedule that is still Draft: on its last
// entry, or — when nothing Draft is left — as a fresh Tambahan bill.
//
// Before a PO is issued there are no invoices at all and this is a no-op.
func (s *PackageOrderService) rebalanceDraftInvoices(ctx context.Context, projectID, total int64) error {
	invoices, err := s.invoices.List(ctx, projectID)
	if err != nil {
		return err
	}
	var committed, draftSum int64
	var drafts []domain.ClientInvoice
	for _, inv := range invoices {
		if inv.Status == domain.InvoiceCancelled {
			continue
		}
		if inv.Status == domain.InvoiceDraft {
			drafts = append(drafts, inv)
			draftSum += inv.Amount
			continue
		}
		committed += inv.Amount
	}
	if len(invoices) == 0 {
		return nil
	}

	diff := total - committed - draftSum
	if diff == 0 {
		return nil
	}

	if len(drafts) == 0 {
		// Nothing left to adjust: the increase becomes its own bill. A
		// decrease at this point cannot be undone by billing less, so it is
		// left alone rather than issuing a negative invoice — Refund is a
		// deliberate, separate act by the WO.
		if diff <= 0 {
			return nil
		}
		_, err := s.invoices.Create(ctx, 0, projectID, 0, ClientInvoiceInput{
			Type:        domain.PaymentTambahan,
			Description: "Penyesuaian dari revisi PO Paket",
			Amount:      diff,
			DueDate:     time.Now(),
		})
		return err
	}

	last := drafts[len(drafts)-1]
	amount := last.Amount + diff
	if amount < 0 {
		amount = 0
	}
	_, err = s.invoices.Update(ctx, projectID, last.ID, 0, ClientInvoiceInput{
		Type: last.Type, Description: last.Description, Amount: amount, DueDate: last.DueDate,
	}, last.Status)
	return err
}

// Issue freezes the agreement and seeds its schedule.
//
// The numbering guard is the subtle part: NextPOSequence runs only when the
// PO has never been numbered (D26). Re-issuing after a Revise keeps the
// original number and only bumps the revision, so a client is never handed
// two differently-numbered documents for one contract — see PLAN.md §9 R6.
func (s *PackageOrderService) Issue(ctx context.Context, tenantID, projectID, actorStaffID int64, clientName, phone string) (*PackageOrderView, error) {
	project, err := s.requireProject(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	order, err := s.requireEditableOrder(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	blocks, err := s.repo.ListBlocks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	adjustments, err := s.repo.ListAdjustments(ctx, projectID)
	if err != nil {
		return nil, err
	}
	total := order.BasePrice + domain.TotalAdjustments(adjustments)

	if !order.IsNumbered() {
		period := time.Now().Format("200601")
		seq, err := s.repo.NextPOSequence(ctx, tenantID, period)
		if err != nil {
			return nil, err
		}
		order.PONumber = fmt.Sprintf("PO/%s/%04d", period, seq)
		order.NumberPeriod = period
		order.NumberSeq = seq
	}

	now := time.Now()
	revision := domain.PackageOrderRevision{
		Revision: order.Revision, IssuedAt: now, BasePrice: order.BasePrice,
		TermsText: order.TermsText, BonusNote: order.BonusNote,
		Blocks: blocks, Adjustments: adjustments, TermsPlan: order.TermsPlan,
		Event: domain.PackageOrderEventSnapshot{
			ClientName: clientName, Phone: phone,
			EventDate:  project.EventDate.Format("2006-01-02"),
			EventStart: derefString(project.EventStartTime),
			EventEnd:   derefString(project.EventEndTime),
			Venue:      project.Venue, Pax: project.Pax,
		},
	}
	if order.Snapshot == nil {
		order.Snapshot = &domain.PackageOrderSnapshot{}
	}
	order.Snapshot.Current = revision
	order.Status = domain.PackageOrderIssued
	order.IssuedAt = &now
	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}

	if err := s.seedScheduleInvoices(ctx, tenantID, projectID, actorStaffID, project.EventDate, total, order.TermsPlan); err != nil {
		return nil, err
	}

	s.activity.Record(ctx, &projectID, domain.ActivityPackageOrderIssued, actorStaffID,
		"package_order", formatID(order.ID), order.PONumber, "PO Paket diterbitkan: "+order.PONumber)

	return s.Get(ctx, tenantID, projectID)
}

// Revise reopens an issued PO for editing, archiving what was signed.
func (s *PackageOrderService) Revise(ctx context.Context, tenantID, projectID, actorStaffID int64) (*PackageOrderView, error) {
	if _, err := s.requireProject(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	order, err := s.requireOrder(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.PackageOrderIssued {
		return nil, apperror.Validation("Hanya PO yang sudah terbit dapat direvisi", nil)
	}
	// Push the signed state into history before reopening (D30) — the
	// cancellation and downgrade clauses in the terms refer to "kesepakatan
	// awal", so the superseded revision has to stay readable.
	if order.Snapshot != nil {
		order.Snapshot.History = append(order.Snapshot.History, order.Snapshot.Current)
	}
	order.Revision++
	order.Status = domain.PackageOrderDraft
	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityPackageOrderRevised, actorStaffID,
		"package_order", formatID(order.ID), order.PONumber,
		fmt.Sprintf("PO Paket direvisi menjadi Revisi %d", order.Revision))
	return s.Get(ctx, tenantID, projectID)
}

// Cancel marks an issued PO as cancelled (D29). One-way: the only route back
// to Draft is Revise. The number is not recycled and the row stays as a trace.
func (s *PackageOrderService) Cancel(ctx context.Context, tenantID, projectID, actorStaffID int64) (*PackageOrderView, error) {
	if _, err := s.requireProject(ctx, tenantID, projectID); err != nil {
		return nil, err
	}
	order, err := s.requireOrder(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.PackageOrderIssued {
		return nil, apperror.Validation("Hanya PO yang sudah terbit dapat dibatalkan", nil)
	}
	order.Status = domain.PackageOrderCancelled
	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityPackageOrderCancelled, actorStaffID,
		"package_order", formatID(order.ID), order.PONumber, "PO Paket dibatalkan: "+order.PONumber)
	return s.Get(ctx, tenantID, projectID)
}

// seedScheduleInvoices turns the frozen TermsPlan into Draft Tagihan rows.
// Only runs for a PO being issued for the first time — a re-issue after
// Revise leaves the existing schedule alone, since some of it may already be
// sent or paid and rebalanceDraftInvoices owns that case instead.
func (s *PackageOrderService) seedScheduleInvoices(ctx context.Context, tenantID, projectID, actorStaffID int64, eventDate time.Time, total int64, plan []domain.TermPlanEntry) error {
	if len(plan) == 0 {
		return nil
	}
	existing, err := s.invoices.List(ctx, projectID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	for i, amount := range ComputeScheduleAmounts(total, plan) {
		entry := plan[i]
		if amount <= 0 {
			continue
		}
		dueDate := eventDate.AddDate(0, 0, -entry.DaysBeforeEvent)
		if _, err := s.invoices.Create(ctx, tenantID, projectID, actorStaffID, ClientInvoiceInput{
			Type: entry.Type, Description: entry.Label, Amount: amount, DueDate: dueDate,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ComputeScheduleAmounts splits a total across a payment schedule.
//
// The last entry always takes the REMAINDER rather than its own percentage.
// That is what guarantees the schedule sums to exactly the total: 30% + 50%
// of an odd number leaves rupiah unaccounted for, and a contract whose
// instalments do not add up to its own total is indefensible in front of a
// client.
//
// Every entry is also capped at the balance still unallocated. Clamping only
// the final remainder is not enough and was a real bug caught by
// TestComputeScheduleAmounts_SumsToTotalExactly: with a flat DP larger than a
// shrunken total, the earlier entries alone already overshoot, the last one
// clamps to zero, and the schedule quietly sums to MORE than the contract.
// Capping per entry keeps the invariant intact for any total, including ones
// smaller than the DP.
//
// Percentages are applied in basis points, never float arithmetic, so the
// result is exact and reproducible.
func ComputeScheduleAmounts(total int64, plan []domain.TermPlanEntry) []int64 {
	amounts := make([]int64, len(plan))
	var running int64
	for i, entry := range plan {
		remaining := total - running
		if remaining < 0 {
			remaining = 0
		}
		if i == len(plan)-1 {
			amounts[i] = remaining
			break
		}
		switch {
		case entry.FixedAmount != nil:
			amounts[i] = *entry.FixedAmount
		case entry.Percent != nil:
			bps := int64(math.Round(*entry.Percent * 100))
			amounts[i] = total * bps / 10000
		}
		if amounts[i] > remaining {
			amounts[i] = remaining
		}
		if amounts[i] < 0 {
			amounts[i] = 0
		}
		running += amounts[i]
	}
	return amounts
}

// CloneComposition copies a source project's blocks and adjustments onto a
// freshly duplicated project, and gives it its own Draft PO (D19).
//
// The PO document itself is deliberately NOT copied: po_number, revision,
// issued_at and the signed snapshot all belong to the original agreement, and
// a duplicate is a structural template, not a second copy of a signed
// contract. What carries over is exactly what a WO reuses — the composition,
// the adjustments, the terms text and the schedule preset.
//
// Best-effort by design: a source project with no PO leaves the duplicate
// with none either, which is simply the empty state (D23).
func (s *PackageOrderService) CloneComposition(ctx context.Context, tenantID, sourceProjectID, newProjectID, actorStaffID int64) error {
	source, err := s.repo.FindByProject(ctx, sourceProjectID)
	if err != nil || source == nil {
		return err
	}
	clone := &domain.PackageOrder{
		ProjectID: newProjectID, BasePrice: source.BasePrice,
		TermsText: source.TermsText, BonusNote: source.BonusNote,
		TermsPlan: source.TermsPlan,
		Status:    domain.PackageOrderDraft, CreatedByStaffID: actorStaffID,
	}
	if err := s.repo.Create(ctx, clone); err != nil {
		return err
	}

	blocks, err := s.repo.ListBlocks(ctx, sourceProjectID)
	if err != nil {
		return err
	}
	for i := range blocks {
		blocks[i].ProjectID = newProjectID
	}
	if err := s.repo.ReplaceBlocks(ctx, newProjectID, blocks); err != nil {
		return err
	}

	adjustments, err := s.repo.ListAdjustments(ctx, sourceProjectID)
	if err != nil {
		return err
	}
	for i := range adjustments {
		adjustments[i].ProjectID = newProjectID
	}
	if err := s.repo.ReplaceAdjustments(ctx, newProjectID, adjustments); err != nil {
		return err
	}
	return s.RecomputeContractValue(ctx, tenantID, newProjectID)
}

func termsPlanFromTemplate(terms []domain.PackageTemplateTerm) []domain.TermPlanEntry {
	plan := make([]domain.TermPlanEntry, 0, len(terms))
	for _, t := range terms {
		plan = append(plan, domain.TermPlanEntry{
			Sequence: t.Sequence, Label: t.Label, Type: t.Type,
			Percent: t.Percent, FixedAmount: t.FixedAmount, DaysBeforeEvent: t.DaysBeforeEvent,
		})
	}
	return plan
}

func (s *PackageOrderService) requireProject(ctx context.Context, tenantID, projectID int64) (*domain.Project, error) {
	p, err := s.projects.FindByID(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperror.NotFound("Project tidak ditemukan")
	}
	return p, nil
}

func (s *PackageOrderService) requireOrder(ctx context.Context, projectID int64) (*domain.PackageOrder, error) {
	order, err := s.repo.FindByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, apperror.NotFound("Project ini belum punya PO Paket")
	}
	return order, nil
}

// requireEditableOrder rejects writes to a PO that is no longer open. An
// issued PO is a signed document; reopening it is Revise's job, not a side
// effect of editing a block.
func (s *PackageOrderService) requireEditableOrder(ctx context.Context, tenantID, projectID int64) (*domain.PackageOrder, error) {
	order, err := s.requireOrder(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.PackageOrderDraft {
		return nil, apperror.Validation("PO Paket sudah terbit — buat revisi lebih dulu untuk mengubahnya", nil)
	}
	return order, nil
}

func (s *PackageOrderService) requireNoOrder(ctx context.Context, projectID int64) error {
	order, err := s.repo.FindByProject(ctx, projectID)
	if err != nil {
		return err
	}
	if order != nil {
		return apperror.Validation("Project ini sudah punya PO Paket", nil)
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
