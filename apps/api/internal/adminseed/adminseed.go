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

	// --- The one tenant (D1: single-tenant, tenant_id column kept as-is) ---
	tenant := &platformdomain.Tenant{
		BusinessName: businessName, OwnerName: ownerName, Username: username,
		Email: email, Phone: phone, City: city, JoinedAt: time.Now(),
		// PlanID stays nil — D7's local plan catalog is gone, this only ever
		// gets set once the Owner actually pays through ElProof.
		PlanID:             nil,
		SubscriptionStatus: platformdomain.StatusActive,
		BrandColorPreset:   platformdomain.DefaultBrandColorPreset,
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

	expiresAt := time.Now().AddDate(0, subscriptionMonths, 0)
	if _, err := db.ExecContext(ctx,
		`UPDATE tenants SET subscription_expires_at = ?, custom_domain = ? WHERE id = ?`,
		expiresAt, customDomain, tenant.ID,
	); err != nil {
		return err
	}
	customDomainLog := "(kosong)"
	if customDomain != nil {
		customDomainLog = *customDomain
	}
	log.Printf("seeded tenant: %s (id=%d), langganan aktif sampai %s, custom_domain=%s", tenant.BusinessName, tenant.ID, expiresAt.Format(time.RFC3339), customDomainLog)

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

// truncateAll's table list must cover every data table jwswedding_db owns —
// verify against `docs/DB_SCHEMA.md`/a fresh `SHOW TABLES` whenever a
// migration adds one, not just from memory. This list previously missed
// `client_payments`, `venue_payments`, `venues`, and `project_milestone_templates`
// (a bug inherited from ElProof, never updated when venues/client-payments/
// venue-payments were added) — see `docs/plan/migrasi-data-jws/PLAN.md` T8/D8:
// running this against a freshly migrated production database left those four
// tables full of rows pointing at a tenant/projects that truncateAll had just
// wiped, and `Run` reported success regardless.
func truncateAll(db *sql.DB) {
	stmts := []string{
		"SET FOREIGN_KEY_CHECKS=0",
		"TRUNCATE TABLE refresh_tokens",
		"TRUNCATE TABLE credentials",
		"TRUNCATE TABLE subscription_transactions",
		"TRUNCATE TABLE staff_members",
		"TRUNCATE TABLE tenants",
		"TRUNCATE TABLE vendors",
		"TRUNCATE TABLE vendor_categories",
		"TRUNCATE TABLE activity_log",
		"TRUNCATE TABLE evidence",
		"TRUNCATE TABLE vendor_issues",
		"TRUNCATE TABLE vendor_payments",
		"TRUNCATE TABLE vendor_milestones",
		"TRUNCATE TABLE project_vendors",
		"TRUNCATE TABLE project_milestones",
		"TRUNCATE TABLE projects",
		"TRUNCATE TABLE clients",
		"TRUNCATE TABLE client_payments",
		"TRUNCATE TABLE venue_payments",
		"TRUNCATE TABLE venues",
		"TRUNCATE TABLE project_milestone_templates",
		// platform's own pending-charge index (D8/D11) — not a business
		// ledger, safe to wipe along with everything else.
		"TRUNCATE TABLE pending_subscription_charges",
		"SET FOREIGN_KEY_CHECKS=1",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("truncate (%s): %v", stmt, err)
		}
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
