# Run from Server/ (matches every other command documented in CLAUDE.md).
#
# migrate-up/migrate-down source Server/.env at the shell level (not via
# Make's `include`) — Make's own comment character is '#', and this repo's
# DB_PASSWORD/REDIS_PASSWORD values contain a literal '#' (e.g.
# "@Dev_prince#2003"). `include .env` would silently truncate that value at
# the '#'; sourcing it as a shell script does not, since shell only treats
# '#' as a comment starter at the beginning of a word, not mid-token.

.PHONY: run build vet fmt migrate-up migrate-down docker-up docker-down swagger seed

run:
	go run ./cmd/api

build:
	go build -o bin/bibliotheca ./cmd/api

vet:
	go vet ./...

fmt:
	gofmt -l .

migrate-up:
	@set -a; . ./.env; set +a; \
	migrate -path ./migrations -database "mysql://$$DB_USER:$$DB_PASSWORD@tcp($$DB_HOST:$$DB_PORT)/$$DB_NAME" up

migrate-down:
	@set -a; . ./.env; set +a; \
	migrate -path ./migrations -database "mysql://$$DB_USER:$$DB_PASSWORD@tcp($$DB_HOST:$$DB_PORT)/$$DB_NAME" down 1

# Whole dev stack (server, all clients, nginx, MySQL, Redis, Umami, dev
# tooling) — see infra/README.md for what's in it and the prod equivalent
# (docker-compose.prod.yml; not wrapped here since it's a deploy-host
# concern, not a local dev convenience).
docker-up:
	docker compose --env-file ../.env -f ../infra/docker/docker-compose.yml -f ../infra/docker/docker-compose.dev.yml up --build -d

docker-down:
	docker compose --env-file ../.env -f ../infra/docker/docker-compose.yml -f ../infra/docker/docker-compose.dev.yml down

# Regenerate internal/swaggerdocs/ after changing a handler's swaggo
# comments (Step 18) — see Server/docs/API.md.
swagger:
	swag init -g cmd/api/main.go --output internal/swaggerdocs --parseInternal --parseDependency
	swag fmt -g cmd/api/main.go

# Dev-only sample data — see cmd/seed/main.go for exactly what it inserts
# and how it avoids duplicating data on repeat runs.
seed:
	go run ./cmd/seed
