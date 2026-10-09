-- First activation has a tenant but no Subscription or source PlanVersion.
-- Reference the existing commercial serialization row; Access tenant existence
-- is independently validated by the declared tenant.get child at preview and
-- confirmation. This does not grant any entitlement or create a Subscription.
-- Historical rows/payloads and the exact target/receipt foreign keys are kept.
-- One atomic ALTER avoids an interval without source/tenant protection. Invalid
-- legacy rows (including a missing entitlement-state anchor) fail migration;
-- never synthesize state, backfill subscriptions, or disable FK checks here.
SET @ce340_preview_migration_sql = (
 SELECT IF(EXISTS(
  SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA=DATABASE()
    AND TABLE_NAME='biz_commercial_change_previews'
    AND CONSTRAINT_NAME='ce09_preview_tenant'
    AND CONSTRAINT_TYPE='FOREIGN KEY'
 ),
 'ALTER TABLE biz_commercial_change_previews
  DROP FOREIGN KEY ce09_preview_tenant,
  DROP FOREIGN KEY ce09_preview_source_plan,
  MODIFY tenant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  MODIFY source_plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  MODIFY source_plan_version BIGINT UNSIGNED NULL,
  ADD CONSTRAINT ce340_preview_tenant FOREIGN KEY(tenant_id)
    REFERENCES biz_commercial_entitlement_state(tenant_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  ADD CONSTRAINT ce340_preview_source_plan FOREIGN KEY(source_plan_code,source_plan_version)
    REFERENCES biz_commercial_plan_versions(plan_code,version) ON DELETE RESTRICT ON UPDATE RESTRICT,
  ADD CONSTRAINT ce340_preview_source_shape CHECK (
    (COALESCE(JSON_UNQUOTE(JSON_EXTRACT(payload,''$.input.action'')),'''')=''INITIAL''
      AND source_plan_code IS NULL AND source_plan_version IS NULL)
    OR
    (COALESCE(JSON_UNQUOTE(JSON_EXTRACT(payload,''$.input.action'')),'''') IN (''SWITCH'',''RENEW'',''STOP_RENEWAL'')
      AND source_plan_code IS NOT NULL AND source_plan_version IS NOT NULL
      AND source_plan_code<>'''' AND source_plan_version>0)
  )',
 'DO 0'
 )
);
PREPARE ce340_preview_migration_stmt FROM @ce340_preview_migration_sql;
EXECUTE ce340_preview_migration_stmt;
DEALLOCATE PREPARE ce340_preview_migration_stmt;
SET @ce340_preview_migration_sql = NULL;
