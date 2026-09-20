-- #176 / Q-004: username is a global Account identifier.
-- Existing duplicate non-NULL usernames intentionally make this migration fail;
-- production qualification must reconcile them explicitly rather than silently
-- merging Accounts.
DROP INDEX idx_biz_users_username ON biz_users;
CREATE UNIQUE INDEX uniq_biz_users_username ON biz_users(username);
