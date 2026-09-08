ALTER TABLE database_users DROP CONSTRAINT IF EXISTS database_users_username_key;
ALTER TABLE database_users
    ADD CONSTRAINT database_users_account_username_engine
    UNIQUE (account_id, username, engine);
