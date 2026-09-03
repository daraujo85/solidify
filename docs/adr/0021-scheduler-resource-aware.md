# ADR 0021 — Scheduler resource-aware

Status: Aceito. 2026-08-20.

## Contexto

SAI-027: 5 classes de analyzer (light/cpu/browser/memory-heavy/
active-network) com custos diferentes. Sem controle, dois browser
jobs derrubam o host (RAM/CPU); 20 light jobs paralelos saturam CPU.

## Decisão

`Scheduler` mantém semaphore (chan buffer) por class. Default:

| Class | Limite |
|---|---|
| light | runtime.NumCPU() |
| cpu | 2 |
| browser | 1 |
| memory-heavy | 1 |
| active-network | 2 |

`RunAll(jobs []Job)` dispara uma goroutine por job; cada uma adquire
o slot da class do analyzer antes de chamar `analyzer.Run`. Limite 0
significa sem semáforo (roda sem controle).

`DetectAndRun` faz Detect em paralelo leve (todos de uma vez) e só
depois roda os detectados sob semáforo.

Ordem dos results é preservada (slice pré-alocado).

## Consequências

- 17 testes, incluindo aceitação crítica: 4 browser jobs com
  delay → max in-flight = 1.
- Default = NumCPU é hardware-aware (laptop vs CI server vs 64-core
  build machine).
- Semaphore pattern evita complexidade de pool/worker; Go channels
  são a primitiva certa.
- `Limits()` devolve cópia do map (não compartilhar internals).
- Cancelamento via ctx: adquire sem OU ctx.Done, devolve erro
  apropriado.

## Trade-offs

- Limite por class fixo na config — não detecta carga do sistema.
  Aceitável para v1; v2 pode integrar `runtime/metrics` ou load
  average.
- Goroutine por job é leve mas bursts de 100 jobs criam pressão.
  Aceitável (analyzers tipicamente 5-15).
