package sweep

import (
	"sync"
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
	mu sync.Mutex

	host       string
	candidates []rules.Rule
	at         int
	since      time.Time
	window     time.Duration

	sawHello  bool
	sawRepeat bool
	done      bool
}

func New(host string, candidates []rules.Rule, window time.Duration, now time.Time) *Sweep {
	return &Sweep{host: host, candidates: candidates, window: window, since: now}
}

func (s *Sweep) Host() string {
	return s.host
}

func (s *Sweep) Current() rules.Rule {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.candidates[s.at]
}

func (s *Sweep) Left() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.candidates) - s.at - 1
}

// Called from the loop and from the reply watcher, which run apart.
func (s *Sweep) Saw(repeat bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}

	s.sawHello = true

	if repeat {
		s.sawRepeat = true
	}
}

// A repeat settles the matter early, no reason to wait the window out.
func (s *Sweep) Judge(now time.Time) (Verdict, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done || (!s.sawRepeat && now.Sub(s.since) < s.window) {
		return Worked, false
	}

	verdict := Quiet

	if s.sawRepeat {
		verdict = Failed
	}

	if s.sawHello && !s.sawRepeat {
		verdict = Worked
	}

	s.done = verdict == Worked

	return verdict, true
}

func (s *Sweep) Next(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.at++
	s.since = now
	s.sawHello = false
	s.sawRepeat = false

	return s.at < len(s.candidates)
}

// Ordered by what has actually worked here, likeliest first. Built as rules
// rather than written out and parsed back, so a candidate cannot fail to parse.
func Candidates(host string) []rules.Rule {
	var out []rules.Rule

	add := func(r rules.Rule) {
		r.Host = host
		out = append(out, r)
	}

	// An overlap only works when a pattern was loaded; without one these come back
	// as "cannot overlap" and cost a few seconds each.
	add(rules.Rule{Overlap: 1})
	add(rules.Rule{Overlap: 2})
	add(rules.Rule{Overlap: 1, BadSeq: 100000, Decoy: "auto"})

	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Cut: "name"})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Cut: "after"})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Cut: "start"})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000})
	add(rules.Rule{Decoy: "auto", BadSum: true, Cut: "name"})
	add(rules.Rule{Decoy: "auto", BadSum: true})

	for _, hops := range []uint8{1, 2, 3, 4, 6, 8} {
		add(rules.Rule{Decoy: "auto", TTL: hops, Cut: "name"})
		add(rules.Rule{Decoy: "auto", TTL: hops})
	}

	add(rules.Rule{Cut: "name"})
	add(rules.Rule{Cut: "start"})
	add(rules.Rule{Cut: "after"})
	add(rules.Rule{BadSeq: 100000, Cut: "name"})
	add(rules.Rule{BadSum: true, Cut: "name"})

	return out
}
