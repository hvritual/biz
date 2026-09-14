CREATE TABLE IF NOT EXISTS biz_departments (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  name VARCHAR(100) NOT NULL,
  parent_id VARCHAR(64) NOT NULL DEFAULT '',
  leader_user_id VARCHAR(64) NOT NULL DEFAULT '',
  email VARCHAR(320) NOT NULL DEFAULT '',
  phone VARCHAR(40) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  sort INT NOT NULL DEFAULT 0,
  version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uniq_department_name (tenant_id, name),
  KEY idx_department_tenant (tenant_id),
  KEY idx_department_parent (parent_id),
  KEY idx_department_leader (leader_user_id),
  KEY idx_department_status (status)
);

-- Runtime AutoMigrateTenantDepartment performs the idempotent owner grant backfill
-- because existing installations may have owner roles created before EC-RI-04.
