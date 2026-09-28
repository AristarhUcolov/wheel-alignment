// Замер камерой: калибровка, развал по фото, полный замер четырёх колёс.
// Вся обработка снимков — в Go, без OpenCV.

import { state, api, toast, h, $, $$ } from '../state.js';
import { WHEELS, esc, fmtDM } from '../fmt.js';
import { t } from '../i18n.js';
import { go } from '../app.js';

let tab = 'calib';
let livePoll = null;

export function render(body) {
  body.append(h(`<div class="panel">
    <div class="warn" style="margin-bottom:14px">${t('Мишень можно закрепить на диске как угодно криво — программа находит ось вращения самого колеса, и перекос крепления уходит. Для схождения и угла тяги нужна ещё напольная мишень в кадре: она связывает четыре колеса в одну систему координат — это вкладка «Полный замер».')}</div>
    <div class="tabs" id="oTabs">
      <button class="tab" data-tab="calib">${t('1. Калибровка камеры')}</button>
      <button class="tab" data-tab="camber">${t('2. Развал по фото')}</button>
      <button class="tab" data-tab="full">${t('3. Полный замер')}</button>
      <button class="tab" data-tab="livecam">${t('4. Живой режим')}</button>
    </div>
    <div id="oBody"></div>
  </div>`));
  $$('#oTabs .tab', body).forEach(b => b.onclick = () => { tab = b.dataset.tab; draw(body); });
  draw(body);
}

function draw(body) {
  $$('#oTabs .tab', body).forEach(b => b.classList.toggle('on', b.dataset.tab === tab));
  const box = $('#oBody', body);
  box.innerHTML = '';
  clearInterval(livePoll);
  ({ calib, camber, full, livecam })[tab](box);
}

function fileZone(label, multiple = true, accept = 'image/png,image/jpeg') {
  const el = h(`<label class="filedrop"><input type="file" accept="${accept}"${multiple ? ' multiple' : ''} hidden><span>${label}</span></label>`);
  const inp = el.querySelector('input');
  const span = el.querySelector('span');
  inp.onchange = () => {
    el.classList.toggle('ready', inp.files.length > 0);
    span.textContent = inp.files.length ? (multiple ? t('Выбрано файлов: {n}', { n: inp.files.length }) : inp.files[0].name) : label;
    el.dispatchEvent(new Event('changed'));
  };
  return el;
}

const warnList = w => (w && w.length) ? `<ul class="plain warn" style="padding-left:28px">${w.map(x => `<li>${esc(x)}</li>`).join('')}</ul>` : '';
const frameStrip = frames => (!frames || !frames.length) ? '' :
  `<div class="frame-strip">${frames.map(f => `<div class="frame-dot ${f.ok ? 'ok' : 'bad'}" title="${esc(f.error || t('СКО {v} пикс', { v: (f.rms_px || 0).toFixed(2) }))}">${f.index + 1}</div>`).join('')}</div>`;

// ── Калибровка ───────────────────────────────────────────────────────

function calib(box) {
  box.append(h(`<div>
    <p class="muted">${t('Нужна один раз для каждой камеры. Снимите шахматную мишень 10–20 раз под разными углами и по всем углам кадра, затем загрузите снимки. Программа посчитает параметры камеры и даст сохранить их в файл.')}</p>
    <div class="cols3">
      <label class="f">${t('Углов по горизонтали')}<input type="number" id="cCols" value="9" min="3">
        <small>${t('Внутренних углов, не клеток: доска 10×7 клеток — это 9×6 углов.')}</small></label>
      <label class="f">${t('Углов по вертикали')}<input type="number" id="cRows" value="6" min="3">
        <small>${t('Сумма сторон нечётная (9+6=15) — иначе ориентация неоднозначна.')}</small></label>
      <label class="f">${t('Клетка, мм')}<input type="number" id="cSq" value="30" step="0.1">
        <small>${t('Измерьте штангенциркулем по распечатке — принтеры масштабируют.')}</small></label>
    </div>
    <div id="cZone"></div>
    <div class="actions"><button class="btn primary" id="cRun" disabled>${t('Откалибровать')}</button><span id="cProg" class="muted"></span></div>
    <div id="cRes"></div>
  </div>`));
  const zone = fileZone(t('Выберите снимки мишени (можно сразу все)'));
  $('#cZone', box).append(zone);
  const inp = zone.querySelector('input');
  zone.addEventListener('changed', () => { $('#cRun', box).disabled = inp.files.length < 3; });
  $('#cRun', box).onclick = async () => {
    const btn = $('#cRun', box);
    btn.disabled = true;
    $('#cProg', box).innerHTML = `<span class="spin"></span> ${t('Обрабатываю {n} снимков — до минуты…', { n: inp.files.length })}`;
    const fd = new FormData();
    fd.append('cols', $('#cCols', box).value);
    fd.append('rows', $('#cRows', box).value);
    fd.append('square_mm', $('#cSq', box).value);
    for (const f of inp.files) fd.append('images', f);
    try {
      const d = await api('/api/optical/calibrate', { method: 'POST', form: fd });
      const grade = {
        good: ['good', t('хорошая — можно измерять')],
        ok: ['', t('приемлемая, но см. замечания')],
        bad: ['bad', t('плохая — переснимите серию')],
      }[d.quality] || ['', ''];
      const blob = URL.createObjectURL(new Blob([JSON.stringify(d.camera, null, 2)], { type: 'application/json' }));
      $('#cRes', box).innerHTML = `
        <div class="metrics">
          <div class="metric"><div class="v ${grade[0]}">${d.rms_px.toFixed(3)}</div><div class="k">${t('СКО, пикс')}</div></div>
          <div class="metric"><div class="v">${d.views_used}</div><div class="k">${t('кадров учтено')}</div></div>
          <div class="metric"><div class="v">${Math.round(d.tilt_spread_deg)}°</div><div class="k">${t('разброс наклона')}</div></div>
          <div class="metric"><div class="v">${Math.round(d.coverage_fraction * 100)}%</div><div class="k">${t('покрытие кадра')}</div></div>
        </div>
        <p><b>${t('Качество калибровки: {q}', { q: grade[1] })}</b></p>
        <div class="actions"><a class="btn primary" href="${blob}" download="camera.json">${t('Сохранить camera.json')}</a>
          <span class="muted" style="font-size:13px">${t('Файл понадобится при каждом замере этой камерой.')}</span></div>
        ${warnList(d.warnings)}`;
    } catch (e) {
      $('#cRes', box).innerHTML = `<div class="danger-box">${esc(e.message)}</div>`;
    } finally {
      $('#cProg', box).textContent = '';
      btn.disabled = false;
    }
  };
}

// ── Развал по фото ───────────────────────────────────────────────────

function camber(box) {
  box.append(h(`<div>
    <p class="muted">${t('Вывесите колесо, закрепите на диске мишень, поставьте камеру строго по уровню сбоку. Сделайте 4–6 снимков, проворачивая колесо на 10–20° между кадрами. Программа найдёт ось вращения колеса.')}</p>
    <div class="cols3">
      <label class="f">${t('Какое колесо')}<select id="kPos">${WHEELS.map(w => `<option value="${w.key}">${w.name}</option>`).join('')}</select></label>
      <label class="f">${t('Файл калибровки камеры')}<span id="kCam"></span></label>
      <label class="chk" style="margin-top:22px"><input type="checkbox" id="kLevel" checked><span>${t('Камера стояла строго по уровню')}</span></label>
    </div>
    <div id="kZone"></div>
    <div class="actions"><button class="btn primary" id="kRun" disabled>${t('Измерить развал')}</button><span id="kProg" class="muted"></span></div>
    <div id="kRes"></div>
  </div>`));
  const cam = fileZone('camera.json', false, 'application/json,.json');
  $('#kCam', box).append(cam);
  const zone = fileZone(t('Снимки колеса (4–6 штук)'));
  $('#kZone', box).append(zone);
  const ready = () => { $('#kRun', box).disabled = zone.querySelector('input').files.length < 3 || !cam.querySelector('input').files.length; };
  zone.addEventListener('changed', ready);
  cam.addEventListener('changed', ready);
  $('#kRun', box).onclick = async () => {
    const btn = $('#kRun', box);
    btn.disabled = true;
    $('#kProg', box).innerHTML = `<span class="spin"></span> ${t('Распознаю мишень…')}`;
    const fd = new FormData();
    fd.append('position', $('#kPos', box).value);
    fd.append('camera', cam.querySelector('input').files[0]);
    for (const f of zone.querySelector('input').files) fd.append('images', f);
    try {
      const d = await api('/api/optical/camber', { method: 'POST', form: fd });
      const w = d.warnings || [];
      if (!$('#kLevel', box).checked) w.unshift(t('Камера стояла не по уровню: развал смещён ровно на её наклон. Выставьте камеру по уровню и переснимите.'));
      $('#kRes', box).innerHTML = `
        <div class="metrics">
          <div class="metric"><div class="v">${fmtDM(d.camber_deg)}</div><div class="k">${t('развал, {wheel}', { wheel: esc(d.position_ru.toLowerCase()) })}</div></div>
          <div class="metric"><div class="v">${d.runout_deg.toFixed(1)}°</div><div class="k">${t('биение мишени')}</div></div>
          <div class="metric"><div class="v ${d.sweep_deg >= 15 ? '' : 'bad'}">${Math.round(d.sweep_deg)}°</div><div class="k">${t('поворот колеса')}</div></div>
          <div class="metric"><div class="v">${d.axis_residual_mm.toFixed(1)}</div><div class="k">${t('разброс оси, мм')}</div></div>
        </div>
        ${frameStrip(d.frames)}
        <div class="actions"><button class="btn" id="kSend">${t('Показать на экране регулировки')}</button></div>
        ${warnList(w)}`;
      $('#kSend', box).onclick = () => api('/api/live/manual', { method: 'POST', body: { wheel: d.position, camber: d.camber_deg } })
        .then(() => go('live')).catch(e => toast(e.message, true));
    } catch (e) {
      $('#kRes', box).innerHTML = `<div class="danger-box">${esc(e.message)}</div>${frameStrip(e.data && e.data.frames)}`;
    } finally {
      $('#kProg', box).textContent = '';
      btn.disabled = false;
    }
  };
}

// ── Полный замер ─────────────────────────────────────────────────────

function full(box) {
  const rim = state.session ? state.session.rim_diameter_in : 15;
  box.append(h(`<div>
    <p class="muted">${t('В каждый кадр вместе с мишенью на колесе должна попадать напольная мишень. Её плоскость служит плоскостью дороги, поэтому камеру можно держать как угодно и переносить между колёсами.')}</p>
    <div class="cols3">
      <label class="f">${t('Файл калибровки камеры')}<span id="fCam"></span></label>
      <label class="f">${t('Мишень на колесе: углы × углы × клетка, мм')}
        <span class="row" style="gap:6px"><input type="number" id="wC" value="8" style="width:70px"><input type="number" id="wR" value="5" style="width:70px"><input type="number" id="wS" value="32" step="0.1" style="width:90px"></span></label>
      <label class="f">${t('Обод, дюймы')}<input type="number" id="fRim" value="${rim}" step="0.5"></label>
    </div>
    <h3>${t('Напольные мишени')}</h3>
    <p class="muted" style="font-size:13px">${t('Одну мишень не видно от всех колёс — мешает машина. Поэтому их две, спереди и сзади, и они должны различаться размером. Клетку берите крупной (80–100 мм).')}</p>
    <div class="cols3">
      <label class="f">${t('Передняя')}<span class="row" style="gap:6px"><input type="number" id="r0C" value="7" style="width:70px"><input type="number" id="r0R" value="6" style="width:70px"><input type="number" id="r0S" value="100" step="0.1" style="width:90px"></span></label>
      <label class="f">${t('Задняя')}<span class="row" style="gap:6px"><input type="number" id="r1C" value="6" style="width:70px"><input type="number" id="r1R" value="5" style="width:70px"><input type="number" id="r1S" value="100" step="0.1" style="width:90px"></span></label>
      <label class="chk" style="margin-top:22px"><input type="checkbox" id="oneRef"><span>${t('Одна напольная мишень (связующие снимки не нужны)')}</span></label>
    </div>
    <div id="linkRow"></div>
    <h3>${t('Снимки колёс — по 4–6 кадров, проворачивая колесо на 10–20°')}</h3>
    <div class="wheelgrid" id="fWheels"></div>
    <div class="actions"><button class="btn primary" id="fRun" disabled>${t('Рассчитать сход-развал')}</button><span id="fProg" class="muted"></span></div>
    <div id="fRes"></div>
  </div>`));

  const cam = fileZone('camera.json', false, 'application/json,.json');
  $('#fCam', box).append(cam);
  const link = fileZone(t('Связующие снимки: в кадре видны ОБЕ напольные мишени (снимайте с поднятых рук)'));
  $('#linkRow', box).append(link);
  const zones = {};
  for (const w of WHEELS) {
    const card = h(`<div class="wcard"><h3>${w.name}</h3>
      <label class="f">${t('Напольная мишень в кадре')}<select data-ref="${w.key}"><option value="0"${w.front ? ' selected' : ''}>${t('Передняя')}</option><option value="1"${w.front ? '' : ' selected'}>${t('Задняя')}</option></select></label></div>`);
    zones[w.key] = fileZone(t('Выбрать снимки'));
    card.append(zones[w.key]);
    $('#fWheels', box).append(card);
  }
  const one = () => $('#oneRef', box).checked;
  const ready = () => {
    const ok = cam.querySelector('input').files.length > 0 &&
      WHEELS.every(w => zones[w.key].querySelector('input').files.length >= 3) &&
      (one() || link.querySelector('input').files.length > 0);
    $('#fRun', box).disabled = !ok;
  };
  [cam, link, ...Object.values(zones)].forEach(z => z.addEventListener('changed', ready));
  $('#oneRef', box).onchange = () => { $('#linkRow', box).classList.toggle('hidden', one()); ready(); };

  $('#fRun', box).onclick = async () => {
    const btn = $('#fRun', box);
    btn.disabled = true;
    $('#fProg', box).innerHTML = `<span class="spin"></span> ${t('Распознаю мишени на всех кадрах — до нескольких минут…')}`;
    const fd = new FormData();
    fd.append('camera', cam.querySelector('input').files[0]);
    fd.append('rim_diameter_in', $('#fRim', box).value);
    if (state.session && state.session.vehicle) fd.append('spec_id', state.session.vehicle.id);
    fd.append('wheel_cols', $('#wC', box).value); fd.append('wheel_rows', $('#wR', box).value); fd.append('wheel_square_mm', $('#wS', box).value);
    fd.append('ref0_cols', $('#r0C', box).value); fd.append('ref0_rows', $('#r0R', box).value); fd.append('ref0_square_mm', $('#r0S', box).value);
    if (!one()) {
      fd.append('ref1_cols', $('#r1C', box).value); fd.append('ref1_rows', $('#r1R', box).value); fd.append('ref1_square_mm', $('#r1S', box).value);
      for (const f of link.querySelector('input').files) fd.append('images_link', f);
    }
    for (const w of WHEELS) {
      for (const f of zones[w.key].querySelector('input').files) fd.append('images_' + w.key, f);
      fd.append('ref_' + w.key, one() ? '0' : $(`[data-ref="${w.key}"]`, box).value);
    }
    try {
      const d = await api('/api/optical/align', { method: 'POST', form: fd });
      const rows = (d.optical || []).map(o => `<tr><td>${esc(o.position_ru)}</td><td class="v">${o.used}/${o.total}</td>
        <td class="v">${Math.round(o.sweep_deg)}°</td><td class="v">${o.runout_deg.toFixed(1)}°</td><td class="v">${o.axis_residual_mm.toFixed(1)}</td></tr>`).join('');
      $('#fRes', box).innerHTML = `
        <div class="ok-box" style="margin-top:12px">${t('Замер выполнен — углы переданы на экран регулировки.')}</div>
        <table class="params" style="margin-top:10px"><thead><tr><th>${t('Колесо')}</th><th>${t('Кадров')}</th><th>${t('Поворот')}</th><th>${t('Биение мишени')}</th><th>${t('Разброс оси, мм')}</th></tr></thead><tbody>${rows}</tbody></table>
        ${warnList(d.result && d.result.warnings)}
        <div class="actions"><button class="btn primary" id="fGo">${t('Открыть экран регулировки →')}</button></div>`;
      $('#fGo', box).onclick = () => go('live');
    } catch (e) {
      $('#fRes', box).innerHTML = `<div class="danger-box">${esc(e.message)}</div>${frameStrip(e.data && e.data.frames)}`;
    } finally {
      $('#fProg', box).textContent = '';
      btn.disabled = false;
    }
  };
}

// ── Живой режим ──────────────────────────────────────────────────────
// Телефон на штативе смотрит на колесо и мишень на полу и шлёт кадры по
// Wi-Fi; каждый кадр обновляет угол на экране регулировки. Здесь — настройка
// мишеней и состояние: какая камера откалибрована, у каких колёс учтено
// биение, какие колёса уже видны.

function livecam(box) {
  box.append(h(`<div>
    <div class="note">${t('Как на профессиональном 3D-стенде: сначала один раз учитывается биение каждого колеса, потом телефон на штативе смотрит на колесо — и развал со схождением меняются на экране регулировки 3–4 раза в секунду, пока вы крутите тягу.')}</div>
    <ol class="steps" style="margin-top:12px">
      <li><b>${t('Напечатайте мишени')}</b><div>${t('Ниже — кнопка печати. На каждом колесе своя мишень: программа сама узнаёт, какое колесо видит. Проверьте линейку 100 мм на листе и наклейте мишени на жёсткий ровный лист.')}</div></li>
      <li><b>${t('Включите телефон по Wi-Fi')}</b><div>${t('«Замер → Телефон на колесе», отсканируйте код, на телефоне нажмите «Телефон как камера».')}</div></li>
      <li><b>${t('Калибровка камеры')}</b><div>${t('Один раз для телефона. Показывайте ему мишень с колеса под разными углами и в разных частях кадра, пока полоска не заполнится. Телефон держите горизонтально — и так же потом при замере.')}</div></li>
      <li><b>${t('Биение каждого колеса')}</b><div>${t('Вывесите колесо, режим «Биение», и медленно проверните колесо рукой на четверть оборота и больше. Мишень на колесе и мишень на полу должны быть в кадре.')}</div></li>
      <li><b>${t('Связь напольных мишеней')}</b><div>${t('Если мишеней на полу две — в режиме «Замер» снимите с поднятых рук два-три кадра, где видны обе.')}</div></li>
      <li><b>${t('Замер')}</b><div>${t('Машина стоит на месте. Покажите камере по очереди все четыре колеса — после этого появится схождение. Дальше ставьте телефон у колеса, которое регулируете.')}</div></li>
      <li><b>${t('Кастер')}</b><div>${t('Передние колёса на поворотных кругах, педаль тормоза зажата упором. Режим «Кастер камерой»: медленно поверните руль на 15–20° в одну сторону, потом в другую.')}</div></li>
    </ol>

    <h3>${t('Мишени')}</h3>
    <div id="lTargets"></div>
    <div class="actions"><button class="btn" id="lSave">${t('Сохранить размеры мишеней')}</button>
      <button class="btn" id="lOne">${t('Одна мишень на все колёса')}</button>
      <button class="btn" id="lFour">${t('Разные мишени (по умолчанию)')}</button>
      <button class="btn danger" id="lReset">${t('Начать заново (другая машина)')}</button></div>
    <p class="muted" id="lMode" style="font-size:13px"></p>

    <h3>${t('Печать мишеней')}</h3>
    <div class="row" style="gap:10px;align-items:flex-end;flex-wrap:wrap">
      <label class="f" style="min-width:180px">${t('Бумага')}<select id="lPaper">
        <option value="a4">A4</option><option value="a3">A3</option><option value="letter">Letter</option>
        <option value="single">${t('Один лист по размеру мишени (типография, плоттер)')}</option></select></label>
      <a class="btn primary" id="lPrintWheels" download="wheel-targets.pdf">${t('PDF: мишени колёс')}</a>
      <a class="btn" id="lPrintFloor" download="floor-targets.pdf">${t('PDF: напольные мишени')}</a>
    </div>
    <div id="lPlan"></div>
    <ul class="plain muted" style="font-size:13px;margin-top:8px">
      <li>${t('Печатайте в масштабе 100 % («Фактический размер»), без «Вписать в страницу». Затем измерьте линейку внизу листа: ровно 100 мм. Если нет — принтер масштабирует, измерьте клетку штангенциркулем и впишите настоящий размер выше.')}</li>
      <li>${t('Мишень должна быть плоской: наклейте на фанеру, ДСП, стекло или композитную панель. Изогнутая бумага даёт ошибку сильнее кривого крепления.')}</li>
      <li>${t('Мишень из нескольких листов: обрежьте листы по меткам у краёв и сложите встык — узор продолжается с листа на лист. Напольную мишень проще заказать одним листом в типографии.')}</li>
    </ul>

    <h3>${t('Колёса')}</h3>
    <div id="lWheels"></div>
    <h3>${t('Камеры')}</h3>
    <div id="lCams"></div>
  </div>`));

  let st = null, filled = false;
  const inputs3 = (id, tg) => `<span class="row" style="gap:6px">
    <input type="number" id="${id}C" value="${tg ? tg.cols : ''}" style="width:64px">
    <input type="number" id="${id}R" value="${tg ? tg.rows : ''}" style="width:64px">
    <input type="number" id="${id}S" value="${tg ? tg.square_mm : ''}" step="0.1" style="width:80px"></span>`;
  const readT = id => ({ cols: Number($(`#${id}C`, box).value), rows: Number($(`#${id}R`, box).value), square_mm: Number($(`#${id}S`, box).value) });
  const fill = () => {
    const wheels = WHEELS.map(w => `<label class="f">${w.name}: ${t('углы × углы × клетка, мм')}${inputs3('lw' + w.key, st.wheel_targets[w.key])}</label>`).join('');
    const r0 = st.refs[0], r1 = st.refs[1];
    $('#lTargets', box).innerHTML = `<div class="cols3">${wheels}
      <label class="f">${t('Напольная передняя')}${inputs3('l0', r0)}</label>
      <label class="f">${t('Напольная задняя')}${inputs3('l1', r1)}</label></div>`;
  };
  const planLink = () => {
    const paper = $('#lPaper', box).value;
    $('#lPrintWheels', box).href = `/api/targets/pdf?which=wheels&paper=${paper}`;
    $('#lPrintFloor', box).href = `/api/targets/pdf?which=floor&paper=${paper}`;
    api(`/api/targets/plan?paper=${paper}`).then(plans => {
      $('#lPlan', box).innerHTML = `<table class="params" style="margin-top:8px"><thead><tr><th>${t('Мишень')}</th><th>${t('Размер')}</th><th>${t('Листов')}</th></tr></thead><tbody>
        ${plans.map(pl => `<tr><td>${esc(pl.name || pl.label)}</td><td>${pl.target.cols}×${pl.target.rows}, ${pl.target.square_mm} ${t('мм')} — ${Math.round(pl.plan.board_w_mm)}×${Math.round(pl.plan.board_h_mm)} ${t('мм')}</td>
          <td class="v">${pl.error ? `<span class="no">${esc(pl.error)}</span>` : pl.plan.pages}</td></tr>`).join('')}</tbody></table>`;
    }).catch(() => {});
  };
  const draw = s2 => {
    st = s2;
    if (!filled) { fill(); filled = true; }
    $('#lMode', box).textContent = st.distinct
      ? t('Мишени на колёсах разные — программа сама узнаёт колесо по мишени.')
      : t('На всех колёсах одна и та же мишень — на телефоне выбирайте, какое колесо он видит.');
    const linked = new Set(st.linked);
    const rows = WHEELS.map(w => {
      const x = st.wheels[w.key] || {};
      const spin = x.clamped ? `<span class="yes">${t('учтено, перекос {v}°', { v: x.runout_deg.toFixed(1) })}</span>`
        : x.spin_deg ? t('проворот {v}°', { v: Math.round(x.spin_deg) }) : `<span class="no">${t('нужно')}</span>`;
      const seen = x.seen_ago_s === undefined ? '—'
        : x.seen_ago_s < 3 ? `<span class="yes">${t('сейчас')}</span>` : t('{s} с назад', { s: Math.round(x.seen_ago_s) });
      const caster = x.caster === undefined ? '—' : `${fmtDM(x.caster)}<br><span class="dim" style="font-size:12px">SAI ${fmtDM(x.sai)}</span>`;
      return `<tr><td>${w.name}</td><td>${spin}</td><td>${seen}</td><td class="v">${x.camber === undefined ? '—' : fmtDM(x.camber)}</td><td class="v">${w.front ? caster : ''}</td></tr>`;
    }).join('');
    $('#lWheels', box).innerHTML = `<table class="params"><thead><tr><th>${t('Колесо')}</th><th>${t('Биение')}</th><th>${t('Видно')}</th><th>${t('Развал')}</th><th>${t('Кастер')}</th></tr></thead><tbody>${rows}</tbody></table>
      <p class="muted" style="font-size:13px">${st.refs.length > 1
        ? (linked.size > 1 ? t('Напольные мишени связаны.') : t('Напольные мишени ещё не связаны: нужен кадр, где видны обе.'))
        : ''}</p>`;
    $('#lCams', box).innerHTML = st.cameras.length
      ? `<table class="params"><tbody>${st.cameras.map(c => `<tr><td>${esc(c.name)}</td><td>${esc(c.size)}</td>
          <td class="v">${t('СКО {v} пикс', { v: c.rms_px.toFixed(2) })}</td>
          <td style="width:1%"><button class="btn small" data-forget-cam="${esc(c.id)}">${t('Забыть')}</button></td></tr>`).join('')}</tbody></table>`
      : `<p class="muted">${t('Пока ни одной: откалибруйте камеру телефона в режиме «Калибровка камеры».')}</p>`;
    box.querySelectorAll('[data-forget-cam]').forEach(b => b.onclick = () => post({ forget_camera: b.dataset.forgetCam }));
  };
  const load = () => api('/api/optical/live').then(draw).catch(() => {});
  const post = body => api('/api/optical/live', { method: 'POST', body })
    .then(s2 => { filled = false; draw(s2); planLink(); }).catch(e => toast(e.message, true));

  $('#lSave', box).onclick = () => {
    const wheel_targets = Object.fromEntries(WHEELS.map(w => [w.key, readT('lw' + w.key)]));
    const refs = [readT('l0')];
    const r1 = readT('l1');
    if (r1.cols && r1.rows && r1.square_mm) refs.push(r1);
    post({ wheel_targets, refs, rim_in: state.session ? state.session.rim_diameter_in : 15 });
  };
  $('#lOne', box).onclick = () => post({ wheel_target: { cols: 8, rows: 5, square_mm: 25 } });
  $('#lFour', box).onclick = () => post({ wheel_targets: {
    FL: { cols: 11, rows: 4, square_mm: 22 }, FR: { cols: 10, rows: 5, square_mm: 24 },
    RL: { cols: 9, rows: 6, square_mm: 24 }, RR: { cols: 8, rows: 7, square_mm: 21 } } });
  $('#lReset', box).onclick = () => {
    if (confirm(t('Забыть биение колёс, связь мишеней и увиденные колёса? Калибровка камер сохранится.'))) post({ reset: true });
  };
  $('#lPaper', box).onchange = planLink;
  load();
  planLink();
  livePoll = setInterval(() => { if (document.body.contains(box)) load(); else clearInterval(livePoll); }, 1500);
}
