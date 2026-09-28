// Шаг 3: замер — выбор способа и ввод данных.

import { state, on, api, save, toast, refreshSession, h, $, $$ } from '../state.js';
import { WHEELS, esc, fmtParam } from '../fmt.js';
import { t } from '../i18n.js';
import { go, snapshot } from '../app.js';
import * as optical from './optical.js';
import * as phone from './phone.js';

let root, method = localStorageGet('method', 'manual');

function localStorageGet(k, d) { try { return JSON.parse(localStorage.getItem('wa.' + k)) ?? d; } catch { return d; } }

const METHODS = () => [
  { id: 'manual', name: t('Струна и угломер'), desc: t('Леска вдоль бортов, рулетка и угломер. Стоит как один заезд на СТО.'), gets: t('развал · схождение · кастер (с поворотными кругами)') },
  { id: 'phone', name: t('Телефон на колесе'), desc: t('Телефон прикладывается к ободу и передаёт углы по Wi-Fi в реальном времени.'), gets: t('развал вживую · кастер с гироскопом') },
  { id: 'optical', name: t('Камера и мишени'), desc: t('Печатные шахматные мишени на колёсах и фотоаппарат или телефон.'), gets: t('развал · схождение · угол тяги · кастер'), beta: true },
  { id: 'sensor', name: t('Свой датчик'), desc: t('Самодельная голова на ESP32 или любой прибор, умеющий отправить JSON.'), gets: t('что умеет датчик') },
  { id: 'demo', name: t('Демонстрация'), desc: t('Учебная машина: всё как на настоящей, но без машины.'), gets: t('все углы') },
];

export function mount(el) {
  root = el;
  root.innerHTML = `
    <h1>${t('Замер')}</h1>
    <p class="lede">${t('Выберите, чем будете мерить. Данные сразу попадают на экран «Регулировка» — там видно, что в допуске, а что нет, и стрелки двигаются, пока вы крутите регулировку.')}</p>
    <div class="method-cards" id="mMethods"></div>
    <div id="mBody"></div>
    <div class="actions" id="mActions"></div>`;
  on('frame', updateMini);
  on('session', () => { if (state.screen === 'measure') renderActions(); });
}

export function show() {
  renderMethods();
  renderBody();
  renderActions();
}

function renderMethods() {
  const box = $('#mMethods', root);
  box.innerHTML = METHODS().map(m => `
    <button class="method${m.id === method ? ' on' : ''}" data-m="${m.id}">
      <b>${m.name}${m.beta ? ` <span class="badge unverified">${t('бета')}</span>` : ''}</b>
      <span>${m.desc}</span>
      <span class="gets">${m.gets}</span>
    </button>`).join('');
  $$('.method', box).forEach(b => b.onclick = () => {
    method = b.dataset.m;
    try { localStorage.setItem('wa.method', JSON.stringify(method)); } catch { /* без сохранения */ }
    renderMethods();
    renderBody();
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
  if (method === 'manual') manual(body);
  else if (method === 'phone') phone.render(body);
  else if (method === 'optical') optical.render(body);
  else if (method === 'sensor') sensor(body);
  else demo(body);
  updateMini();
}

// ── Струна и угломер ─────────────────────────────────────────────────

const manualData = localStorageGet('manual', {});

function manual(body) {
  const ss = state.session || {};
  body.append(h(`<div>
    <div class="panel">
      <h2 style="margin-top:0">${t('Как мерить')}</h2>
      <ol class="plain" style="padding-left:20px;margin:0">
        <li>${t('Развал — угломер на обод (не на резину), «плюс», когда верх колеса наклонён наружу. Затем прокатите машину на пол-оборота колеса и снова приложите угломер к тому же месту обода — программа усреднит и уберёт биение диска.')}</li>
        <li>${t('Схождение — от струны до края обода спереди и сзади колеса, строго на высоте центра ступицы.')}</li>
        <li>${t('Кастер (по желанию, нужны поворотные круги) — развал при повороте колеса на 20° наружу и 20° внутрь.')}</li>
      </ol>
      <p class="muted" style="margin-top:10px;font-size:13px">${t('Каждое введённое число сразу уходит на экран регулировки. Меряете после поворота регулятора — вводите новое число, стрелка сдвинется.')}</p>
    </div>

    <details class="panel" id="boxCheck" style="margin-top:12px" open>
      <summary style="cursor:pointer;font-weight:600">${t('Проверка установки струн')} <span class="muted" style="font-weight:400">${t('— самая частая причина ошибки')}</span></summary>
      <p class="muted" style="margin-top:10px;font-size:13px">${t('Совет «сделайте отступы спереди и сзади одинаковыми» верен только при равных колеях. Введите расстояние от струны до центра ступицы на каждом колесе — программа скажет, как сдвинуть концы струн.')}
        ${t('Колеи — на шаге «Автомобиль» (сейчас: перед {f} мм, зад {r} мм).', { f: ss.track_front_mm || '—', r: ss.track_rear_mm || '—' })}</p>
      <div class="cols4">
        <label class="f">${t('Левая, у переднего')}<input type="number" step="0.5" data-box="lf"></label>
        <label class="f">${t('Левая, у заднего')}<input type="number" step="0.5" data-box="lr"></label>
        <label class="f">${t('Правая, у переднего')}<input type="number" step="0.5" data-box="rf"></label>
        <label class="f">${t('Правая, у заднего')}<input type="number" step="0.5" data-box="rr"></label>
      </div>
      <div id="boxHint" style="margin-top:10px"></div>
    </details>

    <div class="wheelgrid" id="mWheels" style="margin-top:12px"></div>
  </div>`));

  const grid = $('#mWheels', body);
  for (const w of WHEELS) {
    const d = manualData[w.key] || {};
    const v = f => (d[f] ?? '') === '' ? '' : d[f];
    grid.append(h(`<div class="wcard" data-w="${w.key}">
      <h3><span>${w.name}</span><span class="live-mini" data-mini="${w.key}"></span></h3>
      <div class="pair">
        <label class="f">${t('Развал, °')}<input type="number" step="0.01" data-f="camber" value="${v('camber')}"></label>
        <label class="f">${t('Он же через ½ оборота, °')}<input type="number" step="0.01" data-f="camber_180" value="${v('camber_180')}"></label>
      </div>
      <div class="pair">
        <label class="f">${t('Струна → обод спереди, мм')}<input type="number" step="0.1" data-f="toe_front_mm" value="${v('toe_front_mm')}"></label>
        <label class="f">${t('Струна → обод сзади, мм')}<input type="number" step="0.1" data-f="toe_rear_mm" value="${v('toe_rear_mm')}"></label>
      </div>
      <label class="chk" style="font-size:13px;color:var(--muted)"><input type="checkbox" data-f="invert_gauge"${d.invert_gauge ? ' checked' : ''}>
        <span>${t('Угломер показывает наоборот (наклон наружу — минус)')}</span></label>
      ${w.front ? `
      <details style="margin-top:10px">
        <summary style="cursor:pointer;color:var(--blue);font-size:13.5px">${t('Кастер — развал при повороте колеса')}</summary>
        <div class="pair" style="margin-top:8px">
          <label class="f">${t('Колесо повёрнуто НАРУЖУ, °')}<input type="number" step="0.01" data-f="sweep_out" value="${v('sweep_out')}"></label>
          <label class="f">${t('Колесо повёрнуто ВНУТРЬ, °')}<input type="number" step="0.01" data-f="sweep_in" value="${v('sweep_in')}"></label>
        </div>
        <label class="f">${t('На сколько градусов поворачивали в каждую сторону')}<input type="number" step="1" data-f="half_sweep" value="${v('half_sweep') || 20}"></label>
        <small class="dim">${t('«Наружу» — от центра машины. Поворот руля влево даёт «наружу» для левого колеса и «внутрь» для правого.')}</small>
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

  const boxSaved = localStorageGet('box', {});
  $$('[data-box]', body).forEach(inp => {
    inp.value = boxSaved[inp.dataset.box] ?? '';
    inp.oninput = () => { boxSaved[inp.dataset.box] = inp.value; save('box', boxSaved); boxCheck(body); };
  });
  boxCheck(body);
}

function num(card, f) {
  const el = card.querySelector(`[data-f="${f}"]`);
  if (!el || el.value.trim() === '') return null;
  const v = parseFloat(el.value.replace(',', '.'));
  return Number.isFinite(v) ? v : null;
}

async function sendWheel(card) {
  const w = card.dataset.w;
  const d = {
    camber: num(card, 'camber'), camber_180: num(card, 'camber_180'),
    toe_front_mm: num(card, 'toe_front_mm'), toe_rear_mm: num(card, 'toe_rear_mm'),
    sweep_out: num(card, 'sweep_out'), sweep_in: num(card, 'sweep_in'), half_sweep: num(card, 'half_sweep'),
    invert_gauge: card.querySelector('[data-f="invert_gauge"]').checked,
  };
  manualData[w] = d;
  save('manual', manualData);

  const body = { wheel: w, invert_gauge: d.invert_gauge };
  if (d.camber !== null) body.camber = d.camber;
  if (d.camber !== null && d.camber_180 !== null) body.camber_180 = d.camber_180;
  if (d.toe_front_mm !== null && d.toe_rear_mm !== null) {
    body.toe_front_mm = d.toe_front_mm;
    body.toe_rear_mm = d.toe_rear_mm;
  }
  if (d.sweep_out !== null && d.sweep_in !== null) {
    body.sweep = { camber_out: d.sweep_out, camber_in: d.sweep_in, half_sweep_deg: d.half_sweep || 20 };
  }
  if (Object.keys(body).length <= 2 && !body.sweep) return;
  try {
    await api('/api/live/manual', { method: 'POST', body });
  } catch (e) { toast(`${w}: ${e.message}`, true); }
}

function boxCheck(body) {
  const ss = state.session || {};
  const g = k => { const v = parseFloat($(`[data-box="${k}"]`, body).value); return Number.isFinite(v) ? v : null; };
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
      <li>${t('F10 «Показать регулировку» — программа сама приведёт углы в допуск в правильном порядке.')}</li>
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
  if (r) r.onclick = () => api('/api/sim', { method: 'POST', body: { action: 'restart' } }).then(() => toast(t('Машина снова разрегулирована')));
  if (s) s.onclick = async () => { await api('/api/sim', { method: 'POST', body: { action: 'stop' } }); await refreshSession(); renderBody(); };
}
