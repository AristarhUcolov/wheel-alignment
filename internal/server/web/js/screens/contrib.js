// Внести допуски из руководства: проверка, сохранение у себя, файл для проекта.

import { state, api, toast, updateSession, h, $, $$ } from '../state.js';
import { esc, fmtDM } from '../fmt.js';
import { go } from '../app.js';

let root, susp = null;

// Угол так, как его печатают в руководствах: «0°30'», «-0 30», «+1°15′»,
// «0,5». Минуты — это не десятые доли градуса: 0°30' = 0,5°.
export function parseAngle(s) {
  if (s === null || s === undefined) return null;
  s = String(s).trim().replace(/[−–]/g, '-').replace(',', '.');
  if (!s) return null;
  const m = s.match(/^([+-]?)\s*(\d+(?:\.\d+)?)\s*(?:°|º|град\.?|\s)\s*(\d+(?:\.\d+)?)?\s*['′’]?\s*$/);
  if (m && m[3] !== undefined) {
    const v = parseFloat(m[2]) + parseFloat(m[3]) / 60;
    return m[1] === '-' ? -v : v;
  }
  const d = s.match(/^([+-]?\d+(?:\.\d+)?)\s*°?$/);
  if (d) return parseFloat(d[1]);
  const mm = s.match(/^([+-]?)\s*(\d+(?:\.\d+)?)\s*['′’]$/);
  if (mm) { const v = parseFloat(mm[2]) / 60; return mm[1] === '-' ? -v : v; }
  return NaN;
}

const AX = [
  { k: 'camber', n: 'Развал', hint: '«+» — верх колеса наружу', axles: ['front', 'rear'] },
  { k: 'caster', n: 'Кастер (продольный наклон оси/шкворня)', hint: '«+» — верх оси назад', axles: ['front'] },
  { k: 'sai', n: 'Поперечный наклон оси/шкворня', hint: 'обычно 5–15°', axles: ['front'] },
  { k: 'total_toe', n: 'Суммарное схождение, в градусах', hint: '«+» — колёса сходятся спереди', axles: ['front', 'rear'] },
];

export function mount(el) { root = el; }

export async function show() {
  if (!susp) susp = await api('/api/suspensions');
  render();
}

function sel(id, list, cur) {
  return `<select id="${id}"><option value="">не указано</option>${list.filter(s => s.id).map(s => `<option value="${esc(s.id)}"${s.id === cur ? ' selected' : ''}>${esc(s.name)}</option>`).join('')}</select>`;
}

function render() {
  const ss = state.session || {};
  const v = ss.vehicle && ss.vehicle.source_kind !== 'class_guidance' ? ss.vehicle : null;
  const own = v && v.local ? ss.spec : null; // свои допуски — предзаполняем цифрами
  const pre = {
    make: v ? v.make : '', model: v ? v.model : '', cls: v ? v.class : '',
    front: ss.front_suspension ? ss.front_suspension.id : '', rear: ss.rear_suspension ? ss.rear_suspension.id : '',
    rim: ss.rim_diameter_in || '',
    from: v && v.year_from ? v.year_from : '', to: v && v.year_to ? v.year_to : '',
  };
  const range = (ax, k) => own && own[ax] && own[ax][k] ? own[ax][k] : null;
  const val = r => r ? fmtDM(r, { sign: true }).replace('′', "'") : '';

  root.innerHTML = `
    <div style="max-width:1080px">
      <h1>Внести допуски из руководства</h1>
      <div class="warn" style="margin:10px 0 16px"><b>Вносите только то, что видите в документе.</b> Выдуманный угол развала —
        это не опечатка, а съеденная за сезон резина. Пустое поле лучше выдуманного: программа честно покажет «нет данных».</div>

      <div class="panel">
        <h3 style="margin-top:0">Автомобиль</h3>
        <div class="cols3">
          <label class="f">Марка<input type="text" id="cMake" value="${esc(pre.make)}" placeholder="ГАЗ"></label>
          <label class="f">Модель<input type="text" id="cModel" value="${esc(pre.model)}" placeholder="3110 «Волга»"></label>
          <label class="f">Модификация<input type="text" id="cTrim" placeholder="необязательно"></label>
          <label class="f">Год начала выпуска<input type="number" id="cFrom" placeholder="1997" value="${pre.from}"></label>
          <label class="f">Год окончания<input type="number" id="cTo" placeholder="пусто — выпускается" value="${pre.to}"></label>
          <label class="f">Класс<select id="cClass">
            ${[['', 'не указан'], ['car', 'Легковой'], ['suv', 'Внедорожник'], ['lcv', 'Фургон, LCV'], ['truck', 'Грузовой'], ['bus', 'Автобус']]
              .map(([k, n]) => `<option value="${k}"${k === pre.cls ? ' selected' : ''}>${n}</option>`).join('')}</select></label>
          <label class="f">Передняя подвеска${sel('cFront', susp.front, pre.front)}</label>
          <label class="f">Задняя подвеска${sel('cRear', susp.rear, pre.rear)}</label>
          <label class="f">Диаметр обода, дюймы<input type="number" id="cRim" step="0.5" value="${pre.rim}">
            <small>Обязателен, если схождение в миллиметрах: «3 мм» на 13″ и на 17″ — разные углы.</small></label>
        </div>
      </div>

      <div class="panel">
        <h3 style="margin-top:0">Углы</h3>
        <p class="muted" style="font-size:13px">Вводите как в руководстве: <b>0°30'</b>, <b>-0 30</b> или десятичными градусами <b>0.5</b>.
          Справа от поля видно, как программа поняла число.</p>
        ${['front', 'rear'].map(ax => `
          <h3>${ax === 'front' ? 'Передняя ось' : 'Задняя ось'}</h3>
          <div class="cols2">
            ${AX.filter(a => a.axles.includes(ax)).map(a => `
              <label class="f">${a.n} <small>${a.hint}</small>
                <span class="row" style="gap:6px;align-items:center">
                  <input type="text" data-ax="${ax}" data-k="${a.k}" data-b="min" placeholder="от" style="width:110px" value="${val(range(ax, a.k) && range(ax, a.k).min)}">
                  <input type="text" data-ax="${ax}" data-k="${a.k}" data-b="max" placeholder="до" style="width:110px" value="${val(range(ax, a.k) && range(ax, a.k).max)}">
                  <span class="dim num" data-show="${ax}-${a.k}"></span>
                </span></label>`).join('')}
            <label class="f">Суммарное схождение в мм, если в документе так
              <span class="row" style="gap:6px"><input type="number" step="0.1" data-mm="${ax}" data-b="min" placeholder="от, мм" style="width:110px">
              <input type="number" step="0.1" data-mm="${ax}" data-b="max" placeholder="до, мм" style="width:110px"></span></label>
          </div>
          <div class="cols3" style="margin-top:8px">
            ${['camber', ...(ax === 'front' ? ['caster'] : []), 'toe'].map(k => `
              <label class="chk"><input type="checkbox" data-adj="${ax}-${k}"><span>${{ camber: 'Развал', caster: 'Кастер', toe: 'Схождение' }[k]} регулируется</span></label>`).join('')}
          </div>
          <label class="f" style="margin-top:6px">Чем регулируется (по руководству)<input type="text" data-method="${ax}" placeholder="например: регулировочные шайбы под осью нижнего рычага"></label>`).join('')}
        <div class="cols3" style="margin-top:12px">
          <label class="f">Разница развала лев./прав. не более<input type="text" id="cXCam" placeholder="0°30'"></label>
          <label class="f">Разница кастера лев./прав. не более<input type="text" id="cXCas" placeholder="0°30'"></label>
          <label class="f">Угол тяги не более<input type="text" id="cThr" placeholder="0°15'"></label>
        </div>
      </div>

      <div class="panel">
        <h3 style="margin-top:0">Условия замера и источник</h3>
        <div class="cols3">
          <label class="f">Загрузка<input type="text" id="cLoad" placeholder="снаряжённая масса, полный бак"></label>
          <label class="f">Давление в шинах<input type="text" id="cPress" placeholder="по норме, одинаково по бортам"></label>
          <label class="f">Осадка подвески<input type="text" id="cSettle" placeholder="прокатить 3–5 м вперёд"></label>
          <label class="f">Источник<select id="cKind">
            <option value="factory">Заводское руководство</option><option value="licensed">Лицензионная база</option>
            <option value="community">Сообщество, перепроверено</option><option value="unverified" selected>Не проверено</option></select></label>
          <label class="f">Документ, издание, страница<input type="text" id="cRef" placeholder="Руководство по ремонту ГАЗ-3110, стр. 112">
            <small>Для заводского источника обязательно.</small></label>
          <label class="f">Как вас указать<input type="text" id="cWho" placeholder="необязательно"></label>
        </div>
        <label class="f" style="margin-top:10px">Примечания<input type="text" id="cNotes" placeholder="особенности конструкции, порядок регулировки"></label>
      </div>

      <div class="actions">
        <button class="btn primary" id="cCheck">Проверить</button>
        <button class="btn good" id="cSave">Сохранить у себя и выбрать</button>
        <a class="btn hidden" id="cFile" download="vehicle.json">Скачать файл для проекта</a>
      </div>
      <div id="cOut" style="margin-top:14px"></div>
    </div>`;

  $$('[data-k]', root).forEach(inp => inp.addEventListener('input', () => showParsed(inp.dataset.ax, inp.dataset.k)));
  AX.forEach(a => a.axles.forEach(ax => showParsed(ax, a.k)));
  if (own) {
    for (const ax of ['front', 'rear']) {
      const a = own[ax].adjustable || {};
      for (const k of ['camber', 'caster', 'toe']) { const c = $(`[data-adj="${ax}-${k}"]`, root); if (c) c.checked = !!a[k]; }
      $(`[data-method="${ax}"]`, root).value = a.toe_method || a.camber_method || '';
    }
  }
  $('#cCheck', root).onclick = () => check(false);
  $('#cSave', root).onclick = () => check(true);
}

function showParsed(ax, k) {
  const lo = parseAngle($(`[data-ax="${ax}"][data-k="${k}"][data-b="min"]`, root).value);
  const hi = parseAngle($(`[data-ax="${ax}"][data-k="${k}"][data-b="max"]`, root).value);
  const out = $(`[data-show="${ax}-${k}"]`, root);
  if (lo === null && hi === null) { out.textContent = ''; return; }
  if (Number.isNaN(lo) || Number.isNaN(hi)) { out.innerHTML = '<span class="no">не понял число</span>'; return; }
  out.textContent = `= ${lo === null ? '?' : fmtDM(lo)} … ${hi === null ? '?' : fmtDM(hi)}`;
}

function slug(s) { return s.toLowerCase().replace(/[^a-zа-яё0-9]+/gi, '-').replace(/^-|-$/g, ''); }

function draft() {
  const g = id => $(id, root).value.trim();
  const make = g('#cMake'), model = g('#cModel');
  const from = parseInt(g('#cFrom'), 10) || 0;
  const ss = state.session || {};
  const baseId = ss.vehicle && ss.vehicle.source_kind === 'catalog' ? ss.vehicle.id : [slug(make), slug(model), from || ''].filter(Boolean).join('-');
  const id = ss.vehicle && ss.vehicle.local ? ss.vehicle.id : 'my-' + (baseId || 'car');

  const axle = ax => {
    const out = { adjustable: {} };
    for (const a of AX) {
      if (!a.axles.includes(ax)) continue;
      const lo = parseAngle($(`[data-ax="${ax}"][data-k="${a.k}"][data-b="min"]`, root).value);
      const hi = parseAngle($(`[data-ax="${ax}"][data-k="${a.k}"][data-b="max"]`, root).value);
      if (lo === null || hi === null || Number.isNaN(lo) || Number.isNaN(hi)) continue;
      out[a.k] = { min: lo, nominal: (lo + hi) / 2, max: hi };
    }
    for (const k of ['camber', 'caster', 'toe']) {
      const c = $(`[data-adj="${ax}-${k}"]`, root);
      if (c) out.adjustable[k] = c.checked;
    }
    const m = $(`[data-method="${ax}"]`, root).value.trim();
    if (m) {
      if (out.adjustable.toe) out.adjustable.toe_method = m;
      if (out.adjustable.camber) out.adjustable.camber_method = m;
      if (out.adjustable.caster) out.adjustable.caster_method = m;
    }
    return out;
  };
  const spec = {
    id, make, model, trim: g('#cTrim'), notes: g('#cNotes'),
    year_from: from, year_to: parseInt(g('#cTo'), 10) || 0,
    class: g('#cClass'), front_suspension: g('#cFront'), rear_suspension: g('#cRear'),
    rim_diameter_in: parseFloat(g('#cRim')) || 0,
    front: axle('front'), rear: axle('rear'),
    conditions: { load: g('#cLoad'), pressure: g('#cPress'), settle: g('#cSettle') },
    source: { kind: g('#cKind'), reference: g('#cRef'), contributor: g('#cWho'), added: new Date().toISOString().slice(0, 10) },
  };
  for (const ax of ['front', 'rear']) {
    const lo = parseFloat($(`[data-mm="${ax}"][data-b="min"]`, root).value);
    const hi = parseFloat($(`[data-mm="${ax}"][data-b="max"]`, root).value);
    if (Number.isFinite(lo) && Number.isFinite(hi)) spec[ax + '_total_toe_mm'] = { min: lo, max: hi };
  }
  const xc = parseAngle(g('#cXCam')), xk = parseAngle(g('#cXCas')), th = parseAngle(g('#cThr'));
  if (Number.isFinite(xc)) spec.front.max_cross_camber = xc;
  if (Number.isFinite(xk)) spec.front.max_cross_caster = xk;
  if (Number.isFinite(th)) spec.max_thrust_angle = th;
  return spec;
}

async function check(andSave) {
  const out = $('#cOut', root);
  const spec = draft();
  out.innerHTML = '<span class="spin"></span> Проверяю…';
  let d;
  try { d = await api('/api/specs/check', { method: 'POST', body: spec }); } catch (e) { out.innerHTML = `<div class="danger-box">${esc(e.message)}</div>`; return; }
  const errs = (d.errors || []).length ? `<div class="danger-box"><b>Так запись не сохранится:</b><ul class="plain">${d.errors.map(e => `<li>${esc(e)}</li>`).join('')}</ul></div>` : '';
  const warns = (d.warnings || []).length ? `<div class="warn" style="margin-top:10px"><b>Проверьте:</b><ul class="plain">${d.warnings.map(e => `<li>${esc(e)}</li>`).join('')}</ul></div>` : '';
  const res = (d.resolved || []).length ? `<h3>Как программа поняла ваши цифры</h3><table class="params"><tbody>
    ${d.resolved.map(x => `<tr><td>${esc(x.axle)}</td><td>${esc(x.name)}</td><td class="v">${esc(x.range)}</td><td class="muted">${esc(x.detail || '')}</td></tr>`).join('')}</tbody></table>` : '';
  out.innerHTML = `${errs}${d.ok ? `<div class="ok-box">Запись корректна.${d.verified ? '' : ' Источник не заводской — программа будет показывать предупреждение.'}</div>` : ''}${warns}${res}`;

  const file = $('#cFile', root);
  if (d.ok && d.file) {
    file.href = URL.createObjectURL(new Blob([d.file], { type: 'application/json' }));
    file.download = spec.id + '.json';
    file.classList.remove('hidden');
  } else file.classList.add('hidden');

  if (andSave && d.ok) {
    try {
      await api('/api/specs/save', { method: 'POST', body: spec });
      await updateSession({ spec_id: spec.id });
      toast('Сохранено у вас. Эта запись теперь находится поиском первой.');
      out.insertAdjacentHTML('beforeend', `<div class="note" style="margin-top:10px">Хотите помочь всем владельцам этой модели? Скачайте файл
        и пришлите его в проект (github.com/AristarhUcolov/wheel-alignment → Issues), указав, откуда цифры.</div>`);
      setTimeout(() => go('vehicle'), 1200);
    } catch (e) { toast(e.message, true); }
  }
}
