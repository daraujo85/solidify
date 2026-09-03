Você está trabalhando no projeto **Solidify**.

O projeto já possui toda a esteira de análise de qualidade baseada em Git diff, evidence bundle, análise SOLID, ferramentas complementares, peer review, arbitragem, quality gates e geração de relatório JSON/HTML/PDF.

A implementação atual já cobre as tasks até **SAI-118**. Antes de alterar qualquer coisa, leia os ADRs, `TASKS.md`, contratos, schemas e a implementação atual para preservar compatibilidade e não duplicar mecanismos que já existem.

## Objetivo

Implementar uma nova capacidade opcional no Solidify para analisar **a diferença entre duas Git refs**, permitindo usar o Solidify como ferramenta de validação de uma release candidate contra uma branch base.

Exemplo principal:

```bash
solidify run \
  --base main \
  --head release/1.24.0 \
  --profile contractual
```

A intenção é responder:

> O que esta RC introduz em relação à main e qual é a qualidade técnica dessa entrega?

Essa análise deve passar por **toda a esteira já existente do Solidify**, sem criar um pipeline paralelo.

---

# 1. Criar ADR e task

Criar:

```text
ADR 0119 — Git Ref Comparison Analysis
SAI-119 — Branch/Tag/Commit Comparative Analysis
```

Documentar primeiro a decisão arquitetural antes da implementação.

O ADR deve explicar:

* motivação;
* casos de uso;
* diferença entre análise de commit e comparação de refs;
* estratégia de Git diff;
* merge-base vs comparação direta entre árvores;
* impactos em evidence bundle;
* impacto em migrations/envs;
* impacto em release report;
* compatibilidade retroativa;
* performance;
* edge cases;
* critérios de segurança;
* decisão final.

---

# 2. Não limitar a solução a branches

Internamente, tratar `base` e `head` como **Git refs genéricas**.

Deve funcionar com:

```bash
solidify run --base main --head release/1.24.0

solidify run --base v1.3.0 --head v1.4.0

solidify run --base a81bc23 --head f26ad84

solidify run --base origin/main --head HEAD

solidify run --base develop --head feature/pix
```

Uma ref pode ser:

* branch local;
* remote branch;
* tag;
* commit SHA;
* `HEAD`;
* qualquer ref resolvível pelo Git.

Não criar código específico para nomes como `main`, `develop`, `release` ou `rc`.

---

# 3. Git Scope Resolver

Evoluir o início da pipeline para trabalhar com um conceito genérico de `AnalysisTarget`.

Conceitualmente:

```text
Analysis Target
 ├── commit
 ├── working tree
 ├── branch vs branch
 ├── tag vs tag
 ├── commit vs commit
 └── ref vs HEAD
        ↓
 Git Scope Resolver
        ↓
 Evidence Bundle
        ↓
 Pipeline atual do Solidify
```

A partir do `Evidence Bundle`, o restante da arquitetura deve continuar reaproveitando o fluxo existente.

Evitar `if branch mode` espalhado pela aplicação.

Criar abstração coesa para resolução de escopo Git.

---

# 4. Estratégias de diff

Suportar pelo menos duas estratégias.

## 4.1 `merge-base`

Deve ser o padrão para comparação de branches/releases.

Exemplo conceitual:

```bash
git diff main...release/1.24.0
```

Isso responde:

> Quais alterações a branch `release/1.24.0` introduziu desde o ponto em que se separou da `main`?

CLI:

```bash
solidify run \
  --base main \
  --head release/1.24.0 \
  --diff-mode merge-base
```

Se `--diff-mode` não for informado para uma análise `base/head`, usar:

```text
merge-base
```

como default.

## 4.2 `endpoints`

Comparação direta entre os dois snapshots Git.

Conceitualmente:

```bash
git diff main release/1.24.0
```

CLI:

```bash
solidify run \
  --base main \
  --head release/1.24.0 \
  --diff-mode endpoints
```

Isso responde:

> Qual é a diferença entre o estado atual completo dessas duas árvores?

---

# 5. Resolver e persistir SHAs

Nunca deixar apenas nomes simbólicos no relatório.

Resolver e armazenar:

```text
base_ref
base_sha

head_ref
head_sha
```

No modo `merge-base`, também:

```text
merge_base_sha
```

Exemplo:

```json
{
  "analysis_scope": "git_ref_comparison",
  "base_ref": "main",
  "base_sha": "a83bd72...",
  "head_ref": "release/1.24.0",
  "head_sha": "fd816ca...",
  "diff_strategy": "merge_base",
  "merge_base_sha": "733ca91..."
}
```

Isso garante reprodutibilidade e auditabilidade do relatório.

---

# 6. Não alterar comportamento existente

A implementação precisa ser **backward compatible**.

Os modos atuais de execução devem continuar funcionando sem mudança de comportamento.

Por exemplo, se hoje existe:

```bash
solidify run --commit <sha>
```

ou análise do `HEAD`/working tree, isso não deve quebrar.

`--base` e `--head` representam um novo modo opcional.

Não obrigar projetos existentes a alterar configuração.

---

# 7. Validação dos argumentos CLI

Validar combinações inválidas.

Exemplos:

```text
--base sem --head → erro claro

--head sem --base → erro claro

--diff-mode sem comparação base/head → verificar se faz sentido ou rejeitar

ref inexistente → erro antes de iniciar análises caras
```

Erros de Git scope devem ocorrer antes de:

* Sonar;
* Lighthouse;
* scanners;
* Peer A;
* Peer B;
* Arbiter;
* k6;
* geração de relatório pesado.

Fail fast.

---

# 8. Evidence Bundle

Depois de resolver o escopo Git, produzir o mesmo tipo de evidence bundle consumido atualmente pelo Solidify.

A evidence bundle da comparação deve conter, quando disponível:

* refs originais;
* SHAs resolvidos;
* merge-base;
* diff strategy;
* commits pertencentes à entrega;
* arquivos adicionados;
* arquivos modificados;
* arquivos removidos;
* arquivos renomeados;
* linhas adicionadas/removidas;
* símbolos alterados;
* módulos/pacotes afetados;
* migrations;
* envs;
* dependências modificadas;
* manifests modificados;
* testes modificados/adicionados/removidos;
* alterações relevantes para segurança;
* metadados necessários aos analisadores atuais.

Não mandar o repositório inteiro para IA.

Continuar seguindo o princípio:

```text
Evidence First
Diff Scoped
Token Efficient
```

---

# 9. Commits pertencentes à entrega

Para comparação por merge-base, identificar os commits que pertencem ao lado `head` desde o merge-base.

Isso deve alimentar o release notes automático.

Exemplo no relatório:

```text
Release candidate: release/1.24.0
Base: main

Commits da entrega: 17
Arquivos alterados: 43
Linhas: +2.814 / -921
```

Usar mensagens de commit e evidências disponíveis para continuar a classificação já existente:

* Features;
* Bugfixes;
* Refactors;
* Docs;
* Chore;
* Tests;
* outras classificações suportadas atualmente.

Não depender exclusivamente de Conventional Commits; usar quando existir, mas continuar tolerante a repositórios sem padrão perfeito.

---

# 10. Migrations

A comparação de refs deve melhorar a análise de migrations.

Em vez de perguntar apenas:

> Existe migration neste commit?

A análise deve responder:

> Quais migrations existem no `head` e foram introduzidas/alteradas/removidas em relação ao `base`?

Classificar pelo menos:

```text
NEW
MODIFIED
REMOVED
```

Quando possível, identificar risco:

```text
LOW
MEDIUM
HIGH
CRITICAL
UNKNOWN
```

Exemplos de sinais de atenção:

* `DROP TABLE`;
* `DROP COLUMN`;
* mudanças de tipo potencialmente destrutivas;
* rename destrutivo;
* operações que podem travar tabelas;
* migrations sem rollback quando o framework oferece esse conceito;
* alteração de migration histórica já existente na base.

Exemplo no relatório:

```text
Database Changes

2 migrations adicionadas
0 removidas
1 potencialmente destrutiva

20260820_add_payment_status.sql
NEW
Risk: LOW

20260821_drop_legacy_payment_column.sql
NEW
Risk: HIGH
Reason: DROP COLUMN detected
```

Não executar migration automaticamente apenas para analisá-la.

---

# 11. Environment variables

Fazer comparação `base vs head` das envs referenciadas/configuradas no projeto.

Classificar:

```text
NEW
CHANGED
REMOVED
UNCHANGED
```

Exemplo:

```text
NEW
PIX_RETRY_ENABLED

CHANGED
PAYMENT_TIMEOUT

REMOVED
LEGACY_GATEWAY_URL
```

Quando possível, registrar:

* nome;
* status;
* obrigatória/opcional;
* existência de default;
* documentação;
* arquivos onde aparece;
* possível ação necessária no deploy.

**Nunca registrar valor de segredo.**

Continuar respeitando redaction existente.

Se uma env tiver algo como:

```text
DATABASE_PASSWORD
API_SECRET
JWT_PRIVATE_KEY
```

o relatório pode registrar o nome, mas jamais o conteúdo.

---

# 12. Dependências

Detectar alterações em dependências entre base/head.

Exemplos:

* `package.json`;
* lockfiles;
* `.csproj`;
* `go.mod`;
* `go.sum`;
* `requirements.txt`;
* `pyproject.toml`;
* `pom.xml`;
* `build.gradle`;
* `composer.json`;
* outros manifests já suportados.

Relatar:

```text
Added
Updated
Removed
```

Quando os scanners existentes forem aplicáveis, reaproveitá-los.

---

# 13. Score representa a entrega, não necessariamente o repositório inteiro

Esse ponto é crítico.

Se o Solidify avaliou apenas o delta:

```text
main → release/1.24.0
```

não afirmar:

> O projeto inteiro possui Quality Score 91.

O correto é:

> A entrega analisada possui Quality Score 91.

Adicionar ao contrato:

```json
{
  "score_scope": "delivery"
}
```

Possíveis valores devem ser bem definidos, por exemplo:

```text
delivery
repository
```

Mas não implementar `repository` se o Solidify ainda não possuir uma análise realmente global.

---

# 14. Baseline histórico opcional

Se existir uma execução histórica compatível para a `base`, permitir mostrar evolução.

Exemplo:

```text
                    BASE      RC        DELTA

Quality Score        87       91        +4
S — SRP              84       90        +6
O — OCP              88       89        +1
L — LSP              94       94         0
I — ISP              83       88        +5
D — DIP              86       91        +5

Security             93       93         0
Tests                81       87        +6
```

Porém:

* isso deve ser opcional;
* não executar automaticamente uma análise completa da base apenas para ter comparação;
* reaproveitar cache/histórico quando disponível;
* deixar claro quando não existe baseline.

Não comparar scores incompatíveis.

Exemplo:

```text
delivery score
```

não deve ser tratado como equivalente a um futuro:

```text
repository score
```

---

# 15. Pipeline de qualidade

Depois de gerar a evidence bundle, executar **a mesma esteira existente**, conforme aplicabilidade.

Exemplo:

```text
Git Ref Comparison
        ↓
Evidence Bundle
        ↓
Stack / Applicability Detection
        ↓
SOLID Analysis
        ↓
Static Analysis
        ↓
Security
        ↓
Tests
        ↓
Lighthouse (se aplicável)
        ↓
k6 (se configurado/aplicável)
        ↓
Peer A
        ↓
Peer B
        ↓
Arbiter
        ↓
Quality / Independence / Security Gates
        ↓
Final Gate
        ↓
JSON / HTML / PDF
```

Não criar versões especiais dos peers para branch comparison.

Peers devem receber o evidence bundle normalizado.

---

# 16. SAI-116 / SAI-117 / SAI-118 continuam obrigatórios

Não enfraquecer nada implementado nas tasks anteriores.

Preservar:

## Canonical AI Evaluation Schema

Resposta inválida:

```text
generation
→ validate
→ repair #1
→ validate
→ INCOMPLETE
```

Nunca:

```text
parse failure → score 70
```

Nunca:

```text
parse failure → score 0
```

## Exit codes

Manter a semântica:

```text
0 = avaliação válida + PASS
1 = avaliação válida + FAIL
2 = avaliação INCOMPLETE
```

## Independence Gate

Não voltar a usar flag hardcoded.

A independência deve derivar dos atores/modelos realmente executados.

## Peer A / Peer B

Peer A e Peer B devem continuar independentes.

Peer B não pode receber o resultado do Peer A.

## Arbiter

O Arbiter pode receber:

* canonical Peer A;
* canonical Peer B;
* evidence manifest;
* evidence references relevantes;

e deve arbitrar com base nas evidências.

---

# 17. Perfis

Os perfis existentes continuam controlando thresholds.

Não hardcodar:

```text
score >= 75
```

Usar sempre a configuração do profile.

Exemplo atual:

```text
quick       = 60
release     = 75
contractual = 85
```

Se esses valores estiverem centralizados em configuração existente, usar a fonte existente em vez de duplicá-los.

---

# 18. Relatório JSON

Evoluir o schema de release report de forma versionada e backward compatible quando possível.

Adicionar uma seção semelhante a:

```json
{
  "analysis": {
    "scope": "git_ref_comparison",
    "score_scope": "delivery",
    "base_ref": "main",
    "base_sha": "a83bd72",
    "head_ref": "release/1.24.0",
    "head_sha": "fd816ca",
    "diff_strategy": "merge_base",
    "merge_base_sha": "733ca91"
  }
}
```

Adicionar também resumo:

```json
{
  "delivery": {
    "commits": 17,
    "files_changed": 43,
    "lines_added": 2814,
    "lines_removed": 921
  }
}
```

Não quebrar consumidores atuais desnecessariamente.

Se uma alteração breaking no schema for inevitável, documentar explicitamente e versionar corretamente.

---

# 19. Dashboard HTML

O dashboard deve mostrar claramente o escopo da análise.

No topo:

```text
BASE
main
a83bd72

HEAD
release/1.24.0
fd816ca

MODE
merge-base

COMMITS
17

FILES
43

LINES
+2.814 / -921
```

Também deve mostrar:

* Quality Score da entrega;
* notas S/O/L/I/D;
* security;
* performance;
* tests;
* migrations;
* envs;
* dependencies;
* release notes;
* peer review;
* arbitragem;
* gates;
* baseline/delta quando disponível.

Deixar visualmente explícito:

```text
Score scope: Delivery
```

---

# 20. PDF / relatório contratual

O PDF precisa ser adequado para entrega a cliente.

Uma frase conceitual importante:

> Este relatório representa as alterações existentes em `release/1.24.0` em relação à `main`, considerando o escopo Git identificado no momento da análise.

Incluir:

```text
Base ref
Base SHA
Head ref
Head SHA
Merge-base
Diff strategy
Data/hora
Run ID
Profile
```

O relatório deve continuar expondo os modelos utilizados:

```text
Peer A
provider
model
role

Peer B
provider
model
role

Arbiter
provider
model
role
```

E os gates separadamente:

```text
quality_gate
independence_gate
security_gate
final_gate
```

---

# 21. Cache

A comparação deve ser cacheável.

A chave precisa considerar no mínimo:

```text
base_sha
head_sha
diff_strategy
profile
relevant configuration hash
analyzer versions
schema version
```

Não usar apenas nomes de branches como cache key, pois são mutáveis.

Exemplo errado:

```text
main_release-1.24
```

Exemplo conceitualmente correto:

```text
sha256(
  base_sha +
  head_sha +
  diff_strategy +
  profile +
  config_hash +
  analyzer_versions
)
```

---

# 22. Performance

Solidify continua sendo uma ferramenta complementar e não pode se tornar pesada.

Não fazer:

```text
git checkout base
analisar projeto inteiro
git checkout head
analisar projeto inteiro
comparar tudo
```

como comportamento padrão.

Priorizar:

* Git plumbing;
* diff direto;
* merge-base;
* leitura seletiva;
* AST apenas dos arquivos/símbolos relevantes;
* cache;
* execução paralela onde seguro;
* nenhuma duplicação desnecessária de evidence.

O modo branch comparison deve continuar sendo **diff-scoped**.

Não deixar daemon residente.

Não introduzir dependência pesada apenas por conveniência.

---

# 23. Segurança

Não aceitar refs que resultem em execução arbitrária.

Tratar nomes de refs como argumentos Git de forma segura.

Evitar shell string concatenation do tipo:

```go
exec.Command("sh", "-c", "git diff " + base + "..." + head)
```

Preferir argumentos separados:

```go
exec.Command(
    "git",
    "diff",
    base+"..."+head,
)
```

Validar resolução das refs antes.

Preservar proteções existentes contra:

* path traversal;
* secret exposure;
* command injection;
* arquivos fora do workspace permitido.

---

# 24. Working tree

Definir comportamento explícito quando existem alterações locais não commitadas.

Para `--base/--head`, a análise deve, por padrão, representar **refs Git imutavelmente resolvidas**, não misturar working tree acidentalmente.

Se houver alterações locais:

```text
working tree dirty
```

não incorporá-las silenciosamente à comparação.

Pode:

* ignorá-las e registrar warning;
* ou rejeitar conforme configuração existente.

Mas a decisão precisa ser explícita e testada.

---

# 25. Shallow clone

Tratar caso de CI com clone raso.

`merge-base` pode não existir localmente porque o histórico necessário não foi buscado.

Não produzir resultado errado silenciosamente.

Detectar:

```text
merge base unavailable
```

e retornar erro claro ou orientação apropriada.

Não fazer fetch remoto automaticamente sem uma configuração explícita, pois Solidify deve evitar efeitos colaterais de rede inesperados.

---

# 26. Repositórios remotos

Se o usuário passa:

```text
origin/main
```

usar o estado local conhecido dessa ref.

Não assumir que precisa executar:

```bash
git fetch
```

automaticamente.

Rede deve continuar explícita.

---

# 27. Renames e copies

Usar capacidades do Git para diferenciar corretamente quando possível:

```text
ADDED
MODIFIED
DELETED
RENAMED
```

Não tratar todo rename como:

```text
delete old
add new
```

quando Git conseguir identificar a movimentação.

Isso influencia:

* release notes;
* análise de código;
* migrations;
* documentação;
* score de mudança.

---

# 28. Arquivos binários

Não tentar mandar binários para LLM.

Registrar alteração:

```text
binary file changed
```

e metadados necessários.

Continuar obedecendo filtros existentes de extensões/tamanhos.

---

# 29. Testes obrigatórios

Criar cobertura ampla.

No mínimo:

### Git scope

* branch vs branch;
* tag vs tag;
* SHA vs SHA;
* remote ref vs HEAD;
* ref inexistente;
* HEAD inválido;
* merge-base disponível;
* merge-base indisponível;
* endpoints;
* dirty working tree;
* rename;
* delete;
* binary;
* shallow repository.

### CLI

* base/head válidos;
* apenas base;
* apenas head;
* diff-mode inválido;
* combinação com opções antigas;
* backward compatibility.

### Evidence

* changed files;
* commits;
* stats;
* symbols;
* migrations;
* envs;
* dependencies.

### Cache

* mesma comparação → hit;
* mesmo nome de branch com SHA novo → miss;
* mudança de diff strategy → miss;
* mudança de profile → comportamento coerente.

### Report

* refs;
* SHAs;
* merge-base;
* scope;
* score_scope;
* stats;
* migrations;
* envs;
* release notes.

### Gates

Garantir que a nova forma de definir scope não altera:

* quality gate;
* independence gate;
* security gate;
* final gate;
* `INCOMPLETE`;
* exit codes.

---

# 30. Fixtures

Expandir fixtures existentes, preferindo reaproveitar:

```text
fullstack-sample
examples/report-*.json
```

Criar cenário realista contendo:

```text
main
release branch
feature commit
bugfix commit
migration
new env
changed dependency
refactor
tests
```

Permitir validar a feature sem depender de CI remoto real.

---

# 31. UX da CLI

Ao iniciar, imprimir algo enxuto como:

```text
Solidify — Git Ref Comparison

Base:        main (a83bd72)
Head:        release/1.24.0 (fd816ca)
Strategy:    merge-base
Merge base:  733ca91
Profile:     contractual

Commits:     17
Files:       43
Changes:     +2814 / -921
```

Depois seguir com a execução normal.

Na conclusão:

```text
run_id:  r...
scope:   main...release/1.24.0
score:   91.0
gate:    PASS
report:  ...
```

---

# 32. Não inflar a solução

Antes de adicionar nova lib:

1. verificar se Go stdlib + execução do Git já resolvem;
2. verificar libs já existentes no projeto;
3. preferir dependências pequenas e mantidas;
4. justificar nova dependência relevante no ADR.

O Solidify deve continuar leve.

---

# 33. Critérios de aceite

SAI-119 só pode ser considerada concluída quando:

1. `--base` + `--head` funcionarem com refs Git genéricas;
2. `merge-base` for o comportamento default;
3. `endpoints` estiver implementado;
4. refs forem resolvidas para SHAs;
5. evidence bundle representar somente o delta;
6. commits da entrega forem identificados;
7. migrations forem comparadas base/head;
8. envs forem comparadas base/head sem exposição de secrets;
9. dependências alteradas forem detectadas;
10. pipeline existente for reutilizado integralmente;
11. Peer A/B/Arbiter continuarem independentes conforme SAI-116/117/118;
12. gates existentes continuarem funcionando;
13. report JSON guardar todo o escopo Git;
14. dashboard mostrar base/head;
15. PDF mostrar base/head e declarar que o score é da entrega;
16. cache usar SHAs e não apenas nomes de refs;
17. dirty working tree não contaminar silenciosamente o resultado;
18. shallow clone não gerar comparação falsa;
19. testes cobrirem os hot paths;
20. todos os packages internos continuarem passando.

---

# 34. Forma de execução

Antes de implementar:

1. leia o projeto atual;
2. identifique o Git scope resolver existente;
3. identifique os contratos atuais do evidence bundle;
4. identifique o schema atual do report;
5. identifique os comandos CLI atuais;
6. leia ADRs 0116, 0117 e 0118;
7. escreva ADR 0119;
8. apresente brevemente quais arquivos/packages precisarão mudar;
9. só então implemente SAI-119.

A cada decisão arquitetural relevante:

* preservar padrões existentes;
* evitar duplicação;
* registrar no ADR;
* escrever testes antes ou junto da implementação;
* não alterar comportamento existente sem justificativa.

Ao terminar, execute toda a suíte relevante e entregue um resumo objetivo contendo:

```text
SAI-119 status
Arquivos/packages alterados
Quantidade de testes adicionados
Total de testes passando
Novos contratos/schemas
Exemplos de CLI validados
Impacto de performance
Compatibilidade retroativa
ADRs criados/atualizados
Pendências conhecidas
```

Não avance automaticamente para SAI-120 ou outras features.

**O objetivo desta task é transformar o Solidify de um analisador de alteração pontual em uma ferramenta capaz de realizar aceite técnico de uma entrega inteira entre duas referências Git, sem abandonar o princípio de analisar somente o delta relevante.**
