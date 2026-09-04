// dom.js — helpers de DOM/animação compartilhados entre main.js e
// web/src/views/*.js. Extraído de main.js quando este passou de ~500
// linhas (plano Fase A, guidance de split). Zero deps (ADR-0074).

export function el(tag, props, ...children) {
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
    // SAI-129 (achado validação e2e): child não-Node ia direto pro
    // appendChild() e quebrava a página inteira. Primitivo vira texto.
    if (c instanceof Node) e.appendChild(c);
    else e.appendChild(document.createTextNode(String(c)));
  }
  return e;
}

export function badge(status) {
  return el("span", { class: "badge " + status }, status);
}

// SAI-138: animação de números/gráficos ao carregar a página.
export function animateNumber(node, target, { decimals = 0, duration = 600 } = {}) {
  if (target == null || Number.isNaN(target)) return;
  const start = performance.now();
  function tick(now) {
    const t = Math.min(1, (now - start) / duration);
    const eased = 1 - Math.pow(1 - t, 3); // ease-out cubic
    node.textContent = (target * eased).toFixed(decimals);
    if (t < 1) requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);
}

export function animateWidth(node, targetPct) {
  node.style.width = "0%";
  node.style.transition = "width 600ms ease-out";
  requestAnimationFrame(() => requestAnimationFrame(() => {
    node.style.width = Math.max(0, Math.min(100, targetPct || 0)) + "%";
  }));
}

export function animateBarHeight(node, targetPct) {
  node.style.height = "0%";
  node.style.transition = "height 500ms ease-out";
  requestAnimationFrame(() => requestAnimationFrame(() => {
    node.style.height = Math.max(4, Math.min(100, targetPct || 0)) + "%";
  }));
}

// Medidor circular (contrato §"Componentes"): viewBox 0 0 36 36, r=14,
// stroke-dasharray = pct/100 × 87.9 (circunferência arredondada), rotate(-90)
// via CSS (.gauge-circular .value). strokeWidth 3.6 = Lighthouse, 5 = consenso.
const SVG_NS = "http://www.w3.org/2000/svg";
function svgEl(tag, attrs) {
  const e = document.createElementNS(SVG_NS, tag);
  for (const k in attrs) e.setAttribute(k, attrs[k]);
  return e;
}

export function gaugeCircular(pct, { strokeWidth = 3.6, colorClass = "ok" } = {}) {
  const svg = svgEl("svg", { viewBox: "0 0 36 36", class: "gauge-circular" });
  svg.appendChild(svgEl("circle", { class: "track", cx: 18, cy: 18, r: 14, "stroke-width": strokeWidth }));
  const value = svgEl("circle", {
    class: "value " + colorClass, cx: 18, cy: 18, r: 14,
    "stroke-width": strokeWidth, "stroke-dasharray": "0 87.9",
  });
  svg.appendChild(value);
  if (pct != null) animateGaugeArc(value, pct);
  return svg;
}

export function animateGaugeArc(circleEl, targetPct, duration = 800) {
  const start = performance.now();
  const target = Math.max(0, Math.min(100, targetPct || 0));
  function tick(now) {
    const t = Math.min(1, (now - start) / duration);
    const eased = 1 - Math.pow(1 - t, 3);
    circleEl.setAttribute("stroke-dasharray", (target * eased / 100 * 87.9).toFixed(2) + " 87.9");
    if (t < 1) requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);
}

// Tile de KPI com medidor circular + número sobreposto (HTML, nunca <text>
// no SVG — mesma regra do gráfico de linha, ainda que aqui o viewBox seja
// quadrado e não distorça; overlay HTML é mais simples de estilizar).
export function gaugeTile(label, pct, colorClass, strokeWidth) {
  const num = el("div", { class: "gauge-value" }, pct == null ? "—" : "0");
  const wrap = el("div", { class: "gauge-wrap" }, gaugeCircular(pct, { colorClass, strokeWidth }), num);
  if (pct != null) animateNumber(num, pct, { decimals: 0 });
  return el("div", { class: "kpi-tile gauge-tile" }, wrap, el("div", { class: "label" }, label));
}

// Ícone SVG inline (refino visual Fase A1: "ícones SVG", nunca emoji-como-ícone).
export function svgIcon(pathsD, viewBox = "0 0 24 24") {
  const svg = svgEl("svg", { viewBox, class: "icon" });
  for (const d of Array.isArray(pathsD) ? pathsD : [pathsD]) {
    svg.appendChild(svgEl("path", { d }));
  }
  return svg;
}

export function emptyCard(title, reason) {
  return el("div", { class: "card" },
    el("div", { class: "card-header" }, title),
    el("div", { class: "card-body" },
      el("div", { class: "empty-state" }, reason || "sem dados nesta release")));
}
