# pumpy

`pumpy` ingests Solana wallet activity, stores relationship data, and powers graph/dashboard views for wallet analysis.

## Components

- `cmd/pumpy-crawl`: crawls Dune SVM transactions for configured seed wallets.
- `cmd/pumpy-ingest`: ingests events from the Portal source.
- `cmd/pumpy`: utility commands for stats and wallet whois lookups.
- `internal/dashboard`: renders the dashboard-oriented graph and summary views.

## Local development

### Prerequisites

- Go 1.24+
- Docker (optional, for local services via `docker-compose`)

### Common commands

- Run unit tests:

  ```bash
  go test ./...
  ```

- Run dashboard validation script:

  ```bash
  ./scripts/validate-dashboard.sh
  ```

- Build binaries:

  ```bash
  go build ./cmd/...
  ```

## Dune transaction normalization notes

Dune responses can encode `block_time` in different unix precisions depending on endpoint/runtime behavior. The normalizer in `internal/dune/normalize.go` accepts seconds, milliseconds, microseconds, and nanoseconds and converts all of them to epoch seconds before downstream processing.
