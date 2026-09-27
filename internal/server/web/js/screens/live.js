// Экран «Регулировка»: живые показания, как на стенде.

import { state, on, emit, api, save, toast, h, $, $$ } from '../state.js';
import { ValueBox, CarView, camberPicto, casterPicto, toePicto, colorOf } from '../widgets.js';
import { WHEELS, fmtParam, fmtRangeParam, specValue, shortLabel, paramKind, paramWheel, esc, STATUS_RU, inches } from '../fmt.js';
import { openDrawer, closeDrawer, go, drawerKey } from '../app.js';

let root, body, boxes = {}, car = null, pictos = {}, crossEls = {};

const VIEWS = [
  { id: 'overview', name: 'Общий вид' },
  { id: 'front', name: 'Передняя ось' },
  { id: 'rear', name: 'Задняя ось' },
];

export function mount(el) {
  root = el;
  root.innerHTML = `
    <div class="live-head">
      <div class="verdict" id="verdict"><span class="dot"></span><span class="txt">Ждём показаний…</span><small id="verdictSub"></small></div>
      <div class="units" id="units">
        <button data-u="dm" title="Градусы и минуты">° ′</button>
        <button data-u="deg" title="Десятичные градусы">0.00°</button>
        <button data-u="mm" title="Схождение в миллиметрах">мм</button>
      </div>
      <div class="units" id="views">${VIEWS.map(v => `<button data-v="${v.id}">${v.name}</button>`).join('')}</div>
    </div>
    <div class="live-body" id="liveBody"></div>`;
  body = $('#liveBody', root);

  $$('#units button', root).forEach(b => b.onclick = () => setUnits(b.dataset.u));
  $$('#views button', root).forEach(b => b.onclick = () => setView(b.dataset.v));

  on('frame', render);
  on('session', () => { build(); render(); });
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
  const i = VIEWS.findIndex(v => v.id === state.liveView);
  setView(VIEWS[(i + 1) % VIEWS.length].id);
}

function pick(key) {
  state.selected = key;
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
      <div class="axv-head"><div>${front ? 'Переднее левое' : 'Заднее левое'}</div><div>${front ? 'Передняя ось' : 'Задняя ось'}</div><div>${front ? 'Переднее правое' : 'Заднее правое'}</div></div>
    </div>`);
    const rows = front
      ? [['caster', 'Кастер', casterPicto, 'front_cross_caster', 'Δ лев − прав'],
         ['camber', 'Развал', camberPicto, 'front_cross_camber', 'Δ лев − прав'],
         ['toe', 'Схождение', toePicto, 'front_total_toe', 'Σ суммарное']]
      : [['camber', 'Развал', camberPicto, 'rear_cross_camber', 'Δ лев − прав'],
         ['toe', 'Схождение', toePicto, 'rear_total_toe', 'Σ суммарное']];
    for (const [kind, name, picto, cross, xl] of rows) {
      ax.append(mkBox(`${kind}_${L}`, { full: true, label: name }));
      const mid = h(`<div class="axv-mid"><div class="name">${name}</div><div class="pic"></div><div class="cross none">— — —</div><div class="xl">${xl}</div></div>`);
      const p = picto();
      mid.querySelector('.pic').append(p);
      pictos[kind] = p;
      crossEls[cross] = mid.querySelector('.cross');
      mid.onclick = () => pick(cross);
      ax.append(mid);
      ax.append(mkBox(`${kind}_${R}`, { full: true, label: name }));
    }
    if (front) {
      ax.append(mkBox('sai_FL', { small: true, label: 'Попер. наклон оси (SAI)' }));
      ax.append(h(`<div class="axv-mid" style="cursor:default"><div class="xl">SAI из замера с поворотом — только для сравнения бортов</div></div>`));
      ax.append(mkBox('sai_FR', { small: true, label: 'Попер. наклон оси (SAI)' }));
    } else {
      ax.append(h('<div></div>'));
      const mid = h(`<div class="axv-mid"><div class="name">Угол тяги</div><div class="cross none">— — —</div><div class="xl">куда направлена задняя ось</div></div>`);
      crossEls.thrust_angle = mid.querySelector('.cross');
      mid.onclick = () => pick('thrust_angle');
      ax.append(mid);
      ax.append(h('<div></div>'));
    }
    body.append(ax);
  }

  body.append(h(`<div class="live-empty hidden" id="liveEmpty">
    <div class="panel">
      <h2 style="margin-top:0">Показаний пока нет</h2>
      <p class="muted">Экран оживёт, как только придут данные: с телефона на колесе, со своего датчика,
        с камеры — или когда вы введёте замеры струной и угломером.</p>
      <div class="actions" style="justify-content:center">
        <button class="btn primary" data-go="measure">Выбрать способ замера</button>
        <button class="btn" data-sim="start">Запустить демонстрацию</button>
      </div>
    </div></div>`));
  $('[data-go]', body).onclick = () => go('measure');
  $('[data-sim]', body).onclick = () => api('/api/sim', { method: 'POST', body: { action: 'start' } })
    .then(() => { refreshSim(true); toast('Демонстрация: стрелками ← → можно «крутить» выбранный угол'); })
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
    const t = P['toe_' + w] || {};
    const src = [p.src, t.src].filter(Boolean);
    card.querySelector('.src').textContent = [...new Set(src)].map(srcName).join(', ');
  });

  // Итог.
  const v = $('#verdict', root);
  const sub = $('#verdictSub', root);
  let cls = '', txt = 'Ждём показаний…', subtxt = '';
  if (f && f.measured > 0) {
    if (!f.has_limits) { txt = 'Допуски не заданы — показаны только измеренные углы'; }
    else if (f.overall === 'good') { cls = 'good'; txt = 'Все измеренные углы в допуске'; }
    else if (f.overall === 'marginal') { cls = 'marginal'; txt = 'В допуске, но у границы'; }
    else if (f.overall === 'bad') { cls = 'bad'; txt = `Вне допуска: ${f.out_of_spec}`; }
    subtxt = f.all_stable ? 'показания стабильны' : 'показания меняются…';
    if (f.measured > 0 && !f.thrust_referenced && (P.toe_FL?.has || P.toe_FR?.has)) {
      subtxt += ' · схождение от оси машины (задняя ось не измерена)';
    }
  }
  if (f && !state.connected) { cls = ''; txt = 'Нет связи с программой — переподключаюсь…'; }
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
  if (id === 'sim') return 'демо';
  if (id === 'manual') return 'вручную';
  if (id.startsWith('phone')) return 'телефон';
  if (id.startsWith('sensor:')) return 'датчик';
  if (id === 'optical') return 'камера';
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
  camber: 'Развал — наклон колеса поперёк машины. «Плюс» — верх колеса наружу. Разный развал слева и справа тянет машину в сторону большего плюса; большой минус съедает внутренний край шины, большой плюс — наружный.',
  toe: 'Схождение — поворот колеса в плане. «Плюс» — передний край колеса повёрнут к центру машины. Главный угол для износа шин: лишние десять минут стирают протектор «пилой» за сезон.',
  caster: 'Кастер (продольный наклон оси поворота, у шкворневых машин — шкворня). «Плюс» — верх оси назад. Он возвращает руль в прямое положение; разница слева и справа тянет машину в сторону меньшего кастера.',
  sai: 'Поперечный наклон оси поворота. Задан конструкцией и не регулируется. Из замера с поворотом он получается неточным — пользуйтесь им только для сравнения бортов: большая разница означает погнутую деталь.',
  front_total_toe: 'Сумма схождения обоих передних колёс. Её легко получить неправильно распределённой — и тогда руль стоит криво. Поэтому выставляйте схождение каждого колеса, а не только сумму.',
  rear_total_toe: 'Сумма схождения задних колёс. У неразрезного моста и балки не регулируется — отклонение говорит о деформации.',
  front_cross_camber: 'Разница развала левого и правого колеса. Именно она, а не сами значения, чаще всего уводит машину в сторону.',
  rear_cross_camber: 'Разница развала задних колёс. У моста и балки — признак деформации.',
  front_cross_caster: 'Разница кастера левого и правого колеса. Машину тянет в сторону колеса с меньшим кастером.',
  thrust_angle: 'Угол тяги — куда направлена задняя ось относительно оси симметрии машины. Если он не ноль, машина едет чуть боком, а руль при езде прямо стоит криво. Переднее схождение программа отсчитывает от линии тяги, чтобы руль встал ровно.',
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

export function openParam(key, refresh = false) {
  const f = state.frame;
  const ss = state.session;
  const p = f && f.params[key];
  const info = ss && ss.params ? ss.params[key] : null;
  const units = state.units;
  const rimMM = ss ? inches(ss.rim_diameter_in || 0) : 0;
  const fmt = d => specValue(key, d, units, rimMM);
  const kind = paramKind(key);
  const status = !p || !p.has ? 'none' : p.status;

  let togo = '';
  if (p && p.has && p.spec) {
    const d = p.to_nominal;
    const arrow = Math.abs(d) < 1e-4 ? '' : d > 0
      ? '<svg viewBox="0 0 24 24"><path d="M12 3 L21 14 H15 V21 H9 V14 H3 Z" fill="currentColor"/></svg>'
      : '<svg viewBox="0 0 24 24"><path d="M12 21 L21 10 H15 V3 H9 V10 H3 Z" fill="currentColor"/></svg>';
    const color = p.status === 'bad' ? 'var(--red)' : 'var(--green)';
    togo = `<div class="togo" style="color:${color}">${arrow}<span>до номинала ${fmt(d)}</span></div>`;
  }

  const simAdjustable = ss && ss.sim_running && ['camber', 'toe', 'caster'].includes(kind) && paramWheel(key);
  const adv = suspAdvice(key);
  const method = info && info.method;
  let adjust = '';
  if (method) adjust += `<p><b>Для этой модели:</b> ${esc(method)}</p>`;
  if (adv && adv.adj && adv.adj.how) {
    adjust += `<p><b>${esc(adv.susp.name)}:</b> ${esc(adv.adj.how)}</p>`;
    if (adv.adj.tip) adjust += `<p class="note">${esc(adv.adj.tip)}</p>`;
  }
  const notAdjustable = info && !info.adjustable && ['camber', 'toe', 'caster', 'thrust_angle'].includes(kind) && (adv && adv.adj && !adv.adj.usual);

  const html = `
    <h2>${esc(info ? info.label : shortLabel(key))}</h2>
    <div class="muted">${p && p.has ? esc(STATUS_RU[p.status] || '') + (p.stable ? ' · показания стабильны' : ' · показания меняются') : 'нет показаний'}</div>
    <div class="big ${status}">${fmtParam(key, p, units)}</div>
    ${p && p.spec ? `<div class="target">норма ${fmtRangeParam(key, p.spec, units, rimMM)} · номинал ${fmt(p.spec.nominal)}</div>` : '<div class="target muted">допуск не задан</div>'}
    ${togo}
    ${simAdjustable ? `
      <div class="sect"><h3>Демонстрация: крутим регулировку</h3>
        <div class="adj-btns">
          <button class="btn" data-adj="-0.25">−0°15′</button><button class="btn" data-adj="-0.05">−0°03′</button>
          <button class="btn" data-adj="0.05">+0°03′</button><button class="btn" data-adj="0.25">+0°15′</button>
        </div>
        <p class="muted" style="text-align:center;font-size:12.5px">Или стрелками ← → на клавиатуре (с Shift — крупнее).
          Обратите внимание: кастер и развал тянут за собой схождение.</p>
      </div>` : ''}
    <div class="sect"><h3>Что это</h3><p>${esc(EXPLAIN[kind] || '')}</p></div>
    ${adjust ? `<div class="sect"><h3>Чем регулируется</h3>${adjust}</div>` : ''}
    ${notAdjustable ? `<div class="warn">На этой конструкции угол штатно не регулируется. Если он вне допуска — ищите износ или деформацию, а не регулировочный болт.</div>` : ''}
    ${p && p.src ? `<p class="dim" style="margin-top:14px;font-size:12px">Источник: ${esc(srcName(p.src))}</p>` : ''}`;

  openDrawer(html, key, refresh);
  $$('[data-adj]', $('#drawerBody')).forEach(b => b.onclick = () => simAdjust(key, parseFloat(b.dataset.adj)));
}

export function simAdjust(key, delta) {
  return api('/api/sim', { method: 'POST', body: { action: 'adjust', key, delta } })
    .catch(e => toast(e.message, true));
}

export function selectedKey() { return state.selected; }

export function closeParam() {
  state.selected = null;
  closeDrawer();
  render();
}
