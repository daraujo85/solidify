# Solidify — Handoff para o Agente Implementador

## Missão

Implemente o Solidify seguindo rigorosamente os documentos deste diretório. O projeto será desenvolvido majoritariamente por agentes, portanto a especificação é a fonte de decisão arquitetural e `TASKS.md` é a ordem de execução.

## Antes de escrever código

Leia, nesta ordem:

1. `README.md`;
2. `ARCHITECTURE.md`;
3. `CONTRACTS.md`;
4. `REPORT_SPEC.md`;
5. `schemas/peer-review.schema.json`;
6. `schemas/arbiter.schema.json`;
7. `schemas/release-report.schema.json`;
8. `solidify.example.json`;
9. `TASKS.md`.

Somente depois inicie `SAI-001`.

## Restrições arquiteturais obrigatórias

- Core em Go.
- Docker-first, mas sem daemon obrigatório.
- SQLite + artifacts em filesystem.
- MCP + CLI no MVP; não criar REST/GraphQL de aplicação.
- Dashboard estático compilado e embutido no binário.
- PDF renderizado do mesmo `release-report.json`/HTML.
- SOLID é 50% do Quality Score inicial e é o protagonista visual.
- IA nunca varre o projeto inteiro por padrão; começa em Git diff e contexto bounded.
- Peer A e Peer B devem ser cegos entre si.
- Arbiter C deve executar em contexto separado e receber evidence + reviews A/B + divergence map.
- Provider externo deve ser OpenAI-compatible; 9Router é preset de referência.
- Nenhuma API paga é requisito do core.
- Model discovery deve suportar `GET /v1/models` quando o provider oferecer.
- Migrations e env changes são itens obrigatórios do release inventory.
- Nunca persistir valor de env/secret.
- Lighthouse só é aplicável a frontend web afetado e executável.
- Ferramentas pesadas rodam sob demanda e com concorrência limitada.
- Em `contractual`, faltar peer/arbiter/analyzer obrigatório bloqueia o laudo.

## Regra de dependências

Antes de adicionar qualquer biblioteca:

1. verifique se a standard library resolve;
2. estime o impacto no binário/runtime;
3. verifique manutenção/licença;
4. compare com alternativa menor;
5. adicione somente se houver ganho claro.

Dependências já aprovadas:

- SDK MCP oficial Go;
- `modernc.org/sqlite`.

Qualquer framework CLI, ORM, DI container, plugin framework ou logger pesado precisa de justificativa/ADR.

## Regra de performance

O Solidify é ferramenta complementar e deve coexistir com IDE, Docker, banco e LLMs locais.

Não aceite arquitetura que:

- mantenha Sonar/ZAP/Chromium em background;
- suba todos os profiles Docker juntos;
- faça full-repo LLM scan;
- mantenha cache em RAM entre processos;
- use banco servidor no MVP;
- crie serviços separados sem necessidade.

Acompanhe os budgets de `ARCHITECTURE.md` desde o início, não no final.

## Regra de tasks

Para cada `SAI-XXX`:

1. declare que task está iniciando;
2. implemente apenas o escopo dela;
3. escreva/atualize testes;
4. rode verificações pertinentes;
5. registre decisões importantes;
6. só então avance.

Se uma task revelar falha na especificação, não improvise silenciosamente. Crie `docs/adr/NNNN-<tema>.md` com:

- contexto;
- incompatibilidade encontrada;
- opções;
- decisão proposta;
- impacto em performance/compatibilidade;
- invariantes preservados.

## Regra de segurança

- Não imprima secrets.
- Não leia env values para compor o relatório.
- Não execute active scan em target não allowlisted.
- Não execute comandos vindos de commit message.
- Não permita path traversal nas tools MCP.
- Preserve raw scanner output localmente quando necessário, mas sanitize logs e inputs de LLM.

## Regra da IA

A IA produz interpretação, não fatos objetivos.

Os fatos objetivos vêm de:

- Git;
- manifests;
- files/hunks;
- tests;
- Sonar;
- security tools;
- Lighthouse;
- k6;
- migration/env detectors.

O score final deve ser calculado pelo core. O LLM não escreve `quality_score` arbitrariamente.

## Entrega incremental esperada

O primeiro milestone útil não é “dashboard bonito”. É:

```text
Git range
→ changed files
→ component detection
→ migrations/envs
→ Evidence Bundle
→ SQLite/artifacts
→ normalized analyzer framework
```

O segundo milestone é:

```text
MCP
→ Peer A
→ provider OpenAI-compatible/9Router
→ Peer B
→ Arbiter C
→ SOLID final
```

O terceiro é:

```text
Quality Score/Gate
→ canonical JSON
→ dashboard
→ PDF
```

Só depois finalize robustez role-swap, benchmarks e hardening E2E.

## Resultado esperado ao concluir

Uma release deve poder sair com algo como:

```text
Quality Gate: PASS WITH WARNINGS
Quality Score: 86.4 / B
SOLID: 82
S 91 (+3) | O 78 (-4) | L N/A | I 86 (=) | D 71 (-12)
Confidence: HIGH
Risk: MODERATE

Delivery:
- 2 features
- 1 bugfix
- 1 migration
- 2 new envs

Peer review:
- Peer A: terminal model X
- Peer B: 9Router model Y
- Arbiter C: 9Router model Z
- independence: OK
```

com todos os números e afirmações rastreáveis para o artefato canônico.
