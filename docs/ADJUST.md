Você está trabalhando no projeto **Solidify**.

O projeto já possui toda a esteira de análise de qualidade baseada em Git diff/ref comparison, evidence bundle, análise SOLID, ferramentas complementares, peer review independente, arbitragem, quality gates e geração de relatório JSON/HTML/PDF.

A implementação atual já cobre as tasks até **SAI-119**.

Antes de alterar qualquer coisa, leia obrigatoriamente:

* ADRs existentes;
* ADR 0116 — Canonical AI Evaluation Schema;
* ADR 0117 — Real Independence Gate;
* ADR 0118 — Terminal/MCP Peer A;
* ADR 0119 — Git Ref Comparison Analysis;
* `TASKS.md`;
* contratos/schemas atuais;
* implementação atual de provider/model discovery;
* pipeline de `peer_b`;
* pipeline de `arbiter`;
* geração atual de `run_id`;
* geração de `release-report.json`;
* tratamento de exit codes.

Não rediscuta arquitetura já consolidada sem motivo forte.

---

# Contexto do problema real

Foram feitas simulações reais com comparação de refs usando:

```bash id="2ed0cj"
--diff-mode endpoints
```

A comparação funcionou corretamente e encontrou:

```text id="9fbwpq"
1.048 arquivos alterados
```

O provider fez discovery de:

```text id="kwxep2"
112 modelos
```

A pipeline avançou corretamente até `peer_b`.

Porém duas execuções reais falharam por indisponibilidade operacional do modelo/provider.

## Caso 1

Modelo:

```text id="42eoe2"
claude-opus-4-6-thinking
```

Erro:

```text id="trm7ja"
quota exceeded
```

Provider informou reset em aproximadamente 1 hora.

Resultado:

```text id="c9vtxw"
exit code: 21
nenhum HTML
nenhum PDF
nenhum relatório final
```

---

## Caso 2

Modelo:

```text id="e19ht4"
gemini/gemini-3-flash-preview
```

Erro HTTP:

```text id="jv8fbu"
429
```

Mensagem concreta:

```text id="t7jqw6"
Your prepayment credits are depleted.
```

Resultado:

```text id="qqqwxl"
exit code: 21
nenhum HTML
nenhum PDF
nenhum relatório final
```

A SAI-119 está funcionando: refs resolvem, `endpoints` funciona e o diff é gerado.

O problema é estrutural na resiliência da camada de IA.

---

# Objetivo

Implementar:

```text id="m6ia8p"
ADR 0120 — Provider Resilience and Model Failover
SAI-120 — Provider Resilience, Model Failover and Partial Reports
```

A nova implementação deve garantir que falhas temporárias ou operacionais em um modelo não derrubem imediatamente toda a análise, quando existirem outros modelos compatíveis disponíveis.

Também deve garantir que, se uma execução criou um `run_id`, ela produza sempre um artefato auditável, mesmo que termine como:

```text id="9syaq5"
INCOMPLETE
```

---

# 1. Criar ADR 0120 antes da implementação

Criar:

```text id="tup58p"
docs/adr/0120-provider-resilience-model-failover.md
```

O ADR deve documentar:

* contexto;
* problema observado em execução real;
* diferença entre model discovery e model availability;
* classificação semântica de erros;
* política de retry;
* política de failover;
* comportamento por perfil;
* auditoria;
* partial reports;
* exit codes;
* limites de tentativas;
* segurança;
* performance;
* implicações no contractual;
* decisão final.

Não implementar primeiro e documentar depois.

---

# 2. Não tratar HTTP status como semântica suficiente

Hoje um `429` pode significar coisas diferentes.

Exemplos:

```text id="dqlcbq"
rate limit temporário
quota esgotada
créditos pré-pagos esgotados
limite diário
limite mensal
limite por modelo
limite por provider
```

Criar uma classificação semântica interna.

No mínimo:

```text id="0nx053"
RATE_LIMITED
QUOTA_EXHAUSTED
CREDITS_DEPLETED
MODEL_UNAVAILABLE
MODEL_NOT_FOUND
AUTH_ERROR
PERMISSION_DENIED
PROVIDER_UNAVAILABLE
NETWORK_ERROR
TIMEOUT
SCHEMA_ERROR
INVALID_RESPONSE
CONTEXT_LIMIT_EXCEEDED
UNKNOWN_PROVIDER_ERROR
```

Se já existir enum/contrato equivalente, evoluir o existente em vez de duplicar.

---

# 3. Criar erro estruturado

O erro do provider deve carregar estrutura suficiente para decisão automática.

Exemplo conceitual:

```json id="e8flj7"
{
  "category": "CREDITS_DEPLETED",
  "http_status": 429,
  "provider": "9router",
  "model": "gemini/gemini-3-flash-preview",
  "retryable_same_model": false,
  "fallback_allowed": true,
  "retry_after_seconds": null,
  "raw_reason": "Your prepayment credits are depleted."
}
```

Nunca depender apenas de string comparada de forma espalhada pelo código.

Centralizar normalização/classificação.

---

# 4. Retry vs Failover

Diferenciar claramente:

```text id="fu8oi6"
retry
```

de:

```text id="r2peal"
failover
```

## Retry

Mesma combinação:

```text id="o8a2ka"
provider + model
```

tentada novamente.

Exemplo apropriado:

```text id="yl9bs1"
RATE_LIMITED temporário
NETWORK_ERROR transitório
TIMEOUT transitório
```

## Failover

Troca de modelo e/ou rota.

Exemplo apropriado:

```text id="c5r4e4"
QUOTA_EXHAUSTED
CREDITS_DEPLETED
MODEL_UNAVAILABLE
```

Não tentar repetidamente o mesmo modelo quando o erro for estrutural.

---

# 5. Política recomendada de classificação

Implementar comportamento equivalente a:

```text id="flqqu4"
RATE_LIMITED
→ retry_same_model opcional
→ respeitar retry-after quando curto
→ failover permitido

QUOTA_EXHAUSTED
→ não retry same model
→ failover

CREDITS_DEPLETED
→ não retry same model
→ failover

MODEL_UNAVAILABLE
→ não retry same model
→ failover

MODEL_NOT_FOUND
→ não retry
→ remover candidato da run

AUTH_ERROR
→ não retry em loop
→ não mascarar credencial inválida
→ INCOMPLETE

PERMISSION_DENIED
→ não retry em loop
→ INCOMPLETE

NETWORK_ERROR
→ retry curto limitado
→ depois failover quando possível

TIMEOUT
→ retry curto limitado
→ depois failover quando possível

SCHEMA_ERROR
→ seguir SAI-116
→ schema repair
→ não confundir com provider failover por padrão

CONTEXT_LIMIT_EXCEEDED
→ escolher modelo compatível com contexto maior, se disponível
```

---

# 6. Model discovery não significa model availability

O provider retornou:

```text id="rqnjtk"
112 modelos
```

Isso significa apenas que os modelos estão descobertos/anunciados.

Não inferir que todos estão utilizáveis naquele momento.

Separar conceitualmente:

```text id="bh5252"
discovered_models
eligible_models
attempted_models
successful_models
failed_models
```

Se fizer sentido no design atual, também:

```text id="zx8e8n"
temporarily_unavailable_models
```

por execução.

---

# 7. Model Selector baseado em capabilities

Não escolher fallback aleatoriamente.

A seleção deve considerar os requisitos do papel.

Exemplo:

```text id="cwt71z"
role = peer_b
```

pode exigir:

```text id="s7qbrj"
structured_output = true
context_window >= required_context
reasoning_capability >= minimum
provider compatible
model not failed this run
```

O seletor deve filtrar candidatos antes de tentar.

Não usar simplesmente:

```text id="p6zl3t"
primeiro modelo diferente
```

---

# 8. Preferências de modelo

Se o usuário configurar explicitamente:

```text id="mm16eh"
peer_b.model = X
```

tratar `X` como primeira preferência.

Se falhar e failover estiver habilitado:

```text id="obs878"
X
↓ falha
candidate Y
↓
candidate Z
```

O fallback deve ser controlado pela política.

Adicionar configuração clara, se ainda não existir:

```text id="1g59yg"
ai.failover.enabled
ai.failover.max_models
ai.retry.max_attempts
```

Evitar configuração excessivamente granular sem necessidade.

---

# 9. Limite de failover

Não tentar os 112 modelos.

Criar limite seguro.

Default recomendado:

```text id="79ydcz"
max_model_failovers = 3
```

Ou:

```text id="ai3wzx"
1 modelo primário + até 3 alternativos
```

Se o projeto já possuir configuração de retry/failover, integrar nela.

Nunca entrar em loop ilimitado.

---

# 10. Evitar retries caros e lentos

Solidify é uma ferramenta local/complementar e precisa continuar leve.

Não fazer:

```text id="dg2txr"
retry 10x
espera 60s
tenta outro
espera 60s
...
```

Usar política curta e determinística.

Exemplo conceitual:

```text id="4dd1cg"
network timeout
→ retry 1

rate limit com retry-after <= configured_short_wait
→ aguarda

quota/credits
→ failover imediato
```

---

# 11. Registrar toda tentativa

Toda tentativa de modelo deve ser auditável.

Exemplo:

```json id="3ahw7q"
{
  "actor": "peer_b",
  "attempts": [
    {
      "attempt": 1,
      "provider": "9router",
      "model": "claude-opus-4-6-thinking",
      "result": "error",
      "error_category": "QUOTA_EXHAUSTED",
      "latency_ms": 1200
    },
    {
      "attempt": 2,
      "provider": "9router",
      "model": "gemini/gemini-3-flash-preview",
      "result": "error",
      "error_category": "CREDITS_DEPLETED",
      "latency_ms": 800
    },
    {
      "attempt": 3,
      "provider": "9router",
      "model": "minimax/minimax-m3",
      "result": "success",
      "latency_ms": 15400
    }
  ]
}
```

Sem vazar secrets.

---

# 12. Modelo solicitado vs executado

No report, diferenciar:

```text id="0p1iw4"
requested_model
executed_model
```

Exemplo:

```json id="mrpu9i"
{
  "actor": "peer_b",
  "requested_model": "claude-opus-4-6-thinking",
  "executed_model": "minimax/minimax-m3",
  "fallback_used": true,
  "fallback_count": 2
}
```

Isto é obrigatório no perfil:

```text id="cxyjhm"
contractual
```

e desejável em todos os outros.

---

# 13. Failover não pode esconder degradação

Se houve fallback, registrar explicitamente.

Exemplo:

```text id="11zvyv"
analysis_degraded = true
degradation_reason = model_failover
```

Mas não marcar automaticamente a análise como inválida apenas porque houve fallback.

O comportamento deve depender da política/profile.

---

# 14. Contractual profile

Para `contractual`, preservar auditabilidade máxima.

Se Peer B foi solicitado com modelo A e executado com modelo B, o report precisa mostrar isso.

Se o profile exigir modelos distintos:

```text id="fhii7s"
require_distinct_external_models = true
```

recalcular independência considerando os modelos realmente executados, não os configurados inicialmente.

Exemplo:

```text id="y1ugyr"
Peer A → Claude
Peer B requested → Gemini
Peer B fallback → Claude
```

Isso pode quebrar independência.

Não considerar apenas `requested_model`.

Usar:

```text id="3qf2zh"
executed_model
```

para o independence gate.

---

# 15. Arbiter também precisa de resiliência

Aplicar a mesma infraestrutura de provider resilience ao:

```text id="wpwgov"
peer_b
arbiter
```

Se o árbitro falhar por quota e houver outro modelo elegível, permitir failover.

Mas o report deve registrar:

```text id="vl14kj"
arbiter_requested_model
arbiter_executed_model
fallback_reason
```

---

# 16. Peer A via terminal/MCP

Não aplicar model failover automático ao Peer A da mesma forma, porque ele é fornecido pela sessão terminal/MCP.

Mas se o agente principal falhar ou não submeter avaliação, a pipeline deve continuar obedecendo SAI-117/118:

```text id="9eawqv"
independence_gate
INCOMPLETE/FAIL conforme profile
```

Não inventar Peer A substituto silenciosamente.

---

# 17. Partial report obrigatório

Nova regra arquitetural:

> Se um `run_id` foi criado, deve existir um `release-report.json`.

Mesmo em falha.

Isso deve valer para:

```text id="rqc96b"
provider failure
quota failure
credits failure
timeout
schema failure
arbiter failure
tool failure
unexpected stage failure
```

O report pode estar incompleto, mas deve existir.

---

# 18. Estado INCOMPLETE

Quando uma etapa necessária não consegue terminar, gerar algo conceitualmente assim:

```json id="kfh0gy"
{
  "run_id": "r...",
  "status": "INCOMPLETE",
  "score": null,
  "score_status": "unavailable",
  "quality_gate": "INCOMPLETE",
  "final_gate": "INCOMPLETE",
  "failed_stage": "peer_b"
}
```

Não gerar:

```text id="9dj4tx"
score=70
```

Não gerar:

```text id="7h3hue"
score=0
```

A avaliação não aconteceu.

---

# 19. Detalhar o erro no report

Exemplo:

```json id="d3qx4j"
{
  "failure": {
    "stage": "peer_b",
    "category": "CREDITS_DEPLETED",
    "provider": "9router",
    "model": "gemini/gemini-3-flash-preview",
    "http_status": 429,
    "retryable_same_model": false,
    "fallback_attempted": true,
    "fallback_exhausted": true
  }
}
```

---

# 20. HTML e PDF para INCOMPLETE

Se tecnicamente possível sem aumentar demais a complexidade, gerar HTML/PDF mesmo quando a análise está `INCOMPLETE`.

O documento precisa deixar claro:

```text id="mrx3dc"
ANÁLISE INCOMPLETA
```

Mensagem sugerida:

> A análise não pôde ser concluída porque uma etapa externa de IA ficou indisponível. Este documento não representa aprovação nem reprovação da entrega.

Mostrar:

* run id;
* refs/diff scope;
* stages concluídos;
* stage que falhou;
* motivo;
* tentativas de modelo;
* artifacts já coletados;
* score como indisponível.

Se o renderer depender de campos ausentes, corrigir o renderer para suportar `INCOMPLETE`.

---

# 21. Partial report não pode parecer laudo final aprovado

No PDF/HTML:

```text id="5h4v6k"
status = INCOMPLETE
```

deve ser visualmente evidente.

Não exibir:

```text id="gaif0w"
grade C
score 70
Aprovado com ressalvas
```

quando não houve avaliação válida.

---

# 22. Exit codes externos

Normalizar o contrato externo do CLI.

Manter:

```text id="jc3gy6"
0 = PASS
1 = FAIL
2 = INCOMPLETE
```

Não obrigar CI a entender dezenas de códigos.

---

# 23. Reason codes internos

Pode continuar existindo um código detalhado interno.

Exemplo:

```text id="k5lku5"
reason_code = 21
```

Mas ele deve ficar dentro do report/output estruturado.

Exemplo:

```json id="nb6ypi"
{
  "exit_code": 2,
  "reason_code": 21,
  "reason": "AI_PROVIDER_QUOTA_EXCEEDED"
}
```

Se já existirem códigos internos, preservar compatibilidade quando fizer sentido.

---

# 24. Não perder diagnóstico existente

A mensagem do provider:

```text id="m1blfp"
Your prepayment credits are depleted.
```

pode ser registrada de forma segura como:

```text id="jgq2ek"
raw_reason
```

ou versão sanitizada.

Não incluir headers sensíveis, tokens ou payloads confidenciais.

---

# 25. Cache de falha por execução

Quando um modelo falhar por:

```text id="n8znzn"
QUOTA_EXHAUSTED
CREDITS_DEPLETED
MODEL_UNAVAILABLE
```

não tentar novamente esse mesmo modelo durante a mesma run.

Manter:

```text id="fqqwru"
failed_models_this_run
```

Isso evita loops.

---

# 26. Cache temporário opcional entre runs

Avaliar se faz sentido guardar indisponibilidade por curto período.

Exemplo:

```text id="t77kkb"
model X unavailable until provider reset
```

Porém:

* não introduzir isso se complicar demais o MVP;
* não persistir indisponibilidade indefinidamente;
* respeitar `retry-after` quando fornecido;
* documentar no ADR se implementado.

---

# 27. Seleção de fallback por ranking

Se o projeto já possui ranking/configuração de modelos, reutilizar.

Caso não tenha, permitir ordem configurável.

Exemplo:

```text id="s27uf2"
peer_b.preferred_models:
  - claude-opus-4-6-thinking
  - minimax/minimax-m3
  - qwen/qwen3-coder
```

Mas não acoplar a implementação a esses nomes.

São apenas exemplos.

---

# 28. Capability metadata

Se o endpoint `/models` fornecer metadata útil, aproveitar.

Se não fornecer, permitir configuração local de capabilities.

Não inventar capabilities silenciosamente.

Exemplo:

```text id="aoj160"
context_window
structured_output
reasoning
tool_support
```

---

# 29. Required context size

Antes de escolher modelo, calcular estimativa do payload.

Se o evidence bundle exige contexto grande, excluir modelos incompatíveis.

Evitar tentar modelo que certamente vai retornar:

```text id="dbbcpc"
CONTEXT_LIMIT_EXCEEDED
```

---

# 30. Não mandar 1.048 arquivos crus para o modelo

Preservar princípio já existente:

```text id="5f00za"
Diff Scoped
Evidence First
Token Efficient
```

O fato de o diff possuir 1.048 arquivos não significa concatenar todos integralmente.

Reutilizar evidence bundle, symbol extraction, summaries, changed hunks e filtros existentes.

SAI-120 não deve aumentar consumo de tokens desnecessariamente.

---

# 31. Observabilidade da seleção

Adicionar logs claros.

Exemplo:

```text id="vprf2i"
peer_b:
  requested: claude-opus-4-6-thinking
  attempt 1: quota exhausted
  failover: minimax/minimax-m3
  attempt 2: success
```

Mas evitar logging excessivo de payload.

---

# 32. Dashboard

Adicionar informação opcional sobre resiliência.

Exemplo:

```text id="47bohb"
Peer B

Configured:
claude-opus-4-6-thinking

Executed:
minimax/minimax-m3

Fallback:
Yes

Reason:
Quota exhausted
```

Se não houve fallback:

```text id="k0c8kq"
Fallback: No
```

---

# 33. PDF contratual

No PDF, adicionar seção curta:

```text id="rv3apu"
Transparência dos modelos
```

Exemplo:

```text id="eu1c5l"
Peer A
Claude Code / modelo X
Executado conforme configurado

Peer B
Solicitado: Claude Opus
Executado: MiniMax M3
Fallback: quota esgotada

Árbitro
Modelo Y
Executado conforme configurado
```

Sem tornar isso visualmente dominante demais.

---

# 34. Gates

Não misturar indisponibilidade de provider com baixa qualidade.

Exemplo:

```text id="ek7p87"
quality_gate = INCOMPLETE
```

não:

```text id="1h013c"
quality_gate = FAIL
```

quando o score não pôde ser calculado.

---

# 35. Final gate

Exemplo:

```text id="eah19q"
peer_b failed
all fallbacks exhausted
→ final_gate = INCOMPLETE
→ exit = 2
```

Não PASS.

Não FAIL por score.

---

# 36. Se fallback funcionar

Exemplo:

```text id="hthjvu"
Peer B primary fails
Peer B fallback succeeds
Arbiter succeeds
schema valid
gates valid
```

A execução pode terminar:

```text id="lmvt49"
PASS
```

ou:

```text id="oggr4j"
FAIL
```

normalmente.

Mas registrar:

```text id="x487cc"
fallback_used = true
```

---

# 37. Independence Gate após fallback

Esse ponto é obrigatório.

Recalcular independência depois de saber os modelos realmente usados.

Exemplo:

```text id="8exq5m"
Peer A executed_model = Claude
Peer B requested_model = Gemini
Peer B executed_model = Claude
```

Resultado:

```text id="h7csfz"
distinct_models = 1
```

Logo, se o profile exige dois modelos distintos:

```text id="4bxyck"
independence_gate = FAIL
```

ou `INCOMPLETE`, conforme semântica já definida no ADR 0117.

Não usar requested model.

---

# 38. Arbiter distinctness

Se já existe exigência/configuração de arbiter distinto, preservar.

Fallback do árbitro também deve recalcular qualquer regra de distinctness aplicável.

---

# 39. Não mascarar auth/config error com dezenas de fallbacks

Se o provider inteiro estiver mal configurado:

```text id="mnkvfu"
401
invalid token
```

não tentar 3 modelos no mesmo provider inutilmente.

Classificar:

```text id="o1zvsl"
AUTH_ERROR
```

e encerrar aquela rota/provider.

---

# 40. Provider-level failure

Distinguir:

```text id="31wfqa"
model failure
```

de:

```text id="fyx069"
provider failure
```

Se o provider inteiro estiver fora:

```text id="xfrl19"
PROVIDER_UNAVAILABLE
```

não testar vários modelos inúteis dentro dele.

Se houver múltiplos providers configurados, pode haver failover entre providers, se a arquitetura atual permitir.

Não criar suporte multi-provider do zero se isso extrapolar escopo atual; documentar como futura evolução se necessário.

---

# 41. Testes obrigatórios

Criar testes amplos.

No mínimo:

## Error classification

* 429 rate limit;
* 429 credits depleted;
* quota exceeded;
* 401;
* 403;
* 404 model;
* timeout;
* connection refused;
* 5xx;
* invalid JSON;
* context exceeded.

## Retry

* retry transitório;
* no retry em credits depleted;
* no retry em quota exhausted;
* retry cap.

## Failover

* primary fails → fallback succeeds;
* primary + fallback 1 fail → fallback 2 succeeds;
* all fail;
* max failover respected;
* failed model not retried same run.

## Model selection

* capability filter;
* context filter;
* excludes failed models;
* configured preference order.

## Independence

* fallback creates same model as Peer A;
* fallback preserves distinctness;
* requested vs executed not confused.

## Partial report

* report exists after peer_b failure;
* report exists after arbiter failure;
* score null;
* score_status unavailable;
* status INCOMPLETE;
* failed_stage correct;
* failure reason correct.

## Exit

* PASS → 0;
* FAIL → 1;
* INCOMPLETE → 2;
* internal reason code preserved.

## HTML/PDF

* INCOMPLETE renders without panic;
* no fake score;
* no fake grade;
* clear incomplete banner.

## Secrets

* provider tokens redacted;
* headers redacted;
* raw error sanitized.

---

# 42. Fixtures

Criar fixtures para respostas reais semelhantes às observadas.

Exemplo 1:

```text id="fg3djg"
quota exceeded
reset in 1 hour
```

Exemplo 2:

```text id="e83fwr"
HTTP 429
Your prepayment credits are depleted.
```

Esses cenários devem virar regression tests.

---

# 43. Performance budget

A feature de resiliência não pode transformar um erro de provider em execução de vários minutos desnecessários.

Registrar:

```text id="oqfoue"
attempt count
retry waits
failover latency
```

Evitar waits longos por default.

---

# 44. Segurança

Nunca registrar:

* API keys;
* bearer tokens;
* authorization headers;
* cookies;
* secret env values;
* provider credentials.

Erro estruturado deve passar pelo redactor já existente.

---

# 45. Compatibilidade

Não quebrar configurações existentes.

Quando failover não estiver configurado explicitamente, definir default seguro no ADR.

Sugestão:

```text id="zmg41l"
failover.enabled = true
max_failovers = 3
```

Mas, se isso mudar comportamento existente de forma relevante, considerar:

```text id="vjryhj"
default false para quick/release
default true para contractual
```

ou configuração global explícita.

Decidir no ADR com justificativa.

---

# 46. CLI output

Quando houver failover:

```text id="hyem8f"
Peer B
  model: claude-opus-4-6-thinking
  result: quota exhausted
  fallback: minimax/minimax-m3
  result: success
```

Quando todos falharem:

```text id="ehlwns"
Peer B: INCOMPLETE
Reason: provider models unavailable
Attempts: 3
Report: .solidify/runs/<run_id>/release-report.json
```

---

# 47. Critério forte de artefato

Adicionar invariant/teste:

```text id="gpnn6d"
run_id_created ⇒ release_report_exists
```

Este deve ser um invariant arquitetural.

Se existir `run_id` e o processo termina sem report, considerar bug.

---

# 48. Não alterar o repositório analisado

Preservar comportamento read-only já validado.

A pipeline de análise não deve modificar:

* Azure DevOps;
* origem remota;
* refs remotas;
* branch original;
* repositório original.

Failover de IA não pode introduzir efeito colateral Git.

---

# 49. Atualizar TASKS.md

Adicionar:

```text id="n450y6"
SAI-120 — Provider Resilience, Model Failover and Partial Reports
```

Se a implementação for naturalmente grande, dividir em subtasks internas, por exemplo:

```text id="i7plmc"
SAI-120A — Provider Error Classification
SAI-120B — Capability-Aware Model Failover
SAI-120C — Partial Report Invariant
SAI-120D — Exit Code Normalization
```

Mas manter SAI-120 como feature umbrella.

---

# 50. Critérios de aceite

SAI-120 só pode ser considerada concluída quando:

1. erros de provider forem classificados semanticamente;
2. credits depleted não provocar retry inútil no mesmo modelo;
3. quota exhausted não provocar retry inútil no mesmo modelo;
4. rate limit transitório puder ser tratado separadamente;
5. model failover for limitado;
6. fallback usar apenas candidatos elegíveis;
7. todas as tentativas forem auditáveis;
8. requested model e executed model forem distintos no contrato;
9. independence gate usar modelos efetivamente executados;
10. Peer B tiver failover;
11. Arbiter tiver failover;
12. Peer A continuar respeitando arquitetura MCP atual;
13. falha total produzir `INCOMPLETE`;
14. `score=null`;
15. `score_status=unavailable`;
16. não existir fallback fake `70`;
17. existir `release-report.json` sempre que houver `run_id`;
18. HTML/PDF suportarem INCOMPLETE, se renderização parcial estiver dentro do escopo;
19. exit externo usar 0/1/2;
20. reason code detalhado continuar disponível;
21. tokens/segredos nunca forem expostos;
22. suíte inteira passar;
23. os dois casos reais observados virarem regression tests.

---

# 51. Forma de execução

Antes de codar:

1. leia ADRs 0116–0119;
2. encontre o código que trata provider errors;
3. encontre model discovery;
4. encontre model selection;
5. encontre `peer_b`;
6. encontre `arbiter`;
7. encontre criação de `run_id`;
8. encontre geração do report;
9. encontre exit code mapping;
10. escreva ADR 0120;
11. apresente brevemente quais packages/arquivos serão afetados;
12. só então implemente.

Não sair refatorando áreas não relacionadas.

Não avançar automaticamente para SAI-121.

---

# 52. Resultado esperado ao repetir o cenário real

Com o cenário:

```text id="0x30ob"
provider discovery = 112 models
primary model = claude-opus-4-6-thinking
error = quota exhausted
```

esperado:

```text id="c7tcbt"
classify QUOTA_EXHAUSTED
do not retry same model
select eligible fallback
continue
```

Se fallback também falhar com:

```text id="pbqyq3"
Your prepayment credits are depleted.
```

esperado:

```text id="7hfrv3"
classify CREDITS_DEPLETED
do not retry same model
select next eligible candidate
```

Se um candidato funcionar:

```text id="p0rz4u"
pipeline continues
report records fallback
gates run normally
```

Se todos os candidatos permitidos falharem:

```text id="s4rldf"
status = INCOMPLETE
score = null
score_status = unavailable
final_gate = INCOMPLETE
exit = 2
release-report.json exists
```

---

# 53. Entrega final da task

Ao terminar, retornar resumo objetivo:

```text id="888frm"
ADR 0120 status
SAI-120 status
packages alterados
arquivos principais alterados
novos contratos/enums
política de retry
política de failover
max failovers
partial report invariant
exit code mapping
testes adicionados
total de testes passando
regressions dos casos Claude/Gemini
impacto de performance
pendências conhecidas
```

O objetivo desta task é fazer com que o Solidify deixe de depender operacionalmente da disponibilidade de um único modelo e passe a tratar indisponibilidade de IA como parte normal e auditável da execução — sem esconder degradações, sem inventar scores e sem perder o artefato da run.
