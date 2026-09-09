CREATE TABLE IF NOT EXISTS biz_commercial_catalog_state (
 id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
 version BIGINT UNSIGNED NOT NULL,
 CONSTRAINT chk_commercial_catalog_version CHECK (version > 0)
) ENGINE=InnoDB;
INSERT IGNORE INTO biz_commercial_catalog_state(id,version) VALUES(1,1);
