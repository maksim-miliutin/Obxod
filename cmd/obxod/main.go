package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"obxod/internal/bypass"
	"obxod/internal/cut"
	"obxod/internal/divert"
	"obxod/internal/filter"
	"obxod/internal/rules"
	"obxod/internal/sweep"
)

// Where discord opens a call. It moves between versions, so -voice exists.
const voicePorts = "19294-19344,50000-50100"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var ruleTexts repeated

	flag.Var(&ruleTexts, "rule", "a rule per site, repeatable: host=way,way (ways: "+rules.Ways()+")")
	rulesFile := flag.String("rules", "", "a file of rules, one per line; blank lines and lines starting with # are skipped")
	hosts := flag.String("host", "", "sites to work on, comma separated; a bare domain covers its subdomains, \"all\" covers everything")
	ttl := flag.Int("ttl", 0, "hops the forged copy may live; zero leaves the original ttl alone")
	badseq := flag.Int("badseq", 0, "shift the copy's sequence number by this much, either way")
	badsum := flag.Bool("badsum", false, "give the copy a wrong tcp checksum")
	decoy := flag.String("decoy", "", "put another host name in the copy; any length, the hello is rebuilt; \"auto\" makes one the same size")
	where := flag.String("cut", "", "split the real hello: name (through the middle of the host name), after (just past it), start (near the record start)")
	sweepHost := flag.String("sweep", "", "try way after way for this site until one stops the retries")
	seconds := flag.Int("seconds", 12, "how long to give each way while sweeping")
	tcpPorts := flag.String("ports", "443,2053,2083,2087,2096,8443", "tcp ports where hellos are looked for")
	patternFile := flag.String("pattern", "", "a recorded hello from an allowed site, used by overlap")
	fakeFile := flag.String("fake", "", "a recorded hello sent ahead in place of a forged copy, used by the fake way")
	voiceList := flag.String("voice", voicePorts, "udp port ranges where a call is opened, comma separated")
	voicedFile := flag.String("fakeudp", "", "a recorded voice datagram sent ahead, used by the fakeudp way")
	seqovl := flag.Int("seqovl", 0, "how many bytes the overlap reaches back; zero means the whole pattern")
	silence := flag.Int("silence", 45, "seconds of silence after which a connection counts as killed")
	noQUIC := flag.Bool("noquic", false, "drop outgoing quic so the browser falls back to tcp, which we can unblock")
	seen := flag.Bool("seen", false, "name every host no rule covers, once each, so the missing ones can be found")
	wet := flag.Bool("wet", false, "actually send copies; off by default, only reports")
	flag.Parse()

	if *rulesFile != "" {
		raw, err := os.ReadFile(*rulesFile)
		if err != nil {
			return fmt.Errorf("cannot read the rules: %w", err)
		}

		ruleTexts = append(ruleTexts, written(string(raw))...)
	}

	base, err := plan(asked{
		texts:  ruleTexts,
		hosts:  *hosts,
		ttl:    uint8(*ttl),
		badseq: int32(*badseq),
		badsum: *badsum,
		decoy:  *decoy,
		where:  *where,
	})

	if *sweepHost == "" && err != nil {
		return err
	}

	var hunt *sweep.Sweep

	if *sweepHost != "" {
		host := strings.ToLower(*sweepHost)
		hunt = sweep.New(host, sweep.Candidates(host), time.Duration(*seconds)*time.Second, time.Now())
	}

	voice, err := parseRanges(*voiceList)
	if err != nil {
		return err
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

	var recorded []byte

	if *fakeFile != "" {
		recorded, err = os.ReadFile(*fakeFile)
		if err != nil {
			return fmt.Errorf("cannot read the recorded hello: %w", err)
		}

		fmt.Printf("fake: %d bytes from %s\n", len(recorded), *fakeFile)
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

	stopped := onInterrupt(wire, eyes)
	defer stopped()

	var voiced []byte

	if *voicedFile != "" {
		voiced, err = os.ReadFile(*voicedFile)
		if err != nil {
			return fmt.Errorf("cannot read the recorded datagram: %w", err)
		}
	}

	engine := bypass.New(bypass.Settings{
		Rules:    base,
		Hunt:     hunt,
		Pattern:  pattern,
		Recorded: recorded,
		Voiced:   voiced,
		Silence:  time.Duration(*silence) * time.Second,
		DropQUIC: *noQUIC,
		Wet:      *wet,
		Seen:     *seen,
		Report:   func(text string) { fmt.Println(text) },
	})

	if err := engine.Run(wire, eyes); err != nil && !stopped() {
		return err
	}

	return nil
}

// Ctrl+C kills the process where it stands, so the deferred closes never run and
// the driver is left holding handles until Windows notices. Closing them here
// makes the loop fail, which is why the caller asks whether that was us.
func onInterrupt(handles ...io.Closer) func() bool {
	var asked atomic.Bool

	notice := make(chan os.Signal, 1)
	signal.Notify(notice, os.Interrupt)

	go func() {
		<-notice
		asked.Store(true)

		fmt.Println("stopping")

		for _, h := range handles {
			h.Close()
		}
	}()

	return asked.Load
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

// The older single-strategy flags, which say between them what one -rule says.
type asked struct {
	texts  []string
	hosts  string
	ttl    uint8
	badseq int32
	badsum bool
	decoy  string
	where  string
}

// plan turns whatever the command line carried into rules: either -rule entries,
// or the older flags spread over the hosts in -host.
func plan(a asked) (rules.Set, error) {
	if len(a.texts) > 0 {
		return rules.ParseAll(a.texts)
	}

	var ways []string

	if a.ttl != 0 {
		ways = append(ways, fmt.Sprintf("ttl:%d", a.ttl))
	}

	if a.badseq != 0 {
		ways = append(ways, fmt.Sprintf("badseq:%d", a.badseq))
	}

	if a.badsum {
		ways = append(ways, "badsum")
	}

	if a.decoy != "" {
		ways = append(ways, "decoy:"+a.decoy)
	}

	if a.where != "" {
		ways = append(ways, "cut:"+a.where)
	}

	if len(ways) == 0 {
		return nil, fmt.Errorf("say what to do: -rule host=way,way or the -ttl, -badseq, -badsum, -decoy and -cut flags")
	}

	var spread []string

	for _, host := range parseHosts(a.hosts) {
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

// written picks the rules out of a file: everything after a # is a note, and a
// line with nothing left on it is not a rule.
func written(text string) []string {
	var out []string

	for _, line := range strings.Split(text, "\n") {
		if note := strings.Index(line, "#"); note >= 0 {
			line = line[:note]
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		out = append(out, line)
	}

	return out
}

// parseRanges reads "19294-19344,50000-50100". A single port is a range of one,
// so "443" and "443-443" mean the same thing.
func parseRanges(list string) ([]filter.PortRange, error) {
	var out []filter.PortRange

	for _, text := range strings.Split(list, ",") {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		first, last, dashed := strings.Cut(text, "-")
		if !dashed {
			last = first
		}

		from, err := strconv.ParseUint(strings.TrimSpace(first), 10, 16)
		if err != nil {
			return nil, fmt.Errorf("port range %q: %w", text, err)
		}

		to, err := strconv.ParseUint(strings.TrimSpace(last), 10, 16)
		if err != nil {
			return nil, fmt.Errorf("port range %q: %w", text, err)
		}

		if to < from {
			return nil, fmt.Errorf("port range %q runs backwards", text)
		}

		out = append(out, filter.PortRange{From: uint16(from), To: uint16(to)})
	}

	return out, nil
}
