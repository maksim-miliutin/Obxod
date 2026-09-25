package preset

import "testing"

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
