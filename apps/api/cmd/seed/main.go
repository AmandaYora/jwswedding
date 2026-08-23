// Command seed resets jwswedding_db to a minimal clean-slate state: one
// tenant and one staff Owner account — nothing else (no local plan catalog,
// no Platform Console; both were cut when this app went standalone, see
// ADR-0021). Safe to re-run. See internal/adminseed for the actual logic,
// which is shared with `cmd/server`'s `seed` subcommand (the image the VPS
// deploys never has this separate binary, only cmd/server — see
// docs/DEPLOYMENT.md).
package main

import (
	"context"
	"log"

	"jwswedding/internal/adminseed"
	"jwswedding/internal/shared/config"
	"jwswedding/internal/shared/database"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	if err := adminseed.Run(context.Background(), db); err != nil {
		log.Fatal(err)
	}
}
