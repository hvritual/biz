CREATE TABLE IF NOT EXISTS biz_commercial_entitlement_snapshot_heads (
 tenant_id VARCHAR(64) NOT NULL PRIMARY KEY,
 version BIGINT UNSIGNED NOT NULL,
 source_version BIGINT UNSIGNED NOT NULL,
 catalog_revision BIGINT UNSIGNED NOT NULL,
 input_hash CHAR(64) NOT NULL,
 payload_sha256 CHAR(64) NOT NULL,
 invalidated BOOLEAN NOT NULL DEFAULT FALSE,
 CONSTRAINT chk_entitlement_snapshot_head_version CHECK (version > 0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_entitlement_snapshots (
 tenant_id VARCHAR(64) NOT NULL,
 version BIGINT UNSIGNED NOT NULL,
 payload LONGTEXT NOT NULL,
 payload_sha256 CHAR(64) NOT NULL,
 created_at DATETIME(6) NOT NULL,
 PRIMARY KEY(tenant_id,version)
) ENGINE=InnoDB;
