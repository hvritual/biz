ALTER TABLE biz_user_password_credentials
  ADD COLUMN must_change BOOLEAN NOT NULL DEFAULT FALSE AFTER disabled,
  ADD COLUMN temporary_expires_at DATETIME(6) NULL AFTER must_change;

CREATE TABLE IF NOT EXISTS biz_member_activations (
  tenant_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  mode VARCHAR(32) NOT NULL,
  secret_hash VARCHAR(64) NOT NULL DEFAULT '',
  new_account BOOLEAN NOT NULL DEFAULT FALSE,
  state VARCHAR(24) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  consumed_at DATETIME(6) NULL,
  notification_event_id VARCHAR(64) NOT NULL DEFAULT '',
  notification_state VARCHAR(24) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (tenant_id, user_id),
  KEY idx_member_activation_secret (secret_hash),
  KEY idx_member_activation_state_expiry (state, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
