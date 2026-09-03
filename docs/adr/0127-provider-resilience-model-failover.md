# ADR 0127 — Provider Resilience and Model Failover (SAI-127)

Status: Aceito. 2026-08-29.

## Contexto

Execuções reais do Solidify com comparação de refs resolveram o diff corretamente, mas falharam quando o modelo escolhido ficou indisponível: `quota exceeded` e HTTP 429 com `Your prepayment credits are depleted.`. Model discovery retornou modelos anunciados, mas isso não garantiu disponibilidade operacional. O processo terminou com código interno 21 e sem `release-report.json`, embora já tivesse criado `run_id`.

ADR 0120 permanece exclusivamente dedicado a CanonicalPayload v2. Este ADR não altera seu significado.

## Problema

O executor atual trata erros do provider como erro genérico, repete indiscriminadamente a mesma combinação provider/model e aborta antes de gerar artefato auditável. HTTP status isolado não distingue rate limit temporário, quota, créditos esgotados, autenticação ou indisponibilidade do modelo.

## Decisão

### 1. Classificação semântica centralizada

Adicionar erro estruturado em `internal/ai` com categoria, provider, model, HTTP status, retryabilidade, autorização de failover, retry-after e razão sanitizada. Categorias mínimas:

`RATE_LIMITED`, `QUOTA_EXHAUSTED`, `CREDITS_DEPLETED`, `MODEL_UNAVAILABLE`, `MODEL_NOT_FOUND`, `AUTH_ERROR`, `PERMISSION_DENIED`, `PROVIDER_UNAVAILABLE`, `NETWORK_ERROR`, `TIMEOUT`, `SCHEMA_ERROR`, `INVALID_RESPONSE`, `CONTEXT_LIMIT_EXCEEDED`, `UNKNOWN_PROVIDER_ERROR`.

A normalização centraliza reconhecimento por status, timeout/rede e mensagem. Nenhum caller compara strings de provider para decidir política.

### 2. Retry e failover

- `RATE_LIMITED`: no máximo um retry curto; respeita `retry-after` somente quando dentro do limite configurado; failover permitido.
- `NETWORK_ERROR` e `TIMEOUT`: retry curto limitado; depois failover.
- `QUOTA_EXHAUSTED`, `CREDITS_DEPLETED`, `MODEL_UNAVAILABLE`: não repetir mesmo modelo na mesma run; failover imediato.
- `MODEL_NOT_FOUND`: remove candidato da run.
- `AUTH_ERROR` e `PERMISSION_DENIED`: encerra a rota sem testar outros modelos inúteis.
- `SCHEMA_ERROR` e `INVALID_RESPONSE`: seguem repair do ADR 0116; não acionam failover por padrão.
- `CONTEXT_LIMIT_EXCEEDED`: seleciona candidato com contexto suficiente, se houver.

Default: um modelo primário e até 3 modelos alternativos (`max_model_failovers=3`), sem loops ilimitados e sem espera longa por padrão.

### 3. Seleção

Discovery, elegibilidade, tentativas, sucesso e falha são estados distintos por execução. Fallback respeita ordem configurada, exclusões e capabilities disponíveis (`structured output`, contexto e suporte requerido). Modelos falhos estruturalmente entram em `failed_models_this_run`.

### 4. Escopo por ator

A infraestrutura é aplicada a Peer B e Arbiter, que usam provider HTTP. Peer A via terminal/MCP não recebe substituição silenciosa: ausência ou falha permanece sujeita aos gates SAI-117/118.

### 5. Auditoria

Cada tentativa registra actor, provider, model, número, resultado, categoria, latência e motivo sanitizado. O report diferencia `requested_model` de `executed_model`, informa `fallback_used`, `fallback_count` e `analysis_degraded` quando aplicável. Segredos, headers, tokens, cookies e valores de ambiente nunca são registrados.

### 6. Partial report obrigatório

Depois de criar `run_id`, toda saída de execução deve tentar gravar `release-report.json`, inclusive discovery, provider, Peer B, Arbiter, schema, ferramenta e falha inesperada. Em falha sem score válido:

- `status=INCOMPLETE`;
- `score=null`;
- `score_status=unavailable`;
- `quality_gate=INCOMPLETE`;
- `final_gate=INCOMPLETE`;
- `failed_stage` e `failure` estruturados.

Partial report não apresenta grade ou aprovação fictícia. HTML/PDF ficam fora do caminho crítico se o renderer não suportar o estado sem grande refatoração; quando suportado, exibem banner explícito de análise incompleta.

### 7. Exit codes

A interface externa do CLI é normalizada para:

- `0 = PASS`;
- `1 = FAIL`;
- `2 = INCOMPLETE`.

O report preserva `reason_code` interno (`12` para INCOMPLETE, `21` para provider e demais códigos existentes) para diagnóstico e compatibilidade interna. Não se altera o significado histórico dos códigos internos.

### 8. Compatibilidade e segurança

Configurações existentes continuam válidas. Failover habilitado por default para executores HTTP com limite de três alternativas; Peer A/MCP não é substituído silenciosamente. A análise continua read-only em Git e evidence. Não se adiciona dependência: classificação usa stdlib Go.

## Consequências

Falhas operacionais transitórias deixam de derrubar imediatamente a análise quando há candidato elegível. Falha total continua honesta e auditável como INCOMPLETE. Runs podem consumir algumas tentativas adicionais, limitadas e observáveis; quota/créditos não geram retries inúteis.

## Verificação

A suíte cobre classificação, retry, failover limitado, seleção por capabilities, exclusão de modelos falhos, requested/executed model, independência pós-fallback, partial report, normalização 0/1/2 e redaction dos diagnósticos.

## Não objetivos

- Multi-provider criado do zero.
- Cache persistente de indisponibilidade entre runs.
- Substituição automática de Peer A/MCP.
- Alteração de ADR 0120, ADR 0119 ou tasks históricas.
- Avanço automático para SAI-128.
