-- #293 tenant.role.create: apply to the existing database BEFORE deploying
-- a binary with durable role creation replay. Production auto-migration is off.
-- Existing completed claims have no recoverable original result: do not
-- fabricate, backfill or overwrite historical receipts. Keep this table on
-- binary rollback so idempotency history is not lost.
CREATE TABLE IF NOT EXISTS biz_role_creation_receipts (
  receipt_key VARCHAR(64) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  payload MEDIUMTEXT NULL,
  CONSTRAINT access_role_receipt_json CHECK (payload IS NULL OR JSON_VALID(payload))
) ENGINE=InnoDB;
