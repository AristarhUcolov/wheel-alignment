// Шаг 1: автомобиль — поиск, тип подвески, обод и колея.

import { state, on, api, updateSession, toast, h, $, $$ } from '../state.js';
import { esc, fmtDM } from '../fmt.js';
import { t } from '../i18n.js';
import { go } from '../app.js';

let root, suspList = null, klass = '', timer;

const CLASSES = () => [
  ['', t('Все')], ['car', t('Легковые')], ['suv', t('Внедорожники')], ['lcv', t('Фургоны, LCV')], ['truck', t('Грузовые')], ['bus', t('Автобусы')],
];

export function mount(el) {
  root = el;
  root.innerHTML = `
    <div class="vgrid">
      <div>
        <h1>${t('Выберите автомобиль')}</h1>
        <p class="lede">${t('Введите марку и модель — по-русски или латиницей. Если модели нет, возьмите ориентир по классу ниже и укажите тип подвески: советы по регулировке зависят прежде всего от конструкции.')}</p>
        <div class="search-row">
          <input type="search" id="vq" placeholder="${esc(t('например: Волга 3110, ГАЗель, ВАЗ 2107, Нива'))}" autocomplete="off">
          <input type="number" id="vy" class="year" placeholder="${esc(t('Год'))}" min="1900" max="2100">
        </div>
        <div class="chips" id="vclass">${CLASSES().map(([k, n]) => `<button class="chip${k === '' ? ' on' : ''}" data-c="${k}">${n}</button>`).join('')}</div>
        <div class="cards" id="vres"></div>
        <h2>${t('Модели нет в базе?')}</h2>
        <p class="lede" style="margin-bottom:6px">${t('Возьмите ориентир по классу — программа покажет, насколько углы далеки от разумных для такой конструкции. Это не заводские данные: найдя их в руководстве, внесите их кнопкой «Внести допуски».')}</p>
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
  if (s.local) b.unshift(`<span class="badge local">${t('мои данные')}</span>`);
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
  const susp = [s.front_suspension && t('перед: {name}', { name: suspName(s.front_suspension) }),
    s.rear_suspension && t('зад: {name}', { name: suspName(s.rear_suspension) })].filter(Boolean).join(' · ');
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
    res.append(h(`<p class="muted">${q ? t('Ничего не найдено. Попробуйте другое написание — или возьмите ориентир по классу ниже.') : t('В базе нет записей этого класса.')}</p>`));
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
    row(t('Развал, перед'), f.camber), row(t('Кастер, перед'), f.caster), row(t('Попер. наклон оси'), f.sai),
    row(t('Схождение Σ, перед'), f.total_toe), row(t('Развал, зад'), r.camber), row(t('Схождение Σ, зад'), r.total_toe),
  ].join('');
  if (!body) return '';
  return `<table class="params" style="margin-top:10px">
    <thead><tr><th>${t('Параметр')}</th><th>${t('мин')}</th><th>${t('номинал')}</th><th>${t('макс')}</th></tr></thead><tbody>${body}</tbody></table>`;
}

function suspSelect(id, axle, current) {
  const list = suspList ? suspList[axle] : [];
  return `<select id="${id}">
    <option value=""${!current ? ' selected' : ''}>${t('Не знаю / другая')}</option>
    ${list.filter(s => s.id).map(s => `<option value="${esc(s.id)}"${s.id === current ? ' selected' : ''}>${esc(s.name)}</option>`).join('')}
  </select>`;
}

function renderDetail() {
  const box = $('#vdetail', root);
  if (!box) return;
  const ss = state.session;
  if (!ss || !ss.vehicle) {
    box.innerHTML = `<div class="panel"><h2 style="margin-top:0">${t('Автомобиль не выбран')}</h2>
      <p class="muted">${t('Найдите свою машину слева. Можно работать и без выбора — программа покажет углы, но не сравнит их с допуском.')}</p>
      <div class="actions"><button class="btn" id="vskip">${t('Продолжить без автомобиля')}</button></div></div>`;
    $('#vskip', box).onclick = () => go('prep');
    return;
  }
  const v = ss.vehicle;
  const lim = ss.limits;
  const title = v.source_kind === 'class_guidance' ? t('Ориентир по классу: {model}', { model: v.model }) : v.title;
  const fs = ss.front_suspension, rs = ss.rear_suspension;

  let trust = '';
  if (v.disclaimer) trust = `<div class="${v.source_kind === 'community' ? 'note' : 'warn'}" style="margin:10px 0">${esc(v.disclaimer)}</div>`;
  if (v.source_reference && v.source_kind !== 'class_guidance' && v.source_kind !== 'catalog') {
    trust += `<p class="muted" style="font-size:12.5px"><b>${t('Источник:')}</b> ${esc(v.source_reference)}</p>`;
  }
  if (lim && lim.id !== v.id) trust += `<div class="note" style="margin:10px 0">${t('Допуски для сравнения: {model} — ориентир по классу.', { model: `<b>${esc(lim.model)}</b>` })}</div>`;
  if (!lim) trust += `<div class="note" style="margin:10px 0">${t('Допусков для сравнения нет — углы будут показаны без оценки «в норме / не в норме».')}</div>`;

  box.innerHTML = `
    <div class="panel vdetail">
      <h1>${esc(title)}</h1>
      <div style="display:flex;gap:6px;flex-wrap:wrap">${badgeFor(v)}</div>
      ${trust}
      ${v.notes ? `<p class="muted" style="font-size:13.5px">${esc(v.notes)}</p>` : ''}

      <h3>${t('Подвеска')}</h3>
      <div class="susp-pick">
        <label class="f">${t('Передняя')} ${suspSelect('vFront', 'front', fs.id)}
          <details class="help"><summary>${t('Как узнать, какая у меня?')}</summary><div>${esc(fs.identify)}</div></details>
        </label>
        <label class="f">${t('Задняя')} ${suspSelect('vRear', 'rear', rs.id)}
          <details class="help"><summary>${t('Как узнать?')}</summary><div>${esc(rs.identify)}</div></details>
        </label>
      </div>

      <h3>${t('Размеры')}</h3>
      <div class="cols3">
        <label class="f">${t('Обод, дюймы')} <input type="number" id="vRim" step="0.5" min="8" max="30" value="${ss.rim_diameter_in}">
          <small>${t('По нему схождение переводится в мм.')}</small></label>
        <label class="f">${t('Колея перед, мм')} <input type="number" id="vTF" step="1" value="${ss.track_front_mm || ''}" placeholder="1400">
          <small>${t('Нужна для проверки струн.')}</small></label>
        <label class="f">${t('Колея зад, мм')} <input type="number" id="vTR" step="1" value="${ss.track_rear_mm || ''}" placeholder="1380"></label>
      </div>

      ${lim ? `<h3>${lim.id !== v.id ? t('Допуски (ориентир)') : t('Допуски')}</h3>${specTable(ss.spec)}` : ''}
      ${ss.conditions ? `<p class="muted" style="font-size:13px;margin-top:10px"><b>${t('Условия замера:')}</b> ${esc(ss.conditions)}</p>` : ''}

      <div class="actions">
        <button class="btn primary" id="vNext">${t('Далее: подготовка →')}</button>
        <button class="btn" id="vContrib">${v.local ? t('Изменить мои допуски') : t('Внести допуски из руководства')}</button>
        ${v.local ? `<button class="btn danger" id="vDel">${t('Удалить мои данные')}</button>` : ''}
      </div>
    </div>`;

  $('#vFront', box).onchange = e => updateSession({ front_suspension: e.target.value }).catch(err => toast(err.message, true));
  $('#vRear', box).onchange = e => updateSession({ rear_suspension: e.target.value }).catch(err => toast(err.message, true));
  const num = id => { const x = parseFloat($(id, box).value); return Number.isFinite(x) ? x : 0; };
  $('#vRim', box).onchange = () => updateSession({ rim_diameter_in: num('#vRim') }).catch(err => toast(err.message, true));
  $('#vTF', box).onchange = () => updateSession({ track_front_mm: num('#vTF') }).catch(err => toast(err.message, true));
  $('#vTR', box).onchange = () => updateSession({ track_rear_mm: num('#vTR') }).catch(err => toast(err.message, true));
  $('#vNext', box).onclick = () => go('prep');
  $('#vContrib', box).onclick = () => go('contrib', { fromSession: true });
  const del = $('#vDel', box);
  if (del) del.onclick = async () => {
    if (!confirm(t('Удалить ваши данные по этому автомобилю? Файл будет удалён из профиля.'))) return;
    try {
      await api('/api/specs/' + encodeURIComponent(v.id), { method: 'DELETE' });
      await updateSession({ spec_id: '' });
      search();
      toast(t('Удалено'));
    } catch (e) { toast(e.message, true); }
  };
}
