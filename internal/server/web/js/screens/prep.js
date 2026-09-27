// Шаг 2: подготовка — то, без чего регулировать бессмысленно.

import { state, on, save, h, $ } from '../state.js';
import { esc } from '../fmt.js';
import { t } from '../i18n.js';
import { go } from '../app.js';

let root;

export function mount(el) {
  root = el;
  on('session', () => { if (state.screen === 'prep') render(); });
}

export function show() { render(); }

// Группы: [ключ, название, пункты]. Ключ не зависит от языка — отметки
// сохраняются и после переключения.
function groups() {
  const ss = state.session || {};
  const fs = ss.front_suspension || { id: '', name: '', pre_checks: [] };
  const rs = ss.rear_suspension || { id: '', name: '', pre_checks: [] };
  const cond = ss.conditions ? [['cond', t('Условия по данным модели: {c}', { c: ss.conditions })]] : [];
  return [
    ['safety', t('Безопасность'), [
      ['s1', t('Машина стоит на ровной площадке, на стояночном тормозе, под колёсами упоры')],
      ['s2', t('Под машину — только на надёжных подставках. Никогда не под домкратом')],
    ]],
    ['site', t('Площадка'), [
      ['p1', t('Все четыре колеса в одной плоскости: по гидроуровню разница между пятнами контакта меньше 2 мм. Ямка 5 мм под одним колесом — это уже 0,2° ошибки развала')],
      ['p2', t('Для замера кастера передние колёса стоят на поворотных кругах (два листа жести со смазкой между ними — работает)')],
    ]],
    ['car', t('Автомобиль'), [
      ...cond,
      ['c1', t('Давление в шинах по норме и одинаковое слева и справа (разница 0,3 бар — до 0,1° разницы развала)')],
      ['c2', t('Загрузка как требует спецификация: обычно полный бак, запаска и инструмент на местах, без пассажиров')],
      ['c3', t('Подвеска осажена: покачать кузов и прокатить машину 3–5 м вперёд. Не толкать вбок и не катить назад')],
      ['c4', t('Руль строго прямо и зафиксирован (упор в сиденье или ремень) — по метке на валу, а не по спицам')],
    ]],
    ['front:' + fs.id, t('Передняя подвеска — {name}', { name: fs.name }), (fs.pre_checks || []).map((c, i) => ['f' + i, c])],
    ['rear:' + rs.id, t('Задняя подвеска — {name}', { name: rs.name }), (rs.pre_checks || []).map((c, i) => ['r' + i, c])],
  ].filter(([, , items]) => items.length);
}

function keyOf(group, item) {
  const v = state.session && state.session.vehicle ? state.session.vehicle.id : '-';
  return v + '|' + group + '|' + item;
}

function render() {
  const gs = groups();
  const total = gs.reduce((n, [, , items]) => n + items.length, 0);
  const done = gs.reduce((n, [g, , items]) => n + items.filter(([k]) => state.checks[keyOf(g, k)]).length, 0);
  const ss = state.session || {};
  const fs = ss.front_suspension;

  root.innerHTML = `
    <div style="max-width:980px">
      <h1>${t('Подготовка')}</h1>
      <p class="lede">${t('Изношенную подвеску регулировать бесполезно: в статике выставите одно, в движении будет другое. Пройдите список — он составлен под тип подвески вашей машины.')}</p>
      <div class="progress"><div style="width:${total ? (done / total * 100).toFixed(0) : 0}%"></div></div>
      <p class="muted" style="font-size:13px">${t('Выполнено {done} из {total}', { done, total })}</p>
      ${fs && fs.id && fs.id.includes('kingpin') ? `<div class="warn" style="margin:12px 0">
        <b>${t('Шкворневая подвеска.')}</b> ${t('Её чаще всего и отказываются регулировать — потому что без исправных шкворней регулировка не держится. Проверьте люфт шкворней и втулок, прошприцуйте шарниры. Если люфт есть — сначала ремонт.')}</div>` : ''}
      <div id="pgroups"></div>
      <div class="actions">
        <button class="btn primary" id="pNext">${t('Далее: замер →')}</button>
        <button class="btn" id="pAll">${t('Отметить всё')}</button>
        <button class="btn" id="pNone">${t('Снять отметки')}</button>
      </div>
    </div>`;

  const box = $('#pgroups', root);
  for (const [g, title, items] of gs) {
    const sec = h(`<div class="panel" style="margin-top:12px"><h3 style="margin-top:0">${esc(title)}</h3><div class="checklist"></div></div>`);
    const list = sec.querySelector('.checklist');
    for (const [id, text] of items) {
      const k = keyOf(g, id);
      const lab = h(`<label class="chk"><input type="checkbox"${state.checks[k] ? ' checked' : ''}><span>${esc(text)}</span></label>`);
      lab.querySelector('input').onchange = e => {
        if (e.target.checked) state.checks[k] = 1; else delete state.checks[k];
        save('checks', state.checks);
        render();
      };
      list.append(lab);
    }
    box.append(sec);
  }
  $('#pNext', root).onclick = () => go('measure');
  const setAll = on => {
    for (const [g, , items] of gs) for (const [id] of items) { if (on) state.checks[keyOf(g, id)] = 1; else delete state.checks[keyOf(g, id)]; }
    save('checks', state.checks);
    render();
  };
  $('#pAll', root).onclick = () => setAll(true);
  $('#pNone', root).onclick = () => setAll(false);
}
