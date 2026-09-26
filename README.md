# Obxod

Обход DPI-блокировок под Windows. Открывает сайты и сервисы, которые провайдер
режет по имени хоста: на ходу подменяет имя в TLS-приветствии через драйвер
WinDivert. Один проект, две формы — `obxod-ui` (окно) и `obxod` (консоль).

## Окно (obxod-ui)

Скачайте один файл `obxod-ui.exe` и запустите. Windows спросит права
администратора (драйверу они нужны) — согласитесь. Драйвер вшит в exe и
распаковывается рядом при первом запуске.

Вкладки:
- **Обход** — кнопка вкл/выкл, выбор способа, строка скорости.
- **Сайты** — свои домены (добавить/убрать) и список тех, что обходятся всегда.
- **Логи** — что делает движок, с кнопкой «Скопировать».
- **О программе** — коротко о программе.

Способ не подошёл — попробуйте другой: у разных провайдеров работают разные.
Обход открывает заблокированное по имени; он не ускоряет то, что не блокируют,
и не пробивает блокировку по IP.

При первом запуске Windows может сказать «неизвестный издатель» (программа без
подписи) — нажмите «Подробнее», затем «Всё равно запустить».

## Сборка окна

Нужен Go, C-компилятор (например w64devkit) и включённый cgo:

    go env -w CGO_ENABLED=1
    go install github.com/akavel/rsrc@latest
    go generate ./cmd/obxod-ui
    go build -ldflags -H=windowsgui -o obxod-ui.exe ./cmd/obxod-ui

Рядом с `cmd/obxod-ui/main.go` должны лежать `WinDivert.dll`, `WinDivert64.sys`
и `ACTIVE_DISCORD_UDP.bin` — они встраиваются через `//go:embed`.

## Консоль (obxod)

    go build ./cmd/obxod
    obxod.exe -check discord.com               # режут ли сайт и как
    obxod.exe -wet -noquic -rules rules.txt    # включить обход по правилам

Флаги — запустите `obxod.exe` без аргументов; синтаксис правил — в коде.

---

# Obxod (English)

Bypasses DPI blocking on Windows. Opens sites a provider blocks by host name, by
swapping the name in the TLS handshake on the fly through the WinDivert driver.
One project, two forms: `obxod-ui` (a window) and `obxod` (a console tool).

## The window (obxod-ui)

Download the single `obxod-ui.exe` and run it. Windows asks for administrator
rights (the driver needs them) — allow it. The driver is embedded in the exe and
unpacked next to it on first run.

Tabs:
- **Обход (Bypass)** — an on/off button, a method picker, a speed line.
- **Сайты (Sites)** — your own domains (add/remove) and the always-on list.
- **Логи (Logs)** — what the engine is doing, with a Copy button.
- **О программе (About)** — a short note.

If a method does not work, try another: providers differ. The bypass opens what
is blocked by name; it does not speed up what is not blocked, and it does not
defeat blocking by IP.

On first run Windows may warn "unknown publisher" (the program is unsigned) —
choose More info, then Run anyway.

## Building the window

Needs Go, a C compiler (w64devkit, say) and cgo on:

    go env -w CGO_ENABLED=1
    go install github.com/akavel/rsrc@latest
    go generate ./cmd/obxod-ui
    go build -ldflags -H=windowsgui -o obxod-ui.exe ./cmd/obxod-ui

`WinDivert.dll`, `WinDivert64.sys` and `ACTIVE_DISCORD_UDP.bin` must sit next to
`cmd/obxod-ui/main.go`; they are embedded via `//go:embed`.

## The console (obxod)

    go build ./cmd/obxod
    obxod.exe -check discord.com
    obxod.exe -wet -noquic -rules rules.txt

Run `obxod.exe` with no arguments for the flags; the rule syntax is in the code.
