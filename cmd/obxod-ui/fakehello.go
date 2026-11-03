package main

import _ "embed"

// Sent as a decoy and laid over the stream: a real allowed hello the dpi waves
// through. ya.ru, a ru-whitelisted site, is what the current dpi lets by.
//
//go:embed tls_clienthello_ya_ru.bin
var fakeHello []byte

//go:embed quic_initial_www_google_com.bin
var fakeQUICInitial []byte
