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
// Projeto/PR/Equipe, Score anterior, Tendência do score, gráfico k6 no
// tempo, checklist fixo de 4 categorias de segurança, "Testes de
// regressão", "Próximos passos"/"Recomendação").
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
  severityClass,
} from "./mapping.js";

function fmtDelta(n) {
  if (n == null) return null;
  const sign = n > 0 ? "↑ " : n < 0 ? "↓ " : "= ";
  return sign + Math.abs(n);
}

export function hydrateMockup(doc, report) {
  const $ = (id) => doc.querySelector(`[data-dc-tpl="${id}"]`);
  const set = (id, text) => { const e = $(id); if (e) e.textContent = text; };
  const hide = (id, levels = 0) => {
    let e = $(id);
    for (let i = 0; i < levels && e; i++) e = e.parentElement;
    if (e) e.style.display = "none";
  };

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
  const quality = typeof report.scores?.quality === "number" ? report.scores.quality : null;
  if (quality != null) set(145, String(Math.round(quality))); else hide(145);
  hide(150, 1); // "Score anterior" — sem histórico entre releases ainda

  const SOLID_TPL = {
    S: { score: 162, delta: 164 }, O: { score: 174, delta: 176 },
    L: { score: 186, delta: 188 }, I: { score: 198, delta: 200 },
    D: { score: 210, delta: 212 },
  };
  for (const p of mapSolidPrinciples(report)) {
    const t = SOLID_TPL[p.key];
    if (!t) continue;
    set(t.score, p.scored ? String(p.score) : "N/A");
    if (p.scored && p.delta != null) set(t.delta, fmtDelta(p.delta));
    else hide(t.delta);
  }

  const sustain = mapSustainability(report);
  if (!sustain.empty) { set(220, sustain.title); set(221, sustain.message); }
  else { set(220, "Sem avaliação"); set(221, sustain.reason); }

  // Gate de Entrega — só os 4 critérios com fonte real 1:1; "Score mínimo"
  // e "Testes de regressão" não têm campo real equivalente (rules do
  // quality_gate não expõem número solto; não existe contagem de testes
  // no report) -> oculta os 2 boxes inteiros.
  hide(239, 2); // Score mínimo
  hide(287, 2); // Testes de regressão
  const gate = mapGateSection(report);
  const byId = Object.fromEntries(gate.items.map((it) => [it.id, it]));
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
  const actorSlots = [
    { role: 315, status: 317, scope: 321, findings: 324, conf: 328 },
    { role: 335, status: 337, scope: 341, findings: 344, conf: 348 },
    { role: 355, status: 357, scope: 361, findings: 364, conf: 368 },
  ];
  actorSlots.forEach((slot, i) => {
    const a = ai.actors[i];
    if (!a) { hide(slot.role, 3); return; }
    set(slot.role, a.role || "—");
    set(slot.status, a.status === "completed" ? "Aprovado" : (a.status || "—"));
    set(slot.scope, `${a.provider || "—"}/${a.modelId || "—"}`);
    hide(slot.findings);
    hide(slot.conf);
  });
  if (ai.agreementPct != null) set(381, Math.round(ai.agreementPct) + "%"); else hide(381);
  hide(388); // divergência entre pares (%) — sem base real
  hide(391); // confiabilidade da análise — sem base real
  hide(394); // trechos sem revisão — sem base real
  if (ai.divergences[0]) set(395, ai.divergences[0]); else hide(395);

  // Violações SOLID priorizadas — até 4 slots reais.
  const VIOL_TPL = [
    { sev: 414, principle: 415, fileLine: 416, title: 417, desc: 418, sugLabel: 422, sug: 423 },
    { sev: 430, principle: 431, fileLine: 432, title: 433, desc: null, sugLabel: 437, sug: 439 },
    { sev: 446, principle: 447, fileLine: 448, title: 449, desc: 450, sugLabel: 452, sug: 453 },
    { sev: 460, principle: 461, fileLine: 462, title: 463, desc: null, sugLabel: 467, sug: 469 },
  ];
  const violations = mapViolations(report);
  VIOL_TPL.forEach((slot, i) => {
    const v = violations[i];
    if (!v) { hide(slot.sev, 3); return; }
    set(slot.sev, v.severity || "?");
    set(slot.principle, v.principle || "—");
    set(slot.fileLine, v.file ? v.file + (v.line ? ":" + v.line : "") : "—");
    set(slot.title, v.title);
    if (slot.desc) { if (v.description) set(slot.desc, v.description); else hide(slot.desc); }
    if (v.suggestion) set(slot.sug, v.suggestion); else hide(slot.sugLabel, 1);
  });

  // Arquivos de risco — até 4 slots reais (path + cobertura; sem delta
  // vs release anterior, não existe histórico).
  const RISK_TPL = [
    { path: 485, delta: 486, pct: 487 }, { path: 492, delta: 493, pct: 494 },
    { path: 499, delta: 500, pct: 501 }, { path: 506, delta: 507, pct: 508 },
  ];
  const risk = mapRiskFiles(report);
  RISK_TPL.forEach((slot, i) => {
    const f = !risk.empty ? risk.rows[i] : null;
    if (!f) { hide(slot.path, 2); return; }
    set(slot.path, f.path);
    hide(slot.delta);
    set(slot.pct, (f.coveragePct ?? "—") + "%");
  });

  // Arquivos alterados — até 6 slots reais.
  const FILES_TPL = [
    { path: 552, add: 553, del: 554 }, { path: 558, add: 559, del: 560 },
    { path: 564, add: 565, del: 566 }, { path: 570, add: 571, del: 572 },
    { path: 576, add: 577, del: 578 }, { path: 582, add: 583, del: 584 },
  ];
  const changed = mapChangedFiles(report);
  FILES_TPL.forEach((slot, i) => {
    const f = changed.items[i];
    if (!f) { hide(slot.path, 2); return; }
    set(slot.path, f.path);
    set(slot.add, "+" + (f.added_lines ?? 0));
    set(slot.del, "-" + (f.deleted_lines ?? 0));
  });

  // Segurança — checklist fixo de 4 categorias (SQL Injection/Headers/
  // Redação de segredos/Dependency Scan) não tem 1:1 real (findings são
  // freeform, não por categoria nomeada) -> oculta as 4 linhas, mantém
  // só o score real.
  const sec = mapSecuritySection(report);
  if (!sec.empty) set(601, String(Math.round(sec.score ?? 0))); else hide(599, 2);
  hide(603);
  hide(609, 1); hide(615, 1); hide(621, 1); hide(628, 1);

  // SonarQube — métricas 1:1 reais; sem sparkline/quality-gate-passed
  // (sem histórico multi-release).
  const sonar = mapSonarSection(report);
  hide(641); hide(642);
  if (!sonar.empty) {
    const m = sonar.metrics;
    set(648, String(m.bugs ?? "—")); hide(649);
    set(668, String(m.code_smells ?? "—")); hide(669);
    set(688, String(m.vulnerabilities ?? "—")); hide(689);
    set(707, String(m.security_hotspots ?? "—")); hide(708);
    set(712, (m.duplication_pct ?? "—") + "%"); hide(713);
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

  // Tendência do score — histórico multi-release deliberadamente fora
  // de escopo por ora (decisão do usuário). Oculta a seção inteira.
  hide(785, 2);

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
  const ENV_TPL = [936, 939, 942, 945];
  const envs = mapEnvs(report);
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
