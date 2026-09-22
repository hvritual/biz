CREATE TABLE biz_data_policies (
  tenant_id VARCHAR(64) NOT NULL,
  id VARCHAR(160) NOT NULL,
  name VARCHAR(100) NOT NULL,
  status VARCHAR(32) NOT NULL,
  version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  not_before DATETIME(3) NULL,
  expires_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (tenant_id, id),
  KEY idx_data_policy_status (tenant_id, status)
);

CREATE TABLE biz_data_policy_sites (
  tenant_id VARCHAR(64) NOT NULL,
  policy_id VARCHAR(160) NOT NULL,
  site_id VARCHAR(160) NOT NULL,
  PRIMARY KEY (tenant_id, policy_id, site_id),
  KEY idx_data_policy_site (tenant_id, site_id)
);

ALTER TABLE biz_roles
  ADD COLUMN data_policy_id VARCHAR(160) NULL AFTER system_role,
  ADD COLUMN data_policy_accepted_version BIGINT UNSIGNED NULL AFTER data_policy_id;

CREATE INDEX idx_role_data_policy ON biz_roles (tenant_id, data_policy_id);
