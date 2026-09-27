// Сход-развал — открытый стенд. Каркас интерфейса: шаги, верхняя строка,
// строка функциональных клавиш, боковая панель.

import { state, on, emit, api, refreshSession, connectStream, toast, $, $$, h } from './state.js';
import { esc } from './fmt.js';
import * as vehicle from './screens/vehicle.js';
import * as prep from './screens/prep.js';
import * as measure from './screens/measure.js';
import * as live from './screens/live.js';
import * as report from './screens/report.js';
import * as guide from './screens/guide.js';
import * as contrib from './screens/contrib.js';

const SCREENS = { vehicle, prep, measure, live, report, guide, contrib };
const mounted = new Set();

export function go(name, arg) {
  if (!SCREENS[name]) return;
  if (state.screen === 'live' && name !== 'live') closeDrawer();
  state.screen = name;
  const el = document.getElementById('screen-' + name);
  if (!mounted.has(name)) {
    SCREENS[name].mount(el);
    mounted.add(name);
  }
  $$('.screen').forEach(s => s.classList.toggle('active', s === el));
  $$('nav.side button').forEach(b => b.classList.toggle('active', b.dataset.screen === name));
  if (SCREENS[name].show) SCREENS[name].show(arg);
  if (name !== 'live') $('#main').scrollTop = 0;
  renderFkeys();
}

$$('nav.side button').forEach(b => b.onclick = () => go(b.dataset.screen));

// ── Боковая панель ──────────────────────────────────────────────────

let drawerFor = null;
export function openDrawer(html, key = null, refresh = false) {
  const d = $('#drawer');
  const body = $('#drawerBody');
  const scroll = body.scrollTop;
  body.innerHTML = html;
  if (refresh) body.scrollTop = scroll;
  drawerFor = key;
  d.classList.add('open');
  d.setAttribute('aria-hidden', 'false');
  document.querySelector('.app').classList.add('drawer-open');
}
export function closeDrawer() {
  drawerFor = null;
  document.querySelector('.app').classList.remove('drawer-open');
  $('#drawer').classList.remove('open');
  $('#drawer').setAttribute('aria-hidden', 'true');
}
export const drawerKey = () => drawerFor;
$('[data-close-drawer]').onclick = () => { live.closeParam(); };

// ── Верхняя строка ──────────────────────────────────────────────────

function renderTop() {
  const ss = state.session;
  const el = $('#topVehicle');
  if (!ss || !ss.vehicle) {
    el.innerHTML = '<span class="muted">Автомобиль не выбран</span>';
  } else {
    const v = ss.vehicle;
    const lim = ss.limits;
    const limTxt = !lim ? 'допусков нет' : lim.id === v.id ? esc(lim.source_label) : 'допуски: ориентир по классу';
    const title = v.source_kind === 'class_guidance' ? 'Ориентир по классу: ' + v.model : v.title;
    el.innerHTML = `
      <span class="t">${esc(title)}</span>
      <span class="badge ${esc(lim ? lim.source_kind : 'catalog')}">${limTxt}</span>
      <span class="s">перед: ${esc(ss.front_suspension.name)} · зад: ${esc(ss.rear_suspension.name)} · обод ${ss.rim_diameter_in}″</span>`;
  }
}

function renderSources() {
  const f = state.frame;
  const el = $('#topSources');
  if (!state.connected) {
    el.innerHTML = '<span class="src-pill off">нет связи</span>';
    return;
  }
  const src = (f && f.sources) || [];
  if (!src.length) { el.innerHTML = '<span class="src-pill">датчиков нет</span>'; return; }
  el.innerHTML = src.map(s => `<span class="src-pill ${s.online ? 'on' : 'off'}" title="${esc(s.detail || '')}">${esc(s.name)}${s.wheel ? ' · ' + esc(s.wheel) : ''}</span>`).join('');
}

function tickClock() {
  const d = new Date();
  $('#clock').textContent = d.toLocaleTimeString('ru-RU');
}
setInterval(tickClock, 1000);
tickClock();

// ── Функциональные клавиши ──────────────────────────────────────────

function fkeysFor(screen) {
  const common = [{ k: 'F1', label: 'Инструкция', run: () => go('guide') }];
  const ss = state.session || {};
  if (screen === 'live') {
    const view = state.liveView;
    return [
      ...common,
      { k: 'F2', label: 'Сменить вид', run: () => live.cycleView() },
      { k: 'F3', label: 'Передняя ось', run: () => live.setView('front'), on: view === 'front' },
      { k: 'F4', label: 'Задняя ось', run: () => live.setView('rear'), on: view === 'rear' },
      { k: 'F5', label: ss.has_before ? 'Снимок «после»' : 'Снимок «до»', run: () => snapshot(ss.has_before ? 'after' : 'before') },
      { k: 'F6', label: 'Отчёт', run: () => go('report') },
      { k: 'F9', label: ss.sim_running ? 'Остановить демо' : 'Демонстрация', run: toggleSim, on: ss.sim_running },
      ...(ss.sim_running ? [{ k: 'F10', label: ss.sim_auto ? 'Стоп авторегулировки' : 'Показать регулировку', run: toggleAuto, on: ss.sim_auto }] : []),
    ];
  }
  const next = { vehicle: 'prep', prep: 'measure', measure: 'live', report: null }[screen];
  const names = { prep: 'Подготовка', measure: 'Замер', live: 'Регулировка' };
  return [
    ...common,
    { k: 'F2', label: 'Автомобиль', run: () => go('vehicle'), on: screen === 'vehicle' },
    { k: 'F3', label: 'Подготовка', run: () => go('prep'), on: screen === 'prep' },
    { k: 'F4', label: 'Замер', run: () => go('measure'), on: screen === 'measure' },
    { k: 'F5', label: 'Регулировка', run: () => go('live') },
    { k: 'F6', label: 'Отчёт', run: () => go('report'), on: screen === 'report' },
    ...(next ? [{ k: 'F12', label: 'Далее: ' + names[next], run: () => go(next) }] : []),
  ];
}

let currentFkeys = [];
export function renderFkeys() {
  currentFkeys = fkeysFor(state.screen);
  const bar = $('#fkeys');
  bar.innerHTML = currentFkeys.map((f, i) =>
    `<button class="fkey${f.on ? ' on' : ''}" data-i="${i}"><kbd>${f.k}</kbd><span>${esc(f.label)}</span></button>`).join('');
  $$('.fkey', bar).forEach(b => b.onclick = () => currentFkeys[+b.dataset.i].run());
}

document.addEventListener('keydown', e => {
  if (/^F([1-9]|1[0-2])$/.test(e.key)) {
    e.preventDefault(); // F5 не должен перезагружать окно, F3 — открывать поиск
    const f = currentFkeys.find(x => x.k === e.key);
    if (f) f.run();
    return;
  }
  if (e.key === 'Escape') { live.closeParam(); return; }
  // Демонстрация: стрелками «крутим» выбранный угол.
  if (state.screen === 'live' && state.selected && state.session && state.session.sim_running &&
      !['INPUT', 'SELECT', 'TEXTAREA'].includes(document.activeElement.tagName)) {
    const step = e.shiftKey ? 0.25 : 0.05;
    if (e.key === 'ArrowRight' || e.key === 'ArrowUp') { e.preventDefault(); live.simAdjust(state.selected, step); }
    if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') { e.preventDefault(); live.simAdjust(state.selected, -step); }
  }
});

async function toggleSim() {
  const running = state.session && state.session.sim_running;
  try {
    await api('/api/sim', { method: 'POST', body: { action: running ? 'stop' : 'start' } });
    await refreshSession();
    if (!running) {
      toast('Демонстрация запущена. Нажмите на любой угол, чтобы узнать, как его регулировать.');
      if (state.screen !== 'live') go('live');
    }
  } catch (e) { toast(e.message, true); }
}

async function toggleAuto() {
  const auto = state.session && state.session.sim_auto;
  try {
    if (!auto && !(state.session && state.session.limits)) {
      toast('Для демонстрации регулировки выберите автомобиль или ориентир по классу — нужно знать, куда крутить.', true);
      return;
    }
    await api('/api/sim', { method: 'POST', body: { action: auto ? 'auto_off' : 'auto_on' } });
    await refreshSession();
    if (!auto) toast('Смотрите: сначала задняя ось, потом кастер, развал и в конце схождение.');
  } catch (e) { toast(e.message, true); }
}

export async function snapshot(label) {
  try {
    await api('/api/live/snapshot', { method: 'POST', body: { label } });
    await refreshSession();
    toast(label === 'before' ? 'Снимок «до» сохранён. Регулируйте — затем F5 для снимка «после».' : 'Снимок «после» сохранён — отчёт готов (F6).');
  } catch (e) {
    toast(e.status === 409 ? 'Для снимка нужны развал и схождение всех четырёх колёс.' : e.message, true);
  }
}

// Автоматическое обновление признака «авторегулировка закончилась».
setInterval(() => {
  if (state.session && state.session.sim_auto) refreshSession().catch(() => {});
}, 2000);

// ── Запуск ──────────────────────────────────────────────────────────

on('session', () => { renderTop(); renderFkeys(); });
on('frame', renderSources);
on('connection', () => { renderSources(); });
on('fkeys', renderFkeys);

(async function start() {
  try {
    await refreshSession();
  } catch (e) {
    toast('Не удалось связаться с программой: ' + e.message, true);
  }
  connectStream();
  const want = location.hash.replace('#', '');
  go(SCREENS[want] ? want : (state.session && state.session.vehicle ? 'live' : 'vehicle'));
})();

window.addEventListener('hashchange', () => {
  const want = location.hash.replace('#', '');
  if (SCREENS[want]) go(want);
});

export { emit };
