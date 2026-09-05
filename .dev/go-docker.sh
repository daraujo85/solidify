#!/bin/sh
# ponytail: Go não está instalado no host; roda tudo via docker+golang:1.26
# com cache persistido em volumes nomeados. Uso: .dev/go-docker.sh build ./...
cd "$(dirname "$0")/.." || exit 1
docker run --rm \
  -v "$(pwd)":/app \
  -v solidify-gocache:/root/.cache/go-build \
  -v solidify-gomod:/go \
  -w /app golang:1.26 go "$@"
