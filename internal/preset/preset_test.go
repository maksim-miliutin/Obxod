package preset

import (
	"strings"
	"testing"
)

func TestEveryPresetParses(t *testing.T) {
	for _, p := range All() {
		set, err := p.Rules()
		if err != nil {
			t.Errorf("%s: %v", p.Name, err)

			continue
		}

		if len(set) == 0 {
			t.Errorf("%s: no rules", p.Name)
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
	if _, ok := Named("Обычный"); !ok {
		t.Error("Named missed a real preset")
	}

	if _, ok := Named("nope"); ok {
		t.Error("Named returned a preset that does not exist")
	}
}

func TestNamesCoverEveryPreset(t *testing.T) {
	if len(Names()) != len(All()) {
		t.Fatalf("Names %d, All %d", len(Names()), len(All()))
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
