package main

import (
	"testing"
)

func TestPlanSpreadsFlagsOverHosts(t *testing.T) {
	set, err := plan(nil, "discord.com, discord.gg", 0, 100000, false, "auto", "name")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if len(set) != 2 {
		t.Fatalf("plan made %d rules, want 2", len(set))
	}

	for _, r := range set {
		if r.BadSeq != 100000 || r.Decoy != "auto" || r.Cut != "name" {
			t.Errorf("rule %+v lost part of the flags", r)
		}
	}
}

func TestPlanPrefersExplicitRules(t *testing.T) {
	set, err := plan([]string{"discord.gg=ttl:2,cut:start"}, "ignored.example", 4, 0, false, "", "")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if len(set) != 1 || set[0].Host != "discord.gg" || set[0].TTL != 2 {
		t.Errorf("plan = %+v, want the rule as written", set)
	}
}

func TestPlanNeedsSomething(t *testing.T) {
	if _, err := plan(nil, "discord.com", 0, 0, false, "", ""); err == nil {
		t.Error("a plan with no way to bypass was accepted")
	}
}

func TestParseHostsDropsBlanks(t *testing.T) {
	if got := parseHosts(" , ,"); len(got) != 0 {
		t.Errorf("parseHosts = %v, want nothing", got)
	}
}

func TestParsePorts(t *testing.T) {
	got, err := parsePorts("443, 2053 ,8443")
	if err != nil {
		t.Fatalf("parsePorts: %v", err)
	}

	want := []uint16{443, 2053, 8443}

	if len(got) != len(want) {
		t.Fatalf("parsePorts gave %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("port %d = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestParsePortsRejectsNonsense(t *testing.T) {
	for _, text := range []string{"443,http", "70000", "-1"} {
		t.Run(text, func(t *testing.T) {
			if _, err := parsePorts(text); err == nil {
				t.Error("nonsense was accepted")
			}
		})
	}
}

// The overlap reaches back over the whole recording unless told how far, and a
// zero there used to mean zero bytes.
func TestSpreadCoversTheWholeRecordingByDefault(t *testing.T) {
	if got := spread([]byte("abcd"), 0); string(got) != "abcd" {
		t.Errorf("spread gave %q, want the recording whole", got)
	}
}

func TestSpreadStretchesToSeqovl(t *testing.T) {
	got := spread([]byte("abcd"), 9)

	if len(got) != 9 {
		t.Fatalf("spread gave %d bytes, want the 9 asked for", len(got))
	}

	if string(got[:4]) != "abcd" {
		t.Errorf("spread starts with %q, want the recorded bytes", got[:4])
	}
}

func TestSpreadCutsWhenSeqovlIsShorter(t *testing.T) {
	if got := spread([]byte("abcdefgh"), 3); string(got) != "abc" {
		t.Errorf("spread gave %q, want only as far back as asked", got)
	}
}

func TestSpreadOnNothingRecorded(t *testing.T) {
	if got := spread(nil, 0); len(got) != 0 {
		t.Errorf("spread gave %d bytes from nothing", len(got))
	}
}

// A rules file is edited by hand, so it has to tolerate notes, blank lines and
// stray spaces without turning them into rules that fail to parse.
func TestWrittenPicksOutTheRules(t *testing.T) {
	const file = `
# what works here, measured against one provider
discord.com=hostfake:mail.ru,ts

  discord.gg=hostfake:mail.ru,ts   

youtube.com=hostfake:mail.ru,ts  # the browser wants -noquic for this one
#discordapp.com=decoy
`

	got := written(file)

	want := []string{
		"discord.com=hostfake:mail.ru,ts",
		"discord.gg=hostfake:mail.ru,ts",
		"youtube.com=hostfake:mail.ru,ts",
	}

	if len(got) != len(want) {
		t.Fatalf("picked %d rules, want %d: %q", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rule %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWrittenFindsNothingInAnEmptyFile(t *testing.T) {
	for _, text := range []string{"", "\n\n", "# only a note\n", "   \n\t\n"} {
		if got := written(text); len(got) != 0 {
			t.Errorf("picked %q out of %q", got, text)
		}
	}
}

// Whatever the file holds has to survive being parsed as a rule.
func TestWhatIsWrittenParses(t *testing.T) {
	const file = "all=hostfake:mail.ru,ts\ndiscord.com=decoy,badseq:100000,cut:name\n"

	set, err := plan(written(file), "", 0, 0, false, "", "")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if len(set) != 2 {
		t.Fatalf("plan made %d rules, want 2", len(set))
	}
}
