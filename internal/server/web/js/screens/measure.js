// Шаг 3: замер — выбор способа и ввод данных.

import { state, on, api, saveJob, toast, refreshSession, h, $, $$ } from '../state.js';
import { WHEELS, esc, fmtParam, parseNum } from '../fmt.js';
import { t } from '../i18n.js';
import { go, snapshot } from '../app.js';
import { prefs, setPref } from '../prefs.js';
import * as optical from './optical.js';
import { figure } from '../illus.js';
import * as phone from './phone.js';

let root;

const method = () => prefs.method || 'manual';

// Что меряет каждый способ: да, нет или «зависит» — и чем. Видно сразу, без
// чтения инструкции: например, что телефон на ободе схождение не меряет.
const METHODS = () => [
  { id: 'manual', name: t('Струна и угломер'), desc: t('Леска вдоль бортов, рулетка и угломер. Стоит как один заезд на СТО.'),
    caps: [['yes', t('Развал'), t('угломер')], ['yes', t('Кастер'), t('с поворотными кругами')], ['yes', t('Схождение'), t('струна')]] },
  { id: 'phone', name: t('Телефон на колесе'), desc: t('Телефон прикладывается к ободу и передаёт углы по Wi-Fi в реальном времени.'),
    caps: [['yes', t('Развал'), t('вживую')], ['yes', t('Кастер'), t('с поворотными кругами')], ['no', t('Схождение'), t('нужна струна или камера')]] },
  { id: 'optical', name: t('Камера и мишени'), desc: t('Печатные шахматные мишени на колёсах и фотоаппарат или телефон.'), beta: true,
    caps: [['yes', t('Развал'), ''], ['yes', t('Кастер'), ''], ['yes', t('Схождение'), t('и угол тяги')]] },
  { id: 'sensor', name: t('Свой датчик'), desc: t('Самодельная голова на ESP32 или любой прибор, умеющий отправить JSON.'),
    caps: [['dep', t('Развал'), ''], ['dep', t('Кастер'), ''], ['dep', t('Схождение'), t('что умеет датчик')]] },
  { id: 'demo', name: t('Демонстрация'), desc: t('Учебная машина: всё как на настоящей, но без машины.'),
    caps: [['yes', t('Развал'), ''], ['yes', t('Кастер'), ''], ['yes', t('Схождение'), t('без машины')]] },
];

const CAP = { yes: '✓', no: '✕', dep: '?' };

// Способ замера выбирают здесь, в приветствии и на шаге «Подготовка».
export function setMethod(m) {
  if (m === method()) return;
  setPref('method', m);
  if (root && root.classList.contains('active')) { renderMethods(); renderBody(); }
}

export function mount(el) {
  root = el;
  root.innerHTML = `
    <h1>${t('Замер')}</h1>
    <p class="lede">${t('Выберите, чем будете мерить. Данные сразу попадают на экран «Регулировка» — там видно, что в допуске, а что нет, и стрелки двигаются, пока вы крутите регулировку.')}</p>
    <div class="method-cards" id="mMethods" role="group" aria-label="${t('Способ замера')}"></div>
    <p class="muted" style="font-size:13px;margin:-8px 0 16px">${t('Способы можно сочетать: например, развал и кастер — телефоном, схождение — струной. Программа сведёт всё на одном экране.')}</p>
    <div id="mBody"></div>
    <div class="actions" id="mActions"></div>`;
  on('frame', updateMini);
  on('session', () => { if (state.screen === 'measure') renderActions(); });
  // Новая машина: поля прежней — пустые.
  on('job', () => { manualData = null; if (root.classList.contains('active')) renderBody(); });
}

export function show() {
  renderMethods();
  renderBody();
  renderActions();
}

function renderMethods() {
  const box = $('#mMethods', root);
  box.innerHTML = METHODS().map(m => `
    <button class="method${m.id === method() ? ' on' : ''}" data-m="${m.id}" aria-pressed="${m.id === method() ? 'true' : 'false'}">
      <b>${m.name}${m.beta ? ` <span class="badge unverified">${t('бета')}</span>` : ''}</b>
      <span>${m.desc}</span>
      <span class="caps">${m.caps.map(([k, name, how]) =>
        `<span class="cap ${k}"><i aria-hidden="true">${CAP[k]}</i> ${name}${how ? ` <small>— ${how}</small>` : ''}</span>`).join('')}</span>
    </button>`).join('');
  $$('.method', box).forEach(b => b.onclick = () => {
    const was = method();
    setPref('method', b.dataset.m);
    renderMethods();
    renderBody();
    $(`.method[data-m="${b.dataset.m}"]`, root).focus({ preventScroll: true });
    // Карточки способов занимают пол-экрана — сразу к тому, что выбрали.
    if (was !== b.dataset.m) $('#mBody', root).scrollIntoView({ behavior: 'smooth', block: 'start' });
  });
}

function renderActions() {
  const ss = state.session || {};
  const box = $('#mActions', root);
  if (!box) return;
  box.innerHTML = `
    <button class="btn primary" id="mLive">${t('Открыть экран регулировки →')}</button>
    <button class="btn" id="mBefore">${ss.has_before ? t('Переснять «до»') : t('Зафиксировать «до»')}</button>
    <span class="muted" style="font-size:13px">${t('«До» — состояние до регулировки, для отчёта. Нужны развал и схождение всех четырёх колёс.')}</span>`;
  $('#mLive', box).onclick = () => go('live');
  $('#mBefore', box).onclick = () => snapshot('before');
}

function renderBody() {
  const body = $('#mBody', root);
  body.innerHTML = '';
  const m = method();
  if (m === 'manual') manual(body);
  else if (m === 'phone') phone.render(body);
  else if (m === 'optical') optical.render(body);
  else if (m === 'sensor') sensor(body);
  else demo(body);
  updateMini();
}

// ── Струна и угломер ─────────────────────────────────────────────────

// Поля формы по колёсам — как их хранит программа в файле работы. Числа, а
// не текст: «0,5» после перезапуска покажется как «0.5», и это одно и то же.
let manualData = null;
const FIELDS = ['camber', 'camber_180', 'toe_front_mm', 'toe_rear_mm', 'sweep_out', 'sweep_in', 'half_sweep'];

function loadManual() {
  if (manualData) return manualData;
  manualData = {};
  const saved = (state.session && state.session.manual) || {};
  for (const [w, d] of Object.entries(saved)) {
    manualData[w] = { invert_gauge: !!d.invert_gauge };
    for (const f of FIELDS) manualData[w][f] = d[f] ?? null;
  }
  return manualData;
}

function manual(body) {
  const ss = state.session || {};
  const data = loadManual();
  body.append(h(`<div>
    <div class="panel">
      <h2 style="margin-top:0">${t('Как мерить')}</h2>
      <ol class="plain" style="padding-left:20px;margin:0">
        <li>${t('Развал — угломер на обод (не на резину), «плюс», когда верх колеса наклонён наружу. Затем прокатите машину на пол-оборота колеса и снова приложите угломер к тому же месту обода — программа усреднит и уберёт биение диска.')}</li>
        <li>${t('Схождение — от струны до края обода спереди и сзади колеса, строго на высоте центра ступицы.')}</li>
        <li>${t('Кастер (по желанию, нужны поворотные круги) — развал при повороте колеса на 20° наружу и 20° внутрь.')}</li>
      </ol>
      <p class="muted" style="margin-top:10px;font-size:13px">${t('Каждое введённое число сразу уходит на экран регулировки и сохраняется — закроете программу, откроете снова, всё на месте. Меряете после поворота регулятора — вводите новое число, стрелка сдвинется. Стёрли число — оно уйдёт и с экрана.')}</p>
      <details class="figs"><summary>${t('Рисунки: планка на ободе, струны, биение диска')}</summary>
        <div class="illus-row">${figure('bar')}${figure('strings')}${figure('runout')}</div></details>
    </div>

    <details class="panel" id="boxCheck" style="margin-top:12px" open>
      <summary style="cursor:pointer;font-weight:600">${t('Проверка установки струн')} <span class="muted" style="font-weight:400">${t('— самая частая причина ошибки')}</span></summary>
      <p class="muted" style="margin-top:10px;font-size:13px">${t('Совет «сделайте отступы спереди и сзади одинаковыми» верен только при равных колеях. Введите расстояние от струны до центра ступицы на каждом колесе — программа скажет, как сдвинуть концы струн.')}
        ${t('Колеи — на шаге «Автомобиль» (сейчас: перед {f} мм, зад {r} мм).', { f: ss.track_front_mm || '—', r: ss.track_rear_mm || '—' })}</p>
      <div class="cols4">
        <label class="f">${t('Левая, у переднего')}<input type="text" inputmode="decimal" data-num data-box="lf"></label>
        <label class="f">${t('Левая, у заднего')}<input type="text" inputmode="decimal" data-num data-box="lr"></label>
        <label class="f">${t('Правая, у переднего')}<input type="text" inputmode="decimal" data-num data-box="rf"></label>
        <label class="f">${t('Правая, у заднего')}<input type="text" inputmode="decimal" data-num data-box="rr"></label>
      </div>
      <div id="boxHint" style="margin-top:10px"></div>
    </details>

    <div class="wheelgrid" id="mWheels" style="margin-top:12px"></div>

    <details class="panel" style="margin-top:12px">
      <summary style="cursor:pointer;font-weight:600">${t('Кастер: куда поворачивать руль')}</summary>
      <p class="muted" style="margin-top:10px;font-size:13px">${t('Колёса на поворотных кругах, педаль тормоза зажата упором. Для левого колеса «наружу» — руль влево, для правого — вправо. Угол поворота колеса читайте по шкале круга.')}</p>
      <div class="illus-row">${figure('steer', { arg: 'left', caption: false })}${figure('steer', { arg: 'right', caption: false })}${figure('pedal')}</div>
    </details>

    <details class="panel" id="tootBox" style="margin-top:12px">
      <summary style="cursor:pointer;font-weight:600">${t('Углы поворота колёс')} <span class="muted" style="font-weight:400">${t('— по желанию: найти погнутый поворотный рычаг')}</span></summary>
      <p class="muted" style="margin-top:10px;font-size:13px">${t('На поворотных кругах поверните руль так, чтобы наружное колесо встало ровно на 20°, и запишите, сколько показывает круг под внутренним. Потом то же в другую сторону. Внутреннее колесо обычно поворачивается на 1–4° больше наружного; главное — чтобы влево и вправо было одинаково (методика ASE).')}</p>
      <div class="cols3">
        <label class="f">${t('Руль влево: правое на 20°, левое показывает, °')}<input type="text" inputmode="decimal" data-num data-note="toot_l"></label>
        <label class="f">${t('Руль вправо: левое на 20°, правое показывает, °')}<input type="text" inputmode="decimal" data-num data-note="toot_r"></label>
      </div>
      <div id="tootRes" style="margin-top:10px"></div>
    </details>
    <div class="actions"><button class="btn danger" id="mClear">${t('Очистить все поля')}</button></div>
  </div>`));

  const grid = $('#mWheels', body);
  for (const w of WHEELS) {
    const d = data[w.key] || {};
    const v = f => (d[f] === null || d[f] === undefined) ? '' : d[f];
    grid.append(h(`<div class="wcard" data-w="${w.key}">
      <h3><span>${w.name}</span><span class="live-mini" data-mini="${w.key}"></span></h3>
      <div class="pair">
        <label class="f">${t('Развал, °')}<input type="text" inputmode="decimal" data-num data-f="camber" value="${v('camber')}"></label>
        <label class="f">${t('Он же через ½ оборота, °')}<input type="text" inputmode="decimal" data-num data-f="camber_180" value="${v('camber_180')}"></label>
      </div>
      <div class="pair">
        <label class="f">${t('Струна → обод спереди, мм')}<input type="text" inputmode="decimal" data-num data-f="toe_front_mm" value="${v('toe_front_mm')}"></label>
        <label class="f">${t('Струна → обод сзади, мм')}<input type="text" inputmode="decimal" data-num data-f="toe_rear_mm" value="${v('toe_rear_mm')}"></label>
      </div>
      <label class="chk" style="font-size:13px;color:var(--muted)"><input type="checkbox" data-f="invert_gauge"${d.invert_gauge ? ' checked' : ''}>
        <span>${t('Угломер показывает наоборот (наклон наружу — минус)')}</span></label>
      ${w.front ? `
      <details style="margin-top:10px"${d.sweep_out !== null && d.sweep_out !== undefined || d.sweep_in !== null && d.sweep_in !== undefined ? ' open' : ''}>
        <summary style="cursor:pointer;color:var(--blue);font-size:13.5px">${t('Кастер — развал при повороте колеса')}</summary>
        <div class="pair" style="margin-top:8px">
          <label class="f">${t('Колесо повёрнуто НАРУЖУ, °')}<input type="text" inputmode="decimal" data-num data-f="sweep_out" value="${v('sweep_out')}"></label>
          <label class="f">${t('Колесо повёрнуто ВНУТРЬ, °')}<input type="text" inputmode="decimal" data-num data-f="sweep_in" value="${v('sweep_in')}"></label>
        </div>
        <label class="f">${t('На сколько градусов поворачивали в каждую сторону')}<input type="text" inputmode="decimal" data-num data-f="half_sweep" value="${v('half_sweep') || 20}"></label>
        <small class="dim">${w.key === 'FL'
          ? t('«Наружу» для левого колеса — руль влево, «внутрь» — руль вправо.')
          : t('«Наружу» для правого колеса — руль вправо, «внутрь» — руль влево.')}</small>
      </details>` : ''}
    </div>`));
  }

  const timers = {};
  $$('.wcard', grid).forEach(card => {
    const w = card.dataset.w;
    $$('input', card).forEach(inp => inp.addEventListener('input', () => {
      clearTimeout(timers[w]);
      timers[w] = setTimeout(() => sendWheel(card), 350);
    }));
  });

  const boxSaved = { ...((state.session && state.session.box) || {}) };
  $$('[data-box]', body).forEach(inp => {
    inp.value = boxSaved[inp.dataset.box] ?? '';
    inp.oninput = () => { boxSaved[inp.dataset.box] = inp.value; saveJob({ box: { ...boxSaved } }); boxCheck(body); };
  });
  boxCheck(body);

  // Углы поворота: записываются в работу, итог — сразу под полями.
  const notes = () => ({ ...((state.session && state.session.notes) || {}) });
  $$('[data-note]', body).forEach(inp => {
    inp.value = notes()[inp.dataset.note] ?? '';
    inp.oninput = () => { saveJob({ notes: { ...notes(), [inp.dataset.note]: inp.value } }); tootCheck(body); };
  });
  tootCheck(body);
  if (notes().toot_l || notes().toot_r) $('#tootBox', body).open = true;

  $('#mClear', body).onclick = async () => {
    if (!confirm(t('Очистить все введённые вручную числа? Они уйдут и с экрана регулировки.'))) return;
    for (const w of WHEELS) {
      try { await api('/api/live/manual', { method: 'POST', body: { wheel: w.key, replace: true } }); } catch (e) { toast(e.message, true); }
    }
    manualData = {};
    saveJob({ box: {} });
    if (state.session) state.session.manual = {};
    renderBody();
  };
}

function num(card, f) {
  const el = card.querySelector(`[data-f="${f}"]`);
  const v = el ? parseNum(el.value) : null;
  return Number.isFinite(v) ? v : null;
}

// Колесо отправляется целиком: пустое поле — это стёртое число, и оно уходит
// с экрана регулировки. Непонятное число (красная рамка) не отправляется, пока
// его не исправят.
async function sendWheel(card) {
  const w = card.dataset.w;
  if ($$('input[data-num]', card).some(i => Number.isNaN(parseNum(i.value)))) return;
  const d = { invert_gauge: card.querySelector('[data-f="invert_gauge"]').checked };
  for (const f of FIELDS) d[f] = num(card, f);
  loadManual()[w] = d;
  if (state.session) {
    state.session.manual = state.session.manual || {};
    state.session.manual[w] = d;
  }

  const body = { wheel: w, replace: true, invert_gauge: d.invert_gauge };
  for (const f of ['camber', 'camber_180', 'toe_front_mm', 'toe_rear_mm']) if (d[f] !== null) body[f] = d[f];
  if (WHEELS.find(x => x.key === w).front) {
    for (const f of ['sweep_out', 'sweep_in', 'half_sweep']) if (d[f] !== null) body[f] = d[f];
  }
  try {
    await api('/api/live/manual', { method: 'POST', body });
  } catch (e) { toast(`${WHEELS.find(x => x.key === w).name}: ${e.message}`, true); }
}

function boxCheck(body) {
  const ss = state.session || {};
  const g = k => { const v = parseNum($(`[data-box="${k}"]`, body).value); return Number.isFinite(v) ? v : null; };
  const [lf, lr, rf, rr] = ['lf', 'lr', 'rf', 'rr'].map(g);
  const hint = $('#boxHint', body);
  if ([lf, lr, rf, rr].some(v => v === null)) { hint.innerHTML = ''; return; }
  const tf = ss.track_front_mm, tr = ss.track_rear_mm;
  if (!tf || !tr) {
    hint.innerHTML = `<div class="warn">${t('Укажите колеи передней и задней оси на шаге «Автомобиль» — без них проверка невозможна.')}</div>`;
    return;
  }
  // Струна параллельна оси машины, когда отступ спереди минус отступ сзади
  // равен (колея зад − колея перед) / 2.
  const want = (tr - tf) / 2;
  const rows = [[t('Левая'), lf - lr], [t('Правая'), rf - rr]].map(([n, got]) => {
    const off = want - got;
    return Math.abs(off) <= 1
      ? `<div class="ok-box" style="margin-bottom:6px">✓ ${t('{side} струна параллельна оси автомобиля.', { side: n })}</div>`
      : `<div class="danger-box" style="margin-bottom:6px">✗ ${off > 0
          ? t('{side} струна: «перед − зад» = {got} мм, нужно {want} мм. Сдвиньте конец у переднего колеса на {off} мм дальше от колеса.', { side: n, got: got.toFixed(1), want: want.toFixed(1), off: Math.abs(off).toFixed(1) })
          : t('{side} струна: «перед − зад» = {got} мм, нужно {want} мм. Сдвиньте конец у переднего колеса на {off} мм ближе к колесу.', { side: n, got: got.toFixed(1), want: want.toFixed(1), off: Math.abs(off).toFixed(1) })}</div>`;
  });
  hint.innerHTML = rows.join('') + `<p class="muted" style="font-size:12.5px">${t('Требуемая разница {want} мм — из-за разных колей ({tf} и {tr} мм).', { want: want.toFixed(1), tf, tr })}</p>`;
}

// Углы поворота колёс (toe-out on turns). Разница влево и вправо больше 1° —
// погнут поворотный рычаг кулака или рулевая тяга, даже если схождение в
// норме: так колёса и стирают шины в поворотах. Методика ASE.
function tootCheck(body) {
  const box = $('#tootRes', body);
  const g = k => { const v = parseNum($(`[data-note="${k}"]`, body).value); return Number.isFinite(v) ? v : null; };
  const l = g('toot_l'), r = g('toot_r');
  if (l === null || r === null) { box.innerHTML = ''; return; }
  const d = Math.abs(l - r);
  const odd = l < 20 || r < 20
    ? `<p class="muted" style="font-size:12.5px;margin-top:6px">${t('Внутреннее колесо повёрнуто меньше наружного — так бывает редко; проверьте замер и данные завода.')}</p>` : '';
  box.innerHTML = (d > 1
    ? `<div class="danger-box">✕ ${t('Разница {d}°: один из поворотных рычагов кулака или рулевая тяга погнуты. Сравните с данными завода и проверьте рычаги до регулировки схождения.', { d: d.toFixed(1) })}</div>`
    : `<div class="ok-box">✓ ${t('Влево и вправо почти одинаково ({l}° и {r}°): поворотные рычаги в порядке.', { l: l.toFixed(1), r: r.toFixed(1) })}</div>`) + odd;
}

function updateMini() {
  if (state.screen !== 'measure' || !root) return;
  const P = state.frame ? state.frame.params : {};
  $$('[data-mini]', root).forEach(el => {
    const w = el.dataset.mini;
    const c = P['camber_' + w], tt = P['toe_' + w];
    const col = p => !p || !p.has ? 'var(--dim)' : ({ good: 'var(--green)', bad: 'var(--red)', marginal: 'var(--amber)' }[p.status] || 'var(--text)');
    el.innerHTML = `<span style="color:${col(c)}">${fmtParam('camber_' + w, c, state.units)}</span> · <span style="color:${col(tt)}">${fmtParam('toe_' + w, tt, state.units)}</span>`;
  });
}

// ── Свой датчик ──────────────────────────────────────────────────────

function sensor(body) {
  const origin = location.origin;
  body.append(h(`<div class="panel" style="max-width:980px">
    <h2 style="margin-top:0">${t('Открытый протокол для датчиков')}</h2>
    <p>${t('Любое устройство в вашей сети может присылать углы программе: самодельная голова на ESP32 с инклинометром, лазерный указатель схождения со шкалой, промышленный угломер с последовательным портом. Одно JSON-сообщение на показание или массив сообщений — запросом POST на адрес:')}</p>
    <pre class="note" style="font-size:13px;overflow:auto">${esc(origin)}/api/live/sample</pre>
    <pre class="note" style="font-size:13px;overflow:auto">{"source": "esp32-left", "wheel": "FL", "camber": -0.52, "toe": 0.08}</pre>
    <ul class="plain">
      <li>${t('source — имя датчика, обязательно; по нему он виден в строке состояния.')}</li>
      <li>${t('wheel — FL, FR, RL или RR (переднее левое, переднее правое, заднее левое, заднее правое).')}</li>
      <li>${t('camber, toe, caster — в градусах, любые из них. Развал «+» — верх колеса наружу, схождение «+» — колесо повёрнуто передним краем к центру машины.')}</li>
      <li>${t('Частота — до 20 раз в секунду; программа сама сглаживает и определяет, что показания устоялись.')}</li>
    </ul>
    <div class="warn" style="margin-top:10px">${t('Программа слушает только этот компьютер. Чтобы датчики в сети могли достучаться, включите на вкладке «Телефон на колесе» доступ по сети — тот же адрес подойдёт и для датчиков (сертификат самоподписанный, проверку сертификата в датчике отключите).')}</div>
    <h3>${t('Сейчас подключены')}</h3>
    <div id="sensList" class="muted">—</div>
  </div>`));
  const upd = () => {
    const el = $('#sensList', body);
    if (!el) return;
    const src = (state.frame && state.frame.sources || []).filter(s => s.kind === 'sensor');
    el.innerHTML = src.length ? src.map(s => `<span class="src-pill ${s.online ? 'on' : 'off'}">${esc(s.name)} · ${esc(s.wheel)}</span>`).join(' ') : t('Ни одного датчика.');
  };
  upd();
  const off = on('frame', () => { if (!document.body.contains(body)) off(); else upd(); });
}

// ── Демонстрация ─────────────────────────────────────────────────────

function demo(body) {
  const running = state.session && state.session.sim_running;
  body.append(h(`<div class="panel" style="max-width:980px">
    <h2 style="margin-top:0">${t('Учебная машина')}</h2>
    <p>${t('Переднеприводный седан с типичными перекосами: развал по бортам разный, правое колесо смотрит наружу, задняя ось чуть уводит вправо. Показания «шумят» и запаздывают, как у настоящего датчика, а кастер и развал при регулировке тянут за собой схождение — ровно поэтому схождение регулируют последним.')}</p>
    <ul class="plain">
      <li>${t('На экране регулировки нажмите на любой угол — справа откроется, что это и чем регулируется, а кнопки и стрелки ← → «крутят» регулятор.')}</li>
      <li>${t('F10 «Авторегулировка» — программа сама приведёт углы в допуск в правильном порядке.')}</li>
      <li>${t('Для оценки «в норме / не в норме» выберите автомобиль или ориентир по классу на шаге 1.')}</li>
    </ul>
    <div class="actions">
      <button class="btn primary" id="dStart">${running ? t('Открыть экран регулировки') : t('Запустить демонстрацию')}</button>
      ${running ? `<button class="btn" id="dRestart">${t('Начать заново')}</button><button class="btn danger" id="dStop">${t('Остановить')}</button>` : ''}
    </div>
  </div>`));
  $('#dStart', body).onclick = async () => {
    try {
      if (!running) await api('/api/sim', { method: 'POST', body: { action: 'start' } });
      await refreshSession();
      go('live');
    } catch (e) { toast(e.message, true); }
  };
  const r = $('#dRestart', body), s = $('#dStop', body);
  if (r) r.onclick = () => api('/api/sim', { method: 'POST', body: { action: 'restart' } })
    .then(() => toast(t('Машина снова разрегулирована'))).catch(e => toast(e.message, true));
  if (s) s.onclick = async () => {
    try { await api('/api/sim', { method: 'POST', body: { action: 'stop' } }); await refreshSession(); } catch (e) { toast(e.message, true); }
    renderBody();
  };
}
