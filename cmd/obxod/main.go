package main

import (
	"flag"
	"fmt"
	"os"

	"obxod/internal/clienthello"
	"obxod/internal/divert"
	"obxod/internal/filter"
	"obxod/internal/ip"
	"obxod/internal/tcp"
	"obxod/internal/udp"
)

const maxPacket = 0xffff + 40

var voice = []filter.PortRange{{From: 19294, To: 19344}, {From: 50000, To: 50100}}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	count := flag.Int("n", 20, "packets to pass through before stopping")
	flag.Parse()

	outbound, err := filter.Outbound(voice)
	if err != nil {
		return err
	}

	h, err := divert.Open(outbound, divert.Modify)
	if err != nil {
		return err
	}
	defer h.Close()

	fmt.Println("driver opened, passing packets through unchanged")

	buf := make([]byte, maxPacket)

	for i := 1; i <= *count; i++ {
		n, addr, err := h.Recv(buf)
		if err != nil {
			return err
		}

		fmt.Printf("%3d  %5d bytes  outbound=%v  %s\n", i, n, addr.Outbound(), describe(buf[:n]))

		if err := h.Send(buf[:n], &addr); err != nil {
			return err
		}
	}

	return nil
}

func describe(packet []byte) string {
	outer, err := ip.Parse(packet)
	if err != nil {
		return "not ipv4"
	}

	if outer.Protocol == ip.ProtocolUDP {
		datagram, err := udp.Parse(outer.Payload)
		if err != nil {
			return "udp we cannot read"
		}

		return fmt.Sprintf("udp to %d, %d bytes", datagram.DstPort, len(datagram.Payload))
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return "tcp we cannot read"
	}

	hello, err := clienthello.Parse(segment.Payload)
	if err != nil {
		return fmt.Sprintf("tcp to %d, %d bytes", segment.DstPort, len(segment.Payload))
	}

	name, err := hello.ServerName()
	if err != nil {
		return fmt.Sprintf("tcp to %d, hello without a name", segment.DstPort)
	}

	return fmt.Sprintf("tcp to %d, hello for %s", segment.DstPort, name.Host)
}
