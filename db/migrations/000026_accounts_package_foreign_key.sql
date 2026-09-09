ALTER TABLE accounts
    ADD CONSTRAINT accounts_package_id_fkey
    FOREIGN KEY (package_id)
    REFERENCES packages(id)
    ON DELETE RESTRICT
    NOT VALID;
