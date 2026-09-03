# ADR 0007 — Component classifier

## Status

Aceito. 2026-08-19.

## Contexto

SAI-013 pede classificar um range em
`frontend-web / backend-api / worker / library / mobile / infra /
database / unknown`. Saída: uma classificação por release (não por
commit), mais uma partição `ClassifyAll()` para monorepos.

## Decisão

**Match por segmento, não só primeiro.** `apps/api/handlers/user.go`
tem segmentos `[apps, api, handlers, user.go]` — `api` é o sinal
relevante. Restringir ao primeiro segmento erra monorepos onde o
componente vive em `apps/<componente>/`.

**Prioridade na iteração dos hints.** Cada path é testado contra
cada hint em sequência. Primeiro match vence — a ordem dos hints é
`database > infra > mobile > worker > backend-api > frontend-web >
library`. Empate de score entre componentes usa a mesma ordem.

**Path score primeiro, manifest só se paths não votaram.** Manifest
em `apps/mobile/pubspec.yaml` sozinho significa "existe um app
Flutter", não "esse range é mobile". Quando há paths alterados que
casam um componente, o manifest não interfere. Sem paths, o manifest
define.

**Override Flutter:** se a stack detection reporta Flutter com
confidence ≥ 1.0 e há pelo menos um changed path dentro do `Dir` de
algum `pubspec.yaml`, força `Mobile`. Isso cobre o caso Flutter sem
prefixo `/mobile/` no path (ex.: `lib/main.dart` na raiz do projeto
Flutter).

## Consequências

**Positivas:**

- Monorepo `apps/{web,api}/...` classifica corretamente cada um.
- `services/worker/...` casa Worker, não BackendAPI (porque `worker`
  é segmento).
- Database vence API/infra quando há migration no range (específico).

**Negativas:**

- `packages/ui/...` casa FrontendWeb (segmento `ui`), não Library.
  Decisão consciente: prioriza o sinal mais específico de UI. Se o
  usuário realmente quer Library aqui, dá pra adicionar heurística
  "se o pai é packages/ → Library" — fica para iteração futura.
- Heurística "manifest só vota se paths não votaram" perde
  classificação quando há paths `unknown` misturados com manifests
  fortes. Aceitável — paths `unknown` provavelmente são configs
  auxiliares.

## Alternativas consideradas

- **Primeiro segmento só**: falha em `apps/api/...`. Rejeitado.
- **Manifests com mesmo peso de paths**: Flutter manifest em monorepo
  com frontend alterado vira Mobile mesmo quando o changed path é
  web. Rejeitado.
- **IA classifica**: overkill para 8 categorias com prefixos
  canônicos. Rejeitado.