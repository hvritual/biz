-- CE-22: payment facts are additive and remain independently auditable from
-- the subscription change receipt. Provider callbacks never own entitlement
-- rows directly.
CREATE TABLE IF NOT EXISTS biz_commercial_payment_orders (
 order_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id VARCHAR(64) NOT NULL,
 change_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 plan_version BIGINT UNSIGNED NOT NULL,
 price_ref VARCHAR(128) NOT NULL,
 currency CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 amount_minor BIGINT UNSIGNED NOT NULL,
 provider VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 revision BIGINT UNSIGNED NOT NULL,
 provider_transaction_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
 payload MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 expires_at DATETIME(6) NOT NULL,
 paid_at DATETIME(6) NULL,
 UNIQUE KEY ce22_order_change(change_id),
 UNIQUE KEY ce22_provider_transaction(provider,provider_transaction_id),
 KEY ce22_tenant_created(tenant_id,created_at),
 CONSTRAINT ce22_order_change_fk FOREIGN KEY(change_id) REFERENCES biz_commercial_change_previews(change_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
 CONSTRAINT ce22_order_plan_fk FOREIGN KEY(plan_code,plan_version) REFERENCES biz_commercial_plan_versions(plan_code,version) ON DELETE RESTRICT ON UPDATE RESTRICT,
 CONSTRAINT ce22_order_state CHECK(state IN ('PENDING','PAID','CANCELLED','EXPIRED')),
 CONSTRAINT ce22_order_payload CHECK(JSON_VALID(payload))
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS biz_commercial_payment_inbox (
 provider VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 notification_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 order_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 outcome VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 received_at DATETIME(6) NOT NULL,
 PRIMARY KEY(provider,notification_id),
 CONSTRAINT ce22_payment_inbox_order_fk FOREIGN KEY(order_id) REFERENCES biz_commercial_payment_orders(order_id) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
