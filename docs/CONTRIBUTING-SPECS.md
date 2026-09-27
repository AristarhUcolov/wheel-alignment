<a id="ru"></a>

**Русский** · [English](#eng)

# Как добавить автомобиль в базу

Это самый ценный вклад в проект. Одна проверенная запись помогает всем
владельцам этой модели — навсегда.

## Проще всего — через форму

Запустите программу, выберите свою машину (или ближайшую) и нажмите
**«Внести допуски»** в меню слева. Марка, модель, годы и тип подвески
подставятся сами. Углы вводите так, как они напечатаны в руководстве:
`0°30'`, `-0 30` или `0.5` — рядом с полем видно, как программа поняла число.

- **«Проверить»** — программа скажет, всё ли в порядке, и покажет, **во что
  превратились ваши цифры** (миллиметры в градусы, суммарное схождение в
  схождение одного колеса).
- **«Сохранить у себя и выбрать»** — запись сохраняется в профиле
  (`%APPDATA%\wheelalign\vehicles` в Windows), находится поиском первой и
  переживает обновления программы. Сразу можно регулировать по своим цифрам.
- **«Скачать файл для проекта»** — готовый файл, который можно прислать в
  проект, чтобы им пользовались все владельцы этой модели.

Знать JSON не нужно. Go тоже.

Если работаете с файлами напрямую — проверить можно из командной строки:

```bash
wheelalign check-spec internal/specs/data/gaz.json
```

Она выполняет ровно те же проверки, что и программа при запуске, плюс проверку
правдоподобия: ловит потерянный знак, минуты, введённые как десятые доли
градуса, перепутанные местами оси. Возвращает код 1 при ошибках, так что годится
и для CI.

Ниже — описание формата для тех, кто правит файлы руками.

## Правило номер один

**Не вносите цифры «по памяти» или «с форума» под видом заводских.**

Придуманный или перевранный угол развала — это не безобидная опечатка. Человек
отрегулирует по нему машину, съест комплект резины за сезон и получит машину,
которая непредсказуемо ведёт себя при экстренном манёвре.

Поэтому у каждой записи в базе **обязательно** есть поле `source`, и программа
показывает предупреждение везде, где данные не подтверждены заводским
документом. Это не бюрократия — это единственное, что отличает полезную базу
от опасной.

## Уровни доверия

| `kind` | Когда ставить |
|---|---|
| `factory` | Вы держите в руках заводское руководство и указали издание и страницу |
| `licensed` | Данные из лицензионной базы (Autodata, Mitchell, ALLDATA…), с указанием |
| `community` | Внесено владельцем и **перепроверено по независимому источнику** |
| `unverified` | Один непроверенный источник. Работает, но с крупным предупреждением |
| `class_guidance` | Не про конкретную машину — типовые диапазоны для класса конструкции |
| `catalog` | Только конструкция модели (класс, подвеска, годы), **без единого допуска** |

Записи `catalog` существуют, чтобы машину можно было найти и получить советы
под её подвеску, пока допусков нет. Углы тогда сравниваются с ориентиром по
классу, указанным в `guidance_id`, — с явной пометкой. Цифр в такой записи быть
не может: программа её отвергнет. Нашли допуски — замените `catalog` на запись с
источником (тот же `id`, чтобы у людей не потерялся выбор).

Если сомневаетесь между `community` и `unverified` — ставьте `unverified`.

**Цифры из открытого источника** (сайт, пересказывающий руководство) — это
`unverified`, и в `source` указываются и название страницы, и её адрес в `url`:
тогда любой откроет страницу прямо из программы и проверит. Если текст
источника неоднозначен (например, неясно, минус перед числом или тире), —
**не вносите этот угол** и напишите в `notes`, почему его нет.

## Формат

Файл — **по марке**: `internal/specs/data/gaz.json`, `toyota.json`,
`mercedes.json`. Все записи одной марки — в одном файле (это проверяет тест).
Все углы — **в десятичных градусах**.

```json
{
  "specs": [
    {
      "id": "марка-модель-годы",
      "make": "Марка",
      "model": "Модель",
      "trim": "Модификация, если данные для неё отличаются",
      "tags": ["альтернативные", "написания", "и", "прозвища"],
      "year_from": 1985,
      "year_to": 1995,
      "class": "car",
      "front_suspension": "macpherson",
      "rear_suspension": "twist_beam",
      "rim_diameter_in": 14,

      "front_total_toe_mm": { "min": 1.0, "max": 3.0 },

      "front": {
        "camber":    { "min": -0.5, "nominal": 0.0, "max": 0.5 },
        "caster":    { "min": 2.0,  "nominal": 2.5, "max": 3.0 },
        "sai":       { "min": 11.0, "nominal": 12.0, "max": 13.0 },
        "max_cross_camber": 0.5,
        "max_cross_caster": 0.5,
        "adjustable": {
          "camber": true,
          "caster": false,
          "toe": true,
          "camber_method": "эксцентриковый болт нижнего крепления стойки",
          "caster_method": "штатной регулировки нет",
          "toe_method": "резьбовые рулевые тяги"
        }
      },
      "rear": {
        "camber": { "min": -1.5, "nominal": -1.0, "max": -0.5 },
        "total_toe": { "min": 0.0, "nominal": 0.15, "max": 0.3 },
        "adjustable": { "camber": false, "caster": false, "toe": false }
      },

      "max_thrust_angle": 0.2,

      "conditions": {
        "load": "снаряжённая масса, полный бак",
        "pressure": "по табличке завода",
        "settle": "прокатить вперёд 3–5 м"
      },

      "source": {
        "kind": "factory",
        "reference": "Название руководства, издательство, год, страница",
        "url": "https://… (если источник — страница в сети)",
        "contributor": "как вас указать",
        "added": "2026-09-27"
      },

      "en": {
        "make": "Make",
        "model": "Model",
        "trim": "Variant",
        "notes": "…",
        "reference": "Manual title, publisher, year, page",
        "conditions": { "load": "kerb weight, full tank", "pressure": "per the factory placard", "settle": "roll forward 3–5 m" },
        "front": {
          "camber_method": "eccentric bolt at the lower strut mounting",
          "caster_method": "no standard adjustment",
          "toe_method": "threaded tie rods"
        }
      }
    }
  ]
}
```

**Блок `en`** — английский текст той же записи: только слова, **без цифр**
(цифры одни на оба языка). Пустое поле в `en` показывается по-русски, но для
встроенной базы тест требует перевода каждого непустого текста. В своих записях
(«Сохранить у себя») блок `en` необязателен.

## Класс и тип подвески

`class` — `car` (легковой), `suv` (внедорожник), `lcv` (фургон, лёгкий
грузовик, пикап), `truck` (грузовой), `bus` (автобус).

`front_suspension` и `rear_suspension` — от них зависят проверки перед
регулировкой и советы «чем регулируется». Указывайте, только если уверены:
неверный тип отправит человека искать шкворень, которого нет. Если задний мост
бывает и на рессорах, и на пневмобаллонах (седельные тягачи) — оставьте тип
пустым и напишите об этом в `notes`.

| Значение | Подвеска | Ось |
|---|---|---|
| `macpherson` | Макферсон (стойка) | любая |
| `double_wishbone_ball` | Двухрычажная на шаровых опорах | любая |
| `double_wishbone_kingpin` | Двухрычажная шкворневая (ГАЗ-21, 24, 3102, 3110) | передняя |
| `beam_kingpin` | Неразрезная балка (мост) со шкворнями — грузовики, автобусы, УАЗ | передняя |
| `beam_ball_joint` | Неразрезной мост на шаровых опорах | передняя |
| `multilink` | Многорычажная | любая |
| `twist_beam` | Полузависимая балка | задняя |
| `live_axle_leaf` | Неразрезной мост на рессорах | задняя |
| `live_axle_links` | Неразрезной мост на пружинах (пневмобаллонах) и тягах | задняя |
| `trailing_arm` | Независимая на продольных рычагах | задняя |
| `semi_trailing_arm` | Независимая на косых рычагах | задняя |

Если у модели бывали разные исполнения (ГАЗ-3110 — шкворневая и
бесшкворневая), укажите основное и напишите об этом в `notes`: владелец сменит
тип в программе одним выбором.

## Что проверяет программа при загрузке

Запись **не попадёт в базу**, если:

- нет `source`, или у `factory`/`licensed` не указан документ;
- схождение задано в миллиметрах, но нет `rim_diameter_in` — такие данные
  невозможно интерпретировать (3 мм на 13″ и на 17″ это разные углы);
- верхняя граница допуска меньше нижней;
- номинал лежит вне собственного допуска;
- указан неизвестный класс или тип подвески — или тип, которого не бывает на
  этой оси (балка сзади вместо спереди и т.п.);
- запись `catalog` содержит хоть один допуск.

Ошибки печатаются при запуске и видны в `/api/health` — они не проглатываются.

Для встроенной базы тесты дополнительно требуют: `id` не повторяется ни в одном
файле, у марки один файл, у каждой записи есть английский текст.

## Что заполнять обязательно, а что нет

Обязательно: `id`, `make`, `model`, `year_from`, `source`.

Всё остальное — по наличию. **Отсутствующее поле лучше выдуманного.** Если в
мануале нет кастера — не пишите кастер. Программа покажет «нет данных», и это
честный ответ.

Особенно важно заполнять `adjustable`: без него программа не сможет сказать
человеку «этот угол у вашей машины штатно не регулируется, ищите деформацию» —
а это часто самый полезный вывод из всего замера.

## Проверка

```
wheelalign check-spec internal/specs/data/ваш-файл.json
go test ./internal/specs/
go run ./cmd/wheelalign
```

Первая команда — главная: она скажет, загрузится ли запись, и предупредит о
неправдоподобных значениях. Тесты проверяют схему, происхождение и перевод.
Запуск покажет, находится ли запись поиском.

### Что проверяется, помимо структуры

Эти проверки **не отвергают** запись — они задают вопрос. Необычные, но
настоящие значения существуют (гоночный развал, необычная старая конструкция),
и база, которая отказывалась бы записать правду за то, что правда выглядит
странно, была бы хуже той, что переспрашивает. Но типовые ошибки они ловят:

- углы за пределами того, что бывает у дорожных автомобилей — почти всегда это
  минуты, введённые как десятые доли градуса (`0°30'` это `0.5`, а не `0.30`);
- отрицательный кастер на всём диапазоне — потерянный знак;
- кастер у задней оси — у неуправляемой оси его не бывает, это данные передней;
- схождение в миллиметрах, дающее неправдоподобный угол — почти всегда неверен
  диаметр обода, к которому они отнесены;
- угол, заданный одним числом без допуска.

## Заметки на полях

- **Единицы.** Если мануал даёт схождение в миллиметрах по ободу — вносите и
  `front_total_toe_mm`, и `rim_diameter_in`. Программа сама переведёт в градусы
  и будет показывать обе формы, чтобы владелец мог сверяться с мануалом в его
  собственных единицах.
- **Миллиметры по шинам.** Если схождение меряют по шинам (ГАЗель, УАЗ), база
  замера — диаметр шины, а не обода. Переведите в градусы сами
  (`atan(мм / диаметр_шины_мм)`), внесите как `total_toe`, а исходные миллиметры
  и диаметр шины опишите в `notes`. `rim_diameter_in` оставьте настоящим: по нему
  программа переводит замеры струной по ободу.
- **Градусы и минуты.** `0°30'` = `0.5`, `1°15'` = `1.25`. Минуты делите на 60.
- **Знаки.** Развал положителен, когда верх колеса наклонён **наружу**. Кастер
  положителен, когда верх оси поворота наклонён **назад** («нижний конец шкворня
  вперёд» — это тоже плюс). Схождение положительно при схождении внутрь
  (toe-in). Многие мануалы совпадают с этим, но не все — проверяйте.
- **Нагрузка.** ВАЗы, например, дают допуски под нагрузкой 320 кг (четыре
  человека и 40 кг в багажнике), а не в снаряжённом состоянии. Цифры без условий,
  для которых они написаны, — половина информации: заполняйте `conditions`.
- **Модификации.** Если у седана и универсала разные допуски — это две записи с
  разными `trim`, а не усреднение.

---
---

<a id="eng"></a>

[Русский](#ru) · **English**

# How to add a vehicle to the database

This is the most valuable contribution to the project. One verified entry helps
every owner of that model — for good.

## Easiest — through the form

Start the program, choose your car (or the closest one) and press **“Enter
specs”** in the menu on the left. Make, model, years and suspension type are
filled in for you. Enter the angles exactly as printed in the manual: `0°30'`,
`-0 30` or `0.5` — next to the field you see how the program read the number.

- **“Check”** — the program says whether everything is fine and shows **what
  your figures became** (millimetres to degrees, total toe to per-wheel toe).
- **“Save locally and select”** — the entry is saved in your profile
  (`%APPDATA%\wheelalign\vehicles` on Windows), comes first in search and
  survives program updates. You can adjust by your own figures straight away.
- **“Download the file for the project”** — a ready file to send to the
  project so that every owner of the model can use it.

No need to know JSON. Or Go.

If you work with the files directly, you can check them from the command line:

```bash
wheelalign check-spec internal/specs/data/gaz.json
```

It runs exactly the checks the program runs at start-up, plus plausibility
checks: it catches a lost sign, minutes entered as tenths of a degree, swapped
axles. It exits with code 1 on errors, so it works in CI too.

The format is described below for those editing files by hand.

## Rule number one

**Do not enter figures “from memory” or “from a forum” as factory figures.**

An invented or garbled camber angle is not a harmless typo. Someone will align
their car by it, eat a set of tyres in a season and get a car that behaves
unpredictably in an emergency manoeuvre.

That is why every entry **must** have a `source` field, and the program shows a
warning wherever the data is not confirmed by a factory document. This is not
bureaucracy — it is the only thing that separates a useful database from a
dangerous one.

## Trust levels

| `kind` | When to use it |
|---|---|
| `factory` | You hold the factory manual and have given the edition and page |
| `licensed` | Data from a licensed database (Autodata, Mitchell, ALLDATA…), named |
| `community` | Entered by an owner and **cross-checked against an independent source** |
| `unverified` | A single unchecked source. Works, but with a big warning |
| `class_guidance` | Not about a particular car — typical ranges for a class of design |
| `catalog` | The model's design only (class, suspension, years), **not a single tolerance** |

`catalog` entries exist so that a car can be found and get advice for its
suspension while no tolerances are known. The angles are then compared with the
class guidance named in `guidance_id` — clearly marked. Such an entry cannot
carry figures: the program rejects it. Found tolerances? Replace the `catalog`
entry with a sourced one (keep the same `id`, so people's choice is not lost).

When unsure between `community` and `unverified`, use `unverified`.

**Figures from an open source** (a website retelling a manual) are
`unverified`, and `source` names the page and gives its address in `url`: then
anyone can open the page right from the program and check. If the source text is
ambiguous (say, it is unclear whether a character before a number is a minus or
a dash), **do not enter that angle** and say in `notes` why it is missing.

## Format

One file **per make**: `internal/specs/data/gaz.json`, `toyota.json`,
`mercedes.json`. All entries of a make go in one file (a test checks this). All
angles are **in decimal degrees**. The full example is in the
[Russian section](#формат) above; the fields are the same, and the Russian text
fields (`make`, `model`, `trim`, `notes`, `source.reference`, `conditions`, the
`*_method` texts) have their English versions in the `en` block.

**The `en` block** holds the English text of the same entry: words only, **no
figures** (the figures are shared by both languages). An empty field in `en`
falls back to Russian, but for the built-in database a test requires every
non-empty text to be translated. In your own entries (“Save locally”) the `en`
block is optional.

## Class and suspension type

`class` — `car`, `suv`, `lcv` (van, light truck, pickup), `truck`, `bus`.

`front_suspension` and `rear_suspension` drive the pre-adjustment checks and the
“what adjusts it” advice. Only fill them in when you are sure: a wrong type sends
someone looking for a kingpin that does not exist. If the rear axle comes both on
leaf springs and on air bags (tractor units), leave the type empty and say so in
`notes`.

| Value | Suspension | Axle |
|---|---|---|
| `macpherson` | MacPherson strut | either |
| `double_wishbone_ball` | Double wishbone, ball joints | either |
| `double_wishbone_kingpin` | Double wishbone with kingpins (GAZ-21, 24, 3102, 3110) | front |
| `beam_kingpin` | Rigid beam (axle) with kingpins — trucks, buses, UAZ | front |
| `beam_ball_joint` | Rigid axle on ball joints | front |
| `multilink` | Multi-link | either |
| `twist_beam` | Twist beam | rear |
| `live_axle_leaf` | Live axle on leaf springs | rear |
| `live_axle_links` | Live axle on coil springs (air bags) and links | rear |
| `trailing_arm` | Independent trailing arms | rear |
| `semi_trailing_arm` | Independent semi-trailing arms | rear |

If a model came in different versions (the GAZ-3110 with and without
kingpins), give the main one and describe the other in `notes`: the owner
switches the type in the program with one choice.

## What the program checks at load time

An entry is **rejected** if:

- there is no `source`, or a `factory`/`licensed` source names no document;
- toe is given in millimetres without `rim_diameter_in` — such data cannot be
  interpreted (3 mm on a 13″ and on a 17″ rim are different angles);
- a tolerance's upper limit is below its lower one;
- the nominal lies outside its own tolerance;
- the class or suspension type is unknown — or the type does not exist on that
  axle (a beam at the rear instead of the front and so on);
- a `catalog` entry carries any tolerance at all.

Errors are printed at start-up and shown in `/api/health` — they are not
swallowed.

For the built-in database the tests also require: no `id` repeats in any file,
each make has one file, every entry has its English text.

## What is required and what is not

Required: `id`, `make`, `model`, `year_from`, `source`.

Everything else — if you have it. **A missing field is better than an invented
one.** If the manual gives no caster, do not write a caster. The program shows
“no data”, and that is an honest answer.

Filling in `adjustable` matters especially: without it the program cannot tell
someone “this angle has no standard adjustment on your car, look for
deformation” — often the most useful conclusion of the whole measurement.

## Checking

```
wheelalign check-spec internal/specs/data/your-file.json
go test ./internal/specs/
go run ./cmd/wheelalign
```

The first command is the main one: it tells you whether the entry will load and
warns about implausible values. The tests check the schema, provenance and
translation. Running the program shows whether search finds the entry.

### What is checked beyond structure

These checks **do not reject** an entry — they ask a question. Unusual but real
values exist (race camber, an odd old design), and a database that refused to
record the truth because the truth looks strange would be worse than one that
asks again. But they catch the typical mistakes:

- angles beyond anything road cars have — nearly always minutes entered as
  tenths of a degree (`0°30'` is `0.5`, not `0.30`);
- negative caster over the whole range — a lost sign;
- caster on the rear axle — a non-steered axle has none; that is front data;
- millimetre toe giving an implausible angle — nearly always the wrong rim
  diameter for those millimetres;
- an angle given as a single number with no tolerance.

## Notes in the margin

- **Units.** If the manual gives toe in millimetres at the rim, enter both
  `front_total_toe_mm` and `rim_diameter_in`. The program converts to degrees and
  shows both forms, so the owner can check against the manual in its own units.
- **Millimetres at the tyres.** If toe is measured on the tyres (GAZelle, UAZ),
  the reference is the tyre diameter, not the rim's. Convert to degrees yourself
  (`atan(mm / tyre_diameter_mm)`), enter it as `total_toe`, and describe the
  original millimetres and tyre diameter in `notes`. Keep `rim_diameter_in` the
  real one: the program uses it to convert string measurements taken at the rim.
- **Degrees and minutes.** `0°30'` = `0.5`, `1°15'` = `1.25`. Divide the minutes
  by 60.
- **Signs.** Camber is positive when the top of the wheel leans **out**. Caster
  is positive when the top of the steering axis leans **back** (“the lower end of
  the kingpin forward” is also positive). Toe is positive for toe-in. Many manuals
  agree with this, not all — check.
- **Load.** VAZ cars, for example, give tolerances under a 320 kg load (four
  people and 40 kg in the boot), not at kerb weight. Figures without the
  conditions they were written for are half the information: fill in
  `conditions`.
- **Variants.** If the saloon and the estate have different tolerances, that is
  two entries with different `trim`, not an average.
