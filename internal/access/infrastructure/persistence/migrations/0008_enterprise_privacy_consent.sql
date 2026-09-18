ALTER TABLE biz_idp_authorization_requests
  ADD COLUMN authenticated_user_id VARCHAR(64) NULL,
  ADD COLUMN login_audit_id BIGINT UNSIGNED NULL,
  ADD COLUMN authenticated_at DATETIME(6) NULL;

CREATE INDEX idx_biz_idp_authorization_requests_authenticated_user
  ON biz_idp_authorization_requests(authenticated_user_id);
CREATE INDEX idx_biz_idp_authorization_requests_login_audit
  ON biz_idp_authorization_requests(login_audit_id);

CREATE TABLE IF NOT EXISTS biz_privacy_consents (
  user_id VARCHAR(64) NOT NULL,
  agreement_version VARCHAR(128) NOT NULL,
  accepted_at DATETIME(6) NOT NULL,
  source VARCHAR(64) NOT NULL,
  login_audit_id BIGINT UNSIGNED NOT NULL,
  authorization_hash VARCHAR(64) NOT NULL,
  withdrawn_at DATETIME(6) NULL,
  withdrawn_source VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (user_id, agreement_version),
  KEY idx_biz_privacy_consents_accepted_at (accepted_at),
  KEY idx_biz_privacy_consents_login_audit_id (login_audit_id),
  KEY idx_biz_privacy_consents_withdrawn_at (withdrawn_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
