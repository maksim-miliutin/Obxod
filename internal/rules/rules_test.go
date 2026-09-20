package rules

import (
	"errors"
	"strings"
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

func TestBadAckTakesEitherDirection(t *testing.T) {
	cases := map[string]int32{"badack:-66000": -66000, "badack:66000": 66000}

	for text, want := range cases {
		t.Run(text, func(t *testing.T) {
			r, err := Parse("discord.com=" + text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if r.BadAck != want {
				t.Errorf("BadAck = %d, want %d", r.BadAck, want)
			}
		})
	}
}

func TestBadAckRejectsNonsense(t *testing.T) {
	for _, text := range []string{"badack:0", "badack:far", "badack", "badack:5000000000"} {
		t.Run(text, func(t *testing.T) {
			if _, err := Parse("discord.com=badsum," + text); err == nil {
				t.Error("nonsense was accepted")
			}
		})
	}
}

func TestTsWithoutAValueTakesTheDefault(t *testing.T) {
	r, err := Parse("discord.com=ts")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Stale != staleDefault {
		t.Errorf("Stale = %d, want the default %d", r.Stale, staleDefault)
	}
}

func TestTsTakesAnExplicitShift(t *testing.T) {
	r, err := Parse("discord.com=ts:10000")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Stale != 10000 {
		t.Errorf("Stale = %d, want 10000", r.Stale)
	}
}

// Past half the timestamp space the subtraction wraps into the future, and a
// copy from the future is not old at all.
func TestTsRefusesAShiftThatWrapsForward(t *testing.T) {
	for _, text := range []string{"ts:0", "ts:2147483648", "ts:soon", "ts:-5"} {
		t.Run(text, func(t *testing.T) {
			if _, err := Parse("discord.com=badsum," + text); err == nil {
				t.Error("nonsense was accepted")
			}
		})
	}
}

// Every way, from the list itself: a row added there is covered here without
// anyone remembering to add a case.
func TestEveryWayWritesBackAndParsesAgain(t *testing.T) {
	samples := map[string]string{
		"ttl": "ttl:4", "badseq": "badseq:100000", "badack": "badack:-66000",
		"ts": "ts:1000", "badsum": "badsum", "decoy": "decoy:mail.ru",
		"fake": "fake", "hostfake": "hostfake:mail.ru", "cut": "cut:name", "overlap": "overlap:1",
		"disorder": "disorder", "repeats": "repeats:5",
	}

	for _, w := range ways {
		t.Run(w.name, func(t *testing.T) {
			sample, ok := samples[w.name]
			if !ok {
				t.Fatalf("no sample for way %q; add one when adding the way", w.name)
			}

			was, err := Parse("discord.com=badsum," + sample)
			if err != nil {
				t.Fatalf("Parse %q: %v", sample, err)
			}

			back, err := Parse("discord.com=" + was.Text())
			if err != nil {
				t.Fatalf("%q does not parse back: %v", was.Text(), err)
			}

			if back != was {
				t.Errorf("\n got %+v\nwant %+v\nvia %q", back, was, was.Text())
			}

			if !strings.Contains(was.String(), "") || was.String() == "" {
				t.Error("the rule says nothing about itself")
			}
		})
	}
}

// The help offers every way and nothing that is not one.
func TestWaysListsThemAll(t *testing.T) {
	hint := Ways()

	for _, w := range ways {
		if !strings.Contains(hint, w.name) {
			t.Errorf("the help does not offer %q", w.name)
		}
	}
}

func TestOnlyTheRightWaysCountAsOne(t *testing.T) {
	cases := map[string]bool{
		"ttl:4": false, "badseq:2": false, "badack:-1": false, "ts": false,
		"badsum": false, "decoy": false, "fake": false, "hostfake": false, "cut:name": false, "overlap:1": false,
		"disorder": true, "repeats:5": true,
	}

	for text, blank := range cases {
		t.Run(text, func(t *testing.T) {
			_, err := Parse("discord.com=" + text)

			if blank != errors.Is(err, ErrNoWay) {
				t.Errorf("%q alone: err = %v, blank expected = %v", text, err, blank)
			}
		})
	}
}

// Backwards is the direction the reference uses by default, and it was not
// expressible at all while the field was unsigned.
func TestBadSeqGoesEitherWay(t *testing.T) {
	cases := map[string]int32{"badseq:-10000": -10000, "badseq:100000": 100000, "badseq:-1": -1}

	for text, want := range cases {
		t.Run(text, func(t *testing.T) {
			r, err := Parse("discord.com=" + text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if r.BadSeq != want {
				t.Errorf("BadSeq = %d, want %d", r.BadSeq, want)
			}
		})
	}
}

// The number matters: too far back and the copy stops reading as a real
// timestamp, too far and it wraps into the future and is not old at all.
func TestTheDefaultTimestampShiftIsTenMinutesOfTicks(t *testing.T) {
	r, err := Parse("discord.com=ts")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Stale != 600000 {
		t.Errorf("Stale = %d, want 600000", r.Stale)
	}

	if r.Stale > staleMost {
		t.Error("the default shift wraps forward")
	}
}

// A site lives on a dozen names that all want the same treatment, and writing
// the ways out a dozen times is how one of them ends up different by mistake.
func TestSeveralHostsShareOneLine(t *testing.T) {
	set, err := Several("youtube.com, ytimg.com ,ggpht.com=hostfake:mail.ru,ts")
	if err != nil {
		t.Fatalf("Several: %v", err)
	}

	if len(set) != 3 {
		t.Fatalf("made %d rules, want 3", len(set))
	}

	for _, r := range set {
		if r.HostFake != "mail.ru" || r.Stale == 0 {
			t.Errorf("%s got %+v, want the same ways as the rest", r.Host, r)
		}
	}

	if set[0].Host != "youtube.com" || set[1].Host != "ytimg.com" || set[2].Host != "ggpht.com" {
		t.Errorf("hosts came out as %q, %q, %q", set[0].Host, set[1].Host, set[2].Host)
	}
}

func TestOneHostStillWorks(t *testing.T) {
	set, err := Several("discord.com=hostfake:mail.ru,ts")
	if err != nil {
		t.Fatalf("Several: %v", err)
	}

	if len(set) != 1 || set[0].Host != "discord.com" {
		t.Errorf("made %+v, want the one rule", set)
	}
}

func TestAnEmptyHostInTheListIsRefused(t *testing.T) {
	for _, text := range []string{"a.com,=ts", ",a.com=ts", "a.com,,b.com=ts", "=ts"} {
		if _, err := Several(text); !errors.Is(err, ErrNoHost) {
			t.Errorf("%q gave %v, want ErrNoHost", text, err)
		}
	}
}

// Parse makes one rule, so a list handed to it would quietly become a host name
// with a comma in it that matches nothing.
func TestParseRefusesAList(t *testing.T) {
	if _, err := Parse("a.com,b.com=ts"); !errors.Is(err, ErrManyHosts) {
		t.Errorf("Parse took a list: %v", err)
	}
}
