package link

import (
	"testing"
	"time"
)

func TestDataSaysOnlyTheFirstTime(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)

	host, first := h.Data(54321, now.Add(time.Millisecond))
	if !first || host != "gateway.discord.gg" {
		t.Fatalf("first answer came back %q %v", host, first)
	}

	if _, first := h.Data(54321, now.Add(2*time.Millisecond)); first {
		t.Error("the second packet was called the first")
	}
}

// The flaw this guards: reporting once per site hid whether the next connection
// to the same site ever answered.
func TestEveryConnectionIsFollowedApart(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Hello("gateway.discord.gg", 54322, now)

	if _, first := h.Data(54321, now); !first {
		t.Error("the first connection was not reported")
	}

	if _, first := h.Data(54322, now); !first {
		t.Error("the second connection to the same site was passed over")
	}
}

func TestDataOnAnUnknownPort(t *testing.T) {
	h := New()

	if _, first := h.Data(9999, time.Now()); first {
		t.Error("a port we never saw a hello on was reported as answering")
	}
}

func TestWentQuiet(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Data(54321, now)
	h.Data(54321, now.Add(time.Second))

	if got := h.WentQuiet(now.Add(3*time.Second), 5*time.Second); len(got) != 0 {
		t.Errorf("called quiet too early: %+v", got)
	}

	got := h.WentQuiet(now.Add(10*time.Second), 5*time.Second)
	if len(got) != 1 {
		t.Fatalf("reports = %d, want 1", len(got))
	}

	if got[0].Host != "gateway.discord.gg" || got[0].Port != 54321 || got[0].Packets != 2 {
		t.Errorf("report = %+v, want the gateway connection with 2 packets", got[0])
	}
}

func TestQuietIsReportedOnce(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Data(54321, now)

	if got := h.WentQuiet(now.Add(10*time.Second), 5*time.Second); len(got) != 1 {
		t.Fatalf("reports = %d, want 1", len(got))
	}

	if got := h.WentQuiet(now.Add(20*time.Second), 5*time.Second); len(got) != 0 {
		t.Errorf("the same silence was reported twice: %+v", got)
	}
}

func TestConnectionsThatNeverAnsweredAreNotCalledQuiet(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)

	// Never answering is a different fault from answering and stopping, and the
	// hello watcher already covers it.
	if got := h.WentQuiet(now.Add(time.Minute), 5*time.Second); len(got) != 0 {
		t.Errorf("a connection that never answered was called quiet: %+v", got)
	}
}

func TestForget(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Forget(54321)

	if _, first := h.Data(54321, now); first {
		t.Error("a forgotten connection still answers")
	}
}

// The false alarm this guards: a finished request goes quiet exactly like a
// killed one, and calling both killed made the report worthless.
func TestPolitelyClosedIsNotCalledKilled(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("updates.discord.com", 54321, now)
	h.Data(54321, now)
	h.Closed(54321)

	if got := h.WentQuiet(now.Add(time.Minute), 5*time.Second); len(got) != 0 {
		t.Errorf("a connection closed on purpose was reported as killed: %+v", got)
	}
}

func TestClosedOnAnUnknownPortIsHarmless(t *testing.T) {
	New().Closed(9999)
}

func TestKilledIsStillReportedAfterTheFix(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Data(54321, now)

	if got := h.WentQuiet(now.Add(time.Minute), 5*time.Second); len(got) != 1 {
		t.Errorf("a connection that went quiet with no fin was not reported: %+v", got)
	}
}
