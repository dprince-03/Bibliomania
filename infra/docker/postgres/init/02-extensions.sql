-- Extensions, installed as the superuser at init so no service role needs
-- extension privileges. Runs after 01 (alphabetical order).

\set ON_ERROR_STOP on

-- Per-query statistics for slow-query hunting (postgres-exporter +
-- Grafana). The library itself is preloaded by the server command line
-- (shared_preload_libraries=pg_stat_statements in docker-compose.yml /
-- the k8s StatefulSet); this creates the view in the database the
-- exporter connects to.
\connect postgres
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- catalog-service's partial author/title search. Its own migration also
-- runs CREATE EXTENSION IF NOT EXISTS, which becomes a no-op.
\connect bibliomania_catalog
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
