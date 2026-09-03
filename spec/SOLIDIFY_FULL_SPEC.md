# Solidify — Especificação Completa Consolidada

> Documento consolidado para ingestão por agente. Os JSON Schemas e prompts executáveis permanecem como arquivos separados no pacote.



---

<!-- BEGIN HANDOFF: AGENT_HANDOFF.md -->

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


<!-- END HANDOFF: AGENT_HANDOFF.md -->


---

<!-- BEGIN README: README.md -->

# Solidify — Especificação de Engenharia

> **Status:** arquitetura proposta para implementação
> **Data-base:** 2026-08-19
> **Nome de trabalho:** Solidify
> **Objetivo:** ser um “Lighthouse da qualidade de uma entrega”, com SOLID como protagonista, evidência determinística e revisão por pares com IA.

## 1. O que este pacote contém

Este pacote não é um brainstorming. Ele é a especificação de implementação para um agente de software executar o projeto de forma task-driven, preservando as decisões arquiteturais já tomadas.

Arquivos:

- `AGENT_HANDOFF.md`: instrução pronta para o agente iniciar a implementação sem rediscutir a arquitetura.
- `ARCHITECTURE.md`: visão completa, arquitetura, fluxo, componentes, critérios de aplicabilidade, performance, segurança, persistência, dashboard, PDF e decisões técnicas.
- `CONTRACTS.md`: contratos de CLI, MCP, revisão por pares, scoring, artefatos e integrações externas.
- `TASKS.md`: plano de implementação em tarefas pequenas, ordenadas, com critérios de aceite e testes.
- `REPORT_SPEC.md`: contrato visual e de conteúdo do dashboard/PDF empresarial.
- `solidify.example.json`: configuração de referência para um projeto real.
- `schemas/release-report.schema.json`: contrato JSON canônico do relatório final.
- `schemas/peer-review.schema.json`: contrato JSON das análises independentes de IA.
- `schemas/arbiter.schema.json`: contrato JSON do veredito do Árbitro C.
- `prompts/peer-review.md`: contrato de comportamento do Peer A/Peer B.
- `prompts/arbiter.md`: contrato de comportamento do Árbitro C.

## 2. Proposta em uma frase

**Solidify analisa apenas o que mudou em uma entrega Git, combina métricas objetivas e testes aplicáveis com uma revisão SOLID por pares independente e gera um artefato auditável em JSON, dashboard HTML e PDF.**

## 3. Princípios que não devem ser rediscutidos durante o MVP

1. **SOLID é o centro do produto.** Cada letra recebe avaliação independente, evidências, nota quando aplicável e delta antes/depois.
2. **Evidence first.** Nenhum modelo cria fatos; commits, diffs, arquivos, testes e ferramentas são a fonte primária.
3. **Diff first.** A IA não varre o repositório inteiro. O escopo nasce de `base..head`, com expansão de contexto apenas quando necessária.
4. **Revisão por pares.** Em modo contratual: Peer A e Peer B trabalham de forma cega e independente; Árbitro C compara as duas análises e as evidências.
5. **Contextos isolados.** Peer B nunca recebe o texto do Peer A. Árbitro C é uma chamada/sessão nova e não reutiliza a janela de contexto de nenhum peer.
6. **Modelo não é autoridade.** A autoridade vem do processo, das evidências, do consenso e da arbitragem.
7. **Agnóstico de fornecedor.** O agente principal pode ser Claude Code, Codex ou outro terminal compatível com MCP. Juiz/árbitro usam um endpoint OpenAI-compatible; 9Router é o primeiro adapter de referência, não uma dependência rígida.
8. **Sem custo de API obrigatório.** O fluxo básico funciona apenas com o agente já utilizado pelo desenvolvedor. Juiz e árbitro externos são configuráveis; no cenário de referência podem aproveitar assinaturas/provedores já expostos pelo 9Router.
9. **Leve por padrão.** Não existe daemon obrigatório. O core sobe somente quando chamado. Ferramentas pesadas rodam sob demanda e encerram ao terminar.
10. **Docker-first, não Docker-heavy.** Docker serve para isolamento e portabilidade; serviços pesados não ficam residentes.
11. **Aplicabilidade automática.** Lighthouse só roda para frontend web aplicável; carga só roda em serviços/endpoints aplicáveis; migrations e envs só aparecem quando detectadas; itens não aplicáveis não reduzem nota.
12. **Um artefato canônico.** `release-report.json` é a fonte da verdade. Dashboard e PDF apenas renderizam o mesmo conteúdo.
13. **Segredos nunca entram no relatório.** Nomes de env podem ser registrados; valores, tokens e secrets não.
14. **Rastreabilidade de release.** Commits, features/bugfixes, migrations, envs, testes, segurança, performance, modelos usados e decisões de arbitragem aparecem no relatório.
15. **Falhar de forma honesta.** Em release contratual, ausência de evidência obrigatória resulta em `BLOCKED` ou `INCOMPLETE`, não em uma nota artificialmente alta.

## 4. Experiência-alvo

Fluxo esperado em um terminal com agente:

```text
Usuário: faça a revisão Solidify desta release entre origin/main e HEAD

Agente -> MCP Solidify: begin_review(base="origin/main", head="HEAD", profile="contractual")
Solidify -> coleta Git + detecta componentes + roda analisadores objetivos aplicáveis
Agente -> MCP Solidify: obtém Evidence Bundle
Agente -> produz Peer A em JSON
Agente -> MCP Solidify: submit_peer_review(...)
Solidify -> executa Peer B isolado via provider configurado
Solidify -> executa Árbitro C isolado via provider configurado
Solidify -> consolida scores determinísticos
Solidify -> gera release-report.json + dashboard + PDF
```

O usuário também pode iniciar pelo CLI e depois pedir ao agente para continuar via MCP.

## 5. Saídas de uma execução

```text
.solidify/
  solidify.db
  runs/
    01J.../
      run.json
      evidence/
        manifest.json
        git.json
        changes.json
        migrations.json
        env-changes.json
        test-results.json
        sonar.json
        lighthouse.json
        security.json
        load.json
      reviews/
        peer-a.json
        peer-b.json
        arbiter.json
        robustness.json         # se executado
      release-report.json       # fonte canônica
      report.html
      report.pdf
      logs/
```

## 6. Resultado visual mínimo

O dashboard e o PDF devem destacar, nesta ordem:

- resultado do Quality Gate;
- nota geral da release;
- **S · O · L · I · D** com uma nota/estado por letra;
- delta de cada princípio entre o código anterior e o código entregue;
- confiança da análise e grau de consenso entre peers;
- resumo executivo do que foi entregue;
- features, bugfixes, refactors e breaking changes detectados;
- qualidade estática/Sonar;
- testes;
- segurança;
- performance/carga;
- Lighthouse, quando aplicável;
- migrations;
- novas/alteradas/removidas envs;
- riscos e recomendações;
- transparência da revisão por IA: modelo, papel, provider, latência, prompt version e divergências;
- apêndice de rastreabilidade Git.

## 7. Perfis operacionais

### `quick`

Uso durante desenvolvimento. Coleta Git, stack, mudanças, checks rápidos e Peer A. Não exige segundo modelo nem PDF.

### `release`

Uso antes de merge/release. Executa todos os analisadores aplicáveis configurados e pode usar Peer B + Árbitro C.

### `contractual`

Uso para gerar um laudo técnico que acompanha uma entrega. Exige revisão por pares configurada, árbitro, artefato canônico validado, PDF e cumprimento das regras de Quality Gate declaradas no projeto.

### `strict-plus`

Inclui experimento de robustez com troca de papéis dos modelos externos e comparação de estabilidade do veredito.

## 8. Ordem recomendada de leitura para o agente implementador

1. `AGENT_HANDOFF.md`.
2. `ARCHITECTURE.md` inteiro.
3. `CONTRACTS.md` inteiro.
4. `REPORT_SPEC.md`.
5. `schemas/*.json`.
6. `TASKS.md`.
7. Executar as tarefas em ordem, sem antecipar features de fases posteriores.

## 9. Regra de mudança arquitetural

Se durante a implementação uma decisão desta especificação ficar inviável por incompatibilidade técnica comprovada, o agente deve:

1. registrar o problema em um ADR curto;
2. demonstrar a incompatibilidade com teste ou documentação;
3. propor a alternativa mais leve;
4. preservar os invariantes: diff-first, evidence-first, SOLID-first, revisão independente, artefato canônico, baixo consumo e execução sob demanda.

Não trocar tecnologia apenas por preferência do agente.


<!-- END README: README.md -->


---

<!-- BEGIN ARQUITETURA: ARCHITECTURE.md -->

# Solidify — Arquitetura e Especificação Técnica

## 0. Documento normativo

Palavras **DEVE**, **NÃO DEVE**, **DEVERIA**, **PODE** têm sentido normativo. Este documento descreve o MVP e os requisitos para evolução sem sacrificar leveza, auditabilidade ou independência da análise.

---

# 1. Problema

Ferramentas tradicionais de qualidade medem partes objetivas do software: bugs, code smells, cobertura, vulnerabilidades, performance de página, latência etc. Elas são úteis, mas normalmente não respondem de forma clara a perguntas arquiteturais como:

- a entrega piorou a responsabilidade única?
- uma extensão futura ficou mais difícil?
- uma abstração nova quebra substituibilidade?
- uma interface virou um “contrato gordo”?
- o código de alto nível ficou acoplado a detalhes concretos?
- o que exatamente esta release entregou?
- quais migrations ou novas configurações operacionais foram introduzidas?
- houve regressão mensurável em qualidade, segurança ou performance?
- quão confiável é a análise gerada por IA?

A proposta do Solidify é tratar uma **entrega Git** como unidade de análise e produzir um laudo de release baseado em evidências.

---

# 2. Escopo funcional

## 2.1 Entradas suportadas

Uma execução DEVE aceitar:

- `base` e `head` Git;
- um commit isolado;
- um intervalo de commits;
- uma branch comparada contra outra;
- opcionalmente um tag/release range.

PR remoto NÃO é necessário para o MVP. Se a branch/commit já existir localmente, o Solidify trabalha apenas com Git local. Integrações GitHub/GitLab podem ser plugins posteriores.

## 2.2 Saídas

Cada execução DEVE produzir:

1. Evidence Bundle;
2. resultados determinísticos dos analisadores;
3. revisão SOLID estruturada;
4. arbitragem, quando configurada;
5. `release-report.json` validado por JSON Schema;
6. dashboard HTML;
7. PDF quando o perfil exigir;
8. histórico resumido no SQLite.

---

# 3. Arquitetura macro

```text
┌──────────────────────────────────────────────────────────────────────┐
│                     Claude Code / Codex / Agent                      │
│                                │ MCP                                 │
└────────────────────────────────┼─────────────────────────────────────┘
                                 ▼
┌──────────────────────────────────────────────────────────────────────┐
│                         Solidify Core (Go)                            │
│                                                                      │
│  Git Scope → Project Detector → Evidence Builder → Analyzer Runner  │
│      │              │                 │                │             │
│      │              │                 │                ├─ Sonar      │
│      │              │                 │                ├─ Security   │
│      │              │                 │                ├─ Tests      │
│      │              │                 │                ├─ Lighthouse │
│      │              │                 │                └─ k6         │
│      │              │                 │                              │
│      └──────────────┴───────────────► Review Orchestrator            │
│                                         │                            │
│                       Peer A (terminal) │                            │
│                                         ├─ Peer B (OpenAI-compatible)│
│                                         └─ Arbiter C (isolated)      │
│                                                                      │
│  Score Engine → Release Report JSON → HTML Renderer → PDF Renderer  │
│                         │                                            │
│                     SQLite + files                                   │
└──────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
                 9Router / outro endpoint compatível
```

## 3.1 Não haverá REST/GraphQL de aplicação no MVP

A interface de máquina para agentes será **MCP**. A interface humana será **CLI**. O dashboard usa um servidor HTTP local temporário apenas para servir arquivos estáticos/artefatos quando solicitado.

Não criar REST e GraphQL apenas “porque pode ser útil”. Isso adicionaria superfície de manutenção sem necessidade atual.

GraphQL pode aparecer como **tecnologia detectada no projeto analisado** e como alvo do ZAP API Scan, mas não é a API interna do Solidify no MVP.

---

# 4. Linguagem e runtime

## 4.1 Core: Go

Escolha: **Go 1.27** como baseline atual de desenvolvimento, com `go.mod` explicitando a versão mínima suportada conforme testes de CI.

Razões:

- binário único;
- startup rápido;
- bom controle de concorrência;
- baixo consumo de memória;
- excelente suporte a processos externos e streaming JSON;
- cross-compilation simples;
- `go:embed` para UI estática;
- ecossistema adequado para CLI/MCP;
- evita manter Node/Python residente apenas para o core.

O core DEVE preferir standard library antes de dependências.

## 4.2 Dependências Go aprovadas inicialmente

Usar apenas quando necessário:

- `github.com/modelcontextprotocol/go-sdk`: SDK oficial MCP;
- `modernc.org/sqlite`: driver SQLite CGo-free para `database/sql`;
- nenhuma ORM no MVP.

CLI DEVE começar com `flag`/parsing próprio simples. Não adicionar Cobra/Viper automaticamente. Se a ergonomia ficar comprovadamente ruim, registrar ADR antes.

JSON DEVE usar a standard library. Em Go 1.27, manter compatibilidade do contrato serializado através de testes golden; não depender de texto exato de mensagens de erro da implementação JSON.

## 4.3 Frontend do dashboard

Escolha: **Preact + TypeScript + Vite**, compilado em build-time e embutido no binário com `go:embed`.

Objetivos:

- SPA pequena;
- API semelhante a React;
- bundle menor que uma stack React completa;
- nenhuma dependência Node no runtime do usuário.

Evitar Tailwind no MVP. Preferir CSS normal organizado por componentes/tokens. O dashboard é uma ferramenta técnica e não precisa de uma cadeia CSS pesada.

Para gráficos, começar com SVG/CSS próprios para score cards e sparklines simples. Só adicionar biblioteca de chart se houver necessidade real de interação/histórico complexo.

---

# 5. Modelo de execução: zero daemon

O Solidify NÃO DEVE manter processo residente.

Comandos típicos:

```bash
solidify init
solidify inspect --base origin/main --head HEAD
solidify dashboard --run latest
solidify report pdf --run latest
solidify mcp --stdio
```

O servidor MCP é iniciado pelo cliente/terminal apenas durante a sessão. O dashboard sobe em `127.0.0.1` e termina quando o processo é encerrado.

---

# 6. Docker-first com serviços sob demanda

## 6.1 Imagem do core

Build multi-stage:

- builder Go;
- runtime Alpine minimalista com `git`, certificados e timezone somente se necessário;
- `CGO_ENABLED=0`;
- usuário não-root;
- filesystem do container read-only onde possível;
- repositório e `.solidify` montados explicitamente.

Meta inicial: imagem comprimida do core **<= 60 MB**. Não tratar alguns MB acima como falha funcional, mas qualquer crescimento relevante deve ser explicado.

## 6.2 Docker Compose Profiles

Ferramentas pesadas ficam em perfis independentes:

- `sonar-scanner`;
- `sonar-server` — opcional e desligado por padrão;
- `lighthouse`;
- `security-sast`;
- `security-zap`;
- `load`;
- `pdf`.

O Compose Profile existe exatamente para evitar subir componentes que não se aplicam.

## 6.3 Política de recursos

O scheduler interno classifica jobs:

- `light`;
- `cpu`;
- `browser`;
- `memory-heavy`;
- `active-network-test`.

Defaults:

- jobs leves: até `min(4, NumCPU)`;
- ferramentas pesadas: **1 por vez**;
- browser jobs: **1 por vez**;
- active network tests: **1 por vez**.

Isso é proposital: a máquina do desenvolvedor pode estar simultaneamente executando IDE, Docker, banco e LLMs locais.

---

# 7. Git como fronteira de análise

## 7.1 Regra principal

A IA NÃO DEVE receber o repositório inteiro por padrão.

O primeiro artefato é o intervalo Git:

```text
base..head
```

Comandos equivalentes esperados:

- `git merge-base` quando necessário;
- `git diff --name-status -M -C`;
- `git diff --numstat`;
- `git diff --unified=<n>`;
- `git log --format=... base..head`;
- `git show base:path` e `git show head:path` para comparação contextual.

## 7.2 Tipos de mudança

Detectar:

- added;
- modified;
- deleted;
- renamed;
- copied;
- binary;
- mode change.

## 7.3 Antes e depois sem reanalisar tudo

Para SOLID, o Solidify DEVE construir evidência `before` e `after` apenas dos arquivos/símbolos afetados. Isso permite mostrar:

```text
SRP: 82 → 71  (-11)
```

sem executar uma auditoria completa de toda a branch base.

## 7.4 Context expansion bounded

A expansão pode incluir:

- imports diretos;
- interface implementada;
- classe base;
- construtor/dependências;
- símbolos referenciados no hunk;
- chamadas/usos encontrados por busca textual/simbólica;
- arquivo de teste correspondente;
- arquivo de configuração correspondente.

Cada expansão precisa registrar **por que** aquele arquivo entrou no Evidence Bundle.

Nunca expandir recursivamente sem limite.

---

# 8. Monorepo e detector de projetos

## 8.1 Objetivo

Se um commit alterar apenas um backend Java dentro de um monorepo, não rodar Lighthouse no frontend que não mudou.

## 8.2 Manifestos iniciais reconhecidos

- Node/JS/TS: `package.json`, lockfiles;
- .NET: `*.sln`, `*.csproj`, `Directory.Build.*`;
- Java: `pom.xml`, `build.gradle`, `build.gradle.kts`;
- PHP: `composer.json`;
- Python: `pyproject.toml`, `requirements*.txt`, `Pipfile`;
- Go: `go.mod`;
- Flutter/Dart: `pubspec.yaml`;
- Rust: `Cargo.toml`;
- Ruby: `Gemfile`;

## 8.3 Tipos de componente

Um componente pode ser:

- `frontend-web`;
- `backend-api`;
- `worker`;
- `library`;
- `mobile`;
- `infra`;
- `database`;
- `unknown`.

Pode haver mais de um tipo por raiz.

## 8.4 Heurísticas de frontend web

Sinais:

- React/Vue/Angular/Svelte/Next/Nuxt/Vite;
- arquivos HTML/SPA;
- scripts `dev`, `build`, `start` associados a web;
- rotas/páginas.

Lighthouse só fica `APPLICABLE` quando existe frontend web afetado **e** target executável/configurado.

---

# 9. Inventário da entrega e release notes

## 9.1 Git history

Analisar subjects e bodies de commits no intervalo.

Extrair:

- hash curto e completo;
- autor;
- data;
- subject;
- body resumido;
- issue keys detectadas;
- Conventional Commit type/scope quando houver;
- breaking change marker;
- arquivos/componentes tocados.

## 9.2 Classificação das mudanças

Categorias canônicas:

- feature;
- bugfix;
- refactor;
- performance;
- security;
- test;
- docs;
- build;
- ci;
- chore;
- migration;
- configuration;
- breaking-change;
- unknown.

Prioridade da classificação:

1. marcação determinística de Conventional Commits;
2. nomes/paths conhecidos;
3. evidência de diff;
4. IA apenas para ambiguidades.

A IA não deve converter todo `refactor` em “feature”.

## 9.3 Clusterização

Commits relacionados podem ser agrupados em uma mesma entrega lógica usando:

- mesmo scope;
- issue key;
- proximidade de arquivos;
- título semelhante;
- componente comum.

O relatório deve deixar a rastreabilidade dos commits originais disponível.

---

# 10. Migrations — requisito de primeira classe

## 10.1 Objetivo

Toda migration adicionada, alterada, removida ou renomeada dentro do intervalo DEVE aparecer no relatório.

## 10.2 Detectores iniciais

Reconhecer ao menos:

- Flyway: `V*__*.sql`, `R__*.sql`;
- Liquibase: changelog XML/YAML/JSON/SQL;
- EF Core: pasta `Migrations`, classes `Migration`, `Up`/`Down`;
- Prisma: `prisma/migrations/*/migration.sql`;
- TypeORM migrations;
- Sequelize migrations;
- Knex migrations;
- Django migrations;
- Rails `db/migrate`;
- Laravel migrations;
- migrations SQL genéricas em diretórios configuráveis.

## 10.3 Dados capturados

Para cada migration:

- engine/framework inferido;
- path;
- migration id/version;
- descrição/nome;
- change status;
- objetos afetados quando detectáveis;
- operações de schema;
- operações de dados;
- rollback/down presente;
- risco determinístico;
- observações da revisão por IA quando necessário.

## 10.4 Sinais de risco

Marcar para revisão:

- `DROP TABLE/COLUMN/INDEX`;
- redução de tamanho/tipo potencialmente incompatível;
- `NOT NULL` sem default/backfill aparente;
- rename que possa quebrar consumidores;
- update/delete massivo;
- criação de índice potencialmente bloqueante;
- migration sem rollback quando a stack normalmente o suporta;
- ordem dependente;
- migration editada depois de já existir na base, se detectável pela política do projeto.

A ferramenta NÃO executa migration destrutiva automaticamente.

---

# 11. Variáveis de ambiente e configuração operacional

## 11.1 Regra de segurança

O Solidify pode armazenar **nomes** de variáveis e metadados. NÃO PODE armazenar valores secretos.

O redactor deve bloquear valores vindos de `.env`, processo, Docker secrets e headers.

## 11.2 Fontes de detecção

Comparar base vs head em:

- `.env.example`, `.env.sample`, templates;
- Dockerfile/Compose;
- Kubernetes manifests;
- Helm values/templates;
- CI config;
- código-fonte.

Padrões de código iniciais:

- Node: `process.env.X`;
- Vite: `import.meta.env.X`;
- Go: `os.Getenv`, `os.LookupEnv`;
- .NET: `Environment.GetEnvironmentVariable`, Configuration binding;
- Java/Spring: `${ENV}`, `@Value`, config properties;
- Python: `os.environ`, `os.getenv`;
- PHP: `getenv`, `$_ENV`, `env()`;
- Ruby: `ENV[...]`;
- Dart: `String.fromEnvironment` e padrões configurados.

## 11.3 Estados

- added;
- removed;
- renamed — quando inferível;
- changed-usage;
- documentation-added;
- documentation-missing.

## 11.4 Metadados por env

- name;
- component;
- references;
- required/optional — apenas se inferível;
- has_default;
- documented;
- likely_secret — booleano por nome/contexto, sem expor valor;
- deployment impact;
- evidence references.

Exemplo de relatório:

```text
DATABASE_READ_TIMEOUT_MS — NEW — backend-api
Required: não, possui default
Documented in .env.example: sim
Deployment action: opcional
```

ou

```text
PAYMENT_PROVIDER_TOKEN — NEW — billing-worker
Likely secret: sim
Documented: nome presente, valor ausente
Deployment action: configurar secret antes da release
```

---

# 12. Evidence Bundle

## 12.1 Filosofia

Modelos avaliam fatos já coletados. Eles não devem executar exploração desordenada no repo.

## 12.2 Conteúdo

`evidence/manifest.json` referencia artefatos menores:

- Git range;
- commits;
- changed files;
- hunks;
- before/after context;
- project manifests;
- component map;
- migration changes;
- env changes;
- deterministic tool results;
- test results;
- endpoint/route changes;
- config changes;
- known limitations.

## 12.3 Token budget

Não adicionar tokenizer pesado ao core.

Controlar por bytes/chars e linhas:

- limite global configurável;
- limite por arquivo;
- limite por hunk;
- prioridade para código alterado e contratos próximos;
- anexos pagináveis por MCP.

Default inicial sugerido:

- resumo estrutural: até 20k chars;
- código principal por shard: até 60k chars;
- arquivo individual: até 12k chars antes de truncamento seletivo.

Esses números são guardrails, não promessa de tokens exatos.

## 12.4 Sharding de releases grandes

Se a evidência ultrapassar o budget:

1. particionar por componente;
2. dentro do componente, agrupar por cluster de mudança;
3. gerar shards determinísticos;
4. Peer A e Peer B recebem os **mesmos shards**;
5. arbitrar cada shard;
6. consolidar matematicamente os scores;
7. gerar uma síntese de release sobre os vereditos estruturados, não reenviando todo o código.

Isso impede custo/token explosivo em releases grandes.

---

# 13. SOLID — coração do produto

Cada princípio possui:

- `applicable`;
- `before_score` 0–100 quando mensurável;
- `after_score` 0–100;
- `delta`;
- `confidence` 0–1;
- findings;
- evidence refs;
- recommendations.

Se um princípio não tiver evidência suficiente, usar `N/A`. Nunca atribuir 100 só porque “não apareceu problema”.

## 13.1 S — Single Responsibility Principle

Observar mudanças em:

- número de razões de mudança;
- mistura de orchestration, persistence, validation, mapping, transport, logging etc.;
- crescimento de classe/módulo em responsabilidades distintas;
- funções/classes “god object” introduzidas no diff.

## 13.2 O — Open/Closed Principle

Observar:

- `switch/if` crescente por tipo/provider;
- necessidade de editar código central para cada nova variação;
- abstrações/extensões adequadas;
- registries/factories/plugins quando fazem sentido;
- overengineering não ganha ponto automaticamente.

## 13.3 L — Liskov Substitution Principle

Só aplicável quando existem subtipos/implementações/contratos afetados.

Observar:

- pré-condições fortalecidas;
- pós-condições enfraquecidas;
- throws/comportamento incompatível;
- implementação que não consegue cumprir contrato;
- métodos “not supported” em interface supostamente substituível.

## 13.4 I — Interface Segregation Principle

Observar:

- interfaces gordas;
- implementações obrigadas a depender de métodos irrelevantes;
- contratos pequenos/coerentes;
- DTO/interface transportando preocupações não relacionadas.

## 13.5 D — Dependency Inversion Principle

Observar:

- domínio/aplicação dependendo de infraestrutura concreta;
- criação direta de dependências que deveriam ser injetadas;
- abstrações na direção correta;
- service locator/global state;
- acoplamento a SDK/framework em camada de alto nível.

## 13.6 Cálculo do SOLID Score

No MVP, princípios aplicáveis têm peso igual.

```text
SOLID Score = média dos after_score dos princípios applicable=true
```

O delta é calculado sobre as mesmas letras aplicáveis no before/after.

O relatório sempre exibe as cinco letras; `N/A` permanece visível.

---

# 14. Revisão por pares com IA

## 14.1 Papéis

### Peer A — agente do terminal

É o Claude Code/Codex/outro agente que iniciou a revisão via MCP.

Ele recebe evidência estruturada e pode pedir context chunks adicionais por MCP.

### Peer B — revisor independente

É chamado pelo core em **nova requisição e novo contexto** através de provider OpenAI-compatible.

Ele recebe as mesmas evidências relevantes do Peer A, mas NÃO recebe a resposta do Peer A.

### Árbitro C

É chamado em outro contexto isolado.

Recebe:

- evidências brutas necessárias;
- saída estruturada do Peer A;
- saída estruturada do Peer B;
- divergências calculadas deterministicamente.

Seu papel é adjudicar, não simplesmente “escolher quem escreveu melhor”.

## 14.2 Independência

A execução deve registrar:

- modelo;
- provider;
- papel;
- request id quando disponível;
- prompt version;
- temperature;
- latência;
- usage reportado;
- hash da evidence input;
- hash do output.

Se Peer B e Árbitro C forem o mesmo modelo, a execução pode continuar em perfis permissivos, mas deve marcar `independence_degraded=true`.

No perfil `contractual`, a política padrão é exigir modelos distintos para B e C, salvo waiver explícito na configuração.

## 14.3 Por que o árbitro vê os dois

Peer B é revisão cega. Árbitro C não é um terceiro peer cego; ele é o **adjudicador**. Ele precisa ver os dois artefatos para explicar consenso e divergência.

## 14.4 Structured output

Todas as respostas devem validar contra JSON Schema.

Se a resposta vier inválida:

1. tentar extrair o primeiro objeto JSON válido;
2. se falhar, uma única repair request com o erro de schema;
3. se falhar novamente, marcar o ator como `FAILED`.

Nunca aceitar texto livre como nota oficial.

## 14.5 Temperatura

Reviews formais devem usar baixa variabilidade, preferencialmente `0` ou o menor valor suportado. Se o endpoint não aceitar `temperature`, registrar `unsupported`.

---

# 15. 9Router e providers OpenAI-compatible

## 15.1 Adapter genérico

O pacote interno deve se chamar conceitualmente `openai_compatible`, não `nine_router`.

Config:

- base URL;
- API key por referência a env;
- model discovery endpoint;
- chat completions endpoint;
- timeout;
- headers opcionais.

## 15.2 9Router como preset

Preset local host:

```text
http://127.0.0.1:20128/v1
```

Quando Solidify estiver dentro de Docker e 9Router no host:

```text
http://host.docker.internal:20128/v1
```

No Linux Compose, adicionar mapeamento `host-gateway` quando necessário.

9Router oferece endpoint OpenAI-compatible e `GET /v1/models`; o Solidify pode descobrir modelos sem hardcode.

## 15.3 Model discovery

Fluxo:

1. `GET /v1/models`;
2. normalizar IDs;
3. guardar snapshot e timestamp no SQLite;
4. aplicar allow/deny rules;
5. aplicar diversidade de modelo/provider se metadados permitirem;
6. executar capability probe opcional e cacheado;
7. selecionar Peer B e Árbitro C;
8. registrar exatamente por que cada um foi escolhido.

## 15.4 Capability probe

Não tentar adivinhar qualidade apenas pelo nome do modelo.

O probe curto avalia:

- JSON compliance;
- capacidade de seguir um mini contrato de scoring;
- latência;
- contexto máximo, se informado pelo endpoint;
- disponibilidade.

O probe NÃO decide “qual LLM é mais inteligente” de forma absoluta. Ele filtra incompatibilidades operacionais.

## 15.5 Modos de seleção

- `pinned`: IDs explícitos;
- `discover`: auto discovery + policy;
- `hybrid`: preferidos explícitos, fallback por discovery.

Para releases contratuais, `pinned` ou `hybrid` é recomendado por reprodutibilidade.

---

# 16. Robustez e inversão de papéis

Comando conceitual:

```bash
solidify robustness --run <id> --swap-external-models
```

Experimento:

- execução original: Model X = Peer B, Model Y = Arbiter C;
- execução invertida: Model Y = Peer B cego, Model X = Arbiter C;
- Peer A original permanece como evidência de referência;
- comparar score final, findings e gate.

Métricas:

- `score_delta`;
- `solid_letter_delta`;
- finding overlap;
- severity agreement;
- gate stability;
- recommendation overlap aproximado.

Resultado de robustez:

- stable;
- mostly-stable;
- unstable.

Isso não precisa rodar em todo commit, mas deve existir desde o desenho do contrato.

---

# 17. SonarQube

## 17.1 Papel

SonarQube fornece evidência objetiva de qualidade e segurança, mas não substitui SOLID.

Coletar, quando disponível:

- bugs;
- vulnerabilities;
- code smells;
- duplicated lines;
- complexity;
- coverage;
- quality gate;
- new-code findings quando o setup permitir.

## 17.2 Estratégia leve

Não incluir SonarQube Server como daemon obrigatório.

Ordem de preferência:

1. reutilizar servidor Sonar já configurado no projeto/empresa;
2. usar scanner sob demanda;
3. perfil opcional `sonar-server` apenas para quem realmente quiser uma instância local.

O adapter DEVE isolar a Web API, porque a SonarQube Web API evolui e endpoints V2 vêm substituindo os antigos.

## 17.3 Diff relevance

Mesmo quando o scanner precisar analisar o projeto todo, o relatório Solidify deve priorizar issues localizadas em arquivos/linhas afetadas pelo range da release e indicar claramente quais métricas são project-wide.

---

# 18. Segurança

Segurança deve ter score/seção própria e não prometer “sistema seguro”.

## 18.1 Camadas

### 18.1.1 Secrets

Recomendado: Gitleaks ou scanner equivalente, focado no range da entrega quando suportado.

### 18.1.2 SAST

- Sonar security findings;
- Semgrep Community Edition sob demanda para regras relevantes à stack.

### 18.1.3 Dependências

Recomendado: OSV-Scanner, por ser local, multiplataforma e escrito em Go, analisando lockfiles/SBOM/dependências.

### 18.1.4 DAST básico

ZAP Baseline para aplicações web executáveis: spider curto e passive scan.

### 18.1.5 Active API scan

Para SQL Injection e checks ativos mais agressivos, usar ZAP API Scan/active scan **somente** quando:

- target estiver explicitamente marcado como local/test;
- host estiver em allowlist;
- usuário/config habilitar active scan;
- houver healthcheck;
- a política permitir alteração de dados.

Nunca apontar active scan automaticamente para produção.

## 18.2 GraphQL/OpenAPI

Se o projeto expõe OpenAPI/Swagger ou GraphQL schema, o detector pode marcar ZAP API Scan como aplicável.

## 18.3 Score de segurança

Severity-based, com hard gate:

- critical confirmada na entrega → FAIL;
- high nova → FAIL por padrão em `contractual`;
- medium/low → penalidade e warning configurável.

False positive explicitamente waived precisa ficar registrado no artefato.

---

# 19. Testes e cobertura

O detector deve descobrir comandos existentes, não inventar um framework novo.

Exemplos:

- `npm test`, Vitest/Jest;
- `dotnet test`;
- Maven/Gradle test;
- `go test ./...`;
- pytest;
- PHPUnit;
- Flutter test.

Config pode sobrescrever.

Guardar:

- command;
- exit code;
- duration;
- passed/failed/skipped;
- coverage quando disponível;
- logs resumidos;
- artefatos externos referenciados.

Test failure em perfil de release é hard gate por padrão.

---

# 20. Lighthouse

## 20.1 Aplicabilidade

Rodar apenas se:

- componente alterado = frontend web;
- uma URL local/test estiver configurada ou puder ser obtida por runtime hook;
- Chrome/Chromium estiver disponível pelo perfil de ferramenta.

## 20.2 Métricas

Importar scores oficiais do Lighthouse, sem tentar recalculá-los:

- Performance;
- Accessibility;
- Best Practices;
- SEO, se desejado;
- métricas Core Web Vitals/Lab relevantes presentes no JSON.

Lighthouse usa Chrome e é relativamente pesado; por isso é job `browser` e não roda para backends puros.

---

# 21. Teste de carga com k6

## 21.1 Aplicabilidade

Somente para componentes executáveis com target configurado.

## 21.2 Escopo

Priorizar endpoints tocados pela entrega.

Fontes:

- OpenAPI diff;
- rotas/controllers alterados;
- config manual;
- scripts k6 existentes no projeto.

## 21.3 Perfis

- `smoke`: 1–2 VUs, curta duração;
- `baseline-load`: carga moderada definida pelo projeto;
- `custom`: script existente.

Não criar stress test pesado automaticamente.

## 21.4 Resultados

- request count;
- error rate;
- throughput;
- p50/p90/p95/p99 quando disponíveis;
- thresholds;
- comparação com baseline histórica quando houver.

Uma regressão só deve ser afirmada se houver baseline comparável com mesma configuração.

---

# 22. Runtime hooks para ambiente local

Config pode declarar:

- prepare command;
- start command;
- healthcheck URLs/commands;
- stop command;
- fixture/database reset command explicitamente autorizado.

Exemplo de fluxo:

```text
prepare → docker compose up → healthcheck → tests → lighthouse/zap/k6 → stop
```

O Solidify NÃO assume que pode apagar/restaurar banco sem configuração explícita.

---

# 23. Aplicabilidade e estados dos analisadores

Todo analyzer retorna um estado:

- `APPLICABLE` + execução;
- `NOT_APPLICABLE`;
- `SKIPPED`;
- `FAILED`;
- `BLOCKED`.

Semântica:

- `NOT_APPLICABLE`: não entra no denominador da nota;
- `SKIPPED`: aplicável, mas usuário/política pulou; pode reduzir completeness;
- `FAILED`: tentou e falhou;
- `BLOCKED`: pré-requisito ausente e política impede conclusão.

---

# 24. Scoring geral

## 24.1 Duas saídas diferentes

Não confundir:

- **Quality Score** 0–100;
- **Release Risk** LOW/MODERATE/HIGH/CRITICAL.

Uma migration arriscada pode elevar Release Risk sem transformar automaticamente um código bem escrito em score 20.

## 24.2 Pesos iniciais

```text
SOLID                  50
Static Code Quality    15
Security               15
Tests                   10
Performance/Load         5
Frontend/Lighthouse      5
```

Somente pilares aplicáveis entram no denominador. Portanto, se Lighthouse for N/A, os demais pesos são normalizados proporcionalmente.

SOLID permanece naturalmente dominante.

## 24.3 Nota final

```text
quality_score = sum(score_i * weight_i) / sum(weight_i applicable)
```

Arredondamento apenas para apresentação; cálculos internos preservam casas decimais.

## 24.4 Grade visual

Sugestão inicial:

- A: 90–100;
- B: 80–89.99;
- C: 70–79.99;
- D: 60–69.99;
- E: <60.

Quality Gate é separado da grade. Uma release pode ter 92 e ainda `FAIL` por uma vulnerabilidade crítica.

---

# 25. Quality Gate

Estados:

- PASS;
- PASS_WITH_WARNINGS;
- FAIL;
- BLOCKED;
- INCOMPLETE.

Regras padrão `contractual`:

- Peer A ausente → BLOCKED;
- Peer B ausente → BLOCKED;
- Árbitro C ausente → BLOCKED;
- JSON canônico inválido → BLOCKED;
- testes aplicáveis falhando → FAIL;
- critical security finding nova → FAIL;
- high security finding nova → FAIL;
- quality score < 70 → FAIL;
- SOLID score < 65 → FAIL;
- migration critical risk não reconhecida/waived → FAIL;
- analyzer obrigatório SKIPPED → INCOMPLETE/BLOCKED conforme política.

Tudo configurável, mas defaults devem ser conservadores para laudo contratual.

---

# 26. Confidence Score

Confiança NÃO é a mesma coisa que qualidade.

Componentes:

- evidence completeness;
- peer agreement;
- arbiter confidence;
- analyzer completeness;
- model independence;
- schema compliance;
- robustness result se executado.

Faixas:

- HIGH;
- MEDIUM;
- LOW.

Exemplo: código pode obter Quality 88 com Confidence MEDIUM porque o segundo peer falhou.

---

# 27. Persistência

## 27.1 SQLite

Arquivo:

```text
.solidify/solidify.db
```

SQLite guarda metadados e índices, não blobs grandes.

Usar `database/sql` + `modernc.org/sqlite`, sem ORM.

Ativar pragmas adequados via testes; WAL é recomendado para robustez quando dashboard e análise puderem ler/escrever concomitantemente.

## 27.2 Tabelas iniciais

### runs

- id;
- repo_root_hash/path local;
- base_ref/head_ref/base_sha/head_sha;
- profile;
- status;
- started_at/finished_at;
- quality_score;
- solid_score;
- confidence;
- gate;
- report_path.

### components

- id;
- run_id;
- root;
- type;
- stack;
- changed_files_count.

### analyzer_results

- id;
- run_id;
- analyzer;
- version;
- status;
- score;
- duration_ms;
- artifact_path;
- error_summary.

### reviews

- id;
- run_id;
- role;
- provider;
- model_id;
- prompt_version;
- evidence_hash;
- output_hash;
- latency_ms;
- usage_json;
- status;
- artifact_path.

### models

- provider_alias;
- model_id;
- discovered_at;
- metadata_json;
- probe_json;
- probe_expires_at.

### findings

Metadados para filtro/dashboard; detalhes permanecem no artifact JSON.

### artifacts

- type;
- path;
- sha256;
- size_bytes.

## 27.3 Artefatos grandes no filesystem

Nunca colocar PDF, HTML, logs inteiros ou diffs completos no SQLite.

---

# 28. Estrutura de arquivos do projeto

```text
solidify/
  cmd/
    solidify/
      main.go
  internal/
    app/
    config/
    gitx/
    scope/
    detect/
      project/
      migration/
      envchange/
      route/
    evidence/
    analyzer/
      runner/
      sonar/
      tests/
      lighthouse/
      security/
      load/
    ai/
      provider/
        openaicompat/
      discovery/
      review/
      arbiter/
      prompts/
    mcpserver/
    score/
    gate/
    storage/
      sqlite/
      artifacts/
    report/
      canonical/
      html/
      pdf/
    runtimehooks/
    scheduler/
    redaction/
    version/
  web/
    src/
    public/
    package.json
    vite.config.ts
  schemas/
  prompts/
  migrations/           # migrations do SQLite do próprio Solidify
  testdata/
  compose.yaml
  Dockerfile
  Makefile
  go.mod
```

---

# 29. Plugin architecture sem peso desnecessário

## 29.1 Built-ins

Analisadores essenciais são interfaces Go internas.

```go
type Analyzer interface {
    ID() string
    Detect(ctx context.Context, input DetectInput) Applicability
    Run(ctx context.Context, input AnalyzeInput) (Result, error)
}
```

## 29.2 Plugins externos futuros

Não usar Go `.so` plugins; têm limitações de portabilidade/versionamento.

Definir protocolo de processo externo:

- executable;
- JSON request via stdin;
- JSON response via stdout;
- timeout;
- version handshake.

Isso permite plugin em qualquer linguagem sem aumentar o core.

---

# 30. MCP

Usar SDK oficial Go e a especificação MCP atual suportada pelo SDK fixado no projeto.

Transporte MVP: **stdio**.

Razões:

- simples para Claude Code/Codex;
- sem porta permanente;
- reduz superfície de segurança;
- combina com lifecycle do terminal.

Ferramentas MCP detalhadas em `CONTRACTS.md`.

---

# 31. Dashboard

## 31.1 Execução

```bash
solidify dashboard --run latest
```

Servidor:

- bind `127.0.0.1` por padrão;
- porta aleatória ou configurável;
- read-only para report artifacts no MVP;
- sem autenticação porque não deve bindar externamente.

## 31.2 Telas

### Overview

- gate;
- Quality Score;
- SOLID cards S/O/L/I/D;
- confidence;
- release risk;
- entrega resumida;
- principais findings.

### SOLID

- before/after;
- delta;
- evidências;
- Peer A vs Peer B vs Arbiter;
- recommendations.

### Release

- features/bugfixes;
- commits;
- changed components;
- migrations;
- env changes;
- breaking changes.

### Quality

- Sonar;
- tests;
- duplication/complexity;
- coverage.

### Security

- SAST;
- dependencies;
- secrets;
- ZAP;
- waivers.

### Performance

- k6;
- Lighthouse se aplicável;
- baseline comparison.

### AI Review

- modelos;
- papéis;
- prompt versions;
- consenso/divergência;
- independence status;
- robustness experiment.

### History

- score por release;
- SOLID trends;
- gate history.

---

# 32. PDF empresarial

## 32.1 Princípio

O PDF usa a mesma view-model do dashboard, baseada em `release-report.json`.

## 32.2 Geração

Preferência:

1. localizar Chrome/Chromium disponível;
2. renderizar rota HTML em modo print;
3. usar headless print-to-PDF;
4. se browser não existir no host, subir perfil Docker `pdf` temporário;
5. encerrar container ao finalizar.

Não manter engine PDF paralela em Go no MVP; duplicaria layout e lógica.

## 32.3 Estrutura do PDF

1. capa;
2. identificação da release;
3. parecer executivo;
4. Quality Gate e scores;
5. SOLID detalhado;
6. escopo entregue/release notes;
7. qualidade estática;
8. testes;
9. segurança;
10. performance/Lighthouse;
11. migrations;
12. env/config changes;
13. riscos e recomendações;
14. revisão por pares/IA transparency;
15. rastreabilidade Git;
16. limitações e disclaimers.

## 32.4 Linguagem do parecer

Profissional e verificável. Evitar frases absolutas como:

- “o sistema está seguro”;
- “não existem bugs”;
- “a arquitetura está perfeita”.

Usar:

- “não foram detectadas vulnerabilidades críticas pelos checks executados”;
- “os testes executados passaram”;
- “a análise encontra-se limitada ao intervalo Git informado”.

---

# 33. Cache

Cache key conceitual:

```text
SHA256(
  analyzer_id + analyzer_version + config_hash +
  base_sha + head_sha + relevant_input_hash
)
```

Pode reutilizar:

- model discovery/probes;
- deterministic detector results;
- tool outputs quando inputs idênticos;
- rendered HTML/PDF quando report hash idêntico.

Nunca reutilizar review de IA para um evidence hash diferente.

---

# 34. Observabilidade local

Log estruturado JSON opcional + console humano.

Cada step registra:

- start/end;
- duration;
- status;
- resource class;
- tool version;
- artifact produced;
- error sanitized.

Sem telemetry externa no MVP.

---

# 35. Segurança do próprio Solidify

- bind local only;
- API keys somente por env/secret store externo;
- redaction antes de logs e prompts;
- command execution allowlisted pela config;
- runtime hooks exibidos no `init` e versionados no repo;
- nunca executar comandos descobertos em commit message;
- paths normalizados para evitar escape do repo/artifact dir;
- tool containers com mounts read-only quando possível;
- active security target allowlist;
- relatório não contém env values;
- evidence artifacts podem conter código e ficam locais.

---

# 36. Reproducibilidade

Cada report registra:

- Solidify version/commit;
- OS/arch;
- base/head SHA;
- analyzer versions;
- config hash;
- prompt versions;
- model/provider IDs;
- evidence hash;
- report schema version;
- timestamps;
- waivers.

Isso permite comparar duas execuções e entender por que resultados mudaram.

---

# 37. Performance budgets

Metas iniciais do core, excluindo ferramentas externas:

- startup binário: alvo < 300 ms em máquina moderna;
- RSS típico em coleta Git pequena/média: alvo < 100 MB;
- nenhuma cópia integral desnecessária do diff em memória;
- streaming de JSON/logs quando arquivos forem grandes;
- dashboard idle: alvo < 100 MB somando core e browser do usuário, sem browser headless interno;
- core image: alvo <= 60 MB comprimida;
- zero processos após comando terminar.

Não transformar metas em números falsamente universais. CI terá benchmark relativo e alertará regressão percentual.

## 37.1 Regression budget do próprio projeto

Criar benchmarks para:

- parse de 1k changed files;
- diff de 10 MB;
- env detector;
- migration detector;
- JSON report assembly;
- SQLite inserts/query history.

Falhar CI se benchmark de microcomponente crítico piorar acima de limite acordado em execução estável, ou ao menos publicar comparação no MVP inicial.

---

# 38. Estratégia de compatibilidade

## 38.1 Ferramentas externas

Cada adapter tem:

- `DetectVersion()`;
- range testado;
- parser versioned;
- fixture de output real;
- fallback claro para `unsupported`.

Não parsear output humano quando existir JSON/SARIF oficial.

## 38.2 Formatos preferidos

- Sonar: Web API JSON;
- Lighthouse: JSON report;
- k6: JSON/summary export;
- Semgrep: JSON/SARIF;
- OSV-Scanner: JSON;
- ZAP: JSON/XML quando disponível;
- tests: JUnit/TRX/coverage formats quando existentes.

---

# 39. Decisões explicitamente rejeitadas no MVP

- microservices para o próprio Solidify;
- PostgreSQL;
- Redis;
- Kafka/queue;
- REST + GraphQL paralelo ao MCP;
- daemon obrigatório;
- armazenamento de diffs no banco;
- framework de plugin pesado;
- indexação vetorial do repo;
- LLM local obrigatório dentro do Solidify;
- API paga obrigatória;
- SonarQube Server sempre ligado;
- Lighthouse em qualquer commit sem detecção de frontend;
- active ZAP contra targets não allowlisted;
- reanalisar todo repo com IA a cada release;
- PDF com template independente do dashboard.

---

# 40. Critérios de sucesso do MVP

O MVP está tecnicamente completo quando consegue demonstrar, em um repositório exemplo com frontend + backend:

1. comparar `base..head`;
2. detectar os dois componentes e identificar qual foi alterado;
3. classificar commits em release notes;
4. detectar migration adicionada;
5. detectar env adicionada sem capturar valor;
6. produzir Evidence Bundle limitado;
7. rodar testes aplicáveis;
8. consumir Sonar quando configurado;
9. rodar Lighthouse apenas quando frontend alterado;
10. rodar security checks configurados;
11. rodar k6 apenas quando backend/target aplicável;
12. receber Peer A por MCP;
13. executar Peer B via OpenAI-compatible/9Router;
14. executar Árbitro C isolado;
15. gerar S/O/L/I/D com evidence refs e before/after;
16. calcular Quality Score, Confidence e Release Risk;
17. aplicar Quality Gate;
18. gerar JSON canônico válido;
19. renderizar dashboard;
20. gerar PDF a partir da mesma fonte;
21. registrar modelos/papéis no relatório;
22. encerrar sem deixar serviços residentes.

---

# 41. Referências técnicas validadas em 2026-08-19

As decisões foram checadas contra documentação oficial atual:

- Go 1.27 é a release mais recente em agosto de 2026;
- MCP possui especificação 2026-07-28 e SDK oficial Go com suporte correspondente;
- SonarQube expõe Web API e está migrando gradualmente endpoints para Web API V2;
- Lighthouse roda via CLI/Node e requer Chrome; Lighthouse 13 requer Node 22.19+ no fluxo Node;
- Docker Compose Profiles permitem ativar serviços apenas sob demanda;
- k6 suporta execução local/containerizada;
- ZAP Baseline é passivo/curto e ZAP API/Full Scan são ativos;
- 9Router expõe endpoint OpenAI-compatible local e `GET /v1/models`;
- `modernc.org/sqlite` fornece driver `database/sql` CGo-free;
- OSV-Scanner fornece scanner de dependências local e binário Go.

Versões específicas dos tools devem ser pinadas no lock/config de build do projeto e atualizadas por PR, não por `latest` silencioso em releases reprodutíveis.


<!-- END ARQUITETURA: ARCHITECTURE.md -->


---

<!-- BEGIN CONTRATOS: CONTRACTS.md -->

# Solidify — Contratos de CLI, MCP, IA, Scoring e Artefatos

## 1. Convenções gerais

- IDs de execução: UUIDv7/identificador temporal ordenável.
- Timestamps: RFC 3339 UTC.
- Scores: `number` 0–100, sem arredondar internamente.
- Confidence: `number` 0–1.
- Paths em artefatos: relativos ao repo quando possível.
- Hash: SHA-256 em hexadecimal.
- JSON canônico: UTF-8, chaves estáveis nos golden tests.
- Schema version: SemVer independente da versão do binário.

---

# 2. CLI

## 2.1 `solidify init`

Cria:

```text
.solidify/
solidify.json
```

Não sobrescrever config existente sem `--force`.

Saída humana:

- repo detectado;
- componentes detectados;
- ferramentas encontradas;
- sugestões de targets/runtime hooks;
- provider de IA detectado, se houver;
- avisos de segurança.

Opções:

```text
--profile quick|release|contractual
--non-interactive
--force
```

## 2.2 `solidify inspect`

Executa apenas descoberta/diff/evidence deterministicamente.

```bash
solidify inspect --base origin/main --head HEAD
```

Opções:

```text
--base <git-ref>
--head <git-ref>
--commit <sha>          # equivale parent..commit
--output json|text
--no-cache
```

## 2.3 `solidify analyze`

Executa analyzers objetivos aplicáveis, mas não inventa Peer A.

```bash
solidify analyze --base origin/main --head HEAD --profile release
```

Se perfil exigir Peer A e ele não foi submetido, status final do run fica `AWAITING_PEER_A`.

## 2.4 `solidify dashboard`

```bash
solidify dashboard --run latest
```

Opções:

```text
--run <id|latest>
--port 0
--no-open
```

Bind padrão: `127.0.0.1`.

## 2.5 `solidify report`

```bash
solidify report json --run latest
solidify report html --run latest
solidify report pdf --run latest
```

`json` sempre valida o schema antes de escrever.

## 2.6 `solidify models`

```bash
solidify models discover --provider judge
solidify models probe --provider judge
solidify models select --provider judge
```

Saída inclui apenas IDs/metadados; nunca API key.

## 2.7 `solidify robustness`

```bash
solidify robustness --run latest --swap-external-models
```

Requer dois modelos externos distintos ou retorna `BLOCKED`.

## 2.8 `solidify mcp --stdio`

Inicia servidor MCP em stdio, sem banner em stdout que quebre JSON-RPC. Logs vão para stderr.

---

# 3. MCP tools

Nomes usam namespace `solidify_` para evitar colisão.

## 3.1 `solidify_begin_review`

### Input

```json
{
  "base": "origin/main",
  "head": "HEAD",
  "profile": "contractual",
  "force_new_run": false
}
```

### Output

```json
{
  "run_id": "...",
  "status": "AWAITING_PEER_A",
  "base_sha": "...",
  "head_sha": "...",
  "evidence_manifest_ref": "evidence/manifest.json",
  "changed_components": [],
  "applicable_analyzers": [],
  "peer_contract": {
    "schema_version": "1.0.0",
    "prompt_version": "peer-review-v1"
  }
}
```

Semântica:

- coleta/recupera cache;
- roda analyzers conforme perfil;
- não chama Peer B antes de receber Peer A, salvo modo CLI específico futuro.

## 3.2 `solidify_get_evidence_manifest`

Input: `run_id`.

Retorna resumo estrutural e refs, não todo o código.

## 3.3 `solidify_get_diff_chunk`

Input:

```json
{
  "run_id": "...",
  "component_id": "backend",
  "file": "src/...",
  "chunk_id": "hunk-3",
  "view": "before_after"
}
```

Output limitado, com line numbers e hash.

## 3.4 `solidify_get_context`

Permite ao Peer A pedir contexto adicional, mas somente em paths dentro do repo e respeitando budget.

```json
{
  "run_id": "...",
  "path": "src/Foo.cs",
  "start_line": 1,
  "end_line": 180,
  "reason": "need interface contract for LSP evaluation"
}
```

A razão fica auditada.

## 3.5 `solidify_find_symbol_usages`

Busca bounded por símbolo/string no componente relevante.

Não implementa indexador semântico global no MVP. Pode usar busca textual otimizada e adapters por linguagem posteriormente.

## 3.6 `solidify_get_analyzer_result`

Input:

```json
{"run_id":"...","analyzer":"sonar"}
```

Retorna normalized result; logs brutos ficam por ref.

## 3.7 `solidify_get_release_inventory`

Retorna commits classificados, migrations, env changes, routes e components.

## 3.8 `solidify_submit_peer_review`

### Input

```json
{
  "run_id": "...",
  "actor": {
    "client": "claude-code",
    "model_id": "reported-model-id",
    "provider": "reported-provider",
    "identity_source": "client-reported"
  },
  "review": {}
}
```

`review` DEVE validar `schemas/peer-review.schema.json`.

### Side effects

1. salva Peer A;
2. valida evidence hash;
3. em perfil configurado, chama Peer B em contexto novo;
4. calcula divergências;
5. chama Árbitro C em contexto novo;
6. executa Score Engine/Gate;
7. gera `release-report.json`;
8. gera HTML; PDF se perfil exigir;
9. retorna status final e paths.

## 3.9 `solidify_get_final_report`

Retorna overview do JSON final e refs para artefatos.

---

# 4. Contrato do Peer A/Peer B

## 4.1 Invariantes

O peer DEVE:

- avaliar somente o evidence scope;
- citar evidence refs para toda penalização relevante;
- distinguir `N/A` de “bom”;
- avaliar before e after;
- não penalizar preferência estilística como violação SOLID;
- não premiar abstração desnecessária;
- não inventar testes/resultados;
- produzir JSON válido.

## 4.2 Finding SOLID

```json
{
  "id": "S-001",
  "principle": "S",
  "severity": "medium",
  "title": "Responsabilidades de persistência e notificação foram combinadas",
  "file": "src/BillingService.cs",
  "line_start": 72,
  "line_end": 118,
  "evidence_refs": ["git:hunk:...", "context:..."],
  "rationale": "...",
  "recommendation": "...",
  "introduced_by_release": true
}
```

Severity:

- info;
- low;
- medium;
- high;
- critical.

Critical em SOLID deve ser raro; representa quebra arquitetural de alto impacto, não “classe longa”.

## 4.3 Score por letra

```json
{
  "principle": "D",
  "applicable": true,
  "before_score": 86,
  "after_score": 62,
  "confidence": 0.91,
  "summary": "A aplicação passou a instanciar diretamente o adapter concreto.",
  "findings": ["D-001"]
}
```

Se não aplicável:

```json
{
  "principle": "L",
  "applicable": false,
  "before_score": null,
  "after_score": null,
  "confidence": 0.96,
  "summary": "O diff não altera hierarquias, implementações ou contratos substituíveis.",
  "findings": []
}
```

---

# 5. Divergence Engine

Antes do árbitro, o core calcula divergências sem LLM.

Por letra:

```text
abs(peerA.after - peerB.after)
```

Thresholds iniciais:

- <= 5: agreement;
- >5 e <=15: mild divergence;
- >15 e <=30: material divergence;
- >30: severe divergence.

Também comparar:

- applicable disagreement;
- severity mismatch;
- findings em mesmos arquivos/linhas;
- gate-affecting disagreement.

O árbitro recebe esse mapa.

---

# 6. Contrato do Árbitro C

O árbitro não deve tirar média cega.

Para cada princípio:

1. verificar se a evidência suporta A;
2. verificar se suporta B;
3. decidir aplicabilidade;
4. decidir score final;
5. registrar quais findings foram accepted/rejected/merged;
6. explicar divergência material.

Exemplo:

```json
{
  "principle": "O",
  "final_score": 74,
  "confidence": 0.88,
  "decision": "peer_b_preferred",
  "accepted_findings": ["B:O-002"],
  "rejected_findings": ["A:O-001"],
  "reason": "O switch citado por A existia no before state e não foi introduzido pela release."
}
```

O árbitro não pode eliminar finding objetivo de ferramenta externa. Ele arbitra análise IA/SOLID e classificação semântica; Sonar/ZAP/testes mantêm seus próprios fatos.

---

# 7. Provider OpenAI-compatible

## 7.1 Interface interna

```go
type ChatProvider interface {
    ListModels(ctx context.Context) ([]Model, error)
    CompleteJSON(ctx context.Context, req ReviewRequest) (ReviewResponse, error)
}
```

## 7.2 Config de segredo

Config salva:

```json
{
  "api_key_env": "SOLIDIFY_9ROUTER_API_KEY"
}
```

Nunca:

```json
{"api_key":"sk-..."}
```

## 7.3 9Router preset

Host binary:

```text
base_url = http://127.0.0.1:20128/v1
```

Core Docker:

```text
base_url = http://host.docker.internal:20128/v1
```

Discovery usa `/models`; completions usam `/chat/completions` inicialmente por compatibilidade ampla.

## 7.4 Request behavior

- timeout por role;
- retry apenas em falhas transitórias;
- no retry cego em 4xx de schema/model;
- streaming não é necessário para review formal;
- `response_format`/structured output usado quando endpoint/model suportar;
- caso contrário, JSON prompt + schema validation.

---

# 8. Model selection

Config conceitual:

```json
{
  "mode": "hybrid",
  "peer_b": {
    "preferred": ["provider/model-x"],
    "exclude": []
  },
  "arbiter": {
    "preferred": ["provider/model-y"],
    "must_differ_from_peer_b": true
  }
}
```

Discovery NÃO deve escolher modelo apenas por substring “reasoning”.

Selection order:

1. preferred disponível + probe OK;
2. candidate allowlisted + probe OK;
3. fallback disponível com JSON compliance;
4. BLOCKED se policy strict exigir modelo e nenhum candidato for válido.

Registrar `selection_reason`.

---

# 9. Analyzer normalized result

Todo analyzer converge para:

```json
{
  "id": "lighthouse",
  "version": "...",
  "status": "APPLICABLE",
  "execution_status": "PASSED",
  "scope": ["web-app"],
  "score": 91.2,
  "duration_ms": 12430,
  "findings": [],
  "metrics": {},
  "raw_artifact": "evidence/lighthouse.json",
  "limitations": []
}
```

`status` de aplicabilidade é separado de `execution_status`.

---

# 10. Git/release inventory contract

```json
{
  "range": {
    "base_ref": "origin/main",
    "base_sha": "...",
    "head_ref": "HEAD",
    "head_sha": "..."
  },
  "commits": [
    {
      "sha": "...",
      "subject": "fix(billing): prevent duplicate invoice",
      "type": "bugfix",
      "scope": "billing",
      "issues": ["ABC-123"],
      "breaking": false,
      "components": ["backend"]
    }
  ],
  "change_groups": []
}
```

---

# 11. Migration contract

```json
{
  "id": "V20260819_01",
  "path": "db/migration/V20260819_01__add_status.sql",
  "framework": "flyway",
  "status": "added",
  "operations": [
    {
      "kind": "add_column",
      "object": "invoice.status",
      "destructive": false
    }
  ],
  "rollback": {
    "present": false,
    "required_by_framework": false
  },
  "risk": "moderate",
  "evidence_refs": []
}
```

---

# 12. Env change contract

```json
{
  "name": "PAYMENT_TIMEOUT_MS",
  "status": "added",
  "components": ["backend"],
  "required": false,
  "has_default": true,
  "documented": true,
  "likely_secret": false,
  "references": [
    {"path":"src/config.ts","line":14}
  ],
  "deployment_action": "none-if-default-accepted"
}
```

Para secret-like env:

```json
{
  "name": "PAYMENT_API_TOKEN",
  "likely_secret": true,
  "value": "PROHIBITED"
}
```

O campo `value` não deve existir no schema real; exemplo acima é apenas uma proibição explícita.

---

# 13. Scoring contracts

## 13.1 SOLID score

Usar score do árbitro quando presente.

Fallback:

- Peer A + Peer B sem árbitro em perfil permissivo: média apenas se divergence <= threshold; caso contrário Confidence LOW e gate pode exigir revisão manual;
- somente Peer A: score permitido em `quick`, nunca classificado como peer-reviewed.

## 13.2 Static Quality

Normalizar Sonar/linters para 0–100 por política configurável.

Evitar algoritmo “mágico” baseado em quantidade absoluta sem considerar severidade e new-code scope.

Default sugerido:

```text
100
- critical issue: 30 cada
- high: 15 cada
- medium: 5 cada
- low: 1 cada
minimum 0
```

Quando Sonar fornece Quality Gate, gate original aparece separadamente.

## 13.3 Security

Severity penalties + hard gates. Não misturar CVSS de dependência e alerta de header como se fossem equivalentes; normalizar por categoria e depois compor.

## 13.4 Tests

- test failure: 0 + gate failure;
- all pass + coverage available: score pode considerar coverage delta;
- all pass sem coverage: score base configurável, ex. 85, sem fingir cobertura 100;
- no tests applicable: N/A;
- tests applicable skipped: SKIPPED e completeness penalty.

## 13.5 Lighthouse

Usar score composto configurável a partir das categorias oficiais, preservando as notas originais no report.

Default Solidify frontend pillar:

```text
Performance      50%
Accessibility    25%
Best Practices   25%
SEO              informational by default
```

## 13.6 Load

Score baseado em thresholds definidos, não em “quanto menor a latência, melhor” sem SLO.

Sem baseline/threshold: reportar métricas, mas pilar pode ficar `UNSCORED`.

---

# 14. Quality Score formula

Default weights:

```json
{
  "solid": 50,
  "static_quality": 15,
  "security": 15,
  "tests": 10,
  "performance": 5,
  "frontend": 5
}
```

Pseudo:

```text
weighted_sum = 0
applicable_weight = 0
for pillar in pillars:
  if pillar.scored:
    weighted_sum += pillar.score * pillar.weight
    applicable_weight += pillar.weight
quality = weighted_sum / applicable_weight
```

`SKIPPED` aplicável não deve ser convertido para N/A. Ele afeta completeness/gate conforme policy.

---

# 15. Release Risk

Risk factors não são simples score arithmetic.

Exemplos:

- destructive migration;
- breaking API change;
- nova env obrigatória sem documentação;
- auth/security-sensitive code;
- grande blast radius;
- muitos arquivos/shared core;
- load regression;
- review disagreement material;
- test coverage gap.

Engine determinístico produz base risk; árbitro pode explicar, mas não reduzir um hard risk sem waiver/evidence.

---

# 16. Waivers

Formato:

```json
{
  "id": "W-2026-001",
  "finding_ref": "security:OSV-...",
  "reason": "Dependency is not reachable in deployed component",
  "approved_by": "local-config/user",
  "expires_at": "2026-09-30T00:00:00Z"
}
```

MVP não precisa sistema de identidade. Apenas registrar que waiver foi declarado localmente. Não apagar finding; marcar waived.

---

# 17. Artefato canônico

`release-report.json` é imutável depois de finalizado. Para rerun, gerar novo `run_id`.

Conteúdo mínimo:

- identity;
- git range;
- components;
- release notes;
- migrations;
- env changes;
- analyzers;
- SOLID;
- AI review transparency;
- scores;
- risk;
- quality gate;
- recommendations;
- limitations;
- artifact hashes.

Schema em `schemas/release-report.schema.json`.

---

# 18. HTML contract

O HTML não lê SQLite para montar o conteúdo de uma release já finalizada. Ele recebe/serve `release-report.json`.

Isso permite:

- abrir artefato histórico;
- exportar pacote;
- evitar divergência entre DB e laudo.

History view pode consultar SQLite separadamente.

---

# 19. PDF contract

PDF contém no rodapé:

- run id;
- base short SHA;
- head short SHA;
- generated_at;
- page X/Y.

Anexo de IA deve mostrar:

| Papel | Modelo | Provider | Independente? |
|---|---|---|---|
| Peer A | ... | terminal | sim |
| Peer B | ... | 9Router/... | sim |
| Arbiter C | ... | 9Router/... | sim |

Também registrar prompt version e evidence hash em apêndice técnico.

---

# 20. Applicability matrix inicial

| Sinal da entrega | Sonar | Tests | Security SAST | Lighthouse | ZAP | k6 | Migration | Env |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| frontend web alterado | yes | yes | yes | yes | target-dependent | optional | detect | detect |
| backend API alterado | yes | yes | yes | no | target-dependent | target-dependent | detect | detect |
| library alterada | yes | yes | yes | no | no | no | detect | detect |
| docs only | optional/no | no | low/no | no | no | no | detect | detect |
| infra only | rules-dependent | tests-dependent | yes | no | no | no | detect | detect |

A matriz é heurística; config pode sobrescrever.

---

# 21. Failure semantics

## Tool not installed

- se não aplicável: N/A;
- aplicável em quick: SKIPPED + instruction;
- obrigatório em contractual: BLOCKED.

## Tool returns parser-unknown version

- salvar raw output;
- status FAILED/UNSUPPORTED;
- nunca estimar score.

## AI timeout

- retry transitório conforme policy;
- se Peer B/Arbiter obrigatório: BLOCKED;
- manter Peer A e evidências para diagnóstico.

## Invalid JSON from model

- single repair attempt;
- depois FAILED.

---

# 22. Prompt/version contract

Prompts ficam versionados como arquivos do repo e entram no hash da execução.

Mudança semântica em prompt:

- incrementa prompt version;
- deve ter regression fixtures;
- não reescreve reports antigos.

---

# 23. Golden test fixtures obrigatórias

Criar repos sintéticos/fixtures para:

1. SRP piora;
2. OCP melhora com Strategy/Adapter bem aplicado;
3. LSP N/A;
4. ISP violado;
5. DIP violado;
6. frontend-only → Lighthouse applicable;
7. backend-only → Lighthouse N/A;
8. migration destructive;
9. env secret-like adicionada sem valor exposto;
10. Conventional Commit release notes;
11. Peer A/B concordam;
12. Peer A/B divergem;
13. arbiter rejeita finding sem evidência;
14. invalid AI JSON;
15. 9Router model discovery fixture;
16. tool skipped vs N/A;
17. contractual blocked por missing arbiter;
18. role-swap stable/unstable.


<!-- END CONTRATOS: CONTRACTS.md -->


---

<!-- BEGIN RELATÓRIO E DASHBOARD: REPORT_SPEC.md -->

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


<!-- END RELATÓRIO E DASHBOARD: REPORT_SPEC.md -->


---

<!-- BEGIN TASKS: TASKS.md -->

# Solidify — Plano Task-Driven de Implementação

## Regras de execução para o agente

1. Executar tarefas em ordem dentro de cada fase.
2. Não implementar tarefa futura “aproveitando que está no arquivo”.
3. Cada tarefa termina com testes verdes e commit próprio quando possível.
4. Nenhuma mudança arquitetural silenciosa; divergências exigem ADR.
5. Não adicionar dependência sem justificar tamanho, manutenção e alternativa stdlib.
6. Manter `CGO_ENABLED=0` para o core.
7. Medir startup/binário/RSS em checkpoints definidos.
8. Todo parser externo precisa de fixture JSON real/sintética e golden test.
9. Toda feature que toca secrets precisa de teste de redaction.
10. Perfil `contractual` deve falhar fechado quando faltar requisito obrigatório.

---

# Fase 0 — Bootstrap e guardrails

## SAI-001 — Inicializar módulo Go e estrutura mínima

**Objetivo:** criar projeto compilável conforme `ARCHITECTURE.md`.

**Implementar:**

- `go.mod` com Go 1.27;
- `cmd/solidify/main.go`;
- diretórios internos essenciais;
- package `version`;
- `solidify version`;
- Makefile mínimo: `build`, `test`, `lint`, `bench`.

**Aceite:**

- `go test ./...` passa;
- `CGO_ENABLED=0 go build ./cmd/solidify` passa;
- nenhuma dependência externa ainda.

## SAI-002 — CI local e quality baseline do próprio Solidify

**Implementar:**

- `go test -race` quando plataforma suportar;
- `go vet`;
- `go test -bench` pacote benchmark;
- verificação de `gofmt`;
- medição de tamanho do binário.

**Aceite:** script único `make verify` retorna não-zero em falha.

## SAI-003 — Logging estruturado e erros tipados

**Implementar:**

- logger em stderr;
- níveis;
- JSON opcional;
- error codes estáveis para CLI;
- nenhum log de secret.

**Testes:** golden de erro humano e JSON.

## SAI-004 — Config loader JSON

**Implementar:**

- `solidify.json`;
- defaults;
- validação;
- config hash;
- referências `*_env` para secrets;
- override por flags.

**Não implementar:** YAML/TOML.

**Aceite:** config inválida mostra path/campo; secret value nunca é serializado.

## SAI-005 — `solidify init`

**Implementar:**

- detectar repo Git;
- criar `.solidify` e config inicial;
- não sobrescrever por padrão;
- saída `--non-interactive`.

**Aceite:** rodar duas vezes é idempotente.

---

# Fase 1 — Git scope e inventário

## SAI-006 — Resolver base/head/commit

**Implementar:**

- refs → SHA;
- commit isolado → parent..commit;
- merge-base opcional configurável;
- validar que objetos existem.

**Testes:** repo temporário com branch divergente.

## SAI-007 — Changed files robusto

**Implementar:**

- added/modified/deleted/renamed/copied;
- numstat;
- binary;
- mode change;
- `-M/-C`.

**Aceite:** fixture com rename não aparece como delete+add quando Git reconhece rename.

## SAI-008 — Diff hunks before/after

**Implementar:**

- hunks com line ranges;
- conteúdo limitado;
- refs estáveis;
- hash por hunk;
- acesso a `git show base:path` e `head:path`.

**Performance:** streaming; não duplicar diff inteiro em estruturas redundantes.

## SAI-009 — Git log do range

**Implementar:** metadados, Conventional Commits, breaking markers, issue key regex configurável.

**Testes:** feat/fix/refactor/perf/security/chore/breaking.

## SAI-010 — Release classification determinística

**Implementar:** classificação de commits + path hints, sem IA.

**Aceite:** categorias canônicas de `CONTRACTS.md`.

---

# Fase 2 — Detector de componentes/stack

## SAI-011 — Manifest discovery bounded

**Implementar:** localizar manifests apenas nos ancestrais/diretórios relacionados aos changed files e roots configuradas.

**Não fazer:** walk integral de monorepo enorme sem necessidade.

## SAI-012 — Stack detector

**Implementar:** Node, .NET, Java, PHP, Python, Go, Flutter/Dart, Rust, Ruby.

**Output:** stack + confidence + evidence.

## SAI-013 — Component classifier

**Implementar:** frontend-web/backend-api/worker/library/mobile/infra/database/unknown.

**Testes:** monorepo frontend + backend; commit backend-only.

## SAI-014 — Applicability engine v1

**Implementar:** regras para Sonar/tests/security/Lighthouse/ZAP/k6/migration/env.

**Aceite crítico:** backend-only => Lighthouse `NOT_APPLICABLE`.

---

# Fase 3 — Migrations e envs

## SAI-015 — Migration detector framework-aware

**Implementar:** Flyway, Liquibase, EF Core, Prisma, TypeORM, Sequelize, Knex, Django, Rails, Laravel + generic paths.

**Output:** normalized migration object.

## SAI-016 — SQL migration operation parser básico

**Implementar:** detectar CREATE/ALTER/DROP/ADD/RENAME/UPDATE/DELETE/INDEX de modo conservador.

**Não fazer:** tentar implementar parser SQL universal.

**Aceite:** se ambíguo, `unknown_operation` em vez de inventar.

## SAI-017 — Migration risk engine

**Implementar:** regras de risco descritas em `ARCHITECTURE.md`.

**Testes:** DROP column, NOT NULL sem default, index, backfill.

## SAI-018 — Env usage detector multi-stack

**Implementar:** padrões listados na arquitetura.

**Aceite:** detectar nomes sem carregar valores do ambiente.

## SAI-019 — Env base-vs-head comparator

**Implementar:** added/removed/changed-use/documented/missing-doc/default/required inference.

## SAI-020 — Secret redaction framework

**Implementar antes de qualquer IA:**

- redaction de valores de env;
- auth headers;
- tokens comuns;
- config paths secret-like;
- sanitizer de logs/prompts.

**Aceite crítico:** fixture com `PAYMENT_TOKEN=abc123` nunca grava `abc123` em artifacts/logs.

---

# Fase 4 — Evidence Bundle e storage

## SAI-021 — Artifact store filesystem

**Implementar:** run dir, atomic writes, SHA-256, manifest de artifacts.

## SAI-022 — SQLite storage

**Dependência aprovada:** `modernc.org/sqlite`.

**Implementar:** schema/migrations internas, `runs`, `components`, `analyzer_results`, `reviews`, `models`, `findings`, `artifacts`.

**Aceite:** DB é criado sem processo externo; `CGO_ENABLED=0` continua funcionando.

## SAI-023 — Evidence manifest

**Implementar:** refs para Git, changes, components, migrations, envs e limitações.

## SAI-024 — Evidence budget e truncation

**Implementar:** char/line budgets, priorities, reason-for-context, truncation metadata.

**Teste:** diff artificial >10MB não explode memória.

## SAI-025 — Sharding v1

**Implementar:** component → change clusters → shards estáveis.

**Aceite:** Peer A/B receberão mesmas shard IDs e hashes.

---

# Fase 5 — Analyzer framework e scheduler

## SAI-026 — Interface Analyzer

**Implementar:** `Detect`, `Run`, normalized result, version metadata.

## SAI-027 — Scheduler resource-aware

**Implementar:** classes light/cpu/browser/memory-heavy/active-network; concurrency limits.

**Aceite:** dois browser jobs nunca rodam simultaneamente por default.

## SAI-028 — External process runner seguro

**Implementar:** timeout, context cancel, bounded stdout/stderr, exit code, env allowlist, cwd restrito.

**Segurança:** nunca construir shell string com commit message/path não escapado.

## SAI-029 — Cache engine

**Implementar:** key por version/config/base/head/input hash; invalidation correta.

---

# Fase 6 — Test runner

## SAI-030 — Test command discovery

**Implementar:** scripts/manifests por stack + override config.

## SAI-031 — Generic test executor

**Implementar:** command, exit, duration, pass/fail; normalize JUnit/TRX quando arquivo fornecido.

## SAI-032 — Coverage importer

**Implementar inicialmente:** LCOV, Cobertura XML e formatos simples configuráveis.

**Não exigir coverage para todos os projetos.**

---

# Fase 7 — Sonar integration

## SAI-033 — Sonar applicability/config discovery

Detectar `sonar-project.properties`, config Solidify e env refs.

## SAI-034 — Sonar Web API adapter

**Implementar:** client isolado, auth por env, timeout, JSON parser, API-version tolerance.

## SAI-035 — Sonar result normalizer

Normalizar issues, severities, measures, quality gate e indicar project-wide vs diff-relevant.

## SAI-036 — Sonar scanner runner opcional

Rodar scanner em container/perfil quando configurado. Não subir Sonar Server automaticamente.

---

# Fase 8 — Segurança leve e sob demanda

## SAI-037 — Secret scanner adapter

Preferência Gitleaks; rodar no range/repo conforme capacidade da ferramenta; JSON parser.

## SAI-038 — OSV-Scanner adapter

Detectar lockfiles/SBOM relevantes, chamar scanner e normalizar vulnerabilidades.

## SAI-039 — Semgrep CE adapter

Docker/perfil sob demanda; ruleset configurável; JSON/SARIF; scope prioriza componentes alterados.

## SAI-040 — Security score/gates

Severity normalization, hard gates, waiver support.

## SAI-041 — ZAP Baseline adapter

Target allowlist, passive scan, timeout, normalized output.

## SAI-042 — ZAP API/active adapter

**Obrigatório:** `active_scan_enabled=true`, target class local/test e allowlist.

Suportar OpenAPI/Swagger/GraphQL quando configurado/detectado.

**Teste de segurança:** produção não pode ser alvo por descoberta automática.

---

# Fase 9 — Lighthouse

## SAI-043 — Frontend target resolver

Resolver URL via config/runtime hooks; healthcheck.

## SAI-044 — Lighthouse adapter

Rodar container/browser profile; coletar JSON; importar scores oficiais.

**Aceite:** não é sequer invocado em backend-only fixture.

## SAI-045 — Frontend pillar score

Compor performance/accessibility/best-practices conforme config; SEO informativo default.

---

# Fase 10 — k6/load

## SAI-046 — Endpoint change detector v1

Fontes: OpenAPI diff, route/controller patterns, config manual.

## SAI-047 — k6 adapter

Priorizar script existente; fallback a smoke script explicitamente configurado.

## SAI-048 — Load result normalization

p50/p90/p95/p99, errors, throughput, thresholds.

## SAI-049 — Baseline comparator

Comparar apenas runs compatíveis por script/config/target fingerprint. Sem baseline => não afirmar regressão.

---

# Fase 11 — MCP

## SAI-050 — Adicionar SDK MCP oficial Go

Pin de versão; transporte stdio.

## SAI-051 — `solidify_begin_review`

Criar/reusar run, preparar analyzers/evidence, retornar contract refs.

## SAI-052 — Evidence retrieval tools

Implementar manifest/diff chunk/context/symbol search/analyzer/release inventory.

**Budget:** requests grandes devem paginar/truncar.

## SAI-053 — `solidify_submit_peer_review`

Validar schema, actor metadata, evidence hash, atomic save.

## SAI-054 — Agent setup documentation/command

Gerar instruções para registrar MCP em Claude Code/Codex sem hardcode de home path destrutivo.

---

# Fase 12 — Peer review schemas e prompt

## SAI-055 — JSON Schema do Peer Review

Implementar validator e fixtures válidas/inválidas.

## SAI-056 — Prompt Peer v1

Usar `prompts/peer-review.md`, placeholders estruturados, prompt hash/version.

## SAI-057 — Peer A ingestion tests

Casos: SRP, OCP, LSP N/A, ISP, DIP.

---

# Fase 13 — Provider OpenAI-compatible e 9Router

## SAI-058 — Provider interface

ListModels + CompleteJSON + metadata.

## SAI-059 — OpenAI-compatible HTTP adapter

Sem SDK pesado; usar `net/http` e structs mínimas necessárias.

**Razão:** endpoint é simples; evita dependência de SDK de fornecedor.

## SAI-060 — 9Router preset/network handling

Host vs Docker base URL; `host.docker.internal`; `host-gateway` example.

## SAI-061 — Model discovery

`GET /v1/models`, cache SQLite, normalized IDs, expiry.

## SAI-062 — Capability probe

JSON compliance, latency, availability; cache. Não ranquear “inteligência” por nome.

## SAI-063 — Model selector

Pinned/discover/hybrid; distinctness policy; selection reason.

---

# Fase 14 — Peer B independente

## SAI-064 — Peer B request builder

Receber exatamente evidence shards equivalentes ao Peer A; nunca incluir Peer A output.

## SAI-065 — Peer B executor

Nova request/context, structured output, timeout/retry/repair único.

## SAI-066 — Independence audit

Registrar model/provider/context metadata e `independence_degraded` quando aplicável.

---

# Fase 15 — Divergence e Árbitro C

## SAI-067 — Divergence Engine determinístico

Score deltas, applicable mismatch, severity mismatch, finding overlap.

## SAI-068 — Arbiter prompt v1

Inputs: evidence + Peer A + Peer B + divergence map.

## SAI-069 — Arbiter executor

Nova request/context; modelo diferente por default contractual; structured verdict.

## SAI-070 — Arbitration merge

Accepted/rejected/merged findings; final S/O/L/I/D.

**Aceite:** árbitro não pode alterar resultado objetivo de teste/Sonar/ZAP.

---

# Fase 16 — Score, risk, confidence e gate

## SAI-071 — SOLID Score engine

Média das letras aplicáveis; before/after/delta; N/A correto.

## SAI-072 — Pillar score engine

Pesos e normalização sobre scored/applicable pillars.

## SAI-073 — Release Risk engine

Migration/env/breaking/security/blast-radius/review disagreement.

## SAI-074 — Confidence engine

Evidence completeness, peer agreement, independence, analyzer completeness, arbiter confidence.

## SAI-075 — Quality Gate

Perfis quick/release/contractual; PASS/WARN/FAIL/BLOCKED/INCOMPLETE.

**Golden tests:** release 95 + critical CVE => FAIL.

---

# Fase 17 — Release report canônico

## SAI-076 — JSON Schema do release report

Finalizar `schemas/release-report.schema.json` e validator.

## SAI-077 — Canonical report builder

Montar sem lógica de UI.

## SAI-078 — Artifact immutability/hash

Finalized report não é modificado; rerun cria novo run.

## SAI-079 — Executive summary input model

Gerar summary a partir de fatos/arbiter verdict. Se usar IA, deve ser etapa separada e nunca alterar scores.

---

# Fase 18 — Dashboard

## SAI-080 — Bootstrap Preact/Vite/TypeScript

Build estático; zero Node no runtime.

## SAI-081 — Design tokens e print-friendly layout

Sem Tailwind; CSS leve.

## SAI-082 — Overview/SOLID UI

Gate, quality, risk, confidence, S/O/L/I/D before→after.

## SAI-083 — Release/Migrations/Envs UI

Features/bugfixes, commits, migrations, deployment actions, env names only.

## SAI-084 — Quality/Security/Performance UI

Sonar/tests/security/k6/Lighthouse conditional rendering.

## SAI-085 — AI transparency UI

Peers, models, provider, prompt version, consensus, divergences, independence.

## SAI-086 — History UI

SQLite read-only endpoint local apenas para list/trends; detalhe de run continua vindo do JSON canônico.

## SAI-087 — `solidify dashboard`

`net/http`, localhost-only, embedded assets, graceful shutdown.

---

# Fase 19 — PDF

## SAI-088 — Print stylesheet

A4, page breaks, headers/footers, cards legíveis, sem scroll-only content.

## SAI-089 — Browser detector

Chrome/Chromium executável; version log.

## SAI-090 — Headless PDF command

Render do mesmo HTML/report; timeout; artifact hash.

## SAI-091 — Docker PDF fallback

Perfil `pdf` temporário; sem serviço persistente.

## SAI-092 — PDF contractual content validation

Teste automatizado via metadados/text extraction simples para garantir seções obrigatórias e ausência de secrets fixture.

---

# Fase 20 — Robustez por troca de papéis

## SAI-093 — External model role-swap runner

Y como Peer B, X como Arbiter, mantendo contexto isolado.

## SAI-094 — Robustness comparator

Score delta, letter deltas, finding overlap, gate stability.

## SAI-095 — Robustness report integration

Exibir stable/mostly-stable/unstable no JSON/dashboard/PDF quando executado.

---

# Fase 21 — Performance e hardening

## SAI-096 — Benchmark suite realista

Fixtures 1k files/10MB diff/monorepo.

## SAI-097 — Memory profiling

Identificar cópias de diff/artifacts; otimizar streaming/buffers.

## SAI-098 — Core image budget

Multi-stage Alpine; non-root; git; ca-certs; medir compressed size.

## SAI-099 — Tool resource limits

Compose profiles com limites configuráveis e heavy concurrency=1.

## SAI-100 — Cache verification

Re-run idêntico reaproveita analyzer results; mudança de tool/config invalida.

---

# Fase 22 — Segurança do core

## SAI-101 — Path traversal tests

MCP context tool não lê fora do repo/allowed roots.

## SAI-102 — Command injection tests

Paths/commit subjects maliciosos não entram em shell.

## SAI-103 — Secret corpus redaction tests

API keys, bearer, env values, connection strings, passwords.

## SAI-104 — Active target safety tests

ZAP active/k6 destructive profile não roda em host fora de allowlist.

---

# Fase 23 — E2E de aceitação

## SAI-105 — Fixture fullstack sample

Repo exemplo com frontend + backend + migration + env + testes.

## SAI-106 — E2E backend-only release

Esperado: Lighthouse N/A; backend analyzers aplicáveis; PDF correto.

## SAI-107 — E2E frontend-only release

Esperado: Lighthouse aplicável; k6 API N/A salvo config.

## SAI-108 — E2E contractual peer review

Mock OpenAI-compatible server com Peer B + Arbiter; report final peer-reviewed.

## SAI-109 — E2E 9Router adapter smoke

Teste manual/opt-in contra endpoint local; nunca requerido em CI pública.

## SAI-110 — E2E migration/env risk

Migration destrutiva + env required undocumented => Release Risk elevado e action item.

## SAI-111 — E2E hard gate

Security high/critical => FAIL mesmo com Quality Score alto.

---

# Fase 24 — Documentação de uso comunitário

## SAI-112 — README do projeto real

Quickstart Docker/native, exemplos Claude Code/Codex, config 9Router.

## SAI-113 — `solidify doctor`

Diagnóstico de Git/Docker/browser/Sonar/provider/tools sem executar análise pesada.

## SAI-114 — Example reports sanitizados

JSON + screenshots/PDF sample sem código proprietário.

## SAI-115 — ADR index

Registrar decisões tomadas durante implementação.

---

# Definition of Done do MVP

O MVP NÃO está pronto apenas porque “gera uma nota”. Está pronto quando:

- diff scope é correto;
- before/after é rastreável;
- migrations/envs são detectadas;
- nenhum secret fixture vaza;
- applicability evita tools irrelevantes;
- Peer A/B são cegos entre si;
- Arbiter é contexto isolado;
- modelo/papel/provider aparecem no report;
- JSON passa schema;
- dashboard e PDF vêm do mesmo JSON;
- contractual falha fechado;
- core termina sem daemon;
- benchmarks e image budget estão documentados;
- E2E fullstack demonstra o fluxo completo.


<!-- END TASKS: TASKS.md -->
