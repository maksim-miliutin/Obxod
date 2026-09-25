package bypass

import (
	"bytes"
	"errors"
	"fmt"
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

// The flood this stops: one site opens a connection a second, and the same line
// hundreds of times over buries every other host in the log.
func TestTheSameNewsIsSaidOnceThenCounted(t *testing.T) {
	out := &lines{}

	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true, Report: out.say})
	packet := packet443(clientHello("updates.discord.com"))

	for range 50 {
		if _, err := e.forward(&recorder{}, packet, &divert.Addr{}); err != nil {
			t.Fatalf("forward: %v", err)
		}
	}

	var swapped int

	for _, said := range out.all() {
		if strings.Contains(said, "name swapped") {
			swapped++
		}
	}

	if swapped != 1 {
		t.Errorf("said the swap %d times, want once", swapped)
	}
}

func TestTheTallyNamesWhatWasCounted(t *testing.T) {
	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true})
	packet := packet443(clientHello("updates.discord.com"))

	for range 10 {
		if _, err := e.forward(&recorder{}, packet, &divert.Addr{}); err != nil {
			t.Fatalf("forward: %v", err)
		}
	}

	said := e.tally()

	if !strings.Contains(said, "updates.discord.com") || !strings.Contains(said, "9 more") {
		t.Errorf("tally = %q, want the host and the nine it did not say", said)
	}

	if again := e.tally(); again != "" {
		t.Errorf("the tally repeats itself: %q", again)
	}
}

// Different news about the same host is different news.
func TestTwoKindsOfNewsAreCountedApart(t *testing.T) {
	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true})

	for range 3 {
		e.once("a.discord.com", "name swapped for %s", "x.mail.ru")
		e.once("a.discord.com", "asking again")
	}

	said := e.tally()

	if !strings.Contains(said, "name swapped for") || !strings.Contains(said, "asking again") {
		t.Errorf("tally = %q, want both kinds counted", said)
	}
}

// Twice a sweep judged nothing forty-five times over because the browser was
// idle. Saying "no traffic" without saying what to do about it wastes the run.
func TestASilentSweepAsksForTraffic(t *testing.T) {
	out := &lines{}
	now := time.Now()

	e := New(Settings{
		Rules:  setOf(t, "discord.com=hostfake:mail.ru"),
		Hunt:   sweep.New("nothing.example", sweep.Candidates("nothing.example"), time.Second, now),
		Wet:    true,
		Report: out.say,
	})

	for i := range 6 {
		if err := e.judge(now.Add(time.Duration(i+1) * 2 * time.Second)); err != nil {
			t.Fatalf("judge: %v", err)
		}
	}

	var asked bool

	for _, said := range out.all() {
		if strings.Contains(said, "keep reloading") {
			asked = true
		}
	}

	if !asked {
		t.Errorf("a sweep with nothing to judge never said what it needs:\n%s", strings.Join(out.all(), "\n"))
	}
}

// The bug this guards: counting the repeats and never printing them is worse than
// printing them all. The loop writes the counts, the reply watcher reads them, and
// a run with no test across that boundary said nothing for a whole day.
func TestTheHeartbeatShowsWhatWasCounted(t *testing.T) {
	out := &lines{}

	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true, Report: out.say})
	packet := packet443(clientHello("updates.discord.com"))

	for range 10 {
		if _, err := e.forward(&recorder{}, packet, &divert.Addr{}); err != nil {
			t.Fatalf("forward: %v", err)
		}
	}

	e.watched(1)

	var shown bool

	for _, said := range out.all() {
		if strings.Contains(said, "replies watched") && strings.Contains(said, "9 more") {
			shown = true
		}
	}

	if !shown {
		t.Errorf("the heartbeat said nothing about the nine it swallowed:\n%s", strings.Join(out.all(), "\n"))
	}
}

// once runs on the loop and tally on the reply watcher. Without a lock of their own
// this is a data race, and no test crossed that boundary until this one.
func TestCountingIsSafeFromTwoGoroutines(t *testing.T) {
	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true})

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := range 500 {
			e.once(fmt.Sprintf("host%d.discord.com", i%7), "asking again")
		}
	}()

	go func() {
		defer wg.Done()

		for range 500 {
			e.tally()
		}
	}()

	wg.Wait()
}

// The claim this drops: a link is forgotten as soon as there is nothing left to
// say about it, so an unknown port is either one we never touched or one we
// finished with, and the log said the first about both.
func TestAResetOnAnUnknownPortClaimsNothing(t *testing.T) {
	out := &lines{}

	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true, Report: out.say})

	e.reset(54321)

	for _, said := range out.all() {
		if strings.Contains(said, "never touched") {
			t.Errorf("said what it cannot know: %q", said)
		}
	}
}

// And the flood it stops: a machine resets dozens of ports a minute, none of them
// ours, and each one used to take a line.
func TestResetsOnUnknownPortsAreCountedNotListed(t *testing.T) {
	out := &lines{}

	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true, Report: out.say})

	for port := uint16(50000); port < 50040; port++ {
		e.reset(port)
	}

	var said int

	for _, line := range out.all() {
		if strings.Contains(line, "reset") {
			said++
		}
	}

	if said != 1 {
		t.Errorf("forty resets took %d lines, want one", said)
	}
}

// The waste this stops: the loop asked for the bookkeeping once a packet, and
// with forty live links that cost eight times what handling the packet did.
func TestTheBookkeepingRunsOnATimer(t *testing.T) {
	out := &lines{}
	now := time.Now()

	e := New(Settings{
		Rules:   setOf(t, "discord.com=hostfake:mail.ru"),
		Wet:     true,
		Silence: time.Second,
		Report:  out.say,
	})

	e.health.Hello("discord.com", 1111)
	e.health.Data(1111, 1460, now)

	if err := e.step(now.Add(time.Minute)); err != nil {
		t.Fatalf("step: %v", err)
	}

	// A second link, just as quiet, a moment after the last look.
	e.health.Hello("discord.com", 2222)
	e.health.Data(2222, 700, now)

	if err := e.step(now.Add(time.Minute + 10*time.Millisecond)); err != nil {
		t.Fatalf("step: %v", err)
	}

	for _, said := range out.all() {
		if strings.Contains(said, "700 bytes") {
			t.Errorf("looked again ten milliseconds later: %q", said)
		}
	}
}

// And it still runs: a link that goes quiet has to be reported, just not checked
// for on every packet that goes by.
func TestTheBookkeepingStillRuns(t *testing.T) {
	out := &lines{}
	now := time.Now()

	e := New(Settings{
		Rules:   setOf(t, "discord.com=hostfake:mail.ru"),
		Wet:     true,
		Silence: time.Second,
		Report:  out.say,
	})

	e.health.Hello("discord.com", 54321)
	e.health.Data(54321, 1460, now)

	if err := e.step(now.Add(time.Minute)); err != nil {
		t.Fatalf("step: %v", err)
	}

	var told bool

	for _, said := range out.all() {
		if strings.Contains(said, "1460 bytes") {
			told = true
		}
	}

	if !told {
		t.Errorf("the quiet link was never reported:\n%s", strings.Join(out.all(), "\n"))
	}
}

// The bug this guards: the count key was the template, so a line with a %d in it
// showed up in the tally as "%d" instead of the word, exactly on voice datagrams.
func TestTheTallyShowsAWordNotAPlaceholder(t *testing.T) {
	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true})

	for range 4 {
		e.once("voice", "%d recorded datagrams sent first", 5)
	}

	said := e.tally()

	if strings.Contains(said, "%") {
		t.Errorf("the tally still has a placeholder in it: %q", said)
	}

	if !strings.Contains(said, "recorded datagrams") {
		t.Errorf("tally = %q, want the voice line counted", said)
	}

	// The verb letter has to go with the %, or "%d recorded" leaves a stray "d".
	if strings.Contains(said, "d recorded") {
		t.Errorf("tally = %q, a format letter was left behind", said)
	}
}

// And the fold still holds when the tail changes every time, which a swapped name
// does: same first word, so it is the same news however the made up name reads.
func TestLinesFoldByTheirFirstWord(t *testing.T) {
	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Wet: true})

	for _, name := range []string{"aja", "epm", "xgh", "fnu"} {
		e.once("discord.com", "name swapped for %s.mail.ru", name)
	}

	said := e.tally()

	if !strings.Contains(said, "name swapped for") {
		t.Errorf("tally = %q, want the names folded into one", said)
	}
}

func TestDownloadedStartsAtZero(t *testing.T) {
	if n := New(Settings{Report: func(string) {}}).Downloaded(); n != 0 {
		t.Fatalf("Downloaded on a fresh engine = %d, want 0", n)
	}
}
