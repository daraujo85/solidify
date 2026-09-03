# Solidify — Especificação do Dashboard e Laudo de Release

## 1. Objetivo

O relatório deve atender simultaneamente a dois públicos:

- **engenharia:** precisa de evidências, arquivos, linhas, scores, divergências e ações técnicas;
- **empresa/gestão/cliente:** precisa entender o que foi entregue, qualidade, riscos, testes executados e parecer final sem ler código.

A mesma estrutura lógica alimenta dashboard e PDF. O PDF é uma projeção print-friendly do `release-report.json`, não um documento produzido por um segundo pipeline semântico.

---

# 2. Hierarquia visual

A primeira tela/página deve responder em menos de 10 segundos:

1. Qual release foi analisada?
2. Passou no Quality Gate?
3. Qual a nota geral?
4. Como ficou S/O/L/I/D?
5. O que foi entregue?
6. Existe risco crítico?
7. Qual a confiança da análise?

Ordem recomendada:

```text
[Solidify] Release Quality Report
Projeto / release / base → head / data

[GATE]   [QUALITY 87/B]   [CONFIDENCE HIGH]   [RISK MODERATE]

S 91 ↑3   O 78 ↓4   L N/A   I 86 =   D 71 ↓12

Resumo executivo
Principais entregas
Principais riscos / ações antes do deploy
```

SOLID precisa ser visualmente dominante, não escondido em uma aba secundária.

---

# 3. Capa do PDF

Campos:

- logo Solidify;
- projeto;
- nome/identificador da release;
- `base_short_sha → head_short_sha`;
- branch/tag se aplicável;
- data/hora;
- Quality Gate;
- Quality Score;
- SOLID Score;
- Confidence;
- Release Risk;
- texto: “Análise limitada ao intervalo Git e aos checks executados”.

Não colocar lista de findings na capa.

---

# 4. Parecer executivo

Máximo aproximado de 6–10 linhas no PDF.

Estrutura:

- o que a release faz;
- componentes impactados;
- estado geral da qualidade;
- maior ganho/piora arquitetural;
- segurança/testes/performance;
- ação de deploy importante, se houver migration/env;
- conclusão do gate.

Exemplo de forma, não texto fixo:

> A release altera o módulo de cobrança para suportar X e corrige Y. Foram impactados backend e schema de banco. Os testes executados passaram e não foram detectadas vulnerabilidades críticas pelos checks realizados. O principal ponto arquitetural é a redução do DIP no adapter Z. Existe uma nova migration e uma env obrigatória que precisa ser configurada antes do deploy. Quality Gate: PASS WITH WARNINGS.

A síntese nunca pode afirmar algo que não esteja no JSON canônico.

---

# 5. Bloco SOLID

## 5.1 Cards por letra

Cada card:

- letra e nome completo;
- `before → after`;
- delta com seta;
- confidence;
- status `APPLICABLE` ou `N/A`;
- 1 frase de motivo;
- quantidade de findings por severity.

Exemplo:

```text
D — Dependency Inversion
86 → 62   ▼ -24
Confidence 91%
1 HIGH · 1 MEDIUM
“Application service passou a instanciar adapter concreto.”
```

## 5.2 Detalhe por princípio

Para cada finding:

- severity;
- título;
- introduzido pela release?;
- arquivo/linha;
- before/after evidence;
- racional;
- recomendação;
- resultado Peer A;
- resultado Peer B;
- decisão do Árbitro C.

Não mostrar chain-of-thought. Mostrar apenas racional estruturado e evidência.

## 5.3 Divergência

Quando A/B divergirem materialmente:

```text
Peer A: 58
Peer B: 79
Arbiter: 72
Motivo: Finding O-001 do Peer A foi rejeitado porque o switch já existia no base state.
```

Isso é diferencial de credibilidade do produto e deve ser visível.

---

# 6. O que foi entregue — Release Notes

Agrupar por:

- Features;
- Bug fixes;
- Refactors;
- Performance;
- Security;
- Configuration;
- Migrations;
- Breaking changes;
- Outros.

Cada item contém:

- título legível;
- descrição curta;
- componentes;
- commit(s) de origem;
- issue key quando disponível;
- evidence refs.

No PDF, esconder detalhes excessivos em apêndice; no dashboard, permitir expandir.

---

# 7. Escopo técnico

Mostrar:

- quantidade de commits;
- arquivos adicionados/modificados/removidos/renomeados;
- linhas +/- quando disponíveis;
- componentes detectados;
- stacks;
- arquivos mais impactados;
- blast radius estimado;
- módulos não analisados por não terem mudança.

A seção deve reforçar que a análise é diff-scoped.

---

# 8. Migrations

Esta seção aparece quando houver migration change; caso contrário, dashboard pode mostrar “Nenhuma migration alterada” discretamente e PDF pode omitir o detalhe.

Por migration:

- ID/nome;
- framework;
- path;
- added/modified/deleted/renamed;
- operações detectadas;
- objetos afetados;
- rollback/down;
- risco;
- ação antes/durante deploy;
- evidence refs.

Destaques:

- `HIGH/CRITICAL` em bloco de risco;
- operações destrutivas com rótulo claro;
- “migration alterada”, quando não é nova, deve chamar atenção porque pode representar drift/deployment risk.

Exemplo:

```text
V20260819_02__invoice_status.sql — NEW — MODERATE
ADD COLUMN invoice.status NOT NULL DEFAULT 'PENDING'
Rollback: não aplicável ao padrão Flyway configurado
Deploy action: executar migration antes de habilitar feature X
```

---

# 9. Envs / configuração

Nunca mostrar valor.

Tabela:

| Env | Estado | Componente | Required | Default | Documentada | Secret-like | Ação de deploy |
|---|---|---|---|---|---|---|---|

Estados:

- NEW;
- REMOVED;
- RENAMED;
- USAGE CHANGED;
- DOC ADDED;
- DOC MISSING.

Se `required=true` e `documented=false`, gerar warning.

Se `likely_secret=true`, mostrar apenas:

```text
PAYMENT_API_TOKEN — secret-like — CONFIGURE SECRET
```

Nunca incluir hashes do valor, tamanho do valor ou prefixo do token; isso não agrega ao laudo e pode vazar informação.

---

# 10. Qualidade estática / Sonar

Mostrar duas coisas separadas:

1. estado oficial do Sonar/quality gate, se disponível;
2. score normalizado do pilar Solidify.

Métricas:

- bugs;
- vulnerabilities;
- code smells;
- duplication;
- complexity;
- coverage;
- findings de new code/diff quando disponíveis.

Indicar claramente `project-wide` quando a métrica não for isolada ao diff.

---

# 11. Testes

Resumo:

- suites;
- passed/failed/skipped;
- duração;
- coverage;
- coverage delta se comparável;
- comando/runner;
- status.

Testes falhando precisam aparecer antes de detalhes de baixa prioridade.

---

# 12. Segurança

Subseções condicionais:

- secrets;
- SAST;
- dependency vulnerabilities;
- ZAP passive;
- ZAP active/API;
- waivers.

Cada finding:

- severity;
- scanner;
- categoria;
- componente/path/endpoint;
- introduzido pela release quando determinável;
- fix/recommendation;
- waiver.

Rodapé da seção:

> Os checks executados reduzem risco, mas não constituem garantia de ausência de vulnerabilidades.

---

# 13. Lighthouse

Só renderizar como pilar ativo quando aplicável.

Cards:

- Performance;
- Accessibility;
- Best Practices;
- SEO informativo;
- principais métricas lab.

Se frontend não foi alterado:

```text
Lighthouse — NOT APPLICABLE — a release não alterou componente frontend web.
```

No PDF, isso pode entrar apenas no quadro “Checks aplicáveis” para não desperdiçar espaço.

---

# 14. Performance / carga

Mostrar:

- target fingerprint;
- script/profile;
- VUs/duração;
- throughput;
- error rate;
- p50/p90/p95/p99;
- thresholds;
- baseline comparable?;
- deltas.

Sem baseline comparável, usar texto:

> Métricas registradas para esta release; não há baseline equivalente suficiente para afirmar regressão/melhoria.

---

# 15. Quality Score

Visualizar composição:

```text
SOLID                 82 × peso 50
Static Quality        91 × peso 15
Security              96 × peso 15
Tests                 88 × peso 10
Performance           N/A
Frontend              N/A
--------------------------------
Quality Score          86.4 / 100
```

O usuário precisa conseguir entender por que a nota existe.

Não esconder o denominador normalizado quando há N/A.

---

# 16. Release Risk

Nível e fatores, por exemplo:

```text
MODERATE
- nova migration com alteração de schema;
- nova env obrigatória;
- DIP reduziu 18 pontos em billing;
- nenhuma vulnerabilidade high/critical detectada.
```

Risk não é sinônimo de “código ruim”.

---

# 17. Quality Gate

Tabela de regras:

| Regra | Resultado | Evidência |
|---|---|---|
| Quality >= 70 | PASS | 86.4 |
| SOLID >= 65 | PASS | 82 |
| Tests passing | PASS | 143/143 |
| No new High/Critical security | PASS | security summary |
| Required analyzers completed | WARN/PASS | ... |
| Peer review complete | PASS | A/B/C |

Gate final deve ser derivado das regras, não escrito pela IA.

---

# 18. Transparência da IA

Seção obrigatória em `release`, `contractual` e `strict-plus`.

Campos por ator:

- papel;
- client;
- provider;
- model id exato;
- como o modelo foi selecionado;
- prompt version;
- evidence hash;
- latency;
- usage quando endpoint reportar;
- status;
- independente?;

Exemplo:

```text
Peer A
Client: Claude Code
Model: <client-reported-id>
Provider: terminal session
Role: blind reviewer #1

Peer B
Model: <9router model id>
Provider: 9Router
Role: blind reviewer #2
Selection: hybrid discovery / preferred match

Arbiter C
Model: <9router model id>
Provider: 9Router
Role: neutral adjudicator
Distinct from Peer B: yes
```

Não apresentar um modelo como “dono da verdade”.

---

# 19. Robustez por troca de papéis

Quando executada:

```text
Original: X=Peer B / Y=Arbiter → Quality 86.4 / PASS
Swapped:  Y=Peer B / X=Arbiter → Quality 84.9 / PASS
Delta: 1.5
Gate stable: yes
Result: STABLE
```

Se grande divergência, mostrar `UNSTABLE` e recomendar revisão humana antes de usar o laudo como evidência contratual.

---

# 20. Checks executados, N/A, skipped e failed

Quadro auditável:

| Check | Applicability | Execução | Motivo |
|---|---|---|---|
| Sonar | APPLICABLE | PASSED | configured project |
| Tests | APPLICABLE | PASSED | backend changed |
| Lighthouse | NOT_APPLICABLE | — | backend-only release |
| ZAP active | SKIPPED | — | disabled by policy |
| k6 | APPLICABLE | PASSED | API target configured |

Essa tabela impede que um relatório pareça mais completo do que realmente foi.

---

# 21. Recomendações

Ordenar por:

1. blocker/security/migration deployment action;
2. SOLID high impact;
3. test/performance gaps;
4. medium/low improvements.

Cada recomendação deve apontar evidence refs.

Limitar o parecer executivo às ações mais importantes; lista completa pode ficar no detalhe.

---

# 22. Apêndice Git

- commits completos;
- hashes;
- arquivos alterados;
- component map;
- tool versions;
- config hash;
- artifact hashes;
- prompt versions;
- waivers;
- runtime limitations.

O apêndice torna o PDF defensável e reproduzível sem poluir a leitura executiva.

---

# 23. Histórico no dashboard

Views:

- Quality Score por run;
- SOLID Score por run;
- S/O/L/I/D individual por run;
- release risk;
- gate outcomes;
- security highs/criticals;
- test pass/fail;
- performance baseline quando comparável.

Nunca comparar scores entre reports com schema/scoring policy incompatível sem marcar a quebra de série.

---

# 24. Acessibilidade e UX

- contraste adequado;
- não depender só de cor para PASS/FAIL;
- ícone + texto;
- tabelas responsivas;
- print sem cortes;
- scores com rótulo numérico;
- tooltips apenas como complemento;
- teclado funcional no dashboard;
- `N/A` visualmente diferente de `0`.

---

# 25. Regra de ouro do relatório

**Toda afirmação forte deve ser rastreável para evidence, analyzer result ou gate rule.**

O dashboard pode ser bonito; o PDF pode ser empresarial; nenhum deles pode virar marketing desconectado da evidência técnica.
