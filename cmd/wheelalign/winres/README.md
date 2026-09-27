# Ресурсы Windows

Значок, манифест и сведения о версии для `wheelalign.exe`. Уже собраны в
`../rsrc_windows_amd64.syso`, который `go build` подхватывает сам — для
обычной сборки здесь ничего делать не нужно.

Пересобрать после изменений (нужны Python с Pillow и `windres` из MinGW):

```sh
cd cmd/wheelalign/winres
python make_icon.py
windres -c 65001 -O coff -F pe-x86-64 -i app.rc -o ../rsrc_windows_amd64.syso
```

Манифест объявляет поддержку высокого DPI (PerMonitorV2): без него окно на
экранах с масштабом 125–200 % выглядит размытым.
