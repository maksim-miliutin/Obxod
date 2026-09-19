package sweep

import (
	"sync"
	"testing"
	"time"

	"obxod/internal/rules"
)

func candidates(t *testing.T, texts ...string) []rules.Rule {
	t.Helper()

	set, err := rules.ParseAll(texts)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	return set
}

func TestRepeatEndsTheTryAtOnce(t *testing.T) {
	now := time.Now()
	s := New("gateway.discord.gg", candidates(t, "a=cut:name", "a=cut:start"), time.Minute, now)

	s.Saw(false)

	if _, done := s.Judge(now.Add(time.Second)); done {
		t.Fatal("judged before the window was up and before any repeat")
	}

	s.Saw(true)

	verdict, done := s.Judge(now.Add(2 * time.Second))
	if !done {
		t.Fatal("a repeat did not settle the try")
	}

	if verdict != Failed {
		t.Errorf("verdict = %v, want Failed", verdict)
	}
}

func TestQuietWindowCountsAsWorking(t *testing.T) {
	now := time.Now()
	s := New("gateway.discord.gg", candidates(t, "a=cut:name"), 10*time.Second, now)

	s.Saw(false)

	verdict, done := s.Judge(now.Add(11 * time.Second))
	if !done {
		t.Fatal("the window was up but nothing was judged")
	}

	if verdict != Worked {
		t.Errorf("verdict = %v, want Worked", verdict)
	}
}

func TestNoTrafficIsNotAVerdict(t *testing.T) {
	now := time.Now()
	s := New("gateway.discord.gg", candidates(t, "a=cut:name"), 10*time.Second, now)

	verdict, done := s.Judge(now.Add(11 * time.Second))
	if !done {
		t.Fatal("the window was up but nothing was judged")
	}

	if verdict != Quiet {
		t.Errorf("verdict = %v, want Quiet", verdict)
	}
}

func TestNextWalksThroughAndStops(t *testing.T) {
	now := time.Now()
	list := candidates(t, "a=cut:name", "a=cut:start", "a=badsum")
	s := New("gateway.discord.gg", list, time.Second, now)

	if s.Left() != 2 {
		t.Errorf("Left = %d, want 2", s.Left())
	}

	if !s.Next(now) {
		t.Fatal("stopped at the second of three")
	}

	if got := s.Current().Cut; got != "start" {
		t.Errorf("second candidate cuts at %q, want start", got)
	}

	if !s.Next(now) {
		t.Fatal("stopped at the third of three")
	}

	if s.Next(now) {
		t.Error("walked past the last candidate")
	}
}

func TestNextForgetsTheLastTry(t *testing.T) {
	now := time.Now()
	s := New("gateway.discord.gg", candidates(t, "a=cut:name", "a=cut:start"), time.Minute, now)

	s.Saw(true)
	s.Next(now)

	// The repeat belonged to the candidate before; judging the new one on it
	// would fail everything that followed a failure.
	if _, done := s.Judge(now.Add(time.Second)); done {
		t.Error("the new candidate was judged on the old one's repeat")
	}
}

func TestCandidatesAreAllUsable(t *testing.T) {
	list := Candidates("gateway.discord.gg")

	if len(list) < 10 {
		t.Fatalf("only %d candidates, too few to call it a sweep", len(list))
	}

	seen := make(map[rules.Rule]bool)

	for _, r := range list {
		if r.Host != "gateway.discord.gg" {
			t.Errorf("candidate names %q, want the host asked for", r.Host)
		}

		if r.Blank() {
			t.Error("a candidate does nothing at all")
		}

		if seen[r] {
			t.Errorf("candidate %+v appears twice", r)
		}

		seen[r] = true
	}
}

// The race this guards: the loop calls Saw while the reply watcher calls it too,
// and the sweep used to carry no lock of its own.
func TestSweepIsSafeFromTwoGoroutines(t *testing.T) {
	s := New("gateway.discord.gg", Candidates("gateway.discord.gg"), time.Hour, time.Now())

	var wg sync.WaitGroup

	wg.Add(3)

	go func() {
		defer wg.Done()

		for i := 0; i < 500; i++ {
			s.Saw(i%3 == 0)
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < 500; i++ {
			s.Saw(true)
		}
	}()

	go func() {
		defer wg.Done()

		now := time.Now()

		for i := 0; i < 500; i++ {
			if _, done := s.Judge(now); done {
				s.Next(now)
			}

			s.Current()
			s.Left()
		}
	}()

	wg.Wait()
}

// Once something worked the sweep is over: judging again must not start it up.
func TestWorkedEndsTheSweep(t *testing.T) {
	now := time.Now()
	s := New("gateway.discord.gg", Candidates("gateway.discord.gg"), time.Minute, now)

	s.Saw(false)

	verdict, done := s.Judge(now.Add(2 * time.Minute))
	if !done || verdict != Worked {
		t.Fatalf("Judge = %v %v, want a verdict of worked", verdict, done)
	}

	won := s.Current()

	s.Saw(true)

	if _, done := s.Judge(now.Add(time.Hour)); done {
		t.Error("a finished sweep judged again")
	}

	if s.Current() != won {
		t.Error("a finished sweep moved off the candidate that worked")
	}
}

// Every candidate has to be a rule the engine will act on: one that parses is not
// enough, a blank one would be tried for nothing.
func TestEveryCandidateIsAWorkingRule(t *testing.T) {
	const host = "gateway.discord.gg"

	seen := map[string]bool{}

	for i, r := range Candidates(host) {
		if r.Host != host {
			t.Errorf("candidate %d is for %q, want %q", i, r.Host, host)
		}

		if r.Blank() {
			t.Errorf("candidate %d does nothing", i)
		}

		if seen[r.Text()] {
			t.Errorf("candidate %d repeats %q", i, r.Text())
		}

		seen[r.Text()] = true
	}
}

// What the sweep prints as its answer has to be pastable back as a rule.
func TestEveryCandidateWritesBackAndParses(t *testing.T) {
	const host = "gateway.discord.gg"

	for _, r := range Candidates(host) {
		back, err := rules.Parse(host + "=" + r.Text())
		if err != nil {
			t.Fatalf("%q does not parse back: %v", r.Text(), err)
		}

		if back != r {
			t.Errorf("\n got %+v\nwant %+v\nvia %q", back, r, r.Text())
		}
	}
}

// Order is the whole value of the list: swapping the name is the one way measured
// to carry a whole page through, so it goes before anything else.
func TestSwappingTheNameComesFirst(t *testing.T) {
	first := Candidates("discord.com")[0]

	if first.HostFake == "" {
		t.Errorf("the sweep starts on %q, want a swapped name", first.Text())
	}

	if first.Stale == 0 {
		t.Error("the first candidate swaps the name without ageing the timestamp, which measured as nothing")
	}
}

// Which name is used decides almost as much as the method, so more than one is
// tried before the method is given up on.
func TestSeveralNamesAreTried(t *testing.T) {
	names := map[string]bool{}

	for _, r := range Candidates("discord.com") {
		if r.HostFake != "" {
			names[r.HostFake] = true
		}
	}

	if len(names) < 3 {
		t.Errorf("the sweep tries %d names, want several", len(names))
	}
}

// Whether the client asked again says little: we measured retries that had
// nothing to do with the rule. How far the stream went says more.
func TestBytesAreCountedPerCandidate(t *testing.T) {
	now := time.Now()
	s := New("discord.com", Candidates("discord.com"), time.Minute, now)

	s.Carried(1000)
	s.Carried(500)

	if s.Bytes() != 1500 {
		t.Errorf("Bytes = %d, want 1500", s.Bytes())
	}

	s.Next(now)

	if s.Bytes() != 0 {
		t.Errorf("the next candidate starts on %d bytes, want none", s.Bytes())
	}
}

func TestTheBestCandidateIsTheOneThatCarriedMost(t *testing.T) {
	now := time.Now()
	list := Candidates("discord.com")
	s := New("discord.com", list, time.Minute, now)

	s.Carried(100)
	s.Next(now)
	s.Carried(65000)
	s.Next(now)
	s.Carried(300)

	best, bytes := s.Best()

	if bytes != 65000 {
		t.Errorf("the best carried %d, want 65000", bytes)
	}

	if best != list[1] {
		t.Errorf("the best is %+v, want %+v", best, list[1])
	}
}

func TestNothingCarriedLeavesNoBest(t *testing.T) {
	s := New("discord.com", Candidates("discord.com"), time.Minute, time.Now())

	if _, bytes := s.Best(); bytes != 0 {
		t.Errorf("a sweep that carried nothing reports %d bytes", bytes)
	}
}
