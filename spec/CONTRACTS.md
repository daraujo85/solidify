# Solidify — Contratos de CLI, MCP, IA, Scoring e Artefatos

## 1. Convenções gerais

- IDs de execução: UUIDv7/identificador temporal ordenável.
- Timestamps: RFC 3339 UTC.
- Scores: `number` 0–100, sem arredondar internamente.
- Confidence: `number` 0–1.
- Paths em artefatos: relativos ao repo quando possível.
- Hash: SHA-256 em hexadecimal.
- JSON canônico: UTF-8, chaves estáveis nos golden tests.
- Schema version: SemVer independente da versão do binário.

---

# 2. CLI

## 2.1 `solidify init`

Cria:

```text
.solidify/
solidify.json
```

Não sobrescrever config existente sem `--force`.

Saída humana:

- repo detectado;
- componentes detectados;
- ferramentas encontradas;
- sugestões de targets/runtime hooks;
- provider de IA detectado, se houver;
- avisos de segurança.

Opções:

```text
--profile quick|release|contractual
--non-interactive
--force
```

## 2.2 `solidify inspect`

Executa apenas descoberta/diff/evidence deterministicamente.

```bash
solidify inspect --base origin/main --head HEAD
```

Opções:

```text
--base <git-ref>
--head <git-ref>
--commit <sha>          # equivale parent..commit
--output json|text
--no-cache
```

## 2.3 `solidify analyze`

Executa analyzers objetivos aplicáveis, mas não inventa Peer A.

```bash
solidify analyze --base origin/main --head HEAD --profile release
```

Se perfil exigir Peer A e ele não foi submetido, status final do run fica `AWAITING_PEER_A`.

## 2.4 `solidify dashboard`

```bash
solidify dashboard --run latest
```

Opções:

```text
--run <id|latest>
--port 0
--no-open
```

Bind padrão: `127.0.0.1`.

## 2.5 `solidify report`

```bash
solidify report json --run latest
solidify report html --run latest
solidify report pdf --run latest
```

`json` sempre valida o schema antes de escrever.

## 2.6 `solidify models`

```bash
solidify models discover --provider judge
solidify models probe --provider judge
solidify models select --provider judge
```

Saída inclui apenas IDs/metadados; nunca API key.

## 2.7 `solidify robustness`

```bash
solidify robustness --run latest --swap-external-models
```

Requer dois modelos externos distintos ou retorna `BLOCKED`.

## 2.8 `solidify mcp --stdio`

Inicia servidor MCP em stdio, sem banner em stdout que quebre JSON-RPC. Logs vão para stderr.

---

# 3. MCP tools

Nomes usam namespace `solidify_` para evitar colisão.

## 3.1 `solidify_begin_review`

### Input

```json
{
  "base": "origin/main",
  "head": "HEAD",
  "profile": "contractual",
  "force_new_run": false
}
```

### Output

```json
{
  "run_id": "...",
  "status": "AWAITING_PEER_A",
  "base_sha": "...",
  "head_sha": "...",
  "evidence_manifest_ref": "evidence/manifest.json",
  "changed_components": [],
  "applicable_analyzers": [],
  "peer_contract": {
    "schema_version": "1.0.0",
    "prompt_version": "peer-review-v1"
  }
}
```

Semântica:

- coleta/recupera cache;
- roda analyzers conforme perfil;
- não chama Peer B antes de receber Peer A, salvo modo CLI específico futuro.

## 3.2 `solidify_get_evidence_manifest`

Input: `run_id`.

Retorna resumo estrutural e refs, não todo o código.

## 3.3 `solidify_get_diff_chunk`

Input:

```json
{
  "run_id": "...",
  "component_id": "backend",
  "file": "src/...",
  "chunk_id": "hunk-3",
  "view": "before_after"
}
```

Output limitado, com line numbers e hash.

## 3.4 `solidify_get_context`

Permite ao Peer A pedir contexto adicional, mas somente em paths dentro do repo e respeitando budget.

```json
{
  "run_id": "...",
  "path": "src/Foo.cs",
  "start_line": 1,
  "end_line": 180,
  "reason": "need interface contract for LSP evaluation"
}
```

A razão fica auditada.

## 3.5 `solidify_find_symbol_usages`

Busca bounded por símbolo/string no componente relevante.

Não implementa indexador semântico global no MVP. Pode usar busca textual otimizada e adapters por linguagem posteriormente.

## 3.6 `solidify_get_analyzer_result`

Input:

```json
{"run_id":"...","analyzer":"sonar"}
```

Retorna normalized result; logs brutos ficam por ref.

## 3.7 `solidify_get_release_inventory`

Retorna commits classificados, migrations, env changes, routes e components.

## 3.8 `solidify_submit_peer_review`

### Input

```json
{
  "run_id": "...",
  "actor": {
    "client": "claude-code",
    "model_id": "reported-model-id",
    "provider": "reported-provider",
    "identity_source": "client-reported"
  },
  "review": {}
}
```

`review` DEVE validar `schemas/peer-review.schema.json`.

### Side effects

1. salva Peer A;
2. valida evidence hash;
3. em perfil configurado, chama Peer B em contexto novo;
4. calcula divergências;
5. chama Árbitro C em contexto novo;
6. executa Score Engine/Gate;
7. gera `release-report.json`;
8. gera HTML; PDF se perfil exigir;
9. retorna status final e paths.

## 3.9 `solidify_get_final_report`

Retorna overview do JSON final e refs para artefatos.

---

# 4. Contrato do Peer A/Peer B

## 4.1 Invariantes

O peer DEVE:

- avaliar somente o evidence scope;
- citar evidence refs para toda penalização relevante;
- distinguir `N/A` de “bom”;
- avaliar before e after;
- não penalizar preferência estilística como violação SOLID;
- não premiar abstração desnecessária;
- não inventar testes/resultados;
- produzir JSON válido.

## 4.2 Finding SOLID

```json
{
  "id": "S-001",
  "principle": "S",
  "severity": "medium",
  "title": "Responsabilidades de persistência e notificação foram combinadas",
  "file": "src/BillingService.cs",
  "line_start": 72,
  "line_end": 118,
  "evidence_refs": ["git:hunk:...", "context:..."],
  "rationale": "...",
  "recommendation": "...",
  "introduced_by_release": true
}
```

Severity:

- info;
- low;
- medium;
- high;
- critical.

Critical em SOLID deve ser raro; representa quebra arquitetural de alto impacto, não “classe longa”.

## 4.3 Score por letra

```json
{
  "principle": "D",
  "applicable": true,
  "before_score": 86,
  "after_score": 62,
  "confidence": 0.91,
  "summary": "A aplicação passou a instanciar diretamente o adapter concreto.",
  "findings": ["D-001"]
}
```

Se não aplicável:

```json
{
  "principle": "L",
  "applicable": false,
  "before_score": null,
  "after_score": null,
  "confidence": 0.96,
  "summary": "O diff não altera hierarquias, implementações ou contratos substituíveis.",
  "findings": []
}
```

---

# 5. Divergence Engine

Antes do árbitro, o core calcula divergências sem LLM.

Por letra:

```text
abs(peerA.after - peerB.after)
```

Thresholds iniciais:

- <= 5: agreement;
- >5 e <=15: mild divergence;
- >15 e <=30: material divergence;
- >30: severe divergence.

Também comparar:

- applicable disagreement;
- severity mismatch;
- findings em mesmos arquivos/linhas;
- gate-affecting disagreement.

O árbitro recebe esse mapa.

---

# 6. Contrato do Árbitro C

O árbitro não deve tirar média cega.

Para cada princípio:

1. verificar se a evidência suporta A;
2. verificar se suporta B;
3. decidir aplicabilidade;
4. decidir score final;
5. registrar quais findings foram accepted/rejected/merged;
6. explicar divergência material.

Exemplo:

```json
{
  "principle": "O",
  "final_score": 74,
  "confidence": 0.88,
  "decision": "peer_b_preferred",
  "accepted_findings": ["B:O-002"],
  "rejected_findings": ["A:O-001"],
  "reason": "O switch citado por A existia no before state e não foi introduzido pela release."
}
```

O árbitro não pode eliminar finding objetivo de ferramenta externa. Ele arbitra análise IA/SOLID e classificação semântica; Sonar/ZAP/testes mantêm seus próprios fatos.

---

# 7. Provider OpenAI-compatible

## 7.1 Interface interna

```go
type ChatProvider interface {
    ListModels(ctx context.Context) ([]Model, error)
    CompleteJSON(ctx context.Context, req ReviewRequest) (ReviewResponse, error)
}
```

## 7.2 Config de segredo

Config salva:

```json
{
  "api_key_env": "SOLIDIFY_9ROUTER_API_KEY"
}
```

Nunca:

```json
{"api_key":"sk-..."}
```

## 7.3 9Router preset

Host binary:

```text
base_url = http://127.0.0.1:20128/v1
```

Core Docker:

```text
base_url = http://host.docker.internal:20128/v1
```

Discovery usa `/models`; completions usam `/chat/completions` inicialmente por compatibilidade ampla.

## 7.4 Request behavior

- timeout por role;
- retry apenas em falhas transitórias;
- no retry cego em 4xx de schema/model;
- streaming não é necessário para review formal;
- `response_format`/structured output usado quando endpoint/model suportar;
- caso contrário, JSON prompt + schema validation.

---

# 8. Model selection

Config conceitual:

```json
{
  "mode": "hybrid",
  "peer_b": {
    "preferred": ["provider/model-x"],
    "exclude": []
  },
  "arbiter": {
    "preferred": ["provider/model-y"],
    "must_differ_from_peer_b": true
  }
}
```

Discovery NÃO deve escolher modelo apenas por substring “reasoning”.

Selection order:

1. preferred disponível + probe OK;
2. candidate allowlisted + probe OK;
3. fallback disponível com JSON compliance;
4. BLOCKED se policy strict exigir modelo e nenhum candidato for válido.

Registrar `selection_reason`.

---

# 9. Analyzer normalized result

Todo analyzer converge para:

```json
{
  "id": "lighthouse",
  "version": "...",
  "status": "APPLICABLE",
  "execution_status": "PASSED",
  "scope": ["web-app"],
  "score": 91.2,
  "duration_ms": 12430,
  "findings": [],
  "metrics": {},
  "raw_artifact": "evidence/lighthouse.json",
  "limitations": []
}
```

`status` de aplicabilidade é separado de `execution_status`.

---

# 10. Git/release inventory contract

```json
{
  "range": {
    "base_ref": "origin/main",
    "base_sha": "...",
    "head_ref": "HEAD",
    "head_sha": "..."
  },
  "commits": [
    {
      "sha": "...",
      "subject": "fix(billing): prevent duplicate invoice",
      "type": "bugfix",
      "scope": "billing",
      "issues": ["ABC-123"],
      "breaking": false,
      "components": ["backend"]
    }
  ],
  "change_groups": []
}
```

---

# 11. Migration contract

```json
{
  "id": "V20260819_01",
  "path": "db/migration/V20260819_01__add_status.sql",
  "framework": "flyway",
  "status": "added",
  "operations": [
    {
      "kind": "add_column",
      "object": "invoice.status",
      "destructive": false
    }
  ],
  "rollback": {
    "present": false,
    "required_by_framework": false
  },
  "risk": "moderate",
  "evidence_refs": []
}
```

---

# 12. Env change contract

```json
{
  "name": "PAYMENT_TIMEOUT_MS",
  "status": "added",
  "components": ["backend"],
  "required": false,
  "has_default": true,
  "documented": true,
  "likely_secret": false,
  "references": [
    {"path":"src/config.ts","line":14}
  ],
  "deployment_action": "none-if-default-accepted"
}
```

Para secret-like env:

```json
{
  "name": "PAYMENT_API_TOKEN",
  "likely_secret": true,
  "value": "PROHIBITED"
}
```

O campo `value` não deve existir no schema real; exemplo acima é apenas uma proibição explícita.

---

# 13. Scoring contracts

## 13.1 SOLID score

Usar score do árbitro quando presente.

Fallback:

- Peer A + Peer B sem árbitro em perfil permissivo: média apenas se divergence <= threshold; caso contrário Confidence LOW e gate pode exigir revisão manual;
- somente Peer A: score permitido em `quick`, nunca classificado como peer-reviewed.

## 13.2 Static Quality

Normalizar Sonar/linters para 0–100 por política configurável.

Evitar algoritmo “mágico” baseado em quantidade absoluta sem considerar severidade e new-code scope.

Default sugerido:

```text
100
- critical issue: 30 cada
- high: 15 cada
- medium: 5 cada
- low: 1 cada
minimum 0
```

Quando Sonar fornece Quality Gate, gate original aparece separadamente.

## 13.3 Security

Severity penalties + hard gates. Não misturar CVSS de dependência e alerta de header como se fossem equivalentes; normalizar por categoria e depois compor.

## 13.4 Tests

- test failure: 0 + gate failure;
- all pass + coverage available: score pode considerar coverage delta;
- all pass sem coverage: score base configurável, ex. 85, sem fingir cobertura 100;
- no tests applicable: N/A;
- tests applicable skipped: SKIPPED e completeness penalty.

## 13.5 Lighthouse

Usar score composto configurável a partir das categorias oficiais, preservando as notas originais no report.

Default Solidify frontend pillar:

```text
Performance      50%
Accessibility    25%
Best Practices   25%
SEO              informational by default
```

## 13.6 Load

Score baseado em thresholds definidos, não em “quanto menor a latência, melhor” sem SLO.

Sem baseline/threshold: reportar métricas, mas pilar pode ficar `UNSCORED`.

---

# 14. Quality Score formula

Default weights:

```json
{
  "solid": 50,
  "static_quality": 15,
  "security": 15,
  "tests": 10,
  "performance": 5,
  "frontend": 5
}
```

Pseudo:

```text
weighted_sum = 0
applicable_weight = 0
for pillar in pillars:
  if pillar.scored:
    weighted_sum += pillar.score * pillar.weight
    applicable_weight += pillar.weight
quality = weighted_sum / applicable_weight
```

`SKIPPED` aplicável não deve ser convertido para N/A. Ele afeta completeness/gate conforme policy.

---

# 15. Release Risk

Risk factors não são simples score arithmetic.

Exemplos:

- destructive migration;
- breaking API change;
- nova env obrigatória sem documentação;
- auth/security-sensitive code;
- grande blast radius;
- muitos arquivos/shared core;
- load regression;
- review disagreement material;
- test coverage gap.

Engine determinístico produz base risk; árbitro pode explicar, mas não reduzir um hard risk sem waiver/evidence.

---

# 16. Waivers

Formato:

```json
{
  "id": "W-2026-001",
  "finding_ref": "security:OSV-...",
  "reason": "Dependency is not reachable in deployed component",
  "approved_by": "local-config/user",
  "expires_at": "2026-09-30T00:00:00Z"
}
```

MVP não precisa sistema de identidade. Apenas registrar que waiver foi declarado localmente. Não apagar finding; marcar waived.

---

# 17. Artefato canônico

`release-report.json` é imutável depois de finalizado. Para rerun, gerar novo `run_id`.

Conteúdo mínimo:

- identity;
- git range;
- components;
- release notes;
- migrations;
- env changes;
- analyzers;
- SOLID;
- AI review transparency;
- scores;
- risk;
- quality gate;
- recommendations;
- limitations;
- artifact hashes.

Schema em `schemas/release-report.schema.json`.

---

# 18. HTML contract

O HTML não lê SQLite para montar o conteúdo de uma release já finalizada. Ele recebe/serve `release-report.json`.

Isso permite:

- abrir artefato histórico;
- exportar pacote;
- evitar divergência entre DB e laudo.

History view pode consultar SQLite separadamente.

---

# 19. PDF contract

PDF contém no rodapé:

- run id;
- base short SHA;
- head short SHA;
- generated_at;
- page X/Y.

Anexo de IA deve mostrar:

| Papel | Modelo | Provider | Independente? |
|---|---|---|---|
| Peer A | ... | terminal | sim |
| Peer B | ... | 9Router/... | sim |
| Arbiter C | ... | 9Router/... | sim |

Também registrar prompt version e evidence hash em apêndice técnico.

---

# 20. Applicability matrix inicial

| Sinal da entrega | Sonar | Tests | Security SAST | Lighthouse | ZAP | k6 | Migration | Env |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| frontend web alterado | yes | yes | yes | yes | target-dependent | optional | detect | detect |
| backend API alterado | yes | yes | yes | no | target-dependent | target-dependent | detect | detect |
| library alterada | yes | yes | yes | no | no | no | detect | detect |
| docs only | optional/no | no | low/no | no | no | no | detect | detect |
| infra only | rules-dependent | tests-dependent | yes | no | no | no | detect | detect |

A matriz é heurística; config pode sobrescrever.

---

# 21. Failure semantics

## Tool not installed

- se não aplicável: N/A;
- aplicável em quick: SKIPPED + instruction;
- obrigatório em contractual: BLOCKED.

## Tool returns parser-unknown version

- salvar raw output;
- status FAILED/UNSUPPORTED;
- nunca estimar score.

## AI timeout

- retry transitório conforme policy;
- se Peer B/Arbiter obrigatório: BLOCKED;
- manter Peer A e evidências para diagnóstico.

## Invalid JSON from model

- single repair attempt;
- depois FAILED.

---

# 22. Prompt/version contract

Prompts ficam versionados como arquivos do repo e entram no hash da execução.

Mudança semântica em prompt:

- incrementa prompt version;
- deve ter regression fixtures;
- não reescreve reports antigos.

---

# 23. Golden test fixtures obrigatórias

Criar repos sintéticos/fixtures para:

1. SRP piora;
2. OCP melhora com Strategy/Adapter bem aplicado;
3. LSP N/A;
4. ISP violado;
5. DIP violado;
6. frontend-only → Lighthouse applicable;
7. backend-only → Lighthouse N/A;
8. migration destructive;
9. env secret-like adicionada sem valor exposto;
10. Conventional Commit release notes;
11. Peer A/B concordam;
12. Peer A/B divergem;
13. arbiter rejeita finding sem evidência;
14. invalid AI JSON;
15. 9Router model discovery fixture;
16. tool skipped vs N/A;
17. contractual blocked por missing arbiter;
18. role-swap stable/unstable.
