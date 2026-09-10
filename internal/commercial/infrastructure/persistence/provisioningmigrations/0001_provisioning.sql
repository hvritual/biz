CREATE TABLE IF NOT EXISTS biz_commercial_provisioning_tasks (
 task_id VARCHAR(64) NOT NULL PRIMARY KEY,
 tenant_id VARCHAR(64) NOT NULL,
 change_id VARCHAR(64) NOT NULL,
 revision BIGINT UNSIGNED NOT NULL,
 state VARCHAR(32) NOT NULL,
 next_attempt_at DATETIME(6) NOT NULL,
 lease_until DATETIME(6) NULL,
 payload_sha256 CHAR(64) NOT NULL,
 payload LONGTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_provisioning_change(change_id),
 KEY ix_provisioning_due(state,next_attempt_at,task_id),
 KEY ix_provisioning_lease(state,lease_until,task_id),
 KEY ix_provisioning_tenant(tenant_id,task_id),
 CONSTRAINT fk_provisioning_preview FOREIGN KEY(change_id) REFERENCES biz_commercial_change_previews(change_id) ON DELETE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_provisioning_audit (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 task_id VARCHAR(64) NOT NULL,
 revision BIGINT UNSIGNED NOT NULL,
 actor_id VARCHAR(200) NOT NULL,
 action VARCHAR(64) NOT NULL,
 reason VARCHAR(512) NOT NULL,
 payload_sha256 CHAR(64) NOT NULL,
 payload LONGTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_provisioning_audit(task_id,revision),
 CONSTRAINT fk_provisioning_audit_task FOREIGN KEY(task_id) REFERENCES biz_commercial_provisioning_tasks(task_id) ON DELETE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_provisioning_actions (
 receipt_key CHAR(64) NOT NULL PRIMARY KEY,
 actor_id VARCHAR(200) NOT NULL,
 operation VARCHAR(32) NOT NULL,
 request_id VARCHAR(128) NOT NULL,
 fingerprint CHAR(64) NOT NULL,
 task_id VARCHAR(64) NOT NULL,
 payload_sha256 CHAR(64) NOT NULL,
 payload LONGTEXT NOT NULL,
 UNIQUE KEY uq_provisioning_action(actor_id,operation,request_id),
 CONSTRAINT fk_provisioning_action_task FOREIGN KEY(task_id) REFERENCES biz_commercial_provisioning_tasks(task_id) ON DELETE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_outbox (
 event_id VARCHAR(64) NOT NULL PRIMARY KEY,
 tenant_id VARCHAR(64) NOT NULL,
 aggregate_id VARCHAR(64) NOT NULL,
 aggregate_version BIGINT UNSIGNED NOT NULL,
 change_id VARCHAR(64) NOT NULL,
 payload_sha256 CHAR(64) NOT NULL,
 payload LONGTEXT NOT NULL,
 state VARCHAR(24) NOT NULL,
 attempts INT UNSIGNED NOT NULL DEFAULT 0,
 lease_owner VARCHAR(128) NOT NULL DEFAULT '',
 lease_token BIGINT UNSIGNED NOT NULL DEFAULT 0,
 lease_until DATETIME(6) NULL,
 next_attempt_at DATETIME(6) NOT NULL,
 failure_code VARCHAR(128) NOT NULL DEFAULT '',
 occurred_at DATETIME(6) NOT NULL,
 delivered_at DATETIME(6) NULL,
 UNIQUE KEY uq_subscription_event(tenant_id,aggregate_id,aggregate_version),
 KEY ix_commercial_outbox_due(state,next_attempt_at,event_id),
 KEY ix_commercial_outbox_lease(state,lease_until,event_id),
 CONSTRAINT fk_outbox_change FOREIGN KEY(change_id) REFERENCES biz_commercial_change_previews(change_id) ON DELETE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_inbox (
 consumer_id VARCHAR(64) NOT NULL,
 event_id VARCHAR(64) NOT NULL,
 payload_sha256 CHAR(64) NOT NULL,
 outcome VARCHAR(24) NOT NULL,
 received_at DATETIME(6) NOT NULL,
 PRIMARY KEY(consumer_id,event_id),
 CONSTRAINT fk_inbox_outbox FOREIGN KEY(event_id) REFERENCES biz_commercial_outbox(event_id) ON DELETE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS biz_commercial_subscription_notifications (
 tenant_id VARCHAR(64) NOT NULL,
 aggregate_id VARCHAR(64) NOT NULL,
 aggregate_version BIGINT UNSIGNED NOT NULL,
 payload_sha256 CHAR(64) NOT NULL,
 payload LONGTEXT NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 PRIMARY KEY(tenant_id,aggregate_id)
) ENGINE=InnoDB;
