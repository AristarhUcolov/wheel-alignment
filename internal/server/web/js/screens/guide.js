// Инструкция: как пройти сход-развал самостоятельно, от установки до отчёта.
// Текст — в guide/ru.js и guide/en.js: длинные разделы удобнее переводить
// целиком, а не по фразам.

import { api, $, $$ } from '../state.js';
import { esc } from '../fmt.js';
import { t, lang } from '../i18n.js';
import RU from '../guide/ru.js';
import EN from '../guide/en.js';

let root;

export function mount(el) {
  root = el;
  const g = lang === 'en' ? EN : RU;
  root.innerHTML = `<div class="guide">
    <h1>${g.title}</h1>
    <p class="lede">${g.lede}</p>
    <div class="toc">${g.sections.map(([id, name]) => `<button class="chip" data-to="${id}">${name}</button>`).join('')}</div>
    ${g.sections.map(([id, name, html]) => `<h2 id="g-${id}">${name}</h2>${html}`).join('\n')}
  </div>`;
  $$('[data-to]', root).forEach(b => b.onclick = () => {
    const target = $('#g-' + b.dataset.to, root);
    if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
  });
  loadSusp();
}

async function loadSusp() {
  const box = $('#gSusp', root);
  let list;
  try { list = await api('/api/suspensions'); } catch (e) { box.textContent = e.message; return; }
  const AXLE = { front: t('передняя'), rear: t('задняя'), both: t('передняя и задняя') };
  const adj = (name, a) => a && a.how ? `<div>${name}</div><div><b class="${a.usual ? 'yes' : 'no'}">${a.usual ? t('обычно регулируется.') : t('обычно не регулируется.')}</b> ${esc(a.how)}${a.tip ? `<br><span class="muted">${esc(a.tip)}</span>` : ''}</div>` : '';
  box.innerHTML = list.all.filter(s => s.id).map(s => `
    <div class="susp-card">
      <h3>${esc(s.name)} <span class="badge">${AXLE[s.axle] || ''}</span></h3>
      <p>${esc(s.summary)}</p>
      <p class="muted"><b>${t('Как узнать:')}</b> ${esc(s.identify)}</p>
      ${s.examples && s.examples.length ? `<p class="muted"><b>${t('Например:')}</b> ${s.examples.map(esc).join('; ')}</p>` : ''}
      <div class="adj">${adj(t('Развал'), s.camber)}${adj(t('Кастер'), s.caster)}${adj(t('Схождение'), s.toe)}</div>
      <details><summary style="cursor:pointer;color:var(--blue)">${t('Проверить до регулировки')}</summary>
        <ul class="plain">${s.pre_checks.map(c => `<li>${esc(c)}</li>`).join('')}</ul></details>
      ${s.notes && s.notes.length ? `<ul class="plain muted">${s.notes.map(n => `<li>${esc(n)}</li>`).join('')}</ul>` : ''}
    </div>`).join('');
}
