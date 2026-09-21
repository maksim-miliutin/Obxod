package bypass

import (
	"fmt"
	"sort"
	"strings"
	"sync"
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
	Rules    rules.Set
	Hunt     *sweep.Sweep
	Pattern  []byte
	Recorded []byte
	Voiced   []byte // a recorded voice datagram, sent ahead by the fakeudp way

	Silence  time.Duration
	DropQUIC bool
	Wet      bool
	Seen     bool // name the hosts no rule covers, so the missing ones can be added

	Report func(text string)
}

type Engine struct {
	base     rules.Set
	set      rules.Set
	hunt     *sweep.Sweep
	pattern  []byte
	voiced   []byte
	recorded []byte

	silence  time.Duration
	dropQUIC bool
	wet      bool

	report func(text string)

	tries  *attempt.Tracker
	health *link.Health

	quiet   int
	dropped int

	// Grows with the number of distinct names browsed while -seen is on, which is
	// a flag turned on for a short look rather than left running.
	seen map[string]bool

	// The loop writes these and the reply watcher reads and clears them, so they
	// need a lock of their own; every other field here belongs to one goroutine.
	toldMu sync.Mutex
	told   map[string]int
}

func New(s Settings) *Engine {
	e := &Engine{
		base:     s.Rules,
		set:      s.Rules,
		hunt:     s.Hunt,
		pattern:  s.Pattern,
		recorded: s.Recorded,
		voiced:   s.Voiced,
		silence:  s.Silence,
		dropQUIC: s.DropQUIC,
		wet:      s.Wet,
		report:   s.Report,
		tries:    attempt.New(forget),
		health:   link.New(),
	}

	if s.Seen {
		e.seen = map[string]bool{}
	}

	e.told = map[string]int{}

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

		if sent, err := e.voice(wire, packet, &addr); err != nil {
			return err
		} else if sent {
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
		e.say("  %s on port %d: %d bytes in %d packets, then quiet for %s",
			gone.Host, gone.Port, gone.Bytes, gone.Packets, gone.Silence.Round(time.Second))
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

	e.say("  %s: %s, %d bytes carried", e.hunt.Current(), verdict, e.hunt.Bytes())

	if verdict == sweep.Worked {
		e.say("\nthis one works, keeping it:\n  -rule \"%s=%s\"", e.hunt.Host(), e.hunt.Current().Text())

		return nil
	}

	host := e.hunt.Host()

	if !e.hunt.Next(now) {
		best, bytes := e.hunt.Best()

		if bytes == 0 {
			return fmt.Errorf("bypass: tried everything for %s and nothing got through", host)
		}

		return fmt.Errorf("bypass: tried everything for %s; the most that got through was %d bytes with -rule %q",
			host, bytes, host+"="+best.Text())
	}

	e.quiet++

	if verdict != sweep.Quiet {
		e.quiet = 0
	}

	// Every candidate gets its own window, and a window with no hello in it judges
	// nothing. A browser that already has the page open opens no new connection.
	if e.quiet == 4 {
		e.say("\n%s has said nothing for four tries. A sweep judges live traffic:"+
			" keep reloading the site while it runs, and give the rules the rest of it needs with -rule.\n", host)
	}

	e.say("trying %s, %d left", e.hunt.Current().String(), e.hunt.Left())
	e.set = withCandidate(e.base, e.hunt.Current())

	return nil
}

func (e *Engine) watch(eyes Eyes) {
	seen := replies.Watch{
		Seen:   e.watched,
		Reset:  e.reset,
		Closed: e.health.Closed,
		Data:   e.answered,
	}

	if err := seen.Run(eyes); err != nil {
		e.say("watching replies stopped: %v", err)
	}
}

// Every so often, how many replies went by and what was said once and counted after.
func (e *Engine) watched(total int) {
	if total%200 != 1 {
		return
	}

	if counted := e.tally(); counted != "" {
		e.say("  replies watched: %d so far (%s)", total, counted)

		return
	}

	e.say("  replies watched: %d so far", total)
}

func (e *Engine) reset(port uint16) {
	host, known := e.health.Reset(port)
	if !known {
		// A link is forgotten the moment there is nothing left to say about it, so
		// an unknown port is either one we never touched or one already done with.
		e.once("reset", "on a port we are not watching")

		return
	}

	e.say("  %s on port %d: reset by the other side", host, port)

	if e.hunt != nil && e.hunt.Host() == host {
		e.hunt.Saw(true)
	}
}

func (e *Engine) answered(port uint16, bytes int) {
	host, first := e.health.Data(port, bytes, time.Now())
	if host == "" {
		return
	}

	if e.hunt != nil && e.hunt.Host() == host {
		e.hunt.Carried(bytes)
	}

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
		e.say("  %s: %s", r.Host, r)
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

// once says a thing about a host the first time and counts it after that. A site
// opens a connection a second, and the same line hundreds of times over buries
// everything worth reading.
func (e *Engine) once(host, what string, args ...any) {
	key := host + "|" + what

	e.toldMu.Lock()
	e.told[key]++
	first := e.told[key] == 1
	e.toldMu.Unlock()

	if first {
		e.say("  "+host+": "+what, args...)
	}
}

// Tally names what has been happening since the last time, for the lines that
// were said once and counted after.
func (e *Engine) tally() string {
	e.toldMu.Lock()
	defer e.toldMu.Unlock()

	var out []string

	for key, times := range e.told {
		if times < 2 {
			continue
		}

		host, what, _ := strings.Cut(key, "|")
		out = append(out, fmt.Sprintf("%s %s %d more", host, firstWord(what), times-1))
	}

	sort.Strings(out)
	clear(e.told)

	return strings.Join(out, ", ")
}

func firstWord(what string) string {
	word, _, _ := strings.Cut(what, " ")

	return word
}
