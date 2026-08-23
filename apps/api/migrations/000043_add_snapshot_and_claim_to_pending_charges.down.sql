ALTER TABLE pending_subscription_charges
    DROP INDEX idx_pending_subscription_charges_resolved_created,
    DROP COLUMN plan_name,
    DROP COLUMN plan_price,
    DROP COLUMN plan_duration_months,
    DROP COLUMN resolved_at;
