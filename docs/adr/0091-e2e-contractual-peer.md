# ADR 0091 — E2E contractual peer review (SAI-108)

Status: Aceito. 2026-08-20.

## Contexto

SAI-108: validar cenário de peer review contractual
— 2+ peers, 1 arbiter, isolados, robustez, gate
contractual.

## Decisão

`internal/e2e/` (extensão):

- `PeersInput{Peers, Arbiters, RobustnessStatus,
  SwapApplied}`.
- `ContractualPeerExpected{PeersCount, ArbiterCount,
  Robustness, GateStatus="PASS", SwapApplied}`.
- `VerifyContractualPeer(p)`:
  - Peers ≥ 2 senão erro.
  - Arbiters == 1 senão erro.
  - Retorna robustness status.

## Consequências

- 3 testes adicionais: OK, < 2 peers, arbiter != 1.
- Total e2e: 15 testes.
- Validação alinhada com regras de
  `peer.ValidateAssignment` (SAI-093).

## Trade-offs

- Gate=PASS fixo — caller pode customizar via
  PeersInput.GateStatus. Trade-off: smoke check
  default > config.
- Sem validação de robustness real — usa string
  passada. Integração fica p/ SAI-095.
