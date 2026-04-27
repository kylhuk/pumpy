GO=/usr/local/go/bin/go
DB_URL?=postgres://pumpy:pumpy@localhost:5432/pumpy?sslmode=disable

.PHONY: build tidy migrate run stop dashboard-seed top

build: tidy
	$(GO) build -o bin/pumpy-ingest ./cmd/pumpy-ingest
	$(GO) build -o bin/pumpy        ./cmd/pumpy
	$(GO) build -o bin/pumpy-crawl  ./cmd/pumpy-crawl

tidy:
	$(GO) mod tidy

migrate:
	psql $(DB_URL) -f migrations/001_init.sql

run:
	docker compose up -d

stop:
	docker compose down

dashboard-seed: ## Seed the NeoDash dashboard into Neo4j (run after neo4j is up)
	@printf "MERGE (d:_Neodash_Dashboard {uuid: 'wallet-graph-v1'}) SET d.title='Wallet Graph', d.version='2.4', d.user='neo4j', d.content=%s, d.date=datetime()\n" \
	  "$$(cat dashboards/wallet-graph.json | jq -Rs '.')" | \
	docker compose exec -T neo4j cypher-shell \
	  -u neo4j \
	  -p "$${NEO4J_PASSWORD:-pumpypumpy}"
	@echo "Dashboard seeded. Open http://localhost:5005"

top:
	DATABASE_URL=$(DB_URL) bin/pumpy top --window $(WINDOW)
