// Иллюстрации: углы колёс, подготовка, приёмы замера и регулировки, износ
// шин. Рисуются кодом, а не картинками: берут цвета темы (тёмная, светлая,
// контрастная, палитра для дальтонизма), подписи — на языке интерфейса, и
// весят несколько килобайт на всю программу.
//
// Углы на схемах увеличены — в натуральную величину градус не разглядеть;
// так и подписано там, где это важно.

import { t } from './i18n.js';
import { esc } from './fmt.js';

const r1 = v => Math.round(v * 10) / 10;
const rad = d => d * Math.PI / 180;

// Стрелка без <marker>: у нескольких схем на одной странице не должно быть
// одинаковых id.
function arrow(x1, y1, x2, y2, color = 'var(--blue)', w = 3) {
  const a = Math.atan2(y2 - y1, x2 - x1), h = 7 + w * 1.6;
  const bx = x2 - h * 0.7 * Math.cos(a), by = y2 - h * 0.7 * Math.sin(a);
  const p1 = [x2 - h * Math.cos(a - 0.42), y2 - h * Math.sin(a - 0.42)];
  const p2 = [x2 - h * Math.cos(a + 0.42), y2 - h * Math.sin(a + 0.42)];
  return `<line x1="${r1(x1)}" y1="${r1(y1)}" x2="${r1(bx)}" y2="${r1(by)}" stroke="${color}" stroke-width="${w}" stroke-linecap="round"/>` +
    `<polygon points="${r1(x2)},${r1(y2)} ${r1(p1[0])},${r1(p1[1])} ${r1(p2[0])},${r1(p2[1])}" fill="${color}"/>`;
}

// Дуга угла: от направления a0 к a1 (градусы, 0 — вверх, по часовой).
function arc(cx, cy, r, a0, a1, color = 'var(--amber)', w = 2.5) {
  const p = a => [cx + r * Math.sin(rad(a)), cy - r * Math.cos(rad(a))];
  const [x0, y0] = p(a0), [x1, y1] = p(a1);
  const sweep = a1 > a0 ? 1 : 0;
  return `<path d="M${r1(x0)} ${r1(y0)} A${r} ${r} 0 0 ${sweep} ${r1(x1)} ${r1(y1)}" fill="none" stroke="${color}" stroke-width="${w}"/>`;
}

function tx(x, y, s, { size = 12, anchor = 'middle', color = 'var(--muted)', weight = 400 } = {}) {
  return `<text x="${r1(x)}" y="${r1(y)}" text-anchor="${anchor}" font-size="${size}" font-weight="${weight}" fill="${color}">${esc(s)}</text>`;
}

const dash = (x1, y1, x2, y2, color = 'var(--dim)') =>
  `<line x1="${r1(x1)}" y1="${r1(y1)}" x2="${r1(x2)}" y2="${r1(y2)}" stroke="${color}" stroke-width="1.5" stroke-dasharray="4 4"/>`;

// Колесо, вид спереди или сверху: прямоугольник шины, повёрнутый на угол.
function tyre(cx, cy, w, h, angle = 0, extra = '') {
  return `<g transform="translate(${r1(cx)} ${r1(cy)}) rotate(${r1(angle)})">
    <rect x="${-w / 2}" y="${-h / 2}" width="${w}" height="${h}" rx="${Math.min(w, h) * 0.3}" fill="var(--tyre)" stroke="var(--tyre-line)" stroke-width="2"/>
    <line x1="0" y1="${-h / 2 - 9}" x2="0" y2="${h / 2 + 9}" stroke="var(--wheel-axis)" stroke-width="1.6" stroke-dasharray="3 3"/>${extra}</g>`;
}

// Колесо, вид сбоку: шина, обод, ступица.
function wheelSide(cx, cy, r) {
  return `<circle cx="${cx}" cy="${cy}" r="${r}" fill="var(--tyre)" stroke="var(--tyre-line)" stroke-width="2"/>
    <circle cx="${cx}" cy="${cy}" r="${r1(r * 0.68)}" fill="var(--hub)" stroke="var(--car-line)" stroke-width="2"/>
    <circle cx="${cx}" cy="${cy}" r="${r1(r * 0.6)}" fill="var(--car-body1)"/>
    <circle cx="${cx}" cy="${cy}" r="${r1(r * 0.13)}" fill="var(--hub)"/>`;
}

const ok = (x, y) => `<g transform="translate(${x} ${y})"><circle r="11" fill="var(--green)"/><path d="M-5 0 L-1 4 L6 -4" fill="none" stroke="var(--on-accent)" stroke-width="2.6" stroke-linecap="round"/></g>`;
const no = (x, y) => `<g transform="translate(${x} ${y})"><circle r="11" fill="var(--red)"/><path d="M-4.5 -4.5 L4.5 4.5 M4.5 -4.5 L-4.5 4.5" stroke="var(--on-accent)" stroke-width="2.6" stroke-linecap="round"/></g>`;

function svg(w, h, title, body) {
  return `<svg class="illus-svg" viewBox="0 0 ${w} ${h}" role="img" aria-label="${esc(title)}"><title>${esc(title)}</title>${body}</svg>`;
}

// Руль, вид из салона, повёрнутый на угол (градусы, плюс — вправо).
function steeringWheel(cx, cy, r, angle = 0) {
  return `<g transform="translate(${cx} ${cy}) rotate(${angle})">
    <circle r="${r}" fill="none" stroke="var(--text)" stroke-width="${r1(r * 0.18)}"/>
    <circle r="${r1(r * 0.24)}" fill="var(--text)"/>
    <path d="M${-r * 0.95} ${r * 0.06} H${-r * 0.24} M${r * 0.24} ${r * 0.06} H${r * 0.95} M0 ${r * 0.26} V${r * 0.95}" stroke="var(--text)" stroke-width="${r1(r * 0.15)}" stroke-linecap="round"/>
    <rect x="${-r * 0.08}" y="${-r * 1.12}" width="${r * 0.16}" height="${r * 0.26}" fill="var(--amber)"/></g>`;
}

// Полка «перёд ▲» для схем сверху.
const frontUp = (x, y) => tx(x, y, t('перёд ▲'), { size: 11, color: 'var(--dim)' });

const ILLUS = {
  // ── Углы ─────────────────────────────────────────────────────────
  camber() {
    const panel = (x0, sign, title, sub) => {
      const a = 9 * sign;
      return `${tx(x0 + 75, 18, title, { size: 13, color: 'var(--text)', weight: 600 })}${tx(x0 + 75, 34, sub, { size: 11 })}
        <line x1="${x0 + 6}" y1="146" x2="${x0 + 144}" y2="146" stroke="var(--line2)" stroke-width="2"/>
        <rect x="${x0 + 42}" y="86" width="66" height="9" rx="3" fill="var(--car-line)"/>
        ${dash(x0 + 30, 52, x0 + 30, 146)}${dash(x0 + 120, 52, x0 + 120, 146)}
        ${tyre(x0 + 30, 108, 20, 66, -a)}${tyre(x0 + 120, 108, 20, 66, a)}
        ${arc(x0 + 30, 108, 46, 0, -a)}${arc(x0 + 120, 108, 46, 0, a)}`;
    };
    return svg(320, 156, t('Развал: плюс и минус'),
      panel(4, 1, t('Плюс'), t('верх колеса наружу')) + panel(166, -1, t('Минус'), t('верх колеса внутрь')) +
      tx(160, 154, t('вид спереди, угол увеличен'), { size: 10, color: 'var(--dim)' }));
  },

  toe() {
    const panel = (x0, sign, title, sub) => {
      const a = 7 * sign;
      return `${tx(x0 + 75, 18, title, { size: 13, color: 'var(--text)', weight: 600 })}${tx(x0 + 75, 34, sub, { size: 11 })}
        <rect x="${x0 + 44}" y="102" width="62" height="8" rx="3" fill="var(--car-line)"/>
        ${dash(x0 + 32, 56, x0 + 32, 158)}${dash(x0 + 118, 56, x0 + 118, 158)}
        ${tyre(x0 + 32, 106, 18, 64, a)}${tyre(x0 + 118, 106, 18, 64, -a)}
        ${arc(x0 + 32, 106, 44, 0, a)}${arc(x0 + 118, 106, 44, 0, -a)}`;
    };
    return svg(320, 176, t('Схождение: плюс и минус'),
      frontUp(160, 50) + panel(4, 1, t('Схождение (+)'), t('передние края ближе')) +
      panel(166, -1, t('Расхождение (−)'), t('передние края дальше')) +
      tx(160, 173, t('вид сверху, угол увеличен'), { size: 10, color: 'var(--dim)' }));
  },

  caster() {
    const a = 12; // наклон оси на схеме, градусов
    const cx = 168, cy = 104, L = 92;
    const top = [cx + L * Math.sin(rad(a)), cy - L * Math.cos(rad(a))];
    const bot = [cx - 66 * Math.sin(rad(a)) / Math.cos(rad(a)), 168];
    return svg(320, 184, t('Кастер: наклон оси поворота вперёд-назад'),
      `<line x1="10" y1="168" x2="310" y2="168" stroke="var(--line2)" stroke-width="2"/>
      ${wheelSide(cx, cy, 64)}
      ${dash(cx, 18, cx, 168)}
      <line x1="${r1(top[0])}" y1="${r1(top[1])}" x2="${r1(bot[0])}" y2="${r1(bot[1])}" stroke="var(--blue)" stroke-width="4" stroke-linecap="round"/>
      ${arc(cx, cy, 78, 0, a)}
      ${tx(cx + 30, 24, t('ось поворота'), { size: 12, color: 'var(--blue)', anchor: 'start' })}
      ${arrow(70, 30, 26, 30, 'var(--dim)', 2)}${tx(48, 22, t('перёд'), { size: 11 })}
      ${tx(160, 182, t('плюс — верх оси назад; вид сбоку, угол увеличен'), { size: 11 })}`);
  },

  sai() {
    const cx = 110, cy = 100;
    const camberA = 3, saiA = 13;
    const up = [cx + 62 * Math.sin(rad(saiA)), cy - 62 * Math.cos(rad(saiA))];
    const lo = [cx - 64 * Math.sin(rad(saiA)), cy + 64 * Math.cos(rad(saiA))];
    return svg(320, 180, t('Поперечный наклон оси поворота (SAI) и включённый угол'),
      `<line x1="20" y1="172" x2="300" y2="172" stroke="var(--line2)" stroke-width="2"/>
      ${tyre(cx, cy + 6, 26, 112, -camberA)}
      ${dash(cx, 20, cx, 172)}
      <line x1="${r1(up[0])}" y1="${r1(up[1])}" x2="${r1(lo[0])}" y2="${r1(lo[1])}" stroke="var(--blue)" stroke-width="4" stroke-linecap="round"/>
      <circle cx="${r1(up[0])}" cy="${r1(up[1])}" r="6" fill="var(--blue)"/><circle cx="${r1(lo[0])}" cy="${r1(lo[1])}" r="6" fill="var(--blue)"/>
      ${arc(cx, cy, 58, 0, saiA, 'var(--blue)')}${arc(cx, cy, 72, -camberA, saiA)}
      ${tx(176, 36, t('SAI — наклон оси'), { anchor: 'start', color: 'var(--blue)' })}
      ${tx(176, 56, t('к центру машины →'), { anchor: 'start', size: 11 })}
      ${tx(176, 112, t('включённый угол ='), { anchor: 'start', color: 'var(--amber)' })}
      ${tx(176, 128, t('SAI + развал'), { anchor: 'start', color: 'var(--amber)' })}
      ${tx(176, 152, t('не регулируется: разница —'), { anchor: 'start', size: 11 })}
      ${tx(176, 166, t('признак погнутой детали'), { anchor: 'start', size: 11 })}`);
  },

  thrust() {
    const car = `<rect x="110" y="34" width="100" height="196" rx="34" fill="var(--car-body2)" stroke="var(--car-line)" stroke-width="2"/>`;
    const th = 7;
    const tl = [160 + 200 * Math.sin(rad(th)), 196 - 200 * Math.cos(rad(th))];
    return svg(320, 246, t('Угол тяги: куда направлена задняя ось'),
      `${frontUp(160, 22)}${car}
      ${tyre(110, 74, 16, 46)}${tyre(210, 74, 16, 46)}
      ${tyre(110, 196, 16, 46, th)}${tyre(210, 196, 16, 46, th)}
      ${dash(160, 30, 160, 236)}
      <line x1="160" y1="196" x2="${r1(tl[0])}" y2="${r1(tl[1])}" stroke="var(--red)" stroke-width="3"/>
      ${arc(160, 196, 96, 0, th, 'var(--red)')}
      ${tx(236, 100, t('линия тяги'), { anchor: 'start', color: 'var(--red)' })}
      ${tx(236, 118, t('задних колёс'), { anchor: 'start', color: 'var(--red)', size: 11 })}
      ${tx(84, 140, t('ось'), { anchor: 'end' })}${tx(84, 156, t('симметрии'), { anchor: 'end', size: 11 })}
      ${tx(160, 244, t('машина едет чуть боком, руль стоит криво'), { size: 11 })}`);
  },

  // ── Подготовка ───────────────────────────────────────────────────
  level() {
    return svg(320, 150, t('Площадка: все колёса в одной плоскости'),
      `<line x1="10" y1="118" x2="310" y2="118" stroke="var(--line2)" stroke-width="2"/>
      ${wheelSide(70, 84, 34)}${wheelSide(250, 84, 34)}
      <path d="M44 122 V134 Q44 142 52 142 H268 Q276 142 276 134 V122" fill="none" stroke="var(--blue)" stroke-width="4" opacity=".6"/>
      <rect x="40" y="56" width="8" height="62" rx="3" fill="none" stroke="var(--blue)" stroke-width="2"/>
      <rect x="272" y="56" width="8" height="62" rx="3" fill="none" stroke="var(--blue)" stroke-width="2"/>
      <rect x="41" y="80" width="6" height="37" fill="var(--blue)" opacity=".5"/><rect x="273" y="80" width="6" height="37" fill="var(--blue)" opacity=".5"/>
      ${dash(52, 80, 268, 80, 'var(--blue)')}
      ${tx(160, 18, t('прозрачный шланг с водой — гидроуровень'), { color: 'var(--text)' })}
      ${tx(160, 34, t('вода в концах шланга стоит на одной высоте:'), { size: 11 })}
      ${tx(160, 48, t('от неё меряйте до пола у каждого колеса'), { size: 11 })}
      ${tx(160, 72, t('разница — меньше 2 мм'), { color: 'var(--green)', weight: 600 })}`);
  },

  turnplates() {
    const plate = (x, y) => `<circle cx="${x}" cy="${y}" r="34" fill="var(--panel2)" stroke="var(--line2)" stroke-width="2"/>
      ${[-40, -30, -20, -10, 0, 10, 20, 30, 40].map(d => {
        const a = rad(d), r0 = 34, r2 = d % 20 ? 28 : 24;
        return `<line x1="${r1(x + r0 * Math.sin(a))}" y1="${r1(y - r0 * Math.cos(a))}" x2="${r1(x + r2 * Math.sin(a))}" y2="${r1(y - r2 * Math.cos(a))}" stroke="var(--muted)" stroke-width="1.5"/>`;
      }).join('')}`;
    const slip = (x, y) => `<rect x="${x - 26}" y="${y - 34}" width="52" height="68" rx="4" fill="var(--panel2)" stroke="var(--line2)" stroke-width="2"/>
      <rect x="${x - 21}" y="${y - 29}" width="52" height="68" rx="4" fill="none" stroke="var(--line2)" stroke-dasharray="3 3"/>`;
    return svg(320, 236, t('Поворотные круги и скользящие площадки'),
      `${frontUp(160, 16)}
      ${plate(84, 70)}${plate(236, 70)}${tyre(84, 70, 16, 46, -12)}${tyre(236, 70, 16, 46, -12)}
      ${slip(84, 182)}${slip(236, 182)}${tyre(84, 182, 16, 46)}${tyre(236, 182, 16, 46)}
      ${tx(160, 120, t('спереди — поворотные круги'), { color: 'var(--text)' })}
      ${tx(160, 136, t('со шкалой градусов'), { size: 11 })}
      ${tx(160, 226, t('сзади — скользящие площадки: два листа жести со смазкой'), { size: 11 })}`);
  },

  pedal() {
    return svg(320, 170, t('Упор педали тормоза'),
      `<line x1="10" y1="156" x2="310" y2="156" stroke="var(--line2)" stroke-width="2"/>
      <path d="M200 150 V92 Q200 82 210 82 H258 Q266 82 266 92 V150 Z" fill="var(--car-body2)" stroke="var(--car-line)" stroke-width="2"/>
      <path d="M210 82 L230 22 Q232 14 240 16 L250 18 Q256 20 254 28 L238 82" fill="var(--car-body2)" stroke="var(--car-line)" stroke-width="2"/>
      <path d="M60 60 L84 118" stroke="var(--car-line)" stroke-width="5" stroke-linecap="round"/>
      <rect x="74" y="112" width="26" height="9" rx="3" transform="rotate(20 87 116)" fill="var(--text)"/>
      <line x1="96" y1="116" x2="202" y2="96" stroke="var(--amber)" stroke-width="7" stroke-linecap="round"/>
      ${tx(150, 82, t('упор'), { color: 'var(--amber)', weight: 600 })}
      ${tx(78, 144, t('педаль тормоза'), { size: 11 })}${tx(234, 174 - 30, t('сиденье'), { size: 11 })}
      ${tx(160, 18, t('педаль зажата: колесо не катится, пока крутите руль'), { color: 'var(--text)', size: 12 })}`);
  },

  wheelLock() {
    return svg(320, 170, t('Руль прямо и зафиксирован'),
      `${steeringWheel(160, 74, 52)}
      <rect x="136" y="12" width="48" height="12" rx="6" fill="var(--panel2)" stroke="var(--blue)" stroke-width="2"/>
      <circle cx="160" cy="18" r="4" fill="var(--green)"/>
      <line x1="160" y1="128" x2="160" y2="160" stroke="var(--amber)" stroke-width="7" stroke-linecap="round"/>
      ${tx(126, 22, t('уровень:'), { anchor: 'end', size: 11, color: 'var(--blue)' })}${tx(126, 36, t('пузырёк в центре'), { anchor: 'end', size: 11, color: 'var(--blue)' })}
      ${tx(174, 148, t('упор в сиденье'), { anchor: 'start', size: 11, color: 'var(--amber)' })}${tx(174, 162, t('или ремень'), { anchor: 'start', size: 11, color: 'var(--amber)' })}`);
  },

  settle() {
    return svg(320, 160, t('Осадка подвески'),
      `<line x1="10" y1="140" x2="310" y2="140" stroke="var(--line2)" stroke-width="2"/>
      <path d="M40 112 V92 Q40 80 54 78 L96 74 L128 50 Q136 44 150 44 H214 Q228 44 238 54 L262 76 L284 80 Q296 82 296 96 V112 Z" fill="var(--car-body2)" stroke="var(--car-line)" stroke-width="2"/>
      ${wheelSide(92, 114, 24)}${wheelSide(246, 114, 24)}
      ${arrow(170, 4, 170, 36, 'var(--amber)', 3)}${arrow(184, 36, 184, 6, 'var(--amber)', 3)}
      ${tx(200, 24, t('покачать кузов'), { anchor: 'start', color: 'var(--amber)', size: 12 })}
      ${arrow(110, 154, 40, 154, 'var(--blue)', 3)}${tx(120, 158, t('прокатить 3–5 м вперёд, не назад'), { anchor: 'start', size: 11, color: 'var(--blue)' })}`);
  },

  rideHeight() {
    return svg(320, 170, t('Высота кузова'),
      `<line x1="10" y1="160" x2="310" y2="160" stroke="var(--line2)" stroke-width="2"/>
      <path d="M40 40 H280 V92 H222 Q222 50 160 50 Q98 50 98 92 H40 Z" fill="var(--car-body2)" stroke="var(--car-line)" stroke-width="2"/>
      ${wheelSide(160, 112, 48)}
      <line x1="160" y1="112" x2="160" y2="54" stroke="var(--blue)" stroke-width="3"/>
      <polygon points="160,52 154,63 166,63" fill="var(--blue)"/><circle cx="160" cy="112" r="5" fill="var(--blue)"/>
      ${tx(222, 116, t('от центра колеса'), { anchor: 'start', color: 'var(--blue)', size: 11 })}
      ${tx(222, 130, t('до кромки арки'), { anchor: 'start', color: 'var(--blue)', size: 11 })}
      ${tx(160, 16, t('слева и справа — одинаково;'), { size: 11, color: 'var(--text)' })}
      ${tx(160, 30, t('разница — признак просевшей пружины'), { size: 11, color: 'var(--text)' })}`);
  },

  // Люфты: качать колесо сверху-снизу и по бокам.
  play() {
    const hand = (x, y, dir) => arrow(x, y, x + 22 * dir, y, 'var(--amber)', 3) + arrow(x + 22 * dir, y + 8, x, y + 8, 'var(--amber)', 3);
    return svg(320, 200, t('Проверка люфтов: качать колесо'),
      `${wheelSide(96, 104, 66)}
      ${tx(78, 30, '12', { size: 11, color: 'var(--text)', anchor: 'end' })}${tx(78, 188, '6', { size: 11, color: 'var(--text)', anchor: 'end' })}
      <circle cx="96" cy="40" r="7" fill="var(--amber)"/><circle cx="96" cy="168" r="7" fill="var(--amber)"/>
      <circle cx="32" cy="104" r="7" fill="var(--blue)"/><circle cx="160" cy="104" r="7" fill="var(--blue)"/>
      ${tx(184, 40, t('сверху-снизу (12 и 6 ч):'), { anchor: 'start', color: 'var(--amber)', size: 12, weight: 600 })}
      ${tx(184, 56, t('подшипник ступицы,'), { anchor: 'start', size: 11 })}${tx(184, 70, t('шаровые опоры, шкворни'), { anchor: 'start', size: 11 })}
      ${tx(184, 112, t('по бокам (3 и 9 ч):'), { anchor: 'start', color: 'var(--blue)', size: 12, weight: 600 })}
      ${tx(184, 128, t('рулевые тяги'), { anchor: 'start', size: 11 })}${tx(184, 142, t('и наконечники'), { anchor: 'start', size: 11 })}
      ${tx(184, 176, t('колесо вывешено'), { anchor: 'start', size: 11, color: 'var(--dim)' })}`);
  },

  dryPark() {
    return svg(320, 170, t('Проверка рулевых тяг: качать руль'),
      `${steeringWheel(66, 92, 40)}
      ${arc(66, 92, 54, -45, -8, 'var(--amber)', 3)}${arc(66, 92, 54, 8, 45, 'var(--amber)', 3)}
      ${tx(66, 26, '±45°', { size: 13, color: 'var(--amber)', weight: 600 })}
      ${tx(140, 42, t('помощник качает руль,'), { anchor: 'start', size: 12, color: 'var(--text)' })}
      ${tx(140, 58, t('колёса стоят на полу'), { anchor: 'start', size: 11 })}
      ${tx(140, 90, t('вы смотрите снизу'), { anchor: 'start', size: 12, color: 'var(--text)' })}
      ${tx(140, 106, t('на шарниры тяг: тяга'), { anchor: 'start', size: 11 })}
      ${tx(140, 120, t('двигается, а наконечник'), { anchor: 'start', size: 11 })}
      ${tx(140, 134, t('нет — это люфт'), { anchor: 'start', size: 11 })}`);
  },

  // ── Замер ────────────────────────────────────────────────────────
  bar() {
    return svg(320, 200, t('Планка на закраинах обода'),
      `${wheelSide(110, 100, 86)}
      <rect x="104" y="36" width="12" height="128" rx="3" fill="var(--amber)" opacity=".85"/>
      <rect x="90" y="74" width="40" height="52" rx="6" fill="var(--panel)" stroke="var(--text)" stroke-width="2"/>
      ${ok(132, 41)}${ok(132, 159)}
      ${tx(150, 28, t('касается закраин обода'), { anchor: 'start', color: 'var(--green)', size: 12 })}
      ${tx(206, 98, t('угломер или'), { anchor: 'start', size: 11 })}${tx(206, 112, t('телефон на планке'), { anchor: 'start', size: 11 })}
      ${no(210, 158)}${tx(226, 162, t('не на шину'), { anchor: 'start', color: 'var(--red)', size: 12 })}
      ${tx(160, 196, t('шина мягкая и выпуклая — замер по ней неверен'), { size: 11 })}`);
  },

  strings() {
    const wheel = (x, y) => tyre(x, y, 14, 44);
    return svg(320, 260, t('Струны вдоль бортов'),
      `${frontUp(160, 14)}
      <rect x="112" y="40" width="96" height="190" rx="30" fill="var(--car-body2)" stroke="var(--car-line)" stroke-width="2"/>
      ${wheel(112, 82)}${wheel(208, 82)}${wheel(112, 190)}${wheel(208, 190)}
      <line x1="64" y1="26" x2="64" y2="246" stroke="var(--blue)" stroke-width="2"/>
      <line x1="256" y1="26" x2="256" y2="246" stroke="var(--blue)" stroke-width="2"/>
      ${[[64, 26], [64, 246], [256, 26], [256, 246]].map(([x, y]) => `<rect x="${x - 6}" y="${y - 6}" width="12" height="12" fill="var(--blue)"/>`).join('')}
      ${[[82 - 20, 64], [82 + 20, 64]].map(([y]) => `<line x1="66" y1="${y}" x2="104" y2="${y}" stroke="var(--amber)" stroke-width="2"/>`).join('')}
      ${tx(40, 66, t('A'), { color: 'var(--amber)', weight: 600 })}${tx(40, 106, t('Б'), { color: 'var(--amber)', weight: 600 })}
      ${tx(160, 132, t('струна на высоте'), { size: 11, color: 'var(--text)' })}${tx(160, 146, t('центра колёс'), { size: 11, color: 'var(--text)' })}
      ${tx(160, 258, t('A − Б — схождение этого колеса'), { size: 11 })}`);
  },

  runout() {
    const w = (cx, mark, label) => `${wheelSide(cx, 86, 56)}
      <rect x="${cx - 7}" y="${mark === 'top' ? 26 : 136}" width="14" height="10" rx="2" fill="var(--text)"/>
      ${tx(cx, 162, label, { size: 11, color: 'var(--text)' })}`;
    return svg(320, 172, t('Биение диска: замер и пол-оборота'),
      `${w(80, 'top', t('1. метка сверху — замер'))}${w(240, 'bottom', t('2. метка снизу — замер'))}
      ${arrow(140, 74, 180, 74, 'var(--blue)', 3)}${tx(160, 50, t('прокатить'), { size: 11, color: 'var(--blue)' })}
      ${tx(160, 63, t('½ оборота'), { size: 11, color: 'var(--blue)' })}`);
  },

  // Руль влево — какое колесо куда. dir: 'left' | 'right'.
  steer(dir = 'left') {
    const s = dir === 'left' ? -1 : 1;
    const a = 16 * s;
    return svg(320, 200, dir === 'left' ? t('Руль влево') : t('Руль вправо'),
      `${frontUp(160, 14)}
      ${steeringWheel(160, 66, 30, 40 * s)}
      ${arrow(160 - 26 * s, 26, 160 + 30 * s, 26, 'var(--blue)', 3)}
      <rect x="88" y="132" width="144" height="8" rx="3" fill="var(--car-line)"/>
      ${tyre(80, 136, 18, 60, a)}${tyre(240, 136, 18, 60, a)}
      ${tx(80, 186, dir === 'left' ? t('левое: наружу') : t('левое: внутрь'), { size: 12, color: 'var(--text)', weight: 600 })}
      ${tx(240, 186, dir === 'left' ? t('правое: внутрь') : t('правое: наружу'), { size: 12, color: 'var(--text)', weight: 600 })}
      ${tx(160, 112, dir === 'left' ? t('руль ВЛЕВО') : t('руль ВПРАВО'), { size: 13, color: 'var(--blue)', weight: 700 })}`);
  },

  // ── Регулировка ──────────────────────────────────────────────────
  // Рулевая тяга позади оси колеса: длиннее — схождение растёт; впереди —
  // наоборот. Левое переднее колесо, вид сверху.
  tieRod() {
    const panel = (x0, behind) => {
      const dir = behind ? 1 : -1;      // рычаг назад (вниз на схеме) или вперёд
      const rot = behind ? 9 : -9;      // куда повернётся колесо, если тягу удлинить
      const cx = x0 + 44, cy = 112, armY = cy + 34 * dir;
      const old = `<g opacity=".35">${tyre(cx, cy, 18, 60)}</g>`;
      return `${tx(x0 + 75, 16, behind ? t('тяга позади оси колеса') : t('тяга впереди оси колеса'), { size: 12, color: 'var(--text)', weight: 600 })}
        ${tx(x0 + 75, 31, behind ? t('частая схема: рейка сзади') : t('рейка или тяги спереди'), { size: 11 })}
        ${tx(cx, 58, t('перёд ▲'), { size: 10, color: 'var(--dim)' })}
        ${old}
        <g transform="rotate(${rot} ${cx} ${cy})">${tyre(cx, cy, 18, 60)}
          <line x1="${cx}" y1="${cy}" x2="${cx + 18}" y2="${armY}" stroke="var(--car-line)" stroke-width="5" stroke-linecap="round"/>
          <circle cx="${cx + 18}" cy="${armY}" r="4" fill="var(--text)"/></g>
        <line x1="${cx + 22}" y1="${armY}" x2="${x0 + 146}" y2="${armY}" stroke="var(--amber)" stroke-width="6" stroke-linecap="round"/>
        ${arrow(x0 + 136, armY - 12 * dir, x0 + 96, armY - 12 * dir, 'var(--amber)', 2.5)}
        ${tx(x0 + 116, armY - 22 * dir + (behind ? 0 : 4), t('тяга длиннее'), { size: 11, color: 'var(--amber)' })}
        ${tx(x0 + 75, 182, behind ? t('→ схождение больше (+)') : t('→ схождение меньше (−)'), { size: 12, color: behind ? 'var(--green)' : 'var(--red)', weight: 600 })}`;
    };
    return svg(320, 190, t('Куда крутить рулевую тягу'), panel(4, true) + panel(166, false));
  },

  // ── Шины ─────────────────────────────────────────────────────────
  // Профиль протектора в разрезе; kind: center | edges | inner | outer.
  wear(kind) {
    const W = 140, x0 = 10, y0 = 26;
    const depth = x => {
      const u = (x - x0) / W;   // 0 — внутренний край, 1 — наружный
      switch (kind) {
        case 'center': return 13 * Math.exp(-((u - 0.5) ** 2) / 0.035);
        case 'edges': return 13 * (Math.exp(-(u ** 2) / 0.014) + Math.exp(-((1 - u) ** 2) / 0.014));
        case 'inner': return 16 * Math.exp(-(u ** 2) / 0.06);
        case 'outer': return 16 * Math.exp(-((1 - u) ** 2) / 0.06);
      }
      return 0;
    };
    const pts = [];
    for (let i = 0; i <= 40; i++) { const x = x0 + W * i / 40; pts.push(`${r1(x)},${r1(y0 + depth(x))}`); }
    const grooves = [0.25, 0.5, 0.75].map(u => `<rect x="${r1(x0 + W * u - 3)}" y="${r1(y0 + depth(x0 + W * u))}" width="6" height="10" fill="var(--bg2)"/>`).join('');
    return `<svg class="illus-svg" viewBox="0 0 160 70" aria-hidden="true">
      <path d="M${x0} ${y0} H${x0 + W} L${pts.slice().reverse().join(' L')} Z" fill="var(--amber)" opacity=".55"/>
      <path d="M${x0} ${y0 + 36} L${pts.join(' L')} L${x0 + W} ${y0 + 36} Z" fill="var(--tyre)" stroke="var(--tyre-line)" stroke-width="2"/>
      <path d="M${x0} ${y0} H${x0 + W}" stroke="var(--muted)" stroke-width="1.2" stroke-dasharray="3 3"/>${grooves}
      ${tx(x0, 14, t('внутр.'), { size: 9, anchor: 'start', color: 'var(--dim)' })}${tx(x0 + W, 14, t('наруж.'), { size: 9, anchor: 'end', color: 'var(--dim)' })}</svg>`;
  },

  // «Пила»: острые кромки шашек с одной стороны — схождение.
  feather() {
    const block = x => `<path d="M${x} 50 L${x} 24 L${x + 22} 18 L${x + 26} 50 Z" fill="var(--tyre)" stroke="var(--tyre-line)" stroke-width="2"/>`;
    return `<svg class="illus-svg" viewBox="0 0 160 70" aria-hidden="true">${[8, 46, 84, 122].map(block).join('')}
      ${arrow(60, 62, 104, 62, 'var(--amber)', 2)}</svg>`;
  },

  // Пятна по окружности — амортизаторы, балансировка.
  cupping() {
    let d = '';
    for (let i = 0; i < 360; i += 4) {
      const a = rad(i), r = 28 - 3.5 * Math.max(0, Math.sin(rad(i * 5)));
      d += `${i ? 'L' : 'M'}${r1(80 + r * Math.cos(a))} ${r1(35 + r * Math.sin(a))} `;
    }
    return `<svg class="illus-svg" viewBox="0 0 160 70" aria-hidden="true"><path d="${d}Z" fill="var(--tyre)" stroke="var(--tyre-line)" stroke-width="2"/>
      <circle cx="80" cy="35" r="16" fill="var(--hub)"/></svg>`;
  },

  // Контрольная поездка: за сколько машина уходит на метр.
  roadTest() {
    return svg(320, 170, t('Контрольная поездка'),
      `<rect x="96" y="10" width="130" height="150" fill="var(--panel2)"/>
      <line x1="104" y1="10" x2="104" y2="160" stroke="var(--text)" stroke-width="3"/>
      <line x1="218" y1="10" x2="218" y2="160" stroke="var(--text)" stroke-width="3" stroke-dasharray="14 10"/>
      <rect x="118" y="112" width="30" height="46" rx="8" fill="var(--car-body2)" stroke="var(--car-line)" stroke-width="2"/>
      <path d="M133 110 Q140 66 196 22" fill="none" stroke="var(--amber)" stroke-width="3" stroke-dasharray="6 5"/>
      ${arrow(134, 150 - 6, 202, 150 - 6, 'var(--blue)', 2)}${tx(176, 138, t('≈ 1 м'), { size: 11, color: 'var(--blue)' })}
      ${tx(234, 44, t('дольше 6 с'), { anchor: 'start', color: 'var(--green)', size: 12, weight: 600 })}
      ${tx(234, 60, t('— норма'), { anchor: 'start', size: 11 })}
      ${tx(234, 96, t('6 с и быстрее'), { anchor: 'start', color: 'var(--red)', size: 12, weight: 600 })}
      ${tx(234, 112, t('— уводит'), { anchor: 'start', size: 11 })}
      ${tx(10, 44, t('64 км/ч'), { anchor: 'start', size: 12, color: 'var(--text)' })}${tx(10, 60, t('руль'), { anchor: 'start', size: 11 })}${tx(10, 74, t('отпущен'), { anchor: 'start', size: 11 })}`);
  },
};

// Подпись к рисунку: что на нём и откуда это взято, если не очевидно.
const CAPTION = () => ({
  camber: t('Развал — наклон колеса поперёк машины. Разница развала слева и справа уводит машину в сторону большего плюса.'),
  toe: t('Схождение — поворот колёс в плане. Главный угол для износа шин; на увод машины почти не влияет, но от него зависит, ровно ли стоит руль.'),
  caster: t('Кастер возвращает руль прямо. Машину тянет в сторону колеса с меньшим кастером.'),
  sai: t('SAI задан конструкцией. Разница слева и справа больше 1° — признак погнутой детали: меньший SAI — чаще нижний рычаг, больший — верхняя опора стойки (бюллетень Hyundai 20-SS-001H).'),
  thrust: t('Задняя ось задаёт направление движения. Поэтому сначала регулируют её, затем выравнивают и фиксируют руль, и только потом — перед.'),
  level: t('Ямка 5 мм под одним колесом — это уже около 0,2° ошибки развала.'),
  turnplates: t('Без поворотных кругов кастер не измерить: колесо должно поворачиваться, не сдвигаясь в сторону. Скользящие площадки сзади дают подвеске встать свободно.'),
  pedal: t('Упор педали тормоза нужен для кастера: если колесо прокатится при повороте руля, замер испорчен.'),
  wheelLock: t('Перед регулировкой схождения руль ставят строго прямо и фиксируют — иначе схождение выйдет верным, а руль кривым.'),
  settle: t('После подъёма подвеска «висит». Покачайте кузов и прокатите машину вперёд — она встанет в рабочее положение.'),
  rideHeight: t('Просевшая пружина меняет развал и кастер. Высоту проверяют до регулировки, а не после.'),
  play: t('Колесо вывешено, качайте его руками. Любой ощутимый люфт — сначала ремонт: на изношенных шарнирах углы «плавают».'),
  dryPark: t('Так находят люфт в рулевых тягах и наконечниках, рейке, маятниковом рычаге (методика ASE).'),
  bar: t('Угломер и телефон прикладывают к ровной планке, лежащей на закраинах обода.'),
  strings: t('Расстояния меряют на высоте центра ступицы. Выше или ниже — к схождению подмешивается развал.'),
  runout: t('Два замера в одном месте обода через пол-оборота: их среднее — настоящий развал, разница — биение диска.'),
  steer: t('«Наружу» для левого колеса — руль влево, для правого — вправо.'),
  tieRod: t('Не знаете, где тяга, — поверните на четверть оборота и смотрите на стрелку: экран ответит быстрее любой схемы.'),
  roadTest: t('Методика из бюллетеня Hyundai 20-SS-001H: ровная прямая дорога без ветра, 64 км/ч, руль отпущен. Повторите в обратную сторону.'),
});

// Рисунок с подписью.
export function figure(key, opts = {}) {
  const f = ILLUS[key];
  if (!f) return '';
  const cap = opts.caption === false ? '' : (opts.caption || CAPTION()[key] || '');
  return `<figure class="illus${opts.small ? ' small' : ''}">${f(opts.arg)}${cap ? `<figcaption>${esc(cap)}</figcaption>` : ''}</figure>`;
}

// Износ шин: картинка, как выглядит, о чём говорит. По справочнику MOOG
// «Understanding Tire Wear» (knowyourparts.com) и бюллетеню Hyundai.
export function tyreWearRows() {
  return [
    ['center', t('Середина'), t('Перекачана шина.')],
    ['edges', t('Оба края'), t('Шина недокачана.')],
    ['inner', t('Внутренний край'), t('Отрицательный развал или расхождение; изношенные сайлентблоки, просевшие пружины. Самый частый износ.')],
    ['outer', t('Наружный край'), t('Положительный развал или схождение; бывает и от резкой езды в поворотах.')],
    ['feather', t('«Пила» — острые кромки шашек с одной стороны'), t('Схождение. Острая кромка с внутренней стороны шашек — избыточное схождение, с наружной — расхождение.')],
    ['cupping', t('Пятна по окружности'), t('Изношенные амортизаторы или стойки, люфты в подвеске, дисбаланс колеса.')],
  ];
}

export const wearSvg = k => (k === 'feather' ? ILLUS.feather() : k === 'cupping' ? ILLUS.cupping() : ILLUS.wear(k));

export function tyreWearTable() {
  const rows = tyreWearRows();
  return `<div class="wear-grid">${rows.map(([k, name, why]) => `
    <div class="wear-item">${wearSvg(k)}
      <b>${esc(name)}</b><span>${esc(why)}</span></div>`).join('')}</div>`;
}

// Все рисунки — для проверки и для инструкции.
export const illusKeys = () => Object.keys(ILLUS).filter(k => !['wear', 'feather', 'cupping'].includes(k));

// Сама схема без подписи — для выгрузки в файлы документации
// (docs/tools/illus_export.mjs).
export const svgOf = (key, arg) => (ILLUS[key] ? ILLUS[key](arg) : '');
export const captionOf = key => CAPTION()[key] || '';
