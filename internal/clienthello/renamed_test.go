package clienthello

import (
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

// A hello split across segments declares more than it carries, and renaming what
// we cannot see would write lengths for bytes that are not there.
func TestRenamedRefusesATruncatedRecord(t *testing.T) {
	whole := build("updates.discord.com")

	parsed, err := Parse(whole[:len(whole)-10])
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if _, err := parsed.Renamed("mail.ru"); err != ErrTruncated {
		t.Errorf("Renamed on a cut record gave %v, want ErrTruncated", err)
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
