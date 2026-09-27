CREATE TABLE biz_service_api_credentials (
  key_id VARCHAR(128) NOT NULL,
  subject VARCHAR(200) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  secret_digest CHAR(64) NOT NULL,
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  not_before DATETIME(6) NULL,
  expires_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (key_id),
  KEY idx_service_api_subject (subject),
  KEY idx_service_api_tenant (tenant_id),
  KEY idx_service_api_disabled (disabled),
  KEY idx_service_api_expires (expires_at)
);

CREATE TABLE biz_service_api_operations (
  key_id VARCHAR(128) NOT NULL,
  operation VARCHAR(160) NOT NULL,
  PRIMARY KEY (key_id, operation),
  CONSTRAINT fk_service_api_operation_key
    FOREIGN KEY (key_id) REFERENCES biz_service_api_credentials(key_id)
    ON DELETE CASCADE
);

CREATE TABLE biz_service_api_nonces (
  key_id VARCHAR(128) NOT NULL,
  nonce VARCHAR(128) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (key_id, nonce),
  KEY idx_service_api_nonce_expiry (expires_at),
  CONSTRAINT fk_service_api_nonce_key
    FOREIGN KEY (key_id) REFERENCES biz_service_api_credentials(key_id)
    ON DELETE CASCADE
);
