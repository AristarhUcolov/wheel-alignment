// Сход-развал — открытый стенд. Каркас интерфейса: шаги, верхняя строка,
// строка функциональных клавиш, боковая панель, язык, настройки.

import { state, on, emit, api, refreshSession, connectStream, toast, $, $$ } from './state.js';
import { t, setLang, translateStatic, lang } from './i18n.js';
import { esc, parseNum } from './fmt.js';
import { loadPrefs, prefs, flushPrefs } from './prefs.js';
import * as vehicle from './screens/vehicle.js';
import * as prep from './screens/prep.js';
import * as measure from './screens/measure.js';
import * as live from './screens/live.js';
import * as report from './screens/report.js';
import * as guide from './screens/guide.js';
import * as contrib from './screens/contrib.js';
import * as settings from './screens/settings.js';
import * as support from './screens/support.js';
import { maybeWelcome } from './screens/welcome.js';

const SCREENS = { vehicle, prep, measure, live, report, guide, contrib, settings, support };
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
  $$('nav.side button').forEach(b => {
    const on = b.dataset.screen === name;
    b.classList.toggle('active', on);
    if (on) b.setAttribute('aria-current', 'page'); else b.removeAttribute('aria-current');
  });
  if (SCREENS[name].show) SCREENS[name].show(arg);
  if (name !== 'live') $('#main').scrollTop = 0;
  // Фокус остался на элементе скрытого экрана — переносим в начало нового,
  // иначе клавиатура «теряется».
  const a = document.activeElement;
  if (a && a !== document.body && !a.closest('nav.side, header.top, footer.fkeys, .drawer, .modal-back') && !el.contains(a)) {
    $('#main').focus({ preventScroll: true });
  }
  renderFkeys();
  renderSteps();
}

$$('nav.side button').forEach(b => b.onclick = () => go(b.dataset.screen));

// Поле числа, в котором не число, — сразу красная рамка, а не молча
// пропавший замер.
document.addEventListener('input', e => {
  const el = e.target;
  if (!el.matches || !el.matches('input[data-num]')) return;
  const bad = Number.isNaN(parseNum(el.value));
  el.toggleAttribute('aria-invalid', bad);
  el.title = bad ? t('Не понял число — например: -0,5 или 0.25') : '';
});
$('#topSettings').onclick = () => go('settings');

// ── Ссылки наружу ───────────────────────────────────────────────────

// Внешние ссылки открываются в обычном браузере человека, а не в окне
// программы: там адресная строка, сохранённые входы и привычный браузер.
export function openExternal(url) {
  api('/api/open', { method: 'POST', body: { url } }).catch(() => window.open(url, '_blank', 'noopener'));
}
document.addEventListener('click', e => {
  const a = e.target.closest('a[href^="https://"], a[href^="http://"]');
  if (!a || a.host === location.host) return;
  e.preventDefault();
  openExternal(a.href);
});

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
    el.innerHTML = `<span class="muted">${t('Автомобиль не выбран')}</span>`;
  } else {
    const v = ss.vehicle;
    const lim = ss.limits;
    const limTxt = !lim ? t('допусков нет') : lim.id === v.id ? esc(lim.source_label) : t('допуски: ориентир по классу');
    const title = v.source_kind === 'class_guidance' ? t('Ориентир по классу: {model}', { model: v.model }) : v.title;
    el.innerHTML = `
      <span class="t">${esc(title)}</span>
      <span class="badge ${esc(lim ? lim.source_kind : 'catalog')}">${limTxt}</span>
      <span class="s">${esc(t('перед: {front} · зад: {rear} · обод {rim}″', { front: ss.front_suspension.name, rear: ss.rear_suspension.name, rim: ss.rim_diameter_in }))}</span>`;
    el.title = title;
  }
}

function renderSources() {
  const f = state.frame;
  const el = $('#topSources');
  if (!state.connected) {
    el.innerHTML = `<span class="src-pill off">${t('нет связи')}</span>`;
    return;
  }
  const src = (f && f.sources) || [];
  const html = !src.length
    ? `<span class="src-pill">${t('датчиков нет')}</span>`
    : src.map(s => `<span class="src-pill ${s.online ? 'on' : 'off'}" title="${esc(s.detail || '')}">${esc(s.name)}${s.wheel ? ' · ' + esc(s.wheel) : ''}</span>`).join('');
  if (el.innerHTML !== html) el.innerHTML = html;
}

function tickClock() {
  const d = new Date();
  $('#clock').textContent = d.toLocaleTimeString(lang === 'en' ? 'en-GB' : 'ru-RU');
}
setInterval(tickClock, 1000);

// Пройденные шаги отмечаются в списке слева — видно, что сделано и что дальше.
function renderSteps() {
  const ss = state.session || {};
  const f = state.frame;
  const done = {
    vehicle: !!ss.vehicle,
    measure: !!(f && f.measured > 0),
    live: !!(f && f.measured > 0 && f.has_limits && f.overall === 'good'),
    report: !!ss.has_after,
  };
  $$('nav.side .side-steps button').forEach(b => {
    const d = !!done[b.dataset.screen];
    b.classList.toggle('done', d && !b.classList.contains('active'));
  });
}

// В узком окне от кнопок шагов остаются номера — подписи уходят в подсказки.
function navTitles() {
  $$('nav.side button').forEach(b => {
    const span = b.querySelector('span');
    if (span) b.title = span.textContent;
  });
}

// ── Язык ────────────────────────────────────────────────────────────

export async function switchLang(l) {
  if (l === lang) return;
  try {
    await flushPrefs();
    await api('/api/lang', { method: 'POST', body: { lang: l } });
    // Перезагрузка — самый надёжный способ перевести всё сразу, включая
    // то, что уже пришло с сервера.
    location.reload();
  } catch (e) { toast(e.message, true); }
}

function renderLangs() {
  $$('#langs button').forEach(b => {
    b.classList.toggle('on', b.dataset.lang === lang);
    b.setAttribute('aria-pressed', b.dataset.lang === lang ? 'true' : 'false');
    b.onclick = () => switchLang(b.dataset.lang);
  });
}

// ── Функциональные клавиши ──────────────────────────────────────────

function fkeysFor(screen) {
  const common = [{ k: 'F1', label: t('Инструкция'), run: () => go('guide') }];
  const ss = state.session || {};
  if (screen === 'live') {
    const view = state.liveView;
    return [
      ...common,
      { k: 'F2', label: t('Сменить вид'), run: () => live.cycleView() },
      { k: 'F3', label: t('Передняя ось'), run: () => live.setView('front'), on: view === 'front' },
      { k: 'F4', label: t('Задняя ось'), run: () => live.setView('rear'), on: view === 'rear' },
      { k: 'F5', label: ss.has_before ? t('Снимок «после»') : t('Снимок «до»'), run: () => snapshot(ss.has_before ? 'after' : 'before') },
      { k: 'F6', label: t('Отчёт'), run: () => go('report') },
      { k: 'F7', label: t('Звук'), run: () => live.toggleSound(), on: prefs.sound },
      { k: 'F8', label: t('Голос'), run: () => live.toggleVoice(), on: prefs.voice },
      { k: 'F9', label: ss.sim_running ? t('Стоп демо') : t('Демо'), run: toggleSim, on: ss.sim_running },
      ...(ss.sim_running ? [{ k: 'F10', label: ss.sim_auto ? t('Стоп авторегулировки') : t('Авторегулировка'), run: toggleAuto, on: ss.sim_auto }] : []),
    ];
  }
  const next = { vehicle: 'prep', prep: 'measure', measure: 'live', report: null }[screen];
  const names = { prep: t('Подготовка'), measure: t('Замер'), live: t('Регулировка') };
  return [
    ...common,
    { k: 'F2', label: t('Автомобиль'), run: () => go('vehicle'), on: screen === 'vehicle' },
    { k: 'F3', label: t('Подготовка'), run: () => go('prep'), on: screen === 'prep' },
    { k: 'F4', label: t('Замер'), run: () => go('measure'), on: screen === 'measure' },
    { k: 'F5', label: t('Регулировка'), run: () => go('live') },
    { k: 'F6', label: t('Отчёт'), run: () => go('report'), on: screen === 'report' },
    ...(next ? [{ k: 'F12', label: t('Далее: {step}', { step: names[next] }), run: () => go(next) }] : []),
  ];
}

let currentFkeys = [];
export function renderFkeys() {
  currentFkeys = fkeysFor(state.screen);
  const bar = $('#fkeys');
  bar.classList.toggle('many', currentFkeys.length > 8);
  bar.innerHTML = currentFkeys.map((f, i) =>
    `<button class="fkey${f.on ? ' on' : ''}" data-i="${i}" title="${esc(f.k + ' — ' + f.label)}"${f.on !== undefined ? ` aria-pressed="${f.on ? 'true' : 'false'}"` : ''}><kbd>${f.k}</kbd><span>${esc(f.label)}</span></button>`).join('');
  $$('.fkey', bar).forEach(b => b.onclick = () => currentFkeys[+b.dataset.i].run());
}

document.addEventListener('keydown', e => {
  if (document.querySelector('.modal-back')) return; // приветствие ведёт себя само
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

export async function toggleSim() {
  const running = state.session && state.session.sim_running;
  try {
    await api('/api/sim', { method: 'POST', body: { action: running ? 'stop' : 'start' } });
    await refreshSession();
    if (!running) {
      toast(t('Демонстрация запущена. Нажмите на любой угол, чтобы узнать, как его регулировать.'));
      if (state.screen !== 'live') go('live');
    }
  } catch (e) { toast(e.message, true); }
}

async function toggleAuto() {
  const auto = state.session && state.session.sim_auto;
  try {
    if (!auto && !(state.session && state.session.limits)) {
      toast(t('Для демонстрации регулировки выберите автомобиль или ориентир по классу — нужно знать, куда крутить.'), true);
      return;
    }
    await api('/api/sim', { method: 'POST', body: { action: auto ? 'auto_off' : 'auto_on' } });
    await refreshSession();
    if (!auto) toast(t('Смотрите: сначала задняя ось, потом кастер, развал и в конце схождение.'));
  } catch (e) { toast(e.message, true); }
}

export async function snapshot(label) {
  try {
    await api('/api/live/snapshot', { method: 'POST', body: { label } });
    await refreshSession();
    toast(label === 'before'
      ? t('Снимок «до» сохранён. Регулируйте — затем F5 для снимка «после».')
      : t('Снимок «после» сохранён — отчёт готов (F6).'));
  } catch (e) {
    toast(e.status === 409 ? t('Для снимка нужны развал и схождение всех четырёх колёс.') : e.message, true);
  }
}

// Автоматическое обновление признака «авторегулировка закончилась».
setInterval(() => {
  if (state.session && state.session.sim_auto) refreshSession().catch(() => {});
}, 2000);

// ── Запуск ──────────────────────────────────────────────────────────

on('session', () => { renderTop(); renderFkeys(); renderSteps(); });
on('frame', () => { renderSources(); renderSteps(); });
on('connection', () => { renderSources(); });
on('fkeys', renderFkeys);
on('prefs', () => renderFkeys());

(async function start() {
  // Настройки — первыми: тема и масштаб применяются до первой отрисовки.
  await loadPrefs();
  state.units = prefs.units;
  try {
    await refreshSession();
  } catch (e) {
    toast(e.message, true);
  }
  // Язык — до того, как смонтирован хоть один экран.
  setLang(state.session ? state.session.lang : 'ru');
  document.title = t('Сход-развал — открытый стенд');
  translateStatic();
  navTitles();
  renderLangs();
  renderTop();
  tickClock();
  connectStream();
  const want = location.hash.replace('#', '');
  go(SCREENS[want] ? want : (state.session && state.session.vehicle ? 'live' : 'vehicle'));
  maybeWelcome();
})();

window.addEventListener('hashchange', () => {
  const want = location.hash.replace('#', '');
  if (SCREENS[want]) go(want);
});

export { emit };
