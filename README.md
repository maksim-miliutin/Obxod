🇷🇺 [По-русски](README.ru.md)

# Obxod

Bypasses DPI blocking on Windows. Opens sites a provider blocks **by host name**,
by swapping the name in the TLS handshake on the fly through the WinDivert driver.
One project, two forms: `obxod-ui` (a window) and `obxod` (a console tool).

## What it does

- A single `obxod-ui.exe` — the driver is embedded and unpacked next to it on
  first run.
- A button turns the bypass on and off.
- A method picker with a few tested presets; different providers need different
  ones, and the chosen one is remembered.
- Your own sites: add or remove domains beyond the built-in list.
- A speed line, a logs tab (with Copy), and an optional start with Windows.

## How to use

1. Run `obxod-ui.exe` and allow administrator rights — the driver needs them.
2. Click **Включить** (On). Open a blocked site; it should load.
3. If it does not, pick another **method** — providers differ.
4. To cover more sites, add their domains on the **Сайты** tab.

On first run Windows may warn "unknown publisher" (the program is unsigned) —
choose More info, then Run anyway.

## Methods

Six methods form a ladder from light to heavy, each adding one trick on top of the
last: **Лёгкий** (Light, a plain name swap), **Разрез** (Cut, plus a split through
the name), **Разнобой** (Disorder, plus the halves reversed), **Обманка ya.ru**
(ya.ru decoy — a whole decoy hello sent ahead of the real one, the recommended
default, tuned for a modern DPI), **Внахлёст** (Overlay, the decoy laid over the
stream as well) and **Вразнобой** (Scatter, pure fragmentation with no name swap).
Start at the default; if a site stays blocked, climb to a heavier one — no single
method works everywhere.

## What it cannot do

It opens what is blocked **by name**. It does **not**:

- speed up what is not blocked — a slow download from a site that is not filtered
  is your channel, not censorship;
- defeat blocking **by IP** — some services (Telegram, parts of Instagram) are
  blocked by address, and name swapping cannot reach them. For those use a VPN or
  the app's own proxy (Telegram has a built-in MTProxy setting).

Tell which kind a site is with `obxod.exe -check example.com`.

## Building the window

Needs Go, a C compiler (w64devkit, say) and cgo on:

    go env -w CGO_ENABLED=1
    go install github.com/akavel/rsrc@latest
    go generate ./cmd/obxod-ui
    go build -ldflags -H=windowsgui -o obxod-ui.exe ./cmd/obxod-ui

`WinDivert.dll`, `WinDivert64.sys` and `ACTIVE_DISCORD_UDP.bin` must sit next to
`cmd/obxod-ui/main.go`; they are embedded via `//go:embed`.

## The console

    go build ./cmd/obxod
    obxod.exe -check discord.com
    obxod.exe -wet -noquic -rules rules.txt

Run `obxod.exe` with no arguments for the flags; the rule syntax is in the code.
