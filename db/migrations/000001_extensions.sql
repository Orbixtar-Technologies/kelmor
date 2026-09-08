-- pgcrypto is created by panel-install as the postgres superuser.
-- The unprivileged panel role cannot CREATE EXTENSION; ignore that
-- when the extension is already present.
DO $$
BEGIN
  CREATE EXTENSION IF NOT EXISTS pgcrypto;
EXCEPTION
  WHEN insufficient_privilege THEN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgcrypto') THEN
      RAISE;
    END IF;
END $$;
