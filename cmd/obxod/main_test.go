package main

import (
	"os"
	"path/filepath"
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

// Without a pattern there is nothing to lay over, and an overlap rule must not
// silently get an empty one that reaches back zero bytes.
func TestLoadPatternWithoutAFileGivesNothing(t *testing.T) {
	pattern, err := loadPattern("", 0)
	if err != nil || pattern != nil {
		t.Errorf("loadPattern gave %v, %v, want nothing", pattern, err)
	}
}

func TestLoadPatternStretchesToSeqovl(t *testing.T) {
	name := filepath.Join(t.TempDir(), "hello.bin")

	if err := os.WriteFile(name, []byte("abcd"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pattern, err := loadPattern(name, 9)
	if err != nil {
		t.Fatalf("loadPattern: %v", err)
	}

	if len(pattern) != 9 {
		t.Errorf("pattern is %d bytes, want the 9 asked for", len(pattern))
	}

	if string(pattern[:4]) != "abcd" {
		t.Errorf("pattern starts with %q, want the recorded bytes", pattern[:4])
	}
}

func TestLoadPatternReportsAMissingFile(t *testing.T) {
	if _, err := loadPattern(filepath.Join(t.TempDir(), "gone.bin"), 0); err == nil {
		t.Error("a missing pattern file was accepted")
	}
}
