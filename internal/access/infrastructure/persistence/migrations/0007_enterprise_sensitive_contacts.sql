ALTER TABLE biz_users
  ADD COLUMN username VARCHAR(64) NULL,
  ADD COLUMN email_ciphertext TEXT NULL,
  ADD COLUMN email_lookup_hash VARCHAR(64) NULL,
  ADD COLUMN email_key_version VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX idx_biz_users_username ON biz_users(username);
CREATE UNIQUE INDEX uniq_biz_users_email_lookup_hash ON biz_users(email_lookup_hash);

ALTER TABLE biz_memberships
  ADD COLUMN email VARCHAR(320) NOT NULL DEFAULT '',
  ADD COLUMN email_ciphertext TEXT NULL,
  ADD COLUMN email_lookup_hash VARCHAR(64) NULL,
  ADD COLUMN email_key_version VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN phone_ciphertext TEXT NULL,
  ADD COLUMN phone_lookup_hash VARCHAR(64) NULL,
  ADD COLUMN phone_key_version VARCHAR(64) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX uniq_biz_memberships_email_lookup ON biz_memberships(tenant_id, email_lookup_hash);
CREATE UNIQUE INDEX uniq_biz_memberships_phone_lookup ON biz_memberships(tenant_id, phone_lookup_hash);

UPDATE biz_web_identities SET email = NULL WHERE email IS NOT NULL AND email <> '';
