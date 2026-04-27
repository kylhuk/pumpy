FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/pumpy-ingest ./cmd/pumpy-ingest
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/pumpy ./cmd/pumpy
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/pumpy-crawl ./cmd/pumpy-crawl

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin/pumpy-ingest /pumpy-ingest
COPY --from=builder /bin/pumpy /pumpy
COPY --from=builder /bin/pumpy-crawl /pumpy-crawl
ENTRYPOINT ["/pumpy-ingest"]
