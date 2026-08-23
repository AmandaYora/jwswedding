-- Standalone JWS Wedding (see knowledge/decisions/ADR-0021-standalone-jws-payment-via-elproof-api.md D2/D6/D7):
-- the Platform Console (platform_admins), the payment gateway operator role
-- (payment_*, entirely replaced by internal/shared/elproofpay calling
-- ElProof's external API), and the local plan catalog (subscription_plans/
-- plan_features, now sourced live from ElProof) are all gone. Written after
-- the code that used to query these tables was already deleted, so nothing
-- still queries them at boot.
DROP TABLE payment_webhook_events;
DROP TABLE payment_charge_dispatch;
DROP TABLE payment_apps;
DROP TABLE payment_gateway_config;
DROP TABLE platform_admins;
DROP TABLE plan_features;
DROP TABLE subscription_plans;
