package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"obxod/internal/cut"
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
	ttl := flag.Int("ttl", 0, "hops the forged copy may live; zero leaves the original ttl alone")
	badseq := flag.Uint("badseq", 0, "shift the copy's sequence number by this much")
	badsum := flag.Bool("badsum", false, "give the copy a wrong tcp checksum")
	where := flag.String("cut", "", "split the real hello: name (through the middle of the host name), after (just past it), start (near the record start)")
	wet := flag.Bool("wet", false, "actually send copies; off by default, only reports")
	flag.Parse()

	if *host == "" {
		return fmt.Errorf("give -host, e.g. -host gateway.discord.gg")
	}

	// An untouched copy is a second identical hello: the server sees the payload
	// twice and drops the connection, which looks like the bypass making things worse.
	if *wet && *ttl == 0 && *badseq == 0 && !*badsum && *where == "" {
		return fmt.Errorf("give -ttl, -badseq, -badsum or -cut: a copy with nothing wrong would break the connection")
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

	fmt.Printf("watching for %s, %s, %s\n", *host, spoils(uint8(*ttl), uint32(*badseq), *badsum), mode)

	buf := make([]byte, maxPacket)

	for {
		n, addr, err := h.Recv(buf)
		if err != nil {
			return err
		}

		packet := buf[:n]

		sent, err := forward(h, packet, &addr, *host, uint8(*ttl), uint32(*badseq), *badsum, *where, *wet)
		if err != nil {
			return err
		}

		if sent {
			continue
		}

		if err := h.Send(packet, &addr); err != nil {
			return err
		}
	}
}

// forward returns true when it already put the packet on the wire itself, which
// happens for a cut: the original must not follow its own halves.
func forward(h *divert.Handle, packet []byte, addr *divert.Addr, host string, ttl uint8, badseq uint32, badsum bool, where string, wet bool) (bool, error) {
	found, ok := hello.Found(packet)
	if !ok || !strings.EqualFold(found.Host, host) {
		return false, nil
	}

	if where != "" {
		return split(h, packet, addr, found, where, wet)
	}

	copied, err := forge.Copy(packet, forge.Recipe{TTL: ttl, SeqDelta: badseq, BadSum: badsum})
	if err != nil {
		fmt.Printf("  %s: cannot copy: %v\n", found.Host, err)

		return false, nil
	}

	if !wet {
		fmt.Printf("  %s: would send a %d byte copy (%s)\n", found.Host, len(copied), spoils(ttl, badseq, badsum))

		return false, nil
	}

	fmt.Printf("  %s: copy sent ahead (%s)\n", found.Host, spoils(ttl, badseq, badsum))

	return false, h.Send(copied, addr)
}

func split(h *divert.Handle, packet []byte, addr *divert.Addr, found hello.Outgoing, where string, wet bool) (bool, error) {
	point, err := pointFor(found, where)
	if err != nil {
		return false, err
	}

	first, second, err := cut.At(packet, point)
	if err != nil {
		fmt.Printf("  %s: cannot split: %v\n", found.Host, err)

		return false, nil
	}

	if !wet {
		fmt.Printf("  %s: would split into %d and %d bytes at %s\n", found.Host, len(first), len(second), where)

		return false, nil
	}

	fmt.Printf("  %s: split into %d and %d bytes at %s\n", found.Host, len(first), len(second), where)

	if err := h.Send(first, addr); err != nil {
		return false, err
	}

	return true, h.Send(second, addr)
}

func spoils(ttl uint8, badseq uint32, badsum bool) string {
	var named []string

	if ttl != 0 {
		named = append(named, fmt.Sprintf("ttl %d", ttl))
	}

	if badseq != 0 {
		named = append(named, fmt.Sprintf("badseq %d", badseq))
	}

	if badsum {
		named = append(named, "badsum")
	}

	if len(named) == 0 {
		return "nothing spoiled"
	}

	return strings.Join(named, " + ")
}

func pointFor(found hello.Outgoing, where string) (int, error) {
	switch where {
	case "name":
		// Through the middle of the name: neither packet holds it whole, which is
		// what defeats an inspector that reads packets one by one.
		return found.NameStart + len(found.Host)/2, nil
	case "after":
		return found.NameEnd, nil
	case "start":
		return 2, nil
	}

	return 0, fmt.Errorf("unknown -cut %q: use name, after or start", where)
}
