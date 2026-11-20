// Окно поверх программы: подключение телефона и другие короткие дела, ради
// которых не стоит уходить с экрана регулировки. Esc и крестик закрывают его,
// Tab не уходит из окна, фокус возвращается туда, откуда окно открыли.

import { h, $, $$ } from './state.js';
import { esc } from './fmt.js';
import { t } from './i18n.js';

let current = null;

export function openModal(title, fill, { wide = false, onClose } = {}) {
  closeModal();
  const opener = document.activeElement;
  const back = h(`<div class="modal-back" data-modal>
    <div class="modal${wide ? ' wide' : ''}" role="dialog" aria-modal="true" aria-labelledby="mTitle">
      <button class="modal-x" data-x aria-label="${esc(t('Закрыть'))}" title="${esc(t('Закрыть (Esc)'))}">✕</button>
      <h1 id="mTitle" tabindex="-1">${esc(title)}</h1>
      <div class="modal-body"></div>
    </div></div>`);
  document.body.append(back);
  current = { back, opener, onClose };
  const close = () => closeModal();
  $('[data-x]', back).onclick = close;
  back.addEventListener('mousedown', e => { if (e.target === back) close(); });
  back.addEventListener('keydown', e => {
    if (e.key === 'Escape') { e.preventDefault(); close(); return; }
    if (e.key !== 'Tab') return;
    const f = $$('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])', back).filter(x => !x.disabled && x.offsetParent !== null);
    if (!f.length) return;
    const first = f[0], last = f[f.length - 1];
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  });
  fill($('.modal-body', back), close);
  $('#mTitle', back).focus({ preventScroll: true });
  return close;
}

export function closeModal() {
  if (!current) return;
  const { back, opener, onClose } = current;
  current = null;
  back.remove();
  if (onClose) onClose();
  if (opener && document.body.contains(opener)) opener.focus({ preventScroll: true });
}

export const modalOpen = () => !!current;
