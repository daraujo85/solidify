// Narrative analyzer (SAI-138 / Fase A4) — gera o bloco textual de
// ReleaseNotes via LLM, a partir de dados REAIS do Report (score + delta
// + SOLID breakdown + findings de sonar/security/etc). Não fabrica:
// campos vazios significam "não gerado", e o dashboard os esconde.
//
// Padrão de LLM call reusa ai.Provider.CompleteJSON com JSON Schema
// (mesmo do peer review em internal/peer/executor.go). Provider que
// ignora `response_format: json_schema` cai na validação client-side
// do jsonx.ParseContent + extração tolerante de campos ausentes.
//
// Opt-in via cfg.AI.Selection.Narrative.Preferred: lista vazia ⇒
// analyzer não roda, ReleaseNotes fica só com o que buildReleaseNotes
// (determinístico) produziu.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/jsonx"
	"github.com/diegoaraujo/solidify/internal/report"
)

// NarrativeResult saída do analyzer. Campos string vazios = seção
// não-gerada (LLM não rodou, schema falhou, ou provider indisponível).
type NarrativeResult struct {
	ExecutiveSummary    string
	ScoreTrendNarrative string
	SolidInsights       string
	// ResolvedModel é o model que o provider de fato executou (vem do
	// payload HTTP real). Vazio quando o analyzer não rodou.
	ResolvedModel string
}

// NarrativeAnalyzer wiring mínimo: provider + model + timeout. Logger
// é opcional (nil = silencioso).
type NarrativeAnalyzer struct {
	Provider ai.Provider
	Model    string
	Timeout  time.Duration
	Logger   *slog.Logger
}

// narrativeSchema JSON Schema Draft 7 (subset) — força o LLM a devolver
// só 3 strings curtas. Provider com suporte nativo a json_schema recebe
// isso; provider que ignora cai na extração tolerante do jsonx. Campos
// required com minLength=0 para tolerar output parcial sem invalidar o
// schema (o caller só precisa da presença — campo vazio vira omitempty
// no report).
var narrativeSchema = map[string]any{
	"type":                 "object",
	"required":             []string{"executive_summary", "score_trend", "solid_insights"},
	"additionalProperties": false,
	"properties": map[string]any{
		"executive_summary": map[string]any{"type": "string", "minLength": 0, "maxLength": 2000},
		"score_trend":       map[string]any{"type": "string", "minLength": 0, "maxLength": 800},
		"solid_insights":    map[string]any{"type": "string", "minLength": 0, "maxLength": 800},
	},
}

// narrativeSystemPrompt persona + restrição forte de output JSON (mesmo
// padrão do peer review). 1 system + 1 user = setup mínimo; sem shards
// porque o material está embutido no prompt (commits/migrs/envs já são
// pequenos o suficiente pra caber inteiro).
const narrativeSystemPrompt = "You are a release-notes summarizer for a software engineering team. Output ONLY valid JSON conforming to the supplied schema. No prose before or after the JSON. Respond in the language of the supplied evidence."

// Generate roda a chamada LLM uma vez e devolve NarrativeResult.
// Falha (provider/timeout/parse/schema) → todos os campos vazios,
// caller trata como "não gerado". Sem retry: narrativa é nice-to-have,
// não bloqueia o run; gasta 1 tentativa só pra não amplificar custo
// em provider instável.
func (n *NarrativeAnalyzer) Generate(ctx context.Context, in *report.BuilderInput) NarrativeResult {
	if n == nil || n.Provider == nil || n.Model == "" {
		return NarrativeResult{}
	}
	if in == nil {
		return NarrativeResult{}
	}
	prompt := buildNarrativePrompt(in)
	if prompt == "" {
		return NarrativeResult{}
	}
	timeout := n.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := n.Provider.CompleteJSON(pctx, ai.CompleteOptions{
		Model: n.Model,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: narrativeSystemPrompt},
			{Role: ai.RoleUser, Content: prompt},
		},
		JSONSchema: narrativeSchema,
	})
	if err != nil {
		if n.Logger != nil {
			n.Logger.Warn("narrative: provider falhou", "err", err, "model", n.Model)
		}
		return NarrativeResult{}
	}
	parsed, ok := jsonx.ParseContent(res.Content)
	if !ok {
		if n.Logger != nil {
			n.Logger.Warn("narrative: parse falhou", "model", n.Model, "content_preview", truncate(res.Content, 160))
		}
		return NarrativeResult{}
	}
	out := NarrativeResult{
		ExecutiveSummary:    strField(parsed, "executive_summary"),
		ScoreTrendNarrative: strField(parsed, "score_trend"),
		SolidInsights:       strField(parsed, "solid_insights"),
		ResolvedModel:       res.Model,
	}
	// Se TODOS os campos saíram vazios (LLM devolveu campos ausentes/
	// null/vazio), considera não-gerado — caller pode preferir fallback
	// determinístico em vez de mostrar seção vazia no dashboard.
	if out.ExecutiveSummary == "" && out.ScoreTrendNarrative == "" && out.SolidInsights == "" {
		if n.Logger != nil {
			n.Logger.Warn("narrative: LLM devolveu campos vazios", "model", res.Model)
		}
	}
	return out
}

func strField(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// buildNarrativePrompt monta o prompt com TODOS os dados reais já
// computados no run: score + SOLID por princípio + 5 analyzers +
// commits + migrations + envs + risk + recommendations. Sem evidência
// significativa (BuilderInput zerado) → prompt vazio → caller não
// gasta tokens.
//
// Limites de tamanho por seção são deliberadamente conservadores
// (5-10 itens, 240 chars por reason) pra evitar prompt de centenas de
// KB quando o repo tem release gorda. O LLM recebe material
// representativo, não exaustivo.
func buildNarrativePrompt(in *report.BuilderInput) string {
	if in == nil {
		return ""
	}
	hasEvidence := len(in.Git.Commits) > 0 || len(in.Analyzers) > 0 || in.SOLID.Score != nil
	if !hasEvidence {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Run context\n")
	fmt.Fprintf(&b, "run_id=%s profile=%s project=%q base=%s head=%s\n",
		in.RunID, in.Profile, in.ProjectName, in.Git.BaseRef, in.Git.HeadRef)

	b.WriteString("\n## Score global\n")
	if in.Scores.Quality != nil {
		fmt.Fprintf(&b, "quality=%.1f status=%s grade=%s\n", *in.Scores.Quality, in.Scores.ScoreStatus, in.Scores.Grade)
	} else {
		fmt.Fprintf(&b, "quality=null status=%s (sem score nesta release)\n", in.Scores.ScoreStatus)
	}
	if delta := in.SOLID.Delta; delta != nil {
		fmt.Fprintf(&b, "solid_delta=%+.1f\n", *delta)
	}

	b.WriteString("\n## SOLID breakdown\n")
	for _, k := range []string{"S", "O", "L", "I", "D"} {
		p, ok := in.SOLID.Principles[k]
		if !ok {
			continue
		}
		score := "—"
		if p.AfterScore != nil {
			score = fmt.Sprintf("%.1f", *p.AfterScore)
		}
		fmt.Fprintf(&b, "- %s: applicability=%s score=%s reason=%q\n",
			k, p.Applicability, score, truncate(p.Reason, 240))
	}

	if len(in.Analyzers) > 0 {
		b.WriteString("\n## Analyzers\n")
		for _, a := range in.Analyzers {
			score := "—"
			if a.Score != nil {
				score = fmt.Sprintf("%.1f", *a.Score)
			}
			fmt.Fprintf(&b, "- %s (applicability=%s, status=%s): score=%s findings=%d\n",
				a.ID, a.Applicability, derefStr(a.ExecutionStatus), score, len(a.Findings))
		}
	}

	if len(in.Risk.Factors) > 0 {
		b.WriteString("\n## Risk factors (top 5)\n")
		for i, f := range in.Risk.Factors {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- [%s] %s\n", f.Severity, truncate(f.Title, 200))
		}
	}

	if len(in.Git.Commits) > 0 {
		b.WriteString("\n## Commits (top 10)\n")
		for i, c := range in.Git.Commits {
			if i >= 10 {
				break
			}
			mark := ""
			if c.Breaking {
				mark = " [BREAKING]"
			}
			fmt.Fprintf(&b, "- %s %s%s\n", c.ShortSHA, c.Subject, mark)
		}
	}

	if len(in.Migrations) > 0 {
		b.WriteString("\n## Migrations\n")
		for i, m := range in.Migrations {
			if i >= 10 {
				break
			}
			fmt.Fprintf(&b, "- %s (framework=%s, status=%s, risk=%s)\n",
				m.Path, m.Framework, m.Status, m.Risk)
		}
	}

	if len(in.EnvChanges) > 0 {
		b.WriteString("\n## Env changes\n")
		for i, e := range in.EnvChanges {
			if i >= 10 {
				break
			}
			fmt.Fprintf(&b, "- %s (%s)\n", e.Name, e.Status)
		}
	}

	if len(in.Recommendations) > 0 {
		b.WriteString("\n## Recommendations (top 5)\n")
		for i, r := range in.Recommendations {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- [%s] %s\n", r.Priority, truncate(r.Title, 200))
		}
	}

	b.WriteString("\n## Instructions\n")
	b.WriteString("Responda EXCLUSIVAMENTE com JSON válido no schema canônico. 3 campos:\n")
	b.WriteString("- executive_summary: 3-5 frases em português descrevendo o que mudou nesta release (use commits + migrations + envs como fonte primária). Não invente dados.\n")
	b.WriteString("- score_trend: 1-2 frases sobre o estado atual do score. Se não houver histórico, escreva apenas o estado atual sem fabricar tendência.\n")
	b.WriteString("- solid_insights: 1 frase apontando o princípio SOLID mais relevante (positivo ou negativo) com base no breakdown acima. Se nada relevante, devolva string vazia \"\".\n")
	return b.String()
}

func derefStr(p *string) string {
	if p == nil {
		return "—"
	}
	return *p
}

// buildReleaseNotes monta ReleaseNotes a partir de commits já
// classificados pelo backend (Conventional.Type) e migrations detectadas
// — zero tokens gastos. Saída determinística:
//
//   - Groups: 1 ChangeGroup por Conventional.Type com ChangeItem[]
//     (Title=Subject, Type=commit.Type, EvidenceRefs=[ShortSHA]).
//   - BreakingChanges: ChangeItem[] filtrando c.Breaking==true
//     (commit.Type == "feat" + "!" no subject, ver Conventional.Breaking).
//   - ExecutiveSummary: contagem por tipo (fallback quando o LLM
//     não roda; o narrative analyzer sobrescreve se estiver ligado).
//
// Sem commits nem migrations → ReleaseNotes com arrays vazios (nunca
// fabricada com placeholder).
func buildReleaseNotes(commits []report.Commit, migrations []report.Migration) report.ReleaseNotes {
	rn := report.ReleaseNotes{
		Groups:          []report.ChangeGroup{},
		BreakingChanges: []report.ChangeItem{},
	}
	if len(commits) == 0 && len(migrations) == 0 {
		return rn
	}

	groupMap := make(map[string][]report.ChangeItem)
	typeOrder := make([]string, 0, 6)
	for _, c := range commits {
		t := c.Type
		if t == "" {
			t = "other"
		}
		item := report.ChangeItem{
			Title:        c.Subject,
			Type:         t,
			EvidenceRefs: []string{c.ShortSHA},
		}
		if _, seen := groupMap[t]; !seen {
			typeOrder = append(typeOrder, t)
		}
		groupMap[t] = append(groupMap[t], item)
		if c.Breaking {
			rn.BreakingChanges = append(rn.BreakingChanges, item)
		}
	}
	for _, t := range typeOrder {
		rn.Groups = append(rn.Groups, report.ChangeGroup{Type: t, Items: groupMap[t]})
	}

	feat := len(groupMap["feat"])
	fix := len(groupMap["fix"])
	rn.ExecutiveSummary = fmt.Sprintf("%d feature(s), %d fix(es), %d migration(s) nesta release.", feat, fix, len(migrations))
	return rn
}
