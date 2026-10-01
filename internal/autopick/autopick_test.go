package autopick

import (
	"errors"
	"testing"
)

func TestTryReturnsTheFirstThatWorks(t *testing.T) {
	tried := []string{}
	works := map[string]bool{"a": false, "b": true, "c": true}

	name, ok := Try(
		[]string{"a", "b", "c"},
		func(n string) error { tried = append(tried, n); return nil },
		func() bool { return works[tried[len(tried)-1]] },
	)

	if !ok || name != "b" {
		t.Fatalf("got %q %v, want b true", name, ok)
	}

	if len(tried) != 2 {
		t.Errorf("tried %v, should have stopped at b", tried)
	}
}

func TestTryReportsNoneWhenAllFail(t *testing.T) {
	name, ok := Try(
		[]string{"a", "b"},
		func(string) error { return nil },
		func() bool { return false },
	)

	if ok || name != "" {
		t.Errorf("got %q %v, want empty false", name, ok)
	}
}

func TestTrySkipsAMethodThatWillNotApply(t *testing.T) {
	name, ok := Try(
		[]string{"bad", "good"},
		func(n string) error {
			if n == "bad" {
				return errors.New("nope")
			}

			return nil
		},
		func() bool { return true },
	)

	if !ok || name != "good" {
		t.Errorf("got %q %v, want good true", name, ok)
	}
}
