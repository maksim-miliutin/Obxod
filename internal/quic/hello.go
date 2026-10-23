package quic

// clientHello builds the smallest TLS 1.3 ClientHello that still carries a server
// name where a dpi looks for it: one cipher suite, no key share, and the three
// extensions a quic hello would look wrong without. The random is left zero: a
// decoy that dies before the server needs no entropy, only the name.
func clientHello(name string) []byte {
	var ext []byte
	ext = append(ext, serverName(name)...)
	ext = append(ext, supportedVersions()...)
	ext = append(ext, transportParameters()...)

	var body []byte
	body = append(body, 0x03, 0x03)             // legacy version
	body = append(body, make([]byte, 32)...)    // random
	body = append(body, 0x00)                   // empty session id
	body = append(body, 0x00, 0x02, 0x13, 0x01) // one cipher suite: aes-128-gcm-sha256
	body = append(body, 0x01, 0x00)             // one compression method: none
	body = appendPrefixed(body, ext)

	out := []byte{0x01} // ClientHello

	return append(appendUint24(out, len(body)), body...)
}

func serverName(name string) []byte {
	entry := appendPrefixed([]byte{0x00}, []byte(name)) // host_name, then the host

	return extension(0x0000, appendPrefixed(nil, entry))
}

func supportedVersions() []byte {
	return extension(0x002b, []byte{0x02, 0x03, 0x04}) // list of one: TLS 1.3
}

func transportParameters() []byte {
	return extension(0x0039, nil)
}

func extension(kind uint16, body []byte) []byte {
	return appendPrefixed([]byte{byte(kind >> 8), byte(kind)}, body)
}

func appendPrefixed(dst, body []byte) []byte {
	dst = append(dst, byte(len(body)>>8), byte(len(body)))

	return append(dst, body...)
}

func appendUint24(dst []byte, n int) []byte {
	return append(dst, byte(n>>16), byte(n>>8), byte(n))
}
