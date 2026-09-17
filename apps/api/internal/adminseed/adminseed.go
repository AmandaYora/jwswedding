// Package adminseed resets the database to jwswedding's minimal clean-slate
// state: one tenant ("JWS Wedding" — D1, a single-tenant standalone app,
// unlike ElProof's multi-tenant Platform Console) with one staff Owner
// account. There is no Platform Console seed anymore (D2) and no local plan
// catalog (D7 — plans live at ElProof); the seeded tenant starts with an
// active subscription and an operator-chosen expiry (D15) so the app is
// usable before ElProof's plan-catalog endpoint (PLAN.md §5.2) even exists.
//
// This is administrative tooling, not request-path code — unlike the
// modules under internal/modules, it reaches into modules' application/
// infrastructure packages directly for the handful of things no module's
// contracts expose yet, rather than inventing contract methods no real
// caller needs. It is called from both `cmd/seed` (local dev) and
// `cmd/server`'s `seed` subcommand (the same image the VPS deploys, see
// docs/DEPLOYMENT.md) so the logic lives in exactly one place.
package adminseed

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	identityapp "jwswedding/internal/modules/identity/application"
	identitycontracts "jwswedding/internal/modules/identity/contracts"
	identityinfra "jwswedding/internal/modules/identity/infrastructure"
	platformdomain "jwswedding/internal/modules/platform/domain"
	platforminfra "jwswedding/internal/modules/platform/infrastructure"
	staffapp "jwswedding/internal/modules/staff/application"
	staffinfra "jwswedding/internal/modules/staff/infrastructure"
)

func Run(ctx context.Context, db *sql.DB) error {
	truncateAll(db)

	credentialRepo := identityinfra.NewMySQLCredentialRepository(db)
	hasher := identityinfra.NewBcryptHasher()
	management := identityapp.NewManagementService(credentialRepo, hasher)
	identity := identitycontracts.New(management)

	tenantRepo := platforminfra.NewMySQLTenantRepository(db)
	staffService := staffapp.NewStaffService(staffinfra.NewMySQLStaffRepository(db), identity)

	businessName := getEnv("SEED_TENANT_BUSINESS_NAME", "JWS Wedding")
	ownerName := getEnv("SEED_OWNER_NAME", "Owner JWS Wedding")
	username := getEnv("SEED_OWNER_USERNAME", "owner")
	email := getEnv("SEED_OWNER_EMAIL", "owner@jwswedding.local")
	phone := getEnv("SEED_OWNER_PHONE", "-")
	city := getEnv("SEED_TENANT_CITY", "-")
	password := getEnv("SEED_OWNER_PASSWORD", "changeme123")
	subscriptionMonths := getEnvInt("SEED_SUBSCRIPTION_MONTHS", 12)

	// "bronze" (not platformdomain.DefaultBrandColorPreset's "navy") is JWS
	// Wedding's actual brand color, carried over from when this tenant still
	// ran on the ElProof SaaS platform (PLAN.md
	// redesain-pdf-invoice-kwitansi §D12/§D13) — DefaultBrandColorPreset
	// stays "navy" because it mirrors migration 000017's schema DEFAULT, a
	// fact about the column, not a brand choice, so it's deliberately not
	// reused here.
	brandPreset := getEnv("SEED_TENANT_BRAND_PRESET", "bronze")
	if !platformdomain.IsValidBrandColorPreset(brandPreset) {
		return fmt.Errorf("SEED_TENANT_BRAND_PRESET tidak valid: %q", brandPreset)
	}

	// --- The one tenant (D1: single-tenant, tenant_id column kept as-is) ---
	tenant := &platformdomain.Tenant{
		BusinessName: businessName, OwnerName: ownerName, Username: username,
		Email: email, Phone: phone, City: city, JoinedAt: time.Now(),
		// PlanID stays nil — D7's local plan catalog is gone, this only ever
		// gets set once the Owner actually pays through ElProof.
		PlanID:             nil,
		SubscriptionStatus: platformdomain.StatusActive,
		BrandColorPreset:   brandPreset,
	}
	if err := tenantRepo.Create(ctx, tenant); err != nil {
		return err
	}

	// Create() never writes subscription_expires_at or custom_domain (neither
	// is part of the initial insert column list) — set both directly here.
	// subscription_expires_at so the seeded tenant starts usable per D15,
	// without going through UpdateSubscription's planID argument (there is no
	// real plan ID yet). custom_domain so LoginPage's Host-based branding
	// lookup (ADR-0015) actually resolves this tenant on jwswedding's own
	// production domain (D4/T3) — without this, the login page would 404 on
	// GET /public/branding and always fall back to the neutral look.
	//
	// customDomain distinguishes "unset" (use the default production domain)
	// from "explicitly set to empty" (no custom-domain branding at all, e.g.
	// local dev/test) — getEnv's usual fallback-on-empty semantics can't
	// express that second case, so this reads the env var directly.
	var customDomain *string
	if raw, ok := os.LookupEnv("SEED_TENANT_CUSTOM_DOMAIN"); ok {
		if raw != "" {
			customDomain = &raw
		}
	} else {
		def := "journey.jwswedding.com"
		customDomain = &def
	}

	// planID defaults to JWS's own private ElProof plan (id=2, mapped to
	// app_id='app_02ef90c52704' in subscription_plan_apps — see migration
	// 000045 and docs/DB_SCHEMA.md), not ElProof's public/demo plan (id=1).
	// Without this, a freshly seeded tenant's plan_id stays NULL and
	// SubscriptionPage never shows "Paket Aktif Anda" (no code path used to
	// set it). Empty string opts out (nil), same convention as customDomain
	// above.
	var planID *int64
	if raw, ok := os.LookupEnv("SEED_TENANT_PLAN_ID"); ok && raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("SEED_TENANT_PLAN_ID tidak valid: %w", err)
		}
		planID = &parsed
	} else if !ok {
		def := int64(2)
		planID = &def
	}

	expiresAt := time.Now().AddDate(0, subscriptionMonths, 0)
	if _, err := db.ExecContext(ctx,
		`UPDATE tenants SET subscription_expires_at = ?, custom_domain = ?, plan_id = ? WHERE id = ?`,
		expiresAt, customDomain, planID, tenant.ID,
	); err != nil {
		return err
	}
	customDomainLog := "(kosong)"
	if customDomain != nil {
		customDomainLog = *customDomain
	}
	planIDLog := "(kosong)"
	if planID != nil {
		planIDLog = strconv.FormatInt(*planID, 10)
	}
	log.Printf("seeded tenant: %s (id=%d), langganan aktif sampai %s, custom_domain=%s, plan_id=%s", tenant.BusinessName, tenant.ID, expiresAt.Format(time.RFC3339), customDomainLog, planIDLog)

	// --- The one staff Owner account ---
	owner, err := staffService.CreateOwner(ctx, tenant.ID, ownerName, email, phone, username)
	if err != nil {
		return err
	}
	if err := identity.CreateCredential(ctx, identitycontracts.CreateCredentialInput{
		TenantID: &tenant.ID, PrincipalType: identitycontracts.PrincipalStaff,
		PrincipalID: strconv.FormatInt(owner.ID, 10), Username: username, Email: email,
		Password: password, Role: "Owner", DisplayName: ownerName,
	}); err != nil {
		return err
	}
	log.Printf("seeded staff Owner: %s (id=%d)", username, owner.ID)

	log.Println("done")
	return nil
}

// truncateTables must cover every data table jwswedding_db owns — verify
// against `docs/DB_SCHEMA.md`/a fresh `SHOW TABLES` whenever a migration adds
// one, not just from memory. This list previously missed `client_payments`,
// `venue_payments`, `venues`, and `project_milestone_templates` (a bug
// inherited from ElProof, never updated when venues/client-payments/
// venue-payments were added), and missed `client_invoices` a second time
// when that table was added later. Both times, running this against a
// freshly migrated database left rows pointing at a tenant/projects that
// truncateAll had just wiped, and `Run` reported success regardless — see
// TestAdminSeed_TruncateTablesLengkap below, which locks this against a
// third recurrence by comparing against information_schema.TABLES directly
// instead of another hardcoded list.
//
// Exported as a package-level slice (not a local literal inside truncateAll)
// so adminseed_test.go's completeness check reads the exact same list this
// function truncates — the two can never drift apart again the way the
// previous hardcoded-4-tables test did.
var truncateTables = []string{
	"refresh_tokens",
	"credentials",
	"subscription_transactions",
	"staff_members",
	"tenants",
	"vendors",
	"vendor_categories",
	"activity_log",
	"evidence",
	"vendor_issues",
	"vendor_payments",
	"vendor_milestones",
	"project_vendors",
	"project_milestones",
	"projects",
	"clients",
	"client_payments",
	"client_invoices",
	"venue_payments",
	"venues",
	// Label kategori venue (migration 000069, PLAN revisi-vendor-venue-portal).
	"venue_categories",
	"project_milestone_templates",
	// Penawaran-client-master (migrations 000056-000062): komposisi PO lama
	// dibuang; penawaran + anak-anaknya dan kontak terpisah dari master.
	// Order is cosmetic here — truncateAll disables FK checks — but children
	// are listed before parents to match the rest of this list.
	"quotation_adjustments",
	"quotation_blocks",
	"quotations",
	"client_contacts",
	"package_template_blocks",
	"package_templates",
	// TTD Penawaran (migrations 000066-000067): specimen + link tanda tangan.
	"quotation_signature_links",
	"client_signatures",
	// platform's own pending-charge index (D8/D11) — not a business ledger,
	// safe to wipe along with everything else.
	"pending_subscription_charges",
}

func truncateAll(db *sql.DB) {
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS=0"); err != nil {
		log.Fatalf("disable fk checks: %v", err)
	}
	for _, table := range truncateTables {
		if _, err := db.Exec("TRUNCATE TABLE " + table); err != nil {
			log.Fatalf("truncate %s: %v", table, err)
		}
	}
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
		log.Fatalf("enable fk checks: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
