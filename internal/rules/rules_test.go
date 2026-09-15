package rules

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		text string
		want Rule
	}{
		{
			"the one that beat updates",
			"discord.com=decoy,badseq:100000,cut:name",
			Rule{Host: "discord.com", BadSeq: 100000, Decoy: "auto", Cut: "name"},
		},
		{
			"ttl and a cut",
			"discord.gg=ttl:2,cut:start",
			Rule{Host: "discord.gg", TTL: 2, Cut: "start"},
		},
		{
			"a decoy by name",
			"ya.ru=decoy:vk.com",
			Rule{Host: "ya.ru", Decoy: "vk.com"},
		},
		{
			"badsum alone",
			"example.com=badsum",
			Rule{Host: "example.com", BadSum: true},
		},
		{
			"spaces and case do not matter",
			"  Discord.GG = ttl:4 , badsum  ",
			Rule{Host: "discord.gg", TTL: 4, BadSum: true},
		},
		{
			"everything at once",
			"all=ttl:1,badseq:5,badsum,decoy,cut:after",
			Rule{Host: "all", TTL: 1, BadSeq: 5, BadSum: true, Decoy: "auto", Cut: "after"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if got != c.want {
				t.Errorf("\n got %+v\nwant %+v", got, c.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		text string
		want error
	}{
		{"no equals sign", "discord.com ttl:4", ErrNoHost},
		{"no host", "=ttl:4", ErrNoHost},
		{"nothing asked for", "discord.com=", ErrNoWay},
		{"cut sideways", "discord.com=cut:sideways", ErrCutWhere},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse(c.text); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestParseRejectsNonsense(t *testing.T) {
	for _, text := range []string{
		"discord.com=ttl:many",
		"discord.com=badseq:-1",
		"discord.com=ttl:999",
		"discord.com=flip",
	} {
		t.Run(text, func(t *testing.T) {
			if _, err := Parse(text); err == nil {
				t.Error("nonsense was accepted")
			}
		})
	}
}

func TestForPicksTheLongestMatch(t *testing.T) {
	set, err := ParseAll([]string{
		"discord.com=badseq:100000,decoy,cut:name",
		"gateway.discord.gg=ttl:2,cut:start",
		"discord.gg=badsum",
	})
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	cases := map[string]string{
		"updates.discord.com": "discord.com",
		"discord.com":         "discord.com",
		"gateway.discord.gg":  "gateway.discord.gg",
		"media.discord.gg":    "discord.gg",
	}

	for host, want := range cases {
		t.Run(host, func(t *testing.T) {
			got, ok := For(set, host)
			if !ok {
				t.Fatal("no rule matched")
			}

			if got.Host != want {
				t.Errorf("rule for %q is %q, want %q", host, got.Host, want)
			}
		})
	}
}

func For(s Set, host string) (Rule, bool) {
	return s.For(host)
}

func TestForMisses(t *testing.T) {
	set, err := ParseAll([]string{"discord.com=badsum"})
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	for _, host := range []string{"ya.ru", "notdiscord.com", "discord.com.evil.net"} {
		t.Run(host, func(t *testing.T) {
			if _, ok := set.For(host); ok {
				t.Error("a rule matched a host it should not")
			}
		})
	}
}

func TestForAllCatchesEverything(t *testing.T) {
	set, err := ParseAll([]string{"all=decoy,cut:name"})
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	for _, host := range []string{"ya.ru", "gateway.discord.gg", "anything.example"} {
		if _, ok := set.For(host); !ok {
			t.Errorf("%q matched no rule under all", host)
		}
	}
}

func TestParseAllReportsWhichRuleIsWrong(t *testing.T) {
	_, err := ParseAll([]string{"discord.com=badsum", "broken"})
	if err == nil {
		t.Fatal("a broken rule went through")
	}

	if !errors.Is(err, ErrNoHost) {
		t.Errorf("err = %v, want it to carry %v", err, ErrNoHost)
	}
}

func TestParseOverlap(t *testing.T) {
	got, err := Parse("gateway.discord.gg=overlap:1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.Overlap != 1 {
		t.Errorf("Overlap = %d, want 1", got.Overlap)
	}
}

func TestParseOverlapRejectsNonsense(t *testing.T) {
	for _, text := range []string{
		"a=overlap:0",
		"a=overlap:-1",
		"a=overlap:some",
		"a=overlap",
	} {
		t.Run(text, func(t *testing.T) {
			if _, err := Parse(text); err == nil {
				t.Error("nonsense was accepted")
			}
		})
	}
}

func TestBlankCountsEveryWay(t *testing.T) {
	if !(Rule{Host: "a"}).Blank() {
		t.Error("a rule with nothing set was not called blank")
	}

	// Every way, one at a time: a new field forgotten in Blank shows up here.
	filled := []Rule{
		{TTL: 1},
		{BadSeq: 1},
		{BadSum: true},
		{Decoy: "auto"},
		{Cut: "name"},
		{Overlap: 1},
	}

	for _, r := range filled {
		if r.Blank() {
			t.Errorf("%+v was called blank", r)
		}
	}
}

func TestRepeatsParses(t *testing.T) {
	r, err := Parse("gateway.discord.gg=badseq:100000,repeats:5")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Repeats != 5 {
		t.Errorf("Repeats = %d, want 5", r.Repeats)
	}
}

func TestRepeatsRejectsNonsense(t *testing.T) {
	for _, text := range []string{"repeats:0", "repeats:-1", "repeats:many", "repeats:21", "repeats"} {
		t.Run(text, func(t *testing.T) {
			if _, err := Parse("discord.com=badsum," + text); err == nil {
				t.Error("nonsense was accepted")
			}
		})
	}
}

// Repeats multiplies a copy, it does not make one: a rule that only repeats
// spoils nothing and must not pass for a way to bypass anything.
func TestRepeatsAloneIsNotAWay(t *testing.T) {
	if _, err := Parse("discord.com=repeats:5"); !errors.Is(err, ErrNoWay) {
		t.Errorf("Parse gave %v, want ErrNoWay", err)
	}
}

func TestRepeatsIsAbsentByDefault(t *testing.T) {
	r, err := Parse("discord.com=badsum")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Repeats != 0 {
		t.Errorf("Repeats = %d, want nothing asked for", r.Repeats)
	}
}

func TestFakeIsAWayOnItsOwn(t *testing.T) {
	r, err := Parse("discord.com=fake")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !r.Recorded {
		t.Error("fake did not ask for the recorded hello")
	}
}

// The file comes from -fake, not from the rule: a windows path carries a colon
// and would be read as another way.
func TestFakeTakesNoValue(t *testing.T) {
	if _, err := Parse(`discord.com=fake:C:\hello.bin`); err == nil {
		t.Error("a path inside the rule was accepted")
	}
}
