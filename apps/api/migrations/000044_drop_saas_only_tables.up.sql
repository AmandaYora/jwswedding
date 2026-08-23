-- Standalone JWS Wedding (docs/plan/standalone-jwswedding/PLAN.md D2/D6/D7):
-- the Platform Console (platform_admins), the payment gateway operator role
-- (payment_*, entirely replaced by internal/shared/elproofpay calling
-- ElProof's external API), and the local plan catalog (subscription_plans/
-- plan_features, now sourced live from ElProof) are all gone. Written after
-- Fase 5 and 7.1-7.7 so nothing still queries these tables at boot.
DROP TABLE payment_webhook_events;
DROP TABLE payment_charge_dispatch;
DROP TABLE payment_apps;
DROP TABLE payment_gateway_config;
DROP TABLE platform_admins;
DROP TABLE plan_features;
DROP TABLE subscription_plans;
