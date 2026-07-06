package runner

// pgtapSweepSQL drops any run schema left behind by a courseforge process
// that was killed mid-run, so a crash never leaves permanent clutter in the
// courseforge database. Shared between the Unix and Windows PostgresManager
// implementations.
const pgtapSweepSQL = `DO $$
DECLARE r record;
BEGIN
  FOR r IN SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'cf\_run\_%' LOOP
    EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(r.schema_name) || ' CASCADE';
  END LOOP;
END $$;`
