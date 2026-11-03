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

Two methods, both confirmed against a live aggressive DPI. **Лёгкий** (Light) just
swaps the site name in place and ages the copy — quick, and enough when the DPI is
simple. **Обманка ya.ru** (ya.ru decoy, the default) sends a ready ya.ru hello as a
decoy and cuts the real one behind it — the workhorse for a modern DPI, like
Zapret's ALT3. It runs on the default; switch to Лёгкий when you want less overhead.

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
