package main

import _ "embed"

// Sent as a decoy and laid over the stream: a real allowed hello the dpi waves
// through. ya.ru, a ru-whitelisted site, is what the current dpi lets by.
//
//go:embed tls_clienthello_ya_ru.bin
var fakeHello []byte

//go:embed tls_clienthello_www_google_com.bin
var googleHello []byte

//go:embed tls_clienthello_www_sferum_ru.bin
var sferumHello []byte

//go:embed quic_initial_www_google_com.bin
var fakeQUICInitial []byte

// An unknown or empty key falls back to the ya.ru decoy the dpi waves through.
func hello(key string) []byte {
	switch key {
	case "google":
		return googleHello
	case "sferum":
		return sferumHello
	default:
		return fakeHello
	}
}
