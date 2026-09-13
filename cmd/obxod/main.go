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
	"obxod/internal/ip"
	"obxod/internal/rules"
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
	var ruleTexts repeated

	flag.Var(&ruleTexts, "rule", "a rule per site, repeatable: host=way,way (ways: ttl:4 badseq:100000 badsum decoy decoy:name cut:name|after|start)")
	hosts := flag.String("host", "", "sites to work on, comma separated; a bare domain covers its subdomains, \"all\" covers everything")
	ttl := flag.Int("ttl", 0, "hops the forged copy may live; zero leaves the original ttl alone")
	badseq := flag.Uint("badseq", 0, "shift the copy's sequence number by this much")
	badsum := flag.Bool("badsum", false, "give the copy a wrong tcp checksum")
	decoy := flag.String("decoy", "", "put another host name in the copy; \"auto\" makes one of the right length")
	where := flag.String("cut", "", "split the real hello: name (through the middle of the host name), after (just past it), start (near the record start)")
	noQUIC := flag.Bool("noquic", false, "drop outgoing quic so the browser falls back to tcp, which we can unblock")
	wet := flag.Bool("wet", false, "actually send copies; off by default, only reports")
	flag.Parse()

	set, err := plan(ruleTexts, *hosts, uint8(*ttl), uint32(*badseq), *badsum, *decoy, *where)
	if err != nil {
		return err
	}

	// An untouched copy is a second identical hello: the server sees the payload
	// twice and drops the connection, which looks like the bypass making things worse.
	outbound, err := filter.Outbound(voice, *noQUIC)
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

	fmt.Printf("%s\n", mode)

	for _, r := range set {
		fmt.Printf("  %s: %s\n", r.Host, describe(r))
	}

	buf := make([]byte, maxPacket)

	var dropped int

	for {
		n, addr, err := h.Recv(buf)
		if err != nil {
			return err
		}

		packet := buf[:n]

		if *noQUIC && isQUIC(packet) {
			dropped++

			if dropped%50 == 1 {
				fmt.Printf("  quic dropped: %d so far\n", dropped)
			}

			continue
		}

		sent, err := forward(h, packet, &addr, set, *wet)
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

// sender is what the divert handle gives us, narrowed to the one call these take,
// so the order they send in can be checked without a driver.
type sender interface {
	Send(packet []byte, addr *divert.Addr) error
}

// forward returns true when it already put the packet on the wire itself, which
// happens for a cut: the original must not follow its own halves.
func forward(h sender, packet []byte, addr *divert.Addr, set rules.Set, wet bool) (bool, error) {
	found, ok := hello.Found(packet)
	if !ok {
		return false, nil
	}

	r, ok := set.For(found.Host)
	if !ok {
		return false, nil
	}

	// The decoy goes first and the real hello follows, cut or whole: an inspector
	// that reads the decoy and then finds no name in either half has nothing to match.
	if r.TTL != 0 || r.BadSeq != 0 || r.BadSum || r.Decoy != "" {
		if err := fake(h, packet, addr, found, r, wet); err != nil {
			return false, err
		}
	}

	if r.Cut != "" {
		return split(h, packet, addr, found, r.Cut, wet)
	}

	return false, nil
}

func fake(h sender, packet []byte, addr *divert.Addr, found hello.Outgoing, r rules.Rule, wet bool) error {
	recipe := forge.Recipe{TTL: r.TTL, SeqDelta: r.BadSeq, BadSum: r.BadSum}

	if r.Decoy != "" {
		name := r.Decoy
		if name == "auto" {
			name = decoyFor(found.Host)
		}

		if len(name) != len(found.Host) {
			return fmt.Errorf("decoy %q is %d bytes, the real name is %d: they must match", name, len(name), len(found.Host))
		}

		recipe.Name = name
		recipe.NameAt = found.NameStart
	}

	copied, err := forge.Copy(packet, recipe)
	if err != nil {
		fmt.Printf("  %s: cannot copy: %v\n", found.Host, err)

		return nil
	}

	if !wet {
		fmt.Printf("  %s: would send a %d byte copy (%s%s)\n", found.Host, len(copied), spoils(r.TTL, r.BadSeq, r.BadSum), wearing(recipe.Name))

		return nil
	}

	fmt.Printf("  %s: copy sent ahead (%s%s)\n", found.Host, spoils(r.TTL, r.BadSeq, r.BadSum), wearing(recipe.Name))

	return h.Send(copied, addr)
}

func split(h sender, packet []byte, addr *divert.Addr, found hello.Outgoing, where string, wet bool) (bool, error) {
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

func wearing(name string) string {
	if name == "" {
		return ""
	}

	return ", wearing " + name
}

// decoyFor builds a harmless name exactly as long as the real one, because the
// lengths inside a hello count the name and a copy must keep them true.
func decoyFor(host string) string {
	const base = "google.com"

	if len(host) < len(base)+2 {
		return strings.Repeat("a", len(host)-4) + ".com"
	}

	return strings.Repeat("x", len(host)-len(base)-1) + "." + base
}

func parseHosts(list string) []string {
	var watched []string

	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		watched = append(watched, strings.ToLower(name))
	}

	return watched
}

// watches takes a bare domain to cover its subdomains too, so one rule reaches
// gateway, updates and cdn without naming each.
func watches(watched []string, host string) bool {
	host = strings.ToLower(host)

	for _, rule := range watched {
		if rule == "all" || rule == host {
			return true
		}

		if strings.HasSuffix(host, "."+rule) {
			return true
		}
	}

	return false
}

// isQUIC reports a datagram heading for 443, which is how a browser tries HTTP/3
// before it settles for tcp.
func isQUIC(packet []byte) bool {
	outer, err := ip.Parse(packet)
	if err != nil || outer.Protocol != ip.ProtocolUDP {
		return false
	}

	datagram, err := udp.Parse(outer.Payload)
	if err != nil {
		return false
	}

	return datagram.DstPort == 443
}

type repeated []string

func (r *repeated) String() string {
	return strings.Join(*r, " ")
}

func (r *repeated) Set(text string) error {
	*r = append(*r, text)

	return nil
}

// plan turns whatever the command line carried into rules: either -rule entries,
// or the older single-strategy flags spread over the hosts in -host.
func plan(texts []string, hosts string, ttl uint8, badseq uint32, badsum bool, decoy string, where string) (rules.Set, error) {
	if len(texts) > 0 {
		return rules.ParseAll(texts)
	}

	var ways []string

	if ttl != 0 {
		ways = append(ways, fmt.Sprintf("ttl:%d", ttl))
	}

	if badseq != 0 {
		ways = append(ways, fmt.Sprintf("badseq:%d", badseq))
	}

	if badsum {
		ways = append(ways, "badsum")
	}

	if decoy != "" {
		ways = append(ways, "decoy:"+decoy)
	}

	if where != "" {
		ways = append(ways, "cut:"+where)
	}

	if len(ways) == 0 {
		return nil, fmt.Errorf("say what to do: -rule host=way,way or the -ttl, -badseq, -badsum, -decoy and -cut flags")
	}

	var texts2 []string

	for _, host := range parseHosts(hosts) {
		texts2 = append(texts2, host+"="+strings.Join(ways, ","))
	}

	if len(texts2) == 0 {
		return nil, fmt.Errorf("give -host, e.g. -host discord.com,discord.gg, or use -rule")
	}

	return rules.ParseAll(texts2)
}

func describe(r rules.Rule) string {
	var named []string

	if r.TTL != 0 {
		named = append(named, fmt.Sprintf("ttl %d", r.TTL))
	}

	if r.BadSeq != 0 {
		named = append(named, fmt.Sprintf("badseq %d", r.BadSeq))
	}

	if r.BadSum {
		named = append(named, "badsum")
	}

	if r.Decoy != "" {
		named = append(named, "decoy "+r.Decoy)
	}

	if r.Cut != "" {
		named = append(named, "cut at "+r.Cut)
	}

	return strings.Join(named, " + ")
}
