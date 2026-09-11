-- CE-10 extends the already released CE-09 receipt state contract.
-- Preserve the original migration and existing receipt history. One ALTER
-- replaces the check atomically; arbitrary status strings remain forbidden.
-- MigrateProvisioning executes this complete file on one connection. Repeated
-- development/runtime bootstrap must not attempt to drop the old check twice.
SET @ce10_receipt_migration_sql = (
 SELECT IF(EXISTS(
  SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA=DATABASE()
    AND TABLE_NAME='biz_commercial_change_receipts'
    AND CONSTRAINT_NAME='ce10_receipt_status'
    AND CONSTRAINT_TYPE='CHECK'
 ),
 'DO 0',
 'ALTER TABLE biz_commercial_change_receipts DROP CHECK ce09_receipt_status, ADD CONSTRAINT ce10_receipt_status CHECK (status IN (''APPLIED'',''SCHEDULED'',''PROVISIONING''))'
 )
);
PREPARE ce10_receipt_migration_stmt FROM @ce10_receipt_migration_sql;
EXECUTE ce10_receipt_migration_stmt;
DEALLOCATE PREPARE ce10_receipt_migration_stmt;
SET @ce10_receipt_migration_sql = NULL;
