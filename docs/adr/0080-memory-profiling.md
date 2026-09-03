# ADR 0080 — Memory profiling helpers (SAI-097)

Status: Aceito. 2026-08-20.

## Contexto

SAI-097: identificar cópias de diff/artifacts e
otimizar streaming/buffers. Sem dependência externa —
runtime.MemStats + pool caseiro.

## Decisão

`internal/prof/`:

- `prof.go`:
  - `Snapshot{HeapAlloc, HeapObjects, NumGC, Sys,
    Timestamp}` via `runtime.MemStats`.
  - `TakeSnapshot()` — leitura não-invasiva.
  - `Diff(before, after)` — calcula
    AllocDelta/ObjectsDelta/GCDelta/Elapsed.
  - `Measure(fn)` — `runtime.GC()` + before/after +
    diff.
  - `CopyDetector{id, witness}` — compara primeiros
    8 bytes p/ detectar modificação (proxy de cópia).
  - `BufferPool{size, pool chan []byte}` — pool de
    buffers; descarta buffers menores que size.
- `render.go`: `RenderDiff(d)` p/ log textual.

## Consequências

- 7 testes: snapshot, diff, measure, copy detector,
  buffer pool basic, buffer pool small, render.
- Pool limitado a 32 buffers — uso concorrente
  alto pode descartar. Tuning fica p/ ADR futuro.
- CopyDetector é heurístico (8 bytes header) —
  cópias que preservam header passam despercebidas.
  Inadequado p/ integridade, OK p/ smoke check.

## Trade-offs

- Sem integração com `pprof` — caller ainda pode
  chamar `net/http/pprof` se quiser profile
  completo. Trade-off: zero dep > unified API.
- BufferPool sem `sync.Pool` do stdlib — implementação
  caseira p/ controle de tamanho mínimo. Trade-off:
  controle > stdlib reuse.
- `runtime.GC()` no início de `Measure` força estado
  limpo — custa tempo, mas dá leitura estável.
