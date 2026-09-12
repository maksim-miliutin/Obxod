package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"obxod/internal/divert"
	"obxod/internal/filter"
	"obxod/internal/forge"
	"obxod/internal/hello"
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
	host := flag.String("host", "", "send a forged copy ahead of hellos for this site")
	ttl := flag.Int("ttl", 4, "hops the forged copy may live")
	wet := flag.Bool("wet", false, "actually send copies; off by default, only reports")
	flag.Parse()

	if *host == "" {
		return fmt.Errorf("give -host, e.g. -host gateway.discord.gg")
	}

	outbound, err := filter.Outbound(voice)
	if err != nil {
		return err
	}

	h, err := divert.Open(outbound, divert.Modify)
	if err != nil {
		return err
	}
	defer h.Close()

	mode := "dry run, copies are only reported"
	if *wet {
		mode = "sending copies"
	}

	fmt.Printf("watching for %s, ttl %d, %s\n", *host, *ttl, mode)

	buf := make([]byte, maxPacket)

	for {
		n, addr, err := h.Recv(buf)
		if err != nil {
			return err
		}

		packet := buf[:n]

		if err := forward(h, packet, &addr, *host, uint8(*ttl), *wet); err != nil {
			return err
		}

		if err := h.Send(packet, &addr); err != nil {
			return err
		}
	}
}

func forward(h *divert.Handle, packet []byte, addr *divert.Addr, host string, ttl uint8, wet bool) error {
	found, ok := hello.Found(packet)
	if !ok || !strings.EqualFold(found.Host, host) {
		return nil
	}

	copied, err := forge.Copy(packet, forge.Recipe{TTL: ttl})
	if err != nil {
		fmt.Printf("  %s: cannot copy: %v\n", found.Host, err)

		return nil
	}

	if !wet {
		fmt.Printf("  %s: would send a %d byte copy, ttl %d\n", found.Host, len(copied), ttl)

		return nil
	}

	fmt.Printf("  %s: copy sent ahead, ttl %d\n", found.Host, ttl)

	return h.Send(copied, addr)
}
