ifneq (,$(wildcard ../../.env))
    include ../../.env
    export
endif

migration_dir=./migrations
goose_pkg=github.com/pressly/goose/v3/cmd/goose@v3.27.1

.PHONY: create
create:
	@echo "Creating Postgres migration"
	@go run $(goose_pkg) -dir $(migration_dir) create $(ARGS) sql

.PHONY: up
up:
	@echo "Running Postgres migrations"
	@go run $(goose_pkg) -dir $(migration_dir) postgres "host=$(DB_HOST) port=$(DB_PORT) user=$(DB_USER) password=$(DB_PASSWORD) dbname=$(DB_NAME) sslmode=$(DB_SSL_MODE)" up

.PHONY: down
down:
	@echo "Rolling back Postgres migrations"
	@go run $(goose_pkg) -dir $(migration_dir) postgres "host=$(DB_HOST) port=$(DB_PORT) user=$(DB_USER) password=$(DB_PASSWORD) dbname=$(DB_NAME) sslmode=$(DB_SSL_MODE)" down