// Форматирование углов и общие справочники интерфейса.
// Вся математика — на стороне Go; здесь только показ.

export const WHEELS = [
  { key: 'FL', name: 'Переднее левое', short: 'ПЛ', front: true, left: true },
  { key: 'FR', name: 'Переднее правое', short: 'ПП', front: true, left: false },
  { key: 'RL', name: 'Заднее левое', short: 'ЗЛ', front: false, left: true },
  { key: 'RR', name: 'Заднее правое', short: 'ЗП', front: false, left: false },
];

// Короткие подписи параметров для табло. Полные подписи приходят с сервера.
export const PARAM = {
  camber: 'Развал',
  toe: 'Схождение',
  caster: 'Кастер',
  sai: 'Попер. наклон оси',
  front_total_toe: 'Схождение Σ',
  rear_total_toe: 'Схождение Σ',
  front_cross_camber: 'Развал Δ',
  rear_cross_camber: 'Развал Δ',
  front_cross_caster: 'Кастер Δ',
  thrust_angle: 'Угол тяги',
};

export function paramKind(key) {
  const i = key.lastIndexOf('_');
  const tail = key.slice(i + 1);
  if (['FL', 'FR', 'RL', 'RR'].includes(tail)) return key.slice(0, i);
  return key;
}
export function paramWheel(key) {
  const tail = key.slice(key.lastIndexOf('_') + 1);
  return WHEELS.find(w => w.key === tail) || null;
}
export const isToeKey = key => key.startsWith('toe_') || key.endsWith('_total_toe');

export function shortLabel(key) {
  return PARAM[paramKind(key)] || key;
}

// Угол в виде +0°23′ — так печатают все сервисные мануалы.
export function fmtDM(deg, { sign = true } = {}) {
  if (deg === null || deg === undefined || Number.isNaN(deg)) return '—';
  const neg = deg < 0;
  const a = Math.abs(deg);
  let d = Math.floor(a);
  let m = Math.round((a - d) * 60);
  if (m === 60) { d++; m = 0; }
  const s = sign ? (neg && (d || m) ? '−' : '+') : (neg ? '−' : '');
  return `${s}${d}°${String(m).padStart(2, '0')}′`;
}

export function fmtDeg(deg, digits = 2) {
  if (deg === null || deg === undefined || Number.isNaN(deg)) return '—';
  const v = deg.toFixed(digits);
  if (Number(v) === 0) return `0.${'0'.repeat(digits)}°`;
  return (deg > 0 ? '+' : '−') + Math.abs(deg).toFixed(digits) + '°';
}

export function fmtMM(mm, digits = 1) {
  if (mm === null || mm === undefined || Number.isNaN(mm)) return '—';
  const v = Math.abs(mm).toFixed(digits);
  if (Number(v) === 0) return `0.${'0'.repeat(digits)} мм`;
  return (mm > 0 ? '+' : '−') + v + ' мм';
}

// Значение параметра в выбранных единицах. Миллиметры — только для
// схождения и только если сервер знает диаметр обода.
export function fmtParam(key, p, units) {
  if (!p || !p.has) return '— — —';
  if (units === 'mm' && isToeKey(key) && p.mm !== undefined && p.mm !== null) return fmtMM(p.mm);
  if (units === 'deg') return fmtDeg(p.v);
  return fmtDM(p.v);
}

export function fmtRangeParam(key, spec, units, rimMM) {
  if (!spec) return '';
  if (units === 'mm' && isToeKey(key) && rimMM > 0) {
    const mm = d => rimMM * Math.tan(d * Math.PI / 180);
    return `${fmtMM(mm(spec.min))} … ${fmtMM(mm(spec.max))}`;
  }
  const f = units === 'deg' ? fmtDeg : fmtDM;
  return `${f(spec.min)} … ${f(spec.max)}`;
}

export function specValue(key, deg, units, rimMM) {
  if (units === 'mm' && isToeKey(key) && rimMM > 0) return fmtMM(rimMM * Math.tan(deg * Math.PI / 180));
  return units === 'deg' ? fmtDeg(deg) : fmtDM(deg);
}

export const STATUS_RU = {
  good: 'в допуске',
  marginal: 'у границы допуска',
  bad: 'вне допуска',
  no_spec: 'допуск не задан',
};

export function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

export const inches = d => d * 25.4;
