Bypasses DPI blocking of TLS and voice traffic on Windows. Intercepts outgoing
packets through WinDivert and sends forged copies ahead of the real ones, so the
inspector reads the copy and the server reads the original.

## Build

    go build ./cmd/obxod

## Run

Requires Windows, administrator privileges and the WinDivert driver.

![](assets/smile.png)
