<a id="ru"></a>

**Русский** · [English](#eng)

# Оптический режим: камера и печатные мишени

Технические подробности оптического тракта — от пикселей до углов установки
колёс. Как пользоваться — в программе: «Замер → Камера и мишени» и раздел
«Инструкция».

**Работает весь тракт: снимок → углы установки колёс.**

- **детектор шахматной мишени.** Угол доски — это седловая точка яркости, и
  отклик берётся точно, без подгоняемых коэффициентов:
  `R = I_xy² − I_xx·I_yy`, то есть определитель гессиана со знаком минус,
  положительный ровно там, где поверхность яркости седловая. Субпиксельное
  положение — из аналитического решения седла квадратичной поверхности,
  подогнанной по окрестности. **Точность 0,025–0,04 пикс**;
- **отсев ложных откликов.** По периметру доски клетки граничат с фоном и
  образуют T-стыки — они тоже седловые, лежат снаружи всех настоящих углов и
  портят выпуклую оболочку, по которой ищется сетка. Отсекаются точным
  признаком: вокруг настоящего X-стыка яркость по кольцу меняет знак **ровно
  4 раза**, вокруг T-стыка — 2. Порогов подбирать не нужно;
- **разрешение поворота на 180°.** Шахматная доска выглядит одинаково вверх
  ногами, и геометрия здесь бессильна — но при повороте чёрные клетки меняются
  местами с белыми, если сумма сторон доски нечётна. Поэтому доски с чётной
  суммой (8×6, 9×5) программа **отвергает**: они неоднозначны принципиально, а
  детектор, который «угадывал» бы ориентацию, ронял бы позу между кадрами и
  разрушал компенсацию биения;
- модель камеры: пинхол + дисторсия Брауна–Конради. Дисторсия здесь важнее, чем
  обычно: колёса стоят у краёв кадра, а радиальная дисторсия растёт как
  четвёртая и шестая степень радиуса;
- решение PnP для плоской мишени: DLT-гомография с нормализацией Хартли,
  разложение на поворот и перенос, уточнение по методу Левенберга–Марквардта на
  настоящей ошибке перепроецирования в пикселях. На чистых данных
  восстанавливает позу с точностью 10⁻⁶ градуса;
- **обнаружение неоднозначности плоской мишени** — см. ниже;
- восстановление оси вращения колеса из последовательности поз мишени
  (компенсация биения, `geom.FitRotationAxis`);
- восстановление оси поворота методом конуса (`measure.SteeringAxisFromSweep`):
  ось вращения колеса при повороте описывает конус, ось конуса и есть ось
  поворота. Нечувствителен к прокручиванию колеса на кругах и не требует знать
  угол поворота;
- система координат автомобиля, инвариантная к положению оборудования.

Сквозной тест `TestImageToCamber` проверяет всё это разом, **от нарисованных
пикселей до угла колеса, без единого подставленного вручную числа**. Мишень
**намеренно закреплена криво — на 2,5°**, что хуже любого реального зажима и в
несколько раз больше всего допуска на развал. Колесо проворачивают, каждое
положение рендерится как снимок через модель камеры с шумом сенсора,
распознаётся, решается на позу — и по последовательности восстанавливается ось
вращения колеса:

```
детектор: разброс по сетке до 0,113 пикс; PnP: невязка до 0,070 пикс
ось восстановлена с ошибкой 0,0073°
развал -1,3428° (задано -1,35°), схождение 0,2194° (задано 0,22°)
```

Ошибка крепления исчезает полностью — потому что ось, **вокруг** которой
вращается мишень, есть ось колеса при любом угле её посадки. Это и означает,
что зажимы можно делать в гараже из подручного, не выдерживая точности.

### Калибровка камеры

Своя, по методу Чжана — OpenCV не нужен:

```bash
wheelalign calibrate ./снимки-мишени
```

Каждый снимок даёт гомографию плоскости доски в кадр. Так как доска плоская,
`H = λ·K·[r₁ r₂ t]`, а первые два столбца матрицы поворота ортонормированы —
и эти два факта дают по два линейных ограничения на `B = K⁻ᵀK⁻¹`. Трёх снимков
уже достаточно, дальше `K` извлекается в замкнутой форме, а всё вместе
уточняется по настоящей ошибке перепроецирования, уже с дисторсией.

На синтетике без шума восстанавливает камеру **точно** — все пять коэффициентов
дисторсии и обе фокусные. На снимках 8 бит с шумом: fx 980,4 при истинных 980,
k1 −0,205 при −0,21, СКО 0,043 пикс.

**Предупреждения тут важнее цифр.** Калибровка проваливается тихо: серия
снимков, сделанных все «в лоб» или все в середине кадра, даёт отличную невязку
и коэффициенты дисторсии, которые никакими данными не подкреплены — а
проявится это как раз у краёв кадра, где стоят колёса. Поэтому программа меряет
не только невязку, но и **разброс наклона мишени** и **покрытие кадра**, и прямо
говорит, что переснять.

### Поиск сетки наращиванием от затравки

Сложить найденные углы в сетку — отдельная задача, и решается она не поиском по
выпуклой оболочке (наибольший вписанный четырёхугольник как контур доски), а
наращиванием. Разница принципиальная: оболочка спрашивает «какие четыре точки —
углы доски?» и ошибается, стоит сквозь фильтры пройти паре лишних точек — контур
доски перестаёт быть наибольшим четырёхугольником, и подбор порогов тут не
помогает, потому что неверна сама посылка.

Наращивание задаёт локальный вопрос. Берём два соседних угла; следующий по ряду
лежит там, где велит решётка, а его положение предсказуемо по уже найденным
соседям — по правилу параллелограмма (три угла клетки задают четвёртый точно при
любом аффинном преобразовании, а проекция аффинна с точностью до первого порядка
на масштабе одной клетки). Привязываемся к ближайшему реальному углу, повторяем.
Мусор просто никогда не достигается: он ни к чему в решётке не примыкает, и его
количество в кадре перестаёт иметь значение.

Если наращивание перехлестнёт через внутренние углы на T-стыки периметра (они
лежат ровно на продолжении решётки), выросшая сетка оказывается на ряд-другой
больше — тогда внутренние углы доски это окно cols×rows внутри неё, и проверяются
все такие окна. На синтетике с полным периметром из 84 точек это даёт 20
окон-кандидатов, из которых разбор ориентации ниже выбирает единственно верное.

Результат: серия из 8 снимков с наклоном до 43° распознаётся **вся**, включая
кадры с рассыпанным вокруг мусором.

**О неоднозначности плоской мишени.** Плоская мишень допускает второе решение,
зеркальное относительно линии взгляда. Ошибка в выборе даёт ошибку развала вдвое
больше наклона мишени — классический тихий отказ всех систем с плоскими метками.
Программа считает оба решения и сравнивает:

| Условия | Основное | Альтернатива | Вердикт |
|---|---|---|---|
| Мишень на 9 м, наклон 30° | 0,333 пикс | 0,339 пикс | **неоднозначно** |
| Та же мишень на 1,2 м | 0,332 пикс | 5,974 пикс | однозначно |

Различить решения позволяет только перспективное искажение, а оно исчезает,
когда мишень мелкая в кадре. Поэтому программа не «выбирает получше», а прямо
говорит: подойдите ближе или возьмите мишень крупнее.

**Про железо.** Четыре пассивные мишени на колёсах и камера. Мишень —
распечатанная шахматная доска 9×6 клеток по 30 мм на жёстком листе (умещается
на A4). Квадратные доски программа отвергает: у них симметрия 90°, из-за которой
развал можно принять за схождение. Крепление к диску — самодельный зацеп за края
обода; точность крепления, как показано выше, значения не имеет.

## Связывание четырёх колёс

Развал одной камерой меряется по одному колесу и системы координат автомобиля не
требует. Схождение и угол тяги — требуют: это отношения **между** колёсами, а
значит все четыре надо выразить в одной неподвижной системе.

Решение — **напольная реперная мишень**, попадающая в кадр вместе с колёсной.
Тогда каждый кадр даёт позу колеса не относительно камеры, а относительно пола:

```
T_реп→колесо = (T_кам→реп)⁻¹ · T_кам→колесо
```

Камера сокращается — её можно свободно переносить между кадрами и между колёсами.
А поскольку реперная мишень лежит на полу, её плоскость и есть плоскость дороги:
вертикаль берётся прямо из неё, а высота центра колеса над ней — это
**измеренный**, а не предполагаемый радиус качения.

Одну напольную мишень не видно от всех четырёх колёс — мешает сам автомобиль.
Поэтому мишеней несколько (например, спереди и сзади), а связываются они
**связующим кадром**, где видно сразу две: их взаимная поза получается делением
одной на другую, камера снова сокращается. Из таких связей строится граф, обход
которого приводит все мишени, а с ними и все колёса, в одну систему. Камере
никогда не нужно видеть всё сразу — именно это делает полный оптический
сход-развал возможным с одним телефоном.

Проверено сквозным тестом на отрендеренной сцене: автомобиль, две напольные
мишени, один связующий кадр, все мишени на колёсах закреплены криво и по-разному
(2,5° / 1,4° / 3,1° / 0,8°):

```
схождение  восстановлено с ошибкой до 0,017°
развал                              до 0,033°
угол тяги  0,090° при заданном 0,080°
радиус качения измерен как высота центра колеса над плоскостью пола
```

Мишени должны различаться по размеру — две одинаковые в кадре неразличимы.
Детектор ищет их от большей к меньшей, снимая найденные углы с доски: меньшая
сетка укладывается внутрь большей (8×5 — это часть 9×6), поэтому обратный порядок
мог бы «найти» маленькую мишень на куске большой.

В интерфейсе это «Замер → Камера и мишени → Полный замер»: файл калибровки, параметры мишеней,
связующие снимки и по 4–6 кадров на колесо. Результат — обычный протокол со
схемой, таблицей и порядком регулировки, потому что оптический замер выдаёт
ровно тот же `align.Result`, что и замер по струне: способ измерения до слоя
отчёта не доходит.

---
---

<a id="eng"></a>

[Русский](#ru) · **English**

# The optical mode: a camera and printed targets

The technical details of the optical pipeline — from pixels to wheel alignment
angles. How to use it is in the program: “Measurement → Camera and targets” and
the “Guide”.

**The whole pipeline works: photo → wheel alignment angles.**

- **chessboard detector.** A board corner is a saddle point of brightness, and
  the response is taken exactly, with no tuned coefficients:
  `R = I_xy² − I_xx·I_yy`, the negated determinant of the Hessian, positive
  exactly where the brightness surface is a saddle. The sub-pixel position comes
  from the analytic saddle of a quadratic surface fitted to the neighbourhood.
  **Accuracy 0.025–0.04 px**;
- **rejecting false responses.** Along the board's edge the squares meet the
  background and form T-junctions — they are saddles too, lie outside all the
  real corners and spoil the convex hull used to find the grid. They are removed
  by an exact test: around a real X-junction the brightness along a ring changes
  sign **exactly 4 times**, around a T-junction — 2. No thresholds to tune;
- **resolving the 180° turn.** A chessboard looks the same upside down, and
  geometry is powerless here — but on turning, the black squares swap with the
  white ones if the sum of the board's sides is odd. So boards with an even sum
  (8×6, 9×5) are **rejected**: they are ambiguous in principle, and a detector
  that “guessed” the orientation would flip the pose between frames and destroy
  the runout compensation;
- camera model: pinhole + Brown–Conrady distortion. Distortion matters more than
  usual here: the wheels sit at the edges of the frame, and radial distortion
  grows with the fourth and sixth power of the radius;
- PnP for a planar target: a DLT homography with Hartley normalisation,
  decomposition into rotation and translation, refinement by Levenberg–Marquardt
  on the true reprojection error in pixels. On clean data it recovers the pose to
  10⁻⁶ degree;
- **detecting planar-target ambiguity** — see below;
- recovering the wheel's axis of rotation from a sequence of target poses
  (runout compensation, `geom.FitRotationAxis`);
- recovering the steering axis by the cone method
  (`measure.SteeringAxisFromSweep`): while steering, the wheel's axis of rotation
  sweeps a cone, and the cone's axis is the steering axis. Insensitive to the
  wheel rolling on the turn plates, and needs no steer angle;
- a vehicle coordinate system independent of where the equipment stands.

The end-to-end test `TestImageToCamber` checks all of this at once, **from
rendered pixels to the wheel angle, without a single hand-inserted number**. The
target is **deliberately mounted crooked — by 2.5°**, worse than any real clamp
and several times the whole camber tolerance. The wheel is turned, each position
is rendered as a photo through the camera model with sensor noise, detected,
solved for pose — and the wheel's axis of rotation is recovered from the
sequence:

```
detector: grid scatter up to 0.113 px; PnP: residual up to 0.070 px
axis recovered with an error of 0.0073°
camber -1.3428° (set -1.35°), toe 0.2194° (set 0.22°)
```

The mounting error vanishes completely — because the axis **around** which the
target rotates is the wheel's axis whatever the angle it sits at. That is why the
clamps can be made in a garage from whatever is at hand, with no precision.

### Camera calibration

Our own, by Zhang's method — no OpenCV needed:

```bash
wheelalign calibrate ./target-photos
```

Every photo gives a homography from the board plane to the image. Since the
board is flat, `H = λ·K·[r₁ r₂ t]`, and the first two columns of the rotation
matrix are orthonormal — these two facts give two linear constraints each on
`B = K⁻ᵀK⁻¹`. Three photos are already enough; `K` is then extracted in closed
form, and everything is refined together on the true reprojection error, now
with distortion.

On noise-free synthetic data it recovers the camera **exactly** — all five
distortion coefficients and both focal lengths. On 8-bit noisy images: fx 980.4
against a true 980, k1 −0.205 against −0.21, RMS 0.043 px.

**The warnings matter more than the numbers here.** A calibration fails
silently: a series of photos all taken head-on or all in the middle of the frame
gives an excellent residual and distortion coefficients that no data supports —
and it shows exactly at the frame edges, where the wheels are. So the program
measures not only the residual but also the **spread of target tilt** and
**frame coverage**, and says plainly what to retake.

### Finding the grid by growing it from a seed

Putting the detected corners into a grid is a problem of its own, and it is
solved not by a convex-hull search (the largest inscribed quadrilateral as the
board outline) but by growing. The difference is fundamental: the hull asks
“which four points are the board's corners?” and goes wrong as soon as a couple
of stray points pass the filters — the board outline stops being the largest
quadrilateral, and tuning thresholds does not help, because the premise itself
is wrong.

Growing asks a local question. Take two neighbouring corners; the next one along
the row lies where the lattice says, and its position is predicted from the
corners already found — by the parallelogram rule (three corners of a square fix
the fourth exactly under any affine map, and a projection is affine to first
order at the scale of one square). Snap to the nearest real corner, repeat.
Clutter is simply never reached: it is attached to nothing in the lattice, and
how much of it there is in the frame stops mattering.

If growing overshoots the inner corners onto the edge T-junctions (they lie
exactly on the continuation of the lattice), the grown grid is a row or two
larger — the board's inner corners are then a cols×rows window inside it, and
every such window is checked. On synthetic data with the full edge of 84 points
this gives 20 candidate windows, of which the orientation check below picks the
only right one.

Result: a series of 8 photos tilted up to 43° is detected **completely**,
including frames with clutter scattered around.

**About planar-target ambiguity.** A planar target admits a second solution,
mirrored about the line of sight. Picking the wrong one gives a camber error of
twice the target's tilt — the classic silent failure of every system with planar
markers. The program computes both solutions and compares:

| Conditions | Primary | Alternative | Verdict |
|---|---|---|---|
| Target at 9 m, tilted 30° | 0.333 px | 0.339 px | **ambiguous** |
| The same target at 1.2 m | 0.332 px | 5.974 px | unambiguous |

Only perspective distortion tells the solutions apart, and it disappears when the
target is small in the frame. So the program does not “pick the better one”; it
says plainly: come closer or use a bigger target.

**About the hardware.** Four passive targets on the wheels and a camera. A target
is a printed chessboard of 9×6 squares of 30 mm on a rigid sheet (fits on A4).
Square boards are rejected: their 90° symmetry could make camber be taken for
toe. The mount to the rim is a home-made hook over the rim edges; the mounting
accuracy, as shown above, does not matter.

## Linking the four wheels

Camber is measured wheel by wheel with one camera and needs no vehicle coordinate
system. Toe and the thrust angle do: they are relations **between** wheels, so
all four must be expressed in one fixed frame.

The solution is a **floor reference target** in the frame together with the wheel
target. Every frame then gives the wheel's pose not relative to the camera but
relative to the floor:

```
T_ref→wheel = (T_cam→ref)⁻¹ · T_cam→wheel
```

The camera cancels out — it can be moved freely between frames and between
wheels. And since the reference target lies on the floor, its plane is the road
plane: the vertical comes straight from it, and the height of the wheel centre
above it is the **measured**, not assumed, rolling radius.

One floor target cannot be seen from all four wheels — the car itself is in the
way. So there are several targets (for example front and rear), linked by a
**linking frame** that shows two at once: their relative pose is one divided by
the other, and the camera cancels again. The links form a graph whose traversal
brings all targets, and with them all wheels, into one frame. The camera never
has to see everything at once — which is what makes a full optical alignment
possible with a single phone.

Tested end to end on a rendered scene: a car, two floor targets, one linking
frame, all wheel targets mounted crooked and differently (2.5° / 1.4° / 3.1° /
0.8°):

```
toe          recovered with an error of up to 0.017°
camber                                   up to 0.033°
thrust angle 0.090° against a set 0.080°
rolling radius measured as the wheel centre's height above the floor plane
```

The targets must differ in size — two identical ones are indistinguishable in a
frame. The detector looks for them from the larger to the smaller, removing the
corners found from the board: a smaller grid fits inside a larger one (8×5 is
part of 9×6), so the reverse order could “find” the small target on a piece of
the large one.

In the interface this is “Measurement → Camera and targets → Full measurement”:
the calibration file, the target parameters, the linking photos and 4–6 frames
per wheel. The result is the usual report with the diagram, the table and the
adjustment order, because an optical measurement yields exactly the same
`align.Result` as a string measurement: the measuring method never reaches the
report layer.
