# ADR 0024 — Test command discovery

Status: Aceito. 2026-08-20.

## Contexto

SAI-030: cada stack (node/python/go/java/rust/ruby/dotnet) tem
comando próprio pra rodar testes. Config override (`.solidify/test.json`)
deve ter prioridade. Default por stack se nada achado.

## Decisão

`testexec.Discoverer` faz 3 passes:

1. **Override**: `.solidify/test.json` (YAML não tem parser nativo
   no projeto; só JSON por enquanto — YAML como enhancement futuro).
2. **Manifest**: detecta package manager por lockfile (pnpm/yarn/npm);
   lê `package.json` scripts.test, `pyproject.toml`, `Rakefile`.
3. **Default**: comando canônico por stack (`go test ./...`, `pytest`,
   `cargo test`, etc).

Sources ordenados por priority (`override < manifest < default`).
Dedupe por `bin+args`.

`DetectStack` via markers: `go.mod`, `Cargo.toml`, `package.json`,
`pyproject.toml`, `pom.xml`, `Gemfile`, `*.csproj`, etc.

## Consequências

- 22 testes (stack detection, override priority, manifest, dedupe,
  package manager).
- Stack desconhecido + Strict=true → erro. Sem Strict → lista vazia.
- Override YAML inválido é ignorado (não falha discovery).
- Defaults cobrem 7 stacks; SAI-031 (generic executor) consome essa
  lista.

## Trade-offs

- YAML não tem parser nativo → só JSON em v1. Custo baixo pra
  maioria dos projetos; falha silenciosa se YAML inválido.
- Sem suporte a monorepo multi-stack por componente. v1 assume
  um stack por Discoverer; v2 pode somar.
