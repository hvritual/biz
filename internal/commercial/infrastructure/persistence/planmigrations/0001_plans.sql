-- CE-07: authoring and immutable offers only. No subscription or billing tables.
CREATE TABLE IF NOT EXISTS biz_commercial_plans (
 plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 revision BIGINT UNSIGNED NOT NULL,
 latest_version BIGINT UNSIGNED NOT NULL
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_plan_versions (
 plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 version BIGINT UNSIGNED NOT NULL,
 revision BIGINT UNSIGNED NOT NULL,
 state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 content_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 PRIMARY KEY(plan_code,version),
 CONSTRAINT ce07_plan_parent FOREIGN KEY(plan_code) REFERENCES biz_commercial_plans(plan_code) ON DELETE RESTRICT ON UPDATE RESTRICT,
 CONSTRAINT ce07_version_payload CHECK(JSON_VALID(payload)),
 CONSTRAINT ce07_version_state CHECK(state IN ('DRAFT','PUBLISHED','RETIRED'))
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_plan_module_refs (
 plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 version BIGINT UNSIGNED NOT NULL,
 module_code VARCHAR(96) NOT NULL,
 PRIMARY KEY(plan_code,version,module_code),
 CONSTRAINT ce07_plan_module_version FOREIGN KEY(plan_code,version) REFERENCES biz_commercial_plan_versions(plan_code,version) ON DELETE RESTRICT ON UPDATE RESTRICT,
 CONSTRAINT ce07_plan_module_catalog FOREIGN KEY(module_code) REFERENCES biz_commercial_modules(module_code) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_plan_receipts (
 plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 PRIMARY KEY(plan_code,request_id),
 CONSTRAINT ce07_receipt_payload CHECK(JSON_VALID(payload))
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_plan_audit (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
 plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 version BIGINT UNSIGNED NOT NULL,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 action VARCHAR(32) NOT NULL,
 actor_id VARCHAR(200) NOT NULL,
 reason VARCHAR(512) NOT NULL,
 before_json MEDIUMTEXT NOT NULL,
 after_json MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 UNIQUE KEY ce07_once_audit(plan_code,request_id),
 CONSTRAINT ce07_audit_version FOREIGN KEY(plan_code,version) REFERENCES biz_commercial_plan_versions(plan_code,version) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
