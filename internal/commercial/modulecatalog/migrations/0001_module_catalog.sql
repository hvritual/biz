CREATE TABLE IF NOT EXISTS biz_commercial_modules (
  module_code VARCHAR(96) PRIMARY KEY,
  name VARCHAR(160) NOT NULL,
  category VARCHAR(96) NOT NULL,
  sales_scope_json TEXT NOT NULL,
  technical_status VARCHAR(32) NOT NULL,
  sales_status VARCHAR(32) NOT NULL,
  version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
);
CREATE TABLE IF NOT EXISTS biz_commercial_module_dependencies (
  module_code VARCHAR(96) NOT NULL,
  depends_on VARCHAR(96) NOT NULL,
  PRIMARY KEY(module_code, depends_on),
  INDEX idx_commercial_module_depends_on(depends_on)
);
CREATE TABLE IF NOT EXISTS biz_commercial_module_retired_codes (
  module_code VARCHAR(96) PRIMARY KEY,
  retired_at DATETIME(6) NOT NULL
);
CREATE TABLE IF NOT EXISTS biz_commercial_module_audit (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  module_code VARCHAR(96) NOT NULL,
  actor VARCHAR(160) NOT NULL,
  action VARCHAR(64) NOT NULL,
  before_json TEXT,
  after_json TEXT,
  reason VARCHAR(512) NOT NULL,
  request_id VARCHAR(128),
  created_at DATETIME(6) NOT NULL,
  INDEX idx_commercial_audit_module(module_code),
  INDEX idx_commercial_audit_request(request_id)
);
CREATE TABLE IF NOT EXISTS biz_commercial_module_idempotency (
  request_id VARCHAR(128) PRIMARY KEY,
  operation VARCHAR(64) NOT NULL,
  module_code VARCHAR(96) NOT NULL,
  response_json TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL
);
