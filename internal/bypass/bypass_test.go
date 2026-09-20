package bypass

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"obxod/internal/divert"
	"obxod/internal/rules"
	"obxod/internal/sweep"
)

var errStop = errors.New("nothing left to read")

type fakeWire struct {
	queue [][]byte
	sent  [][]byte
}

func (w *fakeWire) Recv(buf []byte) (int, divert.Addr, error) {
	if len(w.queue) == 0 {
		return 0, divert.Addr{}, errStop
	}

	packet := w.queue[0]
	w.queue = w.queue[1:]

	return copy(buf, packet), divert.Addr{}, nil
}

func (w *fakeWire) Send(packet []byte, addr *divert.Addr) error {
	w.sent = append(w.sent, append([]byte(nil), packet...))

	return nil
}

type shutEyes struct{}

func (shutEyes) Recv(buf []byte) (int, divert.Addr, error) {
	return 0, divert.Addr{}, errStop
}

// The watcher reports from its own goroutine while the loop reports from ours.
type lines struct {
	mu   sync.Mutex
	said []string
}

func (l *lines) say(text string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.said = append(l.said, text)
}

func (l *lines) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.said...)
}

func runOver(t *testing.T, s Settings, packets ...[]byte) *fakeWire {
	t.Helper()

	out := &lines{}
	s.Report = out.say

	wire := &fakeWire{queue: packets}

	if err := New(s).Run(wire, shutEyes{}); !errors.Is(err, errStop) {
		t.Fatalf("Run stopped with %v, want the driver error back", err)
	}

	return wire
}

func setOf(t *testing.T, texts ...string) rules.Set {
	t.Helper()

	set, err := rules.ParseAll(texts)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	return set
}

// The bug this guards: a reset arriving more than 20s after the hello was
// reported as a stranger's, because the port was looked up in the retry tracker.
func TestResetOnOurPortNamesTheHost(t *testing.T) {
	const host = "gateway.discord.gg"

	out := &lines{}

	e := New(Settings{Rules: setOf(t, host+"=badsum"), Wet: true, Report: out.say})

	if _, err := e.forward(&recorder{}, packet443(clientHello(host)), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	e.reset(0)

	var named, stranger bool

	for _, said := range out.all() {
		if strings.Contains(said, host) && strings.Contains(said, "reset by the other side") {
			named = true
		}

		if strings.Contains(said, "never touched") {
			stranger = true
		}
	}

	if !named || stranger {
		t.Errorf("reset was reported as named=%v stranger=%v, want our host named", named, stranger)
	}
}

func TestRunPassesWhatNoRuleCovers(t *testing.T) {
	packet := packet443(clientHello("example.org"))

	wire := runOver(t, Settings{Rules: setOf(t, "discord.com=cut:name"), Wet: true}, packet)

	if len(wire.sent) != 1 {
		t.Fatalf("sent %d packets, want the one that came in", len(wire.sent))
	}

	if !bytes.Equal(wire.sent[0], packet) {
		t.Error("a packet no rule covers came out changed")
	}
}

// The bug this guards: the halves go out and then the loop sends the original
// after them, so the server sees the hello twice and drops the connection.
func TestRunDoesNotResendACutHello(t *testing.T) {
	const host = "gateway.discord.gg"

	packet := packet443(clientHello(host))

	wire := runOver(t, Settings{Rules: setOf(t, host+"=cut:name"), Wet: true}, packet)

	if len(wire.sent) != 2 {
		t.Fatalf("sent %d packets, want the two halves alone", len(wire.sent))
	}

	for i, out := range wire.sent {
		if bytes.Equal(out, packet) {
			t.Errorf("packet %d is the whole original, which must not follow its own halves", i)
		}
	}
}

func TestRunDropsQUICOnlyWhenAsked(t *testing.T) {
	cases := map[string]struct {
		drop bool
		want int
	}{
		"asked":     {true, 0},
		"not asked": {false, 1},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			wire := runOver(t, Settings{DropQUIC: c.drop, Wet: true}, datagramTo(443))

			if len(wire.sent) != c.want {
				t.Errorf("sent %d datagrams, want %d", len(wire.sent), c.want)
			}
		})
	}
}

func TestRunLetsVoiceDatagramsThroughEvenWhenQUICIsDropped(t *testing.T) {
	wire := runOver(t, Settings{DropQUIC: true, Wet: true}, datagramTo(50021))

	if len(wire.sent) != 1 {
		t.Errorf("sent %d datagrams, want the voice one to pass", len(wire.sent))
	}
}

func TestIsQUIC(t *testing.T) {
	if !isQUIC(datagramTo(443)) {
		t.Error("a datagram to 443 was not taken for quic")
	}

	if isQUIC(datagramTo(50021)) {
		t.Error("a voice datagram was taken for quic")
	}

	if isQUIC(packet443(clientHello("discord.com"))) {
		t.Error("a tcp hello was taken for quic")
	}

	if isQUIC(nil) {
		t.Error("an empty packet was taken for quic")
	}
}

// The sweep is only useful if what it prints can be pasted back as a rule.
func TestAsRuleRoundTrips(t *testing.T) {
	for _, r := range sweep.Candidates("gateway.discord.gg") {
		text := r.Text()

		back, err := rules.Parse("gateway.discord.gg=" + text)
		if err != nil {
			t.Fatalf("%q does not parse back: %v", text, err)
		}

		if back != r {
			t.Errorf("\n got %+v\nwant %+v\nfrom %q", back, r, text)
		}
	}
}

// The bug this guards: sweeping one host used to throw away the rules that made
// the rest of the site work, so the swept host was never even asked for.
func TestSweepKeepsTheOtherRules(t *testing.T) {
	base := setOf(t,
		"discord.com=decoy,badseq:100000,cut:name",
		"gateway.discord.gg=badsum",
	)

	candidate, err := rules.Parse("gateway.discord.gg=ttl:2,cut:start")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	set := withCandidate(base, candidate)

	if len(set) != 2 {
		t.Fatalf("set holds %d rules, want 2", len(set))
	}

	got, ok := set.For("updates.discord.com")
	if !ok || got.Cut != "name" {
		t.Error("the rule that already worked for discord.com was lost")
	}

	got, ok = set.For("gateway.discord.gg")
	if !ok || got.TTL != 2 || got.Cut != "start" {
		t.Errorf("the swept host got %+v, want the candidate", got)
	}
}

func TestWithCandidateOnAnEmptyBase(t *testing.T) {
	candidate, err := rules.Parse("gateway.discord.gg=badsum")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if set := withCandidate(nil, candidate); len(set) != 1 {
		t.Errorf("set holds %d rules, want just the candidate", len(set))
	}
}

// A sweep starts on its first candidate, not on whatever -rule said for that host.
func TestNewPutsTheFirstCandidateInPlace(t *testing.T) {
	const host = "gateway.discord.gg"

	first := sweep.Candidates(host)[0]

	e := New(Settings{
		Rules: setOf(t, host+"=badsum"),
		Hunt:  sweep.New(host, sweep.Candidates(host), 0, time.Time{}),
	})

	got, ok := e.set.For(host)
	if !ok || got != first {
		t.Errorf("the sweep starts on %+v, want %+v", got, first)
	}
}

// The trap this closes: a site on a name nobody listed goes out untouched and the
// program says nothing, so the missing rule is invisible.
func TestNamesWithNoRuleAreReportedOnce(t *testing.T) {
	out := &lines{}

	e := New(Settings{
		Rules:  setOf(t, "discord.com=hostfake:mail.ru"),
		Wet:    true,
		Seen:   true,
		Report: out.say,
	})

	for range 3 {
		if _, err := e.forward(&recorder{}, packet443(clientHello("cdn.discordapp.com")), &divert.Addr{}); err != nil {
			t.Fatalf("forward: %v", err)
		}
	}

	if _, err := e.forward(&recorder{}, packet443(clientHello("media.discordapp.net")), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	var named []string

	for _, said := range out.all() {
		if strings.Contains(said, "no rule covers") {
			named = append(named, said)
		}
	}

	if len(named) != 2 {
		t.Fatalf("named %d hosts, want each of the two once: %q", len(named), named)
	}
}

// Off by default: a name with no rule is the normal case for everything else on
// the machine, and saying so on every packet would bury the rest of the log.
func TestNamesAreNotReportedUnlessAsked(t *testing.T) {
	out := &lines{}

	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true, Report: out.say})

	if _, err := e.forward(&recorder{}, packet443(clientHello("example.org")), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	for _, said := range out.all() {
		if strings.Contains(said, "no rule covers") {
			t.Errorf("named a host without being asked: %q", said)
		}
	}
}

// A name a rule does cover is not a miss, however the rule was written.
func TestACoveredNameIsNotNamed(t *testing.T) {
	out := &lines{}

	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true, Seen: true, Report: out.say})

	if _, err := e.forward(&recorder{}, packet443(clientHello("updates.discord.com")), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	for _, said := range out.all() {
		if strings.Contains(said, "no rule covers") {
			t.Errorf("a covered name was called a miss: %q", said)
		}
	}
}

// A repeated hello says the client asked again and nothing more. We measured
// retries on hosts no rule touched, so calling one a failed way is a guess.
func TestARepeatIsNotCalledAFailure(t *testing.T) {
	out := &lines{}

	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true, Report: out.say})
	packet := packet443(clientHello("updates.discord.com"))

	for range 2 {
		if _, err := e.forward(&recorder{}, packet, &divert.Addr{}); err != nil {
			t.Fatalf("forward: %v", err)
		}
	}

	var told bool

	for _, said := range out.all() {
		if strings.Contains(said, "not getting through") {
			t.Errorf("a repeat was blamed on the way: %q", said)
		}

		if strings.Contains(said, "asking again") {
			told = true
		}
	}

	if !told {
		t.Error("the repeat was not reported at all")
	}
}
