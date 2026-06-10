-- api_ro: least-privilege read-only login role for the query-side api
-- (security audit F-09, CWE-250). The api container must not connect as the
-- table-owning role: a future query-side bug would otherwise run with enough
-- privilege to disable the append-only triggers or rewrite the hash chain.
--
-- Created NOLOGIN here because migrations are committed to the repo and must
-- not contain credentials. The operator enables login and sets the password
-- from secrets/api_db_password.txt via scripts/set-api-ro-password.sh.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'api_ro') THEN
    CREATE ROLE api_ro NOLOGIN;
  END IF;
END
$$;

GRANT USAGE ON SCHEMA observation, interpretation, control TO api_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA observation, interpretation, control TO api_ro;

-- Future tables created by the owning (migration) role stay readable without
-- a follow-up GRANT per migration.
ALTER DEFAULT PRIVILEGES IN SCHEMA observation GRANT SELECT ON TABLES TO api_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA interpretation GRANT SELECT ON TABLES TO api_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA control GRANT SELECT ON TABLES TO api_ro;

-- Belt and suspenders: every transaction in an api_ro session is read-only
-- even if a query were ever built dynamically.
ALTER ROLE api_ro SET default_transaction_read_only = on;
