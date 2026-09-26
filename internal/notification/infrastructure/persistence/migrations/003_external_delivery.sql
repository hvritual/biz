ALTER TABLE biz_notification_external_tasks
  ADD COLUMN attempts INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN lease_owner VARCHAR(96) NOT NULL DEFAULT '',
  ADD COLUMN lease_token BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN lease_until DATETIME(6) NULL,
  ADD COLUMN next_attempt_at DATETIME(6) NULL,
  ADD COLUMN failure_code VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN provider_receipt VARCHAR(200) NOT NULL DEFAULT '',
  ADD COLUMN accepted_at DATETIME(6) NULL,
  ADD COLUMN delivered_at DATETIME(6) NULL,
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD KEY idx_notification_external_lease (state, lease_until),
  ADD KEY idx_notification_external_due (state, next_attempt_at);
