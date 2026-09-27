// Перевод интерфейса. Русский текст — это и есть ключ: t('Автомобиль').
// Английские версии — в i18n-en.js; тест на Go проверяет, что у каждой строки,
// переданной в t(), есть перевод с теми же подстановками {вида}.

import EN from './i18n-en.js';

export let lang = 'ru';

export function setLang(l) {
  lang = l === 'en' ? 'en' : 'ru';
  document.documentElement.lang = lang;
}

// t('Выполнено {done} из {total}', { done: 3, total: 18 })
export function t(ru, vars) {
  let s = (lang === 'en' && EN[ru]) || ru;
  if (vars) s = s.replace(/\{(\w+)\}/g, (m, k) => (vars[k] !== undefined ? vars[k] : m));
  return s;
}

// Статичный текст страницы: элементы с data-t переводятся целиком, а
// data-t-title — только подсказка.
export function translateStatic(root = document) {
  root.querySelectorAll('[data-t]').forEach(el => {
    if (!el.dataset.tRu) el.dataset.tRu = el.textContent.trim();
    el.textContent = t(el.dataset.tRu);
  });
  root.querySelectorAll('[data-t-title]').forEach(el => {
    if (!el.dataset.tTitleRu) el.dataset.tTitleRu = el.getAttribute('title') || '';
    el.setAttribute('title', t(el.dataset.tTitleRu));
  });
}
