// Solidify dashboard bootstrap (SAI-080).
// Single-file Preact-equivalent: vanilla JS + DOM diffing manual.
// Sem build step no runtime — browser parseia direto.

import {
  mapSonarSection, mapLighthouseSection, mapK6Section,
  mapSecuritySection, mapCoverageSection, mapAnalyzersStrip, mapApplicability,
  mapSolidPrinciples, mapTopbar, mapSustainability,
  scoreClass, severityClass, gateVerdict,
} from "./mapping.js";
import { el, badge, emptyCard, animateNumber, animateWidth, gaugeTile, svgIcon } from "./dom.js";
import {
  viewGateSection, viewViolations, viewRiskFiles, viewDeliveries,
  viewChangedFiles, viewMigrations, viewEnvs, viewAIReviewers,
  viewApplicability, viewSummary, viewFooter,
} from "./views/release.js";
import { hydrateMockup } from "./hydrate.js";

const root = document.getElementById("root");
const runNav = document.getElementById("runNav");
const hash = () => location.hash.replace(/^#\/?/, "") || "runs";

// Sub-nav de âncoras — só existe na página de run (§ da Fase A2).
// ponytail: sem scrollspy (destacar item ativo ao rolar); upgrade se pedirem.
function clearRunNav() { while (runNav.firstChild) runNav.removeChild(runNav.firstChild); }
function paintRunNav(items) {
  clearRunNav();
  for (const [id, label] of items) {
    runNav.appendChild(el("a", {
      href: "#",
      onclick: (e) => { e.preventDefault(); document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" }); },
    }, label));
  }
}

// Variante da sub-nav pra página de run em modo mockup (§ mockup real via
// iframe) — âncoras apontam pra data-dc-tpl dentro do iframe, não IDs do
// document principal.
function paintRunNavTpl(iframeDoc, items) {
  clearRunNav();
  for (const [tpl, label] of items) {
    runNav.appendChild(el("a", {
      href: "#",
      onclick: (e) => {
        e.preventDefault();
        iframeDoc.querySelector(`[data-dc-tpl="${tpl}"]`)?.scrollIntoView({ behavior: "smooth", block: "start" });
      },
    }, label));
  }
}

async function loadRun(runID) {
  const r = await fetch(`/api/run/${encodeURIComponent(runID)}`);
  if (!r.ok) throw new Error("run " + runID);
  return r.json();
}

async function loadRuns() {
  const r = await fetch(`/api/runs`);
  if (!r.ok) throw new Error("runs");
  return r.json();
}

// SAI-125: trend de v1_fraction ao longo do tempo.
async function loadTrend() {
  const r = await fetch("/api/peer-reviews/trend");
  if (!r.ok) throw new Error("trend");
  return r.json();
}

function score(n) {
  if (n == null || Number.isNaN(n)) return el("div", { class: "score na" }, "N/A");
  const node = el("div", { class: "score" }, "0");
  animateNumber(node, n, { decimals: 0, duration: 800 });
  return node;
}

function viewRuns(runs) {
  const wrap = el("div");
  wrap.appendChild(el("h2", null, "Runs"));
  const t = el("table");
  t.appendChild(el("thead", null,
    el("tr", null,
      el("th", null, "Run ID"),
      el("th", null, "Profile"),
      el("th", null, "Gate"),
      el("th", null, "Score"),
      el("th", null, "Risk"))));
  const tbody = el("tbody");
  for (const r of runs) {
    tbody.appendChild(el("tr", null,
      el("td", null,
        el("a", { href: `#/run/${encodeURIComponent(r.run_id)}` }, r.run_id)),
      el("td", null, r.profile),
      el("td", null, badge(r.gate_status || "INCOMPLETE")),
      el("td", null, String(Math.round(r.quality || 0))),
      el("td", null, r.risk_level || "LOW")));
  }
  t.appendChild(tbody);
  wrap.appendChild(t);
  return wrap;
}

// SAI-119: cabeçalho com o escopo da entrega (git.* do release-report.json).
function viewGitHeader(r) {
  const git = r.git || {};
  const wrap = el("div", { class: "card" });
  wrap.appendChild(el("h3", null, "Escopo da entrega"));
  const row = el("div", { class: "row" });
  row.appendChild(el("div", { class: "col" },
    el("p", { class: "muted" }, "Base"), el("p", null, git.base_ref || "—")));
  row.appendChild(el("div", { class: "col" },
    el("p", { class: "muted" }, "Head"), el("p", null, git.head_ref || "—")));
  row.appendChild(el("div", { class: "col" },
    el("p", { class: "muted" }, "Merge base"),
    el("p", null, git.merge_base_sha ? git.merge_base_sha.slice(0, 12) : "—")));
  row.appendChild(el("div", { class: "col" },
    el("p", { class: "muted" }, "Estratégia"), el("p", null, git.diff_strategy || "—")));
  wrap.appendChild(row);

  const commits = git.commits || [];
  wrap.appendChild(el("p", { class: "muted" }, String(commits.length), " commit(s)"));
  if (commits.length) {
    const ul = el("ul", { class: "bullets" });
    for (const c of commits.slice(0, 10)) {
      ul.appendChild(el("li", null, c.short_sha || (c.sha || "").slice(0, 7), " — ", c.subject || ""));
    }
    if (commits.length > 10) ul.appendChild(el("li", { class: "muted" }, "+", String(commits.length - 10), " commits"));
    wrap.appendChild(ul);
  }
  return wrap;
}

// Hero row (fidelidade ao mockup): score | grid SOLID unificado | sustentável.
function scoreHeroCard(r) {
  const quality = (typeof r.scores.quality === "number") ? r.scores.quality : null;
  const verdict = gateVerdict(r.quality_gate.status);
  return el("div", { class: "card card-score" },
    el("div", { class: "card-header" }, "Release Quality Score"),
    el("div", { class: "card-body" },
      score(quality),
      el("span", { class: "pill " + verdict.cls }, verdict.label),
      el("p", { class: "muted" }, "grade ", r.scores.grade || "—"),
      r.scores.score_status ? el("p", { class: "muted" }, "status: ", r.scores.score_status) : null));
}

const SHIELD_PATH = "M12 2l8 4v6c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V6z";
function viewSustainCard(r) {
  const s = mapSustainability(r);
  if (s.empty) return null; // sem critério avaliado -> omite o card (nunca fabrica).
  const verdict = gateVerdict(r.quality_gate.status);
  return el("div", { class: "card card-sustain " + verdict.cls },
    el("div", { class: "shield-icon" }, svgIcon(SHIELD_PATH)),
    el("h3", null, s.title),
    el("p", { class: "muted" }, s.message));
}

function viewOverview(r) {
  const wrap = el("div");
  wrap.appendChild(el("h2", null, "Overview — ", r.run.id));

  const heroRow = el("div", { class: "row" },
    el("div", { class: "col" }, scoreHeroCard(r)),
    el("div", { class: "col col-wide" }, viewSolidGrid(r)));
  const sustain = viewSustainCard(r);
  if (sustain) heroRow.appendChild(el("div", { class: "col" }, sustain));
  wrap.appendChild(heroRow);

  const row = el("div", { class: "row" });
  const gate = el("div", { class: "card col" },
    el("h3", null, "Quality gate"),
    badge(r.quality_gate.status),
    // detalhe completo já vive no card "Gate de entrega" (mapGateSection);
    // r.quality_gate.reason não sobrevive ao unmarshal real (QualityGate só
    // tem Status+Rules em builder.go) — nunca fabricar aqui.
    el("p", { class: "muted" }, (r.quality_gate.rules || []).length + " critério(s)"));
  const risk = el("div", { class: "card col" },
    el("h3", null, "Risk"),
    el("p", null, r.risk.level || "—"),
    el("p", { class: "muted" }, (r.risk.factors || []).length, " fatores"));
  const conf = el("div", { class: "card col" },
    el("h3", null, "Confidence"),
    score(r.scores.confidence * 100),
    el("p", { class: "muted" }, r.scores.confidence_level));
  row.appendChild(gate); row.appendChild(risk); row.appendChild(conf);
  wrap.appendChild(row);

  if ((r.risk.factors || []).length) {
    const factors = el("ul", { class: "bullets" });
    for (const f of r.risk.factors) {
      factors.appendChild(el("li", null, "[" + (f.severity || "?") + "] ",
        f.id, " — ", f.title));
    }
    wrap.appendChild(el("h3", null, "Risk factors"), factors);
  }
  if ((r.recommendations || []).length) {
    const recs = el("ul", { class: "bullets" });
    for (const rec of r.recommendations) {
      recs.appendChild(el("li", null, "(" + rec.priority + ") ",
        rec.title, rec.reason ? " — " + rec.reason : ""));
    }
    wrap.appendChild(el("h3", null, "Recommendations"), recs);
  }
  return wrap;
}

// §4.1 Slider de aprovação: simulação local do limiar de score, NUNCA
// persiste (contrato: "mudar o slider nunca deve gravar nada"). Sem
// fetch/write — só recalcula texto no DOM a partir do state em memória.
// Vive na topbar (layout do mockup), não mais dentro do card de overview.
const DEFAULT_APPROVAL_LIMIT = 80; // [novo] no contrato: vem da política do repo; sem .solidify.yml exposto no report, usa default documentado.
function approvalSliderControl(qualityScore) {
  const out = el("span", { class: "muted" });
  function paint(limit) {
    const folga = qualityScore - limit;
    out.textContent = folga >= 0
      ? `Aprovado (simulado) — folga de ${folga.toFixed(0)} pts acima do limite ${limit}`
      : `Reprovado pelo simulador — faltam ${Math.abs(folga).toFixed(0)} pts para o limite ${limit}`;
  }
  const slider = el("input", {
    type: "range", min: "50", max: "100", value: String(DEFAULT_APPROVAL_LIMIT),
    oninput: (e) => paint(Number(e.target.value)),
  });
  paint(DEFAULT_APPROVAL_LIMIT);
  return el("div", { class: "topbar-slider" },
    el("span", { class: "muted" }, "Aprovar com base em"), slider, out);
}

// Cabeçalho fixo (topbar): contexto git + slider de aprovação por run.
// Fora do fluxo de render() do #root pra ficar sempre visível (mockup).
// Campos = só o que mapTopbar expõe como real (sem project/PR/team — não
// existem no Report hoje, ver mapping.js); cada campo some se ausente.
function paintTopbar(r) {
  const bar = document.getElementById("topbar");
  while (bar.firstChild) bar.removeChild(bar.firstChild);
  const t = mapTopbar(r);
  bar.appendChild(el("span", { class: "page-title", style: "font-size:15px" },
    t.profile || "run", " · ", t.runId || ""));
  const fields = [
    ["Release", t.headRef], ["Base", t.baseRef],
    ["Commit", t.shortSha], ["Analisado em", t.finishedAt],
  ].filter(([, v]) => v);
  for (const [label, value] of fields) {
    bar.appendChild(el("span", { class: "sep" }));
    bar.appendChild(el("div", { class: "tb-field" },
      el("span", { class: "tb-label" }, label), el("strong", null, value)));
  }
  const quality = (typeof r.scores.quality === "number") ? r.scores.quality : null;
  if (quality != null) {
    bar.appendChild(el("span", { class: "sep" }));
    bar.appendChild(approvalSliderControl(quality));
  }
}

function resetTopbar() {
  const bar = document.getElementById("topbar");
  while (bar.firstChild) bar.removeChild(bar.firstChild);
  bar.appendChild(el("span", { class: "page-title", style: "font-size:15px" }, "Solidify Release Quality"));
}

// §3 Painel SOLIDIFY — grid único de 5 colunas, uma por princípio (refino
// visual Fase A1: mockup mostra 1 card só com divisórias sutis entre itens,
// não strip+lista separadas). Detalhe (razão/evidência) que antes vivia numa
// 2ª renderização agora fica embutido como linha muted sob o item sem score.
function viewSolidGrid(r) {
  const items = mapSolidPrinciples(r);
  const grid = el("div", { class: "solid-grid" });
  for (const p of items) {
    const scoreRow = el("div", { class: "solid-score" }, p.scored ? String(p.score) : "N/A",
      el("small", null, p.scored ? "/100" : ""));
    if (p.scored && p.delta != null) {
      scoreRow.appendChild(el("span", { class: "delta " + (p.delta >= 0 ? "pos" : "neg") },
        " " + (p.delta >= 0 ? "↑" : "↓") + Math.abs(p.delta).toFixed(1)));
    }
    const bar = el("div", { class: "bar" });
    if (p.scored) bar.appendChild(el("span", { class: scoreClass(p.score), style: "width:" + Math.max(0, Math.min(100, p.score)) + "%" }));
    grid.appendChild(el("div", { class: "solid-item" },
      el("div", { class: "top" },
        el("div", { class: "solid-letter " + p.key.toLowerCase() }, p.key),
        el("div", { class: "solid-name" }, el("b", null, p.code), p.label)),
      scoreRow, bar,
      !p.scored && p.reason ? el("p", { class: "muted small" }, p.reason) : null));
  }
  return el("div", { class: "card card-solid" },
    el("div", { class: "card-header" }, "SOLIDIFY"),
    el("div", { class: "card-body" }, grid));
}

function scoreCard(title, value, kind, extraRows) {
  if (value == null) return emptyCard(title, null);
  const wrap = el("div", { class: "card" },
    el("div", { class: "card-header" }, title));
  const body = el("div", { class: "card-body" });
  const num = el("div", { class: "score", style: "font-size:32px" }, "0");
  body.appendChild(num);
  const track = el("div", { class: "progress-track" });
  const fill = el("div", { class: "progress-fill " + scoreClass(value, kind) });
  track.appendChild(fill);
  body.appendChild(track);
  for (const row of extraRows || []) body.appendChild(row);
  wrap.appendChild(body);
  animateNumber(num, value, { decimals: 0 });
  animateWidth(fill, value);
  return wrap;
}

// §11.1 Sonar
function viewSonarSection(report) {
  const s = mapSonarSection(report);
  if (s.empty) return emptyCard("SonarQube", s.reason);
  const m = s.metrics;
  return scoreCard("SonarQube", s.score, "sonar", [
    el("p", { class: "muted" }, "bugs: ", String(m.bugs ?? "—"),
      " · vulnerabilities: ", String(m.vulnerabilities ?? "—"),
      " · code smells: ", String(m.code_smells ?? "—")),
  ]);
}

// §11.2 Lighthouse
function viewLighthouseSection(report) {
  const l = mapLighthouseSection(report);
  if (l.empty) return emptyCard("Lighthouse", l.reason);
  const wrap = el("div", { class: "card" }, el("div", { class: "card-header" }, "Lighthouse"));
  const strip = el("div", { class: "kpi-strip" });
  for (const [label, value] of [
    ["Performance", l.performance], ["Acessibilidade", l.accessibility],
    ["Boas práticas", l.best_practices], ["SEO", l.seo],
  ]) {
    strip.appendChild(gaugeTile(label, value, scoreClass(value, "lighthouse"), 3.6));
  }
  wrap.appendChild(strip);
  return wrap;
}

// §12 Performance / Carga (k6)
function viewK6Section(report) {
  const k = mapK6Section(report);
  if (k.empty) return emptyCard("Performance / Carga (k6)", k.reason);
  const wrap = el("div", { class: "card full-width" }, el("div", { class: "card-header" }, "Performance / Carga (k6)"));
  const strip = el("div", { class: "kpi-strip" });
  for (const [label, value, unit] of [
    ["p95", k.p95Ms, "ms"], ["p99", k.p99Ms, "ms"],
    ["erro", (k.errorRate || 0) * 100, "%"], ["throughput", k.throughputRps, "rps"],
  ]) {
    const num = el("span", null, value == null ? "—" : "0");
    const valueEl = el("div", { class: "value" }, num, unit ? el("span", { class: "muted" }, " " + unit) : null);
    strip.appendChild(el("div", { class: "kpi-tile" }, el("div", { class: "label" }, label), valueEl));
    if (value != null) animateNumber(num, value, { decimals: unit === "%" ? 1 : 0 });
  }
  wrap.appendChild(strip);
  wrap.appendChild(el("p", { class: "muted" },
    "thresholds: ", el("span", { class: "pill " + (k.passThresholds ? "ok" : "fail") }, k.passThresholds ? "PASS" : "FAIL")));
  if ((k.thresholdFailed || []).length) {
    const ul = el("ul", { class: "bullets" });
    for (const t of k.thresholdFailed) ul.appendChild(el("li", null, t));
    wrap.appendChild(ul);
  }
  return wrap;
}

// §11 Segurança (gitleaks/osv-scanner/semgrep agregados)
function viewSecuritySection(report) {
  const sec = mapSecuritySection(report);
  if (sec.empty) return emptyCard("Segurança", sec.reason);
  const wrap = el("div", { class: "card" },
    el("div", { class: "card-header" }, "Segurança",
      el("span", { class: "pill " + (sec.gateAllow ? "ok" : "fail") }, sec.gateAllow ? "PASS" : "FAIL")));
  const body = el("div", { class: "card-body" });
  body.appendChild(el("p", { class: "muted" }, sec.gateReason || ""));
  const ul = el("ul", { class: "bullets" });
  for (const f of sec.findings.slice(0, 10)) {
    ul.appendChild(el("li", null,
      el("span", { class: "pill " + severityClass(f.severity) }, f.severity),
      " ", f.source, " — ", f.rule, " — ", f.file_path || ""));
  }
  body.appendChild(ul);
  wrap.appendChild(body);
  return wrap;
}

// §Coverage (não existe seção numerada própria no contrato — anexado a
// maintainability; ver TODO em internal/app/run.go sobre pesos).
function viewCoverageSection(report) {
  const c = mapCoverageSection(report);
  if (c.empty) return emptyCard("Coverage", c.reason);
  return scoreCard("Coverage", c.linePct, "coverage", [
    el("p", { class: "muted" }, String(c.linesCovered ?? "—"), "/", String(c.linesTotal ?? "—"), " linhas · threshold ", String(c.threshold ?? "—"), "%"),
  ]);
}

// §5 Analisadores — faixa de transparência da análise, largura total,
// única com sombra (analyzers-strip).
function viewAnalyzersStrip(report) {
  const items = mapAnalyzersStrip(report);
  const wrap = el("div", { class: "card analyzers-strip full-width" },
    el("div", { class: "card-header" }, "Analisadores"));
  const body = el("div", { class: "card-body row" });
  for (const it of items) {
    const skipped = (it.executionStatus || "").startsWith("skipped");
    body.appendChild(el("div", { class: "col" },
      el("strong", null, it.id),
      el("span", null, " — ", it.applicability),
      skipped
        ? el("p", { class: "muted" }, it.executionStatus)
        : el("p", { class: "muted" }, "score ", it.score != null ? String(Math.round(it.score)) : "—",
            " — ", String(it.durationMs || 0), "ms")));
  }
  wrap.appendChild(body);
  return wrap;
}

// SAI-125: renderiza sparkline SVG inline + tabela compacta.
// Sem deps (ADR-0074 veda libs JS). Threshold line tracejada em 5%.
function viewTrend(data) {
  const wrap = el("div");
  wrap.appendChild(el("h2", null, "Peer reviews — v1 fraction trend"));
  wrap.appendChild(el("p", { class: "muted" },
    "Rolling 30d v1 fraction por dia — ",
    String(data.window_days), " dias. Threshold: ",
    (data.threshold * 100).toFixed(1), "%"));

  const pts = data.points || [];
  if (pts.length === 0) {
    wrap.appendChild(el("p", null, "Sem dados."));
    return wrap;
  }

  // SVG sparkline: viewBox 600x80, eixo Y = 0..1 (v1_fraction).
  const W = 600, H = 80, pad = 4;
  const xStep = (W - 2 * pad) / Math.max(pts.length - 1, 1);
  const yOf = f => H - pad - f * (H - 2 * pad);
  const threshold = data.threshold || 0;
  const lastFraction = pts[pts.length - 1].v1_fraction || 0;
  const lineColor = lastFraction <= threshold ? "var(--pass)" : "var(--fail)";

  const svgNs = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(svgNs, "svg");
  svg.setAttribute("viewBox", `0 0 ${W} ${H}`);
  svg.setAttribute("class", "spark");
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", "v1 fraction trend");

  // threshold line
  if (threshold > 0 && threshold <= 1) {
    const tline = document.createElementNS(svgNs, "line");
    tline.setAttribute("class", "threshold");
    tline.setAttribute("x1", String(pad));
    tline.setAttribute("x2", String(W - pad));
    tline.setAttribute("y1", String(yOf(threshold)));
    tline.setAttribute("y2", String(yOf(threshold)));
    svg.appendChild(tline);
  }

  // polyline principal
  const poly = document.createElementNS(svgNs, "polyline");
  const coords = pts.map((p, i) => `${pad + i * xStep},${yOf(p.v1_fraction || 0)}`).join(" ");
  poly.setAttribute("points", coords);
  poly.setAttribute("fill", "none");
  poly.setAttribute("stroke", lineColor);
  poly.setAttribute("stroke-width", "1.5");
  poly.setAttribute("stroke-linejoin", "round");
  svg.appendChild(poly);

  // dot no último ponto
  const lastDot = document.createElementNS(svgNs, "circle");
  const lx = pad + (pts.length - 1) * xStep;
  const ly = yOf(lastFraction);
  lastDot.setAttribute("cx", String(lx));
  lastDot.setAttribute("cy", String(ly));
  lastDot.setAttribute("r", "2.5");
  lastDot.setAttribute("fill", lineColor);
  svg.appendChild(lastDot);

  // tooltips via <title> em cada ponto
  const title = document.createElementNS(svgNs, "title");
  title.textContent =
    pts.map(p => `${p.date}: v1=${p.v1_count} v2=${p.v2_count} frac=${(p.v1_fraction*100).toFixed(1)}%`).join("\n");
  svg.appendChild(title);

  wrap.appendChild(svg);

  // tabela compacta
  const t = el("table");
  t.appendChild(el("thead", null,
    el("tr", null,
      el("th", null, "Data"),
      el("th", null, "v1"),
      el("th", null, "v2"),
      el("th", null, "%"))));
  const tbody = el("tbody");
  for (const p of pts.slice().reverse().slice(0, 14)) {
    const cls = (p.v1_fraction || 0) <= threshold ? "pos" : "neg";
    tbody.appendChild(el("tr", null,
      el("td", null, p.date),
      el("td", null, String(p.v1_count)),
      el("td", null, String(p.v2_count)),
      el("td", { class: cls }, (p.v1_fraction * 100).toFixed(1) + "%")));
  }
  t.appendChild(tbody);
  wrap.appendChild(el("p", { class: "muted" },
    "Mostrando últimos 14 dias. Threshold: ", (threshold * 100).toFixed(1), "%."));

  return wrap;
}

function markActiveNav(route) {
  const top = route.startsWith("run/") ? "runs" : route;
  document.querySelectorAll("#nav a").forEach((a) => {
    a.classList.toggle("active", a.dataset.route === top);
  });
}

async function render() {
  const route = hash();
  markActiveNav(route);
  // limpa DOM sem innerHTML (mitiga XSS).
  while (root.firstChild) root.removeChild(root.firstChild);
  try {
    if (route === "runs") {
      resetTopbar();
      clearRunNav();
      const runs = await loadRuns();
      root.appendChild(viewRuns(runs));
      return;
    }
    if (route.startsWith("run/")) {
      const id = route.slice(4);
      const r = await loadRun(id);
      // Página de run usa o mockup real do Design Canvas via iframe (pedido
      // explícito: "era pra ficar idêntico") — hidratado com dado real por
      // data-dc-tpl (hydrate.js), em vez de recomposto em CSS/JS à mão.
      // Topbar externo/anchors do app somem: o mockup já traz os próprios.
      resetTopbar();
      clearRunNav();
      // scrolling="no" + overflow:hidden: o iframe NUNCA tem barra própria —
      // só o document externo rola. Sem isso, entre o load e a 1ª medição de
      // altura (ou se o conteúdo crescer depois, ex. fonte carregando), o
      // iframe mostra sua própria scrollbar interna por cima da da página
      // (2 barras verticais simultâneas — bug relatado).
      const frame = el("iframe", {
        src: "/assets/mockup-run.html",
        scrolling: "no",
        style: "width:100%;border:0;display:block;min-height:400px;overflow:hidden",
      });
      root.appendChild(frame);
      await new Promise((resolve) => {
        frame.addEventListener("load", () => {
          const doc = frame.contentDocument;
          // bundler decodifica async (gzip+base64) — espera um nó-marco
          // aparecer antes de hidratar, com timeout de segurança.
          const started = performance.now();
          (function waitReady() {
            if (doc.querySelector('[data-dc-tpl="129"]') || performance.now() - started > 8000) {
              hydrateMockup(doc, r);
              const resize = () => { frame.style.height = doc.documentElement.scrollHeight + "px"; };
              resize();
              // conteúdo pode crescer depois da 1ª medição (fonte carregando,
              // reflow do grid SOLID) — reobserva e remede.
              new ResizeObserver(resize).observe(doc.body);
              paintRunNavTpl(doc, [
                [129, "Visão Geral"], [227, "Gate de Entrega"], [305, "Analisadores (IA)"],
                [401, "Violações SOLID"], [480, "Arquivos de risco"], [520, "Entregas"],
                [546, "Arquivos alterados"], [599, "Segurança"], [640, "SonarQube"],
                [728, "Lighthouse"], [833, "Performance (k6)"], [911, "Migrations"],
                [931, "Envs"], [955, "Aplicabilidade"], [988, "Resumo"],
              ]);
              resolve();
            } else {
              requestAnimationFrame(waitReady);
            }
          })();
        }, { once: true });
      });
      return;
    }
    // SAI-125: rota peer-reviews mostra trend.
    if (route === "peer-reviews") {
      resetTopbar();
      clearRunNav();
      const trend = await loadTrend();
      root.appendChild(el("div", { class: "card" }, viewTrend(trend)));
      return;
    }
    resetTopbar();
    clearRunNav();
    root.appendChild(el("p", null, "Rota desconhecida: ", route));
  } catch (err) {
    resetTopbar();
    clearRunNav();
    root.appendChild(el("p", { class: "muted" }, "Erro: ", err.message));
  }
}

window.addEventListener("hashchange", render);
render();