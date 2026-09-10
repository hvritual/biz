-- Install before the CE-08 binary; preserve durable request history.
CREATE TABLE IF NOT EXISTS biz_tenant_creation_receipts (
 receipt_key VARCHAR(64) NOT NULL PRIMARY KEY,
 fingerprint VARCHAR(64) NOT NULL,
 payload MEDIUMTEXT NULL,
 CONSTRAINT ce08_tenant_receipt_json CHECK (payload IS NULL OR JSON_VALID(payload))
) ENGINE=InnoDB;
