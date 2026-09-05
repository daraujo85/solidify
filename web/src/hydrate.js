// hydrate.js — patch de valores no mockup real do Design Canvas
// (web/public/assets/mockup-run.html), servido via iframe em vez de
// reconstruído em CSS/JS à mão (pedido explícito do usuário: "era pra
// ficar idêntico"). Cada nó-alvo é endereçado pelo atributo estável
// data-dc-tpl="N" (confirmado determinístico entre reloads — as
// classNames com hash do export, tipo scp0/sc-interp, NÃO são estáveis).
//
// Regra do projeto (nunca fabricar dado): campo do mockup sem
// equivalente real no Report vira "—" (valor pontual) ou tem o
// bloco/linha ocultado via display:none (seção inteira sem fonte real:
// Projeto/PR/Equipe, gráfico k6 no tempo, checklist fixo de 4 categorias
// de segurança, "Testes de regressão", "Próximos passos"/"Recomendação").
// Score anterior e sparklines de Sonar são reais quando há run(s) anterior(es)
// do mesmo project_path (opts.history) — senão ficam ocultos, nunca fabricados.
//
// ponytail: mapeamento cobre as seções de maior valor (score, SOLID,
// gate, analisadores, violações, arquivos, sonar/lighthouse/k6/coverage,
// migrations/envs/aplicabilidade, resumo parcial). Campos residuais sem
// tpl mapeado aqui ficam com o texto mock do Design Canvas — não é dado
// fabricado por nós, é o placeholder original do design, mas não deve
// ser lido como valor real; upgrade quando o backend expuser o campo.

import {
  mapSolidPrinciples, mapSustainability, mapGateSection, mapAIReviewers,
  mapViolations, mapRiskFiles, mapDeliveries, mapChangedFiles,
  mapSonarSection, mapLighthouseSection, mapK6Section, mapSecuritySection,
  mapCoverageSection, mapMigrations, mapEnvs, mapApplicability, mapSummary,
  severityClass, gateVerdict, scoreClass,
} from "./mapping.js";

// Cores por status (PASS/WARN/FAIL/neutral) — reusadas no badge do hero,
// nas barras SOLID e no ícone de sustentabilidade, pra nunca deixar a cor
// fixa (verde) do mockup mentir sobre um veredicto ruim/ausente.
const STATUS_COLOR = {
  ok: { bg: "rgb(233, 249, 239)", fg: "rgb(22, 101, 52)", stroke: "#16a34a" },
  warn: { bg: "rgb(254, 246, 231)", fg: "rgb(146, 64, 14)", stroke: "#d97706" },
  fail: { bg: "rgb(254, 232, 232)", fg: "rgb(153, 27, 27)", stroke: "#dc2626" },
  neutral: { bg: "rgb(241, 244, 249)", fg: "rgb(61, 74, 96)", stroke: "#7b8aa3" },
};

// Badge de severidade de violação (Alta/Média/Baixa) — cores tiradas dos
// 3 exemplos reais do mockup (rgb exatos por slot), não STATUS_COLOR
// (severityClass mapeia "low" -> "ok"/verde, errado pra uma violação real:
// baixa severidade ainda é violação, não sucesso — usa cinza neutro).
const SEVERITY_BADGE = {
  high: { label: "Alta", fg: "rgb(185, 28, 28)", bg: "rgb(253, 236, 236)" },
  critical: { label: "Alta", fg: "rgb(185, 28, 28)", bg: "rgb(253, 236, 236)" },
  medium: { label: "Média", fg: "rgb(194, 65, 12)", bg: "rgb(255, 243, 232)" },
  low: { label: "Baixa", fg: "rgb(71, 85, 105)", bg: "rgb(241, 244, 249)" },
};

// Badge de princípio SOLID na violação — cores confirmadas nos exemplos
// reais do mockup (D/O/I); S/L extrapolados por analogia (ponytail:
// ajustar se o Design Canvas expuser referência canônica das 5 cores).
const PRINCIPLE_BADGE = {
  S: { fg: "rgb(37, 99, 235)", bg: "rgb(234, 240, 254)" },
  O: { fg: "rgb(124, 58, 237)", bg: "rgb(243, 238, 254)" },
  L: { fg: "rgb(194, 65, 12)", bg: "rgb(255, 243, 232)" },
  I: { fg: "rgb(15, 118, 110)", bg: "rgb(230, 246, 244)" },
  D: { fg: "rgb(190, 24, 93)", bg: "rgb(253, 238, 245)" },
};

function fmtDelta(n) {
  if (n == null) return null;
  const sign = n > 0 ? "↑ " : n < 0 ? "↓ " : "= ";
  return sign + Math.abs(n);
}

export function hydrateMockup(doc, report, opts = {}) {
  const history = opts.history || []; // reports de runs anteriores do mesmo projeto (ordem asc por finalized_at)
  const $ = (id) => doc.querySelector(`[data-dc-tpl="${id}"]`);
  const set = (id, text) => { const e = $(id); if (e) e.textContent = text; };
  const walk = (id, levels) => {
    let e = $(id);
    for (let i = 0; i < levels && e; i++) e = e.parentElement;
    return e;
  };
  const hide = (id, levels = 0) => { const e = walk(id, levels); if (e) e.style.display = "none"; };
  // show — contrapartida de hide pra nós que o Design Canvas já nasce com
  // display:none inline (ex.: card de ator, escondido até termos ator real).
  const show = (id, levels = 0, display = "flex") => { const e = walk(id, levels); if (e) e.style.display = display; };
  const style = (id, styles) => { const e = $(id); if (e) Object.assign(e.style, styles); };

  // Sidebar própria do mockup (nav/marca "Solidify") — duplica a sidebar
  // real do app (index.html #nav). O iframe só deve renderizar o conteúdo
  // (main), nunca o chrome de página inteira do Design Canvas.
  hide(7);

  // Topbar — sem Projeto/PR/Equipe/slider real (report não carrega esses
  // campos hoje); esconde. Release/Analisado em/Commit têm fonte real.
  hide(82, 2); // Projeto
  hide(91, 0); // badge "Produção"
  hide(111, 2); // Pull Request
  hide(118, 2); // slider "Aprovar a partir de"
  hide(124, 1); // Equipe (avatar)
  const run = report.run || {};
  const git = report.git || {};
  const firstCommit = (git.commits || [])[0];
  if (git.head_ref) set(90, git.head_ref); else hide(88, 2);
  if (run.finished_at) set(100, run.finished_at); else hide(99, 2);
  if (firstCommit) set(105, firstCommit.short_sha); else hide(103, 2);

  // Hero — score + SOLID grid + sustentabilidade.
  // score_status "unavailable" (schema falhou/re-run necessário) tem
  // quality=0 no JSON — 0 É um number, então o guard antigo (só
  // `typeof === "number"`) deixava passar um "0/100" fabricado. Só
  // renderiza o número quando o backend confirma que é score real.
  const scoreOk = report.scores?.score_status === "available" && typeof report.scores?.quality === "number";
  if (scoreOk) set(145, String(Math.round(report.scores.quality))); else hide(144);
  // "Score anterior" — última run real do mesmo projeto com score disponível
  // (history já vem ordenado asc por finalized_at); nunca fabrica se não há
  // run anterior com score_status "available".
  const prevScored = [...history].reverse().find(
    (h) => h.scores?.score_status === "available" && typeof h.scores?.quality === "number"
  );
  if (scoreOk && prevScored) {
    const prevScore = Math.round(prevScored.scores.quality);
    set(150, `Score anterior: ${prevScore}/100`);
    const delta = Math.round(report.scores.quality) - prevScore;
    set(151, fmtDelta(delta) + " pts");
    style(151, { color: STATUS_COLOR[delta >= 0 ? "ok" : "fail"].stroke });
    show(149, 0, "flex");
  } else hide(150, 1);

  // Badge "Aprovado com ressalvas" do mockup é texto fixo (nem tpl id
  // tem — é um <span> cru dentro do container 147); veredicto real vem
  // de gateVerdict(quality_gate.status). Sem score real, nem badge existe.
  if (scoreOk) {
    const verdict = gateVerdict(report.quality_gate?.status);
    const c = STATUS_COLOR[verdict.cls] || STATUS_COLOR.neutral;
    const badge = $(147);
    if (badge) { badge.textContent = verdict.label; badge.style.background = c.bg; badge.style.color = c.fg; }
  } else hide(147);

  const SOLID_TPL = {
    S: { score: 162, delta: 164, fill: 166 }, O: { score: 174, delta: 176, fill: 178 },
    L: { score: 186, delta: 188, fill: 190 }, I: { score: 198, delta: 200, fill: 202 },
    D: { score: 210, delta: 212, fill: 214 },
  };
  for (const p of mapSolidPrinciples(report)) {
    const t = SOLID_TPL[p.key];
    if (!t) continue;
    set(t.score, p.scored ? String(p.score) : "N/A");
    if (p.scored && p.delta != null) set(t.delta, fmtDelta(p.delta));
    else hide(t.delta);
    // Barra de progresso — mockup nasce com largura/cor fixas (ex.: 92%
    // verde) por princípio; sem score real isso é 100% fabricado. Zera
    // quando não aplicável/não avaliado, senão reflete o score real.
    if (p.scored) style(t.fill, { width: p.score + "%", background: STATUS_COLOR[scoreClass(p.score)].stroke });
    else style(t.fill, { width: "0%", background: STATUS_COLOR.neutral.stroke });
  }

  const sustain = mapSustainability(report);
  if (!sustain.empty) { set(220, sustain.title); set(221, sustain.message); }
  else { set(220, "Sem avaliação"); set(221, sustain.reason); }
  // Ícone-escudo do card de sustentabilidade — cor fixa verde no mockup;
  // sincroniza com o status real do gate (mesmo cair em "Sem avaliação").
  const gateStatusNow = report.quality_gate?.status;
  const sustainColor = STATUS_COLOR[
    gateStatusNow === "PASS" ? "ok" : gateStatusNow === "WARN" ? "warn" : gateStatusNow === "FAIL" ? "fail" : "neutral"
  ];
  style(216, { background: sustainColor.bg });
  style(218, { stroke: sustainColor.stroke });
  style(219, { stroke: sustainColor.stroke });

  // Gate de Entrega — só os 4 critérios com fonte real 1:1; "Score mínimo"
  // e "Testes de regressão" não têm campo real equivalente (rules do
  // quality_gate não expõem número solto; não existe contagem de testes
  // no report) -> oculta os 2 boxes inteiros.
  hide(239, 2); // Score mínimo
  hide(287, 2); // Testes de regressão
  const gate = mapGateSection(report);
  const byId = Object.fromEntries(gate.items.map((it) => [it.id, it]));
  // Header "X de Y critérios atendidos" — fixo "5 de 6" no mockup; real é
  // passed/total dos itens que EXISTEM (nunca as 6 categorias fabricadas).
  if (gate.items.length) {
    const passed = gate.items.filter((it) => it.status === "PASS").length;
    set(228, `${passed} de ${gate.items.length} critério(s) atendido(s)`);
  } else hide(228);
  // Pill "Liberação condicionada a N ressalva(s)" — só existe quando o gate
  // real é WARN; conta ressalvas reais (itens não-PASS), nunca o "1" fixo.
  const warnItems = gate.items.filter((it) => it.status !== "PASS").length;
  if (gate.status === "WARN" && warnItems) {
    const pill = $(229);
    const dot = $(230);
    if (pill) {
      pill.textContent = `Liberação condicionada a ${warnItems} ressalva${warnItems === 1 ? "" : "s"}`;
      if (dot) pill.prepend(dot);
    }
  } else hide(229);
  if (byId.coverage) {
    const cov = mapCoverageSection(report);
    set(249, (cov.linePct ?? "—") + "%");
    set(251, "regra: ≥ " + (cov.threshold ?? "—") + "%");
  } else hide(247, 2);
  hide(250); // delta de cobertura — sem histórico
  if (byId.security) {
    const sec = mapSecuritySection(report);
    const crit = (sec.bySeverity && (sec.bySeverity.critical ?? sec.bySeverity.high)) ?? 0;
    set(259, String(crit));
  } else hide(257, 2);
  if (byId.solid_violations) {
    set(268, String(mapViolations(report).filter((v) => v.principle).length));
  } else hide(266, 2);
  hide(269); // delta — sem histórico
  if (byId.migration_rollback) {
    set(279, byId.migration_rollback.message || (byId.migration_rollback.status === "PASS" ? "presente" : "ausente"));
  } else hide(276, 2);
  hide(291, 2); // banner "ressalva ... Equipe Plataforma / PaymentService" — sem atribuição real

  // Analisadores (revisores IA) — Escopo/Achados/Confiança por-ator do
  // mockup não existem no Actor real; Escopo reaproveitado pra
  // provider/model (dado real), Achados/Confiança ocultados.
  const ai = mapAIReviewers(report);
  const ROLE_LABELS = { peer_a: "Avaliador A", peer_b: "Avaliador B", arbiter: "Juiz" };
  // Header "Consenso em 3 de 3" — sem contagem real de atores que
  // concordaram (Actor não carrega esse booleano); sempre oculto.
  hide(308);
  const actorSlots = [
    { box: 311, role: 315, status: 317, scope: 321, findings: 324, conf: 328 },
    { box: 331, role: 335, status: 337, scope: 341, findings: 344, conf: 348 },
    { box: 351, role: 355, status: 357, scope: 361, findings: 364, conf: 368 },
  ];
  actorSlots.forEach((slot, i) => {
    const a = ai.actors[i];
    // Endereça o card pelo próprio tpl id (box), não subindo N parents a
    // partir do role — a versão anterior (hide(slot.role, 3)) contava
    // níveis a partir do DOM estático e não do DOM já mutado pelo runtime
    // do bundler em tempo de execução, então não hidratava o elemento certo.
    if (!a) { hide(slot.box); return; }
    show(slot.box, 0, "flex");
    set(slot.role, ROLE_LABELS[a.role] || a.role || "—");
    set(slot.status, a.status === "completed" ? "Aprovado" : (a.status || "—"));
    // Escopo mostra modelo pedido; se o roteador executou um modelo
    // diferente (fallback do 9router), o executado real vai entre
    // parênteses — sem isso o usuário não sabia qual modelo de fato rodou.
    const modelSuffix = a.executedModel && a.executedModel !== a.modelId ? ` (${a.executedModel})` : "";
    set(slot.scope, `${a.provider || "—"}/${a.modelId || "—"}${modelSuffix}`);
    hide(slot.findings);
    hide(slot.conf);
  });
  // Card "Qualidade do consenso" (gauge + deltas) — só existe fonte real
  // pra 1 número (agreement); "3 de 3", "↑6pp"/"↓4pp" não têm campo
  // equivalente (sem histórico entre releases) e ficam ocultos mesmo
  // quando o card é mostrado.
  if (ai.peerReviewed && ai.agreementPct != null) {
    show(371, 0, "flex");
    style(381, { display: "inline" });
    set(381, Math.round(ai.agreementPct) + "%");
    const CIRC = 87.96; // 2*pi*14 (mesmo raio do svg do mockup)
    const filled = (ai.agreementPct / 100) * CIRC;
    const circle = $(378);
    if (circle) circle.setAttribute("stroke-dasharray", `${filled.toFixed(2)} ${CIRC}`);
  } else {
    hide(371);
  }
  hide(374); // "X de Y" — sem contagem real de concordância
  hide(382); hide(387); // deltas ↑/↓ — sem histórico real
  hide(388); // divergência entre pares (%) — sem base real
  hide(391); // confiabilidade da análise — sem base real
  hide(394); // trechos sem revisão — sem base real
  if (ai.divergences[0]) set(395, ai.divergences[0]); else hide(395);

  // Violações SOLID priorizadas — header e filtros do export carregam
  // contagens fixas ("5 novas", Alta 2/Média 2/Baixa 1). `risk.factors`
  // não informa histórico; exibe só a contagem de ocorrências atuais.
  const violations = mapViolations(report);
  style(397, { flex: "1 1 100%", width: "100%" }); // card ocupa a largura toda (pedido do usuário)
  if (violations.length) {
    set(402, `${violations.length} violação${violations.length === 1 ? "" : "ões"} encontrada${violations.length === 1 ? "" : "s"}`);
    const severityCounts = { high: 0, medium: 0, low: 0 };
    violations.forEach((v) => {
      const s = (v.severity || "").toLowerCase();
      if (s === "critical" || s === "high") severityCounts.high++;
      else if (s === "medium") severityCounts.medium++;
      else severityCounts.low++;
    });
    set(405, `Alta ${severityCounts.high}`);
    set(406, `Média ${severityCounts.medium}`);
    set(407, `Baixa ${severityCounts.low}`);
  } else hide(397);

  // Até 4 slots reais. IDs por slot: sev (badge severidade), principle
  // (badge SOLID), fileLine (mono), commitSha (mono, alinhado à direita —
  // sem fonte real por violação em mapViolations, sempre oculto), title, desc.
  const VIOL_TPL = [
    { sev: 413, principle: 414, fileLine: 415, commitSha: 416, title: 417, desc: 418, sugLabel: 422, sug: 423 },
    { sev: 429, principle: 430, fileLine: 431, commitSha: 432, title: 433, desc: null, sugLabel: 437, sug: 439 },
    { sev: 445, principle: 446, fileLine: 447, commitSha: 448, title: 449, desc: 450, sugLabel: 452, sug: 453 },
    { sev: 459, principle: 460, fileLine: 461, commitSha: 462, title: 463, desc: null, sugLabel: 467, sug: 469 },
  ];
  VIOL_TPL.forEach((slot, i) => {
    const v = violations[i];
    if (!v) { hide(slot.sev, 3); return; }
    const sevBadge = SEVERITY_BADGE[(v.severity || "").toLowerCase()] || SEVERITY_BADGE.low;
    set(slot.sev, sevBadge.label);
    style(slot.sev, { color: sevBadge.fg, background: sevBadge.bg });
    if (v.principle && PRINCIPLE_BADGE[v.principle]) {
      set(slot.principle, v.principle);
      style(slot.principle, { color: PRINCIPLE_BADGE[v.principle].fg, background: PRINCIPLE_BADGE[v.principle].bg });
    } else hide(slot.principle);
    set(slot.fileLine, v.file ? v.file + (v.line ? ":" + v.line : "") : "—");
    hide(slot.commitSha);
    set(slot.title, v.title);
    if (slot.desc) { if (v.description) set(slot.desc, v.description); else hide(slot.desc); }
    if (v.suggestion) set(slot.sug, v.suggestion); else hide(slot.sugLabel, 1);
  });

  // Arquivos de risco — até 4 slots reais (path + cobertura; sem delta
  // vs release anterior, não existe histórico). Card inteiro (título 480 +
  // subtítulo mock 481) some quando não há coverage real — antes só as
  // linhas eram ocultadas e o card ficava com cabeçalho vazio.
  const RISK_TPL = [
    { path: 485, delta: 486, pct: 487 }, { path: 492, delta: 493, pct: 494 },
    { path: 499, delta: 500, pct: 501 }, { path: 506, delta: 507, pct: 508 },
  ];
  const risk = mapRiskFiles(report);
  if (risk.empty) {
    hide(480, 2);
  } else {
    RISK_TPL.forEach((slot, i) => {
      const f = risk.rows[i];
      if (!f) { hide(slot.path, 2); return; }
      set(slot.path, f.path);
      hide(slot.delta);
      set(slot.pct, (f.coveragePct ?? "—") + "%");
    });
  }

  // §9 O que foi entregue — 1:1 de git.commits[].type (Conventional
  // Commits); mockup tem 4 slots fixos (Features/Bugfixes/Refactors/Docs).
  // Tipos sem slot (chore/test/style/...) ficam de fora — sem inventar linha.
  // Resumo 1-linha por commit é [novo]/LLM, indisponível hoje -> oculto.
  const DELIV_TYPE_LABEL = { feat: "✦ Features", fix: "☂ Bugfixes", refactor: "⇄ Refactors", docs: "✎ Docs" };
  const DELIV_TPL = [
    { row: 523, label: 524, desc: 525, count: 526 },
    { row: 527, label: 528, desc: 529, count: 530 },
    { row: 531, label: 532, desc: 533, count: 534 },
    { row: 535, label: 536, desc: 537, count: 538 },
  ];
  const deliveries = mapDeliveries(report);
  const deliverGroups = deliveries.groups.filter((g) => DELIV_TYPE_LABEL[g.type]);
  if (deliverGroups.length) {
    set(521, `${deliveries.totalItens} iten${deliveries.totalItens === 1 ? "" : "s"}`);
    DELIV_TPL.forEach((slot, i) => {
      const g = deliverGroups[i];
      if (!g) { hide(slot.row); return; }
      set(slot.label, DELIV_TYPE_LABEL[g.type]);
      hide(slot.desc);
      set(slot.count, String(g.quantidade));
    });
  } else {
    hide(515); // card inteiro — sem commits classificados nesta release (ex.: profile=quick)
  }

  // Arquivos alterados — até 6 slots reais; sem changed_files (ex.:
  // profile=quick não extrai diff por arquivo) -> oculta o card inteiro,
  // nunca mantém a contagem mockada do header sem corpo correspondente.
  const FILES_TPL = [
    { path: 552, add: 553, del: 554 }, { path: 558, add: 559, del: 560 },
    { path: 564, add: 565, del: 566 }, { path: 570, add: 571, del: 572 },
    { path: 576, add: 577, del: 578 }, { path: 582, add: 583, del: 584 },
  ];
  const changed = mapChangedFiles(report);
  if (changed.total) {
    set(547, `${changed.total} arquivo${changed.total === 1 ? "" : "s"}`);
    FILES_TPL.forEach((slot, i) => {
      const f = changed.items[i];
      if (!f) { hide(slot.path, 2); return; }
      set(slot.path, f.path);
      set(slot.add, "+" + (f.added_lines ?? 0));
      set(slot.del, "-" + (f.deleted_lines ?? 0));
    });
  } else {
    hide(542);
  }

  // Segurança — checklist fixo de 4 categorias (SQL Injection/Headers/
  // Redação de segredos/Dependency Scan) não tem 1:1 real (findings são
  // freeform, não por categoria nomeada) -> oculta as 4 linhas, mantém
  // só o score real.
  const sec = mapSecuritySection(report);
  if (!sec.empty) set(601, String(Math.round(sec.score ?? 0))); else hide(599, 2);
  hide(603);
  hide(609, 1); hide(615, 1); hide(621, 1); hide(628, 1);

  // SonarQube — métricas 1:1 reais. Sparkline/delta usam histórico real
  // (runs anteriores do mesmo project_path, via loadProjectHistory em
  // main.js) — sem run anterior disponível, nunca fabrica tendência.
  const sonar = mapSonarSection(report);
  const sonarSeries = history.map(mapSonarSection).filter((s) => !s.empty).map((s) => s.metrics);
  hide(642); // "Quality Gate: Passed" streak — sem contagem real de releases consecutivas.
  if (!sonar.empty && sonarSeries.length) show(641, 0, "inline"); else hide(641);
  const paintSparkline = (containerId, values) => {
    const container = $(containerId);
    if (!container || !values.length) { hide(containerId); return; }
    const bars = Array.from(container.children);
    const padded = Array(Math.max(0, bars.length - values.length)).fill(null).concat(values.slice(-bars.length));
    const maxV = Math.max(...values, 1);
    bars.forEach((wrap, i) => {
      const bar = wrap.firstElementChild;
      const v = padded[i];
      wrap.style.visibility = v == null ? "hidden" : "visible";
      if (bar && v != null) bar.style.height = Math.max(6, Math.round((v / maxV) * 100)) + "%";
    });
    container.style.display = "flex";
  };
  if (!sonar.empty) {
    const m = sonar.metrics;
    const priorValues = (key) => sonarSeries.map((s) => s[key]).filter((v) => v != null);
    const deltaOf = (key) => {
      const prior = priorValues(key);
      if (!prior.length || m[key] == null) return null;
      return m[key] - prior[prior.length - 1];
    };
    const paintMetric = (valId, deltaId, sparkId, key, suffix = "") => {
      set(valId, (m[key] ?? "—") + suffix);
      const d = deltaOf(key);
      if (d == null) hide(deltaId); else set(deltaId, fmtDelta(d));
      paintSparkline(sparkId, [...priorValues(key), m[key]].filter((v) => v != null));
    };
    paintMetric(648, 649, 650, "bugs");
    paintMetric(668, 669, 670, "code_smells");
    paintMetric(688, 689, 690, "vulnerabilities");
    set(707, m.security_hotspots ?? "—"); hide(708);
    set(712, m.duplication_pct != null ? m.duplication_pct + "%" : "—"); hide(713);
    set(717, m.technical_debt ?? "—"); hide(718);
  } else hide(640, 2);

  // Lighthouse — 4 scores reais; Core Web Vitals só se o backend expuser
  // (metrics.lcp_ms/inp_ms/cls — não garantido, oculta se ausente).
  const lh = mapLighthouseSection(report);
  hide(730);
  if (!lh.empty) {
    set(736, String(Math.round(lh.performance ?? 0)));
    set(742, String(Math.round(lh.accessibility ?? 0)));
    set(748, String(Math.round(lh.best_practices ?? 0)));
    set(754, String(Math.round(lh.seo ?? 0)));
    hide(762, 2); hide(768, 2); hide(774, 2); // LCP/INP/CLS — não mapeados hoje
  } else hide(728, 2);

  // Tendência do score — barras reais das últimas runs scored do mesmo projeto.
  // Container 780 (display:none por default). 6 colunas: 789/792/795/798/801/804
  // (cada uma: span label = col+1, div barra = col+2). Labels de versão: 808-813.
  // Variação nesta release: 818 (delta) + 819 (contexto). Bullets 820/822 são
  // narrativa LLM — ocultos (sem fonte real). Só mostra se >= 2 pontos reais.
  const scoreSeries = history
    .filter((h) => h.scores?.score_status === "available" && typeof h.scores?.quality === "number")
    .map((h) => {
      const ref = h.run?.head_ref;
      const label = ref && ref !== "HEAD" ? ref : (h.run?.finished_at ? new Date(h.run.finished_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" }) : h.run?.id || "—");
      return { score: Math.round(h.scores.quality), ref: label };
    });
  if (scoreOk && scoreSeries.length >= 1) {
    const curRef = report.run?.head_ref;
    const curLabel = curRef && curRef !== "HEAD" ? curRef : (report.run?.finished_at ? new Date(report.run.finished_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" }) : "atual");
    const currentScore = Math.round(report.scores.quality);
    const allPoints = [...scoreSeries.slice(-5), { score: currentScore, ref: curLabel }];
    const maxScore = Math.max(...allPoints.map((p) => p.score), 1);
    const COLS = [789, 792, 795, 798, 801, 804];
    const LBLS = [808, 809, 810, 811, 812, 813];
    const padded = Array(Math.max(0, COLS.length - allPoints.length)).fill(null).concat(allPoints);
    padded.forEach((pt, i) => {
      const col = $(COLS[i]), lbl = $(LBLS[i]);
      if (!col || !lbl) return;
      if (pt == null) {
        col.style.visibility = "hidden";
        lbl.style.visibility = "hidden";
        return;
      }
      col.style.visibility = "visible";
      lbl.style.visibility = "visible";
      const labelSpan = col.children[0];
      const bar = col.children[1];
      const isCurrent = i === padded.length - 1;
      if (labelSpan) {
        labelSpan.textContent = String(pt.score);
        labelSpan.style.color = isCurrent ? STATUS_COLOR.ok.stroke : "";
        labelSpan.style.fontWeight = isCurrent ? "800" : "700";
      }
      if (bar) {
        bar.style.height = Math.max(6, Math.round((pt.score / maxScore) * 100)) + "%";
        bar.style.background = isCurrent ? STATUS_COLOR.ok.stroke : "rgb(230,235,243)";
      }
      lbl.textContent = pt.ref || String(pt.score);
      lbl.style.fontWeight = isCurrent ? "700" : "";
      lbl.style.color = isCurrent ? STATUS_COLOR.ok.stroke : "";
      lbl.style.marginBottom = "5px";
    });
    // Variação: delta entre atual e penúltimo ponto real
    const prevPt = allPoints[allPoints.length - 2];
    if (prevPt) {
      const delta = currentScore - prevPt.score;
      set(818, (delta >= 0 ? "+" : "") + delta + " pts");
      style(818, { color: STATUS_COLOR[delta >= 0 ? "ok" : "fail"].stroke });
    } else hide(815);
    hide(819); hide(820); hide(822); hide(824); // narrativa/link LLM — sem fonte real
    show(780, 0, "flex");
  } else hide(780);

  // Performance / Carga (k6) — Latência/Taxa de erro/Throughput reais;
  // "Usuários (rps)" e o gráfico "ao longo do tempo" não têm campo
  // equivalente (métricas agregadas, sem série temporal) -> oculta.
  const k6 = mapK6Section(report);
  if (!k6.empty) {
    set(838, (k6.p95Ms ?? "—") + " ms"); hide(840); hide(841);
    set(856, (k6.throughputRps ?? "—") + " req/s"); hide(858); hide(859);
    set(865, ((k6.errorRate ?? 0) * 100).toFixed(2) + "%"); hide(867); hide(868);
    hide(846, 1); // Usuários (rps) — sem métrica real distinta (tile único, irmão dos outros 3)
    hide(874, 2); // gráfico latência ao longo do tempo — sem série temporal
  } else hide(833, 2);

  // Migrations — 1:1 real (path/rollback); sem migration -> oculta.
  const mig = mapMigrations(report);
  if (!mig.empty && mig.items[0]) {
    set(914, mig.items[0].path);
    set(916, mig.items[0].rollback_present === false ? "sem rollback" : "OK");
    set(921, mig.items[0].impacto || "—");
  } else hide(911, 2);

  // Envs — 1:1 real; mockup só mostra 4 slots fixos.
  const envs = mapEnvs(report);
  if (!envs.empty) set(933, envs.items.length === 1 ? "1 variável alterada" : `${envs.items.length} variáveis alteradas`);
  else hide(932);
  const ENV_TPL = [936, 939, 942, 945];
  ENV_TPL.forEach((tpl, i) => {
    const it = !envs.empty ? envs.items[i] : null;
    if (!it) { hide(tpl, 1); return; }
    set(tpl, it.name);
    const statusTpl = tpl + 1;
    set(statusTpl, it.status || "—");
  });

  // Aplicabilidade — 1:1 real (5 analyzers); mockup tem 4 linhas fixas
  // (Sonar/Lighthouse/ZAP/k6) — a 4ª (ZAP) não é um dos 5 ids reais,
  // reaproveitada pra Coverage (5º analyzer real).
  const APPL_TPL = [{ nome: 961, status: 962 }, { nome: 968, status: 969 }, { nome: 974, status: 975 }, { nome: 978, status: 979 }];
  const appl = mapApplicability(report);
  const order = ["SonarQube", "Lighthouse", "Coverage", "Performance / Carga (k6)"];
  order.forEach((label, i) => {
    const it = appl.find((a) => a.nome === label);
    const slot = APPL_TPL[i];
    if (!it) { hide(slot.nome, 1); return; }
    set(slot.nome, it.nome);
    set(slot.status, it.aplicavel ? "Aplicável" : "Não aplicável");
  });

  // Resumo da Release — "O que mudou" real; Manutenibilidade/Riscos/
  // Desempenho e "Recomendação"/"Próximos passos" são narrativa fixa do
  // mockup sem geração real equivalente hoje -> oculta.
  const summary = mapSummary(report);
  if (!summary.empty && summary.summary) set(1002, summary.summary); else hide(999, 2);
  hide(1005, 1); hide(1009, 1); hide(1013, 1); // Manutenibilidade/Riscos/Desempenho
  hide(1021, 2); // Recomendação
  hide(1027, 3); // Próximos passos (lista inteira)
}
