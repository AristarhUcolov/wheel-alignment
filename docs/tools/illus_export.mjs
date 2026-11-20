// Export the program's diagrams to docs/img/illus as standalone SVG files, in
// the colours of the light theme, in Russian and English — so the guide on
// GitHub shows the same pictures as the program.
//
//     node docs/tools/illus_export.mjs
//
// The diagrams are drawn by internal/server/web/js/illus.js; colours there are
// CSS variables, resolved here from style.css. Also writes wear.json, the tyre
// wear rows for docs/tools/guide_md.py. Needs Node 22 or newer (it loads the
// interface's ES modules directly).

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

// i18n.setLang touches the page; give it a stand-in.
globalThis.document = { documentElement: {} };

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const web = path.join(root, 'internal', 'server', 'web');
const out = path.join(root, 'docs', 'img', 'illus');
fs.mkdirSync(out, { recursive: true });

const i18n = await import(pathToFileURL(path.join(web, 'js', 'i18n.js')));
const illus = await import(pathToFileURL(path.join(web, 'js', 'illus.js')));

// The light theme: the base variables, overridden by :root[data-theme=light].
const css = fs.readFileSync(path.join(web, 'style.css'), 'utf8');
const vars = block => Object.fromEntries([...block.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)].map(m => [m[1], m[2].trim()]));
const colors = { ...vars(css.match(/:root \{([^}]*)\}/)[1]), ...vars(css.match(/:root\[data-theme=light\] \{([^}]*)\}/)[1]) };
const resolve = s => s.replace(/var\((--[\w-]+)\)/g, (m, n) => colors[n] || '#888');

// A standalone file needs its namespace, a background and a font.
function standalone(svg) {
  const [, w, h] = svg.match(/viewBox="0 0 (\d+) (\d+)"/);
  return resolve(svg)
    .replace('<svg ', `<svg xmlns="http://www.w3.org/2000/svg" width="${w * 2}" height="${h * 2}" font-family="Segoe UI, Arial, sans-serif" `)
    .replace(/(<svg[^>]*>)/, `$1<rect width="${w}" height="${h}" rx="10" fill="${colors['--bg2']}"/>`) + '\n';
}

const keys = [...illus.illusKeys().filter(k => k !== 'steer'), 'steer:left', 'steer:right'];
const wear = {};
for (const lang of ['ru', 'en']) {
  i18n.setLang(lang);
  for (const k of keys) {
    const [key, arg] = k.split(':');
    const name = arg ? `${key}-${arg}` : key;
    fs.writeFileSync(path.join(out, `${name}-${lang}.svg`), standalone(illus.svgOf(key, arg)));
  }
  wear[lang] = illus.tyreWearRows().map(([kind, title, why]) => ({ kind, title, why }));
  for (const { kind } of wear[lang]) {
    fs.writeFileSync(path.join(out, `wear-${kind}-${lang}.svg`), standalone(illus.wearSvg(kind)));
  }
  wear[lang + '_captions'] = Object.fromEntries(keys.map(k => [k, illus.captionOf(k.split(':')[0])]));
}
fs.writeFileSync(path.join(out, 'wear.json'), JSON.stringify(wear, null, 1) + '\n');
console.log(`${keys.length * 2} diagrams and ${wear.ru.length} wear pictures in ${path.relative(root, out)}`);
