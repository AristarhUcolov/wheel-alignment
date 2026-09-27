// Шаг 5: протокол «до / после».

import { state, on, api, save, toast, $, $$ } from '../state.js';
import { esc, fmtDM, fmtMM, statusText } from '../fmt.js';
import { t, lang } from '../i18n.js';
import { go, snapshot } from '../app.js';

let root, data = null;
const meta = (() => { try { return JSON.parse(localStorage.getItem('wa.reportMeta')) || {}; } catch { return {}; } })();

export function mount(el) {
  root = el;
  on('session', () => { if (state.screen === 'report') load(); });
}

export function show() { load(); }

async function load() {
  try { data = await api('/api/report'); } catch (e) { toast(e.message, true); return; }
  render();
}

const AXLES = () => ({ front: t('Передняя ось'), rear: t('Задняя ось'), vehicle: t('Автомобиль в целом') });

function cell(p, isToe) {
  if (!p) return '<td class="v dim">—</td>';
  const mm = isToe && p.measured_mm !== undefined && p.measured_mm !== null ? `<br><span class="dim" style="font-size:12px">${fmtMM(p.measured_mm)}</span>` : '';
  return `<td class="v ${p.status === 'no_spec' ? '' : p.status}">${fmtDM(p.measured)}${mm}</td>`;
}

function table(before, after) {
  const ref = after || before;
  const bmap = Object.fromEntries((before ? before.report.params : []).map(p => [p.key, p]));
  const amap = Object.fromEntries((after ? after.report.params : []).map(p => [p.key, p]));
  let rows = '';
  for (const [axle, name] of Object.entries(AXLES())) {
    const ps = ref.report.params.filter(p => p.axle === axle);
    if (!ps.length) continue;
    rows += `<tr class="axle"><td colspan="5">${name}</td></tr>`;
    for (const p of ps) {
      const isToe = p.key.includes('toe');
      const spec = p.spec ? `${fmtDM(p.spec.min)} … ${fmtDM(p.spec.max)}` : `<span class="dim">${t('нет данных')}</span>`;
      const final = amap[p.key] || bmap[p.key];
      rows += `<tr><td>${esc(p.label)}</td>
        ${before ? cell(bmap[p.key], isToe) : ''}${after ? cell(amap[p.key], isToe) : ''}
        <td>${spec}</td><td>${esc(statusText(final.status))}${final.advice ? `<br><span class="muted" style="font-size:12.5px">${esc(final.advice)}</span>` : ''}</td></tr>`;
    }
  }
  return `<table class="params"><thead><tr><th>${t('Параметр')}</th>${before ? `<th>${t('До')}</th>` : ''}${after ? `<th>${t('После')}</th>` : ''}<th>${t('Допуск')}</th><th>${t('Итог')}</th></tr></thead><tbody>${rows}</tbody></table>`;
}

function render() {
  if (!root || !data) return;
  const ss = data.session;
  const v = ss.vehicle;
  const lim = ss.limits;
  const before = data.before, after = data.after;
  const last = after || before;
  const when = last ? new Date(last.time) : new Date();

  let verdict = '';
  if (last) {
    const st = last.report.overall_status;
    const txt = {
      good: t('Все углы в допуске'), marginal: t('Углы в допуске, но у границы'),
      bad: t('Вне допуска: {n}', { n: last.report.out_of_spec }), no_spec: t('Допуски не заданы'),
    }[st];
    verdict = `<div class="${st === 'good' ? 'ok-box' : st === 'bad' ? 'danger-box' : 'note'}" style="font-size:16px;font-weight:600">${txt} — ${after ? t('после регулировки') : t('до регулировки')}</div>`;
  }

  const warnings = last ? [...new Set([...(last.result.warnings || []),
    ...Object.values(last.result.wheels).flatMap(w => (w.quality && w.quality.Warnings) || [])])] : [];
  const steps = before && before.report.steps ? before.report.steps : [];
  const limName = lim ? (lim.source_kind === 'class_guidance' ? t('ориентир по классу: {model}', { model: lim.model }) : lim.title) : '';

  root.innerHTML = `
    <div style="max-width:1080px">
      <div class="row no-print" style="justify-content:space-between;align-items:center">
        <h1>${t('Протокол сход-развала')}</h1>
        <div class="actions" style="margin:0">
          <button class="btn" id="rBefore">${before ? t('Переснять «до»') : t('Снимок «до»')}</button>
          <button class="btn" id="rAfter">${after ? t('Переснять «после»') : t('Снимок «после»')}</button>
          <button class="btn primary" id="rPrint" ${last ? '' : 'disabled'}>${t('Печать / PDF')}</button>
        </div>
      </div>
      <div class="panel">
        <div class="kv">
          <div>${t('Дата')}</div><div>${when.toLocaleString(lang === 'en' ? 'en-GB' : 'ru-RU')}</div>
          <div>${t('Автомобиль')}</div><div>${v ? esc(v.source_kind === 'class_guidance' ? t('не указан') : v.title) : t('не указан')}</div>
          <div>${t('Госномер / VIN')}</div><div><input type="text" data-meta="plate" value="${esc(meta.plate || '')}" style="width:100%;max-width:360px"></div>
          <div>${t('Пробег, км')}</div><div><input type="text" data-meta="odo" value="${esc(meta.odo || '')}" style="width:100%;max-width:200px"></div>
          <div>${t('Выполнил')}</div><div><input type="text" data-meta="who" value="${esc(meta.who || '')}" style="width:100%;max-width:360px"></div>
          <div>${t('Подвеска')}</div><div>${esc(t('перед: {front}; зад: {rear}', { front: ss.front_suspension.name, rear: ss.rear_suspension.name }))}</div>
          <div>${t('Допуски')}</div><div>${lim ? `${esc(limName)} <span class="badge ${esc(lim.source_kind)}">${esc(lim.source_label)}</span>` : t('не заданы')}</div>
          <div>${t('Обод')}</div><div>${t('{rim}″ — к этому диаметру отнесено схождение в мм', { rim: ss.rim_diameter_in })}</div>
        </div>
        ${lim && lim.disclaimer ? `<div class="warn">${esc(lim.disclaimer)}</div>` : ''}
        ${lim && lim.source_reference && lim.source_kind !== 'class_guidance' ? `<p class="muted" style="margin-top:8px;font-size:12.5px"><b>${t('Источник:')}</b> ${esc(lim.source_reference)}</p>` : ''}
        ${ss.conditions ? `<p class="muted" style="margin-top:10px"><b>${t('Условия замера:')}</b> ${esc(ss.conditions)}</p>` : ''}
      </div>
      ${!last ? `<div class="panel" style="margin-top:14px">
        <h2 style="margin-top:0">${t('Снимков пока нет')}</h2>
        <p class="muted">${t('Протокол строится из двух снимков: «до» — перед регулировкой, «после» — когда всё выставлено. Снимок берёт текущие показания всех четырёх колёс (нужны развал и схождение каждого). Клавиша F5 на экране регулировки делает снимок в один нажим.')}</p>
        <div class="actions"><button class="btn primary" id="rGoLive">${t('К экрану регулировки')}</button></div></div>` : `
      <div style="margin-top:14px">${verdict}</div>
      <div class="panel" style="margin-top:14px">${table(before, after)}</div>
      ${warnings.length ? `<div class="panel"><h2 style="margin-top:0">${t('На что обратить внимание')}</h2><ul class="plain">${warnings.map(w => `<li>${esc(w)}</li>`).join('')}</ul></div>` : ''}
      ${steps.length ? `<div class="panel"><h2 style="margin-top:0">${t('Порядок регулировки (по замеру «до»)')}</h2><ol class="steps">
        ${steps.map(s => `<li><b>${esc(s.title)}</b><div>${esc(s.detail)}</div>${s.why ? `<div class="why">${esc(s.why)}</div>` : ''}</li>`).join('')}</ol></div>` : ''}`}
      <p class="muted" style="font-size:12px;margin-top:14px">${t('Протокол сформирован программой «Сход-развал — открытый стенд» (свободная программа, AGPL-3.0). Программа считает то, что измерено; она не проверяет давление в шинах, люфты и ровность площадки.')}</p>
    </div>`;

  $$('[data-meta]', root).forEach(inp => inp.oninput = () => {
    meta[inp.dataset.meta] = inp.value;
    save('reportMeta', meta);
  });
  $('#rBefore', root).onclick = async () => { await snapshot('before'); load(); };
  $('#rAfter', root).onclick = async () => { await snapshot('after'); load(); };
  $('#rPrint', root).onclick = () => window.print();
  const gl = $('#rGoLive', root);
  if (gl) gl.onclick = () => go('live');
}
