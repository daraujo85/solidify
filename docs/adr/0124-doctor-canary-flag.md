# ADR 0124 — `solidify doctor --canary` (SAI-124)

Status: Aceito. 2026-08-21.

## Contexto

SAI-123 (ADR-0123) adicionou 2 checks canários ao
doctor: `peer_review_canary` (v1_fraction < 5% por
30d) e `peer_review_store_v1` (records v1 no store).
Eles são úteis para acompanhar rollout v2 mas são
"ponto na agenda" — quem rodar `doctor` no CI vê,
mas operador que **só** quer olhar rollout v1 não
precisa do resto.

Pergunta concreta: **como operator filtra só os
checks de rollout v2 sem digitar `--only` 2 vezes?**

```bash
# antes (SAI-123): verbose, fácil esquecer um
solidify doctor --only=peer_review_canary
solidify doctor --only=peer_review_store_v1

# agora (SAI-124): um atalho
solidify doctor --canary
```

Atalho também ajuda em **CI agendado** que monitora
rollout v2: script único roda o flag, sem lista
hardcoded de nomes de checks (que podem crescer em
SAI-125+).

## Decisão

### Flag `--canary`

Nova flag em `runDoctor` (`internal/app/app.go`):

```go
canary := fs.Bool("canary", false, "roda só checks canário v1 (peer_review_canary + peer_review_store_v1) — SAI-124")
```

Quando setada, executa exatamente:

```go
results = []doctor.CheckResult{
    doctor.RunCheck("peer_review_canary"),
    doctor.RunCheck("peer_review_store_v1"),
}
```

Mesma tabela human / `--json` do `RunAll`. Default
(sem flag) = comportamento atual inalterado.

### Mutuamente exclusivo com `--only`

```bash
solidify doctor --canary --only=peer_review_canary
# → exit 2 + "erro [usage]: --canary e --only são mutuamente exclusivos"
```

`--canary` é atalho pra "rode exatamente esses 2";
combinar com `--only` é ambíguo (qual ganha?). Falha
cedo com `CodeUsage` → stderr recebe usage também
(regras padrão de `app.Run`).

### Lista de checks canários é centralizada

Os 2 nomes vivem no **switch do `runDoctor`**,
não na lista `AllChecks`. `AllChecks` continua
incluindo os 2 checks (visíveis no `doctor` sem
flags). `--canary` é uma *view* sobre eles.

Razão: se SAI-125+ adicionar `peer_review_canary_v2`
(métrica diferente), `--canary` precisa ser
atualizado no switch — mudança consciente em vez de
"todo check com `peer_review` no nome entra".

## Não-objetivos

- Lista configurável de checks canários via config
  ou flag — SAI-124 cobre o caso concreto v2. Se
  aparecer outro rollout com checks próprios, vira
  ADR nova (`--canary=foo` ou `--rollout=v2`).
- Auto-aplicar `--canary` por default em algum
  contexto (ex: CI noturno) — opt-in continua
  sendo decisão do operator.
- Output específico pra canário (ex: dashboard
  widget) — fora de escopo; SAI-125+ se virar.

## Consequências

- **Atalho curto**: 1 flag cobre 2 checks. CI
  scripts ficam menores.
- **Falha explícita**: `--canary` + `--only` = erro
  de usage, não "último ganha".
- **Mensagem descritiva**: `--canary` aparece em
  `solidify doctor --help` com lista dos checks
  que roda — operator sabe o que esperar sem rodar.
- **Reuso de checks existentes**: zero código novo
  em `internal/doctor/` — SAI-124 é puramente
  apresentação CLI.
- **Evolução consciente**: novos checks canários
  entram no switch de `runDoctor`, forçando review.

## Verificação

- `go build ./...` → 0 erros
- `internal/app/app_test.go` 2 testes novos:
  - `TestDoctorCanaryMutuallyExclusiveWithOnly` —
    `--canary --only=X` → exit != 0 + stderr contém
    "mutuamente exclusivos" + usage no stderr
  - `TestDoctorCanaryRunsOnlyV1Checks` —
    `--canary` → exit 0 + output contém os 2 checks
    + **não** contém docker/disk_space/9router
- Smoke CLI:
  - `solidify doctor --canary` (sem dados) →
    ```
    OK    peer_review_canary  sem submissões registradas
    OK    peer_review_store_v1  store vazio
    2 checks: 2 OK, 0 FAIL
    ```
  - `solidify doctor --canary --only=peer_review_canary` →
    exit 2, usage no stderr
- Regression: `solidify doctor` (sem flags) →
  comportamento SAI-123 intacto.

## Próximo passo

1. ~~SAI-125: dashboard widget mostra v1_fraction
   trend over time.~~ Diferido — depende de
   persistir histórico além do JSONL append-only.
2. SAI-126: schema "3" com `detailed_findings` por
   pilar SOLID — bump após field usage confirmar
   demanda (ver ADR-0121/0123).
3. Se operator quiser filtro composto
   (`--canary=peer_a_v2`), vira ADR nova — não
   antecipar generalização.