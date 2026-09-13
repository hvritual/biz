ALTER TABLE biz_memberships
  ADD COLUMN name VARCHAR(100) NOT NULL DEFAULT '',
  ADD COLUMN phone VARCHAR(40) NOT NULL DEFAULT '',
  ADD COLUMN employee_id VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN position VARCHAR(100) NOT NULL DEFAULT '',
  ADD COLUMN department_id VARCHAR(64) NOT NULL DEFAULT '',
  ADD INDEX idx_biz_memberships_department (tenant_id, department_id);
