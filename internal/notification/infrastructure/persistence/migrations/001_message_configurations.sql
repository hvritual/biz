-- Additive #184 schema. Apply in the authorized migration lane only.
-- Rollback disables configuration consumption, preserving receipts and audit.
CREATE TABLE IF NOT EXISTS biz_notification_configurations (
 id VARBINARY(64) NOT NULL PRIMARY KEY,
 tenant_id VARBINARY(64) NOT NULL,
 group_id VARBINARY(64) NOT NULL,
 level VARCHAR(16) NOT NULL,
 primary_user_id VARBINARY(64) NOT NULL,
 secondary_user_id VARBINARY(64) NOT NULL,
 notes TEXT NOT NULL,
 version BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(3) NOT NULL,
 updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_notification_configuration (tenant_id,group_id,level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS biz_notification_configuration_channels (
 tenant_id VARBINARY(64) NOT NULL,
 configuration_id VARBINARY(64) NOT NULL,
 channel VARCHAR(64) NOT NULL,
 PRIMARY KEY (tenant_id,configuration_id,channel)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS biz_notification_configuration_recipients (
 tenant_id VARBINARY(64) NOT NULL,
 configuration_id VARBINARY(64) NOT NULL,
 user_id VARBINARY(64) NOT NULL,
 PRIMARY KEY (tenant_id,configuration_id,user_id),
 KEY idx_notification_recipient (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS biz_notification_configuration_receipts (
 tenant_id VARBINARY(64) NOT NULL,
 actor_id VARBINARY(64) NOT NULL,
 operation VARCHAR(64) NOT NULL,
 key_hash VARCHAR(64) NOT NULL,
 request_hash VARCHAR(64) NOT NULL,
 response LONGTEXT NOT NULL,
 created_at DATETIME(3) NOT NULL,
 PRIMARY KEY (tenant_id,actor_id,operation,key_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
