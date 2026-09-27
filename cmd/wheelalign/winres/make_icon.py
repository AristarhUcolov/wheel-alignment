"""Рисует значок программы (icon.ico) — тот же, что в заголовке интерфейса.

Нужен только при изменении значка: готовый icon.ico и собранный из него
rsrc_windows_amd64.syso лежат в репозитории, и обычная сборка `go build`
их просто подхватывает. Как пересобрать — см. README.md в этом каталоге.
"""
from PIL import Image, ImageDraw

S = 1024  # рисуем крупно и уменьшаем — так края получаются сглаженными


def draw():
    im = Image.new('RGBA', (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle([0, 0, S - 1, S - 1], radius=int(S * 0.22), fill=(12, 19, 33, 255))
    c, r, w = S / 2, S * 0.34, int(S * 0.075)
    d.ellipse([c - r, c - r, c + r, c + r], outline=(232, 238, 248, 255), width=w)
    rd = S * 0.11
    d.ellipse([c - rd, c - rd, c + rd, c + rd], fill=(232, 238, 248, 255))
    lw = int(S * 0.08)
    a, b = S * 0.25, S * 0.75
    d.line([(a, b), (b, a)], fill=(43, 212, 106, 255), width=lw)
    for x, y in [(a, b), (b, a)]:
        d.ellipse([x - lw / 2, y - lw / 2, x + lw / 2, y + lw / 2], fill=(43, 212, 106, 255))
    return im


if __name__ == '__main__':
    big = draw()
    sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256]
    big.resize((256, 256), Image.LANCZOS).save('icon.ico', sizes=[(s, s) for s in sizes])
    big.resize((512, 512), Image.LANCZOS).save('icon-512.png')
    print('icon.ico, icon-512.png')
