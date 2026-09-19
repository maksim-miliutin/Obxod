package bypass

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"obxod/internal/attempt"
	"obxod/internal/cut"
	"obxod/internal/divert"
	"obxod/internal/forge"
	"obxod/internal/hello"
	"obxod/internal/ip"
	"obxod/internal/rules"
	"obxod/internal/seal"
	"obxod/internal/tcp"
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
		e.noRule(found.Host)

		return false, nil
	}

	j := job{to: h, packet: packet, addr: addr, found: found, rule: r}

	repeat := e.tries.Saw(found.Host, found.SrcPort, found.Seq, time.Now()) == attempt.Again

	if repeat {
		e.say("  %s: asking again, so this way is not getting through", found.Host)
	}

	e.health.Hello(found.Host, found.SrcPort)

	if e.hunt != nil && e.hunt.Host() == found.Host {
		e.hunt.Saw(repeat)
	}

	// Swapping the name carries the spoiling itself, on the segment that holds the
	// name. A whole forged copy on top of that is a second hello the server has to
	// throw away, and it is not what the reference sends.
	if r.HostFake != "" {
		return e.hostfake(j)
	}

	// The decoy goes first and the real hello follows, cut or whole: an inspector
	// that reads the decoy and then finds no name in either half has nothing to match.
	if r.Forges() {
		if err := e.fake(j); err != nil {
			return false, err
		}
	}

	if r.Overlap != 0 {
		return e.overlay(j)
	}

	if r.Cut != "" {
		return e.split(j)
	}

	return false, nil
}

// What one hello asks for: where it goes, what it says, and the rule that covers
// it. Every step of the path wants all of it, and none of it alone.
type job struct {
	to     sender
	packet []byte
	addr   *divert.Addr
	found  hello.Outgoing
	rule   rules.Rule
}

func (e *Engine) fake(j job) error {
	recipe := forge.Recipe{TTL: j.rule.TTL, SeqDelta: j.rule.BadSeq, AckDelta: j.rule.BadAck, Stale: j.rule.Stale, BadSum: j.rule.BadSum}

	if j.rule.Recorded {
		return e.canned(j, recipe)
	}

	if j.rule.Decoy != "" {
		name := j.rule.Decoy
		if name == "auto" {
			name = decoyFor(j.found.Host)
		}

		recipe.Name = name
	}

	copied, err := forge.Copy(j.packet, recipe)
	if err != nil {
		e.say("  %s: cannot copy: %v", j.found.Host, err)

		return nil
	}

	copies := max(1, j.rule.Repeats)

	if !e.wet {
		e.say("  %s: would send a %d byte copy (%s%s%s)", j.found.Host, len(copied), j.rule.Spoils(), wearing(recipe.Name), times(copies))

		return nil
	}

	e.say("  %s: copy sent ahead (%s%s%s)", j.found.Host, j.rule.Spoils(), wearing(recipe.Name), times(copies))

	for range copies {
		if err := j.to.Send(copied, j.addr); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) split(j job) (bool, error) {
	point, err := pointFor(j.found, j.rule.Cut)
	if err != nil {
		return false, err
	}

	first, second, err := cut.At(j.packet, point)
	if err != nil {
		e.say("  %s: cannot split: %v", j.found.Host, err)

		return false, nil
	}

	if !e.wet {
		e.say("  %s: would split into %d and %d bytes at %s", j.found.Host, len(first), len(second), j.rule.Cut)

		return false, nil
	}

	e.say("  %s: split into %d and %d bytes at %s%s", j.found.Host, len(first), len(second), j.rule.Cut, backwards(j.rule.Disorder))

	first, second = ordered(first, second, j.rule.Disorder)

	if err := j.to.Send(first, j.addr); err != nil {
		return false, err
	}

	return true, j.to.Send(second, j.addr)
}

func (e *Engine) overlay(j job) (bool, error) {
	first, second, err := cut.Overlap(j.packet, e.pattern, j.rule.Overlap)
	if err != nil {
		e.say("  %s: cannot overlap: %v", j.found.Host, err)

		return false, nil
	}

	if !e.wet {
		e.say("  %s: would lay %d recorded bytes over the stream, then %d and %d bytes",
			j.found.Host, len(e.pattern), len(first), len(second))

		return false, nil
	}

	e.say("  %s: %d recorded bytes laid over, then %d and %d bytes%s",
		j.found.Host, len(e.pattern), len(first), len(second), backwards(j.rule.Disorder))

	first, second = ordered(first, second, j.rule.Disorder)

	if err := j.to.Send(first, j.addr); err != nil {
		return false, err
	}

	return true, j.to.Send(second, j.addr)
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

func (e *Engine) canned(j job, recipe forge.Recipe) error {
	made, err := forge.Instead(j.packet, e.recorded, recipe)
	if err != nil {
		e.say("  %s: cannot use the recorded hello: %v", j.found.Host, err)

		return nil
	}

	copies := max(1, j.rule.Repeats)

	if !e.wet {
		e.say("  %s: would send %d recorded bytes (%s%s)", j.found.Host, len(e.recorded), j.rule.Spoils(), times(copies))

		return nil
	}

	e.say("  %s: %d recorded bytes sent ahead (%s%s)", j.found.Host, len(e.recorded), j.rule.Spoils(), times(copies))

	for range copies {
		if err := j.to.Send(made, j.addr); err != nil {
			return err
		}
	}

	return nil
}

// An inspector reads the halves in the order they arrive and finds the hello cut
// open at the wrong end; the server puts them back by sequence number regardless.
func ordered(first, second []byte, backwards bool) ([]byte, []byte) {
	if backwards {
		return second, first
	}

	return first, second
}

func backwards(on bool) string {
	if !on {
		return ""
	}

	return ", back to front"
}

// worn cuts or pads the wanted name to the size of the real one, the way the
// reference does: a longer name keeps its tail, a shorter one gets a subdomain.
func worn(real, want string) string {
	if want == "auto" {
		want = decoyFor(real)
	}

	if len(want) >= len(real) {
		return want[len(want)-len(real):]
	}

	// A run of one letter reads as nothing anybody would register, and an inspector
	// looking for a plausible name has an easy time throwing it out.
	pad := make([]byte, len(real)-len(want))
	for i := range pad[:len(pad)-1] {
		pad[i] = byte('a' + rand.IntN(26))
	}

	pad[len(pad)-1] = '.'

	return string(pad) + want
}

// hostfake puts a made up name where the real one sits, then writes the real one
// over it. What arrives first carries the wrong name; what the server puts back
// together by sequence number carries the right one.
func (e *Engine) hostfake(j job) (bool, error) {
	outer, err := ip.Parse(j.packet)
	if err != nil {
		return false, err
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return false, err
	}

	payload := segment.Payload

	if j.found.NameEnd > len(payload) {
		e.say("  %s: the name runs past this j.packet, cannot swap it", j.found.Host)

		return false, nil
	}

	part := func(bytes []byte, at int) []byte {
		if err != nil {
			return nil
		}

		var made []byte
		made, err = seal.Remade(j.packet, bytes, segment.Seq+uint32(at))

		return made
	}

	before := part(payload[:j.found.NameStart], 0)
	real := part(payload[j.found.NameStart:j.found.NameEnd], j.found.NameStart)
	after := part(payload[j.found.NameEnd:], j.found.NameEnd)

	name := worn(j.found.Host, j.rule.HostFake)

	wrong, made := forge.Instead(j.packet, []byte(name), forge.Recipe{
		TTL:      j.rule.TTL,
		SeqDelta: int32(j.found.NameStart) + j.rule.BadSeq,
		AckDelta: j.rule.BadAck,
		Stale:    j.rule.Stale,
		BadSum:   j.rule.BadSum,
	})

	if err == nil {
		err = made
	}

	if err != nil {
		e.say("  %s: cannot swap the name: %v", j.found.Host, err)

		return false, nil
	}

	if !e.wet {
		e.say("  %s: would swap the name for %s (%s)", j.found.Host, name, j.rule.Spoils())

		return false, nil
	}

	e.say("  %s: name swapped for %s (%s)%s", j.found.Host, name, j.rule.Spoils(), backwards(j.rule.Disorder))

	out := [][]byte{before}

	for range max(1, j.rule.Repeats) {
		out = append(out, wrong)
	}

	// The reference sends what follows the name before writing the real name back
	// when asked to; otherwise after it.
	if j.rule.Disorder {
		out = append(out, after, real)
	} else {
		out = append(out, real, after)
	}

	for _, one := range out {
		if len(one) == 0 {
			continue
		}

		if err := j.to.Send(one, j.addr); err != nil {
			return true, err
		}
	}

	return true, nil
}

// A site living on a name nobody wrote a rule for is invisible: the j.packet goes
// out untouched and nothing is said. Naming it once is how the rule gets written.
func (e *Engine) noRule(host string) {
	if e.seen == nil || e.seen[host] {
		return
	}

	e.seen[host] = true

	e.say("  %s: no rule covers this name", host)
}
