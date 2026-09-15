package rules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrNoHost   = errors.New("rules: a rule starts with a host name and an equals sign")
	ErrNoWay    = errors.New("rules: a rule needs at least one of ttl, badseq, badsum, decoy or cut")
	ErrCutWhere = errors.New("rules: cut takes name, after or start")
)

type Rule struct {
	Host string

	TTL    uint8
	BadSeq uint32
	BadSum bool
	Decoy  string // empty, "auto", or a name of the very same length
	Cut    string // empty, "name", "after" or "start"

	Overlap  int
	Repeats  int
	Recorded bool
}

// Parse reads one rule, written as host=way,way,way. A way is ttl:4, badseq:100000,
// badsum, decoy, decoy:some.host.name or cut:name.
func Parse(text string) (Rule, error) {
	host, ways, found := strings.Cut(strings.TrimSpace(text), "=")
	if !found || strings.TrimSpace(host) == "" {
		return Rule{}, ErrNoHost
	}

	r := Rule{Host: strings.ToLower(strings.TrimSpace(host))}

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

// On the type so every caller counts the same fields; a way forgotten here does nothing.
func (r Rule) Blank() bool {
	return r.TTL == 0 && r.BadSeq == 0 && !r.BadSum && r.Decoy == "" && r.Cut == "" && r.Overlap == 0 && !r.Recorded
}

func (r *Rule) take(way string) error {
	name, value, hasValue := strings.Cut(way, ":")

	switch name {
	case "badsum":
		r.BadSum = true

		return nil
	case "decoy":
		r.Decoy = "auto"
		if hasValue {
			r.Decoy = value
		}

		return nil
	case "ttl":
		hops, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return fmt.Errorf("rules: ttl wants a number of hops: %w", err)
		}

		r.TTL = uint8(hops)

		return nil
	case "badseq":
		shift, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("rules: badseq wants a number: %w", err)
		}

		r.BadSeq = uint32(shift)

		return nil
	case "overlap":
		at, err := strconv.Atoi(value)
		if err != nil || at < 1 {
			return fmt.Errorf("rules: overlap wants how many real bytes go first, at least 1")
		}

		r.Overlap = at

		return nil
	case "fake":
		if value != "" {
			return fmt.Errorf("rules: fake takes no value, the file comes from -fake")
		}

		r.Recorded = true

		return nil
	case "repeats":
		copies, err := strconv.Atoi(value)
		if err != nil || copies < 1 || copies > 20 {
			return fmt.Errorf("rules: repeats wants how many copies go out, 1 to 20")
		}

		r.Repeats = copies

		return nil
	case "cut":
		if value != "name" && value != "after" && value != "start" {
			return ErrCutWhere
		}

		r.Cut = value

		return nil
	}

	return fmt.Errorf("rules: %q is no way to bypass anything", name)
}

type Set []Rule

func ParseAll(texts []string) (Set, error) {
	var set Set

	for _, text := range texts {
		r, err := Parse(text)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", text, err)
		}

		set = append(set, r)
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
