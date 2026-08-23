// Package config loads environment configuration for the API process.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	AppEnv  string
	AppPort string
	AppName string

	DatabaseURL string

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool

	// ElProofPaymentBaseURL is the base URL of the ElProof external API that
	// jwswedding consumes as a `kind=external` App to pay for its own
	// subscription — see knowledge/MODULE_SUBSCRIPTION.md and
	// knowledge/decisions/ADR-0021-standalone-jws-payment-via-elproof-api.md.
	ElProofPaymentBaseURL string
	ElProofAppID          string
	ElProofAppSecret      string

	// ElProofReconcileInterval is how often the platform module re-checks
	// pending charges whose webhook was never received against ElProof
	// directly — the safety net for the fire-and-forget webhook relay.
	ElProofReconcileInterval time.Duration

	// ElProofChargeMaxAge is how old a pending charge with a definitive
	// "unpaid" answer from ElProof must be before the reconciler force-closes
	// it as failed. A charge whose check errored is never force-closed
	// regardless of age.
	ElProofChargeMaxAge time.Duration
}

// Load reads .env from the repo root (or the current directory) into the process
// environment, without overriding variables already set by the real OS environment,
// then builds a Config from it.
func Load() Config {
	loadDotEnv(".env")
	loadDotEnv(filepath.Join("..", "..", ".env"))

	return Config{
		AppEnv:  getEnv("APP_ENV", "development"),
		AppPort: getEnv("APP_PORT", "8080"),
		AppName: getEnv("APP_NAME", "JWS Wedding"),

		DatabaseURL: getEnv("DATABASE_URL", ""),

		JWTSecret:     getEnv("JWT_SECRET", ""),
		JWTAccessTTL:  getDuration("JWT_ACCESS_TTL", 30*time.Minute),
		JWTRefreshTTL: getDuration("JWT_REFRESH_TTL", 168*time.Hour),

		S3Endpoint:  getEnv("S3_ENDPOINT", ""),
		S3Bucket:    getEnv("S3_BUCKET", ""),
		S3AccessKey: getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey: getEnv("S3_SECRET_KEY", ""),
		S3UseSSL:    getEnv("S3_USE_SSL", "true") == "true",

		ElProofPaymentBaseURL:    getEnv("ELPROOF_PAYMENT_BASE_URL", "https://elproof.elcodelabs.com/api/v1"),
		ElProofAppID:             getEnv("ELPROOF_APP_ID", ""),
		ElProofAppSecret:         getEnv("ELPROOF_APP_SECRET", ""),
		ElProofReconcileInterval: getDuration("ELPROOF_RECONCILE_INTERVAL", 3*time.Minute),
		ElProofChargeMaxAge:      getDuration("ELPROOF_CHARGE_MAX_AGE", 24*time.Hour),
	}
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if _, alreadySet := os.LookupEnv(key); !alreadySet {
			_ = os.Setenv(key, value)
		}
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
