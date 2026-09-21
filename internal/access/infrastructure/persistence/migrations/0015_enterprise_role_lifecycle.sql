ALTER TABLE biz_roles
  ADD COLUMN description VARCHAR(120) NOT NULL DEFAULT '' AFTER name,
  ADD COLUMN role_code VARCHAR(64) NULL AFTER description,
  ADD COLUMN system_role BOOLEAN NOT NULL DEFAULT FALSE AFTER role_code;

CREATE UNIQUE INDEX uniq_role_code ON biz_roles (tenant_id, role_code);

UPDATE biz_roles
SET role_code = 'tenant_owner',
    system_role = TRUE,
    status = 'active'
WHERE id = CONCAT(tenant_id, ':owner')
  AND name = 'owner';

INSERT INTO biz_roles (id, tenant_id, name, description, role_code, system_role, status, version)
SELECT CONCAT(t.id, ':admin'),
       t.id,
       'tenant_admin',
       '',
       'tenant_admin',
       TRUE,
       'active',
       1
FROM biz_tenants t
WHERE NOT EXISTS (
  SELECT 1
  FROM biz_roles r
  WHERE r.tenant_id = t.id
    AND r.role_code = 'tenant_admin'
);
