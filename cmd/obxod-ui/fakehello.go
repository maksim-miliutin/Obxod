package main

import _ "embed"

// Laid over the stream by the overlay way: a real allowed hello the dpi waves
// through. A ru-whitelisted site (sferum) passes more surely here than google.
//
//go:embed tls_clienthello_www_sferum_ru.bin
var fakeHello []byte

//go:embed quic_initial_www_google_com.bin
var fakeQUICInitial []byte
