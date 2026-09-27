// Общее состояние интерфейса и связь с программой.

const listeners = new Map();

export const state = {
  session: null,     // /api/session
  frame: null,       // последний кадр живых показаний
  screen: 'vehicle',
  units: load('units', 'dm'),   // 'dm' — градусы и минуты, 'deg' — градусы, 'mm' — схождение в мм
  liveView: load('liveView', 'overview'),
  selected: null,    // ключ параметра, открытого в боковой панели
  checks: load('checks', {}),
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

// localStorage может быть недоступен (режим без сохранения данных) — тогда
// просто работаем без запоминания настроек.
function load(key, def) {
  try {
    const v = localStorage.getItem('wa.' + key);
    return v === null ? def : JSON.parse(v);
  } catch { return def; }
}
export function save(key, value) {
  try { localStorage.setItem('wa.' + key, JSON.stringify(value)); } catch { /* без сохранения */ }
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
    throw new Error('Нет связи с программой: ' + e.message);
  }
  let data = null;
  const text = await r.text();
  try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
  if (!r.ok) {
    const err = new Error((data && data.error) || `Ошибка ${r.status}`);
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
