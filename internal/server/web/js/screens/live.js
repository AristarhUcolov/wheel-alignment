// Экран «Регулировка»: живые показания, как на стенде.

import { state, on, emit, api, save, toast, h, $, $$ } from '../state.js';
import { ValueBox, CarView, camberPicto, casterPicto, toePicto } from '../widgets.js';
import { WHEELS, fmtParam, fmtRangeParam, specValue, shortLabel, paramKind, paramWheel, esc, statusText, inches, fmtDM } from '../fmt.js';
import { t, lang } from '../i18n.js';
import { openDrawer, closeDrawer, go, drawerKey } from '../app.js';

let root, body, boxes = {}, car = null, pictos = {}, crossEls = {};

const VIEWS = () => [
  { id: 'overview', name: t('Общий вид') },
  { id: 'front', name: t('Передняя ось') },
  { id: 'rear', name: t('Задняя ось') },
];

export function mount(el) {
  root = el;
  root.innerHTML = `
    <div class="live-head">
      <div class="verdict" id="verdict"><span class="dot"></span><span class="txt">${t('Ждём показаний…')}</span><small id="verdictSub"></small></div>
      <div class="units" id="units">
        <button data-u="dm" title="${esc(t('Градусы и минуты'))}">° ′</button>
        <button data-u="deg" title="${esc(t('Десятичные градусы'))}">0.00°</button>
        <button data-u="mm" title="${esc(t('Схождение в миллиметрах'))}">${t('мм')}</button>
      </div>
      <div class="units" id="views">${VIEWS().map(v => `<button data-v="${v.id}">${v.name}</button>`).join('')}</div>
    </div>
    <div class="live-body" id="liveBody"></div>`;
  body = $('#liveBody', root);

  $$('#units button', root).forEach(b => b.onclick = () => setUnits(b.dataset.u));
  $$('#views button', root).forEach(b => b.onclick = () => setView(b.dataset.v));

  on('frame', render);
  on('session', () => { build(); drawerBuiltFor = null; render(); });
  build();
}

export function show() {
  if (car) car.revive();
  render();
}

export function setUnits(u) {
  state.units = u;
  save('units', u);
  render();
  emit('units', u);
}

export function setView(v) {
  state.liveView = v;
  save('liveView', v);
  build();
  render();
  emit('fkeys');
}

export function cycleView() {
  const views = VIEWS();
  const i = views.findIndex(v => v.id === state.liveView);
  setView(views[(i + 1) % views.length].id);
}

function pick(key) {
  state.selected = key;
  drawerBuiltFor = null;
  openParam(key);
  render();
}

// ── Построение вида ──────────────────────────────────────────────────

function build() {
  if (!body) return;
  boxes = {};
  pictos = {};
  crossEls = {};
  body.innerHTML = '';
  $$('#views button', root).forEach(b => b.classList.toggle('on', b.dataset.v === state.liveView));

  const mkBox = (key, opts) => {
    const b = new ValueBox(key, opts);
    b.el.onclick = () => pick(key);
    boxes[key] = b;
    return b.el;
  };

  if (state.liveView === 'overview') {
    const ov = h(`<div class="ov"><div class="ov-side" id="ovL"></div><div class="ov-center" id="ovC"></div><div class="ov-side" id="ovR"></div></div>`);
    for (const w of WHEELS) {
      const card = h(`<div class="ov-wheel"><h4><span>${w.name}</span><span class="src dim"></span></h4></div>`);
      card.dataset.wheel = w.key;
      if (w.front) card.append(mkBox('caster_' + w.key, { small: true }));
      card.append(mkBox('camber_' + w.key));
      card.append(mkBox('toe_' + w.key));
      $(w.left ? '#ovL' : '#ovR', ov).append(card);
    }
    car = new CarView({ onPick: pick });
    $('#ovC', ov).append(car.el);
    body.append(ov);
  } else {
    car = null;
    const front = state.liveView === 'front';
    const [L, R] = front ? ['FL', 'FR'] : ['RL', 'RR'];
    const ax = h(`<div class="axv">
      <div class="axv-head"><div>${front ? t('Переднее левое') : t('Заднее левое')}</div><div>${front ? t('Передняя ось') : t('Задняя ось')}</div><div>${front ? t('Переднее правое') : t('Заднее правое')}</div></div>
    </div>`);
    const cross = t('Δ лев − прав'), sum = t('Σ суммарное');
    const rows = front
      ? [['caster', t('Кастер'), casterPicto, 'front_cross_caster', cross],
         ['camber', t('Развал'), camberPicto, 'front_cross_camber', cross],
         ['toe', t('Схождение'), toePicto, 'front_total_toe', sum]]
      : [['camber', t('Развал'), camberPicto, 'rear_cross_camber', cross],
         ['toe', t('Схождение'), toePicto, 'rear_total_toe', sum]];
    for (const [kind, name, picto, crossKey, xl] of rows) {
      ax.append(mkBox(`${kind}_${L}`, { full: true, label: name }));
      const mid = h(`<div class="axv-mid"><div class="name">${name}</div><div class="pic"></div><div class="cross none">— — —</div><div class="xl">${xl}</div></div>`);
      const p = picto();
      mid.querySelector('.pic').append(p);
      pictos[kind] = p;
      crossEls[crossKey] = mid.querySelector('.cross');
      mid.onclick = () => pick(crossKey);
      ax.append(mid);
      ax.append(mkBox(`${kind}_${R}`, { full: true, label: name }));
    }
    if (front) {
      const sai = t('Попер. наклон оси (SAI)');
      ax.append(mkBox('sai_FL', { small: true, label: sai }));
      ax.append(h(`<div class="axv-mid" style="cursor:default"><div class="xl">${t('SAI из замера с поворотом — только для сравнения бортов')}</div></div>`));
      ax.append(mkBox('sai_FR', { small: true, label: sai }));
    } else {
      ax.append(h('<div></div>'));
      const mid = h(`<div class="axv-mid"><div class="name">${t('Угол тяги')}</div><div class="cross none">— — —</div><div class="xl">${t('куда направлена задняя ось')}</div></div>`);
      crossEls.thrust_angle = mid.querySelector('.cross');
      mid.onclick = () => pick('thrust_angle');
      ax.append(mid);
      ax.append(h('<div></div>'));
    }
    body.append(ax);
  }

  body.append(h(`<div class="live-empty hidden" id="liveEmpty">
    <div class="panel">
      <h2 style="margin-top:0">${t('Показаний пока нет')}</h2>
      <p class="muted">${t('Экран оживёт, как только придут данные: с телефона на колесе, со своего датчика, с камеры — или когда вы введёте замеры струной и угломером.')}</p>
      <div class="actions" style="justify-content:center">
        <button class="btn primary" data-go="measure">${t('Выбрать способ замера')}</button>
        <button class="btn" data-sim="start">${t('Запустить демонстрацию')}</button>
      </div>
    </div></div>`));
  $('[data-go]', body).onclick = () => go('measure');
  $('[data-sim]', body).onclick = () => api('/api/sim', { method: 'POST', body: { action: 'start' } })
    .then(() => { refreshSim(true); toast(t('Демонстрация: стрелками ← → можно «крутить» выбранный угол')); })
    .catch(e => toast(e.message, true));
}

// ── Обновление ───────────────────────────────────────────────────────

function render() {
  if (!body) return;
  const f = state.frame;
  const ss = state.session;
  const rimMM = ss ? inches(ss.rim_diameter_in || 0) : 0;
  $$('#units button', root).forEach(b => b.classList.toggle('on', b.dataset.u === state.units));
  const P = f ? f.params : {};

  for (const [key, box] of Object.entries(boxes)) {
    box.update(P[key], state.units, rimMM);
    box.el.classList.toggle('sel', state.selected === key);
  }
  if (car) car.update(f, state.units, rimMM);

  if (pictos.camber) pictos.camber.update(val(P, state.liveView === 'front' ? 'camber_FL' : 'camber_RL'), val(P, state.liveView === 'front' ? 'camber_FR' : 'camber_RR'));
  if (pictos.toe) pictos.toe.update(val(P, state.liveView === 'front' ? 'toe_FL' : 'toe_RL'), val(P, state.liveView === 'front' ? 'toe_FR' : 'toe_RR'));
  if (pictos.caster) pictos.caster.update(val(P, 'caster_FL'));
  for (const [key, el] of Object.entries(crossEls)) {
    const p = P[key];
    el.textContent = fmtParam(key, p, state.units);
    el.className = 'cross ' + (!p || !p.has ? 'none' : p.status === 'no_spec' ? '' : p.status);
  }

  // Источник данных по колёсам в общем виде.
  $$('.ov-wheel', body).forEach(card => {
    const w = card.dataset.wheel;
    const p = P['camber_' + w] || {};
    const tt = P['toe_' + w] || {};
    const src = [p.src, tt.src].filter(Boolean);
    card.querySelector('.src').textContent = [...new Set(src)].map(srcName).join(', ');
  });

  // Итог.
  const v = $('#verdict', root);
  const sub = $('#verdictSub', root);
  let cls = '', txt = t('Ждём показаний…'), subtxt = '';
  if (f && f.measured > 0) {
    if (!f.has_limits) { txt = t('Допуски не заданы — показаны только измеренные углы'); }
    else if (f.overall === 'good') { cls = 'good'; txt = t('Все измеренные углы в допуске'); }
    else if (f.overall === 'marginal') { cls = 'marginal'; txt = t('В допуске, но у границы'); }
    else if (f.overall === 'bad') { cls = 'bad'; txt = t('Вне допуска: {n}', { n: f.out_of_spec }); }
    subtxt = f.all_stable ? t('показания стабильны') : t('показания меняются…');
    if (f.measured > 0 && !f.thrust_referenced && (P.toe_FL?.has || P.toe_FR?.has)) {
      subtxt += ' · ' + t('схождение от оси машины (задняя ось не измерена)');
    }
  }
  if (f && !state.connected) { cls = ''; txt = t('Нет связи с программой — переподключаюсь…'); }
  v.className = 'verdict ' + cls;
  v.querySelector('.txt').textContent = txt;
  sub.textContent = subtxt;

  const empty = $('#liveEmpty', body);
  if (empty) empty.classList.toggle('hidden', !!(f && f.measured > 0));

  if (drawerKey() && state.selected) openParam(state.selected, true);
}

const val = (P, k) => (P[k] && P[k].has ? P[k].v : 0);

function srcName(id) {
  if (!id) return '';
  if (id === 'sim') return t('демо');
  if (id === 'manual') return t('вручную');
  if (id.startsWith('phone')) return t('телефон');
  if (id.startsWith('sensor:')) return t('датчик');
  if (id === 'optical') return t('камера');
  return id;
}

export async function refreshSim(running) {
  if (state.session) {
    state.session.sim_running = running;
    emit('fkeys');
  }
}

// ── Боковая панель: как регулировать выбранный угол ─────────────────

const EXPLAIN = {
  camber: () => t('Развал — наклон колеса поперёк машины. «Плюс» — верх колеса наружу. Разный развал слева и справа тянет машину в сторону большего плюса; большой минус съедает внутренний край шины, большой плюс — наружный.'),
  toe: () => t('Схождение — поворот колеса в плане. «Плюс» — передний край колеса повёрнут к центру машины. Главный угол для износа шин: лишние десять минут стирают протектор «пилой» за сезон.'),
  caster: () => t('Кастер (продольный наклон оси поворота, у шкворневых машин — шкворня). «Плюс» — верх оси назад. Он возвращает руль в прямое положение; разница слева и справа тянет машину в сторону меньшего кастера.'),
  sai: () => t('Поперечный наклон оси поворота. Задан конструкцией и не регулируется. Из замера с поворотом он получается неточным — пользуйтесь им только для сравнения бортов: большая разница означает погнутую деталь.'),
  front_total_toe: () => t('Сумма схождения обоих передних колёс. Её легко получить неправильно распределённой — и тогда руль стоит криво. Поэтому выставляйте схождение каждого колеса, а не только сумму.'),
  rear_total_toe: () => t('Сумма схождения задних колёс. У неразрезного моста и балки не регулируется — отклонение говорит о деформации.'),
  front_cross_camber: () => t('Разница развала левого и правого колеса. Именно она, а не сами значения, чаще всего уводит машину в сторону.'),
  rear_cross_camber: () => t('Разница развала задних колёс. У моста и балки — признак деформации.'),
  front_cross_caster: () => t('Разница кастера левого и правого колеса. Машину тянет в сторону колеса с меньшим кастером.'),
  thrust_angle: () => t('Угол тяги — куда направлена задняя ось относительно оси симметрии машины. Если он не ноль, машина едет чуть боком, а руль при езде прямо стоит криво. Переднее схождение программа отсчитывает от линии тяги, чтобы руль встал ровно.'),
};

function suspAdvice(key) {
  const ss = state.session;
  if (!ss) return null;
  const kind = paramKind(key);
  const w = paramWheel(key);
  const front = w ? w.front : key.startsWith('front');
  const susp = front ? ss.front_suspension : ss.rear_suspension;
  if (key === 'thrust_angle') return { susp: ss.rear_suspension, adj: ss.rear_suspension.toe };
  const field = kind.includes('camber') ? 'camber' : kind.includes('caster') ? 'caster' : kind.includes('toe') ? 'toe' : null;
  if (!field) return { susp, adj: null };
  return { susp, adj: susp[field] };
}

// Помощник регулировки: сколько градусов даёт один шаг регулятора на этой
// машине, узнаётся из первого же шага — дальше программа считает, сколько
// ещё крутить и в какую сторону. Схема из руководства не нужна.
const helper = (() => { try { return JSON.parse(localStorage.getItem('wa.helper')) || {}; } catch { return {}; } })();
const helperKey = key => ((state.session && state.session.vehicle && state.session.vehicle.id) || '-') + '|' + key;
function saveHelper() { try { localStorage.setItem('wa.helper', JSON.stringify(helper)); } catch { /* без сохранения */ } }

function quarters(n) {
  const q = Math.round(Math.abs(n) * 4);
  const whole = Math.floor(q / 4), frac = ['', '¼', '½', '¾'][q % 4];
  return (whole ? String(whole) : '') + frac || '0';
}

// «¼ оборота», «1 оборот», «2 оборота», «5 оборотов» — по-русски; по-английски
// «turn(s)».
function stepWord(key, n = 0.5) {
  const toe = paramKind(key) === 'toe';
  const forms = toe
    ? [t('оборот тяги'), t('оборота тяги'), t('оборотов тяги')]
    : [t('шаг регулятора'), t('шага регулятора'), t('шагов регулятора')];
  const q = Math.round(Math.abs(n) * 4) / 4;
  if (lang === 'en') return q === 1 ? forms[0] : forms[1];
  if (q % 1) return forms[1];
  const k = q % 100, d = q % 10;
  if (k >= 11 && k <= 14) return forms[2];
  if (d === 1) return forms[0];
  if (d >= 2 && d <= 4) return forms[1];
  return forms[2];
}

let drawerBuiltFor = null;

// openParam строит панель один раз; дальше updateParam меняет только цифры —
// иначе перестройка 12 раз в секунду съедала бы нажатия на кнопки.
export function openParam(key, refresh = false) {
  if (refresh && drawerBuiltFor === key) { updateParam(key); return; }
  const ss = state.session;
  const info = ss && ss.params ? ss.params[key] : null;
  const kind = paramKind(key);
  const perWheel = !!paramWheel(key);

  const simAdjustable = ss && ss.sim_running && ['camber', 'toe', 'caster'].includes(kind) && perWheel;
  const adv = suspAdvice(key);
  const method = info && info.method;
  let adjust = '';
  if (method) adjust += `<p><b>${t('Для этой модели:')}</b> ${esc(method)}</p>`;
  if (adv && adv.adj && adv.adj.how) {
    adjust += `<p><b>${esc(adv.susp.name)}:</b> ${esc(adv.adj.how)}</p>`;
    if (adv.adj.tip) adjust += `<p class="note">${esc(adv.adj.tip)}</p>`;
  }
  const notAdjustable = info && !info.adjustable && ['camber', 'toe', 'caster', 'thrust_angle'].includes(kind) && (adv && adv.adj && !adv.adj.usual);
  const canHelp = perWheel && ['camber', 'toe', 'caster'].includes(kind);
  const explain = EXPLAIN[kind];

  const html = `
    <h2>${esc(info ? info.label : shortLabel(key))}</h2>
    <div class="muted" id="dStatus"></div>
    <div class="big" id="dBig"></div>
    <div class="target" id="dTarget"></div>
    <div class="togo" id="dTogo"></div>
    ${simAdjustable ? `
      <div class="sect"><h3>${t('Демонстрация: крутим регулировку')}</h3>
        <div class="adj-btns">
          <button class="btn" data-adj="-0.25">−0°15′</button><button class="btn" data-adj="-0.05">−0°03′</button>
          <button class="btn" data-adj="0.05">+0°03′</button><button class="btn" data-adj="0.25">+0°15′</button>
        </div>
        <p class="muted" style="text-align:center;font-size:12.5px">${t('Или стрелками ← → на клавиатуре (с Shift — крупнее). Обратите внимание: кастер и развал тянут за собой схождение.')}</p>
      </div>` : ''}
    ${canHelp ? `
      <div class="sect"><h3>${t('Помощник: сколько крутить')}</h3>
        <p class="muted" style="font-size:13px">${t('Нажмите «Запомнить», поверните регулятор на известную величину, дождитесь «стабильно» и нажмите, на сколько повернули. Программа узнает, сколько даёт шаг на вашей машине, и дальше будет подсказывать, сколько ещё крутить.')}</p>
        <div id="dHelp"></div>
        <div class="adj-btns" style="flex-wrap:wrap">
          <button class="btn small" data-h="mark">${t('Запомнить положение')}</button>
          <button class="btn small" data-h="0.25">¼</button><button class="btn small" data-h="0.5">½</button>
          <button class="btn small" data-h="1">1</button><button class="btn small" data-h="2">2</button>
          <button class="btn small danger" data-h="forget">${t('Забыть')}</button>
        </div>
        <p class="dim" style="text-align:center;font-size:12px">${paramKind(key) === 'toe' ? t('¼, ½, 1, 2 — на сколько оборотов тяги вы повернули') : t('¼, ½, 1, 2 — на сколько шагов регулятора вы повернули')}</p>
      </div>` : ''}
    ${explain ? `<div class="sect"><h3>${t('Что это')}</h3><p>${esc(explain())}</p></div>` : ''}
    ${adjust ? `<div class="sect"><h3>${t('Чем регулируется')}</h3>${adjust}</div>` : ''}
    ${notAdjustable ? `<div class="warn">${t('На этой конструкции угол штатно не регулируется. Если он вне допуска — ищите износ или деформацию, а не регулировочный болт.')}</div>` : ''}
    <p class="dim" id="dSrc" style="margin-top:14px;font-size:12px"></p>`;

  openDrawer(html, key, refresh);
  drawerBuiltFor = key;
  const b = $('#drawerBody');
  $$('[data-adj]', b).forEach(x => x.onclick = () => simAdjust(key, parseFloat(x.dataset.adj)));
  $$('[data-h]', b).forEach(x => x.onclick = () => helperAction(key, x.dataset.h));
  updateParam(key);
}

function helperAction(key, what) {
  const p = state.frame && state.frame.params[key];
  const hk = helperKey(key);
  const hs = helper[hk] || {};
  if (what === 'forget') { delete helper[hk]; saveHelper(); updateParam(key); return; }
  if (!p || !p.has) { toast(t('Нет показаний по этому углу'), true); return; }
  if (what === 'mark') {
    hs.base = p.v;
    helper[hk] = hs;
    saveHelper();
    toast(t('Запомнено. Поверните регулятор и нажмите, на сколько.'));
  } else {
    if (hs.base === undefined) { toast(t('Сначала нажмите «Запомнить положение», потом поворачивайте'), true); return; }
    if (!p.stable) { toast(t('Дождитесь, пока показания успокоятся'), true); return; }
    const turns = parseFloat(what);
    const delta = p.v - hs.base;
    if (Math.abs(delta) < 0.005) { toast(t('Угол почти не изменился — поверните сильнее или проверьте, тот ли регулятор'), true); return; }
    hs.rate = delta / turns;
    hs.base = p.v;
    helper[hk] = hs;
    saveHelper();
    toast(paramKind(key) === 'toe'
      ? t('Один оборот даёт {v}', { v: fmtDM(Math.abs(hs.rate), { sign: false }) })
      : t('Один шаг даёт {v}', { v: fmtDM(Math.abs(hs.rate), { sign: false }) }));
  }
  updateParam(key);
}

function updateParam(key) {
  const b = $('#drawerBody');
  if (!b || drawerBuiltFor !== key) return;
  const f = state.frame;
  const ss = state.session;
  const p = f && f.params[key];
  const units = state.units;
  const rimMM = ss ? inches(ss.rim_diameter_in || 0) : 0;
  const fmt = d => specValue(key, d, units, rimMM);
  const status = !p || !p.has ? 'none' : p.status;
  const set = (id, v) => { const el = $(id, b); if (el) el.innerHTML = v; };

  set('#dStatus', p && p.has ? esc(statusText(p.status)) + ' · ' + (p.stable ? t('показания стабильны') : t('показания меняются')) : t('нет показаний'));
  const big = $('#dBig', b);
  big.className = 'big ' + status;
  big.textContent = fmtParam(key, p, units);
  const target = $('#dTarget', b);
  target.className = 'target' + (p && p.spec ? '' : ' muted');
  target.textContent = p && p.spec ? t('норма {range} · номинал {nom}', { range: fmtRangeParam(key, p.spec, units, rimMM), nom: fmt(p.spec.nominal) }) : t('допуск не задан');

  const togo = $('#dTogo', b);
  if (p && p.has && p.spec) {
    const d = p.to_nominal;
    const arrow = Math.abs(d) < 1e-4 ? '' : d > 0
      ? '<svg viewBox="0 0 24 24"><path d="M12 3 L21 14 H15 V21 H9 V14 H3 Z" fill="currentColor"/></svg>'
      : '<svg viewBox="0 0 24 24"><path d="M12 21 L21 10 H15 V3 H9 V10 H3 Z" fill="currentColor"/></svg>';
    togo.style.display = '';
    togo.style.color = p.status === 'bad' ? 'var(--red)' : 'var(--green)';
    togo.innerHTML = `${arrow}<span>${t('до номинала {v}', { v: fmt(d) })}</span>`;
  } else togo.style.display = 'none';

  const help = $('#dHelp', b);
  if (help) {
    const hs = helper[helperKey(key)] || {};
    let txt = '';
    if (hs.base !== undefined && p && p.has) {
      txt += `<p class="num" style="text-align:center">${t('запомнено {base} · сейчас {now} · сдвиг {d}', { base: fmtDM(hs.base), now: fmtDM(p.v), d: fmtDM(p.v - hs.base) })}</p>`;
    }
    if (hs.rate && p && p.has && p.spec) {
      const n = p.to_nominal / hs.rate;
      const one = paramKind(key) === 'toe'
        ? t('Один оборот тяги ≈ {v}.', { v: fmtDM(Math.abs(hs.rate), { sign: false }) })
        : t('Один шаг ≈ {v}.', { v: fmtDM(Math.abs(hs.rate), { sign: false }) });
      const rest = Math.abs(n) < 0.125
        ? `<b>${t('Вы на номинале — дальше не крутите.')}</b>`
        : `<b>${t('До номинала ≈ {n} {word}', { n: quarters(n), word: stepWord(key, n) })}</b> — ${n > 0 ? t('в ту же сторону, что и прошлый поворот') : t('в сторону, обратную прошлому повороту')}.`;
      txt += `<div class="ok-box" style="margin:6px 0">${one}<br>${rest}</div>`;
    }
    help.innerHTML = txt;
  }
  set('#dSrc', p && p.src ? t('Источник: {s}', { s: esc(srcName(p.src)) }) : '');
}

export function simAdjust(key, delta) {
  return api('/api/sim', { method: 'POST', body: { action: 'adjust', key, delta } })
    .catch(e => toast(e.message, true));
}

export function selectedKey() { return state.selected; }

export function closeParam() {
  state.selected = null;
  drawerBuiltFor = null;
  closeDrawer();
  render();
}
