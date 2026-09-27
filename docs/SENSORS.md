<a id="ru"></a>

**Русский** · [English](#eng)

# Свой датчик: открытый протокол

Программа принимает углы от любого устройства, которое умеет отправить
HTTP-запрос: самодельная голова на ESP32 с инклинометром, лазерный указатель
схождения со шкалой, промышленный угломер с последовательным портом и
переходником, скрипт на Raspberry Pi. Показания сразу появляются на экране
регулировки — вместе с телефонами, струной и камерой, по колёсам.

## Куда отправлять

С этого же компьютера:

```
POST http://127.0.0.1:8700/api/live/sample
```

(порт виден в строке `Интерфейс:` при запуске; при занятости 8700 программа
берёт любой свободный).

С другого устройства в сети — включите в программе «Замер → Телефон на
колесе → Разрешить подключение по Wi-Fi». Появится адрес вида
`https://192.168.0.57:8701/p/abcd1234ef`; к нему допишите `/sample`:

```
POST https://192.168.0.57:8701/p/abcd1234ef/sample
```

Ключ в адресе меняется при каждом включении — чужое устройство без него ничего
не отправит. Сертификат самоподписанный, проверку сертификата в датчике
отключите (в `curl` — ключ `-k`, в Arduino `WiFiClientSecure` —
`setInsecure()`).

## Что отправлять

Одно показание — один JSON-объект, или массив объектов за раз:

```json
{"source": "esp32-левое", "wheel": "FL", "camber": -0.52, "toe": 0.08}
```

| Поле | Обязательно | Что это |
|---|---|---|
| `source` | да | Имя датчика. По нему он виден в строке состояния; показания одного датчика сглаживаются вместе |
| `name` | нет | Подпись для людей, например «Левая голова» |
| `wheel` | да | `FL`, `FR`, `RL`, `RR` — переднее левое, переднее правое, заднее левое, заднее правое (со стороны водителя) |
| `camber` | нет | Развал, градусы. **Плюс — верх колеса наружу** |
| `toe` | нет | Схождение колеса, градусы, от оси автомобиля. **Плюс — передний край колеса к центру машины** |
| `caster` | нет | Кастер, градусы, только передние колёса. Плюс — верх оси поворота назад |

Любое поле из трёх углов можно не присылать: датчик развала не обязан мерить
схождение. Программа сведёт на одном экране всё, что пришло от разных
источников.

## Как часто

До 20 раз в секунду. Сглаживание и признак «показания устоялись» программа
делает сама: разброс меньше 0,03° и нет тренда за последние полторы секунды.
Если датчик замолчал больше чем на 3 секунды, его показания остаются на экране,
но помечаются как устаревшие.

## Ответ

`200` и `{"ok": true, "accepted": N}` — принято. `400` и `{"error": "…"}` —
нет, с объяснением на языке программы: неизвестное колесо, нет `source`, угол,
которого у дорожного колеса не бывает (развал больше 15°, схождение больше 10°),
кастер на заднем колесе.

## Примеры

Проверить из командной строки:

```bash
curl -X POST http://127.0.0.1:8700/api/live/sample \
  -H 'Content-Type: application/json' \
  -d '{"source":"test","wheel":"FL","camber":-0.5,"toe":0.1}'
```

Python, пачкой по всем колёсам:

```python
import json, urllib.request

readings = [
    {"source": "rig", "wheel": w, "camber": c, "toe": t}
    for w, c, t in [("FL", -0.5, 0.10), ("FR", -0.4, 0.08), ("RL", -1.1, 0.12), ("RR", -1.0, 0.11)]
]
req = urllib.request.Request(
    "http://127.0.0.1:8700/api/live/sample",
    data=json.dumps(readings).encode(),
    headers={"Content-Type": "application/json"},
)
print(urllib.request.urlopen(req).read().decode())
```

## Что важно для точности самодельной головы

- **Ось датчика и ось колеса.** Развал — это наклон плоскости обода. Голова
  должна опираться на закраины обода (три точки или планка), а не на шину.
- **Смещение нуля.** Недорогие MEMS-акселерометры (MPU6050, ADXL345) имеют
  смещение нуля в десятки mg — это градус и больше. Калибруйте переворотом:
  прибор в одном положении, затем развёрнутый на 180° на той же опоре;
  полусумма — смещение. Для развала лучше подходят инклинометры с заводской
  калибровкой (SCA100T, ADXL355) — они дороже, но стабильнее по температуре.
- **Биение диска.** Два замера с прокаткой машины на пол-оборота колеса; среднее
  — развал, полуразность — биение. Это можно сделать в прошивке, а можно
  прислать программе уже компенсированное значение.
- **Кастер** датчик может прислать уже посчитанный, а может присылать только
  развал — и кастер тогда меряется по шагам через поворот колеса (струной и
  угломером или телефоном).

---
---

<a id="eng"></a>

[Русский](#ru) · **English**

# Your own sensor: the open protocol

The program accepts angles from any device that can send an HTTP request: a
home-made ESP32 head with an inclinometer, a laser toe gauge with a scale, an
industrial inclinometer with a serial port and an adapter, a script on a
Raspberry Pi. The readings appear on the adjustment screen at once — together
with phones, strings and the camera, wheel by wheel.

## Where to send

From the same computer:

```
POST http://127.0.0.1:8700/api/live/sample
```

(the port is shown in the `Interface:` line at start-up; if 8700 is taken, the
program picks any free one).

From another device on the network, switch on “Measurement → Phone on the
wheel → Allow Wi-Fi connection” in the program. An address like
`https://192.168.0.57:8701/p/abcd1234ef` appears; add `/sample` to it:

```
POST https://192.168.0.57:8701/p/abcd1234ef/sample
```

The key in the address changes every time access is switched on — a stranger's
device cannot send anything without it. The certificate is self-signed, so turn
off certificate checking in the sensor (`-k` in `curl`, `setInsecure()` in
Arduino's `WiFiClientSecure`).

## What to send

One reading is one JSON object, or send an array of them at once:

```json
{"source": "esp32-left", "wheel": "FL", "camber": -0.52, "toe": 0.08}
```

| Field | Required | Meaning |
|---|---|---|
| `source` | yes | The sensor's name. It is shown in the status bar under this name; one sensor's readings are smoothed together |
| `name` | no | A label for people, e.g. “Left head” |
| `wheel` | yes | `FL`, `FR`, `RL`, `RR` — front left, front right, rear left, rear right (as seen from the driver's seat) |
| `camber` | no | Camber, degrees. **Plus — top of the wheel out** |
| `toe` | no | The wheel's toe, degrees, from the car's centreline. **Plus — front edge of the wheel towards the centre of the car** |
| `caster` | no | Caster, degrees, front wheels only. Plus — top of the steering axis back |

Any of the three angles may be left out: a camber sensor need not measure toe.
The program combines everything from different sources on one screen.

## How often

Up to 20 times a second. The program does the smoothing and the “readings
settled” flag itself: scatter below 0.03° and no trend over the last second and
a half. If a sensor falls silent for more than 3 seconds, its readings stay on
screen but are marked as stale.

## Response

`200` and `{"ok": true, "accepted": N}` — accepted. `400` and `{"error": "…"}` —
rejected, with an explanation in the program's language: unknown wheel, no
`source`, an angle no road wheel has (camber above 15°, toe above 10°), caster
on a rear wheel.

## Examples

From the command line:

```bash
curl -X POST http://127.0.0.1:8700/api/live/sample \
  -H 'Content-Type: application/json' \
  -d '{"source":"test","wheel":"FL","camber":-0.5,"toe":0.1}'
```

Python, a batch for all wheels:

```python
import json, urllib.request

readings = [
    {"source": "rig", "wheel": w, "camber": c, "toe": t}
    for w, c, t in [("FL", -0.5, 0.10), ("FR", -0.4, 0.08), ("RL", -1.1, 0.12), ("RR", -1.0, 0.11)]
]
req = urllib.request.Request(
    "http://127.0.0.1:8700/api/live/sample",
    data=json.dumps(readings).encode(),
    headers={"Content-Type": "application/json"},
)
print(urllib.request.urlopen(req).read().decode())
```

## What matters for a home-made head's accuracy

- **The sensor's axis and the wheel's axis.** Camber is the tilt of the rim's
  plane. The head must rest on the rim flanges (three points or a bar), not on
  the tyre.
- **Zero offset.** Cheap MEMS accelerometers (MPU6050, ADXL345) have zero offsets
  of tens of mg — a degree or more. Calibrate by reversal: the device in one
  position, then turned 180° on the same support; half the sum is the offset.
  Factory-calibrated inclinometers (SCA100T, ADXL355) suit camber better — they
  cost more but are more stable with temperature.
- **Rim runout.** Two readings with the car rolled half a wheel turn between
  them; the mean is camber, half the difference is runout. This can be done in
  firmware, or the compensated value can be sent to the program.
- **Caster** can be sent already computed, or the sensor can send only camber —
  caster is then measured step by step with a steer sweep (with string and
  inclinometer, or with the phone).
