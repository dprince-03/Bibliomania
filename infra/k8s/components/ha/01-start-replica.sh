#!/bin/bash
# Replica, first boot only: follow the primary from the start of its
# binlog, positioned by GTID, then make this server read-only for good.
#
# Sourced by the mysql image's entrypoint (docker_process_sql is in scope —
# see 02-replication-user.sh). read_only is switched on HERE, persisted, not
# via --super-read-only on the command line: that flag also blocked the
# entrypoint's own first-boot setup (root password, these scripts), so the
# replica came up never having been told to replicate.
until mysql -h mysql -urepl -p"$MYSQL_REPLICATION_PASSWORD" --get-server-public-key -e 'SELECT 1' >/dev/null 2>&1; do
  echo "waiting for primary..."; sleep 3
done
docker_process_sql <<SQL
CHANGE REPLICATION SOURCE TO
  SOURCE_HOST='mysql', SOURCE_PORT=3306,
  SOURCE_USER='repl', SOURCE_PASSWORD='${MYSQL_REPLICATION_PASSWORD}',
  SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1,
  SOURCE_CONNECT_RETRY=5;
START REPLICA;
SET PERSIST read_only = ON;
SET PERSIST super_read_only = ON;
SQL
