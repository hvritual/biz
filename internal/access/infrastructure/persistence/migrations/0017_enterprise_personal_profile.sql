-- #181: tenant-scoped personal profile avatar.
-- The value is a server-approved built-in asset key, never a URL or DataURL.
ALTER TABLE biz_memberships
  ADD COLUMN avatar_asset_ref VARCHAR(64) NOT NULL DEFAULT 'avatar:coffee-blue' AFTER department_id;
