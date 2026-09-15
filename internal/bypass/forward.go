package bypass

import (
	"fmt"
	"strings"
	"time"

	"obxod/internal/attempt"
	"obxod/internal/cut"
	"obxod/internal/divert"
	"obxod/internal/forge"
	"obxod/internal/hello"
	"obxod/internal/rules"
)

// sender is what the divert handle gives us, narrowed to the one call these take,
// so the order they send in can be checked without a driver.
type sender interface {
	Send(packet []byte, addr *divert.Addr) error
}

// forward returns true when it already put the packet on the wire itself, which
// happens for a cut: the original must not follow its own halves.
func (e *Engine) forward(h sender, packet []byte, addr *divert.Addr) (bool, error) {
	found, ok := hello.Found(packet)
	if !ok {
		return false, nil
	}

	r, ok := e.set.For(found.Host)
	if !ok {
		return false, nil
	}

	repeat := e.tries.Saw(found.Host, found.SrcPort, found.Seq, time.Now()) == attempt.Again

	if repeat {
		e.say("  %s: asking again, so this way is not getting through", found.Host)
	}

	e.health.Hello(found.Host, found.SrcPort, time.Now())

	if e.hunt != nil && e.hunt.Host() == found.Host {
		e.hunt.Saw(repeat)
	}

	// The decoy goes first and the real hello follows, cut or whole: an inspector
	// that reads the decoy and then finds no name in either half has nothing to match.
	if r.TTL != 0 || r.BadSeq != 0 || r.BadSum || r.Decoy != "" {
		if err := e.fake(h, packet, addr, found, r); err != nil {
			return false, err
		}
	}

	if r.Overlap != 0 {
		return e.overlay(h, packet, addr, found, r)
	}

	if r.Cut != "" {
		return e.split(h, packet, addr, found, r.Cut)
	}

	return false, nil
}

func (e *Engine) fake(h sender, packet []byte, addr *divert.Addr, found hello.Outgoing, r rules.Rule) error {
	recipe := forge.Recipe{TTL: r.TTL, SeqDelta: r.BadSeq, BadSum: r.BadSum}

	if r.Decoy != "" {
		name := r.Decoy
		if name == "auto" {
			name = decoyFor(found.Host)
		}

		if len(name) != len(found.Host) {
			return fmt.Errorf("bypass: decoy %q is %d bytes, the real name is %d: they must match", name, len(name), len(found.Host))
		}

		recipe.Name = name
		recipe.NameAt = found.NameStart
	}

	copied, err := forge.Copy(packet, recipe)
	if err != nil {
		e.say("  %s: cannot copy: %v", found.Host, err)

		return nil
	}

	copies := max(1, r.Repeats)

	if !e.wet {
		e.say("  %s: would send a %d byte copy (%s%s%s)", found.Host, len(copied), spoils(r.TTL, r.BadSeq, r.BadSum), wearing(recipe.Name), times(copies))

		return nil
	}

	e.say("  %s: copy sent ahead (%s%s%s)", found.Host, spoils(r.TTL, r.BadSeq, r.BadSum), wearing(recipe.Name), times(copies))

	for range copies {
		if err := h.Send(copied, addr); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) split(h sender, packet []byte, addr *divert.Addr, found hello.Outgoing, where string) (bool, error) {
	point, err := pointFor(found, where)
	if err != nil {
		return false, err
	}

	first, second, err := cut.At(packet, point)
	if err != nil {
		e.say("  %s: cannot split: %v", found.Host, err)

		return false, nil
	}

	if !e.wet {
		e.say("  %s: would split into %d and %d bytes at %s", found.Host, len(first), len(second), where)

		return false, nil
	}

	e.say("  %s: split into %d and %d bytes at %s", found.Host, len(first), len(second), where)

	if err := h.Send(first, addr); err != nil {
		return false, err
	}

	return true, h.Send(second, addr)
}

func (e *Engine) overlay(h sender, packet []byte, addr *divert.Addr, found hello.Outgoing, r rules.Rule) (bool, error) {
	first, second, err := cut.Overlap(packet, e.pattern, r.Overlap)
	if err != nil {
		e.say("  %s: cannot overlap: %v", found.Host, err)

		return false, nil
	}

	if !e.wet {
		e.say("  %s: would lay %d recorded bytes over the stream, then %d and %d bytes",
			found.Host, len(e.pattern), len(first), len(second))

		return false, nil
	}

	e.say("  %s: %d recorded bytes laid over, then %d and %d bytes",
		found.Host, len(e.pattern), len(first), len(second))

	if err := h.Send(first, addr); err != nil {
		return false, err
	}

	return true, h.Send(second, addr)
}

func pointFor(found hello.Outgoing, where string) (int, error) {
	switch where {
	case "name":
		// Through the middle of the name: neither packet holds it whole, which is
		// what defeats an inspector that reads packets one by one.
		return found.NameStart + len(found.Host)/2, nil
	case "after":
		return found.NameEnd, nil
	case "start":
		return 2, nil
	}

	return 0, fmt.Errorf("bypass: unknown cut %q: use name, after or start", where)
}

// decoyFor builds a harmless name exactly as long as the real one, because the
// lengths inside a hello count the name and a copy must keep them true.
func decoyFor(host string) string {
	const base = "google.com"

	if len(host) < len(base)+2 {
		return strings.Repeat("a", len(host)-4) + ".com"
	}

	return strings.Repeat("x", len(host)-len(base)-1) + "." + base
}

func spoils(ttl uint8, badseq uint32, badsum bool) string {
	var named []string

	if ttl != 0 {
		named = append(named, fmt.Sprintf("ttl %d", ttl))
	}

	if badseq != 0 {
		named = append(named, fmt.Sprintf("badseq %d", badseq))
	}

	if badsum {
		named = append(named, "badsum")
	}

	if len(named) == 0 {
		return "nothing spoiled"
	}

	return strings.Join(named, " + ")
}

func wearing(name string) string {
	if name == "" {
		return ""
	}

	return ", wearing " + name
}

func times(copies int) string {
	if copies == 1 {
		return ""
	}

	return fmt.Sprintf(", %d times", copies)
}
