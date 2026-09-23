-- Git connect and auto-deploy fields for applications.
-- git_auth_token and deploy_webhook_token are opaque and never returned by the API.

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS git_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS git_branch TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS git_auth_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS auto_deploy BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS deploy_webhook_token TEXT NOT NULL DEFAULT '';
