CREATE TABLE IF NOT EXISTS biz_user_password_credentials (
  user_id VARCHAR(64) NOT NULL PRIMARY KEY,
  salt VARCHAR(128) NOT NULL,
  password_hash VARCHAR(128) NOT NULL,
  iterations INT NOT NULL,
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  password_changed_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_authorization_requests (
  request_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  browser_hash VARCHAR(64) NOT NULL,
  csrf_hash VARCHAR(64) NOT NULL,
  client_id VARCHAR(200) NOT NULL,
  redirect_uri VARCHAR(1024) NOT NULL,
  state VARCHAR(1024) NOT NULL,
  nonce VARCHAR(512) NOT NULL,
  code_challenge VARCHAR(128) NOT NULL,
  scope VARCHAR(1024) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_idp_request_browser (browser_hash),
  KEY idx_idp_request_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_authorization_codes (
  code_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  client_id VARCHAR(200) NOT NULL,
  redirect_uri VARCHAR(1024) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  nonce VARCHAR(512) NOT NULL,
  code_challenge VARCHAR(128) NOT NULL,
  scope VARCHAR(1024) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_idp_code_user (user_id),
  KEY idx_idp_code_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_login_throttles (
  identity_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  failure_count INT NOT NULL,
  window_started_at DATETIME(6) NOT NULL,
  blocked_until DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_idp_throttle_blocked (blocked_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_login_audit (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  occurred_at DATETIME(6) NOT NULL,
  outcome VARCHAR(32) NOT NULL,
  user_id VARCHAR(64) NULL,
  email_hash VARCHAR(64) NOT NULL,
  source_hash VARCHAR(64) NOT NULL,
  KEY idx_idp_audit_occurred (occurred_at),
  KEY idx_idp_audit_outcome (outcome),
  KEY idx_idp_audit_user (user_id),
  KEY idx_idp_audit_email (email_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_web_identities (
  issuer VARCHAR(512) NOT NULL,
  subject VARCHAR(255) NOT NULL,
  actor_kind VARCHAR(32) NOT NULL,
  actor_id VARCHAR(200) NOT NULL,
  email VARCHAR(320) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (issuer, subject),
  KEY idx_web_identity_actor_kind (actor_kind),
  KEY idx_web_identity_actor_id (actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_web_sessions (
  token_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  issuer VARCHAR(512) NOT NULL,
  subject VARCHAR(255) NOT NULL,
  active_tenant_id VARCHAR(64) NULL,
  csrf_token VARCHAR(128) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  revoked_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_web_session_identity (issuer, subject),
  KEY idx_web_session_tenant (active_tenant_id),
  KEY idx_web_session_expires (expires_at),
  KEY idx_web_session_revoked (revoked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_web_login_flows (
  state_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  browser_hash VARCHAR(64) NOT NULL,
  code_verifier VARCHAR(160) NOT NULL,
  nonce_hash VARCHAR(64) NOT NULL,
  return_to VARCHAR(1024) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_web_login_browser (browser_hash),
  KEY idx_web_login_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
