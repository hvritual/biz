-- #183 / FR-99..102: absence means the source-required default allow.
-- Only explicit choices are persisted; never backfill allow over existing deny.
-- Retain BOTH tables on application rollback. No destructive down migration.
CREATE TABLE IF NOT EXISTS biz_notification_preferences (
    tenant_id VARBINARY(64) NOT NULL,
    user_id VARBINARY(64) NOT NULL,
    channel VARBINARY(16) NOT NULL,
    state VARCHAR(16) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (tenant_id, user_id, channel)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS biz_notification_preference_receipts (
    tenant_id VARBINARY(64) NOT NULL,
    user_id VARBINARY(64) NOT NULL,
    key_hash VARBINARY(64) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    policy VARCHAR(64) NOT NULL,
    channel VARCHAR(16) NOT NULL,
    state VARCHAR(16) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (tenant_id, user_id, key_hash)
) ENGINE=InnoDB;
