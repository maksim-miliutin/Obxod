package preset

import (
	"strings"
	"testing"

	"obxod/internal/rules"
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

// The pure-desync presets must actually carry their way, or a typo in the way string
// would leave a method in the list that quietly does nothing.
func TestDesyncPresetsCarryTheirWay(t *testing.T) {
	carries := map[string]func(rules.Rule) bool{
		"scatter":    func(r rules.Rule) bool { return r.MultiDisorder != 0 },
		"multisplit": func(r rules.Rule) bool { return r.MultiSplit != "" },
		"ipfrag":     func(r rules.Rule) bool { return r.IPFrag != 0 },
	}

	for key, ok := range carries {
		t.Run(key, func(t *testing.T) {
			p, found := Named(key)
			if !found {
				t.Fatalf("preset %q is missing", key)
			}

			set, err := p.Rules()
			if err != nil {
				t.Fatalf("Rules: %v", err)
			}

			r, _ := set.For("discord.com")
			if !ok(r) {
				t.Errorf("preset %q does not carry its way: %+v", key, r)
			}
		})
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

func TestEveryPresetHasADescription(t *testing.T) {
	for _, p := range All() {
		if p.Desc("ru") == "" {
			t.Errorf("%s has no ru description", p.Key)
		}

		if p.Desc("en") == "" {
			t.Errorf("%s has no en description", p.Key)
		}
	}
}

func TestNameDiffersByLanguage(t *testing.T) {
	p := All()[0]
	if p.Name("ru") == p.Name("en") {
		t.Errorf("name is the same in both languages: %q", p.Name("ru"))
	}
}

func TestNamesByFavouriteWithNoneKeepsOrder(t *testing.T) {
	plain := Names("en")
	sorted := NamesByFavourite("en", nil)

	for i := range plain {
		if plain[i] != sorted[i] {
			t.Fatalf("order changed at %d: %q vs %q", i, plain[i], sorted[i])
		}
	}
}

func TestNamesByFavouriteFloatsStarredToTop(t *testing.T) {
	fav := []string{"signature"}
	sorted := NamesByFavourite("en", fav)
	want, _ := Named("signature")

	if sorted[0] != Label(want, "en", fav) {
		t.Errorf("a starred method is not first: %q", sorted[0])
	}

	if len(sorted) != len(All()) {
		t.Errorf("a method went missing: %d of %d", len(sorted), len(All()))
	}
}

func TestLabelMarksOnlyFavourites(t *testing.T) {
	p, _ := Named("signature")

	if Label(p, "en", nil) != p.Name("en") {
		t.Error("unstarred label carries a mark")
	}

	if Label(p, "en", []string{"signature"}) == p.Name("en") {
		t.Error("starred label is missing its mark")
	}
}

func TestByLabelFindsAMarkedName(t *testing.T) {
	p, _ := Named("signature")
	label := Label(p, "en", []string{"signature"})

	back, ok := ByLabel(label, "en")
	if !ok || back.Key != "signature" {
		t.Errorf("a marked label did not map back: %q -> %v %v", label, back.Key, ok)
	}
}
