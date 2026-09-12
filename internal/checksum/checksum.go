package checksum

import "encoding/binary"

const (
	protocolTCP = 6
	protocolUDP = 17
)

const (
	ipv4At = 10
	tcpAt  = 16
	udpAt  = 6
)

const skipNothing = -1

func IPv4(header []byte) uint16 {
	return fold(sum(header, ipv4At))
}

func TCP(src, dst [4]byte, segment []byte) uint16 {
	return fold(pseudo(src, dst, protocolTCP, len(segment)) + sum(segment, tcpAt))
}

func UDP(src, dst [4]byte, datagram []byte) uint16 {
	c := fold(pseudo(src, dst, protocolUDP, len(datagram)) + sum(datagram, udpAt))

	// Zero means "no checksum here" in UDP, so a real zero goes on the wire as all ones.
	if c == 0 {
		return 0xffff
	}

	return c
}

func Of(data []byte) uint16 {
	return fold(sum(data, skipNothing))
}

func pseudo(src, dst [4]byte, protocol uint8, length int) uint32 {
	var total uint32

	total += uint32(binary.BigEndian.Uint16(src[0:2]))
	total += uint32(binary.BigEndian.Uint16(src[2:4]))
	total += uint32(binary.BigEndian.Uint16(dst[0:2]))
	total += uint32(binary.BigEndian.Uint16(dst[2:4]))
	total += uint32(protocol)
	total += uint32(length)

	return total
}

// skipAt has to be even: the two bytes holding the checksum are read as zero.
func sum(data []byte, skipAt int) uint32 {
	var total uint32

	for i := 0; i+1 < len(data); i += 2 {
		if i == skipAt {
			continue
		}

		total += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}

	if len(data)%2 == 1 {
		total += uint32(data[len(data)-1]) << 8
	}

	return total
}

func fold(total uint32) uint16 {
	for total>>16 != 0 {
		total = total&0xffff + total>>16
	}

	return ^uint16(total)
}
