package main

import _ "embed"

// Laid over the stream by the overlay way: the inspector reads this allowed
// name, the server drops it as bytes before the connection and keeps the real.
//
//go:embed tls_clienthello_www_google_com.bin
var fakeHello []byte
