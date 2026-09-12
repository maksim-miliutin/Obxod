package divert

import (
	"encoding/binary"
	"testing"
)

func addrWith(bits ...int) Addr {
	var a Addr

	var word uint32
	for _, b := range bits {
		word |= 1 << b
	}

	binary.LittleEndian.PutUint32(a[8:12], word)

	return a
}

func TestAddrFlags(t *testing.T) {
	cases := []struct {
		name string
		addr Addr
		want map[string]bool
	}{
		{
			"outbound packet off the wire",
			addrWith(bitOutbound),
			map[string]bool{"outbound": true},
		},
		{
			"loopback is outbound too",
			addrWith(bitOutbound, bitLoopback),
			map[string]bool{"outbound": true, "loopback": true},
		},
		{
			"sniffed reply",
			addrWith(bitSniffed),
			map[string]bool{"sniffed": true},
		},
		{
			"injected by another driver",
			addrWith(bitOutbound, bitImpostor),
			map[string]bool{"outbound": true, "impostor": true},
		},
		{
			"version six",
			addrWith(bitOutbound, bitIPv6),
			map[string]bool{"outbound": true, "ipv6": true},
		},
		{
			"nothing set",
			Addr{},
			map[string]bool{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := map[string]bool{
				"sniffed":  c.addr.Sniffed(),
				"outbound": c.addr.Outbound(),
				"loopback": c.addr.Loopback(),
				"impostor": c.addr.Impostor(),
				"ipv6":     c.addr.IPv6(),
			}

			for name, value := range got {
				if value != c.want[name] {
					t.Errorf("%s = %v, want %v", name, value, c.want[name])
				}
			}
		})
	}
}

func TestAddrLeavesTheRestAlone(t *testing.T) {
	var a Addr

	binary.LittleEndian.PutUint64(a[0:8], 0x1122334455667788)
	a[16] = 7
	a[79] = 0xff

	if a.Outbound() {
		t.Error("Outbound reads a byte that belongs to the timestamp or the union")
	}

	if binary.LittleEndian.Uint64(a[0:8]) != 0x1122334455667788 {
		t.Error("the timestamp changed while flags were read")
	}
}

func TestAddrIsEightyBytes(t *testing.T) {
	// The driver writes exactly this many; a shorter array would corrupt the stack.
	if len(Addr{}) != 80 {
		t.Errorf("Addr is %d bytes, want 80", len(Addr{}))
	}
}

func TestRecvNeedsSomewhereToPutThePacket(t *testing.T) {
	var h Handle

	if _, _, err := h.Recv(nil); err != ErrEmptyBuffer {
		t.Errorf("err = %v, want %v", err, ErrEmptyBuffer)
	}
}

func TestSendNeedsAPacket(t *testing.T) {
	var h Handle
	var a Addr

	if err := h.Send(nil, &a); err != ErrNoPacket {
		t.Errorf("err = %v, want %v", err, ErrNoPacket)
	}
}
