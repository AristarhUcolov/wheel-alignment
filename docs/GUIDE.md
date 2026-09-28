<a id="ru"></a>

**Русский** · [English](#eng)

# Сход-развал — инструкция

Сход-развал — это не магия и не секрет мастерских. Это четыре колеса, несколько углов и порядок, в котором их выставлять. Здесь — всё, что нужно, чтобы сделать это самому и правильно: от установки программы до протокола.

Та же инструкция есть в программе — клавиша **F1**. Подробности о методе и точности — в [PROCEDURE.md](PROCEDURE.md).

**Содержание:** [Коротко: пять шагов](#ru-start) · [Установка и первый запуск](#ru-install) · [Углы простыми словами](#ru-angles) · [Шаг 1. Автомобиль](#ru-vehicle) · [Шаг 2. Подготовка](#ru-prep) · [Инструмент и площадка](#ru-tools) · [Замер: струна и угломер](#ru-string) · [Замер: телефон на колесе](#ru-phone) · [Замер: камера и мишени](#ru-optical) · [Замер: свой датчик](#ru-sensor) · [Шаг 4. Экран регулировки](#ru-live) · [Порядок регулировки — не переставляйте](#ru-order) · [Шаг 5. Отчёт](#ru-report) · [Типы подвески](#ru-susp) · [Грузовики, автобусы, фургоны](#ru-trucks) · [Когда регулировка не поможет](#ru-bent) · [Откуда берутся допуски](#ru-data) · [Если что-то не работает](#ru-trouble) · [Безопасность](#ru-safety) · [Клавиши](#ru-keys)

<a id="ru-start"></a>

## Коротко: пять шагов

1. **Автомобиль.** Найдите машину в базе или возьмите ориентир по классу. Проверьте тип передней и задней подвески — от него зависят проверки и советы по регулировке.
2. **Подготовка.** Чек-лист под вашу подвеску: давление, люфты, загрузка, площадка. Изношенную подвеску регулировать бесполезно — углы «уплывут» при первой же кочке.
3. **Замер.** Выберите, чем мерить: струна и угломер, телефон, камера или свой датчик. Способы можно сочетать — например, развал и кастер телефоном, схождение струной.
4. **Регулировка.** Экран как у стенда: крупные цифры, зелёные — в допуске, красные — нет; над каждой — шкала допуска со стрелкой. Нажмите на угол — справа появится, чем он регулируется на вашей машине. Перед первым поворотом ключа сделайте снимок «до» (F5).
5. **Отчёт.** Когда всё зелёное — снимок «после» (F5) и протокол с таблицей «до / после» (F6). Его можно распечатать или сохранить в PDF.

> Новичку: запустите демонстрацию (F9 на экране регулировки) и нажмите F10 — программа сама покажет, в каком порядке и как выставляются углы. Стрелками ← → можно «покрутить» любой угол самому и посмотреть, что при этом происходит с соседними.

<a id="ru-install"></a>

## Установка и первый запуск

1. **Скачайте** `wheelalign.exe` со страницы релизов проекта на GitHub. Установка не нужна: это один файл, его можно держать хоть на флешке.
2. **Запустите.** Откроется окно программы. Оно работает на движке WebView2, который встроен в Windows 10 и 11. Если окно не появилось, программа откроется в обычном браузере и подскажет, что установить.
3. **Язык** — переключатель RU / EN в правом верхнем углу. При первом запуске выбирается язык системы; ваш выбор запоминается.
4. **Интернет не нужен.** Всё работает на этом компьютере. Сеть нужна только телефону-датчику, и то локальная (Wi-Fi), а не интернет.

Где лежат ваши данные: `%APPDATA%\wheelalign` (в Linux — `~/.config/wheelalign`). Там настройки (`settings.json`), ваши допуски (папка `vehicles`), сертификат для телефона (`tls`) и журнал работы `wheelalign.log` — его стоит приложить к сообщению об ошибке.

Ключи запуска: `-browser` — открыть в браузере вместо окна; `-data <папка>` — хранить данные в другом месте (например, рядом с программой на флешке). Полный список — `wheelalign -h`.

<a id="ru-angles"></a>

## Углы простыми словами

| Угол | Что это | Если не в норме |
|---|---|---|
| **Развал** | Наклон колеса поперёк машины, если смотреть спереди. «Плюс» — верх колеса наружу. | Изнашивается одна сторона протектора (внешняя при «плюсе», внутренняя при «минусе»). Разница развала слева и справа больше ~0,5° — машину тянет в сторону большего развала. |
| **Схождение** | Поворот колеса в плане, если смотреть сверху. «Плюс» — передний край колеса ближе к оси машины (колёса «сходятся»). | Самый «дорогой» угол: резина стирается «пилой» за несколько тысяч километров. Кривой руль при прямом ходе — схождение колёс разделено неправильно. |
| **Кастер** | Продольный наклон оси поворота (у «шкворневых» машин — шкворня). «Плюс» — верх оси назад. | Руль плохо возвращается, машина «рыщет» на скорости. Разница слева и справа — увод в сторону меньшего кастера. |
| **Поперечный наклон оси (SAI)** | Наклон оси поворота поперёк машины. Задан конструкцией. | Не регулируется. Разница между бортами — признак погнутой стойки, кулака или балки. |
| **Включённый угол** | SAI + развал. | Разный слева и справа больше чем на 1° — деталь погнута; регулировкой развала это не лечится. |
| **Угол тяги** | Куда направлена задняя ось относительно оси симметрии машины: половина разности схождения задних колёс. | Машина едет «боком», руль при прямом ходе стоит криво, хотя переднее схождение в норме. |

<a id="ru-vehicle"></a>

## Шаг 1. Автомобиль

- **Поиск** — по марке и модели, по-русски или латиницей: «Волга 3110», «ГАЗель», «2107», «Hilux». Фильтры сверху сужают список до легковых, внедорожников, фургонов, грузовых или автобусов; поле «Год» отсекает другие поколения.
- У каждой записи есть **метка источника**: заводское руководство, сообщество, «не проверено», «только конструкция». Чем метка слабее, тем внимательнее относитесь к цифрам — подробно в разделе «Откуда берутся допуски».
- **Модели нет в базе?** Выберите ориентир по классу («легковой с McPherson», «грузовик с балкой на шкворнях»…). Это типичные значения для конструкции, а не для вашей модели: программа покажет, насколько углы далеки от типичных, и будет честно предупреждать об этом на всех экранах.
- **Подвеска.** Программа подставляет тип подвески из базы; если его нет — выберите сами. Кнопка «Как узнать, какая у меня?» подскажет, по каким признакам отличить шкворень от шаровой опоры, McPherson от двух рычагов.
- **Размеры.** Диаметр обода нужен, чтобы перевести схождение из градусов в миллиметры (так пишут во многих руководствах). Колея передней и задней оси — для проверки установки струн.
- **Свои допуски.** Если у вас есть руководство по ремонту — кнопка «Внести допуски из руководства». Программа проверит цифры на опечатки и будет пользоваться ими на этом компьютере.

Можно работать и без выбора автомобиля — тогда углы показываются без оценки «в норме / не в норме».

![Выбор автомобиля](img/vehicle.png)

<a id="ru-prep"></a>

## Шаг 2. Подготовка

Чек-лист собран под выбранную подвеску: для шкворневой балки — люфт шкворней, для McPherson — опорные подшипники стоек, для рессор — стремянки и центральный болт. Отмечайте пункты по мере проверки — отметки сохраняются.

- **Давление в шинах** — по табличке на стойке двери, на холодных шинах. Разница 0,3 бар между бортами уже меняет развал.
- **Люфты** — рулевые наконечники, шаровые опоры или шкворни, подшипники ступиц, сайлентблоки. Если колесо качается рукой — сначала ремонт.
- **Загрузка** — как требует руководство: обычно полный бак, запаска и инструмент на месте, в салоне никого. У грузовиков и автобусов — указанная нагрузка на ось.
- **Руль прямо.** Перед замером схождения поставьте руль ровно и зафиксируйте (упор между рулём и сиденьем).
- **Подвеска «осела».** После подъёма машины прокатите её вперёд-назад на 1–2 метра и покачайте кузов — рычаги встанут в рабочее положение.

<a id="ru-tools"></a>

## Инструмент и площадка

| Что | Зачем | Чем заменить |
|---|---|---|
| Леска 0,3–0,5 мм, 2 × 5 м | Опорные линии для схождения | Только не нитка — провисает и тянется |
| 4 стойки до высоты ступиц | Держат струны | Кирпичи с брусками, вёдра с песком |
| Грузы по 2–4 кг | Натягивают струны без провисания | Бутыли с водой |
| Рулетка, штангенциркуль или линейка | Отступы до обода | — |
| Угломер или смартфон | Развал | Пузырьковый уровень с транспортиром |
| Ровная планка 40–50 см | Прикладывать угломер или телефон к закраинам обода | Алюминиевое правило |
| Поворотные круги | Кастер | Два листа жести со смазкой между ними |
| Подставки под машину | Безопасность | **Не заменять домкратом. Никогда.** |

**Площадка важнее всего.** Нужна не «горизонтальная», а *плоская*: все четыре колеса в одной плоскости. Ямка 5 мм под одним колесом при колее 1400 мм — это 0,2° ошибки развала, половина заводского допуска. Проверьте гидроуровнем (прозрачный шланг с водой): разница между пятнами контакта — меньше 2 мм. Подкладывайте стальные пластины, не деревяшки. Поворотные круги и подкладки под задние колёса должны быть одной высоты.

<a id="ru-string"></a>

## Замер: струна и угломер

1. **Струны** — вдоль бортов, снаружи колёс, на высоте центра ступиц, натянуты грузом. Отступ от струны до центра ступицы спереди и сзади **не обязан быть одинаковым**: при разной колее передней и задней оси разница должна быть (колея зад − колея перед) / 2. Введите четыре отступа в «Проверку установки струн» — программа скажет, какой конец и на сколько сдвинуть.
2. **Развал** — угломер на планку, прижатую к закраинам обода (не к резине). «Плюс» — верх колеса наружу. Если ваш угломер показывает наоборот, отметьте это галочкой. Затем прокатите машину на пол-оборота колеса (метка мелом на шине) и приложите угломер к тому же месту обода. Программа усреднит два числа и уберёт биение диска; биение больше 0,5° — диск погнут, замеры этого колеса ненадёжны.
3. **Схождение** — от струны до закраины обода спереди и сзади колеса, строго на высоте ступицы. Выше или ниже — в замер подмешивается развал. Колесо смотрит внутрь — переднее расстояние больше; так и вводите.
4. **Кастер** — на поворотных кругах: развал при повороте колеса на 20° наружу и на 20° внутрь. Программа решает задачу точно, а не по правилу «размах × 1,5», которое на машинах с большим кастером ошибается до 0,27°. Угол поворота считывайте со шкалы кругов или отметьте на полу мелом.

Каждое введённое число сразу уходит на экран регулировки. Во время работы: повернули тягу — перемерили — ввели новое число. Экран обновится сразу.

<a id="ru-phone"></a>

## Замер: телефон на колесе

1. На вкладке «Замер → Телефон на колесе» нажмите «Разрешить подключение по Wi-Fi» и отсканируйте QR-код камерой телефона. Телефон и компьютер должны быть в одной сети Wi-Fi (подойдёт точка доступа на самом телефоне).
2. Браузер телефона предупредит о сертификате — он создан этой программой, а не удостоверяющим центром. «Дополнительно» → «Перейти на сайт». Windows может спросить разрешение для брандмауэра — разрешите в частных сетях.
3. Выберите на телефоне колесо и нажмите «Включить датчики». На iPhone Safari спросит доступ к датчикам движения — разрешите.
4. **Калибровка телефона** (один раз): положить на стол экраном вверх, затем экраном вниз. Так вычитается собственная ошибка акселерометра — у телефонов она бывает до 1–2°.
5. **Калибровка крепления** (один раз для телефона и планки): приложить телефон к планке на ободе, затем перевернуть его вверх ногами и приложить к тому же месту. Так вычитается перекос из-за выступа камеры и чехла.
6. **Биение диска** (по желанию): замер в одном положении колеса, затем прокатить машину на пол-оборота и замерить снова — программа уберёт биение из всех следующих показаний.
7. Прижмите телефон к планке экраном наружу — развал идёт на экран регулировки в реальном времени. Метка «стабильно» на телефоне означает, что можно записывать.
8. **Кастер:** колёса на поворотных кругах, кнопка «Кастер» на телефоне. Телефон попросит поставить колесо прямо, повернуть наружу на ~20°, затем внутрь — угол поворота он меряет гироскопом сам.

Приложенный к ободу телефон схождение не меряет: схождение — это поворот колеса вокруг вертикали, а акселерометр чувствует только наклон, сила тяжести при таком повороте не меняется. Гироскоп поворот чувствует, но копит ошибку — за минуту больше, чем весь допуск на схождение. Поэтому схождение берите струной или камерой: тот же телефон, поставленный на штатив, работает камерой в живом режиме (раздел «Камера и мишени»). Два телефона на колёсах — два колеса сразу, четыре — все четыре. Когда закончите, выключите доступ по Wi-Fi. Телефон, подключённый по ошибке, отключается кнопкой «Отключить» в списке.

<img src="img/phone.png" width="220" alt="Страница на телефоне">

<a id="ru-optical"></a>

## Замер: камера и мишени

Печатная шахматная доска на жёстком листе крепится к колесу как угодно криво — программа находит ось вращения колеса по серии снимков, и перекос крепления уходит.

1. **Калибровка камеры** (один раз для каждой камеры и зума): 10–20 снимков шахматной доски под разными углами и во всех углах кадра. Размер клетки измерьте штангенциркулем по распечатке — принтеры масштабируют. Сохраните `camera.json`.
2. **Развал по фото:** вывешенное колесо, мишень на диске, камера строго по уровню сбоку. 4–6 снимков, проворачивая колесо на 10–20° между кадрами.
3. **Полный замер:** в каждый кадр вместе с мишенью на колесе должна попадать напольная мишень — она задаёт плоскость дороги и связывает четыре колеса в одну систему координат. Даёт развал, схождение каждого колеса и угол тяги. Мишеней на полу две (спереди и сзади), разного размера; их связывают снимки, где видны обе.

**Живой режим** (вкладка «4. Живой режим») — как на профессиональном 3D-стенде: телефон на штативе смотрит на колесо и мишень на полу, и развал со схождением меняются на экране регулировки 3–4 раза в секунду, пока вы крутите тягу. На телефоне — кнопка «Телефон как камера»:

1. **Печать мишеней** — на вкладке «4. Живой режим», кнопки «PDF: мишени колёс» и «PDF: напольные мишени». Печатайте в масштабе 100 % и проверьте линейку 100 мм внизу листа. Наклейте мишени на жёсткий ровный лист (фанера, ДСП, композит). На каждом колесе своя мишень разного размера — программа сама узнаёт, какое колесо видит, выбирать его на телефоне не нужно. Напольные мишени большие: их удобнее заказать одним листом в типографии.
2. **Калибровка камеры** — один раз для телефона: показывайте ему мишень с колеса под разными углами и в разных частях кадра, пока полоска не заполнится. Телефон держите горизонтально — и так же потом при замере.
3. **Биение** — для каждого колеса: вывесите его и медленно проверните рукой на четверть оборота и больше. Программа запоминает, как ось колеса стоит относительно мишени, и дальше ей хватает одного кадра.
4. **Замер** — машина на полу. Покажите камере по очереди все четыре колеса (и два-три кадра, где видны обе мишени на полу), после этого появится схождение. Дальше ставьте телефон у колеса, которое регулируете.
5. **Кастер камерой** — передние колёса на поворотных кругах, педаль тормоза зажата упором, чтобы колесо не проворачивалось. Режим «Кастер камерой» на телефоне, медленно поверните руль на 15–20° в одну сторону, затем в другую. Программа найдёт ось поворота колеса и покажет кастер и поперечный наклон оси.

Подробности, размеры мишеней и советы по съёмке — в файле OPTICAL.md в репозитории.

<a id="ru-sensor"></a>

## Замер: свой датчик

Любое устройство в вашей сети — самодельная голова на ESP32, лазерный датчик, другая программа — может присылать углы программе обычным HTTP-запросом с JSON. Формат описан на вкладке «Замер → Свой датчик» и в файле SENSORS.md. Программа сама сглаживает показания, определяет, когда они успокоились, и показывает датчик в строке состояния сверху.

<a id="ru-live"></a>

## Шаг 4. Экран регулировки

- **Цифры.** Зелёная — в допуске, жёлтая — в допуске, но у границы, красная — вне допуска, белая — допуск не задан, серая — показаний нет. Над цифрой — шкала допуска: зелёная зона, стрелка — текущее значение.
- **Виды.** F2 переключает общий вид и крупные виды передней и задней оси; F3 и F4 — сразу нужная ось. Для работы под машиной удобнее крупный вид: цифры видно с двух-трёх метров.
- **Единицы** — градусы с минутами, десятичные градусы; схождение можно показывать в миллиметрах (по диаметру обода с шага «Автомобиль»).
- **«Показания стабильны».** Пока значения меняются, внизу написано «показания меняются…». Записывать и затягивать контргайки — только когда стабильно.
- **Нажмите на любой угол** — справа откроется панель: что это за угол, чем он регулируется на вашей подвеске, что написано для этой модели.
- **Помощник «сколько крутить».** В той же панели: нажмите «Запомнить положение», поверните тягу или эксцентрик на известную величину (¼, ½, 1 оборот) и нажмите, на сколько повернули. Программа посчитает, сколько даёт один оборот на вашей машине, и дальше будет подсказывать: «до номинала ≈ 1¼ оборота, в ту же сторону».
- **Строка источников** вверху показывает, откуда идут данные: телефон, датчик, камера, ручной ввод, демо.

![Экран регулировки](img/live-overview.png)

<a id="ru-order"></a>

## Порядок регулировки — не переставляйте

1. **Задняя ось** (развал, потом схождение) — задаёт линию тяги, от которой отсчитывается переднее схождение.
2. **Кастер** — его изменение двигает и развал, и схождение.
3. **Развал** — двигает схождение.
4. **Схождение — последним.** Руль строго прямо и зафиксирован. Выставляйте схождение *каждого* колеса, а не только сумму: сумму можно набрать бесконечным числом способов, и лишь один оставляет руль прямым. Обе тяги крутите на одинаковую величину в противоположные стороны.
5. **Контрольный замер.** Затяните контргайки (затяжка часто сдвигает схождение), прокатите машину 3–5 м, покачайте кузов и перемерьте. Регулировка без контрольного замера — это не регулировка, а надежда.

**Куда крутить тягу, если не знаете:** четверть оборота — и смотрите на стрелку. Живой экран отвечает на этот вопрос быстрее любой схемы.

<a id="ru-report"></a>

## Шаг 5. Отчёт

Протокол строится из двух снимков: «до» — перед регулировкой, «после» — когда всё выставлено. F5 на экране регулировки делает снимок в один нажим (сначала «до», затем «после»). Для снимка нужны развал и схождение всех четырёх колёс.

В протокол можно вписать госномер или VIN, пробег и имя. «Печать / PDF» открывает системный диалог печати — выберите «Сохранить как PDF», чтобы получить файл. В протоколе указан источник допусков, порядок регулировки и замечания (биение дисков, разница включённых углов и т. п.).

<a id="ru-susp"></a>

## Типы подвески

Заводские допуски у каждой модели свои, а конструкций подвески в мире около дюжины. Тип подвески определяет, что вообще можно отрегулировать, чем, и что нужно проверить до регулировки.

| Значение в программе | Подвеска | Ось |
|---|---|---|
| Макферсон | стойка, совмещённая с амортизатором | любая |
| Двухрычажная на шаровых опорах | два поперечных рычага, кулак на шаровых опорах | любая |
| Двухрычажная шкворневая | ГАЗ-21, 24, 3102, 3110: кулак на шкворне | передняя |
| Неразрезная балка на шкворнях | грузовики, автобусы, ГАЗель, УАЗ | передняя |
| Неразрезной мост на шаровых опорах | мост без шкворней | передняя |
| Многорычажная | несколько рычагов на колесо | любая |
| Полузависимая балка | балка, скручивающаяся между продольными рычагами | задняя |
| Мост на рессорах | неразрезной мост на листовых рессорах | задняя |
| Мост на пружинах и тягах | неразрезной мост, пружины или пневмобаллоны, реактивные тяги | задняя |
| Продольные рычаги | независимая на продольных рычагах | задняя |
| Косые рычаги | независимая на косых рычагах | задняя |

Для каждой конструкции программа показывает, как узнать её на своей машине, что обычно регулируется и чем, и что
проверить до регулировки: **F1 → «Типы подвески»**.

<a id="ru-trucks"></a>

## Грузовики, автобусы, фургоны

- Передняя балка на шкворнях: **развал и поперечный наклон шкворня не регулируются** — отклонение означает погнутую балку, изношенные шкворни или подшипники.
- **Кастер** исправляют клиновыми прокладками между рессорой и площадкой балки — одинаковыми с обеих сторон.
- **Схождение** задаёт поперечная рулевая тяга — только суммарное. Руль «прямо» выставляют длиной продольной тяги (сошка — поворотный рычаг).
- Сдвоенные задние колёса считаются одним колесом — мерьте по наружному; давление в обеих шинах одинаковое.
- Схождение в руководствах грузовиков часто дано в мм по шине или по ободу — укажите в программе тот диаметр, к которому оно отнесено.
- Многоосные машины (задняя тележка) программа пока считает как двухосные: передняя ось и один задний мост. Параллельность мостов тележки проверяйте отдельно.

<a id="ru-bent"></a>

## Когда регулировка не поможет

- **Разный включённый угол** (поперечный наклон оси + развал) слева и справа, больше 1° — погнута стойка, кулак или рычаг. Развал и наклон оси по отдельности могут «врать», их сумма — нет.
- **Разная колёсная база** слева и справа (больше 10 мм) — деформация кузова, рамы или рычагов.
- **Большой сдвиг колёс** вдоль машины (больше 12 мм) — деформация подрамника или лонжерона.
- **Угол тяги** на неразрезном мосту или балке — регулировки нет: срезанный центральный болт рессоры, ослабшие стремянки, погнутая балка.
- **Развал «гуляет»** от замера к замеру — люфт шкворней, шаровых или сайлентблоков. Сначала ремонт.

<a id="ru-data"></a>

## Откуда берутся допуски

Полные заводские базы допусков — платные коммерческие продукты, и выдумывать цифры нельзя: неверный развал — это съеденная за сезон резина и машина, которая плохо ведёт себя в экстренной ситуации. Поэтому у каждой записи в программе указан источник:

- `Заводское руководство` — сверено с документом, указаны издание и страница.
- `Сообщество` — перепроверено по независимому источнику.
- `Не проверено` — из одного источника, показывается с предупреждением.
- `Только конструкция` — известна подвеска, допусков нет; сравнение с ориентиром по классу.
- `Ориентир по классу` — типичные значения для конструкции, **не** для вашей модели.

У вас есть руководство по ремонту? Кнопка «Внести допуски» слева: программа проверит цифры на опечатки, запомнит их у вас и подготовит файл, который можно прислать в проект, — чтобы им пользовались все владельцы этой модели. Укажите издание и страницу: без источника запись не примут в общую базу.

<a id="ru-trouble"></a>

## Если что-то не работает

| Что происходит | Что сделать |
|---|---|
| Окно не открывается, программа открылась в браузере | Не установлен Microsoft Edge WebView2 Runtime (бывает на старых Windows 10). Установите его с сайта Microsoft — или работайте в браузере, это та же программа. |
| Телефон не открывает адрес | Телефон и компьютер в одной сети? Гостевой Wi-Fi часто изолирует устройства — подключите оба к точке доступа телефона. В Windows сеть должна быть «Частной», а брандмауэр — разрешать программу в частных сетях. |
| «Подключение не защищено» на телефоне | Так и должно быть: сертификат создан программой. «Дополнительно» → «Перейти на сайт». |
| «Ссылка устарела» | Доступ по Wi-Fi выключали и включали снова — ключ сменился. Отсканируйте QR-код заново. |
| Датчики телефона не отвечают | Откройте страницу в Chrome (Android) или Safari (iPhone); на iPhone разрешите доступ к датчикам движения. Встроенные браузеры мессенджеров датчики не дают. |
| Цифры «прыгают» | Телефон не прижат к планке, планка опирается на резину, ветер качает струну, кто-то сидит в машине. Дождитесь «показания стабильны». |
| Калибровка камеры «плохая» | Больше снимков, доска во всех углах кадра и под наклоном, доска наклеена на жёсткий плоский лист, размер клетки измерен штангенциркулем. |
| Нашли ошибку | Опишите её в Issues на GitHub и приложите `wheelalign.log` из папки данных. |

<a id="ru-safety"></a>

## Безопасность

- Под машину — только на надёжных подставках. Домкрат — подъёмное устройство, а не опора.
- Тормоза, рулевое и подвеска — узлы, от которых зависит жизнь. Не уверены в сборке — найдите того, кто уверен.
- После регулировки: первая поездка на малой скорости по пустой дороге. Руль стоит прямо, машину не уводит?
- Контргайки рулевых тяг затянуты, шплинты на месте.

<a id="ru-keys"></a>

## Клавиши

|   |   |
|---|---|
| <kbd>F1</kbd> | Инструкция |
| <kbd>F2</kbd>…<kbd>F6</kbd> | Автомобиль, подготовка, замер, регулировка, отчёт (на экране регулировки — свои) |
| <kbd>F12</kbd> | Следующий шаг |
| **На экране регулировки:** |  |
| <kbd>F2</kbd> | Сменить вид |
| <kbd>F3</kbd> / <kbd>F4</kbd> | Передняя / задняя ось крупно |
| <kbd>F5</kbd> | Снимок «до», затем «после» |
| <kbd>F6</kbd> | Отчёт |
| <kbd>F9</kbd> / <kbd>F10</kbd> | Демонстрация / показать регулировку по шагам |
| <kbd>←</kbd> <kbd>→</kbd> | В демонстрации — «крутить» выбранный угол (с Shift — крупнее) |
| <kbd>Esc</kbd> | Закрыть боковую панель |

---

<a id="eng"></a>

[Русский](#ru) · **English**

# Wheel alignment — guide

Wheel alignment is neither magic nor a workshop secret. It is four wheels, a few angles and the order in which to set them. Here is everything you need to do it yourself and do it right, from installing the program to the printed report.

The same guide is inside the program — press **F1**. Method and accuracy in depth — [PROCEDURE.md](PROCEDURE.md#eng).

**Contents:** [In short: five steps](#en-start) · [Installing and first start](#en-install) · [The angles in plain words](#en-angles) · [Step 1. Vehicle](#en-vehicle) · [Step 2. Preparation](#en-prep) · [Tools and floor](#en-tools) · [Measuring: strings and an inclinometer](#en-string) · [Measuring: a phone on the wheel](#en-phone) · [Measuring: camera and targets](#en-optical) · [Measuring: a sensor of your own](#en-sensor) · [Step 4. The adjustment screen](#en-live) · [Adjustment order — do not change it](#en-order) · [Step 5. The report](#en-report) · [Suspension types](#en-susp) · [Trucks, buses, vans](#en-trucks) · [When adjustment will not help](#en-bent) · [Where the specifications come from](#en-data) · [If something does not work](#en-trouble) · [Safety](#en-safety) · [Keys](#en-keys)

<a id="en-start"></a>

## In short: five steps

1. **Vehicle.** Find your car in the database or take the guidance for its class. Check the front and rear suspension type — the checks and the adjustment advice depend on it.
2. **Preparation.** A checklist for your suspension: pressures, play, load, floor. Adjusting a worn suspension is pointless — the angles will wander off at the first bump.
3. **Measurement.** Choose what to measure with: strings and an inclinometer, a phone, a camera or a sensor of your own. Methods can be combined — for example camber and caster by phone, toe by string.
4. **Adjustment.** A screen like an alignment bench: big figures, green when in spec, red when not; above each, a tolerance scale with a pointer. Tap an angle and a panel shows what adjusts it on your car. Before the first turn of a spanner, take the “before” snapshot (F5).
5. **Report.** When everything is green, take the “after” snapshot (F5) and open the report with its before/after table (F6). It can be printed or saved as PDF.

> New to this? Start the demonstration (F9 on the adjustment screen) and press F10 — the program shows by itself in which order and how the angles are set. With the ← → keys you can “turn” any angle yourself and watch what happens to the others.

<a id="en-install"></a>

## Installing and first start

1. **Download** `wheelalign.exe` from the project's releases page on GitHub. No installation is needed: it is a single file and can even live on a USB stick.
2. **Run it.** The program opens its own window, powered by WebView2, which is built into Windows 10 and 11. If the window cannot open, the program opens in your ordinary browser and tells you what to install.
3. **Language** — the RU / EN switch in the top right corner. On first start the system language is used; your choice is remembered.
4. **No internet needed.** Everything runs on this computer. Only the phone sensor needs a network, and a local one (Wi-Fi) at that, not the internet.

Where your data lives: `%APPDATA%\wheelalign` (on Linux `~/.config/wheelalign`). It holds the settings (`settings.json`), your own specifications (the `vehicles` folder), the phone certificate (`tls`) and the log `wheelalign.log` — attach it when reporting a bug.

Start options: `-browser` — open in the browser instead of a window; `-data <folder>` — keep the data elsewhere (for example next to the program on a USB stick). Full list: `wheelalign -h`.

<a id="en-angles"></a>

## The angles in plain words

| Angle | What it is | When it is off |
|---|---|---|
| **Camber** | Tilt of the wheel across the car, seen from the front. “Plus” — the top of the wheel leans out. | One edge of the tread wears (the outer one with positive, the inner with negative camber). A left–right difference above ~0.5° pulls the car towards the side with more camber. |
| **Toe** | Rotation of the wheel seen from above. “Plus” — the front edge of the wheel is closer to the car's centreline (toe-in). | The most expensive angle: it feathers a tyre in a few thousand kilometres. A crooked steering wheel when driving straight means the toe is split wrongly between the wheels. |
| **Caster** | Fore–aft tilt of the steering axis (the kingpin on kingpin axles). “Plus” — the top of the axis leans back. | Steering does not return to centre, the car wanders at speed. A left–right difference pulls towards the side with less caster. |
| **Steering axis inclination (SAI)** | Tilt of the steering axis across the car. Fixed by design. | Not adjustable. A left–right difference points to a bent strut, knuckle or axle beam. |
| **Included angle** | SAI + camber. | Differs left to right by more than 1° — a part is bent; camber adjustment will not cure it. |
| **Thrust angle** | Where the rear axle points relative to the car's centreline: half the difference of the rear wheels' toe. | The car “crabs”, the steering wheel is off-centre when driving straight although the front toe is in spec. |

<a id="en-vehicle"></a>

## Step 1. Vehicle

- **Search** by make and model, in Latin or Cyrillic letters: “Volga 3110”, “GAZelle”, “2107”, “Hilux”. The filters at the top narrow the list to cars, SUVs, vans, trucks or buses; the “Year” box drops other generations.
- Every entry carries a **source badge**: factory manual, community, unverified, design only. The weaker the badge, the more carefully treat the figures — see “Where the specifications come from”.
- **Model not in the database?** Choose the guidance for its class (“car with MacPherson struts”, “truck with a kingpin beam axle”…). These are typical values for the design, not for your model: the program shows how far the angles are from typical and says so honestly on every screen.
- **Suspension.** The program fills in the suspension type from the database; if it is missing, choose it yourself. “How do I tell which one I have?” explains how to tell a kingpin from a ball joint, MacPherson from double wishbones.
- **Dimensions.** The rim diameter converts toe from degrees to millimetres (many manuals give it that way). The front and rear track widths are used to check the strings.
- **Your own specifications.** If you have a workshop manual, use “Enter specs from the manual”. The program checks the figures for typos and uses them on this computer.

You can also work without choosing a vehicle — the angles are then shown without an in-spec / out-of-spec verdict.

![Choosing a vehicle](img/vehicle-en.png)

<a id="en-prep"></a>

## Step 2. Preparation

The checklist follows the chosen suspension: kingpin play for a beam axle, strut top bearings for MacPherson, U-bolts and the centre bolt for leaf springs. Tick items off as you go — the ticks are saved.

- **Tyre pressures** — as on the door-pillar placard, with cold tyres. A 0.3 bar left–right difference already changes camber.
- **Play** — tie-rod ends, ball joints or kingpins, wheel bearings, bushings. If a wheel rocks by hand, repair first.
- **Load** — as the manual requires: usually a full tank, spare wheel and tools in place, nobody inside. For trucks and buses — the stated axle load.
- **Steering straight.** Before measuring toe, set the steering wheel straight and lock it (a prop between wheel and seat).
- **Let the suspension settle.** After lifting the car, roll it back and forth a metre or two and bounce the body — the arms return to their working position.

<a id="en-tools"></a>

## Tools and floor

| What | What for | Substitute |
|---|---|---|
| Fishing line 0.3–0.5 mm, 2 × 5 m | Reference lines for toe | Not thread — it sags and stretches |
| 4 stands up to hub height | Hold the strings | Bricks with blocks, buckets of sand |
| Weights of 2–4 kg | Tension the strings without sag | Bottles of water |
| Tape measure, calipers or a rule | Distances to the rim | — |
| Inclinometer or smartphone | Camber | Spirit level with a protractor |
| Straight bar 40–50 cm | Holds the inclinometer or phone against the rim flanges | Aluminium straightedge |
| Turn plates | Caster | Two sheets of tin with grease between them |
| Axle stands | Safety | **Never a jack instead. Never.** |

**The floor matters most.** It need not be “level” but it must be *flat*: all four wheels in one plane. A 5 mm dip under one wheel on a 1400 mm track is 0.2° of camber error, half a factory tolerance. Check with a water level (a clear hose filled with water): the contact patches should differ by less than 2 mm. Shim with steel plates, not wood. Turn plates and the packing under the rear wheels must be the same height.

<a id="en-string"></a>

## Measuring: strings and an inclinometer

1. **Strings** run along both sides, outside the wheels, at hub-centre height, tensioned by weights. The distance from the string to the hub centre at the front and at the rear **need not be equal**: with different front and rear track widths the difference must be (rear track − front track) / 2. Enter the four distances in “String setup check” and the program tells you which end to move and by how much.
2. **Camber** — the inclinometer on a bar held against the rim flanges (not the tyre). “Plus” — the top of the wheel leans out. If your inclinometer reads the other way, tick the box. Then roll the car half a wheel turn (a chalk mark on the tyre) and measure the same spot of the rim again. The program averages the two readings and removes rim runout; runout above 0.5° means a bent rim and unreliable readings for that wheel.
3. **Toe** — from the string to the rim flange at the front and rear of the wheel, strictly at hub height. Higher or lower mixes camber into the reading. A wheel pointing inwards has the larger front distance; enter it as it is.
4. **Caster** — on turn plates: camber with the wheel steered 20° out and 20° in. The program solves this exactly, not with the “swing × 1.5” rule, which is off by up to 0.27° on cars with large caster. Read the steer angle off the turn plate scale or chalk it on the floor.

Every figure you enter goes straight to the adjustment screen. While working: turn the tie rod, measure, enter the new figure. The screen updates at once.

<a id="en-phone"></a>

## Measuring: a phone on the wheel

1. On “Measurement → Phone on the wheel” press “Allow Wi-Fi connection” and scan the QR code with the phone's camera. Phone and computer must be on the same Wi-Fi network (a hotspot on the phone itself will do).
2. The phone's browser warns about the certificate — it was made by this program, not by a certificate authority. “Advanced” → “Proceed to the site”. Windows may ask for firewall permission — allow private networks.
3. On the phone choose the wheel and press “Start sensors”. On an iPhone Safari asks for motion sensor access — allow it.
4. **Phone calibration** (once): lay it on a table screen up, then screen down. This removes the accelerometer's own error, which on phones can reach 1–2°.
5. **Mount calibration** (once per phone and bar): press the phone against the bar on the rim, then turn it upside down and press it against the same spot. This removes the tilt caused by the camera bump and the case.
6. **Rim runout** (optional): measure in one wheel position, roll the car half a turn and measure again — the program removes runout from all later readings.
7. Press the phone against the bar, screen facing out — camber flows to the adjustment screen in real time. The “steady” tag on the phone means the reading can be trusted.
8. **Caster:** wheels on turn plates, the “Caster” button on the phone. It asks you to set the wheel straight, steer out about 20°, then in — it measures the steer angle with its gyroscope.

A phone held to the rim does not measure toe: toe is a rotation of the wheel about the vertical, and the accelerometer senses only tilt — gravity does not change under such a rotation. The gyroscope does sense it, but drifts — within a minute by more than the whole toe tolerance. So take toe by string or by camera: the same phone on a tripod works as a camera in live mode (see “Camera and targets”). Two phones on the wheels measure two wheels at once, four — all four. When you are done, switch Wi-Fi access off. A phone connected by mistake is removed with “Disconnect” in the list.

<img src="img/phone.png" width="220" alt="The phone page">

<a id="en-optical"></a>

## Measuring: camera and targets

A printed chessboard on a rigid sheet can be fixed to the wheel as crookedly as you like — the program finds the wheel's axis of rotation from a series of photos, and the mounting error drops out.

1. **Camera calibration** (once per camera and zoom): 10–20 photos of a chessboard at different angles and in all corners of the frame. Measure the square size on the printout with calipers — printers scale. Save `camera.json`.
2. **Camber from photos:** wheel lifted, target on the rim, camera strictly level at the side. 4–6 photos, turning the wheel 10–20° between them.
3. **Full measurement:** every frame must show a floor target as well as the wheel target — it defines the road plane and ties the four wheels into one coordinate system. It gives camber, individual toe and the thrust angle. There are two floor targets (front and rear) of different sizes; photos showing both link them.

**Live mode** (tab “4. Live mode”) — as on a professional 3D aligner: a phone on a tripod watches the wheel and the floor target, and camber and toe update on the adjustment screen 3–4 times a second while you turn the tie rod. On the phone — the “Phone as a camera” button:

1. **Print the targets** — on the “4. Live mode” tab, the “PDF: wheel targets” and “PDF: floor targets” buttons. Print at 100 % scale and check the 100 mm ruler at the bottom of the sheet. Glue the targets to a rigid flat board (plywood, chipboard, composite). Each wheel has its own target of a different size — the program recognises which wheel it sees, there is no need to choose it on the phone. The floor targets are large: it is easier to order them as one sheet from a print shop.
2. **Camera calibration** — once per phone: show it the wheel target at different angles and in different parts of the frame until the bar fills. Hold the phone in landscape — and the same way later when measuring.
3. **Runout** — for each wheel: lift it and slowly turn it by hand a quarter turn or more. The program remembers how the wheel's axis sits relative to the target, and from then on a single frame is enough.
4. **Measure** — car on the floor. Show the camera all four wheels in turn (and two or three frames showing both floor targets), then toe appears. After that, put the phone by the wheel you are adjusting.
5. **Caster by camera** — front wheels on turn plates, the brake pedal held down with a prop so the wheel cannot roll. “Caster by camera” mode on the phone, slowly steer 15–20° one way, then the other. The program finds the wheel's steering axis and shows caster and steering axis inclination.

Details, target sizes and photography tips are in OPTICAL.md in the repository.

<a id="en-sensor"></a>

## Measuring: a sensor of your own

Any device on your network — a home-made ESP32 head, a laser sensor, another program — can send angles to the program as a plain HTTP request with JSON. The format is on “Measurement → Own sensor” and in SENSORS.md. The program smooths the readings, detects when they have settled and shows the sensor in the status bar at the top.

<a id="en-live"></a>

## Step 4. The adjustment screen

- **Figures.** Green — in spec, amber — in spec but near the limit, red — out of spec, white — no specification, grey — no reading. Above the figure is the tolerance scale: the green zone and a pointer at the current value.
- **Views.** F2 cycles between the overview and large front- and rear-axle views; F3 and F4 jump straight to an axle. Under the car the large view is easier: the figures are readable from two or three metres.
- **Units** — degrees and minutes or decimal degrees; toe can be shown in millimetres (at the rim diameter from the Vehicle step).
- **“Readings steady”.** While values are changing the screen says “readings changing…”. Record figures and tighten lock nuts only when steady.
- **Tap any angle** — a panel opens on the right: what the angle is, what adjusts it on your suspension, what the data says for this model.
- **The “how far to turn” helper.** In the same panel: press “Remember position”, turn the tie rod or eccentric by a known amount (¼, ½, 1 turn) and press how far you turned it. The program works out what one turn does on your car and from then on tells you: “to nominal ≈ 1¼ turns, the same way”.
- **The source bar** at the top shows where data comes from: phone, sensor, camera, manual entry, demo.

![The adjustment screen](img/live-overview-en.png)

<a id="en-order"></a>

## Adjustment order — do not change it

1. **Rear axle** (camber, then toe) — it sets the thrust line from which front toe is measured.
2. **Caster** — changing it moves both camber and toe.
3. **Camber** — it moves toe.
4. **Toe — last.** Steering wheel dead straight and locked. Set the toe of *each* wheel, not just the total: the total can be reached in endless ways, and only one leaves the steering wheel straight. Turn both tie rods by the same amount in opposite directions.
5. **Check measurement.** Tighten the lock nuts (tightening often shifts toe), roll the car 3–5 m, bounce the body and measure again. An alignment without a check measurement is not an alignment, it is a hope.

**Which way to turn the tie rod if you don't know:** a quarter turn, then watch the pointer. The live screen answers that question faster than any diagram.

<a id="en-report"></a>

## Step 5. The report

The report is built from two snapshots: “before” — ahead of adjustment, and “after” — once everything is set. F5 on the adjustment screen takes a snapshot in one press (“before” first, then “after”). A snapshot needs camber and toe of all four wheels.

You can add the plate number or VIN, the mileage and your name. “Print / PDF” opens the system print dialog — choose “Save as PDF” to get a file. The report lists the source of the specifications, the adjustment order and remarks (rim runout, included-angle differences and so on).

<a id="en-susp"></a>

## Suspension types

Every model has its own factory specification, but there are only about a dozen suspension designs in the world. The design decides what can be adjusted at all, with what, and what to check before adjusting.

| In the program | Suspension | Axle |
|---|---|---|
| MacPherson strut | a strut combined with the damper | either |
| Double wishbone, ball joints | two transverse arms, knuckle on ball joints | either |
| Double wishbone, kingpin | GAZ-21, 24, 3102, 3110: knuckle on a kingpin | front |
| Rigid beam, kingpins | trucks, buses, GAZelle, UAZ | front |
| Rigid axle, ball joints | an axle without kingpins | front |
| Multi-link | several links per wheel | either |
| Twist beam | a beam twisting between trailing arms | rear |
| Live axle on leaf springs | rigid axle on leaf springs | rear |
| Live axle on coil springs and links | rigid axle, coil springs or air bags, reaction rods | rear |
| Trailing arms | independent, trailing arms | rear |
| Semi-trailing arms | independent, semi-trailing arms | rear |

For each design the program shows how to recognise it on your car, what is usually adjustable and with what, and what
to check before adjusting: **F1 → “Suspension types”**.

<a id="en-trucks"></a>

## Trucks, buses, vans

- Kingpin front beam axle: **camber and kingpin inclination are not adjustable** — a deviation means a bent beam, worn kingpins or bearings.
- **Caster** is corrected with taper shims between the leaf spring and the beam pad — the same on both sides.
- **Toe** is set by the cross tie rod — total toe only. The steering wheel is centred with the length of the drag link (pitman arm to steering arm).
- Twin rear wheels count as one wheel — measure the outer one; both tyres at the same pressure.
- Truck manuals often give toe in mm at the tyre or at the rim — enter the diameter the figure refers to.
- Multi-axle vehicles (rear bogie) are treated as two-axle for now: the front axle and one rear axle. Check the bogie axles for parallelism separately.

<a id="en-bent"></a>

## When adjustment will not help

- **Different included angle** (steering axis inclination + camber) left to right by more than 1° — a bent strut, knuckle or arm. Camber and SAI may each “lie”, their sum does not.
- **Different wheelbase** left to right (more than 10 mm) — a deformed body, frame or arms.
- **Large setback** of the wheels along the car (more than 12 mm) — a deformed subframe or chassis rail.
- **Thrust angle** on a rigid axle or beam — there is no adjustment: a sheared spring centre bolt, loose U-bolts, a bent axle.
- **Camber wanders** from one reading to the next — play in kingpins, ball joints or bushings. Repair first.

<a id="en-data"></a>

## Where the specifications come from

Complete factory specification databases are paid commercial products, and figures must never be made up: a wrong camber means tyres eaten in one season and a car that behaves badly in an emergency. So every entry in the program names its source:

- `Factory manual` — checked against the document, edition and page given.
- `Community` — cross-checked against an independent source.
- `Unverified` — from a single source, shown with a warning.
- `Design only` — the suspension is known, the tolerances are not; compared with the class guidance.
- `Class guidance` — typical values for the design, **not** for your model.

Have a workshop manual? The “Enter specs” button on the left: the program checks the figures for typos, keeps them on your computer and prepares a file you can send to the project so that every owner of that model can use it. Give the edition and page: without a source an entry is not accepted into the shared database.

<a id="en-trouble"></a>

## If something does not work

| What happens | What to do |
|---|---|
| No window, the program opened in the browser | Microsoft Edge WebView2 Runtime is missing (happens on older Windows 10). Install it from Microsoft's site — or keep using the browser, it is the same program. |
| The phone cannot open the address | Are phone and computer on the same network? Guest Wi-Fi often isolates devices — connect both to the phone's hotspot. In Windows the network must be “Private” and the firewall must allow the program on private networks. |
| “Connection is not private” on the phone | That is expected: the certificate was made by the program. “Advanced” → “Proceed to the site”. |
| “Link expired” | Wi-Fi access was switched off and on again, so the key changed. Scan the QR code again. |
| The phone's sensors do not respond | Open the page in Chrome (Android) or Safari (iPhone); on an iPhone allow motion sensor access. Browsers built into messengers do not provide the sensors. |
| Figures jump around | The phone is not pressed to the bar, the bar rests on the tyre, wind shakes the string, someone is sitting in the car. Wait for “readings steady”. |
| Camera calibration is “poor” | More photos, the board in every corner of the frame and tilted, the board glued to a rigid flat sheet, the square size measured with calipers. |
| You found a bug | Describe it in Issues on GitHub and attach `wheelalign.log` from the data folder. |

<a id="en-safety"></a>

## Safety

- Get under a car only on sound axle stands. A jack is a lifting device, not a support.
- Brakes, steering and suspension are what lives depend on. Not sure about the assembly — find someone who is.
- After adjustment: the first drive at low speed on an empty road. Is the steering wheel straight, does the car pull?
- Tie-rod lock nuts tight, split pins in place.

<a id="en-keys"></a>

## Keys

|   |   |
|---|---|
| <kbd>F1</kbd> | Guide |
| <kbd>F2</kbd>…<kbd>F6</kbd> | Vehicle, preparation, measurement, adjustment, report (the adjustment screen has its own) |
| <kbd>F12</kbd> | Next step |
| **On the adjustment screen:** |  |
| <kbd>F2</kbd> | Change view |
| <kbd>F3</kbd> / <kbd>F4</kbd> | Front / rear axle, large |
| <kbd>F5</kbd> | “Before” snapshot, then “after” |
| <kbd>F6</kbd> | Report |
| <kbd>F9</kbd> / <kbd>F10</kbd> | Demonstration / show the adjustment step by step |
| <kbd>←</kbd> <kbd>→</kbd> | In the demonstration — “turn” the selected angle (with Shift — in bigger steps) |
| <kbd>Esc</kbd> | Close the side panel |

