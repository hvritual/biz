CREATE TABLE IF NOT EXISTS biz_member_removed_role_snapshots (
  tenant_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  role_id VARCHAR(160) NOT NULL,
  removed_version BIGINT UNSIGNED NOT NULL,
  captured_at DATETIME(6) NOT NULL,
  PRIMARY KEY (tenant_id, user_id, role_id),
  KEY idx_member_removed_role_version (removed_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS biz_member_removed_site_snapshots (
  tenant_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  site_id VARCHAR(64) NOT NULL,
  removed_version BIGINT UNSIGNED NOT NULL,
  captured_at DATETIME(6) NOT NULL,
  PRIMARY KEY (tenant_id, user_id, site_id),
  KEY idx_member_removed_site_version (removed_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS biz_member_status_appeals (
  tenant_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  appeal_id VARCHAR(64) NOT NULL,
  membership_status VARCHAR(32) NOT NULL,
  state VARCHAR(24) NOT NULL,
  reason VARCHAR(500) NOT NULL,
  notification_event_ids TEXT NULL,
  submitted_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (tenant_id, user_id),
  UNIQUE KEY uniq_member_status_appeal_id (appeal_id),
  KEY idx_member_status_appeal_state (state),
  KEY idx_member_status_appeal_submitted (submitted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
