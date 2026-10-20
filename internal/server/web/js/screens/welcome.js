// Приветствие при первом запуске: три шага — как удобнее видеть, чем мерить,
// с чего начать. Можно пропустить; снова открыть — в «Настройках».

import { api, refreshSession, toast, h, $, $$ } from '../state.js';
import { esc } from '../fmt.js';
import { t, lang } from '../i18n.js';
import { prefs, setPref } from '../prefs.js';
import { go, switchLang } from '../app.js';
import { setMethod } from './measure.js';
import { wakeAudio, sounds } from '../assist.js';

let back = null, step = 0, method = 'manual', opener = null;

export function maybeWelcome() {
  if (!prefs.welcomed) showWelcome();
}

export function showWelcome() {
  if (back) return;
  opener = document.activeElement;
  step = 0;
  back = h(`<div class="modal-back"><div class="modal" role="dialog" aria-modal="true" aria-labelledby="wTitle"></div></div>`);
  document.body.append(back);
  back.addEventListener('keydown', onKey);
  draw();
}

function close(done = true) {
  if (!back) return;
  back.remove();
  back = null;
  if (done && !prefs.welcomed) setPref('welcomed', true);
  if (opener && document.body.contains(opener)) opener.focus({ preventScroll: true });
}

// Клавиатура: Esc — пропустить, Tab не уходит из окна.
function onKey(e) {
  if (e.key === 'Escape') { e.preventDefault(); close(); return; }
  if (e.key !== 'Tab') return;
  const f = $$('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])', back).filter(x => !x.disabled && x.offsetParent !== null);
  if (!f.length) return;
  const first = f[0], last = f[f.length - 1];
  if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
  else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
}

function seg(attr, current, options) {
  return `<div class="seg" role="group">${options.map(([v, label]) =>
    `<button ${attr}="${esc(v)}" aria-pressed="${v === current ? 'true' : 'false'}">${label}</button>`).join('')}</div>`;
}

const METHODS = () => [
  ['manual', t('Струна и угломер'), t('Самое доступное: леска, рулетка, угломер.')],
  ['phone', t('Телефон на колесе'), t('Развал и кастер телефоном, по Wi-Fi, вживую.')],
  ['optical', t('Камера и мишени'), t('Мишени печатает программа; телефон на штативе — как 3D-стенд.')],
  ['sensor', t('Свой датчик'), t('Самодельная голова на ESP32 или другой прибор.')],
  ['demo', t('Пока ничего'), t('Посмотреть на учебной машине, как это работает.')],
];

function draw() {
  const m = $('.modal', back);
  const dots = `<div class="steps-dots" aria-hidden="true">${[0, 1, 2].map(i => `<i class="${i === step ? 'on' : ''}"></i>`).join('')}</div>`;
  const stepOf = t('Шаг {n} из {total}', { n: step + 1, total: 3 });
  let body = '';
  if (step === 0) {
    body = `
      <h1 id="wTitle" tabindex="-1">${t('Добро пожаловать')}</h1>
      <p class="lede">${t('Программа показывает углы колёс, как профессиональный стенд, и подсказывает, что и чем регулировать именно на вашей машине. Сначала — как вам удобнее видеть экран.')}</p>
      <div class="set-row"><div><b>${t('Язык')}</b></div><div class="ctl">${seg('data-wl', lang, [['ru', 'Русский'], ['en', 'English']])}</div></div>
      <div class="set-row"><div><b>${t('Тема')}</b><span>${t('Светлая — для яркого гаража и солнца.')}</span></div><div class="ctl">${seg('data-wt', prefs.theme, [['dark', t('Тёмная')], ['light', t('Светлая')], ['contrast', t('Высокий контраст')]])}</div></div>
      <div class="set-row"><div><b>${t('Размер интерфейса')}</b><span>${t('Крупнее — удобнее читать издалека.')}</span></div><div class="ctl">${seg('data-ws', prefs.size, [['normal', t('Обычный')], ['large', t('Крупный')], ['xlarge', t('Очень крупный')]])}</div></div>
      <div class="set-row"><div><b>${t('Цвета')}</b></div><div class="ctl">
        <label class="switch"><input type="checkbox" id="wCb"${prefs.palette === 'cb' ? ' checked' : ''}> ${t('Плохо различаю красный и зелёный — показывать синим и оранжевым')}</label>
        <div class="preview" aria-hidden="true"><span class="good">✓ +0°12′</span><span class="marginal">! −0°27′</span><span class="bad">✕ −0°54′</span></div></div></div>`;
  } else if (step === 1) {
    body = `
      <h1 id="wTitle" tabindex="-1">${t('Чем будете мерить?')}</h1>
      <p class="lede">${t('Можно поменять в любой момент на шаге «Замер», и способы можно сочетать.')}</p>
      <div class="choice-grid">${METHODS().map(([id, name, desc]) =>
        `<button class="choice" data-m="${id}" aria-pressed="${id === method ? 'true' : 'false'}"><b>${name}</b><span>${desc}</span></button>`).join('')}</div>`;
  } else {
    body = `
      <h1 id="wTitle" tabindex="-1">${t('С чего начать')}</h1>
      <ol class="steps">
        <li><b>${t('Автомобиль')}</b> — ${t('найдите свою машину: от неё зависят допуски и советы.')}</li>
        <li><b>${t('Подготовка')}</b> — ${t('давление, люфты, ровная площадка. Без этого регулировка не держится.')}</li>
        <li><b>${t('Замер')}</b> — ${t('введите или подключите показания.')}</li>
        <li><b>${t('Регулировка')}</b> — ${t('зелёное в допуске, красное — нет; нажмите на угол, чтобы узнать, чем его крутить.')}</li>
        <li><b>${t('Отчёт')}</b> — ${t('протокол «до / после» для себя или клиента.')}</li>
      </ol>
      <div class="set-row"><div><b>${t('Работаете один, под машиной?')}</b><span>${t('Экрана оттуда не видно — программа может подсказывать звуком и голосом. Клавиши F7 и F8.')}</span></div>
        <div class="ctl">
          <label class="switch"><input type="checkbox" id="wSound"${prefs.sound ? ' checked' : ''}> ${t('Звуковые сигналы')}</label>
          ${window.speechSynthesis ? `<label class="switch"><input type="checkbox" id="wVoice"${prefs.voice ? ' checked' : ''}> ${t('Голос: читать выбранный угол')}</label>` : ''}
        </div></div>`;
  }
  const last = step === 2;
  m.innerHTML = `${dots}<p class="dim" style="font-size:12.5px;margin:0 0 6px">${stepOf}</p>${body}
    <div class="foot">
      <button class="btn" id="wSkip">${t('Пропустить')}</button>
      <div class="actions" style="margin:0">
        ${step ? `<button class="btn" id="wBack">${t('Назад')}</button>` : ''}
        ${last
          ? `<button class="btn" id="wDemo">${t('Посмотреть демонстрацию')}</button><button class="btn primary" id="wCar">${t('Найти свой автомобиль')}</button>`
          : `<button class="btn primary" id="wNext">${t('Далее')}</button>`}
      </div>
    </div>`;

  const bind = (sel, attr, fn) => $$(sel, m).forEach(b => b.onclick = () => fn(b.getAttribute(attr)));
  bind('[data-wl]', 'data-wl', v => switchLang(v));
  bind('[data-wt]', 'data-wt', v => { setPref('theme', v); draw(); focusAttr('data-wt', v); });
  bind('[data-ws]', 'data-ws', v => { setPref('size', v); draw(); focusAttr('data-ws', v); });
  bind('[data-m]', 'data-m', v => { method = v; draw(); focusAttr('data-m', v); });
  const cb = $('#wCb', m);
  if (cb) cb.onchange = e => { setPref('palette', e.target.checked ? 'cb' : 'standard'); };
  const ws = $('#wSound', m), wv = $('#wVoice', m);
  if (ws) ws.onchange = e => { setPref('sound', e.target.checked); if (e.target.checked) { wakeAudio(); sounds.inside(); } };
  if (wv) wv.onchange = e => setPref('voice', e.target.checked);

  $('#wSkip', m).onclick = () => close();
  const nb = $('#wNext', m), bb = $('#wBack', m);
  if (nb) nb.onclick = () => { step++; draw(); };
  if (bb) bb.onclick = () => { step--; draw(); };
  const car = $('#wCar', m), demo = $('#wDemo', m);
  if (car) car.onclick = () => { finish(); go('vehicle'); setTimeout(() => { const q = $('#vq'); if (q) q.focus(); }, 60); };
  if (demo) demo.onclick = async () => {
    finish();
    try {
      await api('/api/sim', { method: 'POST', body: { action: 'start' } });
      await refreshSession();
      go('live');
      toast(t('Демонстрация запущена. Нажмите на любой угол, чтобы узнать, как его регулировать.'));
    } catch (e) { toast(e.message, true); }
  };
  if (!document.activeElement || !back.contains(document.activeElement)) $('#wTitle', m).focus({ preventScroll: true });
}

function focusAttr(attr, v) {
  const el = back && back.querySelector(`[${attr}="${v}"]`);
  if (el) el.focus({ preventScroll: true });
}

function finish() {
  setMethod(method);
  close();
}
