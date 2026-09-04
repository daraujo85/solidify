# Proposta — schema de política de gate em `solidify.json`

Resolve os 3 gaps 🔴 do `dashboard-gap-mapping.md` que dependem de política
configurável por repo: §4 (6 critérios nomeados do mockup), §4.1
(`limite_aprovacao` default, hoje hardcoded `DEFAULT_APPROVAL_LIMIT=80` em
`main.js`), §12 (`limite` por métrica de performance).

**Formato confirmado**: `internal/config/load.go` — `FileName = "solidify.json"`,
`decodeStrict` (campo desconhecido = erro). É JSON, não YAML. O enunciado da
tarefa citava `.solidify.yml` como exemplo do contrato de dados — mas o
arquivo real do produto é `solidify.json`. Este doc usa o nome real.

Não implementa nada. Proposta pra revisão humana antes de codar.

---

## 1. Exemplo concreto — `solidify.json`

```json
{
  "schema_version": "1.0.0",
  "project": { "name": "solidpay-api", "default_base": "origin/main" },

  "gate_policy": {
    "approval_threshold": 80,
    "criteria": [
      {
        "id": "score_minimo",
        "label": "Score mínimo",
        "source": "scores.quality",
        "operator": "gte",
        "value": 80,
        "blocking": true
      },
      {
        "id": "cobertura",
        "label": "Cobertura de testes",
        "source": "coverage.percent",
        "operator": "gte",
        "value": 80,
        "blocking": true
      },
      {
        "id": "vuln_criticas",
        "label": "Vulnerabilidades críticas",
        "source": "security.critical_count",
        "operator": "eq",
        "value": 0,
        "blocking": true
      },
      {
        "id": "novas_violacoes",
        "label": "Novas violações SOLID",
        "source": "solid.new_violations_count",
        "operator": "lte",
        "value": 3,
        "blocking": false
      },
      {
        "id": "migration_reversivel",
        "label": "Migration reversível",
        "source": "migrations.reversible",
        "operator": "eq",
        "value": true,
        "blocking": true
      },
      {
        "id": "testes_regressao",
        "label": "Testes de regressão",
        "source": "tests.all_green",
        "operator": "eq",
        "value": true,
        "blocking": true
      }
    ],
    "performance_limits": {
      "p95_ms": 400,
      "p99_ms": 1000,
      "error_rate": 0.01
    }
  }
}
```

Notas de design:

- `criteria[].source` é uma referência a um campo já existente em
  `report.Report` (dot-path lógico, não literal Go) — nunca um valor livre
  digitado à mão. Fecha o gap "de onde vem `valor`" do §4 sem inventar
  cálculo novo: o critério só *compara* algo que os analyzers já produzem.
- `performance_limits` é **redundante de propósito** com
  `analyzers.load.threshold_p95_ms` / `threshold_p99_ms` / `max_error_rate`
  que já existem em `Config` hoje (ver §4 abaixo — recomendo eliminar essa
  redundância, não duplicar).
- Sem `ressalva`/aprovação/"quem aprovou" — isso é tela + persistência,
  fora de escopo deste schema (mesmo gap já anotado como fora de alcance no
  gap-mapping, §4 penúltimo parágrafo).

---

## 2. Mapeamento pro struct Go (`internal/config/config.go`)

Seguindo a convenção do arquivo (struct por seção, `json` tag snake_case,
comentário com referência ao ID de spec se houver):

```go
// GatePolicy define os critérios de aprovação de release configuráveis por
// repositório (resolve gaps §4/§4.1/§12 do contrato do dashboard).
type GatePolicy struct {
	ApprovalThreshold float64          `json:"approval_threshold"`
	Criteria          []GateCriterion  `json:"criteria"`
	PerformanceLimits PerformanceLimits `json:"performance_limits"`
}

// GateCriterion é um critério nomeado e comparável contra um campo do report.
type GateCriterion struct {
	ID       string      `json:"id"`
	Label    string      `json:"label"`
	Source   string      `json:"source"`   // dot-path em report.Report, ex. "scores.quality"
	Operator string      `json:"operator"` // gte|lte|eq|gt|lt
	Value    any         `json:"value"`    // number|bool, tipo depende do source
	Blocking bool         `json:"blocking"`
}

// PerformanceLimits são os limites usados tanto pelo critério do gate quanto
// pelo campo `limite` de cada métrica no dashboard (§12).
type PerformanceLimits struct {
	P95MS     int     `json:"p95_ms"`
	P99MS     int     `json:"p99_ms"`
	ErrorRate float64 `json:"error_rate"`
}
```

Adicionar em `Config`:

```go
type Config struct {
	// ... campos existentes ...
	GatePolicy GatePolicy `json:"gate_policy"`
}
```

**Redundância a resolver, não duplicar**: `Analyzers.Load` já tem
`ThresholdP95MS`/`ThresholdP99MS`/`MaxErrorRate` usados pelo k6 runner pra
decidir pass/fail do próprio load test. `GatePolicy.PerformanceLimits` seria
o mesmo número usado pra exibir `limite` no dashboard (§12) e alimentar o
critério do gate. Duas opções, escolher uma antes de codar:

- (a) `GatePolicy.PerformanceLimits` fica vazio por default e, se não
  setado, o report/gate lê de `Analyzers.Load.Threshold*` (fonte única,
  zero duplicação, mas indireção extra pra quem só quer editar a política).
- (b) remover `Threshold*` de `LoadAnalyzer` e migrar tudo pra
  `GatePolicy.PerformanceLimits` (fonte única, mas quebra config existente —
  precisa de migração/aviso de schema).

Recomendo (a): menor blast radius, e `LoadAnalyzer.Threshold*` já é uma
decisão de execução do k6 (não de política de aprovação) — fica ambíguo
misturar os dois estruturalmente.

---

## 3. Mapeamento pro gate engine (`internal/gate`)

Hoje `gate.Evaluate` roda dois subgates fixos: `Quality` (score vs.
threshold por profile) e `Independence` (peers distintos executados). Os
critérios do mockup (`cobertura`, `vuln_criticas`, `novas_violacoes`,
`migration_reversivel`, `testes_regressao`) **não mapeiam 1:1** pra esse
desenho — são checks ad-hoc contra campos do report, não eixos de auditoria
do processo de review como Quality/Independence são.

Duas rotas possíveis:

### Rota A — subgates dinâmicos dentro de `internal/gate`

Generalizar `Subgates` de struct fixo (`Quality`, `Independence`) pra
`map[string]SubgateResult` populado a partir de `GatePolicy.Criteria`, com
um avaliador genérico `evaluateCriterion(criterion, report) SubgateResult`.

- Prós: uma única fonte de verdade pro status do gate (o `GateResult` que já
  é auditável, versionado e teve `IsBlocking()`/`RenderGate()` desenhados
  pra isso). CLI (`solidify run` exit code) e dashboard leem o mesmo motor.
- Contras: reescreve o motor — `Subgates` como struct fixo é usado em vários
  lugares (`run.go`, `report/builder.go` monta `QualityGate.Rules` a partir
  dele). Migrar pra map muda a assinatura pública de `gate.GateResult` e
  possivelmente quebra o schema JSON do report (`quality_gate.rules[]` já
  versionado). Precisa de bump de `report.SchemaVersion`.

### Rota B — critérios extras ficam fora de `internal/gate`, só em `mapGateSection` (frontend)

`internal/gate` continua avaliando só Quality+Independence pro **exit code
do CLI** (o que hoje decide se o pipeline falha). `GatePolicy.Criteria` vira
apenas dado de configuração servido no report (ex.: novo campo
`report.Report.GatePolicy` ecoando a política + os valores medidos), e o
dashboard (`mapGateSection` em `main.js`) compara client-side, como já faz
hoje pros 4 critérios derivados que o gap-mapping cita em §4 ("Real hoje:
... + 4 critérios derivados client-side").

- Prós: zero mudança em `internal/gate`, zero risco de regressão no exit
  code do CLI (que é o que hoje decide bloqueio real de pipeline). Rápido
  de implementar — é só popular mais um bloco no report e estender
  `mapGateSection`.
- Contras: **dois motores de decisão de bloqueio** — o exit code do CLI
  (Quality+Independence) pode dizer PASS enquanto o dashboard mostra
  "reprovado" por `vuln_criticas` falhar, porque o dashboard nunca alimenta
  de volta o exit code. Se `bloqueante: true` em política não bloqueia o
  pipeline de verdade, o campo é decorativo — contradiz a premissa do
  produto ("SOLID quality gate", não "SOLID quality dashboard").

### Recomendação

**Rota A, mas em etapas**: primeiro adicionar os critérios de política como
subgates dinâmicos *adicionais* (`Subgates` vira struct com `Quality`,
`Independence` fixos + `[]SubgateResult` extra nomeado, não migrar tudo pra
map de uma vez — menor blast radius que reescrever a estrutura toda). Isso
preserva compat de schema pros dois subgates existentes e adiciona uma
lista extra opcional. `combineSubgates` já tem a lógica de precedência
(`Blocking` teem prioridade) — só precisa iterar sobre a lista extra
também.

Rota B é aceitável como **passo intermediário** (dashboard mostra o dado
mais cedo) só se o usuário concordar explicitamente que "bloqueante" no
dashboard é informativo até a Rota A ser feita — nunca como estado final,
porque cria a divergência de decisão descrita acima.

---

## 4. Impacto em `validate.go` / `override.go`

### `validate.go`

Novo check `validateGatePolicy()` adicionado à lista em `Validate()`:

- `approval_threshold` entre 0 e 100 (mesmo padrão de
  `scoring.gate.minimum_quality_score`).
- `criteria[].id` não vazio e único na lista (dashboard usa `id` como key
  de contador de filtro).
- `criteria[].operator` ∈ `gte|lte|eq|gt|lt` (mesmo padrão de
  `validSeverity`/enum fechado usado em todo o arquivo).
- `criteria[].source` — validar contra uma allowlist de paths conhecidos
  (não regex livre): se o `source` não existe em nenhum campo do report,
  falha cedo em vez de o critério silenciosamente nunca casar nada em
  runtime. Exige manter essa allowlist sincronizada com `report.Report`
  manualmente (custo de manutenção, mas evita erro de digitação silencioso
  — mesmo trade-off que `envNamePattern` já aceita no arquivo).
- `performance_limits.p95_ms`/`p99_ms` > 0, `error_rate` entre 0 e 1 (mesmo
  padrão de `analyzers.load.max_error_rate`).

### `override.go`

**Recomendação: gate_policy NUNCA entra em `setters` (não é overridable via
`--set` nem qualquer outra flag CLI).**

Justificativa, seguindo o precedente já documentado no topo do próprio
arquivo (comentário do `Overrides` type): "peso de score, política de
perfil e requisitos de contractual ficam fora de propósito: relaxar
contractual por flag de linha de comando anularia o falha fechado". Política
de gate é exatamente essa categoria — é a coisa que decide se uma release
passa. Se for overridable por flag:

- alguém roda `solidify run --set gate_policy.criteria[0].value=999` num
  script de CI e o gate nunca mais reprova nada, sem isso aparecer em
  nenhum diff revisado;
- quebra a garantia que hoje existe pra `LoadAnalyzer.AllowProd` (só true
  num arquivo versionado, nunca por flag) — mesmo princípio, mesmo risco.

Política de gate só muda via commit no `solidify.json`, revisado como
qualquer outra mudança de código. Isso é consistente com o resto do arquivo
(`AllowProd`, requisitos de `contractual`) e deve ser dito explicitamente
no comentário do novo struct, do jeito que já é feito em `LoadAnalyzer`.

---

## 5. Perguntas em aberto (dono do produto decide antes de codar)

1. **Quem pode editar `gate_policy` no `solidify.json`?** Mesma regra de
   CODEOWNERS do resto do repo, ou exige aprovação extra (ex.: 2 reviews)
   por ser a política que decide bloqueio de release?
2. **Falha bloqueante aborta o pipeline com exit code de erro, ou só marca
   o report e quem decide é um humano na tela de aprovação?** Isso decide
   se a Rota A do item 3 é obrigatória agora ou se dá pra shippar a Rota B
   como estado temporário aceito.
3. **`source` dos critérios: lista fechada (allowlist curada por nós) ou
   o usuário pode apontar pra qualquer campo do report livremente?** Lista
   fechada é mais segura (menos erro de digitação) mas trava a política em
   sync manual com o schema do report.
4. **Múltiplos profiles (`quick`/`release`/`contractual`) têm política de
   gate diferente, ou é uma política única pro repo inteiro independente
   do profile rodado?** Hoje `ProfileThresholds` já varia por profile — faz
   sentido `gate_policy` também variar, ou simplificar pra uma só?
5. **`performance_limits` duplica ou substitui
   `analyzers.load.threshold_p95_ms`/`threshold_p99_ms`/`max_error_rate`
   existentes?** (Ver opção (a) vs (b) na seção 2.) Decide se há migração
   de config existente ou não.
