-- CE-09 additive migration. Apply after 0001_subscriptions; no destructive
-- rewrites or deletion of historical plans, subscriptions, sources or receipts.
CREATE TABLE IF NOT EXISTS biz_commercial_change_previews (
 change_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id VARCHAR(64) NOT NULL,
 actor_id VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 source_plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 source_plan_version BIGINT UNSIGNED NOT NULL,
 target_plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 target_plan_version BIGINT UNSIGNED NOT NULL,
 fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 expires_at DATETIME(6) NOT NULL,
 UNIQUE KEY ce09_preview_request(actor_id,request_id),
 CONSTRAINT ce09_preview_json CHECK(JSON_VALID(payload)),
 CONSTRAINT ce09_preview_time CHECK(expires_at>created_at),
 CONSTRAINT ce09_preview_tenant FOREIGN KEY(tenant_id) REFERENCES biz_commercial_subscriptions(tenant_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
 CONSTRAINT ce09_preview_source_plan FOREIGN KEY(source_plan_code,source_plan_version) REFERENCES biz_commercial_plan_versions(plan_code,version) ON DELETE RESTRICT ON UPDATE RESTRICT,
 CONSTRAINT ce09_preview_target_plan FOREIGN KEY(target_plan_code,target_plan_version) REFERENCES biz_commercial_plan_versions(plan_code,version) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_change_receipts (
 change_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id VARCHAR(64) NOT NULL,
 actor_id VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 confirmed_at DATETIME(6) NOT NULL,
 UNIQUE KEY ce09_confirm_request(actor_id,request_id),
 CONSTRAINT ce09_receipt_json CHECK(JSON_VALID(payload)),
 CONSTRAINT ce09_receipt_status CHECK(status IN ('APPLIED','SCHEDULED')),
 CONSTRAINT ce09_receipt_preview FOREIGN KEY(change_id) REFERENCES biz_commercial_change_previews(change_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
 CONSTRAINT ce09_receipt_tenant FOREIGN KEY(tenant_id) REFERENCES biz_commercial_subscriptions(tenant_id) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_change_audit (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
 change_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL UNIQUE,
 tenant_id VARCHAR(64) NOT NULL,
 actor_id VARCHAR(200) NOT NULL,
 payload_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 CONSTRAINT ce09_audit_json CHECK(JSON_VALID(payload)),
 CONSTRAINT ce09_audit_receipt FOREIGN KEY(change_id) REFERENCES biz_commercial_change_receipts(change_id) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
