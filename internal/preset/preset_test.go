package preset

import (
	"strings"
	"testing"
)

func TestEveryPresetParses(t *testing.T) {
	for _, p := range All() {
		set, err := p.Rules()
		if err != nil {
			t.Errorf("%s: %v", p.Key, err)

			continue
		}

		if len(set) == 0 {
			t.Errorf("%s: no rules", p.Key)
		}
	}
}

func TestDefaultCoversTheBlockedSites(t *testing.T) {
	set, err := All()[0].Rules()
	if err != nil {
		t.Fatal(err)
	}

	for _, host := range []string{"discord.com", "youtube.com", "x.com"} {
		if _, ok := set.For(host); !ok {
			t.Errorf("the default preset misses %s", host)
		}
	}
}

func TestNamedFindsAndRejects(t *testing.T) {
	if _, ok := Named("plain"); !ok {
		t.Error("Named missed a real preset")
	}

	if _, ok := Named("nope"); ok {
		t.Error("Named returned a preset that does not exist")
	}
}

func TestNamesCoverEveryPreset(t *testing.T) {
	if len(Names("ru")) != len(All()) {
		t.Fatalf("Names %d, All %d", len(Names("ru")), len(All()))
	}
}

func TestHostsAreTheBlockedSites(t *testing.T) {
	hosts := Hosts()

	if len(hosts) != strings.Count(blocked, ",")+1 {
		t.Fatalf("got %d hosts", len(hosts))
	}

	set, _ := All()[0].Rules()

	for _, h := range hosts {
		if _, ok := set.For(h); !ok {
			t.Errorf("host %s is listed but not in the rules", h)
		}
	}
}

func TestRulesWithKeepsBothPresetAndExtra(t *testing.T) {
	set, err := All()[0].RulesWith([]string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := set.For("example.com"); !ok {
		t.Error("RulesWith dropped the extra host")
	}

	if _, ok := set.For("discord.com"); !ok {
		t.Error("RulesWith dropped the preset hosts")
	}
}

func TestRulesVoiceChangesTheDiscordRule(t *testing.T) {
	for _, p := range All() {
		plain, err := p.Rules()
		if err != nil {
			t.Fatalf("%s: %v", p.Key, err)
		}

		loud, err := p.RulesVoice(nil)
		if err != nil {
			t.Fatalf("%s: %v", p.Key, err)
		}

		before, _ := plain.For("discord.media")
		after, _ := loud.For("discord.media")

		if before.String() == after.String() {
			t.Errorf("%s: voice left the discord rule unchanged (%q)", p.Key, after.String())
		}
	}
}

func TestBlockedCoversTheAddedSites(t *testing.T) {
	set, err := All()[0].Rules()
	if err != nil {
		t.Fatal(err)
	}

	for _, host := range []string{"instagram.com", "fbcdn.net", "web.telegram.org", "telegram.org"} {
		if _, ok := set.For(host); !ok {
			t.Errorf("preset misses %s", host)
		}
	}
}

func TestBlockedCoversTheNewServices(t *testing.T) {
	set, err := All()[0].Rules()
	if err != nil {
		t.Fatal(err)
	}

	for _, host := range []string{"twitter.com", "facebook.com", "linkedin.com", "soundcloud.com"} {
		if _, ok := set.For(host); !ok {
			t.Errorf("preset misses %s", host)
		}
	}
}

func TestByNameRoundTripsInBothLanguages(t *testing.T) {
	for _, p := range All() {
		if got, ok := ByName(p.Name("ru"), "ru"); !ok || got.Key != p.Key {
			t.Errorf("ByName ru failed for %s", p.Key)
		}

		if got, ok := ByName(p.Name("en"), "en"); !ok || got.Key != p.Key {
			t.Errorf("ByName en failed for %s", p.Key)
		}
	}
}

func TestNameDiffersByLanguage(t *testing.T) {
	p := All()[0]
	if p.Name("ru") == p.Name("en") {
		t.Errorf("name is the same in both languages: %q", p.Name("ru"))
	}
}
