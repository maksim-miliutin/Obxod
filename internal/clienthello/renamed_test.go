package clienthello

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// The promise: whatever length the new name has, the hello still parses and the
// name that comes back out is the one we put in.
func TestRenamedSurvivesReparsing(t *testing.T) {
	const was = "updates.discord.com"

	for _, name := range []string{"mail.ru", "a.io", "www.google.com", was, "a-very-long-name.example.co.uk"} {
		t.Run(name, func(t *testing.T) {
			parsed, err := Parse(build(was))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			out, err := parsed.Renamed(name)
			if err != nil {
				t.Fatalf("Renamed: %v", err)
			}

			again, err := Parse(out)
			if err != nil {
				t.Fatalf("the renamed hello does not parse: %v", err)
			}

			got, err := again.ServerName()
			if err != nil {
				t.Fatalf("no name in the renamed hello: %v", err)
			}

			if got.Host != name {
				t.Errorf("name = %q, want %q", got.Host, name)
			}

			if len(out) != len(build(was))+len(name)-len(was) {
				t.Errorf("payload is %d bytes, want the old length shifted by the name", len(out))
			}
		})
	}
}

// The trap this guards: six lengths count the name in, and a renamed hello whose
// record length still claims the old size is read past its own end.
func TestRenamedKeepsEveryLengthTrue(t *testing.T) {
	parsed, err := Parse(build("updates.discord.com"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out, err := parsed.Renamed("mail.ru")
	if err != nil {
		t.Fatalf("Renamed: %v", err)
	}

	again, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if recordHeaderLen+again.RecordLen != len(out) {
		t.Errorf("record claims %d bytes, the payload is %d", recordHeaderLen+again.RecordLen, len(out))
	}

	body := int(out[6])<<16 | int(out[7])<<8 | int(out[8])
	if recordHeaderLen+handshakeHeaderLen+body != len(out) {
		t.Errorf("handshake claims %d bytes, the payload is %d", recordHeaderLen+handshakeHeaderLen+body, len(out))
	}
}

// A hello split across segments declares a record longer than this segment holds.
func split(t *testing.T, host string) Hello {
	t.Helper()

	payload := build(host)
	binary.BigEndian.PutUint16(payload[3:5], binary.BigEndian.Uint16(payload[3:5])+20)

	parsed, err := Parse(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return parsed
}

// Writing lengths for bytes we never saw would leave the hello claiming more than
// it carries.
func TestRenamedRefusesToResizeASplitHello(t *testing.T) {
	if _, err := split(t, "updates.discord.com").Renamed("mail.ru"); err != ErrTruncated {
		t.Errorf("Renamed on a split hello gave %v, want ErrTruncated", err)
	}
}

// The bug this guards: a hello too big for one packet was refused outright, even
// for a name of the same size, which writes no length at all. Discord's own hello
// is that big, so the only decoy that ever worked stopped working.
func TestRenamedTakesASameSizeNameOnASplitHello(t *testing.T) {
	const was = "updates.discord.com"

	out, err := split(t, was).Renamed("xxxxxxxx.google.com")
	if err != nil {
		t.Fatalf("Renamed: %v", err)
	}

	if !bytes.Contains(out, []byte("xxxxxxxx.google.com")) {
		t.Error("the new name is not there")
	}

	if bytes.Contains(out, []byte(was)) {
		t.Error("the real name survived")
	}

	if len(out) != len(build(was)) {
		t.Errorf("payload is %d bytes, want the original %d", len(out), len(build(was)))
	}
}

func TestRenamedRefusesANameThatWillNotFit(t *testing.T) {
	parsed, err := Parse(build("updates.discord.com"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if _, err := parsed.Renamed(strings.Repeat("x", 70000)); err == nil {
		t.Error("a name too long for the length fields was accepted")
	}
}
