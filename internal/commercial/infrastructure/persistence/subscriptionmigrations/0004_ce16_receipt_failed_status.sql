-- CE-16 extends the CE-10 receipt execution-state contract. Preserve all
-- historical rows; only widen the allowed stable execution states.
SET @ce16_receipt_migration_sql = (
 SELECT IF(EXISTS(
  SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA=DATABASE()
    AND TABLE_NAME='biz_commercial_change_receipts'
    AND CONSTRAINT_NAME='ce16_receipt_status'
    AND CONSTRAINT_TYPE='CHECK'
 ),
 'DO 0',
 IF(EXISTS(
  SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA=DATABASE()
    AND TABLE_NAME='biz_commercial_change_receipts'
    AND CONSTRAINT_NAME='ce10_receipt_status'
    AND CONSTRAINT_TYPE='CHECK'
 ),
 'ALTER TABLE biz_commercial_change_receipts DROP CHECK ce10_receipt_status, ADD CONSTRAINT ce16_receipt_status CHECK (status IN (''APPLIED'',''SCHEDULED'',''PROVISIONING'',''FAILED''))',
 'DO 0'))
);
PREPARE ce16_receipt_migration_stmt FROM @ce16_receipt_migration_sql;
EXECUTE ce16_receipt_migration_stmt;
DEALLOCATE PREPARE ce16_receipt_migration_stmt;
SET @ce16_receipt_migration_sql = NULL;
