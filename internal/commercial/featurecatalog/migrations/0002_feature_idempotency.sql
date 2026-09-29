CREATE TABLE IF NOT EXISTS biz_commercial_feature_idempotency (
  request_id VARCHAR(128) PRIMARY KEY,
  operation VARCHAR(64) NOT NULL,
  feature_code VARCHAR(96) NOT NULL,
  response_json TEXT NOT NULL
);
