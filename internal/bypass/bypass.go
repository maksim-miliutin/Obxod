package bypass

import (
	"fmt"
	"strings"
	"time"

	"obxod/internal/attempt"
	"obxod/internal/divert"
	"obxod/internal/ip"
	"obxod/internal/link"
	"obxod/internal/replies"
	"obxod/internal/rules"
	"obxod/internal/sweep"
	"obxod/internal/udp"
)

const maxPacket = 0xffff + 40

// A client that got no acknowledgement stops retransmitting long before this;
// past it the same sequence number is a new request rather than another try.
const forget = 20 * time.Second

type Wire interface {
	Recv(buf []byte) (int, divert.Addr, error)
	Send(packet []byte, addr *divert.Addr) error
}

type Eyes interface {
	Recv(buf []byte) (int, divert.Addr, error)
}

type Settings struct {
	Rules   rules.Set
	Hunt    *sweep.Sweep
	Pattern []byte

	Silence  time.Duration
	DropQUIC bool
	Wet      bool

	Report func(text string)
}

type Engine struct {
	base    rules.Set
	set     rules.Set
	hunt    *sweep.Sweep
	pattern []byte

	silence  time.Duration
	dropQUIC bool
	wet      bool

	report func(text string)

	tries  *attempt.Tracker
	health *link.Health

	quiet   int
	dropped int
}

func New(s Settings) *Engine {
	e := &Engine{
		base:     s.Rules,
		set:      s.Rules,
		hunt:     s.Hunt,
		pattern:  s.Pattern,
		silence:  s.Silence,
		dropQUIC: s.DropQUIC,
		wet:      s.Wet,
		report:   s.Report,
		tries:    attempt.New(forget),
		health:   link.New(),
	}

	if e.hunt != nil {
		e.set = withCandidate(e.base, e.hunt.Current())
	}

	return e
}

// Run blocks until the driver fails. Replies are watched alongside on eyes, and
// that watch only reports: losing it leaves the loop running.
func (e *Engine) Run(wire Wire, eyes Eyes) error {
	go e.watch(eyes)

	e.announce()

	buf := make([]byte, maxPacket)

	for {
		n, addr, err := wire.Recv(buf)
		if err != nil {
			return err
		}

		packet := buf[:n]

		if e.dropQUIC && isQUIC(packet) {
			e.dropped++

			if e.dropped%50 == 1 {
				e.say("  quic dropped: %d so far", e.dropped)
			}

			continue
		}

		if err := e.step(time.Now()); err != nil {
			return err
		}

		sent, err := e.forward(wire, packet, &addr)
		if err != nil {
			return err
		}

		if sent {
			continue
		}

		if err := wire.Send(packet, &addr); err != nil {
			return err
		}
	}
}

func (e *Engine) step(now time.Time) error {
	if err := e.judge(now); err != nil {
		return err
	}

	for _, gone := range e.health.WentQuiet(now, e.silence) {
		e.say("  %s on port %d: answered %d times then went silent for %s, the connection was killed",
			gone.Host, gone.Port, gone.Packets, gone.Silence.Round(time.Second))
	}

	return nil
}

func (e *Engine) judge(now time.Time) error {
	if e.hunt == nil {
		return nil
	}

	verdict, done := e.hunt.Judge(now)
	if !done {
		return nil
	}

	e.say("  %s: %s", describe(e.hunt.Current()), verdict)

	if verdict == sweep.Worked {
		e.say("\nthis one works, keeping it:\n  -rule \"%s=%s\"", e.hunt.Host(), asRule(e.hunt.Current()))

		return nil
	}

	host := e.hunt.Host()

	if !e.hunt.Next(now) {
		return fmt.Errorf("bypass: nothing left to try for %s", host)
	}

	e.quiet++

	if verdict != sweep.Quiet {
		e.quiet = 0
	}

	if e.quiet == 4 {
		e.say("\n%s has not asked for anything yet. Give the rules that already work with -rule, or the site never gets this far.\n", host)
	}

	e.say("trying %s, %d left", describe(e.hunt.Current()), e.hunt.Left())
	e.set = withCandidate(e.base, e.hunt.Current())

	return nil
}

func (e *Engine) watch(eyes Eyes) {
	seen := replies.Watch{
		Seen: func(total int) {
			if total%200 == 1 {
				e.say("  replies watched: %d so far", total)
			}
		},
		Reset:  e.reset,
		Closed: e.health.Closed,
		Data:   e.answered,
	}

	if err := seen.Run(eyes); err != nil {
		e.say("watching replies stopped: %v", err)
	}
}

func (e *Engine) reset(port uint16) {
	host, known := e.health.Reset(port)
	if !known {
		e.say("  reset on port %d, which we never touched", port)

		return
	}

	e.say("  %s on port %d: reset by the other side", host, port)

	if e.hunt != nil && e.hunt.Host() == host {
		e.hunt.Saw(true)
	}
}

func (e *Engine) answered(port uint16) {
	host, first := e.health.Data(port, time.Now())
	if !first {
		return
	}

	e.say("  %s on port %d: the server answered", host, port)
}

func (e *Engine) announce() {
	mode := "dry run, copies are only reported"
	if e.wet {
		mode = "sending copies"
	}

	e.say("%s", mode)

	for _, r := range e.set {
		e.say("  %s: %s", r.Host, describe(r))
	}
}

func (e *Engine) say(format string, args ...any) {
	if e.report == nil {
		return
	}

	e.report(fmt.Sprintf(format, args...))
}

// The rules that already work stay on: without them the site never gets far
// enough to ask for the one being swept.
func withCandidate(base rules.Set, r rules.Rule) rules.Set {
	out := rules.Set{r}

	for _, had := range base {
		if had.Host != r.Host {
			out = append(out, had)
		}
	}

	return out
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

	if r.Overlap != 0 {
		named = append(named, fmt.Sprintf("overlap keeping %d", r.Overlap))
	}

	if r.Repeats != 0 {
		named = append(named, fmt.Sprintf("%d copies", r.Repeats))
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

	if r.Overlap != 0 {
		ways = append(ways, fmt.Sprintf("overlap:%d", r.Overlap))
	}

	if r.Repeats != 0 {
		ways = append(ways, fmt.Sprintf("repeats:%d", r.Repeats))
	}

	return strings.Join(ways, ",")
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
