CREATE TABLE IF NOT EXISTS biz_commercial_module_runtime_verifications (
  module_code VARCHAR(96) NOT NULL,
  module_version BIGINT UNSIGNED NOT NULL,
  evidence_digest CHAR(64) NOT NULL,
  source_tree CHAR(64) NOT NULL,
  actor VARCHAR(160) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (module_code, module_version),
  CONSTRAINT fk_commercial_runtime_verification_module
    FOREIGN KEY (module_code) REFERENCES biz_commercial_modules(module_code)
);
