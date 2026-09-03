# Toolchain roda em container por padrão (ver docs/adr/0001): o host não
# precisa de Go. Use NATIVE=1 para usar o `go` do host quando existir.
GO_IMAGE    ?= golang:1.26-alpine
# Imagem Debian tem C toolchain, necessário para -race (exige CGO).
GO_IMAGE_CGO ?= golang:1.26
MODCACHE    ?= solidify-gomodcache
NATIVE      ?= 0
CGO_ENABLED ?= 0
GOFLAGS     ?=
# Budgets de ARCHITECTURE.md §37; sobrescreva na linha de comando p/ testar o gate.
STARTUP_BUDGET_MS ?= 300
BIN_BUDGET_BYTES  ?= 20971520

DOCKER_RUN = docker run --rm \
  -v "$(CURDIR)":/src -w /src \
  -v $(MODCACHE):/go/pkg/mod \
  -e CGO_ENABLED=$(CGO_ENABLED) -e GOFLAGS='$(GOFLAGS)' \
  -e STARTUP_BUDGET_MS=$(STARTUP_BUDGET_MS) -e BIN_BUDGET_BYTES=$(BIN_BUDGET_BYTES)

ifeq ($(NATIVE),1)
GO_RUN     :=
GO_RUN_CGO :=
else
GO_RUN     := $(DOCKER_RUN) $(GO_IMAGE)
GO_RUN_CGO := $(DOCKER_RUN) $(GO_IMAGE_CGO)
endif

BIN         := bin/solidify-bin
PKG         := ./cmd/solidify
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
  -X github.com/diegoaraujo/solidify/internal/version.Version=$(VERSION) \
  -X github.com/diegoaraujo/solidify/internal/version.Commit=$(COMMIT) \
  -X github.com/diegoaraujo/solidify/internal/version.Date=$(DATE)

# Alvo único de CI local. Falha fechado: `set -e` no script.
# Roda na imagem com C toolchain para que -race seja exercido de fato.
.PHONY: verify
verify:
	$(GO_RUN_CGO) sh scripts/verify.sh

.PHONY: build test race vet fmt-check bench size lint clean sh
build:
	$(GO_RUN) go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

# test/bench usam a imagem CGO (com git + gcc): o resolver git shell-a git,
# e -race exige C toolchain. Build de produção continua no alpine.
test:
	$(GO_RUN_CGO) go test ./...

# -race exige CGO; roda apenas onde a plataforma suporta.
race:
	$(GO_RUN_CGO) env CGO_ENABLED=1 go test -race ./...

vet:
	$(GO_RUN) go vet ./...

fmt-check:
	$(GO_RUN) sh -c 'gofmt -l . | tee /dev/stderr | (! read)'

bench:
	$(GO_RUN_CGO) go test -bench=. -benchmem -run '^$$' ./...

size: build
	@wc -c < $(BIN) | awk '{printf "binário: %d bytes (%.2f MiB)\n", $$1, $$1/1048576}'

lint: fmt-check vet

# Shell dentro do container do toolchain, para depuração.
sh:
	docker run --rm -it -v "$(CURDIR)":/src -w /src -v $(MODCACHE):/go/pkg/mod $(GO_IMAGE) sh

INSTALL_DIR ?= $(HOME)/bin

# Builda binário macOS ARM64 e instala no PATH do host (sem Go no host).
# Sobrescreva INSTALL_DIR, ex: make install-host INSTALL_DIR=/usr/local/bin
.PHONY: install-host
install-host:
	$(DOCKER_RUN) -e GOOS=darwin -e GOARCH=arm64 $(GO_IMAGE) \
	  go build -trimpath -ldflags '$(LDFLAGS)' -o bin/solidify-darwin-arm64 $(PKG)
	mkdir -p $(INSTALL_DIR)
	install -m 755 bin/solidify-darwin-arm64 $(INSTALL_DIR)/solidify
	@echo "instalado: $(INSTALL_DIR)/solidify"
	@$(INSTALL_DIR)/solidify version 2>/dev/null || true

clean:
	rm -rf bin
