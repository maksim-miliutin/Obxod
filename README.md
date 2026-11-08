🇷🇺 [По-русски](README.ru.md)

# Obxod

Bypasses DPI blocking on Windows: Discord, YouTube, Telegram and more. The engine is
pure Go with no dependencies; it desyncs the TLS handshake through the WinDivert driver
so a provider's name-based block never sees a clean hello. One project, two forms:
`obxod-ui` (a window) and `obxod` (a console tool).

## What it does

- A single `obxod-ui.exe` — the driver and the administrator manifest are embedded.
- A button turns the bypass on and off.
- **Auto-pick**: the "Подбор" tab runs each method against a live probe and keeps the
  one that works best on your provider, so you do not have to guess.
- Fifteen tested methods; switching the method changes it **without dropping
  connections** — the driver stays open.
- A tray icon: closing the window hides it there and the bypass keeps running.
- Ignore an app by name or folder, so obxod leaves its traffic alone — handy for a VPN
  like Radmin that punches NAT through STUN and would otherwise clash.
- Your own sites beyond the built-in list, a speed line, a logs tab (with Copy),
  start-with-Windows, Russian and English.

## How to use

1. Run `obxod-ui.exe` and allow administrator rights — the driver needs them.
2. Click **Включить** (On). Open a blocked site; it should load.
3. If a site is stubborn, open the **Подбор** tab and let it pick the best method, or
   choose one by hand on the **Методы** tab.
4. To cover more sites, add their domains on the **Сайты** tab.

Run the auto-pick when you are **not** actively using a site: it tries methods on live
traffic and can interrupt an open session. Let it settle, then use the site.

On first run Windows may warn "unknown publisher" (the program is unsigned) — choose
More info, then Run anyway.

## Methods

Fifteen methods, each confirmed against a live aggressive DPI, grouped by family:
**OV** lays a recorded hello over the stream (seqovl), **DC** sends a decoy hello ahead
(with checksum, ack or sequence tricks), **SW** swaps the site name in place, **FR**
fragments the hello. Providers differ and a DPI shifts over time, so the auto-pick is
the easy path; each method's description in the window says what it does. QUIC (http/3)
is faked for the sites that need it (Discord images and the like), and Discord voice
gets its own UDP bypass.

## What it cannot do

It opens what is blocked **by host name**. It does **not**:

- speed up what is not blocked — a slow download from a site that is not filtered is
  your channel, not censorship;
- defeat blocking **by IP** — some services (Telegram, parts of Instagram) are blocked
  by address, and name swapping cannot reach them. For those use a VPN or the app's own
  proxy (Telegram has a built-in MTProxy setting).

Tell which kind a site is with `obxod.exe -check example.com`.

## Building the window

Needs Go, a C compiler (w64devkit, say) and cgo on:

    go env -w CGO_ENABLED=1
    go install github.com/akavel/rsrc@latest
    go generate ./cmd/obxod-ui
    go build -ldflags -H=windowsgui -o obxod-ui.exe ./cmd/obxod-ui

`WinDivert.dll`, `WinDivert64.sys` and the other embedded assets (the recorded UDP, the
icon, the fonts) must sit next to `cmd/obxod-ui/main.go`; they are pulled in via
`//go:embed`.

## The console

    go build ./cmd/obxod
    obxod.exe -check discord.com
    obxod.exe -wet -noquic -rules rules.txt

Run `obxod.exe` with no arguments for the flags; the rule syntax is in the code.
