GO=/usr/local/go/bin/go
DB_URL?=postgres://pumpy:pumpy@localhost:5432/pumpy?sslmode=disable

.PHONY: build tidy migrate run top stop

build: tidy
	$(GO) build -o bin/pumpy-ingest ./cmd/pumpy-ingest
	$(GO) build -o bin/pumpy        ./cmd/pumpy

tidy:
	$(GO) mod tidy

migrate:
	psql $(DB_URL) -f migrations/001_init.sql

run:
	docker compose up -d

stop:
	docker compose down

top:
	DATABASE_URL=$(DB_URL) bin/pumpy top --window $(WINDOW)
