package sweep

import (
	"fmt"
	"time"

	"obxod/internal/rules"
)

type Verdict int

const (
	Worked Verdict = iota
	Failed
	Quiet
)

func (v Verdict) String() string {
	switch v {
	case Worked:
		return "worked"
	case Failed:
		return "asked again"
	}

	return "no traffic to judge"
}

type Sweep struct {
	host       string
	candidates []rules.Rule
	at         int
	since      time.Time
	window     time.Duration

	sawHello  bool
	sawRepeat bool
}

func New(host string, candidates []rules.Rule, window time.Duration, now time.Time) *Sweep {
	return &Sweep{host: host, candidates: candidates, window: window, since: now}
}

func (s *Sweep) Host() string {
	return s.host
}

func (s *Sweep) Current() rules.Rule {
	return s.candidates[s.at]
}

func (s *Sweep) Left() int {
	return len(s.candidates) - s.at - 1
}

func (s *Sweep) Saw(repeat bool) {
	s.sawHello = true

	if repeat {
		s.sawRepeat = true
	}
}

// Judge gives a verdict once the window is up, or once the client has already
// asked again: a repeat settles the matter, no reason to keep waiting it out.
func (s *Sweep) Judge(now time.Time) (Verdict, bool) {
	if !s.sawRepeat && now.Sub(s.since) < s.window {
		return Worked, false
	}

	verdict := Quiet

	if s.sawRepeat {
		verdict = Failed
	}

	if s.sawHello && !s.sawRepeat {
		verdict = Worked
	}

	return verdict, true
}

// Next moves to the following candidate and reports whether one was left.
func (s *Sweep) Next(now time.Time) bool {
	s.at++
	s.since = now
	s.sawHello = false
	s.sawRepeat = false

	return s.at < len(s.candidates)
}

// Candidates lists the ways worth trying, cheapest and most likely first. The
// order comes from what beat updates.discord.com: a decoy with a wrong sequence
// number, ahead of a hello cut through its own name.
func Candidates(host string) []rules.Rule {
	var out []rules.Rule

	add := func(text string) {
		r, err := rules.Parse(host + "=" + text)
		if err != nil {
			panic(fmt.Sprintf("sweep: built a rule that will not parse: %v", err))
		}

		out = append(out, r)
	}

	add("decoy,badseq:100000,cut:name")
	add("decoy,badseq:100000,cut:after")
	add("decoy,badseq:100000,cut:start")
	add("decoy,badseq:100000")
	add("decoy,badsum,cut:name")
	add("decoy,badsum")

	for _, hops := range []int{1, 2, 3, 4, 6, 8} {
		add(fmt.Sprintf("decoy,ttl:%d,cut:name", hops))
		add(fmt.Sprintf("decoy,ttl:%d", hops))
	}

	add("cut:name")
	add("cut:start")
	add("cut:after")
	add("badseq:100000,cut:name")
	add("badsum,cut:name")

	return out
}
