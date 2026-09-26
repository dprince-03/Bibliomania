# Server/app setup: env, databases, migrations, code generation

`Server/app` is **one Go module with eight binaries** — seven services and
the gateway — since the microservices split (see [`plan.md`](plan.md) →
"Microservices split"):

```text
cmd/
  gateway/        GraphQL + versioned REST façade (the only thing clients reach)
  auth/           MySQL         + RabbitMQ
  catalog/        Postgres      + Redis + RabbitMQ   (gRPC server)
  borrow/         Postgres      + Kafka              (gRPC server)
  reading/        MongoDB       + NATS JetStream     (gRPC server)
  user/           Postgres      + RabbitMQ + NATS    (gRPC server)
  notification/   (no database) consumes RabbitMQ + Kafka, sends email
  payment/        Postgres      + Kafka + Stripe
  seed/           dev sample data (make seed)
  key/            JWT secret generator
proto/<svc>/v1/   gRPC contracts          → gen/<svc>/v1/ (generated, committed)
internal/
  services/<svc>/ each service's model/dto/repository/service/handler/grpc + migrations/
  gateway/        REST proxy table, library aggregation, GraphQL (graph/)
  apiversion/     REST API versioning
  events/         event envelope + types, outbox, RabbitMQ/Kafka/NATS adapters
  grpcx/          gRPC server/client setup, JWT forwarding, error mapping
  telemetry/      OpenTelemetry traces, JSON logs, Prometheus metrics
  server/         process runner (HTTP + gRPC + workers, graceful shutdown)
  database/       embedded-migration runner (golang-migrate)
  config/ cache/ errors/ health/ middleware/ utils/ swaggerdocs/
pkg/
  postgresclient/ mysqlclient/ mongoclient/ redisclient/   one connector per store (retry while the DB starts, pool settings)
  sqlretry/       shared retry/pool logic for the two SQL clients
  jwt/ refreshToken/
```

## .env

`Server/app/.env` (copy `.env.example`) holds the settings **every** service
shares — above all `JWT_SECRET`, which must be identical everywhere since
each service verifies tokens itself. In Docker, each container also gets its
own connection settings (`DATABASE_URL`, broker URLs, gRPC addresses) from
`infra/docker/docker-compose.yml`, which override the file.

Per-service settings (the ones `cmd/<svc>/main.go` `Require`s):

| Var | Used by | Example |
| --- | --- | --- |
| `DATABASE_URL` | auth (MySQL DSN), catalog/borrow/user/payment (postgres URL) | `postgres://catalog:pw@postgres:5432/bibliomania_catalog?sslmode=disable` |
| `MONGO_URL`, `MONGO_DB` | reading | `mongodb://mongo:27017/?replicaSet=rs0` |
| `REDIS_*` | catalog; gateway (idempotency records + shared rate limits); notification (sent-log) | |
| `RABBITMQ_URL` | auth, catalog, user, notification | `amqp://user:pw@rabbitmq:5672/` |
| `KAFKA_BROKERS` | borrow, payment, notification | `kafka:9092` |
| `NATS_URL` | reading, user | `nats://nats:4222` |
| `CATALOG_GRPC_ADDR` (+ `BORROW_/READING_/USER_GRPC_ADDR` on the gateway) | borrow, reading, user, payment, gateway | `catalog:9090` |
| `*_HTTP_URL` | gateway (REST proxy targets) | `http://auth:8080` |
| `SERVER_PORT` / `GRPC_PORT` | all | `8080` / `9090` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | all (empty = don't export traces) | `tempo:4317` |
| `SMTP_HOST/PORT`, `RESEND_API_KEY`, `MAIL_FROM` | notification | |
| `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` | payment | empty = Stripe disabled |
| `PAYSTACK_SECRET_KEY`, `PAYSTACK_CURRENCIES`, `PAYSTACK_BASE_URL` | payment | empty key = Paystack disabled; currencies default `NGN,GHS,ZAR,KES` |
| `PAYMENTS_LIVE_ENABLED` | payment | live keys refused unless `true` (Step 38) |
| `API_DEPRECATED_VERSIONS` | gateway | `v1:2027-01-01:2027-07-01` |
| `OVERDUE_SWEEP_INTERVAL_SECONDS` | borrow | `300` |
| `READ_DATABASE_URL` | catalog (optional) | public reads go to Postgres replicas; empty = primary only. Set by the HA overlay (`pg-ro`) |
| `STORAGE_BACKEND` | catalog | `s3` (Compose/k8s: SeaweedFS) or `local` (bare `go run`, uses `STORAGE_PATH`) |
| `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_USE_SSL`, `S3_REGION` | catalog | `seaweedfs:8333` — "S3" is the protocol; the store is self-hosted SeaweedFS. `S3_REGION` is a protocol field SeaweedFS ignores |
| `MAX_UPLOAD_SIZE_MB` | catalog | `50` |
| `CLAMAV_ADDR` | catalog (optional) | `clamav:3310` — empty = no virus scan; set = scan and fail closed |
| `INTERNAL_API_TOKEN` | auth, user | shared secret for auth's internal accounts endpoint (never routed by the gateway); empty = reconciler off |
| `RECONCILE_INTERVAL_MINUTES` | user | `60` (first pass 30s after boot; `0` = off) |
| `KAFKA_REPLICATION_FACTOR` | borrow, payment | `1` (HA: `3`) |
| `NATS_STREAM_REPLICAS` | reading | `1` (HA: `3`) |
| `RABBITMQ_QUEUE_TYPE` | auth, catalog, user, notification | `classic` (HA: `quorum`) |
| `RATE_LIMIT_RPS`, `RATE_LIMIT_BURST` | gateway | `100` / `200` per client IP (shared across replicas via Redis) |

## Databases — one per service

No service can reach another's database; cross-service data goes over gRPC
or events. Foreign keys only exist *within* a service.

| Service | Engine | Database | Tables / collections |
| --- | --- | --- | --- |
| auth | MySQL 8.4 | `bibliomania_auth` | `accounts`, `refresh_tokens`, `outbox_events`, `processed_events` |
| catalog | Postgres 16 | `bibliomania_catalog` | `authors`, `books` (with a generated `search_vector` tsvector + pg_trgm indexes), `book_authors`, `copy_reservations`, outbox/processed |
| borrow | Postgres 16 | `bibliomania_borrow` | `borrow_records` (title + email snapshots, reservation key, idempotency key), `saga_compensations`, outbox/processed |
| user | Postgres 16 | `bibliomania_user` | `users` (replicated from auth), `users_profile`, `user_library`, outbox/processed |
| payment | Postgres 16 | `bibliomania_payment` | `book_purchases` (with `provider` + `provider_reference`), `webhook_events`, outbox/processed |
| reading | MongoDB 7 (replica set) | `bibliomania_reading` | `reading_sessions`, `bookmarks`, `counters`, `outbox_events` |

The four Postgres databases share one server, each with its own role. Server
setup is plain SQL in `infra/docker/`, run once on a volume's first boot:
`postgres/init/01-roles-and-databases.sql` (roles + databases, passwords
read from env via psql `\getenv`), `postgres/init/02-extensions.sql`
(`pg_stat_statements`, `pg_trgm`), and `mysql/init/01-server-settings.sql`
(slow-query log, statement digests, strict `sql_mode`). The same files are
mounted by the Kubernetes setup (`infra/k8s`). user-service's role is
`users`, since `user` is reserved in Postgres. Tables are never created by
these files, only by each service's migrations.

## Migrations

Each SQL service embeds its own migrations
(`internal/services/<svc>/migrations/`, via `embed.FS`) and applies them on
boot — no migrations folder ships next to the binary, and there's no
shared `migrations/` directory any more. reading-service creates its
MongoDB indexes on boot instead (`reading.EnsureIndexes`).

New migration for a service:

```bash
migrate create -ext sql -dir internal/services/catalog/migrations -seq add_something
```

Apply by hand (normally unnecessary — the service does it on start):

```bash
migrate -path internal/services/catalog/migrations \
  -database "postgres://catalog:pw@localhost:9091/bibliomania_catalog?sslmode=disable" up
```

The pre-split monolith's MySQL schema (`migrations/000001…000011`, one
`bibliomania` database) is gone; its data is **not** migrated — see
`plan.md` → "Still open". `000011`'s lesson (sub-second precision on the
last-write-wins clock) carried over: reading-service stores that clock as
integer nanoseconds.

## Code generation tools

| What | Command | Needs |
| --- | --- | --- |
| gRPC (`proto/` → `gen/`) | `make proto` | `buf`, `protoc-gen-go`, `protoc-gen-go-grpc`: `go install github.com/bufbuild/buf/cmd/buf@v1.57.2 google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.9 google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1` |
| GraphQL (`schema.graphqls` → `graph/generated.go`) | `make graphql` | nothing extra — gqlgen is a `tool` in `go.mod` |
| Swagger | `make swagger` | `go install github.com/swaggo/swag/cmd/swag@v1.16.6` |

All generated code is committed. CI checks the GraphQL output is current
and lints protos (plus `buf breaking` against `main`).
