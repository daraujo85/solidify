# ADR 0059 — Peer B Executor

Status: Aceito. 2026-08-20.

## Contexto

SAI-065: Peer B executor com **nova request/context** por
execução (independência), structured output (JSON schema),
timeout + retry único + repair único quando output inválido.

## Decisão

`ExecutorResult{Request, Content, ParsedContent, Model, Provider,
RepairCount, RetryCount, LatencyMS, StartedAt, CompletedAt,
Errors, ValidationErrors}` em `internal/peer/executor.go`.

`ExecutorOptions{Request, Provider, Model, Timeout, MaxRetries,
JSONSchema, Validator}`:
- timeout default 60s
- MaxRetries=0 (1 tentativa); MaxRetries=N (N+1 tentativas)
- JSONSchema opcional (força response_format)
- Validator opcional (parsed map → []string de erros)

`Execute(ctx, opts)`:
1. Validate request/provider/model
2. Loop até MaxRetries+1:
   - context.WithTimeout(ctx, Timeout)
   - Provider.CompleteJSON com system prompt "JSON-producing"
   - on error → retry
   - parse JSON content
   - if invalid AND RepairCount==0 AND JSONSchema set:
     - repairOutput(prompt="Fix this JSON")
     - re-parse → RepairCount++
   - if still invalid → retry
   - if Validator != nil && errors → ValidationErrors → retry
3. Success → return result

`repairOutput(ctx, opts, broken)`:
- system prompt "JSON repair assistant"
- user prompt: "The following JSON is malformed. Fix it..."

`parseJSONContent(s)`:
- StripCodeFences
- Unmarshal em map[string]any (não any — força object)
- retorna (parsed, ok)

`DefaultValidator(parsed)`:
- required: run_id, actor, verdict
- verdict ∈ {approve, request_changes, comment}

`ComputeOutputHash(content)` SHA256 helper.

`FormatResult(r)` debug.

## Consequências

- 17 testes (new executor, nil request/provider/model, OK,
  retry success/exhausted, repair JSON, validator fail,
  default validator OK/missing/bad verdict, parse JSON ok/
  fenced/fail/array, output hash, format result).
- Retry+repair independentes: retry para erros de rede/timeout,
  repair para output malformado. Spec alinhada.
- `Errors []string` audit trail — caller sabe cada tentativa.
- `ValidationErrors []string` separado de Errors — semântica
  (validator) vs técnico (rede/parse).

## Trade-offs

- Repair tenta 1 vez só — LLMs podem errar JSON consistentemente;
  mais retries não melhoram. Aceitável: spec pede "repair único".
- Validator failures disparam retry, não repair — assumption:
  output JSON válido + validator fail = problema semântico
  que nova tentativa pode resolver com temperature/seed diff.
- Repair prompt não inclui o schema original — depende do
  model "lembrar" do system prompt. ADR seguinte pode passar
  schema no repair prompt.
- Sem cancellation propagation no retry — ctx cancelado mata
  iteração atual mas loop pode tentar de novo se MaxRetries>0.
  Trade-off: resiliência vs shutdown graceful.
- `Validator` é `func(map[string]any) []string` — caller decide
  regra. DefaultValidator cobre caso canônico; spec livre.
