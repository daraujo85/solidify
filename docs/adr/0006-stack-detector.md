# ADR 0006 — Stack detector

## Status

Aceito. 2026-08-19.

## Contexto

SAI-012 pede detectar 9 stacks (Node, .NET, Java, PHP, Python, Go,
Flutter/Dart, Rust, Ruby) com `stack + confidence + evidence`. Entrada:
manifests do SAI-011.

## Decisão

Mapa `manifests.Kind → {Stack, weight}`. Soma weights por stack, cap em
1.0. Empate vai pela ordem canônica
(Node > Go > Python > Java > Rust > PHP > Ruby > Flutter > .NET).

Pesos:

| Kind                 | Stack | Weight | Razão                                  |
|----------------------|-------|--------|----------------------------------------|
| KindNodePackageJSON  | node  | 1.0    | Manifest forte                         |
| KindNodeYarnLock     | node  | 0.5    | Lockfile sozinho não prova Node        |
| KindNodePnpmLock     | node  | 0.5    | Idem                                   |
| KindGoMod            | go    | 1.0    | Manifest forte                         |
| KindPythonPyproject  | py    | 1.0    | Manifest forte                         |
| KindPythonRequire... | py    | 0.7    | Mais fraco que pyproject               |
| KindRustCargo        | rust  | 1.0    | Manifest forte                         |
| KindJavaPom          | java  | 1.0    | Manifest forte                         |
| KindJavaGradle       | java  | 1.0    | Manifest forte                         |
| KindPHPComposer      | php   | 1.0    | Manifest forte                         |
| KindRubyGemfile      | ruby  | 1.0    | Manifest forte                         |
| KindFlutterPubspec   | flutter | 1.0  | Manifest forte                         |
| KindElixirMix        | ruby  | 0.3    | Elixir ≠ Ruby, mas herda ecossistema   |
| KindDotnetCsproj     | dotnet | 1.0   | Manifest forte                         |
| KindDotnetFsproj     | dotnet | 1.0   | Manifest forte                         |
| KindDotnetSln        | dotnet | 0.5   | Solution sozinho não é prova           |
| KindDotnetGlobalJson | dotnet | 0.3   | SDK version pin, fraco                 |

Output: `Detection{Stack, Confidence ∈ [0,1], Evidence []string}`.
`Evidence` é a lista de paths que votaram naquele stack, ordenada
alfabeticamente.

`Primary()` devolve `d[0].Stack` (top por confiança + tiebreak).

## Consequências

**Positivas:**

- Determinístico: mesma entrada → mesma saída.
- Cap em 1.0 evita confiança inflada por lockfiles duplicados.
- Tiebreak explícito: estável entre versões.

**Negativas:**

- Elixir contado como Ruby-like é simplificação. Se precisar separar,
  vira stack próprio (BAIXO risco; Elixir raramente aparece como
  stack primário em release reports).
- Confidence é heurística, não probabilidade calibrada. Não usar
  diretamente como threshold de gate sem calibragem.

## Alternativas consideradas

- **Primeiro manifest vence**: ignora stack dominante. Rejeitado.
- **Sem weights (só presença binária)**: lockfile vale igual a
  package.json — falso positivo alto. Rejeitado.
- **IA classifica stack**: overkill para 9 categorias com manifests
  canônicos. Rejeitado.