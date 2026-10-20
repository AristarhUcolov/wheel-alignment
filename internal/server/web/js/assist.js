// Регулировка на слух: звуковые сигналы и голос.
//
// Человек под машиной не видит экрана — ему нужно слышать, приближается ли
// угол к допуску и когда вошёл в него. Сигналы звучат только для выбранного
// угла (открытого в боковой панели): двенадцать углов, пищащих разом, не
// помогут никому. Голос читает выбранный угол и говорит, когда всё в допуске.
// Это же и для тех, кто плохо видит экран.

import { state, on } from './state.js';
import { prefs } from './prefs.js';
import { t, lang } from './i18n.js';
import { shortLabel, isToeKey, statusText } from './fmt.js';

// ── Звук ─────────────────────────────────────────────────────────────

let ctx = null;

// Звук в браузере разрешён только после действия человека — поэтому
// контекст создаётся или «будится» по первому щелчку или клавише.
export function wakeAudio() {
  try {
    if (!ctx) ctx = new (window.AudioContext || window.webkitAudioContext)();
    if (ctx.state === 'suspended') ctx.resume();
  } catch { ctx = null; }
  return ctx;
}
for (const ev of ['pointerdown', 'keydown']) {
  window.addEventListener(ev, () => { if (prefs.sound) wakeAudio(); }, { capture: true });
}

function tone(freq, dur, { type = 'sine', gain = 0.16, at = 0 } = {}) {
  const c = wakeAudio();
  if (!c) return;
  const t0 = c.currentTime + at;
  const o = c.createOscillator(), g = c.createGain();
  o.type = type;
  o.frequency.value = freq;
  g.gain.setValueAtTime(0, t0);
  g.gain.linearRampToValueAtTime(gain, t0 + 0.012);
  g.gain.exponentialRampToValueAtTime(0.0008, t0 + dur);
  o.connect(g).connect(c.destination);
  o.start(t0);
  o.stop(t0 + dur + 0.02);
}

export const sounds = {
  beep: () => tone(880, 0.07, { type: 'square', gain: 0.07 }),
  inside: () => { tone(660, 0.14); tone(990, 0.22, { at: 0.13 }); },
  outside: () => tone(300, 0.22, { type: 'triangle', gain: 0.2 }),
  allGood: () => { tone(660, 0.12); tone(830, 0.12, { at: 0.12 }); tone(990, 0.3, { at: 0.24 }); },
};

// ── Голос ────────────────────────────────────────────────────────────

const synth = window.speechSynthesis || null;

export function voices() {
  if (!synth) return [];
  const want = lang === 'en' ? 'en' : 'ru';
  return synth.getVoices().filter(v => v.lang && v.lang.toLowerCase().startsWith(want));
}

export const speechAvailable = () => !!synth;

function pickVoice() {
  const list = voices();
  return list.find(v => v.name === prefs.voiceName) || list.find(v => v.localService) || list[0] || null;
}

export function speak(text, { interrupt = true } = {}) {
  if (!synth || !text) return;
  if (interrupt) synth.cancel();
  const u = new SpeechSynthesisUtterance(text);
  u.lang = lang === 'en' ? 'en-US' : 'ru-RU';
  const v = pickVoice();
  if (v) u.voice = v;
  u.rate = Number(prefs.voiceRate) || 1;
  synth.speak(u);
}

export function stopSpeaking() { if (synth) synth.cancel(); }

// Число для чтения вслух: «минус 0 градусов 24 минуты», а не «−0°24′» —
// знаки градуса и минуты синтезатор речи читает как попало.
function plural(n, [one, few, many]) {
  if (lang === 'en') return n === 1 ? one : few;
  const k = n % 100, d = n % 10;
  if (k >= 11 && k <= 14) return many;
  if (d === 1) return one;
  if (d >= 2 && d <= 4) return few;
  return many;
}

const W = () => ({
  deg: [t('градус'), t('градуса'), t('градусов')],
  min: [t('минута'), t('минуты'), t('минут')],
  mm: [t('миллиметр'), t('миллиметра'), t('миллиметров')],
});

function signWord(v) {
  if (Math.abs(v) < 1e-9) return '';
  return (v < 0 ? t('минус') : t('плюс')) + ' ';
}

function decimal(v, digits) {
  const s = Math.abs(v).toFixed(digits);
  return lang === 'en' ? s : s.replace('.', ',');
}

export function sayAngle(deg, units = 'dm', key = '', rimMM = 0) {
  if (deg === null || deg === undefined || Number.isNaN(deg)) return t('нет показаний');
  const w = W();
  // Дробное число — «1,2 миллиметра», «0,25 градуса»; целое — по обычным
  // правилам числительных.
  if (units === 'mm' && isToeKey(key) && rimMM > 0) {
    const mm = rimMM * Math.tan(deg * Math.PI / 180);
    const r = Math.round(Math.abs(mm) * 10) / 10;
    const frac = r % 1 !== 0;
    return `${signWord(r ? mm : 0)}${decimal(r, frac ? 1 : 0)} ${frac ? w.mm[1] : plural(r, w.mm)}`;
  }
  if (units === 'deg') {
    const r = Math.round(Math.abs(deg) * 100) / 100;
    return `${signWord(r ? deg : 0)}${decimal(r, 2)} ${w.deg[1]}`;
  }
  const a = Math.abs(deg);
  let d = Math.floor(a), m = Math.round((a - d) * 60);
  if (m === 60) { d++; m = 0; }
  const sign = d || m ? signWord(deg) : '';
  return `${sign}${d} ${plural(d, w.deg)} ${m} ${plural(m, w.min)}`;
}

// ── Слежение за выбранным углом ──────────────────────────────────────

const track = { key: null, status: null, spokenAt: 0, spokenV: null, nextBeep: 0, overall: null };

function labelOf(key) {
  const info = state.session && state.session.params ? state.session.params[key] : null;
  return info ? info.label : shortLabel(key);
}

function rimMM() {
  const ss = state.session;
  return ss ? (ss.rim_diameter_in || 0) * 25.4 : 0;
}

function phrase(key, p, withGo = true) {
  const units = state.units;
  let s = `${labelOf(key)}: ${sayAngle(p.v, units, key, rimMM())}.`;
  if (p.spec) {
    s += ' ' + statusText(p.status) + '.';
    if (withGo && p.status === 'bad') s += ' ' + t('До номинала {v}.', { v: sayAngle(p.to_nominal, units, key, rimMM()) });
  }
  return s;
}

// Новый выбранный угол — прочитать его сразу.
export function announce(key) {
  const p = state.frame && state.frame.params[key];
  track.key = key;
  track.status = p && p.has ? p.status : null;
  track.spokenAt = performance.now();
  track.spokenV = p && p.has ? p.v : null;
  if (prefs.voice && p && p.has) speak(phrase(key, p));
}

function onFrame(f) {
  if (!f || state.screen !== 'live') return;
  const now = performance.now();

  // Всё в допуске — это стоит сказать, даже если угол не выбран.
  if (f.has_limits && f.measured > 0 && f.overall !== track.overall) {
    const was = track.overall;
    track.overall = f.overall;
    if (f.overall === 'good' && was && was !== 'good') {
      if (prefs.sound) sounds.allGood();
      if (prefs.voice && !state.selected) speak(t('Все измеренные углы в допуске'));
    }
  }

  const key = state.selected;
  if (!key) { track.key = null; return; }
  const p = f.params[key];
  if (key !== track.key) { announce(key); return; }
  if (!p || !p.has || !p.spec) return;

  const was = track.status;
  if (p.status !== was) {
    track.status = p.status;
    const inside = s => s === 'good' || s === 'marginal';
    if (prefs.sound) {
      if (inside(p.status) && !inside(was)) sounds.inside();
      else if (!inside(p.status) && inside(was)) sounds.outside();
    }
    if (prefs.voice && (inside(p.status) !== inside(was))) {
      speak(phrase(key, p, false));
      track.spokenAt = now; track.spokenV = p.v;
    }
  }

  // Вне допуска — пищим тем чаще, чем ближе к границе, как парктроник.
  if (prefs.sound && p.status === 'bad' && now >= track.nextBeep) {
    const half = Math.max((p.spec.max - p.spec.min) / 2, 0.05);
    const r = Math.min(Math.abs(p.deviation) / half, 4);
    sounds.beep();
    track.nextBeep = now + (220 + 320 * r);
  }

  // Голос: устоявшееся значение, заметно изменившееся, — не чаще раза в 5 с.
  if (prefs.voice && p.stable && now - track.spokenAt > 5000 &&
      (track.spokenV === null || Math.abs(p.v - track.spokenV) >= 1 / 60)) {
    speak(phrase(key, p), { interrupt: false });
    track.spokenAt = now; track.spokenV = p.v;
  }
}

on('frame', onFrame);
