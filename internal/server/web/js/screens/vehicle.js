// Шаг 1: автомобиль — поиск, тип подвески, обод и колея.

import { state, on, api, updateSession, toast, h, $, $$ } from '../state.js';
import { esc, fmtDM } from '../fmt.js';
import { go } from '../app.js';

let root, suspList = null, klass = '', timer;

const CLASSES = [
  ['', 'Все'], ['car', 'Легковые'], ['suv', 'Внедорожники'], ['lcv', 'Фургоны, LCV'], ['truck', 'Грузовые'], ['bus', 'Автобусы'],
];

export function mount(el) {
  root = el;
  root.innerHTML = `
    <div class="vgrid">
      <div>
        <h1>Выберите автомобиль</h1>
        <p class="lede">Введите марку и модель — по-русски или латиницей. Если модели нет, возьмите ориентир
          по классу ниже и укажите тип подвески: советы по регулировке зависят прежде всего от конструкции.</p>
        <div class="search-row">
          <input type="search" id="vq" placeholder="например: Волга 3110, ГАЗель, ВАЗ 2107, Нива" autocomplete="off">
          <input type="number" id="vy" class="year" placeholder="Год" min="1900" max="2100">
        </div>
        <div class="chips" id="vclass">${CLASSES.map(([k, n]) => `<button class="chip${k === '' ? ' on' : ''}" data-c="${k}">${n}</button>`).join('')}</div>
        <div class="cards" id="vres"></div>
        <h2>Модели нет в базе?</h2>
        <p class="lede" style="margin-bottom:6px">Возьмите ориентир по классу — программа покажет, насколько углы далеки от разумных для
          такой конструкции. Это не заводские данные: найдя их в руководстве, внесите их кнопкой «Внести допуски».</p>
        <div class="cards" id="vguid"></div>
      </div>
      <div id="vdetail"></div>
    </div>`;
  const q = $('#vq', root), y = $('#vy', root);
  q.oninput = y.oninput = () => { clearTimeout(timer); timer = setTimeout(search, 160); };
  $$('#vclass .chip', root).forEach(b => b.onclick = () => {
    klass = b.dataset.c;
    $$('#vclass .chip', root).forEach(x => x.classList.toggle('on', x === b));
    search();
  });
  on('session', renderDetail);
  loadSusp().then(renderDetail);
  search();
}

export function show() {
  renderDetail();
  setTimeout(() => $('#vq', root).focus(), 50);
}

async function loadSusp() {
  if (!suspList) suspList = await api('/api/suspensions');
  return suspList;
}

function badgeFor(s) {
  const b = [`<span class="badge ${esc(s.source_kind)}">${esc(s.source_label)}</span>`];
  if (s.local) b.unshift('<span class="badge local">мои данные</span>');
  if (s.class_name) b.push(`<span class="badge">${esc(s.class_name)}</span>`);
  return b.join('');
}

function suspName(id) {
  if (!suspList || !id) return '';
  const s = suspList.all.find(x => x.id === id);
  return s ? s.name : id;
}

function card(s) {
  const title = s.source_kind === 'class_guidance' ? s.model : s.title;
  const susp = [s.front_suspension && 'перед: ' + suspName(s.front_suspension), s.rear_suspension && 'зад: ' + suspName(s.rear_suspension)].filter(Boolean).join(' · ');
  const el = h(`<button class="vcard">
      <span class="t">${esc(title)}</span>
      <span class="badges">${badgeFor(s)}</span>
      <span class="m">${esc(susp || s.notes || '')}</span>
    </button>`);
  el.classList.toggle('sel', !!(state.session && state.session.vehicle && state.session.vehicle.id === s.id));
  el.onclick = () => choose(s.id);
  return el;
}

async function search() {
  const q = $('#vq', root).value.trim();
  const y = $('#vy', root).value.trim();
  let data;
  try {
    data = await api(`/api/specs/search?q=${encodeURIComponent(q)}&year=${encodeURIComponent(y)}&class=${encodeURIComponent(klass)}`);
  } catch (e) { toast(e.message, true); return; }
  await loadSusp();
  const res = $('#vres', root);
  res.replaceChildren();
  if (!data.results.length) {
    res.append(h(`<p class="muted">${q ? 'Ничего не найдено. Попробуйте другое написание — или возьмите ориентир по классу ниже.' : 'В базе нет записей этого класса.'}</p>`));
  }
  data.results.forEach(s => res.append(card(s)));
  const g = $('#vguid', root);
  g.replaceChildren();
  (data.guidance || []).forEach(s => g.append(card(s)));
}

async function choose(id) {
  try {
    await updateSession({ spec_id: id });
    $$('.vcard', root).forEach(c => c.classList.remove('sel'));
    search();
  } catch (e) { toast(e.message, true); }
}

// ── Карточка выбранного автомобиля ───────────────────────────────────

function specTable(spec) {
  if (!spec) return '';
  const row = (name, r) => r ? `<tr><td>${name}</td><td class="v">${fmtDM(r.min)}</td><td class="v">${fmtDM(r.nominal)}</td><td class="v">${fmtDM(r.max)}</td></tr>` : '';
  const f = spec.front, r = spec.rear;
  const body = [
    row('Развал, перед', f.camber), row('Кастер, перед', f.caster), row('Попер. наклон оси', f.sai),
    row('Схождение Σ, перед', f.total_toe), row('Развал, зад', r.camber), row('Схождение Σ, зад', r.total_toe),
  ].join('');
  if (!body) return '';
  return `<table class="params" style="margin-top:10px">
    <thead><tr><th>Параметр</th><th>мин</th><th>номинал</th><th>макс</th></tr></thead><tbody>${body}</tbody></table>`;
}

function suspSelect(id, axle, current) {
  const list = suspList ? suspList[axle] : [];
  return `<select id="${id}">
    <option value=""${!current ? ' selected' : ''}>Не знаю / другая</option>
    ${list.filter(s => s.id).map(s => `<option value="${esc(s.id)}"${s.id === current ? ' selected' : ''}>${esc(s.name)}</option>`).join('')}
  </select>`;
}

function renderDetail() {
  const box = $('#vdetail', root);
  if (!box) return;
  const ss = state.session;
  if (!ss || !ss.vehicle) {
    box.innerHTML = `<div class="panel"><h2 style="margin-top:0">Автомобиль не выбран</h2>
      <p class="muted">Найдите свою машину слева. Можно работать и без выбора — программа покажет углы, но не
        сравнит их с допуском.</p>
      <div class="actions"><button class="btn" id="vskip">Продолжить без автомобиля</button></div></div>`;
    $('#vskip', box).onclick = () => go('prep');
    return;
  }
  const v = ss.vehicle;
  const lim = ss.limits;
  const title = v.source_kind === 'class_guidance' ? 'Ориентир по классу: ' + v.model : v.title;
  const fs = ss.front_suspension, rs = ss.rear_suspension;

  let trust = '';
  if (v.disclaimer) trust = `<div class="${v.source_kind === 'community' ? 'note' : 'warn'}" style="margin:10px 0">${esc(v.disclaimer)}</div>`;
  if (lim && lim.id !== v.id) trust += `<div class="note" style="margin:10px 0">Допуски для сравнения: <b>${esc(lim.model)}</b> — ориентир по классу.</div>`;
  if (!lim) trust += `<div class="note" style="margin:10px 0">Допусков для сравнения нет — углы будут показаны без оценки «в норме / не в норме».</div>`;

  box.innerHTML = `
    <div class="panel vdetail">
      <h1>${esc(title)}</h1>
      <div style="display:flex;gap:6px;flex-wrap:wrap">${badgeFor(v)}</div>
      ${trust}
      ${v.notes ? `<p class="muted" style="font-size:13.5px">${esc(v.notes)}</p>` : ''}

      <h3>Подвеска</h3>
      <div class="susp-pick">
        <label class="f">Передняя ${suspSelect('vFront', 'front', fs.id)}
          <details class="help"><summary>Как узнать, какая у меня?</summary><div>${esc(fs.identify || suspList?.all.find(s => !s.id)?.identify || '')}</div></details>
        </label>
        <label class="f">Задняя ${suspSelect('vRear', 'rear', rs.id)}
          <details class="help"><summary>Как узнать?</summary><div>${esc(rs.identify)}</div></details>
        </label>
      </div>

      <h3>Размеры</h3>
      <div class="cols3">
        <label class="f">Обод, дюймы <input type="number" id="vRim" step="0.5" min="8" max="30" value="${ss.rim_diameter_in}">
          <small>По нему схождение переводится в мм.</small></label>
        <label class="f">Колея перед, мм <input type="number" id="vTF" step="1" value="${ss.track_front_mm || ''}" placeholder="1400">
          <small>Нужна для проверки струн.</small></label>
        <label class="f">Колея зад, мм <input type="number" id="vTR" step="1" value="${ss.track_rear_mm || ''}" placeholder="1380"></label>
      </div>

      ${lim ? `<h3>Допуски${lim.id !== v.id ? ' (ориентир)' : ''}</h3>${specTable(ss.spec)}` : ''}
      ${ss.conditions ? `<p class="muted" style="font-size:13px;margin-top:10px"><b>Условия замера:</b> ${esc(ss.conditions)}</p>` : ''}

      <div class="actions">
        <button class="btn primary" id="vNext">Далее: подготовка →</button>
        <button class="btn" id="vContrib">${v.local ? 'Изменить мои допуски' : 'Внести допуски из руководства'}</button>
        ${v.local ? '<button class="btn danger" id="vDel">Удалить мои данные</button>' : ''}
      </div>
    </div>`;

  $('#vFront', box).onchange = e => updateSession({ front_suspension: e.target.value }).catch(err => toast(err.message, true));
  $('#vRear', box).onchange = e => updateSession({ rear_suspension: e.target.value }).catch(err => toast(err.message, true));
  const num = id => { const v = parseFloat($(id, box).value); return Number.isFinite(v) ? v : 0; };
  $('#vRim', box).onchange = () => updateSession({ rim_diameter_in: num('#vRim') }).catch(err => toast(err.message, true));
  $('#vTF', box).onchange = () => updateSession({ track_front_mm: num('#vTF') }).catch(err => toast(err.message, true));
  $('#vTR', box).onchange = () => updateSession({ track_rear_mm: num('#vTR') }).catch(err => toast(err.message, true));
  $('#vNext', box).onclick = () => go('prep');
  $('#vContrib', box).onclick = () => go('contrib', { fromSession: true });
  const del = $('#vDel', box);
  if (del) del.onclick = async () => {
    if (!confirm('Удалить ваши данные по этому автомобилю? Файл будет удалён из профиля.')) return;
    try {
      await api('/api/specs/' + encodeURIComponent(v.id), { method: 'DELETE' });
      await updateSession({ spec_id: '' });
      search();
      toast('Удалено');
    } catch (e) { toast(e.message, true); }
  };
}
