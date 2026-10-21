// Шаг 2: подготовка — то, без чего регулировать бессмысленно.
//
// У каждого пункта коротко «что сделать» и мелко «почему», у каждой группы —
// свой счётчик. Список собран под тип подвески и под выбранный способ замера.

import { state, on, save, h, $, $$ } from '../state.js';
import { esc } from '../fmt.js';
import { t } from '../i18n.js';
import { go } from '../app.js';
import { setMethod } from './measure.js';

let root;

export function mount(el) {
  root = el;
  on('session', () => { if (state.screen === 'prep') render(); });
}

export function show() { render(); }

function method() {
  try { return JSON.parse(localStorage.getItem('wa.method')) || 'manual'; } catch { return 'manual'; }
}

const METHOD_NAME = () => ({ manual: t('Струна и угломер'), phone: t('Телефон на колесе'), optical: t('Камера и мишени'), sensor: t('Свой датчик') });

// Что понадобится — под способ замера, выбранный на шаге «Замер».
function tools() {
  switch (method()) {
    case 'manual': return [
      ['m1', t('Леска или струна, четыре подставки под неё, рулетка'), t('Струна натягивается вдоль борта на высоте центра колёс.')],
      ['m2', t('Угломер с точностью 0,1° и ровная планка на обод'), t('Планка ложится на закраины обода, не на шину.')],
    ];
    case 'phone': return [
      ['f1', t('Телефон и ровная планка на закраины обода'), t('Телефон прижимается к планке экраном наружу.')],
      ['f2', t('Компьютер и телефон в одной сети Wi-Fi'), t('Подойдёт и точка доступа на самом телефоне.')],
    ];
    case 'optical': return [
      ['o1', t('Мишени напечатаны в масштабе 100 % и наклеены на ровный жёсткий лист'), t('Изогнутая бумага даёт ошибку больше, чем кривое крепление.')],
      ['o2', t('Штатив или опора для телефона и напольные мишени'), t('Мишень на колесе и мишень на полу должны быть в кадре.')],
    ];
    case 'sensor': return [
      ['d1', t('Датчик подключён и виден в строке состояния вверху'), ''],
    ];
  }
  return [];
}

// Группы: [ключ, название, пункты]; пункт — [ключ, что сделать, почему].
// Ключи не зависят от языка — отметки сохраняются и после переключения.
// Инструмент — сразу после безопасности: его собирают раньше всего.
function groups() {
  const ss = state.session || {};
  const fs = ss.front_suspension || { id: '', name: '', pre_checks: [] };
  const rs = ss.rear_suspension || { id: '', name: '', pre_checks: [] };
  const cond = ss.conditions ? [['cond', t('Условия по данным модели'), ss.conditions]] : [];
  return [
    ['safety', t('Безопасность'), [
      ['s1', t('Ровная площадка, стояночный тормоз, упоры под колёсами'), ''],
      ['s2', t('Под машину — только на надёжных подставках'), t('Никогда не под домкратом.')],
    ]],
    ['tools:' + method(), t('Инструмент'), tools()],
    ['site', t('Площадка'), [
      ['p1', t('Все четыре колеса в одной плоскости'), t('Разница между пятнами контакта по гидроуровню — меньше 2 мм. Ямка 5 мм под одним колесом — это уже 0,2° ошибки развала.')],
      ['p2', t('Поворотные круги под передними колёсами — для кастера'), t('Два листа жести со смазкой между ними тоже работают.')],
    ]],
    ['car', t('Автомобиль'), [
      ...cond,
      ['c1', t('Давление в шинах по норме и одинаковое слева и справа'), t('Разница 0,3 бар — до 0,1° разницы развала.')],
      ['c2', t('Загрузка — как требует спецификация'), t('Обычно: полный бак, запаска и инструмент на местах, без пассажиров.')],
      ['c3', t('Подвеска осажена'), t('Покачайте кузов и прокатите машину 3–5 м вперёд. Не толкайте вбок и не катите назад.')],
      ['c4', t('Руль строго прямо и зафиксирован'), t('Упор в сиденье или ремень; «прямо» — по метке на валу, а не по спицам.')],
    ]],
    ['front:' + fs.id, t('Передняя подвеска — {name}', { name: fs.name }), (fs.pre_checks || []).map((c, i) => ['f' + i, ...split(c)])],
    ['rear:' + rs.id, t('Задняя подвеска — {name}', { name: rs.name }), (rs.pre_checks || []).map((c, i) => ['r' + i, ...split(c)])],
  ].filter(([, , items]) => items.length);
}

// Способ замера выбирают на шаге 3, а инструмент нужен уже здесь — выбор
// продублирован в группе «Инструмент».
function methodPick() {
  return `<label class="pmethod"><span>${t('Способ замера')}</span><select id="pMethod">${Object.entries(METHOD_NAME()).map(([id, name]) =>
    `<option value="${id}"${id === method() ? ' selected' : ''}>${esc(name)}</option>`).join('')}</select></label>`;
}

// Длинную проверку из описания подвески — на «что сделать» и «почему»:
// по первому двоеточию или тире, если оно близко к началу.
function split(text) {
  for (const sep of [': ', ' — ']) {
    const i = text.indexOf(sep);
    if (i > 3 && i < 110) {
      const rest = text.slice(i + sep.length);
      return [text.slice(0, i), rest.charAt(0).toUpperCase() + rest.slice(1)];
    }
  }
  return [text, ''];
}

function keyOf(group, item) {
  const v = state.session && state.session.vehicle ? state.session.vehicle.id : '-';
  return v + '|' + group + '|' + item;
}

function counts(gs) {
  let total = 0, done = 0;
  const per = {};
  for (const [g, , items] of gs) {
    const d = items.filter(([k]) => state.checks[keyOf(g, k)]).length;
    per[g] = [d, items.length];
    total += items.length;
    done += d;
  }
  return { total, done, per };
}

// Только счётчики, полоска и плашка «готово» — без перерисовки списка, иначе
// фокус с клавиатуры уходил бы в начало страницы после каждой отметки.
function progress(gs) {
  const { total, done, per } = counts(gs);
  const bar = $('.progress', root);
  bar.setAttribute('aria-valuenow', done);
  bar.firstElementChild.style.width = `${total ? (done / total * 100).toFixed(0) : 0}%`;
  $('#pCount', root).textContent = t('Выполнено {done} из {total}', { done, total });
  $$('[data-group]', root).forEach(sec => {
    const [d, n] = per[sec.dataset.group] || [0, 0];
    const full = d === n && n > 0;
    sec.classList.toggle('done', full);
    sec.querySelector('.gcount').textContent = full ? '✓' : `${d}/${n}`;
  });
  $('#pReady', root).classList.toggle('hidden', !(total && done === total));
}

function render() {
  const gs = groups();
  const { total, done } = counts(gs);
  const ss = state.session || {};
  const fs = ss.front_suspension;

  root.innerHTML = `
    <div class="prep">
      <h1>${t('Подготовка')}</h1>
      <p class="lede">${t('Изношенную подвеску регулировать бесполезно: в статике выставите одно, в движении будет другое. Пройдите список — он составлен под тип подвески вашей машины и под способ замера.')}</p>
      <div class="prep-top">
        <div class="grow">
          <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="${total}" aria-valuenow="${done}" aria-label="${esc(t('Подготовка'))}"><div style="width:${total ? (done / total * 100).toFixed(0) : 0}%"></div></div>
          <p class="muted" style="font-size:13px;margin:0" id="pCount">${t('Выполнено {done} из {total}', { done, total })}</p>
        </div>
        <div class="actions" style="margin:0">
          <button class="btn" id="pAll">${t('Отметить всё')}</button>
          <button class="btn" id="pNone">${t('Снять отметки')}</button>
        </div>
      </div>
      ${fs && fs.id && fs.id.includes('kingpin') ? `<div class="warn" style="margin:12px 0">
        <b>${t('Шкворневая подвеска.')}</b> ${t('Её чаще всего и отказываются регулировать — потому что без исправных шкворней регулировка не держится. Проверьте люфт шкворней и втулок, прошприцуйте шарниры. Если люфт есть — сначала ремонт.')}</div>` : ''}
      <div class="ok-box hidden" id="pReady" style="margin:12px 0;font-weight:600">✓ ${t('Всё готово — можно мерить.')}</div>
      <div class="prep-grid" id="pgroups"></div>
      <div class="actions">
        <button class="btn primary" id="pNext">${t('Далее: замер →')}</button>
      </div>
    </div>`;

  const box = $('#pgroups', root);
  for (const [g, title, items] of gs) {
    const sec = h(`<section class="panel pgroup" data-group="${esc(g)}"><h3><span>${esc(title)}</span><span class="gcount"></span></h3>${g.startsWith('tools:') ? methodPick() : ''}<div class="checklist"></div></section>`);
    const list = sec.querySelector('.checklist');
    for (const [id, text, why] of items) {
      const k = keyOf(g, id);
      const lab = h(`<label class="chk"><input type="checkbox"${state.checks[k] ? ' checked' : ''}><span><b>${esc(text)}</b>${why ? `<small>${esc(why)}</small>` : ''}</span></label>`);
      lab.querySelector('input').onchange = e => {
        if (e.target.checked) state.checks[k] = 1; else delete state.checks[k];
        save('checks', state.checks);
        progress(gs);
      };
      list.append(lab);
    }
    box.append(sec);
  }
  progress(gs);
  const pm = $('#pMethod', root);
  if (pm) pm.onchange = () => { setMethod(pm.value); render(); $('#pMethod', root).focus(); };
  $('#pNext', root).onclick = () => go('measure');
  const setAll = on => {
    for (const [g, , items] of gs) for (const [id] of items) { if (on) state.checks[keyOf(g, id)] = 1; else delete state.checks[keyOf(g, id)]; }
    save('checks', state.checks);
    render();
    $(on ? '#pAll' : '#pNone', root).focus();
  };
  $('#pAll', root).onclick = () => setAll(true);
  $('#pNone', root).onclick = () => setAll(false);
}
