// mapping.js (SAI-138 / Fase A3) — uma função por seção. Cada uma lê
// report.analyzers.find(a => a.id === "...") e normaliza pra shape de view.
// Analyzer ausente ou execution_status começando com "skipped" -> {empty:true,
// reason}. Nunca fabrica score/finding: regra do contrato (estado vazio
// explícito).

function findAnalyzer(report, id) {
  return (report.analyzers || []).find((a) => a.id === id);
}

function emptyOf(a, fallbackReason) {
  const status = a && a.execution_status;
  if (!a || (typeof status === "string" && status.startsWith("skipped"))) {
    return { empty: true, reason: (a && a.limitations && a.limitations[0]) || fallbackReason };
  }
  return null;
}

export function mapSonarSection(report) {
  const a = findAnalyzer(report, "sonar");
  const e = emptyOf(a, "sem dados nesta release");
  if (e) return e;
  return {
    empty: false,
    score: a.score,
    metrics: a.metrics || {},
    findings: a.findings || [],
  };
}

export function mapLighthouseSection(report) {
  const a = findAnalyzer(report, "lighthouse");
  const e = emptyOf(a, "sem dados nesta release");
  if (e) return e;
  const m = a.metrics || {};
  return {
    empty: false,
    score: a.score,
    performance: m.performance, accessibility: m.accessibility,
    best_practices: m.best_practices, seo: m.seo,
  };
}

export function mapK6Section(report) {
  const a = findAnalyzer(report, "k6");
  const e = emptyOf(a, "carga não configurada nesta release");
  if (e) return e;
  const m = a.metrics || {};
  return {
    empty: false,
    score: a.score,
    p95Ms: m.p95_ms, p99Ms: m.p99_ms, errorRate: m.error_rate,
    requests: m.requests, throughputRps: m.throughput_rps,
    passThresholds: m.pass_thresholds, thresholdFailed: (a.findings || []).map((f) => f.threshold),
  };
}

export function mapSecuritySection(report) {
  const a = findAnalyzer(report, "security");
  const e = emptyOf(a, "sem dados nesta release");
  if (e) return e;
  const m = a.metrics || {};
  return {
    empty: false,
    score: a.score,
    gateAllow: m.gate_allow, gateReason: m.gate_reason,
    bySeverity: m.by_severity || {}, bySource: m.by_source || {},
    findings: a.findings || [],
  };
}

export function mapCoverageSection(report) {
  const a = findAnalyzer(report, "coverage");
  const e = emptyOf(a, "relatório de coverage não configurado");
  if (e) return e;
  const m = a.metrics || {};
  return {
    empty: false,
    score: a.score,
    linePct: m.line_pct, branchPct: m.branch_pct,
    linesTotal: m.lines_total, linesCovered: m.lines_covered,
    threshold: m.threshold, isGood: m.is_good,
    worstFiles: a.findings || [],
  };
}

// Faixa "Analisadores" (transparência da análise, §5) — visão agregada dos 5.
export function mapAnalyzersStrip(report) {
  const ids = ["sonar", "lighthouse", "security", "k6", "coverage"];
  return ids.map((id) => {
    const a = findAnalyzer(report, id);
    if (!a) return { id, applicability: "NOT_APPLICABLE", executionStatus: "skipped:not_wired" };
    return {
      id, applicability: a.applicability, executionStatus: a.execution_status || "completed",
      score: a.score, durationMs: a.duration_ms,
    };
  });
}

// §15 Aplicabilidade — mesma fonte de mapAnalyzersStrip (analyzers[].
// applicability/limitations, já produzido por applicability.Decide no
// backend), só que como lista nome/aplicável/motivo em vez da faixa de
// score+duração. `motivo` obrigatório quando não aplicável (contrato);
// sem limitations nesse caso, usa fallback fixo em vez de fabricar texto.
const APPLICABILITY_LABELS = { sonar: "SonarQube", lighthouse: "Lighthouse", security: "Segurança", k6: "Performance / Carga (k6)", coverage: "Coverage" };
export function mapApplicability(report) {
  return mapAnalyzersStrip(report).map((it) => {
    const aplicavel = it.applicability === "APPLICABLE";
    return {
      nome: APPLICABILITY_LABELS[it.id] || it.id,
      aplicavel,
      motivo: aplicavel ? null : ((report.limitations || []).find((l) => l.toLowerCase().includes(it.id)) || "não aplicável nesta release"),
    };
  });
}

// §4 Gate de Entrega. Real hoje: quality_gate.rules[] só tem G1 (quality)
// e G2 (independence) — as 6 categorias nomeadas no mockup (score_minimo,
// cobertura, vuln_criticas, ...) não existem como avaliações discretas no
// pipeline. Em vez de inventar as 6, monta um checklist com o que É real:
// as rules do gate + 2 sinais já calculados por outros analyzers
// (security.gate_allow, coverage.is_good) — cada item citando sua fonte.
export function mapGateSection(report) {
  const g = report.quality_gate || {};
  const items = (g.rules || []).map((rule) => ({
    id: rule.id, status: rule.status, message: rule.message, source: "quality_gate",
    blocking: !!rule.blocking,
  }));
  const sec = mapSecuritySection(report);
  if (!sec.empty && sec.gateAllow != null) {
    items.push({ id: "security", status: sec.gateAllow ? "PASS" : "FAIL", message: sec.gateReason, source: "security" });
  }
  const cov = mapCoverageSection(report);
  if (!cov.empty && cov.isGood != null) {
    items.push({ id: "coverage", status: cov.isGood ? "PASS" : "FAIL", message: `${cov.linePct ?? "—"}% (limite ${cov.threshold ?? "—"}%)`, source: "coverage" });
  }
  // "Novas violações SOLID" — derivado de risk.factors[] casado por
  // princípio (mesmo join de mapViolations); só entra se existir ao menos
  // uma violação com princípio identificado, nunca conta pra 0/0 fabricado.
  const solidViolations = mapViolations(report).filter((v) => v.principle);
  if (solidViolations.length) {
    const highSev = solidViolations.filter((v) => v.severity === "high" || v.severity === "critical").length;
    items.push({
      id: "solid_violations", status: highSev ? "FAIL" : "WARN",
      message: `${solidViolations.length} violação(ões) SOLID (${highSev} de alta severidade)`, source: "risk",
    });
  }
  // "Migration reversível" — 1:1 de migrations[].rollback_present.
  const migItems = report.migrations || [];
  if (migItems.length) {
    const noRollback = migItems.filter((m) => m.rollback_present === false);
    items.push({
      id: "migration_rollback", status: noRollback.length ? "FAIL" : "PASS",
      message: noRollback.length ? `${noRollback.length}/${migItems.length} sem rollback` : `${migItems.length} migration(ns) com rollback`,
      source: "migrations",
    });
  }
  return { status: g.status || "INCOMPLETE", reason: g.reason || "", items };
}

// §6 Violações priorizadas. Fonte real: risk.factors[] (RiskFactor não
// carrega arquivo/linha/princípio — Principle.Findings fica sempre vazio
// no pipeline atual, ver internal/app/run.go:projectPrincipleNode). Deriva
// arquivo:linha do 1º evidence_ref (mesmo formato usado em todo o report)
// e casa princípio por overlap de evidence_refs com solid.principles[P]
// (best-effort; sem match, princípio fica "—" — nunca fabricado).
export function mapViolations(report) {
  const factors = (report.risk && report.risk.factors) || [];
  const principles = (report.solid && report.solid.principles) || {};
  const recs = report.recommendations || [];
  return factors.map((f) => {
    const ref = (f.evidence_refs || [])[0] || "";
    const [file, line] = ref.includes(":") ? ref.split(":") : [ref, null];
    let principle = null;
    for (const k of ["S", "O", "L", "I", "D"]) {
      if (((principles[k] || {}).evidence_refs || []).includes(ref)) { principle = k; break; }
    }
    const rec = recs.find((r) => r.title === f.title);
    return {
      id: f.id, severity: f.severity, title: f.title, description: f.description || "",
      file: file || null, line: line || null, principle,
      suggestion: (rec && rec.reason) || null,
    };
  });
}

// §7 Arquivos de risco: join coverage worst-files (findings) x churn
// (git.changed_files). Sem sobreposição -> só cobertura, sem inventar churn.
export function mapRiskFiles(report) {
  const cov = mapCoverageSection(report);
  if (cov.empty) return { empty: true, reason: cov.reason };
  const churnByPath = new Map((report.git?.changed_files || []).map((c) => [c.path, (c.added_lines || 0) + (c.deleted_lines || 0)]));
  const rows = cov.worstFiles.map((f) => ({
    path: f.file_path, coveragePct: f.line_pct, severity: f.severity,
    churn: churnByPath.get(f.file_path) ?? null,
  }));
  rows.sort((a, b) => (b.churn || 0) - (a.churn || 0) || (a.coveragePct || 0) - (b.coveragePct || 0));
  return { empty: false, rows: rows.slice(0, 4) };
}

// §9 O que foi entregue — agrupado por commit.type (Conventional Commits,
// já classificado no backend). `descricao` (resumo 1-linha via LLM) é
// [novo]/indisponível hoje -> omitido, nunca fabricado.
export function mapDeliveries(report) {
  const commits = (report.git && report.git.commits) || [];
  const groups = new Map();
  for (const c of commits) {
    const type = c.type || "outros";
    if (!groups.has(type)) groups.set(type, []);
    groups.get(type).push(c);
  }
  return {
    totalItens: commits.length,
    groups: [...groups.entries()].map(([type, items]) => ({ type, quantidade: items.length, items })),
  };
}

// §10 Arquivos alterados — 1:1 de git.changed_files[].
export function mapChangedFiles(report) {
  const files = (report.git && report.git.changed_files) || [];
  return { total: files.length, items: files };
}

// §13 Migrations — 1:1 de report.migrations[].
export function mapMigrations(report) {
  const items = report.migrations || [];
  if (!items.length) return { empty: true, reason: "nenhuma migration detectada nesta release" };
  return { empty: false, items };
}

// §14 Envs — 1:1 de report.env_changes[].
export function mapEnvs(report) {
  const items = report.env_changes || [];
  if (!items.length) return { empty: true, reason: "nenhuma variável de ambiente nova/alterada" };
  return { empty: false, items };
}

// §5 Analisadores (revisores de IA + consenso) — DISTINTO da faixa de
// tool-analyzers (mapAnalyzersStrip, que serve §15 Aplicabilidade). Fonte:
// ai_review.actors[]/.agreement/.divergences. Campos ricos do mockup
// (escopo, achados, confiança por revisor) não existem no Actor real —
// mostra só o que é real (papel/provider/model/status).
export function mapAIReviewers(report) {
  const ai = report.ai_review || {};
  return {
    mode: ai.mode || "—",
    peerReviewed: !!ai.peer_reviewed,
    agreementPct: typeof ai.agreement === "number" ? ai.agreement * 100 : null,
    independenceDegraded: !!ai.independence_degraded,
    divergences: ai.divergences || [],
    actors: (ai.actors || []).map((a) => ({
      role: a.role, provider: a.provider, modelId: a.model_id,
      status: a.status, fallbackUsed: !!a.fallback_used,
    })),
  };
}

// §16 Resumo da Release.
export function mapSummary(report) {
  const rn = report.release_notes || {};
  if (!rn.executive_summary && !(rn.breaking_changes || []).length) {
    return { empty: true, reason: "resumo executivo não gerado nesta release" };
  }
  return {
    empty: false, summary: rn.executive_summary || "",
    breakingChanges: (rn.breaking_changes || []).map((b) => ({ title: b.title, description: b.description || "" })),
  };
}

// §3 Painel SOLIDIFY — uma entrada por princípio (S O L I D). Fonte única
// pra viewSolidGrid (antes duplicada entre viewSolidifyPanel/viewSOLID em
// main.js — refino visual Fase A1 unificou num grid só, contrato pede isso).
const PRINCIPLE_LABELS = {
  S: "Single Responsibility", O: "Open/Closed", L: "Liskov Substitution",
  I: "Interface Segregation", D: "Dependency Inversion",
};
const PRINCIPLE_CODES = { S: "SRP", O: "OCP", L: "LSP", I: "ISP", D: "DIP" };
export function mapSolidPrinciples(report) {
  const ps = (report.solid && report.solid.principles) || {};
  return ["S", "O", "L", "I", "D"].map((k) => {
    const p = ps[k] || {};
    const applicability = p.applicability || (p.applicable ? "APPLICABLE" : "NOT_APPLICABLE");
    const scored = p.after_score != null && applicability === "APPLICABLE";
    return {
      key: k, code: PRINCIPLE_CODES[k], label: PRINCIPLE_LABELS[k], applicability, scored,
      score: scored ? p.after_score : null,
      delta: typeof p.delta === "number" ? p.delta : null,
      reason: p.reason || null, evidenceRefs: p.evidence_refs || [],
    };
  });
}

// Card "Qualidade Sustentável" do mockup — texto [derivado] do veredicto real
// do gate + violações SOLID + risk.level, nunca métrica inventada. Sem gate
// avaliado -> empty (mesma regra do resto do dashboard, view omite o card).
export function mapSustainability(report) {
  const g = mapGateSection(report);
  if (!g.items.length) return { empty: true, reason: "sem critérios avaliados nesta release" };
  const violations = mapViolations(report).filter((v) => v.principle).length;
  const risk = ((report.risk && report.risk.level) || "—").toLowerCase();
  if (g.status === "PASS") {
    return {
      empty: false, title: "Qualidade sustentável",
      message: `Release atende aos ${g.items.length} critério(s) avaliados, risco ${risk}.`,
    };
  }
  if (g.status === "WARN") {
    return {
      empty: false, title: "Aprovado com ressalvas",
      message: `${violations} violação(ões) SOLID identificada(s), risco ${risk}. Ver gate de entrega.`,
    };
  }
  return {
    empty: false, title: "Risco de regressão",
    message: `Gate ${(g.status || "—").toLowerCase()}: ${violations} violação(ões) SOLID, risco ${risk}.`,
  };
}

// severidade -> classe de cor (usada por violation-item/pill).
export function severityClass(sev) {
  switch ((sev || "").toLowerCase()) {
    case "critical": case "high": return "fail";
    case "medium": return "warn";
    default: return "ok";
  }
}

// §13 impacto (Baixo/Médio/Alto) -> classe de cor, mesmo padrão de
// severityClass/scoreClass. Sem valor (migration sem heurística
// conclusiva, campo omitido) = "neutral", nunca chuta "ok".
export function impactClass(impacto) {
  switch (impacto) {
    case "Alto": return "fail";
    case "Médio": return "warn";
    case "Baixo": return "ok";
    default: return "neutral";
  }
}

// §2 Release Quality Score — veredicto é [derivado] do status do gate
// (contrato: todos os critérios ok -> aprovado; falha não-bloqueante ->
// aprovado_com_ressalvas; falha bloqueante -> reprovado). Sem conceito de
// "bloqueante" no gate real ainda, aproxima por status agregado (PASS/WARN/FAIL).
export function gateVerdict(gateStatus) {
  if (gateStatus === "PASS") return { label: "Aprovado", cls: "ok" };
  if (gateStatus === "WARN") return { label: "Aprovado com ressalvas", cls: "warn" };
  if (gateStatus === "FAIL") return { label: "Reprovado", cls: "fail" };
  return { label: gateStatus || "—", cls: "neutral" };
}

export function scoreClass(value, kind) {
  if (value == null) return "neutral";
  if (kind === "lighthouse") {
    if (value >= 90) return "ok";
    if (value >= 50) return "warn";
    return "fail";
  }
  if (value >= 80) return "ok";
  if (value >= 70) return "warn";
  return "fail";
}
