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

	carried   int
	best      rules.Rule
	bestBytes int
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

// Carried counts what the site actually got back while this candidate was on.
// Whether the client asked again says little; how far the stream went says more.
func (s *Sweep) Carried(bytes int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}

	s.carried += bytes

	if s.carried > s.bestBytes {
		s.bestBytes = s.carried
		s.best = s.candidates[s.at]
	}
}

// Bytes is what the candidate on now has carried so far.
func (s *Sweep) Bytes() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.carried
}

// Best is the candidate that carried the most, and how much.
func (s *Sweep) Best() (rules.Rule, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.best, s.bestBytes
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
	s.carried = 0

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

	// Swapping the name inside the stream is the one way measured to carry a whole
	// page through, and which name is used decides almost as much as the method.
	for _, name := range []string{"mail.ru", "ya.ru", "vk.com", "www.google.com", "auto"} {
		add(rules.Rule{HostFake: name, Stale: 600000})
	}

	add(rules.Rule{HostFake: "mail.ru", Stale: 600000, Disorder: true})
	add(rules.Rule{HostFake: "mail.ru", Stale: 600000, Repeats: 5})
	add(rules.Rule{HostFake: "mail.ru", BadSum: true})
	add(rules.Rule{HostFake: "mail.ru", TTL: 4})

	// An overlap only works when a pattern was loaded; without one these come back
	// as "cannot overlap" and cost a few seconds each.
	add(rules.Rule{Overlap: 1})
	add(rules.Rule{Overlap: 2})
	add(rules.Rule{Overlap: 1, BadSeq: 100000, Decoy: "auto"})

	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Cut: "name"})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Cut: "after"})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Cut: "start"})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Repeats: 5})
	add(rules.Rule{Decoy: "auto", Stale: 1 << 30, Cut: "name"})
	add(rules.Rule{Decoy: "auto", Stale: 1 << 30})
	add(rules.Rule{Decoy: "auto", BadAck: -66000})
	add(rules.Rule{Decoy: "auto", BadSeq: 100000, BadAck: -66000})
	add(rules.Rule{Decoy: "auto", Signed: true})
	add(rules.Rule{HostFake: "mail.ru", Signed: true})
	add(rules.Rule{Decoy: "auto", BadSum: true, Cut: "name"})
	add(rules.Rule{Decoy: "auto", BadSum: true})

	// Short names rebuild the hello, which a hello split across packets refuses;
	// they cost nothing to try and tell that apart from a way that simply fails.
	add(rules.Rule{Decoy: "mail.ru", BadSeq: 100000})
	add(rules.Rule{Decoy: "ya.ru", BadSeq: 100000, Cut: "name"})

	// A recorded hello needs -fake; without one these say so and move on.
	add(rules.Rule{Recorded: true, BadSeq: 100000})
	add(rules.Rule{Recorded: true, BadSum: true, Repeats: 5})

	add(rules.Rule{Decoy: "auto", BadSeq: 100000, Cut: "name", Disorder: true})

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
