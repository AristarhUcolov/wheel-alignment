<a id="ru"></a>

**Русский** · [English](#eng)

# Сход-развал своими руками: полная процедура

Этот документ — рабочая инструкция. Он описывает метод, который даёт точность
**лучше 0,1°** по развалу и схождению при аккуратной работе, на оборудовании
стоимостью примерно как один заезд на СТО.

Программа считает углы. Этот документ отвечает на вопрос, откуда взять числа,
которые в неё вводить. Как пользоваться самой программой, экран за экраном, —
[GUIDE.md](GUIDE.md).

---

## 1. Инструмент

### Обязательное

| Что | Зачем | Чем заменить |
|---|---|---|
| **Струна** — леска 0,3–0,5 мм, 2 отрезка по 5 м | Опорная линия для схождения | Только не нитка: она провисает и тянется |
| **Стойки для струн** — 4 шт., высотой до центра ступицы | Держат струну | Домкраты, кирпичи + бруски, стулья, ведра с песком |
| **Груз для натяжения** — 2–4 кг на каждую струну | Убирает провисание | Гантели, бутыли с водой |
| **Рулетка** 5 м + **штангенциркуль** или **металлическая линейка** | Замер отступов | — |
| **Угломер** — магнитный, цифровой, или смартфон | Развал | Пузырьковый уровень с транспортиром |
| **Ключи** для контргаек рулевых тяг | Регулировка схождения | — |
| **Опоры (подставки)** | Безопасность при работе под машиной | **Не заменять домкратом. Никогда.** |

### Для кастера (по желанию, но очень желательно)

| Что | Зачем | Как сделать самому |
|---|---|---|
| **Поворотные круги**, 2 шт. | Дают колесу поворачиваться без «закусывания» | Два листа жести 30×30 см, между ними солидол или графитная смазка. Работает |
| **Транспортир** на круге | Отмерить 20° поворота | Распечатать шкалу и приклеить к нижнему листу |

### Для проверки площадки

**Гидроуровень** (прозрачный шланг с водой, 10 м) или **длинное правило** +
обычный уровень. Это самая недооценённая позиция в списке — см. раздел 2.

---

## 2. Площадка: почему это главное

Программе нужно, чтобы все четыре колеса стояли **в одной плоскости**.
Не «горизонтально» — именно в одной плоскости.

Почему это критично: если под одним колесом яма 5 мм, а колея 1400 мм, то
кузов наклонён на `atan(5/1400)` ≈ **0,2°**. Ровно на столько же врёт развал
этого борта — а 0,2° это половина заводского допуска на многих машинах.

**Как проверить.** Гидроуровнем отметьте уровень воды на четырёх точках, где
стоят колёса. Разница между любыми двумя точками должна быть **меньше 2 мм**.
Если больше — подложите под колёса стальные пластины нужной толщины
(не деревяшки: они сминаются под весом).

**Если ровной площадки нет вообще** — в программе есть режим отсчёта от
плоскости дороги вместо гравитации, но он требует оптического замера (см.
[OPTICAL.md](OPTICAL.md)). Для угломера и телефона ровная площадка обязательна:
оба меряют наклон относительно силы тяжести.

---

## 3. Подготовка автомобиля

Порядок именно такой:

1. **Давление в шинах** — по заводской табличке, обязательно **одинаковое
   слева и справа**. Разница 0,3 бар это до 0,1° разницы развала.
2. **Люфты.** Покачайте каждое колесо руками — сверху-снизу (шаровые опоры,
   ступичный подшипник) и слева-справа (рулевые наконечники). Любой ощутимый
   люфт означает, что **регулировать бесполезно**: в статике вы выставите одно,
   а в движении будет другое. Сначала ремонт.
3. **Сайлентблоки рычагов.** Потрескавшиеся или продавленные — та же история.
4. **Загрузка.** Приведите машину в то состояние, которое указано в
   спецификации: обычно полный бак, запаска и инструмент на местах, без
   пассажиров. Для некоторых машин требуется груз на сиденьях (ВАЗы — 320 кг:
   четыре человека и 40 кг в багажнике) — программа покажет условия в блоке
   «Условия замера».
5. **Осадка подвески.** Покачайте кузов и **прокатите машину 3–5 м вперёд**.
   Не толкайте вбок и не катите назад — подвеска должна занять то положение,
   которое принимает в движении.
6. **Руль — строго прямо.** Зафиксируйте его (палкой в упор в сиденье, или
   ремнём). Проверьте по метке на рулевом валу, а не «на глаз по спицам»:
   спицы могли поставить криво при предыдущем ремонте.

---

## 4. Натяжка струн

Струны идут **вдоль бортов, снаружи колёс, на высоте центра ступицы**,
параллельно продольной оси автомобиля.

### Почему «одинаковые отступы» — это ошибка

Народный совет «сделай расстояние от струны до ступицы одинаковым спереди и
сзади» **верен только если передняя и задняя колея равны**. У большинства машин
они разные.

Если колея передней оси `Tf`, задней `Tr`, то для струны, параллельной оси
автомобиля, отступы должны отличаться на:

```
отступ_спереди − отступ_сзади = (Tr − Tf) / 2
```

Пример: ВАЗ-2101, Tf = 1349 мм, Tr = 1305 мм. Разница = (1305 − 1349)/2 =
**−22 мм**. То есть спереди струна должна быть на 22 мм **ближе** к ступице,
чем сзади. Сделаете одинаково — получите ошибку схождения около 0,5° на борт.

**Программа считает эту величину за вас.** Введите колеи и четыре отступа в
блоке «Проверка установки струн» — она скажет, на сколько миллиметров и в какую
сторону подвинуть каждый конец.

### Практика

- Струна должна быть **натянута грузом**, а не завязана. Провисание 1 мм — это
  ошибка замера 1 мм.
- Замеряйте отступ **до центра ступицы** (до центра колпака ступицы или до
  центра болта). Это единственная точка, положение которой не зависит от
  развала и схождения.
- Струна не должна касаться шины нигде.

---

## 5. Замер развала

Магнитный угломер ставится **на обод, а не на резину**. Если обод литой и
неровный — приложите к нему ровную планку и меряйте по планке.

### Компенсация биения диска — делайте её всегда

Ни один диск не сидит идеально перпендикулярно оси. Ошибка посадки в 0,3°
полностью съедает заводской допуск.

1. Замерьте развал, запишите. Это значение «0°».
2. **Прокатите машину так, чтобы колесо провернулось ровно на пол-оборота.**
   Отметьте мелом точку на ободе, чтобы не ошибиться.
3. Поставьте угломер **на то же самое место обода**. Запишите. Это «180°».
4. Введите оба значения.

Программа усреднит их (истинный развал) и покажет разницу (биение диска).
Работает это потому, что при повороте на 180° перекос обода меняет знак, а сам
развал — нет.

Если программа показывает биение больше 0,5° — диск погнут или на нём грязь,
и **все замеры на этом колесе недостоверны**.

### Развал телефоном

Вместо угломера можно взять смартфон — он передаёт развал на экран в реальном
времени. В программе: «Замер → Телефон на колесе → Разрешить подключение по
Wi-Fi», отсканировать QR-код телефоном, выбрать колесо.

Телефон без калибровки врёт на 0,5–2°: смещение нуля его акселерометра больше
любого заводского допуска. Калибровка делается **один раз** и запоминается:

1. **Калибровка телефона.** Положить на стол экраном вверх, подождать, пока
   заполнится полоска; перевернуть экраном вниз на то же место. Стол не обязан
   быть горизонтальным — наклон стола сюда не входит.
2. **Калибровка крепления.** Приложить телефон к планке на ободе экраном
   наружу, вертикально; затем перевернуть вверх ногами и приложить к тому же
   месту. Так вычитается перекос из-за выступа камеры и чехла.

После этого — прижимать телефон к планке экраном наружу. Кнопка «Биение диска»
делает то же, что два замера угломером с прокаткой на пол-оборота.

Точность — около 0,05–0,1°, если планка ровная и лежит на закраинах обода.
Один раз сверьте телефон с известным углом, прежде чем ему доверять.

---

## 6. Замер схождения

От струны до обода, **на высоте центра ступицы**, в двух точках: спереди
колеса и сзади, на одинаковом удалении от центра.

**Почему на высоте ступицы:** выше или ниже развал наклоняет обод, и его
наклон подмешивается в замер схождения. На высоте оси вращения этого эффекта нет.

**Как мерить точно:** удобно использовать закладной брусок известной толщины,
который упирается в обод, и мерить зазор от струны до бруска штангенциркулем.
Плавающие показания ±0,5 мм это ±0,1° — уже уровень заводского допуска.

**Знак.** Струна снаружи: если колесо «смотрит» внутрь (схождение
положительное), передний край обода уходит от струны, и переднее расстояние
**больше**. Так и вводите — программа разберётся.

---

## 7. Замер кастера (продольного наклона оси поворота)

Кастер нельзя измерить напрямую — он измеряется по тому, **как меняется развал
при повороте колеса**.

1. Поставьте передние колёса на поворотные круги.
2. Замерьте развал в положении «прямо» — это ваш обычный замер развала.
3. Поверните руль так, чтобы **левое** колесо повернулось на 20° **наружу**
   (руль влево). Замерьте развал обоих колёс. Для левого это «наружу», для
   правого — «внутрь». Записывайте соответственно.
4. Поверните руль в другую сторону на те же 20°. Снова замерьте оба колеса.
5. Введите четыре значения.

**Кастер телефоном.** Если телефон на колесе и у него есть гироскоп — кнопка
«Кастер» на телефоне. Педаль тормоза зафиксировать упором (колесо не должно
проворачиваться на круге). Телефон попросит поставить колёса прямо (в это время
он меряет собственный уход гироскопа), повернуть наружу примерно на 20°, затем
внутрь — угол поворота он измеряет сам, ровно 20° выдерживать не нужно:
программа решает задачу точно при любых двух углах.

**Про угол поворота.** 20° — классика и даёт лучшую точность. 10° допустимо,
если не хватает хода рейки или места, но погрешность вырастает примерно втрое.

**Что программа делает иначе.** Сервисные мануалы дают правило «кастер = размах
развала × 1,5». Это приближение первого порядка, и на машинах с большим кастером
(6–8°, то есть почти всё, что выпущено после 2000 года) оно систематически
врёт до **0,27°**. Программа решает задачу точно, через уравнение Родрига —
вывод и проверка в `internal/measure/sweep.go` и в тестах рядом.

**Про SAI.** Программа посчитает и поперечный наклон оси поворота, но честно
предупредит: этот угол получается делением на `(1 − cos 20°) = 0,06`, то есть
ошибка угломера в 0,1° превращается в **1,8° SAI**. Пользуйтесь им **только**
для сравнения левого борта с правым: заметная разница выдаёт погнутую стойку
или кулак. Как абсолютное значение он бесполезен.

---

## 8. Порядок регулировки — не переставляйте местами

Это то, на чём чаще всего спотыкаются самостоятельные попытки.

```
1. Задняя ось (развал, потом схождение)
        ↓  задаёт линию тяги, к которой привязано переднее схождение
2. Кастер
        ↓  его изменение двигает и развал, и схождение
3. Развал
        ↓  его изменение двигает схождение
4. Схождение — всегда последним
```

**Почему задняя ось первой.** Переднее схождение отсчитывается не от кузова, а
от **линии тяги** — направления, куда фактически едет задняя ось. Пока линия
тяги не выставлена, руль ровно не встанет никакими манипуляциями спереди.

**Почему схождение каждого колеса отдельно.** Суммарное схождение можно набрать
бесконечным числом способов, и только один из них оставляет руль прямым.
Классическая жалоба «сделали схождение, а руль кривой» — это ровно тот случай,
когда выставили сумму, а не каждое колесо.

Крутите **обе тяги на одинаковую величину в противоположные стороны**: тогда
суммарное схождение меняется, а положение руля — нет.

**Сколько крутить.** На экране регулировки нажмите на угол, затем «Запомнить
положение», поверните тягу на четверть оборота, дождитесь «стабильно» и нажмите
«¼». Программа узнает, сколько даёт оборот тяги на вашей машине, и дальше будет
показывать: «до номинала ≈ 1¾ оборота — в ту же сторону». Работает и для
эксцентриков, и для шайб (тогда «шаг» — одна шайба).

---

## 9. Контрольный замер

**Обязательно.**

1. Затяните контргайки рулевых тяг. Затяжка часто сдвигает схождение — это
   нормально и это надо учесть.
2. Прокатите машину 3–5 м вперёд, покачайте кузов.
3. Перемерьте всё заново.

Регулировка без контрольного замера — это не регулировка, а надежда.

---

## 10. Ожидаемая точность

| Параметр | Реально достижимо | Что ограничивает |
|---|---|---|
| Развал | ±0,05…0,1° | Угломер, ровность площадки |
| Схождение | ±0,05° | Провисание струны, точность отступов |
| Кастер | ±0,15° | Угломер (усиление ×1,5 от развала) |
| SAI | ±2° | Усиление ×17 — только для сравнения бортов |
| Развал телефоном | ±0,05…0,1° | Калибровка телефона, ровность планки |
| Кастер телефоном | ±0,1…0,15° | Развал телефона (усиление ×1,5) |
| Угол тяги | ±0,05° | То же, что схождение |

Для сравнения: типичный заводской допуск на развал — ±0,5°, на схождение —
±0,1°. То есть аккуратный ручной замер **укладывается в заводские требования**.

---

## 11. Когда регулировка не поможет

Программа сама поднимет эти флаги, но знать их полезно заранее:

- **Разная колёсная база слева и справа** (> 10 мм) — деформация кузова или
  рычагов.
- **Разный включённый угол** (SAI + развал) слева и справа (> 1°) — погнута
  стойка, кулак или рычаг. Именно этот показатель отличает «погнуто» от
  «разрегулировано»: развал и SAI по отдельности могут врать, а их сумма — нет.
- **Большой сдвиг колёс по продольной оси** (setback > 12 мм) — деформация
  подрамника или лонжерона.
- **Угол тяги не выставляется** на машине с неразрезной задней балкой —
  деформация балки или её креплений. Регулировки там нет.

---

## 12. Шкворневая подвеска, балки, грузовики и автобусы

**Шкворневая двухрычажная** (ГАЗ-21, 24, 3102, 3110 в шкворневом исполнении).
Регулировка у неё есть — прокладками или эксцентриками, как у двухрычажной на
шаровых опорах. Отказываются её делать обычно из-за другого: изношенный шкворень
или его втулки дают люфт, и развал «гуляет». Поэтому сначала:

- вывесить колесо и покачать за верх и низ при **нажатой педали тормоза** — так
  исключается подшипник ступицы, и весь люфт приходится на шкворень и шарниры;
- проверить осевой зазор кулака на шкворне (стук на неровностях);
- проверить резьбовые шарниры стоек и прошприцевать их вместе со шкворнями.

Есть люфт — сначала ремонт, потом регулировка. Кастер у такой подвески
называется продольным наклоном шкворня, SAI — поперечным. Для ГАЗ-3110 в
программе есть цифры из открытого пересказа руководства (постранично не
сверены): кастер +4°30′…+6° без нагрузки, развал 0°±30′, схождение 0,7–1,3 мм по
ободьям.

**Неразрезная балка на шкворнях** (ГАЗель, ГАЗ-53, ЗИЛ, КамАЗ, ПАЗ, УАЗ):

- развал и поперечный наклон шкворня **не регулируются** — задаются балкой;
  отклонение означает погнутую балку, изношенные шкворни или подшипники;
- кастер исправляют клиновыми прокладками между рессорой и площадкой балки;
- поперечная рулевая тяга задаёт только **суммарное** схождение; руль «прямо»
  выставляют длиной продольной тяги;
- сдвоенные задние колёса — одно колесо, мерьте по наружному; давление в обеих
  шинах одинаковое.

Многоосные машины (задняя тележка) программа пока считает как двухосные:
передняя ось и один задний мост.

---

## 13. Безопасность

- Работайте **только на надёжных опорах**. Домкрат — подъёмное устройство, а не
  опорное.
- Тормоза, рулевое и подвеска — узлы, отказ которых убивает. Если что-то
  разобрали и не уверены в сборке, найдите того, кто уверен.
- После регулировки первую поездку сделайте на малой скорости на пустой дороге.
  Проверьте, что руль стоит прямо и машина не уводит.
- Если поведение машины изменилось непонятным образом — остановитесь и
  проверьте всё заново.

---
---

<a id="eng"></a>

[Русский](#ru) · **English**

# DIY wheel alignment: the full procedure

This document is a working manual. It describes a method that gives an accuracy
**better than 0.1°** in camber and toe with careful work, on equipment costing
about as much as one visit to a workshop.

The program calculates the angles. This document answers where to get the
numbers you enter into it. For using the program itself, screen by screen, see
[GUIDE.md](GUIDE.md#eng).

---

## 1. Tools

### Essential

| What | What for | Substitute |
|---|---|---|
| **String** — fishing line 0.3–0.5 mm, 2 pieces of 5 m | Reference line for toe | Just not thread: it sags and stretches |
| **String stands** — 4, up to hub-centre height | Hold the string | Jacks, bricks + blocks, chairs, buckets of sand |
| **Tension weight** — 2–4 kg per string | Removes sag | Dumbbells, bottles of water |
| **Tape measure** 5 m + **calipers** or a **steel rule** | Measuring distances | — |
| **Inclinometer** — magnetic, digital, or a smartphone | Camber | Spirit level with a protractor |
| **Spanners** for the tie-rod lock nuts | Adjusting toe | — |
| **Stands** | Safety when working under the car | **Never a jack instead. Never.** |

### For caster (optional, but highly recommended)

| What | What for | How to make it yourself |
|---|---|---|
| **Turn plates**, 2 | Let the wheel steer without binding | Two 30×30 cm sheets of tin with grease or graphite between them. It works |
| **Protractor** on the plate | Measuring 20° of steer | Print a scale and glue it to the lower sheet |

### For checking the floor

A **water level** (a clear 10 m hose filled with water) or a **long straightedge**
+ an ordinary level. The most underrated item on the list — see section 2.

---

## 2. The floor: why it matters most

The program needs all four wheels to stand **in one plane**. Not “level” —
in one plane.

Why it is critical: if one wheel stands in a 5 mm dip on a 1400 mm track, the
body tilts by `atan(5/1400)` ≈ **0.2°**. The camber on that side is off by
exactly that much — and 0.2° is half the factory tolerance on many cars.

**How to check.** With the water level, mark the water height at the four points
where the wheels stand. Any two points must differ by **less than 2 mm**. If more,
put steel plates of the right thickness under the wheels (not wood: it crushes
under the weight).

**If there is no flat floor at all**, the program can measure from the road plane
instead of gravity, but that needs an optical measurement (see
[OPTICAL.md](OPTICAL.md#eng)). For the inclinometer and the phone a flat floor
is a must: both measure tilt relative to gravity.

---

## 3. Preparing the car

In exactly this order:

1. **Tyre pressures** — per the factory placard, and **equal left and right**.
   A 0.3 bar difference means up to 0.1° of camber difference.
2. **Play.** Rock each wheel by hand — top to bottom (ball joints, wheel bearing)
   and side to side (tie-rod ends). Any noticeable play means **adjusting is
   pointless**: you set one thing standing still and get another on the move.
   Repair first.
3. **Arm bushings.** Cracked or squashed — the same story.
4. **Load.** Bring the car to the state the specification requires: usually a
   full tank, the spare wheel and tools in place, no passengers. Some cars need
   weight on the seats (VAZ cars — 320 kg: four people and 40 kg in the boot) —
   the program shows the conditions in the “Measuring conditions” block.
5. **Let the suspension settle.** Bounce the body and **roll the car 3–5 m
   forward**. Do not push it sideways or roll it back — the suspension must take
   the position it has on the move.
6. **Steering wheel dead straight.** Lock it (a stick against the seat, or a
   strap). Check by the mark on the steering shaft, not “by eye on the spokes”:
   the wheel may have been refitted crooked at a previous repair.

---

## 4. Setting up the strings

The strings run **along both sides, outside the wheels, at hub-centre height**,
parallel to the car's centreline.

### Why “equal distances” is a mistake

The folk advice “make the distance from the string to the hub equal at the
front and rear” **holds only if the front and rear track widths are equal**. On
most cars they are not.

With a front track `Tf` and a rear track `Tr`, for a string parallel to the
car's centreline the distances must differ by:

```
distance_front − distance_rear = (Tr − Tf) / 2
```

Example: VAZ-2101, Tf = 1349 mm, Tr = 1305 mm. Difference = (1305 − 1349)/2 =
**−22 mm**. That is, at the front the string must be 22 mm **closer** to the hub
than at the rear. Make them equal and you get a toe error of about 0.5° per side.

**The program calculates this for you.** Enter the track widths and the four
distances in the “String setup check” block — it tells you how many millimetres
to move each end, and which way.

### In practice

- The string must be **tensioned by a weight**, not tied. 1 mm of sag is a 1 mm
  measuring error.
- Measure the distance **to the hub centre** (the centre of the hub cap or of a
  bolt). It is the one point whose position does not depend on camber and toe.
- The string must not touch the tyre anywhere.

---

## 5. Measuring camber

A magnetic inclinometer goes **on the rim, not on the rubber**. If the rim is
cast and uneven, hold a straight bar against it and measure on the bar.

### Rim runout compensation — always do it

No rim sits perfectly square to the axle. A 0.3° mounting error eats the whole
factory tolerance.

1. Measure camber and write it down. This is the “0°” value.
2. **Roll the car so that the wheel turns exactly half a revolution.** Mark a
   point on the rim with chalk to be sure.
3. Put the inclinometer **on the very same spot of the rim**. Write it down. This
   is “180°”.
4. Enter both values.

The program averages them (the true camber) and shows the difference (rim
runout). This works because turning 180° reverses the sign of the rim's wobble,
while camber itself stays.

If the program shows a runout above 0.5°, the rim is bent or dirty, and **every
measurement on that wheel is unreliable**.

### Camber with a phone

Instead of an inclinometer you can use a smartphone — it sends camber to the
screen in real time. In the program: “Measurement → Phone on the wheel → Allow
Wi-Fi connection”, scan the QR code with the phone, choose the wheel.

An uncalibrated phone is off by 0.5–2°: its accelerometer's zero offset exceeds
any factory tolerance. The calibration is done **once** and remembered:

1. **Phone calibration.** Lay it on a table screen up and wait for the bar to
   fill; turn it over screen down on the same spot. The table need not be level —
   its tilt drops out.
2. **Mount calibration.** Hold the phone against the bar on the rim, screen out,
   upright; then turn it upside down and hold it to the same spot. This removes
   the tilt from the camera bump and the case.

After that, press the phone against the bar with the screen facing out. The “Rim
runout” button does the same as two inclinometer readings with half a turn of
rolling.

Accuracy is about 0.05–0.1° if the bar is straight and rests on the rim flanges.
Check the phone once against a known angle before trusting it.

---

## 6. Measuring toe

From the string to the rim, **at hub-centre height**, at two points: in front of
the wheel and behind it, equally far from the centre.

**Why at hub height:** higher or lower, camber tilts the rim and its tilt mixes
into the toe reading. At the height of the axis of rotation there is no such
effect.

**How to measure precisely:** a spacer block of known thickness resting against
the rim is handy — measure the gap from the string to the block with calipers.
Readings wandering by ±0.5 mm are ±0.1° — already the size of a factory tolerance.

**Sign.** With the string outside: if the wheel points inwards (positive toe),
the front edge of the rim moves away from the string, and the front distance is
**larger**. Enter it as it is — the program sorts it out.

---

## 7. Measuring caster (the fore–aft tilt of the steering axis)

Caster cannot be measured directly — it is measured from **how camber changes
while the wheel is steered**.

1. Put the front wheels on turn plates.
2. Measure camber straight ahead — your ordinary camber reading.
3. Turn the steering so that the **left** wheel steers 20° **out** (steering to
   the left). Measure the camber of both wheels. For the left one this is “out”,
   for the right one “in”. Write them down accordingly.
4. Turn the steering the other way by the same 20°. Measure both wheels again.
5. Enter the four values.

**Caster with a phone.** If the phone is on the wheel and has a gyroscope — the
“Caster” button on the phone. Lock the brake pedal with a prop (the wheel must not
roll on the plate). The phone asks you to set the wheels straight (meanwhile it
measures its own gyro drift), steer out about 20°, then in — it measures the steer
angle itself, there is no need to hold exactly 20°: the program solves the problem
exactly for any two angles.

**About the steer angle.** 20° is the classic and gives the best accuracy. 10° is
acceptable if the rack travel or space is short, but the error grows about
threefold.

**What the program does differently.** Service manuals give the rule “caster =
camber swing × 1.5”. That is a first-order approximation, and on cars with large
caster (6–8°, i.e. nearly everything built after 2000) it is systematically off by
up to **0.27°**. The program solves the problem exactly, with Rodrigues' formula —
derivation and proof in `internal/measure/sweep.go` and the tests beside it.

**About SAI.** The program also computes the steering axis inclination but warns
honestly: this angle comes from dividing by `(1 − cos 20°) = 0.06`, so an
inclinometer error of 0.1° becomes **1.8° of SAI**. Use it **only** to compare
the left side with the right: a noticeable difference points to a bent strut or
knuckle. As an absolute value it is useless.

---

## 8. Adjustment order — do not change it

This is where DIY attempts most often stumble.

```
1. Rear axle (camber, then toe)
        ↓  sets the thrust line to which front toe is referenced
2. Caster
        ↓  changing it moves both camber and toe
3. Camber
        ↓  changing it moves toe
4. Toe — always last
```

**Why the rear axle first.** Front toe is measured not from the body but from the
**thrust line** — where the rear axle actually points. Until the thrust line is
set, no amount of work at the front makes the steering wheel straight.

**Why each wheel's toe separately.** Total toe can be reached in endless ways, and
only one of them leaves the steering wheel straight. The classic complaint “they
did the toe and the steering wheel is crooked” is exactly the case where the total
was set, not each wheel.

Turn **both tie rods by the same amount in opposite directions**: the total toe
changes, the steering wheel position does not.

**How far to turn.** On the adjustment screen tap the angle, then “Remember
position”, turn the tie rod a quarter turn, wait for “steady” and press “¼”. The
program learns what one tie-rod turn does on your car and from then on shows: “to
nominal ≈ 1¾ turns — the same way”. It works for eccentrics and shims too (a
“step” is then one shim).

---

## 9. The check measurement

**Mandatory.**

1. Tighten the tie-rod lock nuts. Tightening often shifts toe — that is normal and
   must be allowed for.
2. Roll the car 3–5 m forward, bounce the body.
3. Measure everything again.

An alignment without a check measurement is not an alignment, it is a hope.

---

## 10. Expected accuracy

| Parameter | Realistically achievable | Limited by |
|---|---|---|
| Camber | ±0.05…0.1° | Inclinometer, flatness of the floor |
| Toe | ±0.05° | String sag, accuracy of the distances |
| Caster | ±0.15° | Inclinometer (×1.5 gain from camber) |
| SAI | ±2° | ×17 gain — for side-to-side comparison only |
| Camber by phone | ±0.05…0.1° | Phone calibration, straightness of the bar |
| Caster by phone | ±0.1…0.15° | Phone camber (×1.5 gain) |
| Thrust angle | ±0.05° | The same as toe |

For comparison: a typical factory tolerance is ±0.5° for camber and ±0.1° for toe.
So a careful manual measurement **meets factory requirements**.

---

## 11. When adjustment will not help

The program raises these flags itself, but it is good to know them in advance:

- **Different wheelbase left and right** (> 10 mm) — a deformed body or arms.
- **Different included angle** (SAI + camber) left and right (> 1°) — a bent
  strut, knuckle or arm. This is what tells “bent” from “misaligned”: camber and
  SAI may each lie, their sum does not.
- **Large setback** along the car (> 12 mm) — a deformed subframe or chassis rail.
- **The thrust angle cannot be set** on a car with a rigid rear beam — the beam or
  its mountings are deformed. There is no adjustment there.

---

## 12. Kingpin suspensions, beams, trucks and buses

**Double wishbone with kingpins** (GAZ-21, 24, 3102, 3110 in the kingpin
version). It does have adjustment — shims or eccentrics, as with ball joints.
Workshops usually refuse it for another reason: a worn kingpin or its bushes give
play, and camber “wanders”. So first:

- lift the wheel and rock it by the top and bottom with the **brake pedal pressed**
  — this rules out the wheel bearing, and all the play is in the kingpin and joints;
- check the knuckle's end float on the kingpin (knocking over bumps);
- check the threaded strut bushings and grease them together with the kingpins.

If there is play — repair first, then align. On such a suspension caster is called
the kingpin's fore–aft inclination, SAI — its sideways inclination. For the
GAZ-3110 the program carries the figures from an open retelling of the manual (not
checked page by page): caster +4°30′…+6° unladen, camber 0°±30′, toe 0.7–1.3 mm at
the rims.

**Rigid kingpin beam** (GAZelle, GAZ-53, ZIL, KAMAZ, PAZ, UAZ):

- camber and kingpin inclination are **not adjustable** — the beam sets them; a
  deviation means a bent beam, worn kingpins or bearings;
- caster is corrected with taper shims between the leaf spring and the beam pad;
- the cross tie rod sets only the **total** toe; the steering wheel is centred
  with the length of the drag link;
- twin rear wheels count as one wheel — measure the outer one; both tyres at the
  same pressure.

Multi-axle vehicles (a rear bogie) are treated as two-axle for now: the front axle
and one rear axle.

---

## 13. Safety

- Work **only on sound stands**. A jack is a lifting device, not a support.
- Brakes, steering and suspension are parts whose failure kills. If you took
  something apart and are not sure of the assembly, find someone who is.
- After adjustment, make the first drive at low speed on an empty road. Check that
  the steering wheel is straight and the car does not pull.
- If the car's behaviour has changed in a way you do not understand, stop and
  check everything again.
