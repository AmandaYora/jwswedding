CREATE TABLE subscription_plans (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(100) NOT NULL,
  duration_months SMALLINT UNSIGNED NOT NULL,
  price BIGINT UNSIGNED NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
);

CREATE TABLE plan_features (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  plan_id BIGINT UNSIGNED NOT NULL,
  label VARCHAR(255) NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_plan_features_plan (plan_id),
  CONSTRAINT fk_plan_features_plan FOREIGN KEY (plan_id) REFERENCES subscription_plans (id)
);

CREATE TABLE platform_admins (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(150) NOT NULL,
  title VARCHAR(100) NOT NULL,
  role ENUM('Super Admin', 'Support') NOT NULL,
  username VARCHAR(100) NOT NULL,
  email VARCHAR(150) NOT NULL,
  phone VARCHAR(30) NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
);

CREATE TABLE payment_gateway_config (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  active_provider VARCHAR(50) NULL,
  is_sandbox BOOLEAN NOT NULL DEFAULT TRUE,
  tripay_merchant_code VARCHAR(100) NULL,
  tripay_api_key_encrypted TEXT NULL,
  tripay_private_key_encrypted TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
);

INSERT INTO payment_gateway_config (id, active_provider, is_sandbox) VALUES (1, NULL, TRUE);

CREATE TABLE payment_apps (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  app_id VARCHAR(100) NOT NULL,
  name VARCHAR(150) NOT NULL,
  kind ENUM('internal', 'external') NOT NULL,
  secret_hash VARCHAR(255) NULL,
  secret_encrypted TEXT NULL,
  callback_url VARCHAR(500) NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_payment_apps_app_id (app_id)
);

CREATE TABLE payment_charge_dispatch (
  order_ref VARCHAR(100) NOT NULL,
  app_id VARCHAR(100) NOT NULL,
  provider_ref VARCHAR(150) NULL,
  expires_at TIMESTAMP NULL,
  resolved_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (order_ref),
  INDEX idx_payment_charge_dispatch_unresolved (resolved_at, created_at)
);

CREATE TABLE payment_webhook_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  provider VARCHAR(50) NOT NULL,
  event_id VARCHAR(150) NOT NULL,
  received_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_payment_webhook_events (provider, event_id)
);
