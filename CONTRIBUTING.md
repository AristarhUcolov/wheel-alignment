<a id="ru"></a>

**Русский** · [English](#eng)

# Как помочь проекту

Спасибо, что заглянули. Цель проекта простая: чтобы человек с гаражом и
рулеткой мог сам отрегулировать углы установки колёс своей машины и не зависеть
от того, возьмётся за неё СТО или нет.

## Самое ценное: данные по автомобилям

Программа уже умеет считать. Чего ей не хватает — **проверенных заводских
допусков**. Комплектная база это лицензионный продукт, и купить её проект не
может, а выдумывать цифры нельзя: по неверному развалу человек угробит резину
и получит машину, которая плохо ведёт себя в экстренной ситуации.

Если у вас **есть руководство по ремонту** — внесите оттуда данные. Одна запись
помогает всем владельцам этой модели, навсегда. Особенно нужна **сверка
записей «не проверено»**: у ГАЗ-3110, ГАЗели, УАЗ-469, ВАЗ-2101…2110, «Нивы» и
Chevrolet Niva цифры взяты из открытых пересказов руководств — сверьте их с
изданием, укажите страницу, и запись станет заводской.

Правила и формат: **[docs/CONTRIBUTING-SPECS.md](docs/CONTRIBUTING-SPECS.md)**

Коротко:
- у каждой записи обязательно поле `source` с указанием документа;
- не уверены в источнике — ставьте `unverified`, программа покажет
  предупреждение, и это честно;
- отсутствующее поле лучше выдуманного: «нет данных» — нормальный ответ;
- у каждой записи есть блок `en` с английским текстом — тест проверит, что он
  не забыт.

CI проверит формат за вас. Go устанавливать не обязательно — можно прислать
pull request прямо через веб-интерфейс GitHub.

## Ошибки и неточности

Если программа посчитала не то, что показал профессиональный стенд —
**это важный баг, откройте issue**. Приложите:

- измеренные значения (или JSON запроса из вкладки «Сеть» браузера);
- что показал стенд;
- марку, модель, год, размер дисков;
- файл `wheelalign.log` из папки данных (`%APPDATA%\wheelalign`).

Расхождение по кастеру до ~0,3° может быть не ошибкой: многие стенды считают
по приближённой формуле «размах развала × 1,5», а здесь задача решается точно
(см. `SweepReading.Solve` и `CasterClassic` для сравнения). Но напишите
всё равно — интересно.

## Код

```bash
go test ./...     # должно быть зелено
gofmt -w .        # перед коммитом
go vet ./...
```

Что стоит знать об устройстве:

- **`internal/align` знает только про оси вращения колёс в пространстве.**
  Откуда они взялись — камера, лазер, струна или угломер — ему безразлично.
  Новый способ измерения подключается через `align.RawWheel`, математику
  трогать не нужно.
- **Соглашения о знаках зафиксированы тестами.** `internal/simulate` — прямая
  модель (углы → оси), `internal/align` — обратная. Тесты гоняют одно через
  другое для обоих бортов. Если меняете знак — тест упадёт, и это правильно.
- **Числа, которые нельзя измерить точно, должны об этом говорить.** SAI из
  замера с поворотом считается делением на 0,06 — программа его выдаёт вместе
  с предупреждением. Такой честности придерживаемся везде.
- **Живой экран и печатный отчёт не должны расходиться.** `internal/live`
  формирует линию тяги, суммы и разницы по бортам так же, как `align.Assemble`,
  а допуски берёт через `specs.Lookup` — ту же функцию, что и `specs.Compare`.
  Тесты сверяют их до последнего знака.
- **Тестовые данные строятся из физики, а не из решаемых уравнений.** Модель
  телефона в `internal/phone` — это «телефон с таким-то смещением нуля и
  выступом камеры, приложенный к такому-то колесу», а не обращённые формулы
  калибровки. Иначе тест согласится с ошибкой в выводе.
- Интерфейс — обычные HTML, CSS и JS-модули в `internal/server/web`, без сборки.
  Проверить глазами: `go run ./cmd/wheelalign -browser`.

### Два языка

Ключ перевода — **русская строка**. Этого достаточно, чтобы ничего не забыть:

- в Go сообщение пишется как `i18n.T("…")`, `i18n.F("… %s", x)` или
  `i18n.Err("…")`, английский — в `internal/i18n/en_*.go`;
- в интерфейсе — `t('…', {переменная})` или атрибут `data-t` в HTML, английский —
  в `internal/server/web/js/i18n-en.js`;
- страница телефона держит свой маленький словарь внутри `phone.html`;
- инструкция (F1) — целыми разделами в `js/guide/ru.js` и `js/guide/en.js`.

Тесты находят каждую строку в исходниках и падают, если у неё нет перевода, если
в переводе потерялся `%s` или `{переменная}`, или если в словаре осталась
строка, которой больше нет в коде. `I18N_DUMP=файл go test ./internal/i18n/`
выпишет недостающие строки в файл.

## Что сейчас нужнее всего

1. **Данные по автомобилям** — см. выше. Проще всего — кнопкой «Внести допуски»
   в программе: она проверит цифры и подготовит файл.
2. **Проверка на реальных машинах.** Замерили этой программой (струной,
   телефоном, камерой) и стендом — расскажите, насколько сошлось. Особенно
   ценно для телефона: у разных моделей разные акселерометры.
3. **Типы подвески в каталоге.** Если точно знаете, какая подвеска у модели,
   которой нет в `internal/specs/data/`, — добавьте запись `catalog` в файл её
   марки (`toyota.json`, `gaz.json`…): цифр она не требует, а человек найдёт
   свою машину и получит советы под её конструкцию.
4. **Датчики.** Сделали голову на ESP32 или лазерный указатель схождения —
   поделитесь схемой и прошивкой. Протокол — [docs/SENSORS.md](docs/SENSORS.md).
5. **Другие языки.** Русский и английский уже есть; устройство перевода описано
   выше — третий язык добавляется тем же путём.

Можно поддержать и деньгами — ссылки в [README](README.md#поддержать-проект).

## Лицензия

Проект под **AGPL-3.0**. Внося вклад, вы соглашаетесь опубликовать его на тех
же условиях. Смысл выбора: кто поднимет этот стенд как платный веб-сервис,
обязан открыть свои доработки. Программа должна остаться бесплатной для тех,
ради кого она сделана.

## Безопасность

Тормоза, рулевое и подвеска — узлы, отказ которых убивает. Если ваше изменение
может привести к тому, что человек отрегулирует машину по неверным числам —
напишите об этом прямо в pull request. Лучше обсудить лишний раз.

---
---

<a id="eng"></a>

[Русский](#ru) · **English**

# How to help the project

Thanks for stopping by. The goal is simple: a person with a garage and a tape
measure should be able to align their own car's wheels and not depend on
whether a workshop agrees to take it.

## Most valuable: vehicle data

The program can already calculate. What it lacks is **verified factory
tolerances**. A complete database is a licensed product that the project cannot
buy, and figures must never be made up: with a wrong camber a person ruins their
tyres and gets a car that behaves badly in an emergency.

If you **have a workshop manual**, enter its data. One entry helps every owner
of that model, for good. **Checking the “unverified” entries** is especially
needed: for the GAZ-3110, GAZelle, UAZ-469, VAZ-2101…2110, Niva and Chevrolet
Niva the figures come from open retellings of manuals — check them against the
edition, give the page, and the entry becomes a factory one.

Rules and format: **[docs/CONTRIBUTING-SPECS.md](docs/CONTRIBUTING-SPECS.md#eng)**

In short:
- every entry must have a `source` field naming the document;
- not sure about the source — use `unverified`; the program shows a warning,
  and that is honest;
- a missing field is better than an invented one: “no data” is a fine answer;
- every entry has an `en` block with its English text — a test checks it is
  not forgotten.

CI checks the format for you. You don't need to install Go — a pull request can
be sent straight from GitHub's web interface.

## Bugs and inaccuracies

If the program calculated something different from what a professional aligner
showed — **that is an important bug, open an issue**. Attach:

- the measured values (or the request JSON from the browser's Network tab);
- what the aligner showed;
- make, model, year, rim size;
- `wheelalign.log` from the data folder (`%APPDATA%\wheelalign`).

A caster difference of up to ~0.3° may not be a bug: many aligners use the
approximate “camber swing × 1.5” formula, while here the problem is solved
exactly (see `SweepReading.Solve`, and `CasterClassic` for comparison). Write
anyway — it is interesting.

## Code

```bash
go test ./...     # must be green
gofmt -w .        # before committing
go vet ./...
```

Worth knowing about the design:

- **`internal/align` knows only about the wheels' axes of rotation in space.**
  Where they came from — camera, laser, string or inclinometer — does not
  matter to it. A new measuring method plugs in through `align.RawWheel`; the
  mathematics stays untouched.
- **Sign conventions are pinned by tests.** `internal/simulate` is the forward
  model (angles → axes), `internal/align` the inverse. Tests run one through the
  other for both sides. Change a sign and a test fails — as it should.
- **Numbers that cannot be measured precisely must say so.** SAI from a steer
  sweep is divided by 0.06 — the program reports it together with a warning. We
  keep to that honesty everywhere.
- **The live screen and the printed report must never disagree.** `internal/live`
  derives the thrust line, totals and cross differences the same way as
  `align.Assemble`, and takes tolerances through `specs.Lookup` — the same
  function `specs.Compare` uses. Tests compare them to the last digit.
- **Test data is built from physics, not from the equations being solved.** The
  phone model in `internal/phone` is “a phone with such a zero offset and camera
  bump, held to such a wheel”, not the calibration formulas inverted. Otherwise
  the test would agree with a mistake in the derivation.
- The interface is plain HTML, CSS and JS modules in `internal/server/web`, no
  build step. To look at it: `go run ./cmd/wheelalign -browser`.

### Two languages

The translation key is **the Russian string**. That is enough to forget nothing:

- in Go a message is written as `i18n.T("…")`, `i18n.F("… %s", x)` or
  `i18n.Err("…")`; the English is in `internal/i18n/en_*.go`;
- in the interface — `t('…', {variable})` or a `data-t` attribute in HTML; the
  English is in `internal/server/web/js/i18n-en.js`;
- the phone page keeps its own small dictionary inside `phone.html`;
- the guide (F1) is translated section by section in `js/guide/ru.js` and
  `js/guide/en.js`.

Tests find every string in the sources and fail if it has no translation, if a
`%s` or `{variable}` got lost in the translation, or if the dictionary keeps a
string the code no longer uses. `I18N_DUMP=file go test ./internal/i18n/` writes
the missing strings to a file.

## Most needed right now

1. **Vehicle data** — see above. The easiest way is the “Enter specs” button in
   the program: it checks the figures and prepares the file.
2. **Testing on real cars.** Measured with this program (string, phone, camera)
   and on an aligner — tell us how well they agreed. Especially valuable for the
   phone: different models have different accelerometers.
3. **Suspension types in the catalogue.** If you know for sure which suspension
   a model missing from `internal/specs/data/` has, add a `catalog` entry to its
   make's file (`toyota.json`, `gaz.json`…): it needs no figures, and people will
   find their car and get advice for its design.
4. **Sensors.** Built an ESP32 head or a laser toe gauge — share the schematic
   and firmware. Protocol: [docs/SENSORS.md](docs/SENSORS.md#eng).
5. **Other languages.** Russian and English are done; the translation set-up is
   described above — a third language goes the same way.

You can also support the project with money — links in the
[README](README.md#support-the-project).

## Licence

The project is under **AGPL-3.0**. By contributing you agree to publish your
contribution under the same terms. The reason: whoever runs this aligner as a
paid web service must publish their changes. The program must stay free for
the people it was made for.

## Safety

Brakes, steering and suspension are parts whose failure kills. If your change
could lead someone to align a car by wrong numbers, say so plainly in the pull
request. Better to discuss it once too often.
