// Общее состояние интерфейса и связь с программой.

import { t } from './i18n.js';

const listeners = new Map();

export const state = {
  session: null,     // /api/session — машина на стенде и всё, что для неё введено
  frame: null,       // последний кадр живых показаний
  screen: 'vehicle',
  units: 'dm',       // 'dm' — градусы и минуты, 'deg' — градусы, 'mm' — схождение в мм
  liveView: 'overview',
  selected: null,    // ключ параметра, открытого в боковой панели
  connected: false,
};

export function on(event, fn) {
  if (!listeners.has(event)) listeners.set(event, new Set());
  listeners.get(event).add(fn);
  return () => listeners.get(event).delete(fn);
}
export function emit(event, data) {
  for (const fn of listeners.get(event) || []) {
    try { fn(data); } catch (e) { console.error(event, e); }
  }
}

// Части работы, которые читает только интерфейс: отметки подготовки, струны,
// шапка протокола. Хранит их программа — в файле работы, а не в браузере:
// закрыли программу, открыли завтра — всё на месте. Запись отложенная и
// по частям: печатать в поле можно сколько угодно быстро.
const jobPending = {};
let jobTimer = null;
export function saveJob(patch) {
  Object.assign(jobPending, patch);
  if (state.session) {
    if (patch.checks) state.session.checks = patch.checks;
    if (patch.box) state.session.box = patch.box;
    if (patch.report) state.session.report_meta = patch.report;
    if (patch.notes) state.session.notes = patch.notes;
  }
  clearTimeout(jobTimer);
  jobTimer = setTimeout(flushJob, 400);
}
export async function flushJob() {
  clearTimeout(jobTimer);
  const body = { ...jobPending };
  for (const k of Object.keys(jobPending)) delete jobPending[k];
  if (!Object.keys(body).length) return;
  try { await api('/api/job', { method: 'POST', body }); } catch (e) { toast(e.message, true); }
}
// Перед закрытием окна — дописать то, что ещё не ушло.
window.addEventListener('beforeunload', () => {
  if (!Object.keys(jobPending).length) return;
  try {
    navigator.sendBeacon && navigator.sendBeacon('/api/job', new Blob([JSON.stringify(jobPending)], { type: 'application/json' }));
  } catch { /* не успели — не страшно */ }
});

// Новая машина: всё, что было введено и измерено для прежней, уходит.
// Калибровки телефонов и камер остаются — они принадлежат оборудованию.
export async function newJob() {
  for (const k of Object.keys(jobPending)) delete jobPending[k];
  clearTimeout(jobTimer);
  state.session = await api('/api/job/new', { method: 'POST', body: {} });
  state.selected = null;
  emit('job');
  emit('session', state.session);
  return state.session;
}

// ── HTTP ─────────────────────────────────────────────────────────────

export async function api(path, { method = 'GET', body, form } = {}) {
  const opt = { method, headers: {} };
  if (form) {
    opt.body = form;
  } else if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  let r;
  try {
    r = await fetch(path, opt);
  } catch (e) {
    throw new Error(t('Нет связи с программой: {msg}', { msg: e.message }));
  }
  let data = null;
  const text = await r.text();
  try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
  if (!r.ok) {
    const err = new Error((data && data.error) || t('Ошибка {code}', { code: r.status }));
    err.data = data;
    err.status = r.status;
    throw err;
  }
  return data;
}

export async function refreshSession() {
  state.session = await api('/api/session');
  emit('session', state.session);
  return state.session;
}

export async function updateSession(patch) {
  state.session = await api('/api/session', { method: 'POST', body: patch });
  emit('session', state.session);
  return state.session;
}

// ── Поток живых кадров ───────────────────────────────────────────────

let es = null;
export function connectStream() {
  if (es) es.close();
  es = new EventSource('/api/live/stream');
  es.onmessage = ev => {
    try {
      state.frame = JSON.parse(ev.data);
    } catch { return; }
    if (!state.connected) { state.connected = true; emit('connection', true); }
    emit('frame', state.frame);
  };
  es.onerror = () => {
    if (state.connected) { state.connected = false; emit('connection', false); }
    // EventSource переподключается сам; если сервер перезапущен —
    // заодно перечитаем сессию.
    setTimeout(() => { if (!state.connected) refreshSession().catch(() => {}); }, 1500);
  };
}

// ── Уведомления ──────────────────────────────────────────────────────

let toastTimer;
export function toast(msg, bad = false) {
  const t = document.getElementById('toast');
  // Ошибку экранный диктор зачитывает сразу, обычное сообщение — в паузе.
  t.setAttribute('role', bad ? 'alert' : 'status');
  t.setAttribute('aria-live', bad ? 'assertive' : 'polite');
  t.textContent = msg;
  t.classList.toggle('bad', bad);
  t.classList.add('show');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.remove('show'), bad ? 5000 : 2600);
}

export const $ = (s, r = document) => r.querySelector(s);
export const $$ = (s, r = document) => [...r.querySelectorAll(s)];

export function h(html) {
  const t = document.createElement('template');
  t.innerHTML = html.trim();
  return t.content.firstElementChild;
}
