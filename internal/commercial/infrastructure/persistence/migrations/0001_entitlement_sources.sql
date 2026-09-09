-- CE-04: source facts only; no editable entitlement snapshot and no usage counter.
CREATE TABLE IF NOT EXISTS biz_commercial_entitlement_state (
 tenant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 version BIGINT UNSIGNED NOT NULL
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_entitlement_sources (
 tenant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 source_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 version BIGINT UNSIGNED NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 PRIMARY KEY(tenant_id,source_id),
 CONSTRAINT ce04_source_json CHECK (JSON_VALID(payload))
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_entitlement_receipts (
 tenant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 PRIMARY KEY(tenant_id,request_id),
 CONSTRAINT ce04_receipt_json CHECK (JSON_VALID(payload))
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_entitlement_audit (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
 tenant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 actor_id VARCHAR(200) NOT NULL,
 actor_type VARCHAR(32) NOT NULL,
 action VARCHAR(32) NOT NULL,
 reason VARCHAR(512) NOT NULL,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 before_version BIGINT UNSIGNED NOT NULL,
 after_version BIGINT UNSIGNED NOT NULL,
 before_json MEDIUMTEXT NOT NULL,
 after_json MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 INDEX ce04_audit_tenant(tenant_id,id)
) ENGINE=InnoDB;
