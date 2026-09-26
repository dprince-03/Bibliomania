# Run from Server/app/ (matches every other command documented in CLAUDE.md).
#
# Server/app is one Go module with eight binaries (cmd/<service>): seven
# services + the gateway. See docs/plan.md → "Microservices split".
#
# .env files are loaded as data by infra/backup/load-env.sh — not via
# Make's `include` (Make's comment character is '#', and password values
# can contain one) and not by sourcing them as shell (values like
# MAIL_FROM=Name <addr> are shell redirects).

# load-env.sh (used by `seed`) needs bash, not the /bin/sh default.
SHELL := /bin/bash

SERVICES := gateway auth catalog borrow reading user notification payment
SERVICE ?= gateway
COMPOSE := docker compose --env-file ../../infra/docker/.env -f ../../infra/docker/docker-compose.yml -f ../../infra/docker/docker-compose.dev.yml

.PHONY: run build vet fmt test proto graphql swagger generate seed docker-up docker-down docker-logs

# Run one service locally, outside Docker: `make run SERVICE=catalog`.
# It still needs its database/broker — easiest with the stack's infra up.
run:
	go run ./cmd/$(SERVICE)

build:
	@for s in $(SERVICES); do echo "build $$s"; go build -o bin/$$s ./cmd/$$s || exit 1; done

vet:
	go vet ./...

fmt:
	gofmt -l $$(find . -name '*.go' -not -path './gen/*' -not -name 'generated.go' -not -name 'models_gen.go')

test:
	go test ./...

# Regenerate gRPC code in gen/ from proto/ (needs buf, protoc-gen-go,
# protoc-gen-go-grpc — see docs/project_setup.md).
proto:
	buf lint
	buf generate

# Regenerate the GraphQL gateway after editing
# internal/gateway/graph/schema.graphqls.
graphql:
	go tool gqlgen generate

# Regenerate internal/swaggerdocs/ after changing any handler's swaggo
# comments — covers every service (the gateway serves the combined spec).
swagger:
	swag init -g cmd/gateway/main.go --output internal/swaggerdocs --parseInternal --parseDependency
	swag fmt -g cmd/gateway/main.go

generate: proto graphql swagger

# Dev-only sample data, against the running dev stack (`make docker-up`
# first) — see cmd/seed/main.go.
seed:
	@. ../../infra/backup/load-env.sh; \
	env_file=../../infra/docker/.env; [ -f $$env_file ] || env_file=../../infra/docker/.env.example; \
	load_env $$env_file; \
	SEED_AUTH_DATABASE_URL="$${AUTH_DB_USER:-auth}:$${AUTH_DB_PASSWORD:-changeme}@tcp(localhost:$${DB_PORT:-9086})/$${AUTH_DB_NAME:-bibliomania_auth}" \
	SEED_GATEWAY_URL="http://localhost:$${SERVER_HOST_PORT:-9081}" \
	go run ./cmd/seed

# Whole dev stack (services, clients, databases, brokers, observability,
# nginx, dev tooling) — see infra/README.md.
docker-up:
	$(COMPOSE) up --build -d

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f $(SERVICES)
