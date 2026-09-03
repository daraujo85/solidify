# Solidify — Plano Task-Driven de Implementação

## Regras de execução para o agente

1. Executar tarefas em ordem dentro de cada fase.
2. Não implementar tarefa futura “aproveitando que está no arquivo”.
3. Cada tarefa termina com testes verdes e commit próprio quando possível.
4. Nenhuma mudança arquitetural silenciosa; divergências exigem ADR.
5. Não adicionar dependência sem justificar tamanho, manutenção e alternativa stdlib.
6. Manter `CGO_ENABLED=0` para o core.
7. Medir startup/binário/RSS em checkpoints definidos.
8. Todo parser externo precisa de fixture JSON real/sintética e golden test.
9. Toda feature que toca secrets precisa de teste de redaction.
10. Perfil `contractual` deve falhar fechado quando faltar requisito obrigatório.

---

# Fase 0 — Bootstrap e guardrails

## SAI-001 — Inicializar módulo Go e estrutura mínima

**Objetivo:** criar projeto compilável conforme `ARCHITECTURE.md`.

**Implementar:**

- `go.mod` com Go 1.27;
- `cmd/solidify/main.go`;
- diretórios internos essenciais;
- package `version`;
- `solidify version`;
- Makefile mínimo: `build`, `test`, `lint`, `bench`.

**Aceite:**

- `go test ./...` passa;
- `CGO_ENABLED=0 go build ./cmd/solidify` passa;
- nenhuma dependência externa ainda.

## SAI-002 — CI local e quality baseline do próprio Solidify

**Implementar:**

- `go test -race` quando plataforma suportar;
- `go vet`;
- `go test -bench` pacote benchmark;
- verificação de `gofmt`;
- medição de tamanho do binário.

**Aceite:** script único `make verify` retorna não-zero em falha.

## SAI-003 — Logging estruturado e erros tipados

**Implementar:**

- logger em stderr;
- níveis;
- JSON opcional;
- error codes estáveis para CLI;
- nenhum log de secret.

**Testes:** golden de erro humano e JSON.

## SAI-004 — Config loader JSON

**Implementar:**

- `solidify.json`;
- defaults;
- validação;
- config hash;
- referências `*_env` para secrets;
- override por flags.

**Não implementar:** YAML/TOML.

**Aceite:** config inválida mostra path/campo; secret value nunca é serializado.

## SAI-005 — `solidify init`

**Implementar:**

- detectar repo Git;
- criar `.solidify` e config inicial;
- não sobrescrever por padrão;
- saída `--non-interactive`.

**Aceite:** rodar duas vezes é idempotente.

---

# Fase 1 — Git scope e inventário

## SAI-006 — Resolver base/head/commit

**Implementar:**

- refs → SHA;
- commit isolado → parent..commit;
- merge-base opcional configurável;
- validar que objetos existem.

**Testes:** repo temporário com branch divergente.

## SAI-007 — Changed files robusto

**Implementar:**

- added/modified/deleted/renamed/copied;
- numstat;
- binary;
- mode change;
- `-M/-C`.

**Aceite:** fixture com rename não aparece como delete+add quando Git reconhece rename.

## SAI-008 — Diff hunks before/after

**Implementar:**

- hunks com line ranges;
- conteúdo limitado;
- refs estáveis;
- hash por hunk;
- acesso a `git show base:path` e `head:path`.

**Performance:** streaming; não duplicar diff inteiro em estruturas redundantes.

## SAI-009 — Git log do range

**Implementar:** metadados, Conventional Commits, breaking markers, issue key regex configurável.

**Testes:** feat/fix/refactor/perf/security/chore/breaking.

## SAI-010 — Release classification determinística

**Implementar:** classificação de commits + path hints, sem IA.

**Aceite:** categorias canônicas de `CONTRACTS.md`.

---

# Fase 2 — Detector de componentes/stack

## SAI-011 — Manifest discovery bounded

**Implementar:** localizar manifests apenas nos ancestrais/diretórios relacionados aos changed files e roots configuradas.

**Não fazer:** walk integral de monorepo enorme sem necessidade.

## SAI-012 — Stack detector

**Implementar:** Node, .NET, Java, PHP, Python, Go, Flutter/Dart, Rust, Ruby.

**Output:** stack + confidence + evidence.

## SAI-013 — Component classifier

**Implementar:** frontend-web/backend-api/worker/library/mobile/infra/database/unknown.

**Testes:** monorepo frontend + backend; commit backend-only.

## SAI-014 — Applicability engine v1

**Implementar:** regras para Sonar/tests/security/Lighthouse/ZAP/k6/migration/env.

**Aceite crítico:** backend-only => Lighthouse `NOT_APPLICABLE`.

---

# Fase 3 — Migrations e envs

## SAI-015 — Migration detector framework-aware

**Implementar:** Flyway, Liquibase, EF Core, Prisma, TypeORM, Sequelize, Knex, Django, Rails, Laravel + generic paths.

**Output:** normalized migration object.

## SAI-016 — SQL migration operation parser básico

**Implementar:** detectar CREATE/ALTER/DROP/ADD/RENAME/UPDATE/DELETE/INDEX de modo conservador.

**Não fazer:** tentar implementar parser SQL universal.

**Aceite:** se ambíguo, `unknown_operation` em vez de inventar.

## SAI-017 — Migration risk engine

**Implementar:** regras de risco descritas em `ARCHITECTURE.md`.

**Testes:** DROP column, NOT NULL sem default, index, backfill.

## SAI-018 — Env usage detector multi-stack

**Implementar:** padrões listados na arquitetura.

**Aceite:** detectar nomes sem carregar valores do ambiente.

## SAI-019 — Env base-vs-head comparator

**Implementar:** added/removed/changed-use/documented/missing-doc/default/required inference.

## SAI-020 — Secret redaction framework

**Implementar antes de qualquer IA:**

- redaction de valores de env;
- auth headers;
- tokens comuns;
- config paths secret-like;
- sanitizer de logs/prompts.

**Aceite crítico:** fixture com `PAYMENT_TOKEN=abc123` nunca grava `abc123` em artifacts/logs.

---

# Fase 4 — Evidence Bundle e storage

## SAI-021 — Artifact store filesystem

**Implementar:** run dir, atomic writes, SHA-256, manifest de artifacts.

## SAI-022 — SQLite storage

**Dependência aprovada:** `modernc.org/sqlite`.

**Implementar:** schema/migrations internas, `runs`, `components`, `analyzer_results`, `reviews`, `models`, `findings`, `artifacts`.

**Aceite:** DB é criado sem processo externo; `CGO_ENABLED=0` continua funcionando.

## SAI-023 — Evidence manifest

**Implementar:** refs para Git, changes, components, migrations, envs e limitações.

## SAI-024 — Evidence budget e truncation

**Implementar:** char/line budgets, priorities, reason-for-context, truncation metadata.

**Teste:** diff artificial >10MB não explode memória.

## SAI-025 — Sharding v1

**Implementar:** component → change clusters → shards estáveis.

**Aceite:** Peer A/B receberão mesmas shard IDs e hashes.

---

# Fase 5 — Analyzer framework e scheduler

## SAI-026 — Interface Analyzer

**Implementar:** `Detect`, `Run`, normalized result, version metadata.

## SAI-027 — Scheduler resource-aware

**Implementar:** classes light/cpu/browser/memory-heavy/active-network; concurrency limits.

**Aceite:** dois browser jobs nunca rodam simultaneamente por default.

## SAI-028 — External process runner seguro

**Implementar:** timeout, context cancel, bounded stdout/stderr, exit code, env allowlist, cwd restrito.

**Segurança:** nunca construir shell string com commit message/path não escapado.

## SAI-029 — Cache engine

**Implementar:** key por version/config/base/head/input hash; invalidation correta.

---

# Fase 6 — Test runner

## SAI-030 — Test command discovery

**Implementar:** scripts/manifests por stack + override config.

## SAI-031 — Generic test executor

**Implementar:** command, exit, duration, pass/fail; normalize JUnit/TRX quando arquivo fornecido.

## SAI-032 — Coverage importer

**Implementar inicialmente:** LCOV, Cobertura XML e formatos simples configuráveis.

**Não exigir coverage para todos os projetos.**

---

# Fase 7 — Sonar integration

## SAI-033 — Sonar applicability/config discovery

Detectar `sonar-project.properties`, config Solidify e env refs.

## SAI-034 — Sonar Web API adapter

**Implementar:** client isolado, auth por env, timeout, JSON parser, API-version tolerance.

## SAI-035 — Sonar result normalizer

Normalizar issues, severities, measures, quality gate e indicar project-wide vs diff-relevant.

## SAI-036 — Sonar scanner runner opcional

Rodar scanner em container/perfil quando configurado. Não subir Sonar Server automaticamente.

---

# Fase 8 — Segurança leve e sob demanda

## SAI-037 — Secret scanner adapter

Preferência Gitleaks; rodar no range/repo conforme capacidade da ferramenta; JSON parser.

## SAI-038 — OSV-Scanner adapter

Detectar lockfiles/SBOM relevantes, chamar scanner e normalizar vulnerabilidades.

## SAI-039 — Semgrep CE adapter

Docker/perfil sob demanda; ruleset configurável; JSON/SARIF; scope prioriza componentes alterados.

## SAI-040 — Security score/gates

Severity normalization, hard gates, waiver support.

## SAI-041 — ZAP Baseline adapter

Target allowlist, passive scan, timeout, normalized output.

## SAI-042 — ZAP API/active adapter

**Obrigatório:** `active_scan_enabled=true`, target class local/test e allowlist.

Suportar OpenAPI/Swagger/GraphQL quando configurado/detectado.

**Teste de segurança:** produção não pode ser alvo por descoberta automática.

---

# Fase 9 — Lighthouse

## SAI-043 — Frontend target resolver

Resolver URL via config/runtime hooks; healthcheck.

## SAI-044 — Lighthouse adapter

Rodar container/browser profile; coletar JSON; importar scores oficiais.

**Aceite:** não é sequer invocado em backend-only fixture.

## SAI-045 — Frontend pillar score

Compor performance/accessibility/best-practices conforme config; SEO informativo default.

---

# Fase 10 — k6/load

## SAI-046 — Endpoint change detector v1

Fontes: OpenAPI diff, route/controller patterns, config manual.

## SAI-047 — k6 adapter

Priorizar script existente; fallback a smoke script explicitamente configurado.

## SAI-048 — Load result normalization

p50/p90/p95/p99, errors, throughput, thresholds.

## SAI-049 — Baseline comparator

Comparar apenas runs compatíveis por script/config/target fingerprint. Sem baseline => não afirmar regressão.

---

# Fase 11 — MCP

## SAI-050 — Adicionar SDK MCP oficial Go

Pin de versão; transporte stdio.

## SAI-051 — `solidify_begin_review`

Criar/reusar run, preparar analyzers/evidence, retornar contract refs.

## SAI-052 — Evidence retrieval tools

Implementar manifest/diff chunk/context/symbol search/analyzer/release inventory.

**Budget:** requests grandes devem paginar/truncar.

## SAI-053 — `solidify_submit_peer_review`

Validar schema, actor metadata, evidence hash, atomic save.

## SAI-054 — Agent setup documentation/command

Gerar instruções para registrar MCP em Claude Code/Codex sem hardcode de home path destrutivo.

---

# Fase 12 — Peer review schemas e prompt

## SAI-055 — JSON Schema do Peer Review

Implementar validator e fixtures válidas/inválidas.

## SAI-056 — Prompt Peer v1

Usar `prompts/peer-review.md`, placeholders estruturados, prompt hash/version.

## SAI-057 — Peer A ingestion tests

Casos: SRP, OCP, LSP N/A, ISP, DIP.

---

# Fase 13 — Provider OpenAI-compatible e 9Router

## SAI-058 — Provider interface

ListModels + CompleteJSON + metadata.

## SAI-059 — OpenAI-compatible HTTP adapter

Sem SDK pesado; usar `net/http` e structs mínimas necessárias.

**Razão:** endpoint é simples; evita dependência de SDK de fornecedor.

## SAI-060 — 9Router preset/network handling

Host vs Docker base URL; `host.docker.internal`; `host-gateway` example.

## SAI-061 — Model discovery

`GET /v1/models`, cache SQLite, normalized IDs, expiry.

## SAI-062 — Capability probe

JSON compliance, latency, availability; cache. Não ranquear “inteligência” por nome.

## SAI-063 — Model selector

Pinned/discover/hybrid; distinctness policy; selection reason.

---

# Fase 14 — Peer B independente

## SAI-064 — Peer B request builder

Receber exatamente evidence shards equivalentes ao Peer A; nunca incluir Peer A output.

## SAI-065 — Peer B executor

Nova request/context, structured output, timeout/retry/repair único.

## SAI-066 — Independence audit

Registrar model/provider/context metadata e `independence_degraded` quando aplicável.

---

# Fase 15 — Divergence e Árbitro C

## SAI-067 — Divergence Engine determinístico

Score deltas, applicable mismatch, severity mismatch, finding overlap.

## SAI-068 — Arbiter prompt v1

Inputs: evidence + Peer A + Peer B + divergence map.

## SAI-069 — Arbiter executor

Nova request/context; modelo diferente por default contractual; structured verdict.

## SAI-070 — Arbitration merge

Accepted/rejected/merged findings; final S/O/L/I/D.

**Aceite:** árbitro não pode alterar resultado objetivo de teste/Sonar/ZAP.

---

# Fase 16 — Score, risk, confidence e gate

## SAI-071 — SOLID Score engine

Média das letras aplicáveis; before/after/delta; N/A correto.

## SAI-072 — Pillar score engine

Pesos e normalização sobre scored/applicable pillars.

## SAI-073 — Release Risk engine

Migration/env/breaking/security/blast-radius/review disagreement.

## SAI-074 — Confidence engine

Evidence completeness, peer agreement, independence, analyzer completeness, arbiter confidence.

## SAI-075 — Quality Gate

Perfis quick/release/contractual; PASS/WARN/FAIL/BLOCKED/INCOMPLETE.

**Golden tests:** release 95 + critical CVE => FAIL.

---

# Fase 17 — Release report canônico

## SAI-076 — JSON Schema do release report

Finalizar `schemas/release-report.schema.json` e validator.

## SAI-077 — Canonical report builder

Montar sem lógica de UI.

## SAI-078 — Artifact immutability/hash

Finalized report não é modificado; rerun cria novo run.

## SAI-079 — Executive summary input model

Gerar summary a partir de fatos/arbiter verdict. Se usar IA, deve ser etapa separada e nunca alterar scores.

---

# Fase 18 — Dashboard

## SAI-080 — Bootstrap Preact/Vite/TypeScript

Build estático; zero Node no runtime.

## SAI-081 — Design tokens e print-friendly layout

Sem Tailwind; CSS leve.

## SAI-082 — Overview/SOLID UI

Gate, quality, risk, confidence, S/O/L/I/D before→after.

## SAI-083 — Release/Migrations/Envs UI

Features/bugfixes, commits, migrations, deployment actions, env names only.

## SAI-084 — Quality/Security/Performance UI

Sonar/tests/security/k6/Lighthouse conditional rendering.

## SAI-085 — AI transparency UI

Peers, models, provider, prompt version, consensus, divergences, independence.

## SAI-086 — History UI

SQLite read-only endpoint local apenas para list/trends; detalhe de run continua vindo do JSON canônico.

## SAI-087 — `solidify dashboard`

`net/http`, localhost-only, embedded assets, graceful shutdown.

---

# Fase 19 — PDF

## SAI-088 — Print stylesheet

A4, page breaks, headers/footers, cards legíveis, sem scroll-only content.

## SAI-089 — Browser detector

Chrome/Chromium executável; version log.

## SAI-090 — Headless PDF command

Render do mesmo HTML/report; timeout; artifact hash.

## SAI-091 — Docker PDF fallback

Perfil `pdf` temporário; sem serviço persistente.

## SAI-092 — PDF contractual content validation

Teste automatizado via metadados/text extraction simples para garantir seções obrigatórias e ausência de secrets fixture.

---

# Fase 20 — Robustez por troca de papéis

## SAI-093 — External model role-swap runner

Y como Peer B, X como Arbiter, mantendo contexto isolado.

## SAI-094 — Robustness comparator

Score delta, letter deltas, finding overlap, gate stability.

## SAI-095 — Robustness report integration

Exibir stable/mostly-stable/unstable no JSON/dashboard/PDF quando executado.

---

# Fase 21 — Performance e hardening

## SAI-096 — Benchmark suite realista

Fixtures 1k files/10MB diff/monorepo.

## SAI-097 — Memory profiling

Identificar cópias de diff/artifacts; otimizar streaming/buffers.

## SAI-098 — Core image budget

Multi-stage Alpine; non-root; git; ca-certs; medir compressed size.

## SAI-099 — Tool resource limits

Compose profiles com limites configuráveis e heavy concurrency=1.

## SAI-100 — Cache verification

Re-run idêntico reaproveita analyzer results; mudança de tool/config invalida.

---

# Fase 22 — Segurança do core

## SAI-101 — Path traversal tests

MCP context tool não lê fora do repo/allowed roots.

## SAI-102 — Command injection tests

Paths/commit subjects maliciosos não entram em shell.

## SAI-103 — Secret corpus redaction tests

API keys, bearer, env values, connection strings, passwords.

## SAI-104 — Active target safety tests

ZAP active/k6 destructive profile não roda em host fora de allowlist.

---

# Fase 23 — E2E de aceitação

## SAI-105 — Fixture fullstack sample

Repo exemplo com frontend + backend + migration + env + testes.

## SAI-106 — E2E backend-only release

Esperado: Lighthouse N/A; backend analyzers aplicáveis; PDF correto.

## SAI-107 — E2E frontend-only release

Esperado: Lighthouse aplicável; k6 API N/A salvo config.

## SAI-108 — E2E contractual peer review

Mock OpenAI-compatible server com Peer B + Arbiter; report final peer-reviewed.

## SAI-109 — E2E 9Router adapter smoke

Teste manual/opt-in contra endpoint local; nunca requerido em CI pública.

## SAI-110 — E2E migration/env risk

Migration destrutiva + env required undocumented => Release Risk elevado e action item.

## SAI-111 — E2E hard gate

Security high/critical => FAIL mesmo com Quality Score alto.

---

# Fase 24 — Documentação de uso comunitário

## SAI-112 — README do projeto real

Quickstart Docker/native, exemplos Claude Code/Codex, config 9Router.

## SAI-113 — `solidify doctor`

Diagnóstico de Git/Docker/browser/Sonar/provider/tools sem executar análise pesada.

## SAI-114 — Example reports sanitizados

JSON + screenshots/PDF sample sem código proprietário.

## SAI-115 — ADR index

Registrar decisões tomadas durante implementação.

---

# Definition of Done do MVP

O MVP NÃO está pronto apenas porque “gera uma nota”. Está pronto quando:

- diff scope é correto;
- before/after é rastreável;
- migrations/envs são detectadas;
- nenhum secret fixture vaza;
- applicability evita tools irrelevantes;
- Peer A/B são cegos entre si;
- Arbiter é contexto isolado;
- modelo/papel/provider aparecem no report;
- JSON passa schema;
- dashboard e PDF vêm do mesmo JSON;
- contractual falha fechado;
- core termina sem daemon;
- benchmarks e image budget estão documentados;
- E2E fullstack demonstra o fluxo completo.

---

# Fase 25 — Resiliência operacional de IA

## SAI-127 — Provider Resilience, Model Failover and Partial Reports

Implementar classificação semântica de erros de provider, retry curto e failover limitado por capabilities e preferência configurada para Peer B e Arbiter. Registrar todas as tentativas sem secrets, distinguir `requested_model` de `executed_model`, recalcular independência com modelos efetivamente executados e garantir `release-report.json` após criação de `run_id`, inclusive em `INCOMPLETE`.

A interface externa usa `0=PASS`, `1=FAIL`, `2=INCOMPLETE`; o report preserva `reason_code` interno, incluindo `12` para INCOMPLETE e códigos existentes para diagnóstico. Peer A/MCP não recebe substituição silenciosa. ADR 0120 permanece CanonicalPayload v2; esta task é definida por ADR 0127.

**Subtasks:** SAI-127A — classificação; SAI-127B — failover; SAI-127C — partial report; SAI-127D — exit mapping e regressões.

**Não iniciar automaticamente:** SAI-128.

---
