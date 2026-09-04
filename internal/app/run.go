package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/arbiter"
	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/errs"
	"github.com/diegoaraujo/solidify/internal/gate"
	"github.com/diegoaraujo/solidify/internal/gitx"
	"github.com/diegoaraujo/solidify/internal/peer"
	"github.com/diegoaraujo/solidify/internal/report"
)

// peerReviewPromptTemplate (SAI-129B) — rubrica real (peer.PeerReviewRubric,
// embed de prompts/peer-review.md) + seção de contexto do run.
//
// Corrige 2 bugs do peerBPromptTemplate anterior (SAI-127 MVP):
//  1. identidade hardcoded "Você é o Peer B" — usado também pro peer_a.
//     {{actor_label}} agora vem de RequestOptions.Vars, setado por
//     ator (run.go monta "Peer A"/"Peer B").
//  2. schema de exemplo no shape antigo (applicable bool + after_score) —
//     LLM nunca via o shape SAI-129A de verdade (applicability enum +
//     score condicional). Agora referencia o schema canônico real.
//
// {{heuristic_hints}} (peer.FormatHeuristicHints) injeta o pre-analysis
// determinístico — vazio quando não há hints (compat com testes antigos
// que não passam essa var).
var peerReviewPromptTemplate = peer.PeerReviewRubric + `

## Run context

Você é o {{actor_label}}, revisor independente de princípios SOLID (run_id={{run_id}}, schema_version={{schema_version}}).

{{heuristic_hints}}
Revise as evidências (diff) na próxima mensagem e responda SOMENTE com JSON no schema canônico:
{"solid":{"S":{"applicability":"APPLICABLE|NOT_APPLICABLE|INSUFFICIENT_EVIDENCE","score":0-100_ou_null,"evidence_refs":["..."]},"O":{...},"L":{...},"I":{...},"D":{...}},"quality_score":0-100,"confidence":0-1,"summary":"...","issues":[{"id":"...","severity":"low|medium|high|critical","title":"..."}]}`

// PartialReportDeferred é executado via defer. Se err != nil ou panics acontecerem,
// ele garante que a estrutura atual do Builder gere um release-report.json de falha.
func PartialReportDeferred(builderInput *report.BuilderInput, logger *slog.Logger, currentErr *error) {
	if r := recover(); r != nil {
		logger.Error("panic capturado gerando partial report", "panic", r)
		panicErr := fmt.Errorf("panic: %v", r)
		*currentErr = errs.New(errs.CodeIncomplete, panicErr.Error())
	}

	if *currentErr != nil {
		builderInput.FinishedAt = time.Now()
		// Determina estágio
		if builderInput.FinalGate == nil {
			// Não chegou até o fim
			builderInput.FinalGate = &report.FinalGate{
				Status: "INCOMPLETE",
				Reason: "AI Provider failure or unhandled exception",
			}
		}

		rep, err := report.NewBuilder(*builderInput).Build()
		if err == nil {
			// SAI-129: FinalGate já setado (pelo caller, ANTES do erro) com o
			// veredicto real do gate (FAIL/BLOCKED/INCOMPLETE) reflete um run
			// que completou até a avaliação — status/failed_stage do envelope
			// tem que refletir isso, não achatar tudo em "INCOMPLETE" genérico
			// (bug: consumidor que checa status==INCOMPLETE achava que faltou
			// dado num run que só reprovou score).
			rep.Status = builderInput.FinalGate.Status
			if builderInput.FinalGate.Status == "INCOMPLETE" {
				rep.FailedStage = "AI_ORCHESTRATION"
			} else {
				rep.FailedStage = "QUALITY_GATE"
			}

			if perr, ok := (*currentErr).(*ai.ProviderError); ok {
				rep.ReasonCode = 21 // fallback code for AI
				rep.Failure = map[string]any{
					"message":  perr.Error(),
					"category": perr.Category,
				}
			} else {
				rep.ReasonCode = errs.CodeOf(*currentErr).ExitCode()
				rep.Failure = map[string]any{
					"message": (*currentErr).Error(),
				}
			}

			_ = writeReport(rep)
			logger.Warn("release-report.json escrito após falha.", "status", rep.Status)
		}
	}
}

// dashboardStoreDir é o default que dashboard.Config.StoreDir e
// `solidify report --format=pdf|json` usam quando --store não é passado
// (internal/dashboard/dashboard.go, internal/app/report.go) — writeReport
// precisa gravar aqui também ou os dois comandos nunca enxergam um run.
const dashboardStoreDir = "out/runs"

// writeReport grava o release-report.json em .solidify/out/ (path fixo,
// mesmo destino usado no caminho de sucesso e no de falha via
// PartialReportDeferred — mantido por compat, sobrescrito a cada run) e
// ADICIONALMENTE em out/runs/<run_id>.json (histórico por run_id, o que
// dashboard/report --format=pdf|json esperam por padrão). Sem a 2ª
// gravação, `solidify dashboard`/`solidify report --format=pdf` nunca
// encontram nenhum run — achado da validação e2e manual do usuário.
func writeReport(rep *report.Report) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(".solidify/out", 0755); err != nil {
		return err
	}
	if err := os.WriteFile(".solidify/out/release-report.json", b, 0644); err != nil {
		return err
	}
	if rep.Run.ID == "" {
		return nil // sem run_id não dá pra nomear o arquivo do histórico; fixo já foi gravado
	}
	if err := os.MkdirAll(dashboardStoreDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dashboardStoreDir, rep.Run.ID+".json"), b, 0644)
}

// runRun implementa `solidify run` (SAI-127): orquestra peer_a → peer_b →
// arbiter → gate → report de verdade. Antes disso (Fase 0) era stub.
func runRun(args []string, env Env, logger *slog.Logger) (retErr error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profileName := fs.String("profile", gate.ProfileQuick, "perfil: quick|release|contractual")
	base := fs.String("base", "HEAD~1", "ref base do diff")
	head := fs.String("head", "HEAD", "ref head do diff")
	dir := fs.String("dir", ".", "diretório do repositório git")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			_, werr := io.WriteString(env.Stdout, "uso: solidify run [--profile=quick|release|contractual] [--base=REF] [--head=REF] [--dir=PATH]\n")
			return werr
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para run", err)
	}
	if !gate.ValidProfile(*profileName) {
		return errs.Newf(errs.CodeUsage, "perfil inválido: %s", *profileName)
	}

	runID := report.NewRunID("run", time.Now())
	bInput := report.BuilderInput{
		RunID:     runID,
		Profile:   *profileName,
		StartedAt: time.Now(),
		Git:       report.GitInfo{BaseRef: *base, HeadRef: *head},
	}
	defer PartialReportDeferred(&bInput, logger, &retErr)

	loaded, err := config.Load("", config.Overrides{})
	if err != nil {
		return errs.Wrap(errs.CodeConfig, "carregar solidify.json", err)
	}
	cfg := loaded.Config
	bInput.ConfigHash = loaded.Hash

	prof, ok := cfg.Profiles[*profileName]
	if !ok {
		return errs.Newf(errs.CodeConfig, "perfil %q ausente na config", *profileName)
	}
	requirePeerA := boolOr(prof.RequirePeerA, true)
	requirePeerB := boolOr(prof.RequirePeerB, false)
	requireArbiter := boolOr(prof.RequireArbiter, false)
	requireDistinct := boolOr(prof.RequireDistinctExternalModels, false)

	// SAI-135: falha cedo se o profile exige ator sem model preferido —
	// antes de gastar diff/tokens (firstPreferred() só detectava isso depois).
	if verr := cfg.ValidateActiveProfile(*profileName); verr != nil {
		return verr
	}

	ctx := context.Background()

	// Evidência: diff real entre --base e --head (internal/gitx).
	diffFiles, err := gitx.Diff(*dir, *base+".."+*head, gitx.DiffOpts{})
	if err != nil {
		return errs.Wrap(errs.CodeGit, "gerar diff de evidência", err)
	}
	shardSet, err := peer.NewEvidenceShardSet(runID, diffFilesToShards(diffFiles))
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "montar evidence shard set", err)
	}
	bInput.EvidenceHash = shardSet.Hash()

	// SAI-138: wiring dos 5 analyzers determinísticos (Sonar/Lighthouse/
	// Security/k6/Coverage) — cada gate decide applicability sobre o diff
	// real antes de rodar exec/rede; skip nunca fabrica score.
	changedPaths := diffPaths(diffFiles)
	bInput.Analyzers = buildAnalyzers(ctx, cfg, *dir, changedPaths, hasMigrationChanges(changedPaths), hasEnvChanges(changedPaths), logger)

	// Wiring de git.commits[]/git.changed_files[]/migrations[] — antes
	// disso nenhum dos 3 arrays era populado em produção (só na fixture
	// estática), gap documentado em docs/dashboard-gap-mapping.md §9/§10/§13.
	rng := *base + ".." + *head
	if commits, cErr := buildCommits(*dir, rng); cErr != nil {
		logger.Warn("git log falhou, commits[] fica vazio", "err", cErr)
	} else {
		bInput.Git.Commits = commits
	}
	changes, chErr := gitx.Changes(*dir, rng, gitx.ChangesOpts{RenameDetection: cfg.Git.RenameDetection, CopyDetection: cfg.Git.CopyDetection})
	if chErr != nil {
		logger.Warn("git diff --numstat falhou, changed_files[]/migrations[] ficam vazios", "err", chErr)
	} else {
		bInput.Git.ChangedFiles = buildChangedFiles(changes)
		bInput.Migrations = buildMigrations(*dir, changes, cfg.Detectors)
	}

	// SAI-129B: heurística determinística de applicability roda 1x sobre
	// o diff bruto, ANTES de qualquer peer — mesmo sinal pros dois (mesmo
	// diff). Se os 5 princípios vierem CLEARLY_NOT_APPLICABLE (golden case
	// do rename), nenhum peer chama LLM (peer.SynthesizeHeuristicResult).
	hints := peer.ClassifyApplicability(diffFiles)
	heuristicSkip := peer.AllNotApplicable(hints)
	heuristicHintsText := peer.FormatHeuristicHints(hints)

	// Provider 9Router: mesma resolução host/token do doctor (SAI-113).
	host := cfg.AI.ExternalProvider.BaseURLDocker
	if ai.DetectNetworkContext() == ai.NetworkHost {
		host = cfg.AI.ExternalProvider.BaseURLHost
	}
	keyEnv := cfg.AI.ExternalProvider.APIKeyEnv
	if keyEnv == "" {
		keyEnv = ai.NineRouterTokenEnv
	}
	provider := ai.NewOpenAIProvider(host, os.Getenv(keyEnv)).WithProviderName("9router")
	timeout := time.Duration(cfg.AI.ExternalProvider.RequestTimeoutSeconds) * time.Second

	// --- peer_a ---
	var peerARes *peer.ExecutorResult
	var peerACalled bool
	if requirePeerA {
		model, merr := firstPreferred(cfg.AI.Selection.PeerA, requirePeerA)
		if merr != nil {
			return merr
		}
		if heuristicSkip {
			// SAI-129B: 0 tokens gastos — heurística já resolveu os 5
			// princípios como CLEARLY_NOT_APPLICABLE.
			peerARes, peerACalled = peer.SynthesizeHeuristicResult("peer_a", "heuristic", "heuristic", hints), true
		} else {
			peerAReq, rerr := peer.NewRequestBuilder(peerReviewPromptTemplate, "1").Build(peer.RequestOptions{
				RunID: runID, Actor: "peer_a", Evidence: shardSet,
				Vars: map[string]string{"actor_label": "Peer A", "heuristic_hints": heuristicHintsText},
			})
			if rerr != nil {
				return errs.Wrap(errs.CodeInternal, "montar request peer_a", rerr)
			}
			storeDir, serr := resolveStoreDir(cfg.AI.PeerA.StoreDir)
			if serr != nil {
				return serr
			}
			res, called, perr := peer.ExecutePeerA(ctx, peer.PeerAOptions{
				Source: peer.ResolveSource(cfg.AI.PeerA.Source), Request: peerAReq,
				Provider: provider, Model: model, RunID: runID, StoreDir: storeDir, Timeout: timeout,
			})
			if perr != nil {
				logger.Warn("peer_a falhou", "err", perr)
			}
			peerARes, peerACalled = res, called
		}
	}

	// --- peer_b (isolado — nunca vê peerARes.Content) ---
	var peerBRes *peer.ExecutorResult
	var peerBCalled bool
	if requirePeerB {
		model, merr := firstPreferred(cfg.AI.Selection.PeerB, requirePeerB)
		if merr != nil {
			return merr
		}
		if heuristicSkip {
			peerBRes, peerBCalled = peer.SynthesizeHeuristicResult("peer_b", "heuristic", "heuristic", hints), true
		} else {
			peerBReq, rerr := peer.NewRequestBuilder(peerReviewPromptTemplate, "1").Build(peer.RequestOptions{
				RunID: runID, Actor: "peer_b", Evidence: shardSet,
				Vars: map[string]string{"actor_label": "Peer B", "heuristic_hints": heuristicHintsText},
			})
			if rerr != nil {
				return errs.Wrap(errs.CodeInternal, "montar request peer_b", rerr)
			}
			res, perr := peer.NewExecutor().Execute(ctx, peer.ExecutorOptions{
				Request: peerBReq, Provider: provider, Model: model, Timeout: timeout,
			})
			if perr != nil {
				logger.Warn("peer_b falhou", "err", perr)
			} else {
				peerBRes, peerBCalled = res, true
			}
		}
	}

	// --- divergência (auditável; alimenta o arbiter) ---
	var divergence *peer.DivergenceMap
	if peerACalled && peerBCalled {
		dm, derr := peer.Compute(
			parsedToPeerScores(runID, "A", peerARes.ParsedContent),
			parsedToPeerScores(runID, "B", peerBRes.ParsedContent),
		)
		if derr == nil {
			divergence = dm
		}
	}

	// SAI-129C §9: divergência de applicability (peer_a/peer_b discordaram se
	// um princípio sequer se aplica) força o arbiter a resolver
	// applicability_verdicts+score_verdicts por princípio, não só resolutions
	// genéricas — ver arbiter.ExecutorOptions.RequirePrincipleVerdicts.
	requirePrincipleVerdicts := divergence != nil && len(divergence.ApplicableMismatch) > 0

	// --- arbiter ---
	var arbiterRes *arbiter.ExecutorResult
	var arbiterCalled bool
	if requireArbiter {
		model, merr := firstPreferred(config.ModelChoice{Preferred: cfg.AI.Selection.Arbiter.Preferred}, requireArbiter)
		if merr != nil {
			return merr
		}
		divJSON := ""
		if divergence != nil {
			if b, jerr := json.Marshal(divergence); jerr == nil {
				divJSON = string(b)
			}
		}
		exec, eerr := arbiter.NewExecutor(arbiter.ExecutorOptions{
			RunID: runID, Evidence: peer.FormatRequest(nil), PeerAOutput: resultContent(peerARes),
			PeerBOutput: resultContent(peerBRes), DivMap: divJSON,
			Provider: provider, Model: model, Timeout: timeout,
			RequirePrincipleVerdicts: requirePrincipleVerdicts,
		})
		if eerr != nil {
			return errs.Wrap(errs.CodeInternal, "construir arbiter", eerr)
		}
		res, aerr := exec.Execute(ctx)
		if aerr != nil {
			logger.Warn("arbiter falhou", "err", aerr)
		}
		arbiterRes, arbiterCalled = res, res != nil && res.Verdict != nil
		if !arbiterCalled && res != nil {
			logger.Warn("arbiter não produziu verdict",
				"errors", res.Errors, "validation_errors", res.ValidationErrors,
				"score_status", res.ScoreStatus, "content", res.Content)
		}
	}

	// --- score global: peer_b tem precedência (2ª opinião real); senão peer_a ---
	globalScore, scoreStatus := 0.0, peer.ScoreStatusUnavailable
	switch {
	case peerBCalled:
		globalScore, scoreStatus = peerBRes.QualityScore, peerBRes.ScoreStatus
	case peerACalled:
		globalScore, scoreStatus = peerARes.QualityScore, peerARes.ScoreStatus
	}

	// SAI-129 §7 (achado do usuário, 2026-09-02): AggregateSolidScore() é
	// universalmente a fonte de verdade do score global — quality_score
	// autorreportado (switch acima) nunca deveria decidir o gate. Antes,
	// isso só substituía o switch no path estreito de arbiter+divergência
	// de applicability, deixando o caso comum (peer_a/peer_b sem
	// disputa) ler o autorreportado direto — confirmado via e2e
	// (TestRunRun_GlobalScoreSourceOfTruth) que o gate usava 99 em vez de
	// 30 pra S=10,O=20,L=30,I=40,D=50. Só cai pro autorreportado quando
	// não há NENHUM princípio legível (schema falhou/execução com erro)
	// ou quando a heurística (SAI-129B) já resolveu o score sozinha.
	resolvedSolid := buildResolvedSolid(peerACalled, peerARes, peerBCalled, peerBRes, arbiterRes)
	if len(resolvedSolid) > 0 && scoreStatus == peer.ScoreStatusAvailable {
		score, status := peer.AggregateSolidScore(map[string]any{"solid": resolvedSolid})
		scoreStatus = mapSolidScoreStatus(status)
		if score != nil {
			globalScore = *score
		} else {
			globalScore = 0
		}
	}

	gateIn := gate.GateInput{
		RunID: runID, Profile: *profileName,
		GlobalScore: globalScore, ScoreStatus: scoreStatus,
		EvidenceComplete: true,
		ArbiterResolved:  !requireArbiter || arbiterCalled,
		Independence: gate.IndependenceInput{
			PeerACalled: peerACalled, PeerBCalled: peerBCalled, ArbiterCalled: arbiterCalled,
			PeerAModel: modelOf(peerARes), PeerBModel: modelOf(peerBRes),
			RequirePeerA: requirePeerA, RequirePeerB: requirePeerB, RequireArbiter: requireArbiter,
			RequireDistinctModels: requireDistinct,
		},
	}
	gateRes, err := gate.Evaluate(gateIn)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "avaliar gate", err)
	}

	_, distinct, _ := gateIn.Independence.Evaluate()
	bInput.FinishedAt = time.Now()
	bInput.AIReview = report.AIReview{
		Mode: *profileName, PeerReviewed: peerBCalled,
		Actors:               buildActors(peerARes, peerBRes, arbiterRes),
		IndependenceDegraded: gateRes.Subgates.Independence.Status != gate.StatusPass,
		Divergences:          divergenceStrings(divergence),
	}
	reportQuality := &globalScore
	if scoreStatus == peer.SolidScoreNotApplicable || scoreStatus == peer.SolidScoreInsufficientEvidence {
		reportQuality = nil
	}
	bInput.Scores = report.ScoresBlock{Quality: reportQuality, ScoreStatus: scoreStatus}

	// SAI-129D: popula SOLIDBlock.Principles + Pillars + RiskBlock +
	// Recommendations + summary a partir do peer que teve precedência
	// (peer_b > peer_a, mesmo critério do globalScore). Sem isso, o
	// builder serializava principles={} e o report/dashboard/PDF saíam
	// zerados (achado do cenário 1).
	parsedForReport := selectParsedContent(peerARes, peerBRes, arbiterRes, requirePrincipleVerdicts)
	solid, pillars, risks, recs, summary := projectPeerResultToReport(parsedForReport)
	bInput.SOLID = solid
	bInput.Scores.Pillars = pillars
	bInput.Risk = risks
	bInput.Recommendations = recs
	if summary != "" {
		bInput.Limitations = append(bInput.Limitations, "review_summary: "+summary)
	}
	bInput.QualityGate = report.QualityGate{
		// Status combinado (gateRes.Status), não só o subgate de quality: G2
		// (independence) também é regra deste gate e pode reprovar sozinho.
		Status: gateRes.Status,
		Rules: []report.GateRule{
			{ID: "G1", Status: gateRes.Subgates.Quality.Status, Message: gateRes.Subgates.Quality.Reason, Blocking: gateRes.Subgates.Quality.Blocking},
			{ID: "G2", Status: gateRes.Subgates.Independence.Status, Message: gateRes.Subgates.Independence.Reason, Blocking: gateRes.Subgates.Independence.Blocking},
		},
	}
	bInput.IndependenceGate = &report.IndependenceGate{
		Status: gateRes.Subgates.Independence.Status, Reason: gateRes.Subgates.Independence.Reason,
		DistinctModels: distinct, PeerACalled: peerACalled, PeerBCalled: peerBCalled, ArbiterCalled: arbiterCalled,
	}
	bInput.FinalGate = &report.FinalGate{
		Status: gateRes.Status, Reason: gateRes.Reason,
		Rules: []report.GateRule{
			{ID: "G1", Status: gateRes.Subgates.Quality.Status, Blocking: gateRes.Subgates.Quality.Blocking},
			{ID: "G2", Status: gateRes.Subgates.Independence.Status, Blocking: gateRes.Subgates.Independence.Blocking},
		},
	}

	rep, berr := report.NewBuilder(bInput).Build()
	if berr != nil {
		return errs.Wrap(errs.CodeInternal, "montar report", berr)
	}
	if werr := writeReport(rep); werr != nil {
		return errs.Wrap(errs.CodeIO, "gravar release-report.json", werr)
	}

	if gateRes.IsBlocking() {
		code := errs.CodeIncomplete
		switch gateRes.Status {
		case gate.StatusFail:
			code = errs.CodeGateFail
		case gate.StatusBlocked:
			code = errs.CodeGateBlocked
		}
		return errs.Newf(code, "gate %s: %s", gateRes.Status, gateRes.Reason)
	}
	return nil
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// projectPeerResultToReport (SAI-129D) projeta um ParsedContent canônico
// (peer.ExecutorResult.ParsedContent) para os blocos do report —
// SOLIDBlock.Principles, Pillars, RiskBlock, Recommendations e summary.
// É o único lugar onde o report ganha a visão por princípio do que o
// peer viu; antes desse helper o Builder recebia bInput.SOLID vazio e
// serializava principles={} (report visualmente inútil).
//
// Sem inventar dado: NOT_APPLICABLE → score=nil, sem achado; INSUFFICIENT_
// EVIDENCE → score=nil, sem achado; APPLICABLE → score+evidence_refs só
// se o peer efetivamente devolveu. Risk/Recommendation são derivados de
// `issues` com mapeamento severidade→nível/prioridade.
func projectPeerResultToReport(parsed map[string]any) (report.SOLIDBlock, []report.Pillar, report.RiskBlock, []report.Recommendation, string) {
	solid := report.SOLIDBlock{Principles: map[string]report.Principle{}}
	pillars := make([]report.Pillar, 0, 5)
	risks := report.RiskBlock{Level: "LOW"}
	var recs []report.Recommendation

	// --- summary ---
	summary, _ := parsed["summary"].(string)

	// --- principles (S/O/L/I/D sempre presentes, mesmo quando ausentes) ---
	if solidRaw, ok := parsed["solid"].(map[string]any); ok {
		principleOrder := []string{"S", "O", "L", "I", "D"}
		for idx, p := range principleOrder {
			node, _ := solidRaw[p].(map[string]any)
			principle := projectPrincipleNode(p, node)
			solid.Principles[p] = principle

			// pillar (id/weight/Applicable/Scored/Score) — mesma precedência.
			pillars = append(pillars, report.Pillar{
				ID:         p,
				Weight:     1.0 / float64(len(principleOrder)),
				Applicable: principle.Applicable,
				Scored:     principle.AfterScore != nil,
				Score:      principle.AfterScore,
			})
			_ = idx
		}
	}

	// --- issues → risk factors + recommendations ---
	if issues, ok := parsed["issues"].([]any); ok {
		highest := "LOW"
		for _, raw := range issues {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			sev, _ := m["severity"].(string)
			title, _ := m["title"].(string)
			refs := extractEvidenceRefs(m)
			if id == "" {
				continue
			}
			risks.Factors = append(risks.Factors, report.RiskFactor{
				ID: id, Severity: sev, Title: title, EvidenceRefs: refs,
			})
			if reason, _ := m["reason"].(string); reason != "" || title != "" {
				recs = append(recs, report.Recommendation{
					Priority: mapSeverityToPriority(sev), Title: title, Reason: reason, EvidenceRefs: refs,
				})
			}
			if rank := severityRank(sev); rank > severityRank(highest) {
				highest = sev
			}
		}
		risks.Level = mapSeverityToLevel(highest)
	}

	if recs == nil {
		recs = []report.Recommendation{}
	}
	return solid, pillars, risks, recs, summary
}

// projectPrincipleNode converte o nó `solid.<P>` do ParsedContent em
// report.Principle preservando applicability/reason/evidence_refs e
// materializando score só quando APPLICABLE + numérico.
func projectPrincipleNode(id string, node map[string]any) report.Principle {
	pr := report.Principle{Applicability: peer.ApplicabilityInsufficientEvidence}
	if node == nil {
		return pr
	}
	if app, ok := node["applicability"].(string); ok {
		pr.Applicability = app
	}
	pr.Applicable = pr.Applicability == peer.ApplicabilityApplicable
	pr.Reason, _ = node["reason"].(string)
	pr.EvidenceRefs = extractEvidenceRefs(node)

	if pr.Applicable {
		if raw, ok := node["score"]; ok {
			if f, ok := raw.(float64); ok {
				pr.AfterScore = &f
			}
		}
		if c, ok := node["confidence"].(float64); ok {
			pr.Confidence = c
		}
	}
	pr.Summary = pr.Reason // Summary == Reason no report canônico atual
	return pr
}

// extractEvidenceRefs lê evidence_refs em qualquer dos 2 shapes comuns
// ([]any vinda de JSON genérico; []string).
func extractEvidenceRefs(m map[string]any) []string {
	raw, ok := m["evidence_refs"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		if len(v) == 0 {
			return nil
		}
		out := make([]string, len(v))
		copy(out, v)
		return out
	case []any:
		if len(v) == 0 {
			return nil
		}
		out := make([]string, 0, len(v))
		for _, r := range v {
			if s, ok := r.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 4
	case "high":
		return 3
	case "moderate", "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

// mapSeverityToLevel / mapSeverityToPriority: severidade do finding (low/
// moderate/high/critical) → enum que RiskBlock.Level e Recommendation.
// Priority aceitam (LOW/MODERATE/HIGH/CRITICAL e low/medium/high/critical
// respectivamente).
func mapSeverityToLevel(s string) string {
	switch s {
	case "critical":
		return "CRITICAL"
	case "high":
		return "HIGH"
	case "moderate", "medium":
		return "MODERATE"
	default:
		return "LOW"
	}
}

func mapSeverityToPriority(s string) string {
	switch s {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "moderate", "medium":
		return "medium"
	default:
		return "low"
	}
}

// firstPreferred lê o 1º modelo preferido da config (SAI-127 MVP).
// ponytail: sem ai.Selector real (probing/distinctness policy);
// upgrade quando precisar de distinctness policy de verdade (SAI-128).
func firstPreferred(choice config.ModelChoice, required bool) (string, error) {
	if len(choice.Preferred) == 0 {
		if required {
			return "", errs.New(errs.CodeConfig, "ator required sem model preferido configurado (selection.*.preferred vazio)")
		}
		return "", nil
	}
	return choice.Preferred[0], nil
}

func diffFilesToShards(files []gitx.DiffFile) []peer.EvidenceShard {
	shards := make([]peer.EvidenceShard, 0, len(files))
	for i, f := range files {
		var content string
		for _, h := range f.Hunks {
			content += h.Header + "\n" + h.Content + "\n"
		}
		shards = append(shards, peer.EvidenceShard{
			ID: fmt.Sprintf("diff-%d", i), Kind: "diff", Content: content, SourceFile: f.Path,
		})
	}
	return shards
}

// parsedToPeerScores converte o ParsedContent (schema canônico SAI-129A:
// `applicability`+`score` por princípio) pra peer.PeerScores, que
// alimenta peer.Compute() (divergência) e o arbiter.
//
// SAI-129C: reescrito pra ler o shape novo via
// peer.PrincipleApplicabilityAndScore — antes lia só o shape legado
// (`applicable` bool / `after_score.value`), que o LLM não devolve mais
// desde SAI-129A. Resultado do bug antigo: Scores/Applicable sempre
// vinham vazios em qualquer run real (não-heurístico), e
// peer.Compute() nunca via divergência de verdade — arbiter recebia
// DivMap sempre "sem mismatch" mesmo quando os peers discordavam.
func parsedToPeerScores(runID, source string, parsed map[string]any) peer.PeerScores {
	ps := peer.PeerScores{RunID: runID, Source: source, Scores: map[string]float64{}, Applicable: map[string]bool{}}
	solid, _ := parsed["solid"].(map[string]any)
	for _, k := range []string{"S", "O", "L", "I", "D"} {
		node, ok := solid[k].(map[string]any)
		if !ok {
			continue
		}
		applicability, score, scored := peer.PrincipleApplicabilityAndScore(node)
		ps.Applicable[k] = applicability == peer.ApplicabilityApplicable
		if scored {
			ps.Scores[k] = score
		}
	}
	if issues, ok := parsed["issues"].([]any); ok {
		for _, raw := range issues {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			sev, _ := m["severity"].(string)
			if id == "" {
				continue
			}
			ps.Findings = append(ps.Findings, peer.FindingLite{ID: id, Severity: sev})
		}
	}
	return ps
}

// buildResolvedSolid monta o nó `solid.<P>` pós-arbitragem (SAI-129C §9):
// base é o consenso do peer que rodou (peer_b > peer_a, mesma precedência
// do switch de globalScore); o arbiter SÓ precisa se pronunciar sobre os
// princípios que de fato divergiram (ApplicabilityVerdicts/ScoreVerdicts),
// então os demais ficam com o valor bruto do peer. Resultado alimenta
// peer.AggregateSolidScore via peer.PrincipleApplicabilityAndScore (mesma
// leitura usada em parsedToPeerScores — não duplica o parsing).
func buildResolvedSolid(peerACalled bool, peerA *peer.ExecutorResult, peerBCalled bool, peerB *peer.ExecutorResult, arb *arbiter.ExecutorResult) map[string]any {
	solid := map[string]any{}
	var base map[string]any
	switch {
	case peerBCalled && peerB != nil:
		base, _ = peerB.ParsedContent["solid"].(map[string]any)
	case peerACalled && peerA != nil:
		base, _ = peerA.ParsedContent["solid"].(map[string]any)
	}
	for _, p := range []string{"S", "O", "L", "I", "D"} {
		if node, ok := base[p]; ok {
			solid[p] = node
		}
	}
	if arb == nil || arb.Verdict == nil {
		return solid
	}

	scoreByPrinciple := map[string]arbiter.ScoreVerdict{}
	for _, sv := range arb.Verdict.ScoreVerdicts {
		scoreByPrinciple[sv.Principle] = sv
	}
	for _, av := range arb.Verdict.ApplicabilityVerdicts {
		node := map[string]any{
			"applicability": av.Applicability,
			"reason":        av.Reasoning,
			"score":         nil,
		}
		if sv, ok := scoreByPrinciple[av.Principle]; ok && sv.Score != nil {
			node["score"] = *sv.Score
		}
		solid[av.Principle] = node
	}
	return solid
}

// mapSolidScoreStatus converte peer.SolidScoreStatus (agregação
// determinística) pro vocabulário que gate.GateInput.ScoreStatus já
// entende (gate.go trata "NOT_APPLICABLE" e "unavailable" como casos
// especiais não-FAIL/INCOMPLETE) — SolidScoreNotApplicable já É a string
// literal "NOT_APPLICABLE" (aggregate.go), então passa direto.
func mapSolidScoreStatus(s string) string {
	switch s {
	case peer.SolidScoreAvailable, peer.SolidScorePartial:
		return peer.ScoreStatusAvailable
	case peer.SolidScoreNotApplicable:
		return peer.SolidScoreNotApplicable
	default: // SolidScoreInsufficientEvidence
		return peer.ScoreStatusUnavailable
	}
}

// selectParsedContent (SAI-129D) escolhe o peer source para
// projectPeerResultToReport com a mesma precedência de score/resolved:
// arbiterResolved > peer_b > peer_a. Sem arbiter, mantém peer_b como
// "2ª opinião" canônica; com arbiter, usa buildResolvedSolid para
// carregar applicability/score pós-arbitragem. Summary/issues saem do
// peer_b quando há, senão peer_a — mesma precedência.
func selectParsedContent(peerA *peer.ExecutorResult, peerB *peer.ExecutorResult, arb *arbiter.ExecutorResult, arbiterResolved bool) map[string]any {
	if arbiterResolved {
		solid := buildResolvedSolid(peerA != nil, peerA, peerB != nil, peerB, arb)
		out := map[string]any{"solid": nil}
		out["solid"] = solid
		return mergeSummaryIssues(out, peerA, peerB)
	}
	switch {
	case peerB != nil && len(peerB.ParsedContent) > 0:
		return peerB.ParsedContent
	case peerA != nil && len(peerA.ParsedContent) > 0:
		return peerA.ParsedContent
	}
	return map[string]any{}
}

// mergeSummaryIssues preserva summary/issues do peer escolhido quando o
// arbiter resolveu só os princípios — sem isso o report perde a
// explicação textual da revisão.
func mergeSummaryIssues(base map[string]any, peerA *peer.ExecutorResult, peerB *peer.ExecutorResult) map[string]any {
	src := peerB
	if src == nil || len(src.ParsedContent) == 0 {
		src = peerA
	}
	if src == nil {
		return base
	}
	for _, k := range []string{"summary", "issues"} {
		if v, ok := src.ParsedContent[k]; ok {
			base[k] = v
		}
	}
	return base
}

func resultContent(r *peer.ExecutorResult) string {
	if r == nil {
		return ""
	}
	return r.Content
}

func modelOf(r *peer.ExecutorResult) string {
	if r == nil {
		return ""
	}
	return r.Model
}

func divergenceStrings(d *peer.DivergenceMap) []string {
	if d == nil || !d.HasMismatch() {
		return nil
	}
	return []string{peer.RenderDivergence(d)}
}

func buildActors(peerA, peerB *peer.ExecutorResult, arb *arbiter.ExecutorResult) []report.Actor {
	var actors []report.Actor
	if peerA != nil {
		actors = append(actors, report.Actor{
			Role: "peer_a", Provider: peerA.Provider, ModelID: peerA.Model, Status: peerA.ScoreStatus,
			RequestedModel: peerA.RequestedModel, ExecutedModel: peerA.ResolvedModel,
		})
	}
	if peerB != nil {
		actors = append(actors, report.Actor{
			Role: "peer_b", Provider: peerB.Provider, ModelID: peerB.Model, Status: peerB.ScoreStatus,
			RequestedModel: peerB.RequestedModel, ExecutedModel: peerB.ResolvedModel,
		})
	}
	if arb != nil {
		actors = append(actors, report.Actor{
			Role: "arbiter", Provider: arb.Provider, ModelID: arb.Model, Status: arb.ScoreStatus,
			RequestedModel: arb.RequestedModel, ExecutedModel: arb.ResolvedModel,
		})
	}
	return actors
}
