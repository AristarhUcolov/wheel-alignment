"""Build docs/GUIDE.md from the in-app guide (internal/server/web/js/guide/ru.js
and en.js), so the guide on GitHub says exactly what the program says.

    python docs/tools/guide_md.py

The section "Suspension types" is filled in the program from the suspension
catalogue; here its table is kept from the current GUIDE.md.
"""
import html
import os
import re

ROOT = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
GUIDE = os.path.join(ROOT, 'internal', 'server', 'web', 'js', 'guide')
OUT = os.path.join(ROOT, 'docs', 'GUIDE.md')

HEAD = {
    'ru': dict(anchor='ru', lang_line='**Русский** · [English](#eng)', title='Сход-развал — инструкция',
               after='Та же инструкция есть в программе — клавиша **F1**. Подробности о методе и точности — в [PROCEDURE.md](PROCEDURE.md).',
               toc='**Содержание:**'),
    'en': dict(anchor='eng', lang_line='[Русский](#ru) · **English**', title='Wheel alignment — guide',
               after='The same guide is inside the program — press **F1**. Method and accuracy in depth — [PROCEDURE.md](PROCEDURE.md#eng).',
               toc='**Contents:**'),
}
PREFIX = {'ru': 'ru', 'en': 'en'}

# Pictures at the end of a section — only on GitHub, the program shows itself.
IMAGES = {
    'ru': {'vehicle': '![Выбор автомобиля](img/vehicle.png)',
           'phone': '<img src="img/phone.png" width="220" alt="Страница на телефоне">',
           'live': '![Экран регулировки](img/live-overview.png)',
           'access': '![Настройки](img/settings.png)'},
    'en': {'vehicle': '![Choosing a vehicle](img/vehicle-en.png)',
           'phone': '<img src="img/phone.png" width="220" alt="The phone page">',
           'live': '![The adjustment screen](img/live-overview-en.png)',
           'access': '![Settings](img/settings-en.png)'},
}


def read_js(lang):
    src = open(os.path.join(GUIDE, lang + '.js'), encoding='utf-8').read()
    lede = re.search(r"lede:\s*`(.*?)`", src, re.S).group(1)
    sections = re.findall(r"^\['(\w+)', '((?:[^'\\]|\\.)*)', `(.*?)`\],", src, re.S | re.M)
    return lede, sections


def unjs(s):
    # Template literals: \\ is a backslash.
    return s.replace('\\\\', '\\')


def inline(s):
    s = re.sub(r'\s+', ' ', s).strip()
    s = re.sub(r'<span class="badge[^"]*">(.*?)</span>', r'`\1`', s)
    s = re.sub(r'</?(b|strong)>', '**', s)
    s = re.sub(r'</?i>', '*', s)
    s = re.sub(r'<code>(.*?)</code>', lambda m: '`' + html.unescape(m.group(1)) + '`', s)
    s = re.sub(r'<a href="([^"]+)">(.*?)</a>', r'[\2](\1)', s)
    s = re.sub(r'<br\s*/?>', '<br>', s)
    s = re.sub(r'<span[^>]*>(.*?)</span>', r'\1', s)
    s = s.replace('&lt;', '<').replace('&gt;', '>').replace('&amp;', '&')
    return s


def table(t):
    rows = re.findall(r'<tr>(.*?)</tr>', t, re.S)
    head = re.search(r'<thead>(.*?)</thead>', t, re.S)
    out = []
    if head:
        cells = re.findall(r'<th>(.*?)</th>', head.group(1), re.S)
        out.append('| ' + ' | '.join(inline(c) for c in cells) + ' |')
        out.append('|' + '|'.join('---' for _ in cells) + '|')
        rows = re.findall(r'<tr>(.*?)</tr>', t.split('</thead>', 1)[1], re.S)
    else:
        out.append('|   |   |')
        out.append('|---|---|')
    for r in rows:
        m = re.match(r'\s*<td colspan="2"[^>]*>(.*?)</td>\s*$', r, re.S)
        if m:
            out.append(f'| **{inline(m.group(1))}** |  |')
            continue
        cells = re.findall(r'<td[^>]*>(.*?)</td>', r, re.S)
        out.append('| ' + ' | '.join(inline(c) for c in cells) + ' |')
    return '\n'.join(out)


def lists(tag, body):
    items = re.findall(r'<li>(.*?)</li>', body, re.S)
    mark = (lambda i: f'{i + 1}. ') if tag == 'ol' else (lambda i: '- ')
    return '\n'.join(mark(i) + inline(x) for i, x in enumerate(items))


def block(h, susp_table):
    out = []
    pos = 0
    pat = re.compile(r'<(p|ol|ul|table|div)([^>]*)>(.*?)</\1>', re.S)
    for m in pat.finditer(h):
        tag, attrs, body = m.group(1), m.group(2), m.group(3)
        if tag == 'p':
            out.append(inline(body))
        elif tag in ('ol', 'ul'):
            out.append(lists(tag, body))
        elif tag == 'table':
            out.append(table(m.group(0)))
        elif tag == 'div':
            if 'note' in attrs:
                out.append('> ' + inline(body))
            elif 'gSusp' in attrs:
                out.append(susp_table)
            else:
                out.append(inline(body))
    return '\n\n'.join(x for x in out if x)


def current_susp(lang):
    """The suspension table as it stands in GUIDE.md now."""
    if not os.path.exists(OUT):
        return ''
    md = open(OUT, encoding='utf-8').read()
    p = PREFIX[lang]
    m = re.search(rf'<a id="{p}-susp"></a>\n\n## [^\n]*\n\n[^\n]*\n\n(.*?)\n\n<a id="{p}-trucks"></a>', md, re.S)
    return m.group(1).strip() if m else ''


def build(lang):
    lede, sections = read_js(lang)
    h = HEAD[lang]
    p = PREFIX[lang]
    out = [f'<a id="{h["anchor"]}"></a>', h['lang_line'], f'# {h["title"]}', inline(lede), h['after'],
           h['toc'] + ' ' + ' · '.join(f'[{inline(unjs(name))}](#{p}-{sid})' for sid, name, _ in sections)]
    susp = current_susp(lang)
    for sid, name, body in sections:
        out.append(f'<a id="{p}-{sid}"></a>')
        out.append(f'## {inline(unjs(name))}')
        out.append(block(unjs(body), susp))
        if sid in IMAGES[lang]:
            out.append(IMAGES[lang][sid])
    return '\n\n'.join(out)


doc = build('ru') + '\n\n---\n\n' + build('en') + '\n'
open(OUT, 'w', encoding='utf-8', newline='\n').write(doc)
print(len(doc.splitlines()), 'lines')
