package meter

import (
	"testing"
	"time"
)

func TestSampleIsRateNotTotal(t *testing.T) {
	var r Rate
	base := time.Now()

	if got := r.Sample(0, base); got != 0 {
		t.Fatalf("first sample = %v, want 0", got)
	}

	got := r.Sample(2<<20, base.Add(2*time.Second))
	want := float64(2<<20) / 2

	if got != want {
		t.Fatalf("Sample = %v, want %v (rate, not total)", got, want)
	}
}

func TestHumanScalesUnits(t *testing.T) {
	cases := map[float64]string{
		0:             "0 Б/с",
		2048:          "2 КБ/с",
		3 * (1 << 20): "3.0 МБ/с",
	}

	for in, want := range cases {
		if got := Human(in); got != want {
			t.Errorf("Human(%v) = %q, want %q", in, got, want)
		}
	}
}
