-- CE-16 durable time transitions. This table stores only server-derived authority
-- references and bounded leases; no browser timer or client-provided script is trusted.
-- due_at is authoritative UTC; business_timezone records display semantics separately.
CREATE TABLE IF NOT EXISTS biz_commercial_time_transitions (
 transition_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 tenant_id VARCHAR(64) NOT NULL,
 authority_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 authority_version BIGINT UNSIGNED NOT NULL,
 due_at DATETIME(6) NOT NULL,
 business_timezone VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 revision BIGINT UNSIGNED NOT NULL,
 state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 lease_until DATETIME(6) NULL,
 payload_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 UNIQUE KEY ce16_authority(kind,tenant_id,authority_id,authority_version,due_at),
 KEY ce16_due(state,due_at,lease_until,transition_id),
 CONSTRAINT ce16_transition_json CHECK(JSON_VALID(payload)),
 CONSTRAINT ce16_transition_kind CHECK(kind IN ('SCHEDULED_CHANGE','ENTITLEMENT_EXPIRY','SUBSCRIPTION_BOUNDARY')),
 CONSTRAINT ce16_transition_state CHECK(state IN ('QUEUED','RUNNING','APPLIED','SUPERSEDED','RECONCILIATION_REQUIRED'))
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS biz_commercial_time_transition_audit (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
 transition_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 revision BIGINT UNSIGNED NOT NULL,
 state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 outcome VARCHAR(160) NOT NULL,
 payload_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 UNIQUE KEY ce16_transition_revision(transition_id,revision),
 CONSTRAINT ce16_transition_audit_json CHECK(JSON_VALID(payload)),
 CONSTRAINT ce16_transition_audit_fk FOREIGN KEY(transition_id) REFERENCES biz_commercial_time_transitions(transition_id) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
