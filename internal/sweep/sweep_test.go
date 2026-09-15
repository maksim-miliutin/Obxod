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
