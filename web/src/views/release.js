// views/release.js — seções do contrato ainda faltando no dashboard
// (§4,6,7,9,10,13,14,5,16,17). Extraído pra cá quando main.js passou de
// ~500 linhas (guidance do plano de split, ES modules sem bundler).
import {
  mapGateSection, mapViolations, mapRiskFiles, mapDeliveries,
  mapChangedFiles, mapMigrations, mapEnvs, mapAIReviewers, mapSummary,
  mapApplicability, severityClass, scoreClass, impactClass,
} from "../mapping.js";
import { el, emptyCard, gaugeTile } from "../dom.js";

const GATE_CRITERION_LABELS = {
  security: "Vulnerabilidades", coverage: "Cobertura de testes",
  solid_violations: "Violações SOLID", migration_rollback: "Migration reversível",
  G1: "G1", G2: "G2",
};

function statusPill(status) {
  const cls = status === "PASS" ? "ok" : status === "WARN" ? "warn" : status === "INCOMPLETE" ? "neutral" : "fail";
  return el("span", { class: "pill " + cls }, status);
}

// §4 Gate de Entrega — grid de critérios (layout do mockup), cada card com
// o que É real e avaliado nesta release (mapGateSection nunca fabrica as
// 6 categorias fixas do mockup; se só 2 rules rodaram, mostra 2).
export function viewGateSection(report) {
  const g = mapGateSection(report);
  const passed = g.items.filter((it) => it.status === "PASS").length;
  const wrap = el("div", { class: "card full-width" },
    el("div", { class: "card-header" }, "Gate de entrega",
      el("span", { class: "muted" }, " — ", String(passed), " de ", String(g.items.length), " critério(s) atendidos"),
      statusPill(g.status)));
  const body = el("div", { class: "card-body" });
  if (!g.items.length) {
    body.appendChild(el("div", { class: "empty-state" }, "sem critérios avaliados nesta release"));
  } else {
    const grid = el("div", { class: "gate-grid" });
    for (const it of g.items) {
      grid.appendChild(el("div", { class: "gate-criterion" },
        el("div", { class: "top-row" }, statusPill(it.status), el("span", { class: "muted" }, it.source)),
        el("p", { class: "title" }, GATE_CRITERION_LABELS[it.id] || it.id),
        it.message ? el("p", { class: "muted" }, it.message) : null,
        it.blocking ? el("span", { class: "pill fail" }, "bloqueante") : null));
    }
    body.appendChild(grid);
  }
  wrap.appendChild(body);
  return wrap;
}

// §6 Violações priorizadas (top 4; ver mapping.js pra fonte real)
export function viewViolations(report) {
  const items = mapViolations(report);
  if (!items.length) return emptyCard("Violações priorizadas", "nenhum risco identificado nesta release");
  const wrap = el("div", { class: "card full-width" }, el("div", { class: "card-header" }, "Violações priorizadas"));
  const body = el("div", { class: "card-body" });
  for (const v of items.slice(0, 4)) {
    const row = el("div", { class: "violation-item" },
      el("div", { class: "rail " + severityClass(v.severity) }),
      el("div", { class: "content" },
        el("div", { class: "top-row" },
          el("span", { class: "pill " + severityClass(v.severity) }, v.severity || "?"),
          v.principle ? el("span", { class: "muted" }, " princípio ", v.principle) : null,
          v.file ? el("span", { class: "muted" }, " — ", v.file, v.line ? ":" + v.line : "") : null),
        el("div", { class: "title" }, v.title),
        v.description ? el("div", { class: "desc" }, v.description) : null,
        v.suggestion ? el("div", { class: "suggestion" }, "sugestão: ", v.suggestion) : null));
    body.appendChild(row);
  }
  if (items.length > 4) body.appendChild(el("p", { class: "muted" }, "+", String(items.length - 4), " outras"));
  wrap.appendChild(body);
  return wrap;
}

// §7 Arquivos de risco
export function viewRiskFiles(report) {
  const r = mapRiskFiles(report);
  if (r.empty) return emptyCard("Arquivos de risco", r.reason);
  const wrap = el("div", { class: "card" }, el("div", { class: "card-header" }, "Arquivos de risco"));
  const body = el("div", { class: "card-body" });
  for (const f of r.rows) {
    const track = el("div", { class: "progress-track" });
    if (f.coveragePct != null) {
      track.appendChild(el("div", { class: "progress-fill " + scoreClass(f.coveragePct), style: "width:" + Math.max(0, Math.min(100, f.coveragePct)) + "%" }));
    }
    body.appendChild(el("div", { class: "row" },
      el("div", { class: "col" }, el("code", null, f.path)),
      el("div", { class: "col" }, "cobertura ", String(f.coveragePct ?? "—"), "%", track),
      el("div", { class: "col" }, f.churn != null ? el("span", { class: "muted" }, String(f.churn), " linhas alteradas") : null)));
  }
  wrap.appendChild(body);
  return wrap;
}

// §9 O que foi entregue
export function viewDeliveries(report) {
  const d = mapDeliveries(report);
  if (!d.totalItens) return emptyCard("O que foi entregue", "sem commits no diff analisado");
  const wrap = el("div", { class: "card" },
    el("div", { class: "card-header" }, "O que foi entregue",
      el("span", { class: "muted" }, " (", String(d.totalItens), ")")));
  const body = el("div", { class: "card-body" });
  for (const g of d.groups) {
    body.appendChild(el("p", null, el("strong", null, g.type), " — ", String(g.quantidade), " item(ns)"));
    const ul = el("ul", { class: "bullets" });
    for (const c of g.items.slice(0, 5)) ul.appendChild(el("li", null, c.short_sha || "", " — ", c.subject || ""));
    body.appendChild(ul);
  }
  wrap.appendChild(body);
  return wrap;
}

// §10 Arquivos alterados
export function viewChangedFiles(report) {
  const c = mapChangedFiles(report);
  if (!c.total) return emptyCard("Arquivos alterados", "nenhum arquivo no diff");
  const wrap = el("div", { class: "card" },
    el("div", { class: "card-header" }, "Arquivos alterados",
      el("span", { class: "muted" }, " (", String(c.total), ")")));
  const t = el("table");
  t.appendChild(el("thead", null, el("tr", null, el("th", null, "Caminho"), el("th", null, "+"), el("th", null, "-"))));
  const tbody = el("tbody");
  for (const f of c.items.slice(0, 20)) {
    tbody.appendChild(el("tr", null,
      el("td", null, el("code", null, f.path)),
      el("td", { class: "pos" }, "+" + String(f.added_lines ?? 0)),
      el("td", { class: "neg" }, "-" + String(f.deleted_lines ?? 0))));
  }
  t.appendChild(tbody);
  wrap.appendChild(el("div", { class: "card-body" }, t));
  return wrap;
}

// §13 Migrations
export function viewMigrations(report) {
  const m = mapMigrations(report);
  if (m.empty) return emptyCard("Migrations", m.reason);
  const wrap = el("div", { class: "card" }, el("div", { class: "card-header" }, "Migrations"));
  const ul = el("ul", { class: "bullets" });
  for (const it of m.items) {
    ul.appendChild(el("li", null,
      el("code", null, it.path), " — ", it.framework || "?", " · risco ", it.risk || "?",
      it.impacto ? el("span", { class: "pill " + impactClass(it.impacto) }, " impacto: " + it.impacto + " ") : null,
      it.rollback_present === false ? el("span", { class: "pill fail" }, " sem rollback ") : null));
  }
  wrap.appendChild(el("div", { class: "card-body" }, ul));
  return wrap;
}

// §14 Envs
export function viewEnvs(report) {
  const e = mapEnvs(report);
  if (e.empty) return emptyCard("Variáveis de ambiente", e.reason);
  const wrap = el("div", { class: "card" }, el("div", { class: "card-header" }, "Variáveis de ambiente"));
  const ul = el("ul", { class: "bullets" });
  for (const it of e.items) {
    ul.appendChild(el("li", null,
      el("code", null, it.name), " — ", it.status,
      el("span", { class: "pill " + (it.documented ? "ok" : "warn") }, it.documented ? "documentada" : "sem doc"),
      it.likely_secret ? el("span", { class: "pill fail" }, "possível secret") : null));
  }
  wrap.appendChild(el("div", { class: "card-body" }, ul));
  return wrap;
}

// §15 Aplicabilidade
export function viewApplicability(report) {
  const items = mapApplicability(report);
  const wrap = el("div", { class: "card" }, el("div", { class: "card-header" }, "Aplicabilidade"));
  const ul = el("ul", { class: "bullets" });
  for (const it of items) {
    ul.appendChild(el("li", null,
      el("span", { class: "pill " + (it.aplicavel ? "ok" : "neutral") }, it.aplicavel ? "aplicável" : "não aplicável"),
      " ", it.nome, it.motivo ? el("span", { class: "muted" }, " — ", it.motivo) : null));
  }
  wrap.appendChild(el("div", { class: "card-body" }, ul));
  return wrap;
}

// §5 Analisadores (revisores de IA) — distinto da faixa de tool-analyzers.
export function viewAIReviewers(report) {
  const a = mapAIReviewers(report);
  const wrap = el("div", { class: "card full-width" },
    el("div", { class: "card-header" }, "Revisores (IA)",
      el("span", { class: "muted" }, " — ", a.mode)));
  const body = el("div", { class: "card-body row" });
  for (const actor of a.actors) {
    body.appendChild(el("div", { class: "col" },
      el("strong", null, actor.role),
      el("p", { class: "muted" }, actor.provider, "/", actor.modelId),
      el("span", { class: "pill " + (actor.status === "completed" ? "ok" : "fail") }, actor.status),
      actor.fallbackUsed ? el("p", { class: "muted" }, "fallback usado") : null));
  }
  if (a.agreementPct != null) {
    body.appendChild(el("div", { class: "col" }, gaugeTile("Consenso", a.agreementPct, scoreClass(a.agreementPct), 5)));
  }
  wrap.appendChild(body);
  const footer = el("div", { class: "card-footer" },
    a.independenceDegraded ? el("span", { class: "pill fail" }, "independência degradada") : null);
  wrap.appendChild(footer);
  if (a.divergences.length) {
    const ul = el("ul", { class: "bullets" });
    for (const d of a.divergences) ul.appendChild(el("li", null, d));
    wrap.appendChild(ul);
  }
  return wrap;
}

// §16 Resumo da release
export function viewSummary(report) {
  const s = mapSummary(report);
  if (s.empty) return emptyCard("Resumo da release", s.reason);
  const wrap = el("div", { class: "card full-width" }, el("div", { class: "card-header" }, "Resumo da release"));
  const body = el("div", { class: "card-body" });
  if (s.summary) body.appendChild(el("p", null, s.summary));
  if (s.breakingChanges.length) {
    body.appendChild(el("p", null, el("strong", null, "Breaking changes")));
    const ul = el("ul", { class: "bullets" });
    for (const b of s.breakingChanges) ul.appendChild(el("li", null, el("strong", null, b.title), b.description ? " — " + b.description : ""));
    body.appendChild(ul);
  }
  wrap.appendChild(body);
  return wrap;
}

// §17 Rodapé
export function viewFooter(report) {
  const run = report.run || {};
  return el("footer", { class: "muted", style: "padding:16px 0;text-align:center" },
    "solidify ", run.solidify_version || "dev", " — run ", run.id || "—",
    run.finished_at ? " — " + run.finished_at : "");
}
