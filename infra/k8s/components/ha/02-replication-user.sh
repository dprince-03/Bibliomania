#!/bin/bash
# Primary, first boot only: the account replicas connect with. Not
# binlogged (SQL_LOG_BIN=0) — each server's accounts are its own.
#
# Sourced by the mysql image's entrypoint, so docker_process_sql is in scope:
# it knows the temporary init server's socket and the root password. A bare
# `mysql` here fails — the client's default socket path isn't the one the
# init server listens on.
docker_process_sql <<SQL
SET SQL_LOG_BIN=0;
CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED BY '${MYSQL_REPLICATION_PASSWORD}';
GRANT REPLICATION SLAVE ON *.* TO 'repl'@'%';
SET SQL_LOG_BIN=1;
SQL
