CREATE TABLE IF NOT EXISTS biz_commercial_features (
  feature_code VARCHAR(96) PRIMARY KEY,
  name VARCHAR(160) NOT NULL,
  product_state VARCHAR(32) NOT NULL,
  sales_state VARCHAR(32) NOT NULL,
  runtime_state VARCHAR(32) NOT NULL,
  migration_state VARCHAR(32) NOT NULL,
  replacement_feature_code VARCHAR(96) NOT NULL DEFAULT '',
  version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
);

CREATE TABLE IF NOT EXISTS biz_commercial_feature_modules (
  feature_code VARCHAR(96) NOT NULL,
  module_code VARCHAR(96) NOT NULL,
  capability_codes_json TEXT NOT NULL,
  PRIMARY KEY (feature_code, module_code),
  CONSTRAINT fk_commercial_feature_module_feature
    FOREIGN KEY (feature_code) REFERENCES biz_commercial_features(feature_code)
);

CREATE TABLE IF NOT EXISTS biz_commercial_feature_audit (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  feature_code VARCHAR(96) NOT NULL,
  actor VARCHAR(160) NOT NULL,
  action VARCHAR(64) NOT NULL,
  before_json TEXT,
  after_json TEXT,
  reason VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_commercial_feature_audit_feature (feature_code),
  INDEX idx_commercial_feature_audit_request (request_id)
);
