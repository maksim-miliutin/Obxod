# Obxod :)

Bypasses DPI blocking of TLS and voice traffic on Windows. Intercepts outgoing
packets through WinDivert and sends forged copies ahead of the real ones, so the
inspector reads the copy and the server reads the original.

## Build

    go build ./cmd/obxod

## Run

Needs Windows, administrator privileges, and two files from WinDivert 2.2 sitting
next to the executable:

    WinDivert.dll
    WinDivert64.sys

Both come from the x64 folder of the official release at
https://reqrypt.org/windivert.html and are signed by its author. The driver
installs itself the first time the program opens it, which is what the
administrator privileges are for. To remove it, delete both files and reboot.

    obxod.exe

![](assets/smile.png)
