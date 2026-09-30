CREATE TABLE IF NOT EXISTS biz_commercial_plan_feature_refs (
  plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT UNSIGNED NOT NULL,
  feature_code VARCHAR(96) NOT NULL,
  PRIMARY KEY (plan_code, version, feature_code),
  CONSTRAINT fk_commercial_plan_feature_ref_version
    FOREIGN KEY (plan_code, version) REFERENCES biz_commercial_plan_versions(plan_code, version)
);
