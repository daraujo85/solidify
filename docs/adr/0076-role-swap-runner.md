# ADR 0076 — Role-swap runner (SAI-093)

Status: Aceito. 2026-08-20.

## Contexto

SAI-093: rodar pipeline com Y como Peer B e X como
Arbiter. Contexto isolado entre runs (cada ator recebe
seu próprio snapshot de evidence).

## Decisão

`internal/peer/roleswap.go`:

- 3 roles: `peer_a`, `peer_b`, `arbiter_c`.
- `ActorAssignment{Role, Provider, Model}`.
- `ValidateAssignment` regras:
  - não-vazio;
  - todos com role/provider/model preenchidos;
  - sem role duplicada;
  - ≥ 2 peers;
  - 1 arbiter.
- Erros sentinel: `ErrInvalidAssignment`,
  `ErrDuplicateRole`, `ErrMissingArbiter`,
  `ErrInsufficientPeers`.
- `BuildSwap(orig)`: PeerA preserva, PeerB↔Arbiter
  trocam. Valida swapped após construir.
- `IsolatedContext{RunID, ActorRole, Evidence,
  StartedAt}` — cópia defensiva de evidence na
  construção (string copy-on-read).
- `EvidenceAt(t)` retorna nova cópia — caller
  não consegue mutar evidence original via slice
  tricks.
- `ExecuteSwap(runID, orig, swapped, evidence)`
  valida orig + swapped + runID, cria contexto
  isolado por actor.
- `HashAssignment(a)` representa plan como string
  ordenada (`peer_a:gc/g1|peer_b:kr/k1|...`).

## Consequências

- 13 testes: validate (ok/vazio/incomplete/dup/no
  peers/no arb), swap básico/inválido, isolamento de
  evidence, evidence copy, execute ok/empty runid/bad
  orig/bad swapped, ActorsByRole missing, hash
  determinístico, contextos independentes.
- Isolamento por cópia — se evidence for muito
  grande (>MB), custo de memória é proporcional.
  Trade-off: simplicidade > streaming.
- Swap é simétrico: PeerB↔Arbiter (regra fixa do
  projeto). Variantes (ex: rotação 3-way) ficam p/
  ADR futuro.

## Trade-offs

- Sem streaming de evidence — tudo em memória.
  Trade-off: zero-copy absoluto > simplicidade.
- `EvidenceAt(t)` ignora `t` por enquanto —
  placeholder p/ futuro (evidence temporal).
- HashAssignment não é SHA — só string concat.
  Serve p/ log/cache key, não p/ integridade.