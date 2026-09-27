CREATE TABLE IF NOT EXISTS biz_notification_inbox_mark_all (
  command_id VARCHAR(64) NOT NULL,
  tenant_id VARBINARY(64) NOT NULL,
  user_id VARBINARY(64) NOT NULL,
  state VARCHAR(16) NOT NULL,
  marked_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  read_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (command_id),
  KEY idx_notification_inbox_command_owner (tenant_id,user_id),
  KEY idx_notification_inbox_command_state (state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS biz_notification_inbox_mark_all_items (
  command_id VARCHAR(64) NOT NULL,
  message_id VARCHAR(64) NOT NULL,
  PRIMARY KEY (command_id,message_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE biz_notification_in_app
  ADD KEY idx_notification_inapp_unread (tenant_id,user_id,read_at,created_at DESC,message_id DESC);
