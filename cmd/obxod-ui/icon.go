package main

import _ "embed"

//go:embed icon.png
var iconPNG []byte

// The windows tray wants an ico; a bare png shows blank there. The window icon
// stays png since gio decodes it itself.
//
//go:embed icon.ico
var iconICO []byte
