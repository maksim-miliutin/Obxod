package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"obxod/internal/attempt"
	"obxod/internal/cut"
	"obxod/internal/divert"
	"obxod/internal/filter"
	"obxod/internal/forge"
	"obxod/internal/hello"
	"obxod/internal/ip"
	"obxod/internal/link"
	"obxod/internal/replies"
	"obxod/internal/rules"
	"obxod/internal/sweep"
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
	sweepHost := flag.String("sweep", "", "try way after way for this site until one stops the retries")
	seconds := flag.Int("seconds", 12, "how long to give each way while sweeping")
	noQUIC := flag.Bool("noquic", false, "drop outgoing quic so the browser falls back to tcp, which we can unblock")
	wet := flag.Bool("wet", false, "actually send copies; off by default, only reports")
	flag.Parse()

	var hunt *sweep.Sweep

	base, err := plan(ruleTexts, *hosts, uint8(*ttl), uint32(*badseq), *badsum, *decoy, *where)

	if *sweepHost == "" && err != nil {
		return err
	}

	set := base

	if *sweepHost != "" {
		host := strings.ToLower(*sweepHost)
		hunt = sweep.New(host, sweep.Candidates(host), time.Duration(*seconds)*time.Second, time.Now())

		// The rules that already work stay on: without them the site never gets
		// far enough to ask for the one being swept.
		set = withCandidate(base, hunt.Current())
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

	tries := attempt.New(20 * time.Second)

	eyes, err := divert.Open(filter.Replies(), divert.Sniff)
	if err != nil {
		return fmt.Errorf("cannot watch replies: %w", err)
	}
	defer eyes.Close()

	var guard sync.Mutex

	health := link.New()

	go func() {
		watch := replies.Watch{
			Seen: func(total int) {
				if total%200 == 1 {
					fmt.Printf("  replies watched: %d so far\n", total)
				}
			},
			Reset: func(port uint16) {
				host, known := tries.HostOn(port)
				if !known {
					fmt.Printf("  reset on port %d, which we never touched\n", port)

					return
				}

				fmt.Printf("  %s on port %d: reset by the other side\n", host, port)
				health.Forget(port)

				guard.Lock()
				defer guard.Unlock()

				if hunt != nil && hunt.Host() == host {
					hunt.Saw(true)
				}
			},
			Data: func(port uint16) {
				host, first := health.Data(port, time.Now())
				if !first {
					return
				}

				fmt.Printf("  %s on port %d: the server answered\n", host, port)
			},
		}

		if err := watch.Run(eyes); err != nil {
			fmt.Fprintf(os.Stderr, "watching replies stopped: %v\n", err)
		}
	}()

	fmt.Printf("%s\n", mode)

	for _, r := range set {
		fmt.Printf("  %s: %s\n", r.Host, describe(r))
	}

	buf := make([]byte, maxPacket)

	var dropped int
	var quiet int

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

		guard.Lock()

		if hunt != nil {
			if verdict, done := hunt.Judge(time.Now()); done {
				fmt.Printf("  %s: %s\n", describe(hunt.Current()), verdict)

				if verdict == sweep.Worked {
					fmt.Printf("\nthis one works, keeping it:\n  -rule \"%s=%s\"\n", hunt.Host(), asRule(hunt.Current()))

					hunt = nil
				} else if !hunt.Next(time.Now()) {
					return fmt.Errorf("nothing left to try for %s", hunt.Host())
				} else {
					quiet++

					if verdict != sweep.Quiet {
						quiet = 0
					}

					if quiet == 4 {
						fmt.Printf("\n%s has not asked for anything yet. Give the rules that already work with -rule, or the site never gets this far.\n\n", hunt.Host())
					}

					fmt.Printf("trying %s, %d left\n", describe(hunt.Current()), hunt.Left())
					set = withCandidate(base, hunt.Current())
				}
			}
		}

		for _, gone := range health.WentQuiet(time.Now(), 8*time.Second) {
			fmt.Printf("  %s on port %d: answered %d times then went silent for %s, the connection was killed\n",
				gone.Host, gone.Port, gone.Packets, gone.Silence.Round(time.Second))
		}

		guard.Unlock()

		sent, err := forward(h, packet, &addr, set, tries, health, hunt, *wet)
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
func forward(h sender, packet []byte, addr *divert.Addr, set rules.Set, tries *attempt.Tracker, health *link.Health, watcher *sweep.Sweep, wet bool) (bool, error) {
	found, ok := hello.Found(packet)
	if !ok {
		return false, nil
	}

	r, ok := set.For(found.Host)
	if !ok {
		return false, nil
	}

	repeat := tries.Saw(found.Host, found.SrcPort, found.Seq, time.Now()) == attempt.Again

	if repeat {
		fmt.Printf("  %s: asking again, so this way is not getting through\n", found.Host)
	}

	health.Hello(found.Host, found.SrcPort, time.Now())

	if watcher != nil && watcher.Host() == found.Host {
		watcher.Saw(repeat)
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

func asRule(r rules.Rule) string {
	var ways []string

	if r.TTL != 0 {
		ways = append(ways, fmt.Sprintf("ttl:%d", r.TTL))
	}

	if r.BadSeq != 0 {
		ways = append(ways, fmt.Sprintf("badseq:%d", r.BadSeq))
	}

	if r.BadSum {
		ways = append(ways, "badsum")
	}

	if r.Decoy != "" {
		ways = append(ways, "decoy:"+r.Decoy)
	}

	if r.Cut != "" {
		ways = append(ways, "cut:"+r.Cut)
	}

	return strings.Join(ways, ",")
}

// withCandidate puts one rule in place of whatever covered the same host, leaving
// every other rule alone.
func withCandidate(base rules.Set, r rules.Rule) rules.Set {
	out := rules.Set{r}

	for _, had := range base {
		if had.Host != r.Host {
			out = append(out, had)
		}
	}

	return out
}
