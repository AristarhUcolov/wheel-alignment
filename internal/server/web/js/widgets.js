// Табло, шкалы допуска и схема автомобиля.
//
// Всё строится один раз, а на каждом кадре меняются только числа, цвета и
// положения стрелок — так экран не мерцает и стрелки двигаются плавно.
// Цвета — переменные CSS: тема и палитра для различения цветов меняют и
// схемы тоже.

import { fmtParam, specValue, fmtDM, shortLabel, paramKind, isToeKey } from './fmt.js';
import { h } from './state.js';
import { t } from './i18n.js';

const clamp = (v, a, b) => Math.max(a, Math.min(b, v));

// Знак состояния рядом с цветом: при дальтонизме красный и зелёный
// неразличимы, а ✓ и ✕ — различимы всегда.
export const GLYPH = { good: '✓', marginal: '!', bad: '✕' };
export const glyphOf = p => (p && p.has && !p.stale && GLYPH[p.status]) || '';

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

// Шкала рисуется в настоящих пикселях своего размера: подписи всегда одного
// читаемого размера, а не уменьшаются вместе с табло до точек. Когда места
// мало, лишние подписи убираются, а не наезжают друг на друга.
const barResize = new ResizeObserver(entries => {
  for (const e of entries) if (e.target.bar) e.target.bar.layout();
});

export class ToleranceBar {
  constructor({ full = false } = {}) {
    this.full = full;
    this.el = h(`
      <svg class="bar" aria-hidden="true" focusable="false">
        <rect class="track" rx="3" fill="var(--track)"/>
        <g class="zones">
          <rect class="zl" fill="var(--zone-bad)"/>
          <rect class="zg" fill="var(--zone-good)"/>
          <rect class="zr" fill="var(--zone-bad)"/>
          <line class="kn" stroke="var(--zone-nominal)" stroke-width="1.5"/>
        </g>
        <text class="t0 tl" text-anchor="middle"></text>
        <text class="t1 tl nom" text-anchor="middle"></text>
        <text class="t2 tl" text-anchor="middle"></text>
        <g class="ptr" style="transition: transform .18s linear">
          <line class="pl" x1="0" x2="0" stroke="var(--ptr)" stroke-width="2.2"/>
          <path class="pt" fill="var(--ptr)" stroke="var(--ptr-edge)" stroke-width="1"/>
        </g>
        <text class="off"></text>
      </svg>`);
    this.q = s => this.el.querySelector(s);
    this.el.bar = this;
    this.spec = undefined;
    this.v = null;
    this.W = 0; this.H = 0;
    barResize.observe(this.el);
  }

  layout() {
    const W = Math.round(this.el.clientWidth), H = Math.round(this.el.clientHeight);
    if (W < 20 || H < 8 || (W === this.W && H === this.H)) return;
    this.W = W; this.H = H;
    this.el.setAttribute('viewBox', `0 0 ${W} ${H}`);
    this.draw();
  }

  x(v) {
    const m = 8;
    return m + (v - this.lo) / (this.hi - this.lo) * (this.W - 2 * m);
  }

  // Перерисовка только при смене допуска или единиц: подписи сравниваются
  // готовым текстом — функция форматирования создаётся заново на каждом кадре.
  setSpec(spec, fmt) {
    const key = spec ? [spec.min, spec.max, spec.nominal, fmt(spec.min), fmt(spec.max), fmt(spec.nominal)].join('|') : '-';
    if (key === this.key) return;
    this.key = key;
    this.spec = spec || null;
    this.fmt = fmt;
    if (this.spec) [this.lo, this.hi] = barDomain(this.spec);
    this.draw();
  }

  // Геометрия по высоте: подписи сверху (если помещаются), полоса, под ней
  // треугольник стрелки.
  geom() {
    const fs = this.full ? 12 : 11;
    const labels = this.H >= fs + 17;
    const y = labels ? fs + 4 : 2;
    const th = clamp(this.H - y - 9, 5, this.full ? 16 : 13);
    return { fs, labels, y, th };
  }

  draw() {
    if (!this.W) return;
    const { fs, labels, y, th } = this.geom();
    const has = !!this.spec;
    const W = this.W;
    const tr = this.q('.track');
    tr.setAttribute('x', 4); tr.setAttribute('y', y); tr.setAttribute('width', W - 8); tr.setAttribute('height', th);
    this.q('.zones').style.display = has ? '' : 'none';
    for (const c of ['.t0', '.t1', '.t2']) { const e = this.q(c); e.textContent = ''; e.style.display = 'none'; }

    const pl = this.q('.pl'), pt = this.q('.pt');
    pl.setAttribute('y1', y - 3); pl.setAttribute('y2', y + th + 3);
    pt.setAttribute('d', `M0 ${y + th + 1} L6.5 ${y + th + 10} L-6.5 ${y + th + 10} Z`);
    const off = this.q('.off');
    off.setAttribute('y', y + th + 10); off.setAttribute('font-size', 12);

    if (has) {
      const x0 = this.x(this.spec.min), x1 = this.x(this.spec.max), xn = this.x(this.spec.nominal);
      const set = (sel, a, b) => { const r = this.q(sel); r.setAttribute('x', a); r.setAttribute('width', Math.max(0, b - a)); r.setAttribute('y', y); r.setAttribute('height', th); };
      set('.zl', 4, x0); set('.zg', x0, x1); set('.zr', x1, W - 4);
      const kn = this.q('.kn');
      kn.setAttribute('x1', xn); kn.setAttribute('x2', xn); kn.setAttribute('y1', y); kn.setAttribute('y2', y + th);
      if (labels) this.placeLabels(fs, x0, x1, xn);
    }
    this.place();
  }

  // Подписи границ и номинала без наложений: сначала убирается номинал, потом
  // границы раздвигаются от зоны наружу, и только если не помещаются и так —
  // подписи не показываются вовсе (норма есть в подсказке и в панели угла).
  placeLabels(fs, x0, x1, xn) {
    const W = this.W, gap = 6;
    const lab = (sel, text, x, anchor = 'middle') => {
      const e = this.q(sel);
      e.textContent = text; e.style.display = '';
      e.setAttribute('font-size', fs); e.setAttribute('y', fs); e.setAttribute('text-anchor', anchor);
      e.setAttribute('x', x);
      return e;
    };
    const width = e => { const w = e.getComputedTextLength ? e.getComputedTextLength() : 0; return w || e.textContent.length * fs * 0.56; };
    const span = (x, w, anchor) => anchor === 'middle' ? [x - w / 2, x + w / 2] : anchor === 'end' ? [x - w, x] : [x, x + w];
    const hit = (a, b) => a[0] < b[1] + gap && b[0] < a[1] + gap;
    const fit = (x, w, anchor) => {
      const [a, b] = span(x, w, anchor);
      if (a < 1) return x + (1 - a);
      if (b > W - 1) return x - (b - (W - 1));
      return x;
    };

    const e0 = lab('.t0', this.fmt(this.spec.min), x0), e2 = lab('.t2', this.fmt(this.spec.max), x1);
    const w0 = width(e0), w2 = width(e2);
    let a0 = 'middle', a2 = 'middle';
    let p0 = fit(x0, w0, a0), p2 = fit(x1, w2, a2);
    if (hit(span(p0, w0, a0), span(p2, w2, a2))) {
      a0 = 'end'; a2 = 'start';
      p0 = fit(x0 - 2, w0, a0); p2 = fit(x1 + 2, w2, a2);
      e0.setAttribute('text-anchor', a0); e2.setAttribute('text-anchor', a2);
    }
    if (hit(span(p0, w0, a0), span(p2, w2, a2))) {
      e0.style.display = 'none'; e2.style.display = 'none';
      return;
    }
    e0.setAttribute('x', p0); e2.setAttribute('x', p2);
    if (this.full) {
      const en = lab('.t1', this.fmt(this.spec.nominal), xn);
      const wn = width(en);
      const pn = fit(xn, wn, 'middle');
      const sn = span(pn, wn, 'middle');
      if (hit(sn, span(p0, w0, a0)) || hit(sn, span(p2, w2, a2))) en.style.display = 'none';
      else en.setAttribute('x', pn);
    }
  }

  // Стрелка: положение и «за краем шкалы».
  place() {
    const ptr = this.q('.ptr'), off = this.q('.off');
    if (this.v === null || this.v === undefined || !this.spec || !this.W) {
      ptr.style.display = 'none';
      off.textContent = '';
      return;
    }
    ptr.style.display = '';
    const x = this.x(this.v);
    const cx = clamp(x, 8, this.W - 8);
    ptr.style.transform = `translateX(${cx}px)`;
    // За краем шкалы — «◀◀» или «▶▶», чтобы было видно, что до допуска далеко.
    if (x < 8) { off.textContent = '◀◀'; off.setAttribute('x', 16); off.setAttribute('text-anchor', 'start'); }
    else if (x > this.W - 8) { off.textContent = '▶▶'; off.setAttribute('x', this.W - 16); off.setAttribute('text-anchor', 'end'); }
    else off.textContent = '';
  }

  setValue(v) {
    this.v = v;
    this.place();
  }
}

// ── Табло параметра ──────────────────────────────────────────────────

export class ValueBox {
  constructor(key, { label, full = false, small = false } = {}) {
    this.key = key;
    this.label = label || shortLabel(key);
    this.bar = new ToleranceBar({ full });
    this.el = h(`
      <div class="vbox none${small ? ' small' : ''}" data-key="${key}" tabindex="0" role="button">
        <div class="lbl"><span class="name"><span class="st"></span><span class="nm"></span></span><span class="mm"></span></div>
        <div class="val">— — —</div>
        <div class="hint"><span class="glyph" aria-hidden="true"></span><span class="ht"></span></div>
      </div>`);
    this.el.querySelector('.nm').textContent = this.label;
    this.el.insertBefore(this.bar.el, this.el.querySelector('.val'));
    this.valEl = this.el.querySelector('.val');
    this.hintEl = this.el.querySelector('.ht');
    this.glyphEl = this.el.querySelector('.glyph');
    this.mmEl = this.el.querySelector('.mm');
    this.aria = '';
    // Enter и пробел открывают панель угла так же, как щелчок.
    this.el.addEventListener('keydown', e => {
      if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); this.el.click(); }
    });
  }

  update(p, units, rimMM) {
    const el = this.el;
    const status = !p || !p.has ? 'none' : p.status === 'no_spec' ? 'nospec' : p.status;
    el.className = el.className.replace(/\b(good|bad|marginal|none|nospec|stale|unstable|settled)\b/g, '').replace(/\s+/g, ' ').trim();
    el.classList.add(status);
    if (p && p.has) {
      el.classList.add(p.stable ? 'settled' : 'unstable');
      if (p.stale) el.classList.add('stale');
    }
    const fmt = d => specValue(this.key, d, units, rimMM);
    this.bar.setSpec(p && p.spec ? p.spec : null, fmt);
    this.bar.setValue(p && p.has ? p.v : null);
    const value = fmtParam(this.key, p, units);
    this.valEl.textContent = value;

    // Миллиметры рядом с угловым схождением — для тех, кто сверяется с
    // руководством, где схождение дано в мм.
    this.mmEl.textContent = (p && p.has && isToeKey(this.key) && units !== 'mm' && p.mm !== undefined && p.mm !== null)
      ? t('{v} мм', { v: `${p.mm > 0 ? '+' : p.mm < 0 ? '−' : ''}${Math.abs(p.mm).toFixed(1)}` }) : '';

    let hint = '';
    // Пустое схождение — сразу сказать, откуда его взять: телефон на ободе его
    // не меряет, и это самый частый вопрос.
    if (!p || !p.has) hint = paramKind(this.key) === 'toe' ? t('нет показаний — струна или камера') : t('нет показаний');
    else if (p.stale) hint = t('датчик молчит');
    else if (!p.spec) hint = t('допуск не задан');
    else if (p.status === 'bad') hint = t('до допуска {v}', { v: fmt(-p.deviation) });
    else if (!p.stable) hint = t('показания меняются…');
    else if (p.status === 'marginal') hint = t('у границы допуска');
    else hint = t('в допуске');
    this.hintEl.textContent = hint;
    this.glyphEl.textContent = glyphOf(p);

    // Для экранного диктора — одной фразой; меняется только вместе с текстом.
    const aria = `${this.label}: ${p && p.has ? value : t('нет показаний')}. ${p && p.has ? hint : ''}`.trim();
    if (aria !== this.aria) { this.aria = aria; el.setAttribute('aria-label', aria); }
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
          <stop offset="0" stop-color="var(--car-body1)"/><stop offset=".5" stop-color="var(--car-body2)"/><stop offset="1" stop-color="var(--car-body1)"/>
        </linearGradient>
        <marker id="arr" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
          <path d="M0 0 L10 5 L0 10 z" fill="var(--needle)"/>
        </marker>
      </defs>
      <g aria-hidden="true">
        <text x="280" y="28" text-anchor="middle" fill="var(--dim)" font-size="14" letter-spacing="2">${t('ПЕРЁД')}</text>
        <line x1="280" y1="40" x2="280" y2="745" stroke="var(--car-glass-line)" stroke-dasharray="6 6"/>
        <path d="M190 78 Q280 52 370 78 L398 150 Q408 190 406 260 L404 640 Q402 700 372 716 Q280 734 188 716 Q158 700 156 640 L154 260 Q152 190 162 150 Z"
              fill="url(#body)" stroke="var(--car-line)" stroke-width="2"/>
        <path d="M186 300 Q280 272 374 300 L360 360 Q280 344 200 360 Z" fill="var(--car-glass)" stroke="var(--car-glass-line)"/>
        <path d="M200 520 Q280 506 360 520 L370 566 Q280 580 190 566 Z" fill="var(--car-glass)" stroke="var(--car-glass-line)"/>
        <line x1="150" y1="215" x2="410" y2="215" stroke="var(--car-line)" stroke-width="3"/>
        <line x1="150" y1="585" x2="410" y2="585" stroke="var(--car-line)" stroke-width="3"/>
        <g class="thrust"><line x1="280" y1="585" x2="280" y2="60" stroke="var(--red)" stroke-width="2" opacity=".8"/></g>
        <g class="arcs"></g>
        <g class="wheels"></g>
        <text class="gain" x="280" y="772" text-anchor="middle" fill="var(--dim)" font-size="11">${t('схождение на схеме увеличено в {n} раз', { n: TOE_GAIN })}</text>
      </g>
      <g class="readouts"></g>`;

    const wheels = s.querySelector('.wheels');
    for (const [k, [x, y]] of Object.entries(W)) {
      const g = document.createElementNS(NS, 'g');
      g.setAttribute('transform', `translate(${x} ${y})`);
      g.innerHTML = `
        <g class="rot" style="cursor:pointer">
          <rect x="-21" y="-50" width="42" height="100" rx="9" fill="var(--tyre)" stroke="var(--tyre-line)" stroke-width="2"/>
          ${[-38, -26, -14, -2, 10, 22, 34].map(yy => `<line x1="-17" y1="${yy}" x2="17" y2="${yy + 6}" stroke="var(--tread)" stroke-width="3"/>`).join('')}
          <rect x="-7" y="-26" width="14" height="52" rx="3" fill="var(--hub)"/>
          <line x1="0" y1="-66" x2="0" y2="66" stroke="var(--wheel-axis)" stroke-width="1.6" stroke-dasharray="4 4"/>
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

    // Табло осевых величин посередине машины. Их можно выбрать и с
    // клавиатуры: на общем виде больше негде увидеть эти значения.
    const ro = s.querySelector('.readouts');
    this.readouts = {};
    const mk = (key, x, y, w = 150) => {
      const g = document.createElementNS(NS, 'g');
      g.setAttribute('transform', `translate(${x} ${y})`);
      g.setAttribute('class', 'ro');
      g.setAttribute('tabindex', '0');
      g.setAttribute('role', 'button');
      g.style.cursor = 'pointer';
      g.innerHTML = `
        <text x="0" y="-7" text-anchor="middle" fill="var(--muted)" font-size="13">${shortLabel(key)}</text>
        <rect class="box" x="${-w / 2}" y="-2" width="${w}" height="34" rx="6" fill="var(--screen)" stroke="var(--car-glass-line)"/>
        <text class="g" x="${-w / 2 + 14}" y="22" text-anchor="middle" font-size="15" font-weight="700" fill="var(--dim)"></text>
        <text class="v" x="6" y="24" text-anchor="middle" font-size="22" font-family="var(--num)" font-weight="600" fill="var(--dim)">— — —</text>`;
      g.addEventListener('click', () => this.onPick && this.onPick(key));
      g.addEventListener('keydown', e => {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); this.onPick && this.onPick(key); }
      });
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
      const text = fmtParam(key, p, units);
      v.textContent = text;
      v.setAttribute('fill', colorOf(p));
      const gl = g.querySelector('.g');
      gl.textContent = glyphOf(p);
      gl.setAttribute('fill', colorOf(p));
      const aria = `${shortLabel(key)}: ${p && p.has ? text : t('нет показаний')}`;
      if (g.getAttribute('aria-label') !== aria) g.setAttribute('aria-label', aria);
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
  if (!p || !p.has) return 'var(--dim)';
  if (p.stale) return 'var(--dim)';
  return { good: 'var(--green)', bad: 'var(--red)', marginal: 'var(--amber)' }[p.status] || 'var(--text)';
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
      <path class="bg" fill="none" stroke="var(--track)" stroke-width="13"/>
      <path class="zl" fill="none" stroke="var(--zone-bad)" stroke-width="13"/>
      <path class="zg" fill="none" stroke="var(--zone-good)" stroke-width="13"/>
      <path class="zr" fill="none" stroke="var(--zone-bad)" stroke-width="13"/>
      <text class="lm" font-size="13" font-weight="700" fill="var(--arc-sign)" text-anchor="middle" dominant-baseline="central"></text>
      <text class="lp" font-size="13" font-weight="700" fill="var(--arc-sign)" text-anchor="middle" dominant-baseline="central"></text>
      <line class="needle" stroke="var(--needle)" stroke-width="4" stroke-linecap="round" marker-end="url(#arr)"/>`;
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

const WHEEL_SIDE = 'fill="var(--tyre)" stroke="var(--tyre-line)" stroke-width="2"';
const AXIS = 'stroke="var(--wheel-axis)" stroke-dasharray="3 3"';

// Развал: вид спереди, два колеса наклоняются на измеренный угол (увеличено).
export function camberPicto() {
  const el = h(`
    <svg viewBox="0 0 240 110" aria-hidden="true">
      <line x1="40" y1="100" x2="200" y2="100" stroke="var(--car-glass-line)" stroke-width="2"/>
      <rect x="70" y="52" width="100" height="10" rx="3" fill="var(--car-line)"/>
      <g class="l" transform="translate(50 100)"><rect x="-13" y="-80" width="26" height="80" rx="6" ${WHEEL_SIDE}/><line x1="0" y1="-92" x2="0" y2="6" ${AXIS}/></g>
      <g class="r" transform="translate(190 100)"><rect x="-13" y="-80" width="26" height="80" rx="6" ${WHEEL_SIDE}/><line x1="0" y1="-92" x2="0" y2="6" ${AXIS}/></g>
      <line x1="50" y1="8" x2="50" y2="100" stroke="var(--car-glass-line)" stroke-dasharray="2 3"/>
      <line x1="190" y1="8" x2="190" y2="100" stroke="var(--car-glass-line)" stroke-dasharray="2 3"/>
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
    <svg viewBox="0 0 240 110" aria-hidden="true">
      <text x="30" y="16" fill="var(--dim)" font-size="11">${t('перёд ◀')}</text>
      <circle cx="120" cy="60" r="42" ${WHEEL_SIDE}/>
      <circle cx="120" cy="60" r="16" fill="var(--hub)"/>
      <line x1="120" y1="4" x2="120" y2="108" stroke="var(--car-glass-line)" stroke-dasharray="2 3"/>
      <g class="ax" transform="translate(120 60)"><line x1="0" y1="-56" x2="0" y2="50" stroke="var(--needle)" stroke-width="4" stroke-linecap="round"/></g>
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
    <svg viewBox="0 0 240 110" aria-hidden="true">
      <text x="120" y="12" fill="var(--dim)" font-size="11" text-anchor="middle">${t('перёд ▲')}</text>
      <line x1="120" y1="18" x2="120" y2="108" stroke="var(--car-glass-line)" stroke-dasharray="2 3"/>
      <rect x="62" y="58" width="116" height="8" rx="3" fill="var(--car-line)"/>
      <g class="l" transform="translate(50 62)"><rect x="-12" y="-40" width="24" height="80" rx="6" ${WHEEL_SIDE}/><line x1="0" y1="-50" x2="0" y2="50" ${AXIS}/></g>
      <g class="r" transform="translate(190 62)"><rect x="-12" y="-40" width="24" height="80" rx="6" ${WHEEL_SIDE}/><line x1="0" y1="-50" x2="0" y2="50" ${AXIS}/></g>
    </svg>`);
  el.update = (l, r) => {
    const a = v => clamp((v || 0) * TOE_GAIN, -25, 25);
    el.querySelector('.l').setAttribute('transform', `translate(50 62) rotate(${a(l).toFixed(2)})`);
    el.querySelector('.r').setAttribute('transform', `translate(190 62) rotate(${(-a(r)).toFixed(2)})`);
  };
  return el;
}

export { fmtDM, paramKind };
