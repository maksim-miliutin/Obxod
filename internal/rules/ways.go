package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// What a way is for, beyond changing a field.
const (
	counts = 1 << iota // stands as a way to bypass on its own
	forges             // wants a forged copy on the wire
	spoils             // named among what is wrong with that copy
)

type way struct {
	name string
	hint string // how it is written, for the command line help
	kind int

	read  func(r *Rule, value string, given bool) error
	write func(r Rule) string // back into rule text, empty when unset
	say   func(r Rule) string // into words, empty when unset
}

// The one list. A way added here is parsed, written back, described and offered
// in the help at once; a way added anywhere else is half a way.
var ways = []way{
	{
		name: "ttl", hint: "ttl:4", kind: counts | forges | spoils,
		read: func(r *Rule, value string, _ bool) error {
			hops, err := strconv.ParseUint(value, 10, 8)
			if err != nil || hops == 0 {
				return fmt.Errorf("rules: ttl wants how many hops the copy may live, 1 to 255")
			}

			r.TTL = uint8(hops)

			return nil
		},
		write: func(r Rule) string { return number("ttl", int64(r.TTL), r.TTL == 0) },
		say:   func(r Rule) string { return words("ttl", int64(r.TTL), r.TTL == 0) },
	},
	{
		name: "badseq", hint: "badseq:100000", kind: counts | forges | spoils,
		read: func(r *Rule, value string, _ bool) error {
			shift, err := strconv.ParseUint(value, 10, 32)
			if err != nil || shift == 0 {
				return fmt.Errorf("rules: badseq wants how far to move the sequence number")
			}

			r.BadSeq = uint32(shift)

			return nil
		},
		write: func(r Rule) string { return number("badseq", int64(r.BadSeq), r.BadSeq == 0) },
		say:   func(r Rule) string { return words("badseq", int64(r.BadSeq), r.BadSeq == 0) },
	},
	{
		name: "badack", hint: "badack:-66000", kind: counts | forges | spoils,
		read: func(r *Rule, value string, _ bool) error {
			shift, err := strconv.ParseInt(value, 10, 32)
			if err != nil || shift == 0 {
				return fmt.Errorf("rules: badack wants how far to move the acknowledgement, usually back, e.g. -66000")
			}

			r.BadAck = int32(shift)

			return nil
		},
		write: func(r Rule) string { return number("badack", int64(r.BadAck), r.BadAck == 0) },
		say:   func(r Rule) string { return words("badack", int64(r.BadAck), r.BadAck == 0) },
	},
	{
		name: "ts", hint: "ts or ts:1000", kind: counts | forges | spoils,
		read: func(r *Rule, value string, given bool) error {
			if !given {
				r.Stale = staleDefault

				return nil
			}

			back, err := strconv.ParseUint(value, 10, 32)
			if err != nil || back == 0 || back > staleMost {
				return fmt.Errorf("rules: ts wants how far back to set the timestamp, 1 to %d", staleMost)
			}

			r.Stale = uint32(back)

			return nil
		},
		write: func(r Rule) string { return number("ts", int64(r.Stale), r.Stale == 0) },
		say: func(r Rule) string {
			return words("timestamp back", int64(r.Stale), r.Stale == 0)
		},
	},
	{
		name: "badsum", hint: "badsum", kind: counts | forges | spoils,
		read:  func(r *Rule, _ string, given bool) error { return bare(&r.BadSum, "badsum", given) },
		write: func(r Rule) string { return flag("badsum", r.BadSum) },
		say:   func(r Rule) string { return flag("badsum", r.BadSum) },
	},
	{
		name: "decoy", hint: "decoy or decoy:mail.ru", kind: counts | forges,
		read: func(r *Rule, value string, given bool) error {
			r.Decoy = "auto"
			if given {
				if value == "" {
					return fmt.Errorf("rules: decoy wants a host name, or nothing at all for one of the right size")
				}

				r.Decoy = value
			}

			return nil
		},
		write: func(r Rule) string { return text("decoy", r.Decoy) },
		say:   func(r Rule) string { return text("decoy", r.Decoy) },
	},
	{
		name: "fake", hint: "fake", kind: counts | forges,
		read:  func(r *Rule, _ string, given bool) error { return bare(&r.Recorded, "fake", given) },
		write: func(r Rule) string { return flag("fake", r.Recorded) },
		say: func(r Rule) string {
			if !r.Recorded {
				return ""
			}

			return "a recorded hello"
		},
	},
	{
		name: "cut", hint: "cut:name|after|start", kind: counts,
		read: func(r *Rule, value string, _ bool) error {
			if value != "name" && value != "after" && value != "start" {
				return ErrCutWhere
			}

			r.Cut = value

			return nil
		},
		write: func(r Rule) string { return text("cut", r.Cut) },
		say: func(r Rule) string {
			if r.Cut == "" {
				return ""
			}

			return "cut at " + r.Cut
		},
	},
	{
		name: "overlap", hint: "overlap:1", kind: counts,
		read: func(r *Rule, value string, _ bool) error {
			at, err := strconv.Atoi(value)
			if err != nil || at < 1 {
				return fmt.Errorf("rules: overlap wants how many real bytes go first, at least 1")
			}

			r.Overlap = at

			return nil
		},
		write: func(r Rule) string { return number("overlap", int64(r.Overlap), r.Overlap == 0) },
		say:   func(r Rule) string { return words("overlap keeping", int64(r.Overlap), r.Overlap == 0) },
	},
	{
		name: "disorder", hint: "disorder", kind: 0,
		read:  func(r *Rule, _ string, given bool) error { return bare(&r.Disorder, "disorder", given) },
		write: func(r Rule) string { return flag("disorder", r.Disorder) },
		say: func(r Rule) string {
			if !r.Disorder {
				return ""
			}

			return "halves back to front"
		},
	},
	{
		name: "repeats", hint: "repeats:5", kind: 0,
		read: func(r *Rule, value string, _ bool) error {
			copies, err := strconv.Atoi(value)
			if err != nil || copies < 1 || copies > 20 {
				return fmt.Errorf("rules: repeats wants how many copies go out, 1 to 20")
			}

			r.Repeats = copies

			return nil
		},
		write: func(r Rule) string { return number("repeats", int64(r.Repeats), r.Repeats == 0) },
		say:   func(r Rule) string { return words("copies", int64(r.Repeats), r.Repeats == 0) },
	},
}

func bare(field *bool, name string, given bool) error {
	if given {
		return fmt.Errorf("rules: %s takes no value", name)
	}

	*field = true

	return nil
}

func number(name string, value int64, unset bool) string {
	if unset {
		return ""
	}

	return fmt.Sprintf("%s:%d", name, value)
}

func words(name string, value int64, unset bool) string {
	if unset {
		return ""
	}

	return fmt.Sprintf("%s %d", name, value)
}

func text(name, value string) string {
	if value == "" {
		return ""
	}

	return name + ":" + value
}

func flag(name string, on bool) string {
	if !on {
		return ""
	}

	return name
}

func gather(r Rule, of func(way) func(Rule) string, only int) []string {
	var out []string

	for _, w := range ways {
		if only != 0 && w.kind&only == 0 {
			continue
		}

		if said := of(w)(r); said != "" {
			out = append(out, said)
		}
	}

	return out
}

// Text writes the rule back the way it was written, so a sweep can print what
// worked and have it pasted straight back in.
func (r Rule) Text() string {
	return strings.Join(gather(r, func(w way) func(Rule) string { return w.write }, 0), ",")
}

func (r Rule) String() string {
	return strings.Join(gather(r, func(w way) func(Rule) string { return w.say }, 0), " + ")
}

// Spoils names what is wrong with the forged copy, as against what is done to
// the real hello.
func (r Rule) Spoils() string {
	said := gather(r, func(w way) func(Rule) string { return w.say }, spoils)
	if len(said) == 0 {
		return "nothing spoiled"
	}

	return strings.Join(said, " + ")
}

func (r Rule) Forges() bool {
	return len(gather(r, func(w way) func(Rule) string { return w.write }, forges)) > 0
}

func (r Rule) Blank() bool {
	return len(gather(r, func(w way) func(Rule) string { return w.write }, counts)) == 0
}

// Ways lists how each way is written, for the command line help.
func Ways() string {
	var out []string

	for _, w := range ways {
		out = append(out, w.hint)
	}

	return strings.Join(out, " ")
}
