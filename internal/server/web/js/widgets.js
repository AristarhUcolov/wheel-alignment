// Табло, шкалы допуска и схема автомобиля.
//
// Всё строится один раз, а на каждом кадре меняются только числа, цвета и
// положения стрелок — так экран не мерцает и стрелки двигаются плавно.

import { fmtParam, specValue, fmtDM, shortLabel, paramKind, isToeKey } from './fmt.js';
import { h } from './state.js';
import { t } from './i18n.js';

const clamp = (v, a, b) => Math.max(a, Math.min(b, v));

// Видимый диапазон шкалы: зелёная зона допуска посередине и по половине её
// ширины красным с каждой стороны — как на экранах стендов. Минимальная
// ширина — чтобы узкий допуск по схождению не превращался в щель.
export function barDomain(spec) {
  const band = spec.max - spec.min;
  const pad = Math.max(band * 0.5, 0.12);
  let lo = spec.min - pad, hi = spec.max + pad;
  if (hi - lo < 0.4) { const c = (lo + hi) / 2; lo = c - 0.2; hi = c + 0.2; }
  return [lo, hi];
}

const NS = 'http://www.w3.org/2000/svg';

// ── Шкала допуска ────────────────────────────────────────────────────

export class ToleranceBar {
  constructor({ full = false } = {}) {
    this.full = full;
    this.W = 300;
    this.el = h(`
      <svg class="bar" viewBox="0 0 300 44" preserveAspectRatio="xMidYMid meet">
        <rect class="track" x="4" y="15" width="292" height="14" rx="3" fill="#1b2335"/>
        <g class="zones">
          <rect class="zl" y="15" height="14" fill="#b92a2a"/>
          <rect class="zg" y="15" height="14" fill="#1fa653"/>
          <rect class="zr" y="15" height="14" fill="#b92a2a"/>
          <line class="k0" y1="12" y2="15" stroke="#2bd46a" stroke-width="1.5"/>
          <line class="k2" y1="12" y2="15" stroke="#2bd46a" stroke-width="1.5"/>
          <line class="kn" y1="15" y2="29" stroke="#0d4d27" stroke-width="1.5"/>
        </g>
        <text class="t0" y="10" font-size="11.5" fill="#2bd46a" font-family="Bahnschrift, sans-serif" text-anchor="middle"></text>
        <text class="t1" y="10" font-size="11.5" fill="#8ce9ad" font-family="Bahnschrift, sans-serif" text-anchor="middle"></text>
        <text class="t2" y="10" font-size="11.5" fill="#2bd46a" font-family="Bahnschrift, sans-serif" text-anchor="middle"></text>
        <text class="nospec" x="150" y="26" font-size="11" fill="#6b7a93" text-anchor="middle">${t('допуск не задан')}</text>
        <g class="ptr" style="transition: transform .18s linear">
          <line x1="0" y1="12" x2="0" y2="32" stroke="#fff" stroke-width="2.2"/>
          <path d="M0 31 L7.5 42 L-7.5 42 Z" fill="#fff" stroke="#02050b" stroke-width="1"/>
        </g>
        <text class="off" y="41" font-size="12" font-weight="700" fill="#ff4d4d"></text>
      </svg>`);
    this.q = s => this.el.querySelector(s);
    this.spec = undefined;
  }

  x(v) {
    return 4 + (v - this.lo) / (this.hi - this.lo) * 292;
  }

  setSpec(spec, fmt) {
    const same = spec && this.spec && spec.min === this.spec.min && spec.max === this.spec.max;
    if (same && this.fmt === fmt) return;
    this.spec = spec;
    this.fmt = fmt;
    const has = !!spec;
    this.q('.zones').style.display = has ? '' : 'none';
    this.q('.nospec').style.display = has ? 'none' : '';
    for (const c of ['.t0', '.t1', '.t2']) this.q(c).textContent = '';
    if (!has) return;
    [this.lo, this.hi] = barDomain(spec);
    const x0 = this.x(spec.min), x1 = this.x(spec.max);
    const set = (sel, a, b) => { const r = this.q(sel); r.setAttribute('x', a); r.setAttribute('width', Math.max(0, b - a)); };
    set('.zl', 4, x0);
    set('.zg', x0, x1);
    set('.zr', x1, 296);
    const t0 = this.q('.t0'), t1 = this.q('.t1'), t2 = this.q('.t2');
    t0.setAttribute('x', clamp(x0, 24, 276)); t0.textContent = fmt(spec.min);
    t2.setAttribute('x', clamp(x1, 24, 276)); t2.textContent = fmt(spec.max);
    for (const [c, x] of [['.k0', x0], ['.k2', x1], ['.kn', this.x(spec.nominal)]]) {
      this.q(c).setAttribute('x1', x); this.q(c).setAttribute('x2', x);
    }
    if (this.full) { t1.setAttribute('x', this.x(spec.nominal)); t1.textContent = fmt(spec.nominal); }
  }

  setValue(v) {
    const ptr = this.q('.ptr'), off = this.q('.off');
    if (v === null || v === undefined || !this.spec) {
      ptr.style.display = 'none';
      off.textContent = '';
      return;
    }
    ptr.style.display = '';
    const x = this.x(v);
    const cx = clamp(x, 8, 292);
    ptr.style.transform = `translateX(${cx}px)`;
    // За краем шкалы — стрелка «»», чтобы было видно, что до допуска далеко.
    if (x < 8) { off.textContent = '◀◀'; off.setAttribute('x', 14); }
    else if (x > 292) { off.textContent = '▶▶'; off.setAttribute('x', 262); }
    else off.textContent = '';
  }
}

// ── Табло параметра ──────────────────────────────────────────────────

export class ValueBox {
  constructor(key, { label, full = false, small = false } = {}) {
    this.key = key;
    this.bar = new ToleranceBar({ full });
    this.el = h(`
      <div class="vbox none${small ? ' small' : ''}" data-key="${key}">
        <div class="lbl"><span class="name"><span class="st"></span><span class="nm"></span></span><span class="mm"></span></div>
        <div class="val">— — —</div>
        <div class="hint"></div>
      </div>`);
    this.el.querySelector('.nm').textContent = label || shortLabel(key);
    this.el.insertBefore(this.bar.el, this.el.querySelector('.val'));
    this.valEl = this.el.querySelector('.val');
    this.hintEl = this.el.querySelector('.hint');
    this.mmEl = this.el.querySelector('.mm');
  }

  update(p, units, rimMM) {
    const el = this.el;
    const status = !p || !p.has ? 'none' : p.status === 'no_spec' ? 'nospec' : p.status;
    el.className = el.className.replace(/\b(good|bad|marginal|none|nospec|stale|unstable|settled)\b/g, '').trim();
    el.classList.add(status);
    if (p && p.has) {
      el.classList.add(p.stable ? 'settled' : 'unstable');
      if (p.stale) el.classList.add('stale');
    }
    const fmt = d => specValue(this.key, d, units, rimMM);
    this.bar.setSpec(p && p.spec ? p.spec : null, fmt);
    this.bar.setValue(p && p.has ? p.v : null);
    this.valEl.textContent = fmtParam(this.key, p, units);

    // Миллиметры рядом с угловым схождением — для тех, кто сверяется с
    // руководством, где схождение дано в мм.
    this.mmEl.textContent = (p && p.has && isToeKey(this.key) && units !== 'mm' && p.mm !== undefined && p.mm !== null)
      ? t('{v} мм', { v: `${p.mm > 0 ? '+' : p.mm < 0 ? '−' : ''}${Math.abs(p.mm).toFixed(1)}` }) : '';

    let hint = '';
    if (!p || !p.has) hint = t('нет показаний');
    else if (p.stale) hint = t('датчик молчит');
    else if (!p.spec) hint = t('допуск не задан');
    else if (p.status === 'bad') hint = t('до допуска {v}', { v: fmt(-p.deviation) });
    else if (!p.stable) hint = t('показания меняются…');
    else if (p.status === 'marginal') hint = t('у границы допуска');
    else hint = t('в допуске');
    this.hintEl.textContent = hint;
  }
}

// ── Схема автомобиля сверху ──────────────────────────────────────────

// Углы установки колёс — доли градуса; в натуральную величину на схеме их не
// видно. Поэтому схождение показано в 10 раз крупнее, и это подписано.
export const TOE_GAIN = 10;
export const CAMBER_GAIN = 5;
export const CASTER_GAIN = 2.2;

const W = { FL: [128, 215], FR: [432, 215], RL: [128, 585], RR: [432, 585] };

export class CarView {
  constructor({ onPick } = {}) {
    this.onPick = onPick;
    this.el = h(`<svg viewBox="0 0 560 780" preserveAspectRatio="xMidYMid meet" class="car"></svg>`);
    this.build();
    this.shown = { FL: 0, FR: 0, RL: 0, RR: 0, thrust: 0 };
    this.target = { ...this.shown };
    this.anim = this.anim.bind(this);
    requestAnimationFrame(this.anim);
  }

  build() {
    const s = this.el;
    s.innerHTML = `
      <defs>
        <linearGradient id="body" x1="0" x2="1">
          <stop offset="0" stop-color="#1a2640"/><stop offset=".5" stop-color="#22324f"/><stop offset="1" stop-color="#1a2640"/>
        </linearGradient>
        <marker id="arr" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
          <path d="M0 0 L10 5 L0 10 z" fill="#5b9bff"/>
        </marker>
      </defs>
      <text x="280" y="28" text-anchor="middle" fill="#5d6d87" font-size="14" letter-spacing="2">${t('ПЕРЁД')}</text>
      <line x1="280" y1="40" x2="280" y2="745" stroke="#2d4163" stroke-dasharray="6 6"/>
      <path d="M190 78 Q280 52 370 78 L398 150 Q408 190 406 260 L404 640 Q402 700 372 716 Q280 734 188 716 Q158 700 156 640 L154 260 Q152 190 162 150 Z"
            fill="url(#body)" stroke="#34496f" stroke-width="2"/>
      <path d="M186 300 Q280 272 374 300 L360 360 Q280 344 200 360 Z" fill="#0c1424" stroke="#2d4163"/>
      <path d="M200 520 Q280 506 360 520 L370 566 Q280 580 190 566 Z" fill="#0c1424" stroke="#2d4163"/>
      <line x1="150" y1="215" x2="410" y2="215" stroke="#34496f" stroke-width="3"/>
      <line x1="150" y1="585" x2="410" y2="585" stroke="#34496f" stroke-width="3"/>
      <g class="thrust"><line x1="280" y1="585" x2="280" y2="60" stroke="#ff4d4d" stroke-width="2" opacity=".8"/></g>
      <g class="arcs"></g>
      <g class="wheels"></g>
      <g class="readouts"></g>
      <text class="gain" x="280" y="772" text-anchor="middle" fill="#5d6d87" font-size="11">${t('схождение на схеме увеличено в {n} раз', { n: TOE_GAIN })}</text>`;

    const wheels = s.querySelector('.wheels');
    for (const [k, [x, y]] of Object.entries(W)) {
      const g = document.createElementNS(NS, 'g');
      g.setAttribute('transform', `translate(${x} ${y})`);
      g.innerHTML = `
        <g class="rot" style="cursor:pointer">
          <rect x="-21" y="-50" width="42" height="100" rx="9" fill="#0b0f16" stroke="#46597c" stroke-width="2"/>
          ${[-38, -26, -14, -2, 10, 22, 34].map(yy => `<line x1="-17" y1="${yy}" x2="17" y2="${yy + 6}" stroke="#2a3346" stroke-width="3"/>`).join('')}
          <rect x="-7" y="-26" width="14" height="52" rx="3" fill="#3a4a66"/>
          <line x1="0" y1="-66" x2="0" y2="66" stroke="#ffb020" stroke-width="1.6" stroke-dasharray="4 4"/>
        </g>`;
      g.dataset.wheel = k;
      g.querySelector('.rot').addEventListener('click', () => this.onPick && this.onPick('toe_' + k));
      wheels.appendChild(g);
    }

    // Дуговые шкалы над колёсами, как на стендах: внешняя — развал,
    // внутренняя — схождение. Стрелка идёт в ту сторону, куда физически
    // наклоняется или поворачивается колесо.
    const arcs = s.querySelector('.arcs');
    this.arcs = {};
    for (const [k, [x, y]] of Object.entries(W)) {
      const left = k[1] === 'L';
      this.arcs['camber_' + k] = new ArcGauge(arcs, x, y, 118, left ? -1 : 1, () => this.onPick && this.onPick('camber_' + k));
      this.arcs['toe_' + k] = new ArcGauge(arcs, x, y, 92, left ? 1 : -1, () => this.onPick && this.onPick('toe_' + k));
    }

    // Табло осевых величин посередине машины.
    const ro = s.querySelector('.readouts');
    this.readouts = {};
    const mk = (key, x, y, w = 150) => {
      const g = document.createElementNS(NS, 'g');
      g.setAttribute('transform', `translate(${x} ${y})`);
      g.style.cursor = 'pointer';
      g.innerHTML = `
        <text x="0" y="-7" text-anchor="middle" fill="#93a3bb" font-size="13">${shortLabel(key)}</text>
        <rect x="${-w / 2}" y="-2" width="${w}" height="34" rx="6" fill="#02050b" stroke="#2d4163"/>
        <text class="v" x="0" y="24" text-anchor="middle" font-size="22" font-family="Bahnschrift, sans-serif" font-weight="600" fill="#5d6d87">— — —</text>`;
      g.addEventListener('click', () => this.onPick && this.onPick(key));
      ro.appendChild(g);
      this.readouts[key] = g;
    };
    mk('front_total_toe', 280, 128);
    mk('front_cross_camber', 280, 190);
    mk('front_cross_caster', 280, 252);
    mk('rear_total_toe', 280, 628);
    mk('rear_cross_camber', 280, 690);
    mk('thrust_angle', 280, 430, 160);
  }

  update(frame, units, rimMM) {
    if (!frame) return;
    const P = frame.params;
    for (const w of ['FL', 'FR', 'RL', 'RR']) {
      const t = P['toe_' + w];
      this.target[w] = t && t.has ? t.v : 0;
    }
    const th = P.thrust_angle;
    this.target.thrust = th && th.has ? th.v : 0;
    this.el.querySelector('.thrust').style.opacity = th && th.has ? 1 : 0;

    for (const [key, arc] of Object.entries(this.arcs)) arc.update(P[key]);

    for (const [key, g] of Object.entries(this.readouts)) {
      const p = P[key];
      const v = g.querySelector('.v');
      v.textContent = fmtParam(key, p, units);
      v.setAttribute('fill', colorOf(p));
    }
  }

  anim() {
    // Плавно подтягиваем показанный угол к измеренному: колёса двигаются,
    // а не прыгают, даже если кадры приходят 10 раз в секунду.
    for (const k of Object.keys(this.shown)) this.shown[k] += (this.target[k] - this.shown[k]) * 0.18;
    for (const g of this.el.querySelectorAll('.wheels > g')) {
      const k = g.dataset.wheel;
      const toe = clamp(this.shown[k] * TOE_GAIN, -25, 25);
      const rot = k[1] === 'L' ? toe : -toe;
      g.querySelector('.rot').setAttribute('transform', `rotate(${rot.toFixed(2)})`);
    }
    const ang = clamp(this.shown.thrust * TOE_GAIN, -20, 20) * Math.PI / 180;
    const line = this.el.querySelector('.thrust line');
    line.setAttribute('x2', (280 + Math.tan(ang) * 525).toFixed(1));
    if (this.el.isConnected) requestAnimationFrame(this.anim);
    else this.dead = true;
  }

  revive() {
    if (this.dead) { this.dead = false; requestAnimationFrame(this.anim); }
  }
}

export function colorOf(p) {
  if (!p || !p.has) return '#5d6d87';
  if (p.stale) return '#6b7a93';
  return { good: '#2bd46a', bad: '#ff4d4d', marginal: '#ffb020' }[p.status] || '#e8eef8';
}

// ── Дуговая шкала ────────────────────────────────────────────────────

class ArcGauge {
  // dir: +1 — рост значения ведёт стрелку по часовой (вправо), −1 — против.
  constructor(parent, cx, cy, r, dir, onClick) {
    this.cx = cx; this.cy = cy; this.r = r; this.dir = dir;
    this.span = 44; // градусов дуги в каждую сторону от вертикали
    this.g = document.createElementNS(NS, 'g');
    this.g.style.cursor = 'pointer';
    this.g.addEventListener('click', onClick);
    parent.appendChild(this.g);
    this.g.innerHTML = `
      <path class="bg" fill="none" stroke="#1b2335" stroke-width="13"/>
      <path class="zl" fill="none" stroke="#b92a2a" stroke-width="13"/>
      <path class="zg" fill="none" stroke="#1fa653" stroke-width="13"/>
      <path class="zr" fill="none" stroke="#b92a2a" stroke-width="13"/>
      <text class="lm" font-size="13" font-weight="700" fill="#fff" text-anchor="middle" dominant-baseline="central"></text>
      <text class="lp" font-size="13" font-weight="700" fill="#fff" text-anchor="middle" dominant-baseline="central"></text>
      <line class="needle" stroke="#5b9bff" stroke-width="4" stroke-linecap="round" marker-end="url(#arr)"/>`;
    this.q = s => this.g.querySelector(s);
    this.q('.bg').setAttribute('d', this.arc(-this.span, this.span));
    // «+» на том конце дуги, куда уходит стрелка при росте значения.
    const lp = this.pt(this.span - 5, this.r), lm = this.pt(-this.span + 5, this.r);
    const plus = dir > 0 ? lp : lm, minus = dir > 0 ? lm : lp;
    this.q('.lp').setAttribute('x', plus[0].toFixed(1)); this.q('.lp').setAttribute('y', plus[1].toFixed(1)); this.q('.lp').textContent = '+';
    this.q('.lm').setAttribute('x', minus[0].toFixed(1)); this.q('.lm').setAttribute('y', minus[1].toFixed(1)); this.q('.lm').textContent = '−';
    this.spec = undefined;
    this.shown = null;
  }

  pt(a, r) {
    const t = (a - 90) * Math.PI / 180;
    return [this.cx + r * Math.cos(t), this.cy + r * Math.sin(t)];
  }
  arc(a0, a1) {
    const [x0, y0] = this.pt(a0, this.r), [x1, y1] = this.pt(a1, this.r);
    return `M${x0.toFixed(1)} ${y0.toFixed(1)} A${this.r} ${this.r} 0 0 1 ${x1.toFixed(1)} ${y1.toFixed(1)}`;
  }
  ang(v) {
    const k = (v - this.lo) / (this.hi - this.lo) * 2 - 1; // −1…1
    return this.dir * clamp(k, -1.08, 1.08) * this.span;
  }

  update(p) {
    const spec = p && p.spec;
    const has = !!spec;
    if (spec !== this.spec && (!spec || !this.spec || spec.min !== this.spec.min || spec.max !== this.spec.max)) {
      this.spec = spec;
      for (const c of ['.zl', '.zg', '.zr']) this.q(c).style.display = has ? '' : 'none';
      if (has) {
        [this.lo, this.hi] = barDomain(spec);
        const a = this.ang(spec.min), b = this.ang(spec.max);
        const [g0, g1] = a < b ? [a, b] : [b, a];
        this.q('.zl').setAttribute('d', this.arc(-this.span, g0));
        this.q('.zg').setAttribute('d', this.arc(g0, g1));
        this.q('.zr').setAttribute('d', this.arc(g1, this.span));
      }
    }
    const n = this.q('.needle');
    if (!p || !p.has || !has) { n.style.display = 'none'; return; }
    n.style.display = '';
    const target = this.ang(p.v);
    this.shown = this.shown === null ? target : this.shown + (target - this.shown) * 0.5;
    const [x0, y0] = this.pt(this.shown, 26);
    const [x1, y1] = this.pt(this.shown, this.r - 9);
    n.setAttribute('x1', x0.toFixed(1)); n.setAttribute('y1', y0.toFixed(1));
    n.setAttribute('x2', x1.toFixed(1)); n.setAttribute('y2', y1.toFixed(1));
  }
}

// ── Пиктограммы оси (вид «Передняя/Задняя ось») ──────────────────────

// Развал: вид спереди, два колеса наклоняются на измеренный угол (увеличено).
export function camberPicto() {
  const el = h(`
    <svg viewBox="0 0 240 110">
      <line x1="40" y1="100" x2="200" y2="100" stroke="#2d4163" stroke-width="2"/>
      <rect x="70" y="52" width="100" height="10" rx="3" fill="#34496f"/>
      <g class="l" transform="translate(50 100)"><rect x="-13" y="-80" width="26" height="80" rx="6" fill="#0b0f16" stroke="#46597c" stroke-width="2"/><line x1="0" y1="-92" x2="0" y2="6" stroke="#ffb020" stroke-dasharray="3 3"/></g>
      <g class="r" transform="translate(190 100)"><rect x="-13" y="-80" width="26" height="80" rx="6" fill="#0b0f16" stroke="#46597c" stroke-width="2"/><line x1="0" y1="-92" x2="0" y2="6" stroke="#ffb020" stroke-dasharray="3 3"/></g>
      <line x1="50" y1="8" x2="50" y2="100" stroke="#2d4163" stroke-dasharray="2 3"/>
      <line x1="190" y1="8" x2="190" y2="100" stroke="#2d4163" stroke-dasharray="2 3"/>
    </svg>`);
  el.update = (l, r) => {
    // Положительный развал — верх колеса наружу.
    const a = v => clamp((v || 0) * CAMBER_GAIN, -30, 30);
    el.querySelector('.l').setAttribute('transform', `translate(50 100) rotate(${(-a(l)).toFixed(2)})`);
    el.querySelector('.r').setAttribute('transform', `translate(190 100) rotate(${a(r).toFixed(2)})`);
  };
  return el;
}

// Кастер: вид сбоку, наклон оси поворота (верх назад — положительный).
export function casterPicto() {
  const el = h(`
    <svg viewBox="0 0 240 110">
      <text x="30" y="16" fill="#5d6d87" font-size="11">${t('перёд ◀')}</text>
      <circle cx="120" cy="60" r="42" fill="#0b0f16" stroke="#46597c" stroke-width="2"/>
      <circle cx="120" cy="60" r="16" fill="#3a4a66"/>
      <line x1="120" y1="4" x2="120" y2="108" stroke="#2d4163" stroke-dasharray="2 3"/>
      <g class="ax" transform="translate(120 60)"><line x1="0" y1="-56" x2="0" y2="50" stroke="#5b9bff" stroke-width="4" stroke-linecap="round"/></g>
    </svg>`);
  el.update = c => {
    // Перёд слева: верх оси назад — поворот по часовой стрелке.
    const a = clamp((c || 0) * CASTER_GAIN, -35, 35);
    el.querySelector('.ax').setAttribute('transform', `translate(120 60) rotate(${a.toFixed(2)})`);
  };
  return el;
}

// Схождение: вид сверху, колёса повёрнуты на угол схождения (увеличено).
export function toePicto() {
  const el = h(`
    <svg viewBox="0 0 240 110">
      <text x="120" y="12" fill="#5d6d87" font-size="11" text-anchor="middle">${t('перёд ▲')}</text>
      <line x1="120" y1="18" x2="120" y2="108" stroke="#2d4163" stroke-dasharray="2 3"/>
      <rect x="62" y="58" width="116" height="8" rx="3" fill="#34496f"/>
      <g class="l" transform="translate(50 62)"><rect x="-12" y="-40" width="24" height="80" rx="6" fill="#0b0f16" stroke="#46597c" stroke-width="2"/><line x1="0" y1="-50" x2="0" y2="50" stroke="#ffb020" stroke-dasharray="3 3"/></g>
      <g class="r" transform="translate(190 62)"><rect x="-12" y="-40" width="24" height="80" rx="6" fill="#0b0f16" stroke="#46597c" stroke-width="2"/><line x1="0" y1="-50" x2="0" y2="50" stroke="#ffb020" stroke-dasharray="3 3"/></g>
    </svg>`);
  el.update = (l, r) => {
    const a = v => clamp((v || 0) * TOE_GAIN, -25, 25);
    el.querySelector('.l').setAttribute('transform', `translate(50 62) rotate(${a(l).toFixed(2)})`);
    el.querySelector('.r').setAttribute('transform', `translate(190 62) rotate(${(-a(r)).toFixed(2)})`);
  };
  return el;
}

export { fmtDM, paramKind };
