// Поддержать проект: спонсоры и другие способы помочь.

import { t } from '../i18n.js';

const REPO = 'https://github.com/AristarhUcolov/wheel-alignment';

const DONATE = [
  {
    name: 'Ko-fi',
    url: 'https://ko-fi.com/AristarhUcolov',
    note: () => t('Разовая или ежемесячная поддержка картой из любой страны.'),
    color: '#29abe0',
  },
  {
    name: 'Buy Me a Coffee',
    url: 'https://buymeacoffee.com/Aristarh.Ucolov',
    note: () => t('«Угостить кофе» — быстрый платёж картой, без регистрации.'),
    color: '#ffdd00',
  },
  {
    name: 'DonationAlerts',
    url: 'https://www.donationalerts.com/c/aristarh_ucolov',
    note: () => t('Удобно платить из России и стран СНГ.'),
    color: '#f57507',
  },
];

export function mount(el) {
  el.innerHTML = `
    <div class="support">
      <h1>${t('Поддержать проект')}</h1>
      <p class="lede">${t('Программа бесплатная и останется такой: без рекламы, без подписки, без сбора данных. Её делают, чтобы любой человек мог сам выставить углы колёс — даже когда мастерская отказывает.')}</p>
      <p class="lede">${t('Поддержка оплачивает время на разработку, проверку на настоящих машинах против профессионального стенда, датчики для экспериментов и сбор проверенных допусков. Каждая сумма помогает.')}</p>

      <div class="donate-grid">
        ${DONATE.map(d => `
          <a class="donate" href="${d.url}" style="--brand:${d.color}">
            <b>${d.name}</b>
            <span>${d.note()}</span>
            <em>${d.url.replace('https://', '')}</em>
          </a>`).join('')}
      </div>

      <h2>${t('Помочь без денег')}</h2>
      <ul class="plain">
        <li><b>${t('Внесите допуски из руководства по ремонту.')}</b> ${t('Одна проверенная запись помогает всем владельцам модели — это самое ценное для проекта.')}</li>
        <li><b>${t('Сравните с профессиональным стендом.')}</b> ${t('Замерили программой и на СТО — напишите, насколько сошлось.')}</li>
        <li><b>${t('Расскажите тем, кому отказали.')}</b> ${t('Владельцам «Волг», УАЗов, «классики», грузовиков и автобусов.')}</li>
        <li><b>${t('Поставьте звезду на GitHub и сообщайте об ошибках.')}</b> ${t('Так проект находят другие.')}</li>
      </ul>
      <div class="actions">
        <a class="btn primary" href="${REPO}">${t('Проект на GitHub')}</a>
        <a class="btn" href="${REPO}/issues">${t('Сообщить об ошибке')}</a>
      </div>
      <p class="muted" style="margin-top:18px;font-size:12.5px">${t('Ссылки откроются в вашем браузере.')}</p>
    </div>`;
}
