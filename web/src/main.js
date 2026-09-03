// Solidify dashboard bootstrap (SAI-080).
// Single-file Preact-equivalent: vanilla JS + DOM diffing manual.
// Sem build step no runtime — browser parseia direto.

const root = document.getElementById("root");
const hash = () => location.hash.replace(/^#\/?/, "") || "runs";

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

function el(tag, props, ...children) {
  const e = document.createElement(tag);
  if (props) {
    for (const k in props) {
      if (k === "class") e.className = props[k];
      else if (k.startsWith("on")) e.addEventListener(k.slice(2), props[k]);
      else e.setAttribute(k, props[k]);
    }
  }
  for (const c of children) {
    if (c == null) continue;
    // SAI-129 (achado validação e2e): child não-Node (ex: número cru de
    // "(arr||[]).length" em viewOverview) ia direto pro appendChild() e
    // quebrava a página inteira (TypeError, sem stack visível). Qualquer
    // primitivo vira texto; só Node passa direto.
    if (c instanceof Node) e.appendChild(c);
    else e.appendChild(document.createTextNode(String(c)));
  }
  return e;
}

function badge(status) {
  return el("span", { class: "badge " + status }, status);
}

function score(n) {
  if (n == null || Number.isNaN(n)) return el("div", { class: "score na" }, "N/A");
  return el("div", { class: "score" }, String(Math.round(n)));
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

function viewOverview(r) {
  const wrap = el("div");
  wrap.appendChild(el("h2", null, "Overview — ", r.run.id));
  const row = el("div", { class: "row" });
  const gate = el("div", { class: "card col" },
    el("h3", null, "Quality gate"),
    badge(r.quality_gate.status),
    el("p", { class: "muted" }, r.quality_gate.reason || ""));
  const quality = (typeof r.scores.quality === "number") ? r.scores.quality : null;
  const qual = el("div", { class: "card col" },
    el("h3", null, "Score"),
    score(quality),
    el("p", { class: "muted" }, "grade ", r.scores.grade || "—"),
    r.scores.score_status ? el("p", { class: "muted" }, "status: ", r.scores.score_status) : null);
  const risk = el("div", { class: "card col" },
    el("h3", null, "Risk"),
    el("p", null, r.risk.level || "—"),
    el("p", { class: "muted" }, (r.risk.factors || []).length, " fatores"));
  const conf = el("div", { class: "card col" },
    el("h3", null, "Confidence"),
    score(r.scores.confidence * 100),
    el("p", { class: "muted" }, r.scores.confidence_level));
  row.appendChild(gate); row.appendChild(qual);
  row.appendChild(risk); row.appendChild(conf);
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

function viewSOLID(r) {
  const wrap = el("div");
  wrap.appendChild(el("h2", null, "SOLID"));
  const ps = r.solid.principles || {};
  for (const k of ["S", "O", "L", "I", "D"]) {
    const p = ps[k] || {};
    const app = p.applicability || (p.applicable ? "APPLICABLE" : "NOT_APPLICABLE");
    const scored = p.after_score != null && app === "APPLICABLE";
    const scoreText = scored ? String(p.after_score) : "N/A";
    const d = (typeof p.delta === "number") ? p.delta : 0;
    const cls = scored ? "delta " + (d >= 0 ? "pos" : "neg") : "delta na";
    const card = el("div", { class: "principle " + (scored ? "scored" : "na") },
      el("strong", null, k),
      el("span", { class: "app-badge " + app.toLowerCase().replace(/_/g, "-") }, app),
      el("span", null, "score: ", scoreText),
      el("span", { class: cls }, scored ? ((d >= 0 ? "+" : "") + d.toFixed(1)) : "—"));
    if (p.reason) {
      card.appendChild(el("p", { class: "reason muted" }, p.reason));
    }
    if (Array.isArray(p.evidence_refs) && p.evidence_refs.length) {
      card.appendChild(el("p", { class: "evidence muted" }, "evidência: ", p.evidence_refs.join(", ")));
    }
    wrap.appendChild(card);
  }
  return wrap;
}

function viewQuality(r) {
  const wrap = el("div");
  wrap.appendChild(el("h2", null, "Quality"));
  const ul = el("ul", { class: "bullets" });
  for (const f of r.analyzers || []) {
    ul.appendChild(el("li", null,
      f.id, " — ", f.applicability,
      " — ", String(f.duration_ms), "ms"));
  }
  wrap.appendChild(ul);
  return wrap;
}

function viewAI(r) {
  const wrap = el("div");
  wrap.appendChild(el("h2", null, "AI Review"));
  wrap.appendChild(el("p", null, "mode: ", r.ai_review.mode,
    " — peer reviewed: ", String(r.ai_review.peer_reviewed)));
  const ul = el("ul", { class: "bullets" });
  for (const a of r.ai_review.actors || []) {
    ul.appendChild(el("li", null,
      a.role, " — ", a.provider, "/", a.model_id,
      " — status ", a.status));
  }
  wrap.appendChild(ul);
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

async function render() {
  const route = hash();
  // limpa DOM sem innerHTML (mitiga XSS).
  while (root.firstChild) root.removeChild(root.firstChild);
  try {
    if (route === "runs") {
      const runs = await loadRuns();
      root.appendChild(viewRuns(runs));
      return;
    }
    if (route.startsWith("run/")) {
      const id = route.slice(4);
      const r = await loadRun(id);
      root.appendChild(viewGitHeader(r));
      root.appendChild(viewOverview(r));
      root.appendChild(el("div", { class: "card" }, viewSOLID(r)));
      root.appendChild(el("div", { class: "card" }, viewQuality(r)));
      root.appendChild(el("div", { class: "card" }, viewAI(r)));
      return;
    }
    // SAI-125: rota peer-reviews mostra trend.
    if (route === "peer-reviews") {
      const trend = await loadTrend();
      root.appendChild(el("div", { class: "card" }, viewTrend(trend)));
      return;
    }
    root.appendChild(el("p", null, "Rota desconhecida: ", route));
  } catch (err) {
    root.appendChild(el("p", { class: "muted" }, "Erro: ", err.message));
  }
}

window.addEventListener("hashchange", render);
render();