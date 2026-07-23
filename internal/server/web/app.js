'use strict';

/* Интерфейс открытого стенда сход-развала.
   Вся математика — на сервере (Go). Здесь только ввод, вывод и чертёж. */

const $  = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];

const WHEELS = [
  { key: 'FL', name: 'Переднее левое',  front: true  },
  { key: 'FR', name: 'Переднее правое', front: true  },
  { key: 'RL', name: 'Заднее левое',    front: false },
  { key: 'RR', name: 'Заднее правое',   front: false },
];

const state = { spec: null, last: null };

/* ── Форматирование ──────────────────────────────────────────────────── */

// Углы приходят с сервера в десятичных градусах. В градусах-минутах читать
// привычнее по бумажным мануалам, поэтому показываем обе формы.
function fmtDegMin(deg) {
  if (deg === null || deg === undefined) return '—';
  const sign = deg < 0 ? '-' : '+';
  const a = Math.abs(deg);
  let d = Math.floor(a), m = (a - d) * 60;
  if (m >= 59.995) { d++; m = 0; }
  return `${sign}${d}°${m.toFixed(0).padStart(2, '0')}'`;
}
const fmtDeg = d => (d === null || d === undefined) ? '—' : `${d >= 0 ? '+' : ''}${d.toFixed(2)}°`;
const fmtMM  = v => (v === null || v === undefined) ? '' : `${v >= 0 ? '+' : ''}${v.toFixed(1)} мм`;

const STATUS_RU = { good: 'в допуске', marginal: 'на границе допуска', bad: 'вне допуска', no_spec: 'нет данных' };

/* ── Навигация по шагам ──────────────────────────────────────────────── */

function goto(step) {
  $$('.step').forEach(s => s.classList.toggle('active', s.id === 'step-' + step));
  $$('#stepnav button').forEach(b => b.classList.toggle('active', b.dataset.step === step));
  window.scrollTo({ top: 0, behavior: 'smooth' });
}
$$('#stepnav button').forEach(b => b.onclick = () => goto(b.dataset.step));

/* ── Шаг 1: поиск автомобиля ─────────────────────────────────────────── */

function specCard(s) {
  const el = document.createElement('button');
  el.className = 'card';
  el.innerHTML = `
    <b></b>
    <span class="badge ${s.source_kind}"></span>
    <span class="note"></span>`;
  el.querySelector('b').textContent = s.title;
  el.querySelector('.badge').textContent = s.source_label;
  el.querySelector('.note').textContent = s.notes || '';
  el.onclick = () => chooseSpec(s);
  return el;
}

let searchTimer;
async function runSearch() {
  const q = $('#q').value.trim();
  const year = $('#year').value.trim();
  const r = await fetch(`/api/specs/search?q=${encodeURIComponent(q)}&year=${encodeURIComponent(year)}`);
  const data = await r.json();

  const box = $('#results');
  box.replaceChildren();
  if (!data.results.length && q) {
    const p = document.createElement('p');
    p.className = 'lede';
    p.textContent = 'Ничего не найдено. Попробуйте другое написание или возьмите ориентир по классу ниже.';
    box.append(p);
    $('#guidance-block').open = true;
  }
  data.results.forEach(s => box.append(specCard(s)));

  const g = $('#guidance');
  g.replaceChildren();
  (data.guidance || []).forEach(s => g.append(specCard(s)));
}
$('#q').oninput = $('#year').oninput = () => { clearTimeout(searchTimer); searchTimer = setTimeout(runSearch, 180); };

async function chooseSpec(summary) {
  const r = await fetch('/api/specs/' + encodeURIComponent(summary.id));
  const data = await r.json();
  state.spec = data.spec;

  const c = $('#chosen');
  c.replaceChildren();
  const left = document.createElement('div');
  left.innerHTML = `<b></b><br><span class="badge ${data.spec.source.kind}"></span>`;
  left.querySelector('b').textContent = data.title;
  left.querySelector('.badge').textContent = data.source;
  const change = document.createElement('button');
  change.className = 'ghost';
  change.textContent = 'Выбрать другой';
  change.onclick = () => goto('car');
  c.append(left, change);

  if (data.disclaimer) {
    const d = document.createElement('div');
    d.className = 'disclaimer' + (data.spec.source.kind === 'community' ? ' soft' : '');
    d.textContent = data.disclaimer;
    c.after(d);
  }
  if (data.spec.rim_diameter_in) $('#rim').value = data.spec.rim_diameter_in;

  goto('measure');
}

/* ── Шаг 2: форма замеров ────────────────────────────────────────────── */

function buildWheelForm() {
  const box = $('#wheels');
  box.replaceChildren();
  for (const w of WHEELS) {
    const card = document.createElement('div');
    card.className = 'wheelcard';
    card.innerHTML = `
      <h3>${w.name}</h3>

      <div class="pair">
        <label>Развал, ° (0°)<input type="number" step="0.01" data-w="${w.key}" data-f="camber_0"></label>
        <label>Развал, ° (180°)<input type="number" step="0.01" data-w="${w.key}" data-f="camber_180"></label>
      </div>
      <small style="color:var(--nospec);display:block;margin:-.4rem 0 .8rem">
        Второй замер — после прокатки на пол-оборота колеса, на том же месте обода.
        Программа усреднит и покажет биение диска.</small>

      <div class="pair">
        <label>Струна → обод, спереди, мм<input type="number" step="0.1" data-w="${w.key}" data-f="toe_front_mm"></label>
        <label>Струна → обод, сзади, мм<input type="number" step="0.1" data-w="${w.key}" data-f="toe_rear_mm"></label>
      </div>

      ${w.front ? `
      <details>
        <summary>Кастер — замер с поворотом колеса</summary>
        <div class="pair" style="margin-top:.6rem">
          <label>Развал, повёрнуто НАРУЖУ, °<input type="number" step="0.01" data-w="${w.key}" data-f="sweep_out"></label>
          <label>Развал, повёрнуто ВНУТРЬ, °<input type="number" step="0.01" data-w="${w.key}" data-f="sweep_in"></label>
        </div>
        <label>Угол поворота в каждую сторону, °
          <input type="number" step="1" value="20" data-w="${w.key}" data-f="half_sweep"></label>
        <small style="color:var(--nospec)">«Наружу» — от центра автомобиля. Один поворот руля влево
          даёт «наружу» для левого колеса и «внутрь» для правого: оба замера снимаются за один проход.</small>
      </details>` : ''}
    `;
    box.append(card);
  }
}

const val = (w, f) => {
  const el = document.querySelector(`[data-w="${w}"][data-f="${f}"]`);
  if (!el || el.value === '') return null;
  const n = parseFloat(el.value);
  return Number.isFinite(n) ? n : null;
};

/* Живая проверка струн: считаем то же условие, что и сервер, чтобы человек
   поправил натяжку до замеров, а не узнал об ошибке после. */
function checkBox() {
  const lf = parseFloat($('#boxLF').value), lr = parseFloat($('#boxLR').value);
  const rf = parseFloat($('#boxRF').value), rr = parseFloat($('#boxRR').value);
  const tf = parseFloat($('#trackF').value), tr = parseFloat($('#trackR').value);
  const hint = $('#boxhint');
  if (![lf, lr, rf, rr].every(Number.isFinite)) { hint.textContent = ''; return; }
  if (!Number.isFinite(tf) || !Number.isFinite(tr)) {
    hint.innerHTML = '<span class="no">Введите колеи осей — без них проверку сделать нельзя.</span>';
    return;
  }
  const want = (tr - tf) / 2;
  const rows = [['Левая', lf - lr], ['Правая', rf - rr]].map(([n, got]) => {
    const off = want - got;
    return Math.abs(off) <= 1
      ? `<div><span class="ok">✓ ${n} струна параллельна оси автомобиля.</span></div>`
      : `<div><span class="no">✗ ${n} струна: разница «перед − зад» = ${got.toFixed(1)} мм,
         нужно ${want.toFixed(1)} мм. Сдвиньте передний конец на ${off.toFixed(1)} мм.</span></div>`;
  });
  hint.innerHTML = rows.join('') +
    `<div style="color:var(--nospec);margin-top:.4rem">Требуемая разница ${want.toFixed(1)} мм следует
     из разных колей осей (${tf} и ${tr} мм) — одинаковые отступы здесь были бы ошибкой.</div>`;
}
['boxLF', 'boxLR', 'boxRF', 'boxRR', 'trackF', 'trackR'].forEach(id => $('#' + id).oninput = checkBox);

$('#btn-fill').onclick = () => {
  const demo = {
    FL: [-0.85, -0.95, 51.2, 50.0, -1.35, -3.15],
    FR: [-0.15, -0.25, 51.2, 50.0, -3.55, -1.30],
    RL: [-1.40, -1.40, 51.6, 50.0, null, null],
    RR: [-1.10, -1.10, 50.5, 50.0, null, null],
  };
  for (const [k, v] of Object.entries(demo)) {
    const set = (f, x) => { const e = document.querySelector(`[data-w="${k}"][data-f="${f}"]`); if (e && x !== null) e.value = x; };
    set('camber_0', v[0]); set('camber_180', v[1]);
    set('toe_front_mm', v[2]); set('toe_rear_mm', v[3]);
    set('sweep_out', v[4]); set('sweep_in', v[5]);
  }
  $('#trackF').value ||= 1520; $('#trackR').value ||= 1510;
  $('#boxLF').value = 100; $('#boxLR').value = 105;
  $('#boxRF').value = 100; $('#boxRR').value = 105;
  checkBox();
};

$('#btn-calc').onclick = async () => {
  $('#measure-error').textContent = '';
  const rim = parseFloat($('#rim').value);
  if (!Number.isFinite(rim) || rim <= 0) {
    $('#measure-error').textContent = 'Укажите диаметр обода — без него схождение в миллиметрах не перевести в угол.';
    return;
  }

  const wheels = {};
  for (const w of WHEELS) {
    const c0 = val(w.key, 'camber_0'), c180 = val(w.key, 'camber_180');
    const tf = val(w.key, 'toe_front_mm'), tr = val(w.key, 'toe_rear_mm');
    if (c0 === null || tf === null || tr === null) {
      $('#measure-error').textContent = `Не заполнены замеры: ${w.name}. Нужны развал и оба расстояния до струны.`;
      return;
    }
    const e = { camber_0: c0, camber_180: c180 ?? 0, has_180: c180 !== null, toe_front_mm: tf, toe_rear_mm: tr };
    const so = val(w.key, 'sweep_out'), si = val(w.key, 'sweep_in');
    if (w.front && so !== null && si !== null) {
      e.sweep = { camber_out: so, camber_in: si, half_sweep_deg: val(w.key, 'half_sweep') ?? 20 };
    }
    wheels[w.key] = e;
  }

  const body = {
    spec_id: state.spec ? state.spec.id : '',
    rim_diameter_in: rim,
    track_front_mm: parseFloat($('#trackF').value) || 0,
    track_rear_mm: parseFloat($('#trackR').value) || 0,
    wheels,
  };
  const bf = parseFloat($('#boxLF').value);
  if (Number.isFinite(bf)) {
    body.box = {
      left_front_mm: bf, left_rear_mm: parseFloat($('#boxLR').value) || 0,
      right_front_mm: parseFloat($('#boxRF').value) || 0, right_rear_mm: parseFloat($('#boxRR').value) || 0,
    };
  }

  const r = await fetch('/api/measure/manual', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
  });
  const data = await r.json();
  if (!r.ok) { $('#measure-error').textContent = data.error || 'Ошибка расчёта'; return; }
  render(data);
  goto('result');
};

/* ── Шаг 3: результат ────────────────────────────────────────────────── */

// Схема автомобиля сверху. Схождение и развал усилены, иначе на реальных
// долях градуса не видно ничего — коэффициент подписан, чтобы никто не читал
// картинку как масштабную.
const TOE_GAIN = 14, CAMBER_GAIN = 2.4;

function diagram(res) {
  const pos = { FL: [70, 62], FR: [190, 62], RL: [70, 232], RR: [190, 232] };
  let wheels = '', labels = '';

  for (const [k, [x, y]] of Object.entries(pos)) {
    const w = res.wheels[k];
    const toe = (w.toe_thrust || 0) * TOE_GAIN;
    // Схождение внутрь = передний край колеса к центру машины.
    const rot = (k[1] === 'L' ? toe : -toe);
    const cam = (w.camber || 0) * CAMBER_GAIN;
    const lean = (k[1] === 'L' ? -cam : cam);

    wheels += `
      <g transform="translate(${x} ${y}) rotate(${rot})">
        <rect x="-7" y="-26" width="14" height="52" rx="3"
              fill="#232c39" stroke="#3fb950" stroke-width="1.5"
              transform="skewX(${lean.toFixed(2)})"/>
        <line x1="0" y1="-34" x2="0" y2="34" stroke="#ffb02e" stroke-width="1" stroke-dasharray="3 3"/>
      </g>`;
    const tx = k[1] === 'L' ? x - 56 : x + 12;
    labels += `
      <text x="${tx}" y="${y - 4}" fill="#8b98a9" font-size="9">${fmtDegMin(w.camber)}</text>
      <text x="${tx}" y="${y + 8}" fill="#8b98a9" font-size="9">${fmtDegMin(w.toe_thrust)}</text>`;
  }

  // Геометрическая ось и линия тяги.
  const thrust = (res.thrust_angle || 0) * TOE_GAIN;
  const dx = Math.tan(thrust * Math.PI / 180) * 170;

  return `
  <figure class="diagram">
    <svg viewBox="0 0 260 300" role="img" aria-label="Схема углов установки колёс">
      <rect x="40" y="40" width="180" height="214" rx="26" fill="#151b24" stroke="#263041"/>
      <line x1="130" y1="30" x2="130" y2="270" stroke="#263041" stroke-width="1" stroke-dasharray="5 4"/>
      <line x1="130" y1="232" x2="${(130 + dx).toFixed(1)}" y2="52" stroke="#f85149" stroke-width="1.5"/>
      <text x="130" y="22" fill="#8b98a9" font-size="9" text-anchor="middle">перёд</text>
      ${wheels}${labels}
      <text x="130" y="288" fill="#f85149" font-size="9" text-anchor="middle">
        линия тяги ${fmtDegMin(res.thrust_angle)}</text>
    </svg>
    <figcaption>Вид сверху. Углы увеличены в ${TOE_GAIN}× (схождение) и ${CAMBER_GAIN}× (развал)
      — иначе доли градуса не разглядеть. Красная линия — направление, куда машину ведёт задняя ось.</figcaption>
  </figure>`;
}

function paramRows(report) {
  const groups = { front: 'Передняя ось', rear: 'Задняя ось', vehicle: 'Автомобиль' };
  let html = '';
  for (const [axle, title] of Object.entries(groups)) {
    const rows = report.params.filter(p => p.axle === axle);
    if (!rows.length) continue;
    html += `<tr class="axlehead"><td colspan="4">${title}</td></tr>`;
    for (const p of rows) {
      const spec = p.spec
        ? `${fmtDegMin(p.spec.min)} … ${fmtDegMin(p.spec.max)}`
        : '<span style="opacity:.5">нет данных</span>';
      const mm = (p.measured_mm !== undefined && p.measured_mm !== null)
        ? `<br><span style="color:var(--muted);font-size:.8rem">${fmtMM(p.measured_mm)}</span>` : '';
      html += `
        <tr>
          <td><span class="st ${p.status}"></span>${p.label}</td>
          <td class="val">${fmtDegMin(p.measured)}<br>
              <span style="color:var(--muted);font-size:.8rem">${fmtDeg(p.measured)}</span>${mm}</td>
          <td class="spec">${spec}</td>
          <td class="advice">${p.advice || ''}${p.method && !p.advice ? p.method : ''}</td>
        </tr>`;
    }
  }
  return html;
}

function render(data) {
  state.last = data;
  const { result: res, report: rep } = data;
  const st = rep.overall_status || 'no_spec';

  const verdictText = {
    good: 'Все углы в допуске',
    marginal: 'Углы в допуске, но близко к границе',
    bad: `Вне допуска: ${rep.out_of_spec} ${rep.out_of_spec === 1 ? 'параметр' : 'параметра(ов)'}`,
    no_spec: 'Углы измерены, допуски неизвестны',
  }[st];

  const warnings = (res.warnings || []).concat(
    Object.values(res.wheels).flatMap(w => (w.quality && w.quality.Warnings) || []));

  $('#result-body').innerHTML = `
    <div class="verdict ${st}">
      <span class="dot"></span>
      <div>
        <b>${verdictText}</b><br>
        <span>${rep.spec_title ? rep.spec_title + ' · ' + rep.source_label : 'Автомобиль не выбран — сравнение не выполнялось'}</span>
      </div>
    </div>

    ${rep.disclaimer ? `<div class="disclaimer">${rep.disclaimer}</div>` : ''}
    ${rep.conditions_ru ? `<div class="disclaimer soft"><b>Условия замера:</b> ${rep.conditions_ru}</div>` : ''}

    <div class="layout">
      ${diagram(res)}
      <div class="panel">
        <h2 style="margin-top:0">Измеренные углы</h2>
        <table class="params">
          <thead><tr><th>Параметр</th><th>Измерено</th><th>Допуск</th><th>Что делать</th></tr></thead>
          <tbody>${paramRows(rep)}</tbody>
        </table>
      </div>
    </div>

    ${warnings.length ? `<div class="panel" style="margin-top:1.25rem">
      <h2 style="margin-top:0">На что обратить внимание</h2>
      <ul class="warnlist">${warnings.map(w => `<li>${w}</li>`).join('')}</ul>
    </div>` : ''}

    <div class="panel" style="margin-top:1.25rem">
      <h2 style="margin-top:0">Порядок регулировки</h2>
      <ol class="steps-list">
        ${rep.steps.map(s => `
          <li><b>${s.title}</b><p>${s.detail}</p>
          ${s.why ? `<div class="why">${s.why}</div>` : ''}</li>`).join('')}
      </ol>
    </div>

    <div class="actions">
      <button class="ghost" onclick="window.print()">Распечатать протокол</button>
      <button class="ghost" id="btn-back">Вернуться к замерам</button>
    </div>`;

  $('#btn-back').onclick = () => goto('measure');
}

/* ── Демо и справка ──────────────────────────────────────────────────── */

$('#btn-demo').onclick = async () => {
  const r = await fetch('/api/demo');
  render(await r.json());
  goto('result');
};
$('#btn-help').onclick = () => $('#help').classList.add('open');

// Every modal closes the same way: click the backdrop, the ✕, or press Escape.
$$('.modal').forEach(m => {
  m.onclick = e => {
    if (e.target === m || e.target.dataset.close !== undefined) m.classList.remove('open');
  };
});
document.addEventListener('keydown', e => {
  if (e.key === 'Escape') $$('.modal').forEach(m => m.classList.remove('open'));
});

/* ── Оптический режим ─────────────────────────────────────────────────── */

$('#btn-optical').onclick = () => $('#optical').classList.add('open');

$$('.opt-tab').forEach(b => b.onclick = () => {
  $$('.opt-tab').forEach(x => x.classList.toggle('active', x === b));
  $$('.opt-pane').forEach(p => p.classList.toggle('active', p.id === 'opt-' + b.dataset.opt));
});

// Wire a hidden file input to its drop-zone label and a count readout.
function fileZone(inputId, countId, runBtn, minCount) {
  const input = $('#' + inputId);
  const zone = input.closest('.filedrop');
  const count = $('#' + countId);
  input.onchange = () => {
    const n = input.files.length;
    if (zone) zone.classList.toggle('ready', n > 0);
    count.textContent = n ? `Выбрано снимков: ${n}` : '';
    checkReady();
  };
  return input;
}

const calibInput = fileZone('calibFiles', 'calibCount');
const camberInput = fileZone('camberFiles', 'camberCount');

function checkReady() {
  $('#btn-calib-run').disabled = calibInput.files.length < 3;
  $('#btn-camber-run').disabled =
    camberInput.files.length < 3 || !$('#camberCamera').files.length;
}
$('#camberCamera').onchange = checkReady;

const degMin = deg => fmtDegMin(deg);

// ── Калибровка ──
$('#btn-calib-run').onclick = async () => {
  const btn = $('#btn-calib-run');
  const out = $('#calibResult');
  btn.disabled = true;
  $('#calibProgress').innerHTML = `<span class="spin"></span> Обработка ${calibInput.files.length} снимков — это может занять до минуты…`;
  out.innerHTML = '';

  const fd = new FormData();
  fd.append('cols', $('#calibCols').value);
  fd.append('rows', $('#calibRows').value);
  fd.append('square_mm', $('#calibSquare').value);
  for (const f of calibInput.files) fd.append('images', f);

  try {
    const r = await fetch('/api/optical/calibrate', { method: 'POST', body: fd });
    const d = await r.json();
    $('#calibProgress').textContent = '';
    if (!r.ok) { out.innerHTML = `<div class="error">${d.error || 'Ошибка калибровки'}</div>`; return; }
    renderCalib(d);
  } catch (e) {
    $('#calibProgress').textContent = '';
    out.innerHTML = `<div class="error">${e}</div>`;
  } finally {
    btn.disabled = false;
  }
};

function renderCalib(d) {
  const grade = { good: ['good', 'Хорошая — можно измерять'],
                  ok: ['', 'Приемлемая, но см. замечания'],
                  bad: ['bad', 'Плохая — переснимите серию'] }[d.quality] || ['', ''];
  const json = JSON.stringify(d.camera, null, 2);
  const blob = URL.createObjectURL(new Blob([json], { type: 'application/json' }));

  $('#calibResult').innerHTML = `
    <div class="opt-metrics">
      <div class="metric"><div class="v ${grade[0]}">${d.rms_px.toFixed(3)}</div><div class="k">СКО, пикс</div></div>
      <div class="metric"><div class="v">${d.views_used}</div><div class="k">кадров учтено</div></div>
      <div class="metric"><div class="v">${Math.round(d.tilt_spread_deg)}°</div><div class="k">разброс наклона</div></div>
      <div class="metric"><div class="v">${Math.round(d.coverage_fraction * 100)}%</div><div class="k">покрытие кадра</div></div>
    </div>
    <p><b>Качество: ${grade[1]}</b></p>
    <div class="copybar">
      <a class="primary" style="text-decoration:none;padding:.55rem 1rem;border-radius:8px" href="${blob}" download="camera.json">Сохранить camera.json</a>
      <span style="color:var(--muted);font-size:.84rem">Этот файл понадобится на вкладке «Развал по фото» и при каждом замере.</span>
    </div>
    ${warnBlock(d.warnings)}`;
}

// ── Развал по фото ──
$('#btn-camber-run').onclick = async () => {
  const btn = $('#btn-camber-run');
  const out = $('#camberResult');
  btn.disabled = true;
  $('#camberProgress').innerHTML = `<span class="spin"></span> Распознаю мишень на ${camberInput.files.length} кадрах…`;
  out.innerHTML = '';

  const fd = new FormData();
  fd.append('position', $('#camberPos').value);
  fd.append('camera', $('#camberCamera').files[0]);
  for (const f of camberInput.files) fd.append('images', f);
  // The server measures relative to gravity, and with no tilt data it assumes a
  // level camera. If the operator has told us it was not level, we cannot fix
  // that here — so we say so plainly rather than print a number that is wrong by
  // the unknown tilt.
  const notLevel = !$('#camberLevel').checked;

  try {
    const r = await fetch('/api/optical/camber', { method: 'POST', body: fd });
    const d = await r.json();
    $('#camberProgress').textContent = '';
    if (!r.ok) {
      out.innerHTML = `<div class="error">${d.error || 'Не удалось измерить'}</div>` + frameStrip(d.frames);
      return;
    }
    if (notLevel) {
      d.warnings = ['Вы отметили, что камера стояла не по уровню. Показанный развал смещён ровно на ' +
        'наклон камеры — выставьте её по пузырьковому уровню и переснимите. Учесть наклон камеры ' +
        'программа пока не умеет.', ...(d.warnings || [])];
    }
    renderCamber(d);
  } catch (e) {
    $('#camberProgress').textContent = '';
    out.innerHTML = `<div class="error">${e}</div>`;
  } finally {
    btn.disabled = false;
  }
};

function renderCamber(d) {
  const sweepOK = d.sweep_deg >= 15;
  $('#camberResult').innerHTML = `
    <div class="bigcamber">
      <div class="num">${degMin(d.camber_deg)}</div>
      <div class="lbl">развал, ${d.position_ru.toLowerCase()} · ${d.camber_deg.toFixed(2)}°</div>
    </div>
    <div class="opt-metrics">
      <div class="metric"><div class="v">${d.runout_deg.toFixed(1)}°</div><div class="k">биение мишени</div></div>
      <div class="metric"><div class="v ${sweepOK ? '' : 'bad'}">${Math.round(d.sweep_deg)}°</div><div class="k">поворот колеса</div></div>
      <div class="metric"><div class="v">${d.axis_residual_mm.toFixed(1)}</div><div class="k">разброс оси, мм</div></div>
      <div class="metric"><div class="v">${d.used}</div><div class="k">кадров учтено</div></div>
    </div>
    ${frameStrip(d.frames)}
    ${warnBlock(d.warnings)}`;
}

function frameStrip(frames) {
  if (!frames || !frames.length) return '';
  const dots = frames.map(f =>
    `<div class="frame-dot ${f.ok ? 'ok' : 'bad'}" title="${f.error || ('СКО ' + (f.rms_px || 0).toFixed(2) + ' пикс')}">${f.index + 1}</div>`
  ).join('');
  return `<div class="frame-strip">${dots}</div>`;
}

function warnBlock(warnings) {
  if (!warnings || !warnings.length) return '';
  return `<ul class="warnlist">${warnings.map(w => `<li>${w}</li>`).join('')}</ul>`;
}

/* ── Полный оптический замер ──────────────────────────────────────────── */

// Per-wheel file inputs, each with the reference board that wheel was shot
// against. Front wheels default to the front board, rear to the rear one —
// which is the only arrangement in which a board is actually visible.
function buildFullWheelForm() {
  const box = $('#fullWheels');
  box.replaceChildren();
  for (const w of WHEELS) {
    const card = document.createElement('div');
    card.className = 'wheelfile';
    card.innerHTML = `
      <h4>${w.name}</h4>
      <label>Напольная мишень в кадре
        <select data-ref="${w.key}">
          <option value="0"${w.front ? ' selected' : ''}>Передняя</option>
          <option value="1"${w.front ? '' : ' selected'}>Задняя</option>
        </select>
      </label>
      <label class="filedrop">
        <input type="file" accept="image/png,image/jpeg" multiple hidden data-full="${w.key}">
        <span>Выбрать снимки</span>
      </label>
      <div class="hint" data-count="${w.key}"></div>`;
    box.append(card);
  }
  $$('[data-full]').forEach(inp => {
    inp.onchange = () => {
      const k = inp.dataset.full;
      const n = inp.files.length;
      inp.closest('.filedrop').classList.toggle('ready', n > 0);
      $(`[data-count="${k}"]`).textContent = n ? `Снимков: ${n}` : '';
      checkFullReady();
    };
  });
}

const fullLinkInput = fileZone('fullLink', 'fullLinkCount');

function usingOneRef() { return $('#oneRef').checked; }

$('#oneRef').onchange = () => {
  $('#linkRow').classList.toggle('hidden', usingOneRef());
  checkFullReady();
};
$('#fullCamera').onchange = checkFullReady;

function checkFullReady() {
  const haveCam = $('#fullCamera').files.length > 0;
  const haveWheels = WHEELS.every(w => {
    const inp = document.querySelector(`[data-full="${w.key}"]`);
    return inp && inp.files.length >= 3;
  });
  const haveLink = usingOneRef() || fullLinkInput.files.length > 0;
  $('#btn-full-run').disabled = !(haveCam && haveWheels && haveLink);
}

$('#btn-full-run').onclick = async () => {
  const btn = $('#btn-full-run');
  const out = $('#fullResult');
  btn.disabled = true;
  out.innerHTML = '';
  $('#fullProgress').innerHTML =
    '<span class="spin"></span> Распознаю мишени на всех кадрах. Это самая долгая часть — до нескольких минут.';

  const fd = new FormData();
  fd.append('camera', $('#fullCamera').files[0]);
  fd.append('rim_diameter_in', $('#fullRim').value);
  if (state.spec) fd.append('spec_id', state.spec.id);

  fd.append('wheel_cols', $('#wCols').value);
  fd.append('wheel_rows', $('#wRows').value);
  fd.append('wheel_square_mm', $('#wSq').value);

  fd.append('ref0_cols', $('#r0Cols').value);
  fd.append('ref0_rows', $('#r0Rows').value);
  fd.append('ref0_square_mm', $('#r0Sq').value);
  if (!usingOneRef()) {
    fd.append('ref1_cols', $('#r1Cols').value);
    fd.append('ref1_rows', $('#r1Rows').value);
    fd.append('ref1_square_mm', $('#r1Sq').value);
    for (const f of fullLinkInput.files) fd.append('images_link', f);
  }

  for (const w of WHEELS) {
    const inp = document.querySelector(`[data-full="${w.key}"]`);
    for (const f of inp.files) fd.append('images_' + w.key, f);
    const ref = usingOneRef() ? '0' : document.querySelector(`[data-ref="${w.key}"]`).value;
    fd.append('ref_' + w.key, ref);
  }

  try {
    const r = await fetch('/api/optical/align', { method: 'POST', body: fd });
    const d = await r.json();
    $('#fullProgress').textContent = '';
    if (!r.ok) {
      out.innerHTML = `<div class="error">${d.error || 'Не удалось рассчитать'}</div>` + frameStrip(d.frames);
      return;
    }
    // The response is an ordinary alignment result, so it goes through exactly
    // the same rendering as a string-line measurement.
    out.innerHTML = opticalSummary(d.optical);
    render(d);
    $('#optical').classList.remove('open');
    goto('result');
  } catch (e) {
    $('#fullProgress').textContent = '';
    out.innerHTML = `<div class="error">${e}</div>`;
  } finally {
    btn.disabled = false;
  }
};

function opticalSummary(optical) {
  if (!optical || !optical.length) return '';
  const rows = optical.map(o => `
    <tr>
      <td>${o.position_ru}</td>
      <td class="val">${o.used}/${o.total}</td>
      <td class="val">${Math.round(o.sweep_deg)}°</td>
      <td class="val">${o.runout_deg.toFixed(1)}°</td>
      <td class="val">${o.axis_residual_mm.toFixed(1)}</td>
    </tr>`).join('');
  return `<table class="params" style="margin-top:1rem">
      <thead><tr><th>Колесо</th><th>Кадров</th><th>Поворот</th><th>Биение мишени</th><th>Разброс оси, мм</th></tr></thead>
      <tbody>${rows}</tbody></table>`;
}

buildWheelForm();
buildFullWheelForm();
checkFullReady();
runSearch();
