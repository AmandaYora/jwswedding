package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jwswedding/internal/adminseed"
	"jwswedding/internal/loginslides"
	"jwswedding/internal/migrator"
	"jwswedding/internal/modules/billing"
	"jwswedding/internal/modules/clients"
	"jwswedding/internal/modules/identity"
	"jwswedding/internal/modules/platform"
	"jwswedding/internal/modules/projects"
	"jwswedding/internal/modules/staff"
	staffcontracts "jwswedding/internal/modules/staff/contracts"
	"jwswedding/internal/modules/vendors"
	"jwswedding/internal/shared/config"
	"jwswedding/internal/shared/database"
	"jwswedding/internal/shared/elproofpay"
	"jwswedding/internal/shared/middleware"
	"jwswedding/internal/shared/response"
	"jwswedding/internal/shared/storage"
)

// staffNameResolver adapts staffcontracts.Contracts to
// projects/application.StaffNameResolver's narrow, primitive-typed
// interface. Kept here (not in either module) to avoid an import cycle:
// staff/application already imports projects/contracts (for its own
// hard-delete impact-lookup bridge), so projects/application importing
// staff/contracts back would close the loop
// (projects/application -> staff/contracts -> staff/application ->
// projects/contracts -> projects/application) — see
// application.StaffNameResolver's own doc comment.
type staffNameResolver struct {
	contracts staffcontracts.Contracts
}

func (r staffNameResolver) GetName(ctx context.Context, tenantID, staffID int64) (string, bool, error) {
	summary, err := r.contracts.GetSummary(ctx, tenantID, staffID)
	if err != nil {
		return "", false, err
	}
	if summary == nil {
		return "", false, nil
	}
	return summary.Name, true, nil
}

// main dispatches on an optional subcommand so the one compiled binary this
// project ships as its deploy image (see infra/docker/Dockerfile) is
// everything a production host needs — no separate migrate CLI, seed
// binary, or curl/wget in the container for Docker's HEALTHCHECK. With no
// argument it serves, exactly as before this dispatch existed.
//
//	./api                       serve the API (default, unchanged behavior)
//	./api migrate up|down        apply/roll back one embedded SQL migration step
//	./api migrate force <version> clear a "dirty" migration state without running SQL
//	./api seed                   reset to the minimal clean-slate dataset
//	./api healthcheck             exit 0/1 for Docker's HEALTHCHECK (self GET /api/v1/health)
//	./api upload-login-slides <dir>  compress+upload the login page's fixed
//	                                  marketing photos to object storage
func main() {
	cfg := config.Load()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			runMigrate(cfg, os.Args[2:])
			return
		case "seed":
			runSeed(cfg)
			return
		case "healthcheck":
			runHealthcheck(cfg)
			return
		case "upload-login-slides":
			runUploadLoginSlides(cfg, os.Args[2:])
			return
		}
	}

	serve(cfg)
}

func runMigrate(cfg config.Config, args []string) {
	if len(args) == 2 && args[0] == "force" {
		version, convErr := strconv.Atoi(args[1])
		if convErr != nil {
			log.Fatalf("usage: api migrate force <version>")
		}
		if err := migrator.Force(cfg.DatabaseURL, version); err != nil {
			log.Fatalf("migrate force %d: %v", version, err)
		}
		log.Printf("migrate force %d: done", version)
		return
	}

	if len(args) != 1 || (args[0] != "up" && args[0] != "down") {
		log.Fatal("usage: api migrate up|down|force <version>")
	}

	var err error
	if args[0] == "up" {
		err = migrator.Up(cfg.DatabaseURL)
	} else {
		err = migrator.Down(cfg.DatabaseURL)
	}
	if err != nil {
		log.Fatalf("migrate %s: %v", args[0], err)
	}
	log.Printf("migrate %s: done", args[0])
}

func runSeed(cfg config.Config) {
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := adminseed.Run(context.Background(), db); err != nil {
		log.Fatal(err)
	}
}

// runUploadLoginSlides is a one-off, re-runnable operator command (not part
// of serve()'s request path) — see internal/loginslides for what it does.
func runUploadLoginSlides(cfg config.Config, args []string) {
	if len(args) != 1 {
		log.Fatal("usage: api upload-login-slides <source-dir>")
	}

	storageClient, err := storage.New(storage.Config{
		Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: cfg.S3UseSSL,
	})
	if err != nil {
		log.Fatalf("failed to create object storage client: %v", err)
	}

	if err := loginslides.Run(context.Background(), storageClient, args[0]); err != nil {
		log.Fatalf("upload-login-slides: %v", err)
	}
	log.Println("upload-login-slides: done")
}

func runHealthcheck(cfg config.Config) {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/api/v1/health", cfg.AppPort))
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	resp.Body.Close()
	os.Exit(0)
}

func serve(cfg config.Config) {
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		response.OK(w, "ok", nil)
	})

	identityModule := identity.NewModule(db, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	identityModule.RegisterRoutes(mux)

	storageClient, err := storage.New(storage.Config{
		Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: cfg.S3UseSSL,
	})
	if err != nil {
		log.Fatalf("failed to create object storage client: %v", err)
	}

	// One shared elproofpay.Client instance (D12) — jwswedding is a
	// `kind=external` App consuming ElProof's API to pay for its own
	// subscription. Both `billing` (plan catalog) and `platform` (charges)
	// need it; the token cache MUST be shared between them, since ElProof
	// rate-limits POST /auth/app/token to 10 attempts/minute/IP.
	elproofClient := elproofpay.NewClient(cfg.ElProofPaymentBaseURL, cfg.ElProofAppID, cfg.ElProofAppSecret)

	staffModule := staff.NewModule(db, identityModule.Contracts())
	billingModule := billing.NewModule(db, elproofClient)
	platformModule := platform.NewModule(db, billingModule.Contracts(), elproofClient, cfg.ElProofChargeMaxAge, storageClient)
	// Pre-auth, Host-header-resolved tenant branding (ADR-0015), plus
	// ElProof's webhook relay — registered unwrapped, same style as
	// identityModule.RegisterRoutes(mux) above.
	platformModule.RegisterPublicRoutes(mux)

	// gate is the read-only subscription guard (D5/D10/D13) — nested INSIDE
	// RequireAuth (not the reverse) so Claims already exist in the request
	// context by the time gate reads PrincipalType/TenantID. The whitelist is
	// mandatory: without it, a tenant whose subscription just expired could
	// never pay again, locking jwswedding permanently.
	gate := middleware.RequireActiveSubscription(platformModule.Contracts().WritesAllowed, []string{
		"/api/v1/subscriptions/pay",
		"/api/v1/subscriptions/pending-charge/cancel",
	})
	authed := func(h http.Handler) http.Handler {
		return middleware.RequireAuth(cfg.JWTSecret)(gate(h))
	}

	mux.Handle("/api/v1/auth/me", authed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := middleware.FromContext(r.Context())
		response.OK(w, "ok", claims)
	})))

	// projects is built before vendors — vendors' "Lihat Project" resolves a
	// vendor's cross-project engagement history through projects.Contracts()
	// (project_vendors is owned by projects, not vendors).
	projectsModule := projects.NewModule(db, storageClient, staffNameResolver{contracts: staffModule.Contracts()}, platformModule.Contracts())
	// Two-phase wiring: staff needs projects' contract (hard-delete "impact"
	// lookup, PLAN.md) but staffModule is built above, before projectsModule
	// exists.
	staffModule.SetProjectReferenceLookup(projectsModule.Contracts())
	vendorsModule := vendors.NewModule(db, projectsModule.Contracts(), storageClient)
	// Same bridge, the new reciprocal direction (ADR-0016): projects needs to
	// resolve a project's attached venue_id into venue details for its own
	// Project Detail tab and Client Portal's Venue tab, but projectsModule is
	// built above, before vendorsModule exists.
	projectsModule.SetVenueResolver(vendorsModule.Contracts())
	clientsModule := clients.NewModule(db, projectsModule.Contracts(), identityModule.Contracts())
	// Two-phase wiring: projects needs clients' contract (Fase 6 client-portal
	// scoping) but clients.NewModule already needs projects.Contracts() to
	// exist first, so this can't be a constructor argument either direction.
	projectsModule.SetClientAccessResolver(clientsModule.Contracts())
	// Same bridge, second interface (ADR-0013's hard-delete client cleanup) —
	// clientsModule.Contracts() satisfies both ClientAccessResolver and
	// ClientCleaner, so this is the same object as the call above.
	projectsModule.SetClientCleaner(clientsModule.Contracts())

	// Runs for the lifetime of the process (T1's safety net for a missed
	// webhook) — no separate shutdown signal exists anywhere else in this
	// server either (see server.ListenAndServe() below), so this goroutine
	// simply ends when the process does.
	platformModule.StartReconciler(context.Background(), cfg.ElProofReconcileInterval)

	billingModule.RegisterRoutes(mux, authed)
	platformModule.RegisterRoutes(mux, authed)
	staffModule.RegisterRoutes(mux, authed)
	vendorsModule.RegisterRoutes(mux, authed)
	projectsModule.RegisterRoutes(mux, authed)
	clientsModule.RegisterRoutes(mux, authed)

	// Serves the built frontend (see infra/docker/Dockerfile, which copies
	// apps/web's build output here) — a no-op locally, where the frontend
	// runs on its own Vite dev server (`npm run dev:web`) instead.
	mux.Handle("/", spaFileServer("./public", platformModule.SiteMetaForHost))

	handler := middleware.CORS(cfg.AppEnv == "development")(mux)

	log.Printf("%s api listening on :%s (%s)", cfg.AppName, cfg.AppPort, cfg.AppEnv)
	server := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: handler,
		// WriteTimeout is looser than ReadTimeout so it doesn't clip legitimate
		// large evidence-upload responses.
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

// spaFileServer serves static assets from root, falling back to
// root/index.html for any path that doesn't match a real file — required so
// React Router's client-side routes resolve on a hard refresh/direct link.
// Registered on "/", it would otherwise also swallow any unmatched
// /api/v1/... request (e.g. demo-login when disabled) into a fake 200 HTML
// response instead of a real 404, so those are rejected before falling back.
//
// siteMeta resolves the incoming request's Host to a tenant's own branding
// (platform.Module.SiteMetaForHost, ADR-0015's Host lookup) — every
// index.html fallback response runs through it so a shared link's preview
// card (WhatsApp/Telegram/etc.) shows the tenant's own actual name/logo,
// not the static default baked into index.html. Those crawlers never
// execute the SPA's JS, so tabIdentity.ts's client-side title swap never
// reaches them; this is the server-side equivalent for the one document a
// crawler actually reads.
func spaFileServer(root string, siteMeta func(ctx context.Context, host string) (platform.SiteMeta, bool)) http.Handler {
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			response.Error(w, http.StatusNotFound, "Endpoint tidak ditemukan", nil)
			return
		}
		fullPath := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(fullPath); err != nil || info.IsDir() {
			serveIndexWithSiteMeta(w, r, root, siteMeta)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// Exact default <title>/Open Graph tag strings in apps/web/index.html —
// injectTenantSiteMeta replaces these verbatim, so the two must stay in
// sync; each carries a matching comment in index.html itself.
const (
	defaultTitleTag      = `<title>JWS Wedding — Client Transparency Portal</title>`
	defaultOGSiteNameTag = `<meta property="og:site_name" content="JWS Wedding" />`
	defaultOGTitleTag    = `<meta property="og:title" content="JWS Wedding — Client Transparency Portal" />`
	defaultOGDescription = `<meta property="og:description" content="Transparansi persiapan pernikahan Anda, dari WO hingga hari H." />`
	headCloseTag         = `</head>`
)

// serveIndexWithSiteMeta serves index.html, rewritten for the request's Host
// when it resolves to a tenant's own custom domain; unmatched (the
// platform's own domain, localhost, any unconfigured domain) falls straight
// back to the static file, byte-for-byte what http.ServeFile always served.
func serveIndexWithSiteMeta(w http.ResponseWriter, r *http.Request, root string, siteMeta func(ctx context.Context, host string) (platform.SiteMeta, bool)) {
	indexPath := filepath.Join(root, "index.html")
	meta, ok := siteMeta(r.Context(), r.Host)
	if !ok {
		http.ServeFile(w, r, indexPath)
		return
	}
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		http.ServeFile(w, r, indexPath)
		return
	}
	body := injectTenantSiteMeta(raw, meta, r.Host)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
}

// injectTenantSiteMeta swaps index.html's platform-default <title>/Open
// Graph tags for a matched tenant's own name — html.EscapeString guards
// against a business name containing HTML-significant characters, since
// this is spliced into a raw document served to unauthenticated crawlers,
// not passed through React's own escaping.
func injectTenantSiteMeta(raw []byte, meta platform.SiteMeta, host string) []byte {
	name := html.EscapeString(meta.BusinessName)
	page := string(raw)
	page = strings.Replace(page, defaultTitleTag, fmt.Sprintf(`<title>%s</title>`, name), 1)
	page = strings.Replace(page, defaultOGSiteNameTag, fmt.Sprintf(`<meta property="og:site_name" content="%s" />`, name), 1)
	page = strings.Replace(page, defaultOGTitleTag, fmt.Sprintf(`<meta property="og:title" content="%s" />`, name), 1)
	page = strings.Replace(page, defaultOGDescription,
		fmt.Sprintf(`<meta property="og:description" content="Pantau progress pernikahan Anda secara real-time bersama %s." />`, name), 1)
	if meta.HasLogo {
		bareHost := host
		if i := strings.IndexByte(bareHost, ':'); i >= 0 {
			bareHost = bareHost[:i]
		}
		imageTag := fmt.Sprintf(`<meta property="og:image" content="https://%s/api/v1/public/logo" />`, html.EscapeString(strings.ToLower(bareHost)))
		page = strings.Replace(page, headCloseTag, imageTag+"\n  "+headCloseTag, 1)
	}
	return []byte(page)
}
