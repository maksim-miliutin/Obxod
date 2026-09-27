package lang

import "testing"

func TestEveryTextHasBothLanguages(t *testing.T) {
	for x := Text(0); x < last; x++ {
		if ru[x] == "" {
			t.Errorf("ru is missing text %d", x)
		}

		if en[x] == "" {
			t.Errorf("en is missing text %d", x)
		}
	}
}

func TestOfPicksEnglishOnlyForEn(t *testing.T) {
	if Of("en").T(TurnOn) != "Turn on" {
		t.Error("Of(en) did not give English")
	}

	if Of("ru").T(TurnOn) != "Включить" {
		t.Error("Of(ru) did not give Russian")
	}

	if Of("").T(TurnOn) != "Включить" {
		t.Error("Of with no code should fall back to Russian")
	}
}
