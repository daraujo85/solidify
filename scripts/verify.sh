#!/bin/sh
# CI local do Solidify (SAI-002). Falha fechado: qualquer etapa não-zero aborta.
#
# Roda dentro do container do toolchain (`make verify`) ou nativo (NATIVE=1).
# Budgets vindos de ARCHITECTURE.md §37: startup < 300ms, binário do core
# folgado dentro do alvo de imagem <= 60 MB comprimida.
set -eu

STARTUP_BUDGET_MS=${STARTUP_BUDGET_MS:-300}
BIN_BUDGET_BYTES=${BIN_BUDGET_BYTES:-20971520} # 20 MiB
BIN=${BIN:-bin/solidify}
PKG=./cmd/solidify

step() { printf '\n=== %s\n' "$1"; }
skip() { printf '    SKIP: %s\n' "$1"; }

step 'gofmt'
unformatted=$(gofmt -l . || true)
if [ -n "$unformatted" ]; then
	printf 'arquivos fora do gofmt:\n%s\n' "$unformatted" >&2
	exit 1
fi
echo '    ok'

step 'go vet'
go vet ./...
echo '    ok'

step 'go test'
CGO_ENABLED=0 go test ./...

step 'go test -race'
# -race exige CGO e um C toolchain; ausente na imagem alpine enxuta.
if command -v gcc >/dev/null 2>&1 || command -v clang >/dev/null 2>&1; then
	CGO_ENABLED=1 go test -race ./...
else
	skip 'sem C toolchain (-race indisponível nesta plataforma)'
fi

step 'go test -bench'
CGO_ENABLED=0 go test -bench=. -benchmem -run '^$' ./...

step 'build'
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "$BIN" "$PKG"

step 'tamanho do binário'
size=$(wc -c < "$BIN" | tr -d ' ')
printf '    %s bytes (budget %s)\n' "$size" "$BIN_BUDGET_BYTES"
if [ "$size" -gt "$BIN_BUDGET_BYTES" ]; then
	echo "binário acima do budget; explique o crescimento ou ajuste BIN_BUDGET_BYTES" >&2
	exit 1
fi

step 'startup'
# Mede o comando mais barato disponível; sem daemon, sem estado.
start=$(date +%s%N 2>/dev/null || echo '')
"./$BIN" version >/dev/null
end=$(date +%s%N 2>/dev/null || echo '')
if [ -n "$start" ] && [ -n "$end" ] && [ "$start" -ne 0 ] 2>/dev/null; then
	ms=$(( (end - start) / 1000000 ))
	printf '    %s ms (budget %s ms)\n' "$ms" "$STARTUP_BUDGET_MS"
	if [ "$ms" -gt "$STARTUP_BUDGET_MS" ]; then
		echo 'startup acima do budget' >&2
		exit 1
	fi
else
	skip 'date +%s%N indisponível; startup não medido'
fi

printf '\nverify: OK\n'
