ALTER TABLE biz_security_notification_outbox
  ADD COLUMN lease_owner VARCHAR(96) NOT NULL DEFAULT '',
  ADD COLUMN lease_token BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN lease_until DATETIME(6) NULL,
  ADD COLUMN next_attempt_at DATETIME(6) NULL,
  ADD COLUMN last_attempt_at DATETIME(6) NULL,
  ADD KEY idx_security_notification_due (state, next_attempt_at),
  ADD KEY idx_security_notification_lease (state, lease_until);
