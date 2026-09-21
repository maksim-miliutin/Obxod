package rules

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrManyHosts = errors.New("rules: a host name cannot hold a comma")
	ErrNoHost    = errors.New("rules: a rule starts with a host name and an equals sign")
	ErrNoWay     = errors.New("rules: a rule needs a way to bypass, such as decoy or cut:name")
	ErrCutWhere  = errors.New("rules: cut takes name, after or start")
)

// Ten minutes of ticks: old enough for the server to call the copy a stale
// duplicate, recent enough to still read as a real timestamp. Short of half the
// timestamp space, past which the subtraction wraps into the future instead.
const (
	staleDefault = 600000
	staleMost    = 1<<31 - 1
)

type Rule struct {
	Host string

	TTL    uint8
	BadSeq int32
	BadSum bool
	Decoy  string // empty, "auto", or a name of the very same length
	Cut    string // empty, "name", "after" or "start"

	Overlap  int
	Repeats  int
	Recorded bool
	BadAck   int32
	Stale    uint32
	Disorder bool
	HostFake string
	FakeUDP  int
}

// Parse reads one rule, written as host=way,way,way.
func Parse(text string) (Rule, error) {
	host, ways, found := strings.Cut(strings.TrimSpace(text), "=")
	if !found || strings.TrimSpace(host) == "" {
		return Rule{}, ErrNoHost
	}

	host = strings.ToLower(strings.TrimSpace(host))
	if strings.Contains(host, ",") {
		return Rule{}, ErrManyHosts
	}

	r := Rule{Host: host}

	for _, way := range strings.Split(ways, ",") {
		way = strings.TrimSpace(way)
		if way == "" {
			continue
		}

		if err := r.take(way); err != nil {
			return Rule{}, err
		}
	}

	if r.Blank() {
		return Rule{}, ErrNoWay
	}

	return r, nil
}

func (r *Rule) take(text string) error {
	name, value, given := strings.Cut(text, ":")

	for _, w := range ways {
		if w.name == name {
			return w.read(r, value, given)
		}
	}

	return fmt.Errorf("rules: %q is no way to bypass anything", name)
}

type Set []Rule

// Several reads one line that may name more than one host for the same ways,
// which is how a list of a site's names stays one line instead of twenty.
func Several(text string) (Set, error) {
	hosts, ways, found := strings.Cut(strings.TrimSpace(text), "=")
	if !found {
		return nil, ErrNoHost
	}

	var set Set

	for _, host := range strings.Split(hosts, ",") {
		r, err := Parse(host + "=" + ways)
		if err != nil {
			return nil, err
		}

		set = append(set, r)
	}

	return set, nil
}

func ParseAll(texts []string) (Set, error) {
	var set Set

	for _, text := range texts {
		some, err := Several(text)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", text, err)
		}

		set = append(set, some...)
	}

	return set, nil
}

// Longest match wins, so gateway.discord.gg can differ from discord.com.
func (s Set) For(host string) (Rule, bool) {
	host = strings.ToLower(host)

	best := -1

	for i, r := range s {
		if r.Host != "all" && r.Host != host && !strings.HasSuffix(host, "."+r.Host) {
			continue
		}

		if best < 0 || len(r.Host) > len(s[best].Host) {
			best = i
		}
	}

	if best < 0 {
		return Rule{}, false
	}

	return s[best], true
}
