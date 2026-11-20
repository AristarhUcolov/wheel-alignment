// Телефон как беспроводной датчик: включение доступа по Wi-Fi, адрес и
// QR-код для подключения, список подключённых телефонов.
//
// Доступ, включённый однажды, программа помнит: после перезапуска он
// включается сам, адрес и код остаются прежними, и телефон с открытой
// страницей подключается снова без сканирования.
//
// Панель может быть открыта сразу в двух местах — на шаге «Замер» и в окне
// «Подключить телефон», — поэтому у каждой свои состояние и опрос.

import { api, toast, h, $ } from '../state.js';
import { esc, fmtDM, WHEELS } from '../fmt.js';
import { t } from '../i18n.js';

// render рисует панель в body. autoEnable — включить доступ сразу: человек уже
// нажал «Подключить телефон», второй раз нажимать «Разрешить» незачем.
export function render(body, { autoEnable = false } = {}) {
  const p = new PhonePanel(body);
  p.start(autoEnable);
  return p;
}

export function update() { /* обновляется опросом */ }

function wheelName(k) { const w = WHEELS.find(x => x.key === k); return w ? w.name : t('колесо не выбрано'); }

class PhonePanel {
  constructor(body) {
    this.host = h(`<div class="panel phone-panel"><div class="ph-body"><span class="spin"></span> ${t('Загрузка…')}</div></div>`);
    this.box = $('.ph-body', this.host);
    this.info = null;
    // Опрос каждые 1,5 с перерисовывает только список телефонов, и то если
    // он изменился: адрес и QR-код остаются на месте — их можно выделить и
    // скопировать, а фокус клавиатуры не сбрасывается.
    this.drawnHead = '';
    this.drawnDevs = '';
    body.append(this.host);
  }

  alive() { return document.body.contains(this.host); }

  async start(autoEnable) {
    await this.refresh();
    if (autoEnable && this.info && !this.info.enabled && this.alive()) await this.setEnabled(true);
    this.poll = setInterval(() => { if (this.alive()) this.refresh(); else clearInterval(this.poll); }, 1500);
  }

  async refresh() {
    try { this.info = await api('/api/phone'); } catch (e) {
      this.box.innerHTML = `<div class="danger-box">${esc(e.message)}</div>`;
      this.drawnHead = '';
      return;
    }
    this.draw();
  }

  async post(body) {
    try { this.info = await api('/api/phone', { method: 'POST', body }); this.draw(); return true; } catch (e) { toast(e.message, true); return false; }
  }

  forget(id, name) {
    if (!confirm(t('Отключить «{name}»? Его показания уйдут с экрана регулировки, а подключиться снова он сможет, только если вы разрешите.', { name }))) return;
    this.post({ forget: id });
  }

  async setEnabled(on) {
    if (await this.post({ enable: on }) && on) toast(t('Доступ по Wi-Fi включён. Если Windows спросит про брандмауэр — разрешите доступ в частных сетях.'));
  }

  async newKey() {
    if (!confirm(t('Сменить ключ в адресе? Все подключённые телефоны отключатся, и каждому нужно будет отсканировать новый код.'))) return;
    this.drawnHead = '';
    if (await this.post({ new_key: true })) toast(t('Ключ сменён — отсканируйте новый код на каждом телефоне.'));
  }

  draw() {
    const box = this.box, info = this.info;
    if (!info) return;
    const head = JSON.stringify([info.enabled, info.urls, !!info.qr_svg]);
    if (head === this.drawnHead && info.enabled && $('.ph-devs', box)) { this.drawDevices(); return; }
    this.drawnHead = head;
    this.drawnDevs = '';
    if (!info.enabled) {
      box.innerHTML = `
        <h2 style="margin-top:0">${t('Телефон как датчик развала и кастера')}</h2>
        <p>${t('Телефон прикладывается к ободу через ровную планку и по Wi-Fi передаёт развал в реальном времени — стрелки на экране двигаются, пока вы крутите регулировку. С поворотными кругами он же меряет кастер: гироскоп сам определяет угол поворота колеса, программа ведёт по шагам.')}</p>
        <div class="caps-list">
          <div class="cap yes"><i aria-hidden="true">✓</i> <b>${t('Развал')}</b> — ${t('вживую: телефон на планке, прижатой к ободу.')}</div>
          <div class="cap yes"><i aria-hidden="true">✓</i> <b>${t('Кастер')}</b> — ${t('на передних колёсах, с поворотными кругами: угол поворота телефон меряет гироскопом.')}</div>
          <div class="cap no"><i aria-hidden="true">✕</i> <b>${t('Схождение')}</b> — ${t('телефон на колесе его не меряет. Мерьте струной («Струна и угломер») или этим же телефоном как камерой с мишенями («Камера и мишени»).')}</div>
        </div>
        <ul class="plain">
          <li>${t('Подойдёт любой смартфон с браузером (Chrome, Safari). Приложение ставить не нужно.')}</li>
          <li>${t('Телефон и этот компьютер — в одной сети Wi-Fi (подойдёт точка доступа на самом телефоне).')}</li>
          <li>${t('Два телефона — два колеса сразу, четыре — все четыре.')}</li>
        </ul>
        <div class="actions"><button class="btn primary big" data-ph="on">${t('Разрешить подключение по Wi-Fi')}</button></div>
        <p class="muted" style="font-size:12.5px">${t('Включает защищённый адрес в локальной сети с ключом. Программа запомнит это: после перезапуска доступ включится сам, и телефоны подключатся снова без нового кода. Выключите, когда телефоны больше не нужны.')}</p>`;
      $('[data-ph="on"]', box).onclick = () => this.setEnabled(true);
      return;
    }

    const urls = info.urls || [];
    box.innerHTML = `
      <div class="row" style="gap:22px;align-items:flex-start">
        <div class="qr">${info.qr_svg || ''}</div>
        <div class="grow">
          <h2 style="margin-top:0">${t('Отсканируйте код камерой телефона')}</h2>
          <p>${t('Или наберите адрес в браузере телефона:')}</p>
          ${urls.map(u => `<div class="note" style="font:600 17px var(--num);margin:6px 0;user-select:all;word-break:break-all">${esc(u)}</div>`).join('')}
          ${urls.length ? '' : `<div class="danger-box">${t('Не найдено ни одного сетевого подключения. Подключите компьютер к Wi-Fi.')}</div>`}
          <ol class="plain" style="padding-left:20px;margin-top:10px">
            <li>${t('Браузер телефона предупредит «подключение не защищено» — сертификат сделан этой программой, а не удостоверяющим центром. Нажмите «Дополнительно» → «Перейти на сайт».')}</li>
            <li>${t('На телефоне выберите колесо и нажмите «Включить датчики».')}</li>
            <li>${t('Вкладка «Развал»: один раз пройдите «Подготовку телефона» — калибровку телефона и крепления, около двух минут.')}</li>
            <li>${t('Приложите телефон к планке на ободе — развал появится на экране регулировки. Кастер — на вкладке «Кастер».')}</li>
          </ol>
          <p class="muted" style="font-size:12.5px">${t('Адрес не меняется между запусками программы: можно добавить страницу на главный экран телефона, и в следующий раз она подключится сама.')}</p>
        </div>
      </div>
      <div class="ph-devs"></div>
      <div class="warn" style="margin-top:14px"><b>${t('Точность.')}</b> ${t('После калибровки телефон даёт развал с точностью около 0,1° — если планка ровная и прижата к закраинам обода, а не к резине. Сверьте один раз с известным углом (например, уровнем на ровной стене), прежде чем доверять регулировку.')}</div>
      <div class="actions">
        <button class="btn danger" data-ph="off">${t('Выключить доступ по Wi-Fi')}</button>
        <button class="btn" data-ph="key" title="${esc(t('Если код видел кто-то посторонний'))}">${t('Сменить ключ')}</button>
      </div>`;
    $('[data-ph="off"]', box).onclick = () => this.setEnabled(false);
    $('[data-ph="key"]', box).onclick = () => this.newKey();
    this.drawDevices();
  }

  drawDevices() {
    const el = $('.ph-devs', this.box);
    if (!el) return;
    const info = this.info;
    const devs = info.devices || [];
    const html = `
      <h3>${t('Подключённые телефоны')}</h3>
      ${devs.length ? `<table class="params"><thead><tr><th>${t('Телефон')}</th><th>${t('Колесо')}</th><th>${t('Калибровка')}</th><th>${t('Развал')}</th><th>${t('Что делает')}</th><th></th></tr></thead><tbody>
        ${devs.map(d => `<tr>
          <td>${esc(d.name)}<br><span class="dim" style="font-size:12px">${d.online ? t('на связи') : t('нет связи {s} с', { s: Math.round(d.age_s) })}</span></td>
          <td>${esc(wheelName(d.wheel))}</td>
          <td>${d.calibrated ? `<span class="yes">✓ ${t('готов')}</span>` : `<span class="no">✕ ${t('нужна')}</span>`}${d.check === 'ok' ? `<br><span class="yes" style="font-size:12px">✓ ${t('точность проверена')}</span>` : d.check === 'fail' ? `<br><span class="no" style="font-size:12px">✕ ${t('проверка не пройдена — откалибруйте заново')}</span>` : ''}</td>
          <td class="v">${d.camber === null || d.camber === undefined ? '—' : fmtDM(d.camber)}</td>
          <td>${esc(d.prompt || '')}</td>
          <td><button class="btn small" data-forget="${esc(d.id)}" data-name="${esc(d.name)}" title="${esc(t('Отключить этот телефон'))}">${t('Отключить')}</button></td></tr>`).join('')}
      </tbody></table>` : `<p class="muted">${t('Пока ни одного. Откройте адрес на телефоне.')}</p>`}
      ${(info.blocked || []).length ? `<h3>${t('Отключённые')}</h3>
        <p class="muted" style="font-size:13px">${t('Эти телефоны не смогут присылать показания, пока вы их не разрешите или не смените ключ.')}</p>
        <table class="params"><tbody>${info.blocked.map(d => `<tr><td>${esc(d.name)}</td>
          <td style="width:1%"><button class="btn small" data-allow="${esc(d.id)}">${t('Разрешить снова')}</button></td></tr>`).join('')}</tbody></table>` : ''}`;
    if (html === this.drawnDevs) return;
    this.drawnDevs = html;
    el.innerHTML = html;
    el.querySelectorAll('[data-forget]').forEach(b => b.onclick = () => this.forget(b.dataset.forget, b.dataset.name));
    el.querySelectorAll('[data-allow]').forEach(b => b.onclick = () => this.post({ allow: b.dataset.allow }));
  }
}
