-- #182: irreversible current-tenant self deletion marker.
-- Global Account credentials and other tenant memberships remain untouched.
ALTER TABLE biz_memberships
  ADD COLUMN self_deleted_at DATETIME(6) NULL AFTER avatar_asset_ref,
  ADD INDEX idx_biz_memberships_self_deleted_at (self_deleted_at);
