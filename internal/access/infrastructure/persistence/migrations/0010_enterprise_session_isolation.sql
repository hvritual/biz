ALTER TABLE biz_web_sessions
  ADD COLUMN context_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN revoked_reason VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN revoked_scope VARCHAR(160) NOT NULL DEFAULT '';

CREATE INDEX idx_biz_web_sessions_context_version
  ON biz_web_sessions(context_version);
