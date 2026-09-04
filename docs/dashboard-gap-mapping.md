# Mapeamento de gaps — contrato do dashboard × backend real

Gerado após a reformulação visual (`web/src/main.js`, `mapping.js`, `views/release.js`)
contra `CONTRATO-DE-DADOS.md` (17 seções). Objetivo: para cada seção da UI, dizer
**o que já é real hoje** e **o que falta no backend** para o JSON final (`internal/report.Report`)
cobrir 100% do contrato sem fabricar nada.

Legenda de status:
- ✅ **real** — campo existe em `report.Report` (builder.go) e está mapeado em `mapping.js`.
- 🟡 **derivado, calculável hoje** — não existe como campo mas dá pra computar client-side
  a partir de campos ✅ já existentes (é o que as Fases A fizeram até agora).
- 🔴 **novo, precisa de backend** — não existe base nenhuma; precisa de analisador, política
  de config, persistência ou geração por LLM antes de a UI poder mostrar dado real.

---

## §1 Header — ✅ quase tudo real
`run.id/profile/solidify_version/started_at/finished_at`, `git.*` → todos ✅ (`RunInfo`, `GitInfo`).
Gaps 🔴: `release.ambiente` (mapa branch/tag→ambiente — não existe em config), `equipe` (dono do
repo — não existe em config). Baixa prioridade, cosmético.

## §2 Release Quality Score — misto
- `valor` (score) ✅ `scores.quality`. `grade`/`confidence` ✅.
- `veredicto` 🟡 já implementado (`gateVerdict()` em `mapping.js`, deriva de `quality_gate.status`).
- `score_anterior`/`delta` 🔴 — **precisa de persistência de histórico entre runs** (mesmo gap
  do §8, ver abaixo). Fórmula de score em si (`valor`) já existe e é versionada
  (`internal/score`), não é gap.

## §3 Painel SOLIDIFY — ✅ real
`solid.principles.{S,O,L,I,D}.after_score` já ✅, implementado em `viewSolidifyPanel`.
Sem gap — os "5 princípios" do mockup nunca foram fabricados, são reais desde antes desta fase.

## §4 Gate de Entrega — misto, maior gap estrutural da seção
- Real hoje ✅: `quality_gate.rules[]` (G1 quality, G2 independence) + 4 critérios derivados
  client-side (`mapGateSection`): security.gate_allow, coverage.is_good, violações SOLID
  (via `risk.factors` casado por princípio), migration rollback.
- ✅ **RESOLVIDO**: `report.GateRule` ganhou campo `Blocking bool` (`json:"blocking"`), populado em
  `run.go` a partir de `gateRes.Subgates.{Quality,Independence}.Blocking`. UI mostra pill
  "bloqueante" em `viewGateSection` quando `true` (`mapGateSection`/`release.js`). Achado extra
  no mesmo fix: `bInput.QualityGate.Rules` só emitia G1 (independence/G2 só ia pro `FinalGate`,
  campo que a UI não lê) — corrigido para emitir G1+G2, e `Status` do gate passou a usar
  `gateRes.Status` (combinado) em vez de só o subgate de quality, já que G2 sozinho pode reprovar.
- 🔴 As **6 categorias nomeadas fixas do mockup** (score_minimo, cobertura, vuln_criticas,
  testes_passando, migration_ok, ...) não existem como política — o gate real roda regras
  dinâmicas (`.solidify.yml` hoje não tem seção de política de gate). Criar isso é trabalho de
  produto (schema de política), não só wiring; **não fazer sem definição do usuário**.
- 🔴 `ressalva` (texto livre escrito por humano na aprovação) e os botões "Ver política do
  gate"/"Aprovar entrega" — exigem tela de aprovação + persistência de decisão. Fora de escopo
  do dashboard read-only atual.

## §4.1 Slider de aprovação — ✅ 🟡 completo
`limite_aprovacao` default 🔴 (hoje hardcoded `DEFAULT_APPROVAL_LIMIT=80` em `main.js`, o contrato
pede vir de política de repo — mesmo gap de política do §4). Resto (folga, veredicto recalculado,
contagem) 🟡 já implementado e corretamente não-persistente.

## §5 Analisadores (transparência) — ✅ real
`mapAnalyzersStrip` já cobre applicability/execution_status/score/duration dos 5 analyzers.
Sem gap.

## §5 (AI reviewers / consenso) — ✅ parcial, gaps são de enriquecimento
`ai_review.actors[].{role,provider,model_id,status,fallback_used}` ✅. Gaps 🔴: `achados` (texto
curto por revisor — contrato pede derivado da contagem de violações *daquele revisor*, mas
`Principle.Findings` fica sempre vazio no pipeline atual, então não tem o que contar por revisor);
`cobertura_analise_pct`/`trechos_sem_revisao` (exige o pipeline registrar explicitamente o que
ficou de fora — arquivo grande/binário/gerado — não registra hoje).

## §6 Violações SOLID priorizadas — ✅ real com 1 gap pontual
`mapViolations` já deriva tudo de `risk.factors` + `solid.principles[X].evidence_refs`. Gaps 🔴:
`nova_nesta_release` (precisa histórico, mesmo gap do §8); `task_url` (integração Jira/Linear/
GitHub Issues — feature nova, botão "Criar tarefa" no mockup não tem call site nenhum hoje).

## §7 Arquivos de risco — ✅ real
`mapRiskFiles` (coverage worst-files × churn) já implementado com barra de cobertura colorida.
Sem gap.

## §8 Tendência do score — 🔴 100% novo, maior item de infraestrutura pendente
**Explicitamente fora de escopo desta fase (decisão do usuário).** Precisa de: tabela de
histórico de runs por repositório (schema + storage — hoje `internal/store` guarda runs mas não
há índice por repo/branch nem leitura de "últimas 6 releases"), agregação por princípio
("SRP é o princípio com mais reincidência"). É pré-requisito de `score_anterior`/`delta` em
§2, `nova_nesta_release` em §6, `serie` em §12/11.1. **Se for atacado, é o primeiro item — todo
o resto de "cross-release" depende dele.**

## §9 O que foi entregue — ✅ RESOLVIDO (wiring feito)
Era 🔴 (`run.go` nunca populava `Commits`). Corrigido: novo `internal/app/git_wiring.go:buildCommits`
chama `gitx.Log(dir, base..head, LogOpts{})` e mapeia `gitx.Commit` → `report.Commit` usando
`Conventional.Type` diretamente (não `classification.ClassifyCommit`, que responde outra pergunta
e não é o que `mapDeliveries` em `mapping.js` lê — ele agrupa por `commit.type` cru). Chamado em
`run.go` logo após o diff de evidência. `ReleaseNotes.Groups`/`descricao` (resumo LLM por grupo)
continuam 🔴 fora de escopo — geração de texto é passo separado, agora possível sobre `Commits`
real em vez de bloqueado por array vazio.

## §10 Arquivos alterados — ✅ RESOLVIDO (wiring feito, gap descoberto durante §9/§13)
Correção sobre a 1ª versão deste doc (que dizia "✅ real, 1:1 de `report.git.changed_files[]`" —
errado): `bInput.Git.ChangedFiles` também nunca era atribuído em `run.go`, mesma classe de gap do
§9/§13. Corrigido junto: `git_wiring.go:buildChangedFiles` mapeia `gitx.Changes` (git diff --raw +
--numstat) pro shape do report, incluindo `binary: true` sem fabricar `added_lines`/`deleted_lines`
pra arquivo binário (viraria `null`/omitido, não `0`).

## §11/11.1/11.2 Segurança / SonarQube / Lighthouse — ✅ real (pós Fase B)
`mapSecuritySection`/`mapSonarSection`/`mapLighthouseSection` já ✅ desde o wiring dos analyzers.
Gap 🔴 pontual: `metricas[].serie` do SonarQube (6 pontos históricos, mini-gráfico de dívida
técnica) — mesmo gap do §8, precisa de `/api/measures/search_history` do Sonar OU do histórico
interno; nenhum dos dois está implementado.

## §12 Performance / Carga (k6) — ✅ agregado real, 🔴 série temporal
`mapK6Section` cobre p95/p99/erro/throughput/thresholds — reais, pós wiring do runner k6.
Gap 🔴 (**explicitamente adiado**): `serie_p95` (~23 pontos dentro de uma única run) — o k6
summary atual só produz estatísticas agregadas, não série temporal por timestamp; exigiria
processar o `.jsonl` de saída do k6 (não só o summary) para extrair pontos ao longo do teste.
Também 🔴: `limite` de cada métrica (vem de política do gate, mesmo gap do §4).

## §13 Migrations — ✅ RESOLVIDO (wiring feito)
Era 🔴 (`bInput.Migrations` nunca atribuído). Corrigido: `git_wiring.go:buildMigrations` detecta
migrations no diff via `internal/migrations.Detect` (usando `cfg.Detectors.MigrationPaths` como
`genericDirs`, já existia em config), roda `migrations.Parse` só quando o arquivo é `.sql` (única
fonte que o parser sabe ler — frameworks `.py/.rb/.php/.cs/.js` ficam sem `Operations`, nunca
inventamos parsing que não existe), `Assess` pra findings de risco, e `ImpactForFindings` pro
`Impacto` (Baixo/Médio/Alto). `Risk` (string livre) é o pior `RiskLevel` em minúsculo, mesma
convenção da fixture; sem findings, `""` — nunca fabrica "low" como default.

**`RollbackPresent` — decisão de design explícita**: `internal/migrations.Detect` não faz
sibling-file discovery (`TestDetectRollbackFlag` confirma que é o *caller* quem informa o path do
rollback). Implementar isso de forma confiável exigiria varrer o repo inteiro (rollback pode
existir sem aparecer no diff atual); em vez de arriscar um "sem rollback" fabricado, o wiring só
avalia convenções de nome bem definidas E dentro do próprio diff (Flyway `V.../U...`, genérico
`.up.sql`/`.down.sql`) e só pra migration `status:"added"` — nesses casos `RollbackPresent` vem
`true`/`false` real; em qualquer outro caso (framework sem convenção reconhecida, ou migration
`modified`/`deleted` cujo sibling pode não ter mudado no diff) fica `nil`/omitido, nunca `false`
fabricado. Coberto por `TestBuildMigrations_RollbackOnlyWhenChecked`.

## §14 Envs — ✅ real
1:1 de `report.env_changes[]`. Gap 🔴 fora de alcance: se a fonte de verdade for painel de infra
e não o repo, precisa de integração externa — não avaliado, não é gap de código e sim de produto.

## §15 Aplicabilidade — ✅ real
`mapApplicability` reusa applicability/limitations já reais. Sem gap.

## §16 Resumo da Release — ✅ parcial
`release_notes.executive_summary`/`breaking_changes` ✅ (`mapSummary`). Gaps 🔴: os "4 blocos"
descritos no contrato como geração LLM por fonte específica (não um prompt genérico) — hoje só
existe o resumo executivo único; `recomendacao`/`proximos_passos` (derivados do gate, mas não
implementados); botões "Copiar resumo"/"Anexar ao PR" (ações novas, exigem integração com PR).

## §17 Rodapé — ✅ real
`run.solidify_version`/`run.id`/`run.finished_at`. Sem gap.

## Comparativos (setas ↑↓ em qualquer seção) — 🔴 bloqueado por §8
Todo `delta` cross-release depende do mesmo histórico do §8. Não há como implementar
parcialmente — ou tem a tabela de histórico, ou a seta não aparece (correto, é o que a UI faz
hoje: omite).

---

## Prioridade sugerida (menor esforço → maior valor primeiro)

1. **`GateRule.Blocking`** (§4) — 1 campo no struct + popular em `run.go`. Menor esforço de
   todos os itens 🔴 listados, desbloqueia "bloqueante" real em vez de aproximado.
2. **Política de gate em `.solidify.yml`** (§4, §4.1, §12 `limite`) — schema novo
   (`criterios[].regra/bloqueante`, `limite_aprovacao` default) — decisão de produto antes de
   codar.
3. **Histórico de runs por repo** (§8) — maior item, mas desbloqueia §2 `score_anterior`,
   §6 `nova_nesta_release`, §11.1 `serie` do Sonar, §12 `serie_p95`, e todos os comparativos
   ↑↓. Candidato a "próxima fase" se o usuário confirmar.
4. ~~Wiring de `migrations[]`/`git.commits[]`/`git.changed_files[]` em `run.go`~~ (§9, §10, §13) —
   **feito**: `internal/app/git_wiring.go` (`buildCommits`/`buildChangedFiles`/`buildMigrations`),
   chamado em `run.go` logo após o diff de evidência. Testes: `git_wiring_test.go` +
   `go test ./internal/app/... ./internal/gitx/... ./internal/migrations/...` verde. `descricao`
   por LLM (§9/§16) e `ReleaseNotes.Groups` seguem 🔴, agora desbloqueados (não mais código morto).
5. Resto (`task_url` em §6, integrações de infra em §14/§16) — enriquecimentos independentes,
   sem bloqueio mútuo, podem ser feitos a qualquer momento.
