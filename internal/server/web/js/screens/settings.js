// Настройки: язык, вид, единицы, звук и голос, данные мастерской, о программе.
// Всё меняется сразу, без кнопки «Сохранить», и хранится в файле настроек.

import { api, on, toast, $, $$ } from '../state.js';
import { esc } from '../fmt.js';
import { t, lang } from '../i18n.js';
import { prefs, setPref, setWorkshop, resetPrefs } from '../prefs.js';
import { switchLang } from '../app.js';
import { sounds, wakeAudio, speak, voices, speechAvailable, sayAngle } from '../assist.js';
import { showWelcome } from './welcome.js';

let root, about = null;

export function mount(el) {
  root = el;
  on('prefs', () => { if (root && root.classList.contains('active')) render(); });
  if (window.speechSynthesis) window.speechSynthesis.addEventListener('voiceschanged', () => { if (root.classList.contains('active')) render(); });
}

export async function show() {
  render();
  if (!about) {
    try { about = await api('/api/about'); render(); } catch { /* без сведений */ }
  }
}

// Переключатель: attr — атрибут кнопок, по нему вешается действие.
function seg(name, attr, current, options) {
  return `<div class="seg" role="group" aria-label="${esc(name)}">${options.map(([v, label]) =>
    `<button ${attr}="${esc(v)}" aria-pressed="${v === current ? 'true' : 'false'}">${label}</button>`).join('')}</div>`;
}

// Что было в фокусе до перерисовки — чтобы вернуть туда фокус после неё:
// иначе каждое нажатие отбрасывало бы клавиатуру в начало страницы.
function focusKey() {
  const a = document.activeElement;
  if (!a || !root.contains(a)) return null;
  if (a.id) return '#' + a.id;
  const attr = [...a.attributes].find(x => x.name.startsWith('data-') && x.name.endsWith('-set'));
  return attr ? `[${attr.name}="${attr.value}"]` : null;
}

function row(title, hint, ctl) {
  return `<div class="set-row"><div><b>${title}</b>${hint ? `<span>${hint}</span>` : ''}</div><div class="ctl">${ctl}</div></div>`;
}

function render() {
  if (!root) return;
  const keep = focusKey();
  const ws = prefs.workshop || {};
  const vs = voices();
  const voiceRow = !speechAvailable()
    ? `<span class="muted">${t('Синтез речи недоступен в этом окне.')}</span>`
    : `
      <label class="switch"><input type="checkbox" id="sVoice"${prefs.voice ? ' checked' : ''}> ${t('Озвучивать выбранный угол и итог')}</label>
      ${vs.length ? `
        <label class="f" style="min-width:280px">${t('Голос')}
          <select id="sVoiceName"><option value="">${t('по умолчанию')}</option>
          ${vs.map(v => `<option value="${esc(v.name)}"${v.name === prefs.voiceName ? ' selected' : ''}>${esc(v.name)}</option>`).join('')}</select></label>`
        : `<div class="warn">${t('В системе не найден голос для этого языка. В Windows: Параметры → Время и язык → Речь → «Добавить голоса».')}</div>`}
      <label class="f" style="min-width:280px">${t('Скорость речи')}
        <input type="range" id="sRate" min="0.6" max="1.6" step="0.1" value="${prefs.voiceRate}"></label>
      <button class="btn small" id="sVoiceTest">${t('Проверить голос')}</button>`;

  root.innerHTML = `
    <div class="settings">
      <h1>${t('Настройки')}</h1>
      <p class="lede">${t('Всё применяется сразу и запоминается программой — в том числе после обновления.')}</p>

      <div class="panel">
        <h2 style="margin-top:0">${t('Язык и вид')}</h2>
        ${row(t('Язык'), '', seg(t('Язык'), 'data-lang-set', lang, [['ru', 'Русский'], ['en', 'English']]))}
        ${row(t('Тема'), t('Тёмная — как на стенде; светлая — для яркого гаража и солнца; контрастная — для слабого зрения.'),
          seg(t('Тема'), 'data-theme-set', prefs.theme, [['dark', t('Тёмная')], ['light', t('Светлая')], ['contrast', t('Высокий контраст')]]))}
        ${row(t('Размер интерфейса'), t('Увеличивает всё сразу: текст, кнопки, цифры. Удобно читать издалека и при слабом зрении.'),
          seg(t('Размер интерфейса'), 'data-size-set', prefs.size, [['normal', t('Обычный')], ['large', t('Крупный')], ['xlarge', t('Очень крупный')]]))}
        ${row(t('Цвета «в допуске / вне допуска»'), t('Синий и оранжевый различимы при дальтонизме. Знаки ✓ ! ✕ показываются всегда.'),
          seg(t('Цвета'), 'data-palette-set', prefs.palette, [['standard', t('Зелёный и красный')], ['cb', t('Синий и оранжевый')]]) +
          `<div class="preview" aria-hidden="true"><span class="good">✓ +0°12′</span><span class="marginal">! −0°27′</span><span class="bad">✕ −0°54′</span></div>`)}
        ${row(t('Анимация'), t('Без анимации — без мигания и плавных переходов.'),
          seg(t('Анимация'), 'data-motion-set', prefs.motion, [['auto', t('Как в системе')], ['reduce', t('Без анимации')]]))}
      </div>

      <div class="panel">
        <h2 style="margin-top:0">${t('Единицы')}</h2>
        ${row(t('Как показывать углы'), t('Можно менять и прямо на экране регулировки.'),
          seg(t('Единицы'), 'data-units-set', prefs.units, [['dm', t('Градусы и минуты (0°30′)')], ['deg', t('Десятичные градусы (0.50°)')], ['mm', t('Схождение в мм')]]))}
      </div>

      <div class="panel">
        <h2 style="margin-top:0">${t('Регулировка на слух')}</h2>
        <p class="muted" style="font-size:13px">${t('Когда лежишь под машиной, экрана не видно. Выберите угол на экране регулировки — программа подскажет звуком или голосом, куда он идёт.')}</p>
        ${row(t('Звуковые сигналы'), t('Вне допуска — писк, тем чаще, чем ближе к допуску; вошли в допуск — перезвон; вышли — низкий тон. Клавиша F7.'),
          `<label class="switch"><input type="checkbox" id="sSound"${prefs.sound ? ' checked' : ''}> ${t('Включить звуковые сигналы')}</label>
           <button class="btn small" id="sSoundTest">${t('Проверить звук')}</button>`)}
        ${row(t('Голос'), t('Читает выбранный угол, говорит, когда он вошёл в допуск и когда всё в допуске. Клавиша F8.'), voiceRow)}
      </div>

      <div class="panel">
        <h2 style="margin-top:0">${t('Мастерская — для протокола')}</h2>
        <p class="muted" style="font-size:13px">${t('Попадает в шапку протокола. Можно не заполнять.')}</p>
        <div class="cols3">
          <label class="f">${t('Название')}<input type="text" data-ws="name" value="${esc(ws.name || '')}" placeholder="${esc(t('Гараж или СТО'))}"></label>
          <label class="f">${t('Адрес, телефон')}<input type="text" data-ws="contacts" value="${esc(ws.contacts || '')}"></label>
          <label class="f">${t('Кто выполняет (по умолчанию)')}<input type="text" data-ws="who" value="${esc(ws.who || '')}"></label>
        </div>
      </div>

      <div class="panel">
        <h2 style="margin-top:0">${t('О программе')}</h2>
        <div class="kv">
          <div>${t('Версия')}</div><div class="num">${esc(about ? about.version : '…')}</div>
          <div>${t('База')}</div><div>${about ? esc(t('{n} записей, {m} марок', { n: about.vehicles, m: about.makes })) : '…'}</div>
          <div>${t('Ваши данные')}</div><div><code style="user-select:all">${esc(about ? about.data_dir : '…')}</code><br>
            <span class="muted" style="font-size:12.5px">${t('Здесь ваши допуски, калибровки телефонов и камер, настройки. Обновление программы их не трогает.')}</span></div>
        </div>
        <div class="actions">
          <button class="btn" id="sWelcome">${t('Показать приветствие снова')}</button>
          <button class="btn danger" id="sReset">${t('Сбросить настройки вида и звука')}</button>
          <a class="btn" href="https://github.com/AristarhUcolov/wheel-alignment">${t('Проект на GitHub')}</a>
          <a class="btn" href="https://github.com/AristarhUcolov/wheel-alignment/issues">${t('Сообщить об ошибке')}</a>
        </div>
      </div>
    </div>`;

  const bind = (attr, fn) => $$(`[${attr}]`, root).forEach(b => b.onclick = () => fn(b.getAttribute(attr)));
  bind('data-lang-set', v => switchLang(v));
  bind('data-theme-set', v => setPref('theme', v));
  bind('data-size-set', v => setPref('size', v));
  bind('data-palette-set', v => setPref('palette', v));
  bind('data-motion-set', v => setPref('motion', v));
  bind('data-units-set', v => setPref('units', v));

  $('#sSound', root).onchange = e => { setPref('sound', e.target.checked); if (e.target.checked) { wakeAudio(); sounds.inside(); } };
  $('#sSoundTest', root).onclick = () => {
    wakeAudio();
    sounds.beep();
    setTimeout(sounds.beep, 450);
    setTimeout(sounds.beep, 800);
    setTimeout(sounds.inside, 1200);
  };
  const v = $('#sVoice', root);
  if (v) {
    v.onchange = e => setPref('voice', e.target.checked);
    const vn = $('#sVoiceName', root);
    if (vn) vn.onchange = e => setPref('voiceName', e.target.value);
    $('#sRate', root).onchange = e => setPref('voiceRate', Number(e.target.value));
    $('#sVoiceTest', root).onclick = () => speak(t('Развал, переднее левое: {v}. В допуске.', { v: sayAngle(-12 / 60) }));
  }
  $$('[data-ws]', root).forEach(inp => inp.oninput = () => setWorkshop(inp.dataset.ws, inp.value));
  $('#sWelcome', root).onclick = () => showWelcome();
  $('#sReset', root).onclick = () => {
    if (!confirm(t('Вернуть тему, размер, палитру, единицы, звук и голос к исходным?'))) return;
    resetPrefs();
    toast(t('Настройки вида и звука сброшены'));
  };
  if (keep) { const el = $(keep, root); if (el) el.focus({ preventScroll: true }); }
}
