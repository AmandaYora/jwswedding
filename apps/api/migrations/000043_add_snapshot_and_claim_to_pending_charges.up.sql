-- Standalone JWS Wedding (docs/plan/standalone-jwswedding/PLAN.md D8/D11): snapshot
-- the plan's name/price/duration at charge-creation time (jwswedding never calls
-- ElProof again on the activation path), plus an atomic claim column so a webhook
-- and the reconciler racing on the same order_ref can never double-activate.
ALTER TABLE pending_subscription_charges
    ADD COLUMN plan_name VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN plan_price BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN plan_duration_months INT NOT NULL DEFAULT 0,
    ADD COLUMN resolved_at TIMESTAMP NULL DEFAULT NULL,
    ADD INDEX idx_pending_subscription_charges_resolved_created (resolved_at, created_at);
