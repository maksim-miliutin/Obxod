package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"obxod/internal/bypass"
	"obxod/internal/cut"
	"obxod/internal/divert"
	"obxod/internal/filter"
	"obxod/internal/rules"
	"obxod/internal/sweep"
)

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
	tcpPorts := flag.String("ports", "443,2053,2083,2087,2096,8443", "tcp ports where hellos are looked for")
	patternFile := flag.String("pattern", "", "a recorded hello from an allowed site, used by overlap")
	seqovl := flag.Int("seqovl", 0, "how many bytes the overlap reaches back; zero means the whole pattern")
	silence := flag.Int("silence", 45, "seconds of silence after which a connection counts as killed")
	noQUIC := flag.Bool("noquic", false, "drop outgoing quic so the browser falls back to tcp, which we can unblock")
	wet := flag.Bool("wet", false, "actually send copies; off by default, only reports")
	flag.Parse()

	base, err := plan(ruleTexts, *hosts, uint8(*ttl), uint32(*badseq), *badsum, *decoy, *where)

	if *sweepHost == "" && err != nil {
		return err
	}

	var hunt *sweep.Sweep

	if *sweepHost != "" {
		host := strings.ToLower(*sweepHost)
		hunt = sweep.New(host, sweep.Candidates(host), time.Duration(*seconds)*time.Second, time.Now())
	}

	ports, err := parsePorts(*tcpPorts)
	if err != nil {
		return err
	}

	var pattern []byte

	if *patternFile != "" {
		raw, err := os.ReadFile(*patternFile)
		if err != nil {
			return fmt.Errorf("cannot read the pattern: %w", err)
		}

		pattern = spread(raw, *seqovl)

		fmt.Printf("pattern: %d bytes from %s, laid over %d bytes\n", len(raw), *patternFile, len(pattern))
	}

	outbound, err := filter.Outbound(filter.Ports{TCP: ports, Voice: voice, QUIC: *noQUIC})
	if err != nil {
		return err
	}

	watching, err := filter.Replies(ports)
	if err != nil {
		return err
	}

	wire, err := divert.Open(outbound, divert.Modify)
	if err != nil {
		return err
	}
	defer wire.Close()

	eyes, err := divert.Open(watching, divert.Sniff)
	if err != nil {
		return fmt.Errorf("cannot watch replies: %w", err)
	}
	defer eyes.Close()

	engine := bypass.New(bypass.Settings{
		Rules:    base,
		Hunt:     hunt,
		Pattern:  pattern,
		Silence:  time.Duration(*silence) * time.Second,
		DropQUIC: *noQUIC,
		Wet:      *wet,
		Report:   func(text string) { fmt.Println(text) },
	})

	return engine.Run(wire, eyes)
}

// A zero seqovl means the overlap reaches back over the whole recording.
func spread(raw []byte, seqovl int) []byte {
	if seqovl == 0 {
		return cut.Filler(raw, len(raw))
	}

	return cut.Filler(raw, seqovl)
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

	var spread []string

	for _, host := range parseHosts(hosts) {
		spread = append(spread, host+"="+strings.Join(ways, ","))
	}

	if len(spread) == 0 {
		return nil, fmt.Errorf("give -host, e.g. -host discord.com,discord.gg, or use -rule")
	}

	return rules.ParseAll(spread)
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

func parsePorts(list string) ([]uint16, error) {
	var ports []uint16

	for _, text := range strings.Split(list, ",") {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		port, err := strconv.ParseUint(text, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("port %q: %w", text, err)
		}

		ports = append(ports, uint16(port))
	}

	return ports, nil
}
