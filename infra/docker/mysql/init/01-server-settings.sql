-- Runs once, on the mysql container's first boot (empty data volume), via
-- /docker-entrypoint-initdb.d — after the entrypoint has created
-- AUTH_DB_NAME and AUTH_DB_USER from the environment. Also mounted by the
-- Kubernetes setup (infra/k8s).
--
-- auth-service's tables are NOT created here: the service applies its own
-- embedded migrations on boot (Server/app/internal/services/auth/migrations).
-- This file only holds server-level settings. SET PERSIST writes them to
-- mysqld-auto.cnf in the data volume, so they survive restarts.

-- Slow-query log: anything over 500ms, for catching bad queries the
-- Prometheus metrics alone won't point at.
SET PERSIST slow_query_log = ON;
SET PERSIST long_query_time = 0.5;
SET PERSIST log_queries_not_using_indexes = OFF;

-- Statement digests feed mysqld-exporter's per-query metrics.
UPDATE performance_schema.setup_consumers
   SET ENABLED = 'YES'
 WHERE NAME IN ('events_statements_current', 'events_statements_history', 'statements_digest');

-- Reject zero dates / silent truncation (the MySQL 8 default, pinned here
-- so a server-wide config change can't loosen it for auth's data).
SET PERSIST sql_mode = 'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION';
