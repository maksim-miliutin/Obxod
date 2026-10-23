package seal

import (
	"encoding/binary"
	"errors"

	"obxod/internal/checksum"
	"obxod/internal/ip"
	"obxod/internal/udp"
)

const (
	ipLengthAt    = 2
	udpHeaderLen  = 8
	udpLengthAt   = 4
	udpChecksumAt = 6
)

var ErrNotUDP = errors.New("seal: only udp packets are made into datagrams")

// Datagram returns the packet carrying another payload, with every length and
// checksum made true again. A datagram has no sequence number and no window, so
// there is nothing here to shift or to overlap.
func Datagram(packet []byte, payload []byte) ([]byte, error) {
	return withSum(packet, payload, false)
}

// SpoiledDatagram is Datagram with a deliberately wrong udp checksum: the copy
// passes the routers and an inspector, but the destination drops it. A decoy must
// die before the server, or the server answers a connection the client never opened.
func SpoiledDatagram(packet []byte, payload []byte) ([]byte, error) {
	return withSum(packet, payload, true)
}

func withSum(packet []byte, payload []byte, spoil bool) ([]byte, error) {
	outer, err := ip.Parse(packet)
	if err != nil {
		return nil, err
	}

	if outer.Protocol != ip.ProtocolUDP {
		return nil, ErrNotUDP
	}

	if _, err := udp.Parse(outer.Payload); err != nil {
		return nil, err
	}

	made := make([]byte, 0, outer.HeaderLen+udpHeaderLen+len(payload))
	made = append(made, packet[:outer.HeaderLen+udpHeaderLen]...)
	made = append(made, payload...)

	datagram := made[outer.HeaderLen:]

	binary.BigEndian.PutUint16(made[ipLengthAt:ipLengthAt+2], uint16(len(made)))
	binary.BigEndian.PutUint16(datagram[udpLengthAt:udpLengthAt+2], uint16(len(datagram)))

	binary.BigEndian.PutUint16(made[ipChecksumAt:ipChecksumAt+2], checksum.IPv4(made[:outer.HeaderLen]))

	var src, dst [4]byte

	copy(src[:], packet[12:16])
	copy(dst[:], packet[16:20])

	sum := checksum.UDP(src, dst, datagram)

	if spoil {
		sum = ^sum
		if sum == 0 {
			// Zero in a udp datagram means "no checksum", which the server would accept;
			// any other wrong value keeps the field present and failing.
			sum = 0x0001
		}
	}

	binary.BigEndian.PutUint16(datagram[udpChecksumAt:udpChecksumAt+2], sum)

	return made, nil
}
