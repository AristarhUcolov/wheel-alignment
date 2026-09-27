// Телефон как беспроводной датчик: включение доступа по Wi-Fi, адрес и
// QR-код для подключения, список подключённых телефонов.

import { api, toast, h, $ } from '../state.js';
import { esc, fmtDM, WHEELS } from '../fmt.js';
import { t } from '../i18n.js';

let host = null, info = null, poll = null;

export async function render(body) {
  host = h(`<div class="panel" style="max-width:1100px"><div id="phBody"><span class="spin"></span> ${t('Загрузка…')}</div></div>`);
  body.append(host);
  await refresh();
  clearInterval(poll);
  poll = setInterval(() => { if (document.body.contains(host)) refresh(); else clearInterval(poll); }, 1500);
}

export function update() { /* обновляется опросом */ }

async function refresh() {
  try { info = await api('/api/phone'); } catch (e) { $('#phBody', host).innerHTML = `<div class="danger-box">${esc(e.message)}</div>`; return; }
  draw();
}

async function setEnabled(on) {
  try {
    info = await api('/api/phone', { method: 'POST', body: { enable: on } });
    draw();
    if (on) toast(t('Доступ по Wi-Fi включён. Если Windows спросит про брандмауэр — разрешите доступ в частных сетях.'));
  } catch (e) { toast(e.message, true); }
}

function wheelName(k) { const w = WHEELS.find(x => x.key === k); return w ? w.name : t('колесо не выбрано'); }

function draw() {
  const box = $('#phBody', host);
  if (!box || !info) return;
  if (!info.enabled) {
    box.innerHTML = `
      <h2 style="margin-top:0">${t('Телефон как датчик развала и кастера')}</h2>
      <p>${t('Телефон прикладывается к ободу через ровную планку и по Wi-Fi передаёт развал в реальном времени — стрелки на экране двигаются, пока вы крутите регулировку. С поворотными кругами он же меряет кастер: гироскоп сам определяет угол поворота колеса, программа ведёт по шагам.')}</p>
      <ul class="plain">
        <li>${t('Подойдёт любой смартфон с браузером (Chrome, Safari). Приложение ставить не нужно.')}</li>
        <li>${t('Телефон и этот компьютер — в одной сети Wi-Fi (подойдёт точка доступа на самом телефоне).')}</li>
        <li>${t('Два телефона — два колеса сразу, четыре — все четыре.')}</li>
        <li>${t('Схождение телефон не меряет: его вводите по струне на вкладке «Струна и угломер» или берите с камеры — программа сведёт всё на одном экране.')}</li>
      </ul>
      <div class="actions"><button class="btn primary big" id="phOn">${t('Разрешить подключение по Wi-Fi')}</button></div>
      <p class="muted" style="font-size:12.5px">${t('Включает защищённый адрес в локальной сети с одноразовым ключом. Выключите, когда закончите.')}</p>`;
    $('#phOn', box).onclick = () => setEnabled(true);
    return;
  }

  const urls = info.urls || [];
  const devs = info.devices || [];
  box.innerHTML = `
    <div class="row" style="gap:22px;align-items:flex-start">
      <div style="background:#fff;padding:12px;border-radius:10px;flex:none;width:236px;height:236px">${info.qr_svg || ''}</div>
      <div class="grow">
        <h2 style="margin-top:0">${t('Отсканируйте код камерой телефона')}</h2>
        <p>${t('Или наберите адрес в браузере телефона:')}</p>
        ${urls.map(u => `<div class="note" style="font:600 17px var(--num);margin:6px 0;user-select:all">${esc(u)}</div>`).join('')}
        ${urls.length ? '' : `<div class="danger-box">${t('Не найдено ни одного сетевого подключения. Подключите компьютер к Wi-Fi.')}</div>`}
        <ol class="plain" style="padding-left:20px;margin-top:10px">
          <li>${t('Браузер телефона предупредит «подключение не защищено» — сертификат сделан этой программой, а не удостоверяющим центром. Нажмите «Дополнительно» → «Перейти на сайт».')}</li>
          <li>${t('На телефоне выберите колесо и пройдите калибровку (один раз, около минуты).')}</li>
          <li>${t('Приложите телефон к планке на ободе — развал появится на экране регулировки.')}</li>
        </ol>
      </div>
    </div>
    <h3>${t('Подключённые телефоны')}</h3>
    ${devs.length ? `<table class="params"><thead><tr><th>${t('Телефон')}</th><th>${t('Колесо')}</th><th>${t('Калибровка')}</th><th>${t('Развал')}</th><th>${t('Что делает')}</th></tr></thead><tbody>
      ${devs.map(d => `<tr>
        <td>${esc(d.name)}<br><span class="dim" style="font-size:12px">${d.online ? t('на связи') : t('нет связи {s} с', { s: Math.round(d.age_s) })}</span></td>
        <td>${esc(wheelName(d.wheel))}</td>
        <td>${d.calibrated ? `<span class="yes">${t('готов')}</span>` : `<span class="no">${t('нужна')}</span>`}</td>
        <td class="v">${d.camber === null || d.camber === undefined ? '—' : fmtDM(d.camber)}</td>
        <td>${esc(d.prompt || '')}</td></tr>`).join('')}
    </tbody></table>` : `<p class="muted">${t('Пока ни одного. Откройте адрес на телефоне.')}</p>`}
    <div class="warn" style="margin-top:14px"><b>${t('Точность.')}</b> ${t('После калибровки телефон даёт развал с точностью около 0,1° — если планка ровная и прижата к закраинам обода, а не к резине. Сверьте один раз с известным углом (например, уровнем на ровной стене), прежде чем доверять регулировку.')}</div>
    <div class="actions"><button class="btn danger" id="phOff">${t('Выключить доступ по Wi-Fi')}</button></div>`;
  $('#phOff', box).onclick = () => setEnabled(false);
}
