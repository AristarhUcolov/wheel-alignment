// Настройки интерфейса: тема, размер текста, палитра, единицы, звук и голос,
// данные мастерской для протокола.
//
// Хранятся в файле настроек программы (/api/settings), а не в хранилище
// браузера: то привязано к порту, на котором открылся интерфейс, и теряется,
// как только порт сменится.

import { api, emit } from './state.js';

const DEFAULTS = {
  theme: 'dark',        // dark — как стенд; light — для яркого света; contrast — высокий контраст
  size: 'normal',       // normal | large | xlarge — масштаб всего интерфейса
  palette: 'standard',  // standard — зелёный/красный; cb — синий/оранжевый, различим при дальтонизме
  motion: 'auto',       // auto — как в системе; reduce — без анимации и мигания
  units: 'dm',          // dm — ° ′; deg — 0.00°; mm — схождение в мм
  sound: false,         // звуковые сигналы при регулировке
  voice: false,         // озвучивать выбранный угол
  voiceName: '',        // голос системы; пусто — первый подходящий по языку
  voiceRate: 1,
  workshop: { name: '', contacts: '', who: '' },
  welcomed: false,      // приветствие уже показано
  recent: [],           // последние выбранные автомобили
};

export const ZOOM = { normal: 1, large: 1.15, xlarge: 1.3 };

export const prefs = structuredClone(DEFAULTS);

function merge(src) {
  if (!src || typeof src !== 'object') return;
  for (const k of Object.keys(DEFAULTS)) {
    if (src[k] === undefined) continue;
    if (k === 'workshop') prefs.workshop = { ...DEFAULTS.workshop, ...(src.workshop || {}) };
    else prefs[k] = src[k];
  }
}

// Старые версии хранили единицы в браузере — переносим один раз.
function legacy() {
  try {
    const u = localStorage.getItem('wa.units');
    return u ? { units: JSON.parse(u) } : null;
  } catch { return null; }
}

export async function loadPrefs() {
  let fromServer = null;
  try {
    const r = await api('/api/settings');
    fromServer = r && r.ui && Object.keys(r.ui).length ? r.ui : null;
  } catch { /* без настроек — значения по умолчанию */ }
  merge(fromServer || legacy());
  applyPrefs();
  return prefs;
}

let timer = null;
function persist() {
  clearTimeout(timer);
  timer = setTimeout(() => {
    api('/api/settings', { method: 'POST', body: { ui: prefs } }).catch(() => {});
  }, 300);
}

// Записать немедленно — перед перезагрузкой страницы (смена языка).
export async function flushPrefs() {
  clearTimeout(timer);
  try { await api('/api/settings', { method: 'POST', body: { ui: prefs } }); } catch { /* без сохранения */ }
}

export function setPref(key, value) {
  prefs[key] = value;
  applyPrefs();
  persist();
  emit('prefs', key);
}

export function setWorkshop(field, value) {
  prefs.workshop = { ...prefs.workshop, [field]: value };
  persist();
}

export function rememberVehicle(id) {
  if (!id) return;
  prefs.recent = [id, ...prefs.recent.filter(x => x !== id)].slice(0, 6);
  persist();
}

export function resetPrefs() {
  const keep = { welcomed: true, recent: prefs.recent, workshop: prefs.workshop };
  Object.assign(prefs, structuredClone(DEFAULTS), keep);
  applyPrefs();
  persist();
  emit('prefs', '*');
}

export const zoom = () => ZOOM[prefs.size] || 1;

// Применить к странице: тема и палитра — атрибутами, масштаб — через zoom
// (вместе с размерами окна меняются и раскладки для узкого и широкого экрана).
export function applyPrefs() {
  const html = document.documentElement;
  html.dataset.theme = prefs.theme;
  html.dataset.palette = prefs.palette;
  html.style.setProperty('--zoom', String(zoom()));
  html.classList.toggle('reduce-motion', prefs.motion === 'reduce');
  layoutClasses();
}

function layoutClasses() {
  const html = document.documentElement;
  const w = window.innerWidth / zoom();
  html.classList.toggle('narrow', w < 1150);
  html.classList.toggle('wide', w >= 1300);
}
window.addEventListener('resize', layoutClasses);

export const reducedMotion = () => prefs.motion === 'reduce' ||
  (prefs.motion === 'auto' && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
