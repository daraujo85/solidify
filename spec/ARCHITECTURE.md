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
