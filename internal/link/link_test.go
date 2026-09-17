package link

import (
	"testing"
	"time"
)

func TestDataSaysOnlyTheFirstTime(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)

	host, first := h.Data(54321, 100, now.Add(time.Millisecond))
	if !first || host != "gateway.discord.gg" {
		t.Fatalf("first answer came back %q %v", host, first)
	}

	if _, first := h.Data(54321, 100, now.Add(2*time.Millisecond)); first {
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

	if _, first := h.Data(54321, 100, now); !first {
		t.Error("the first connection was not reported")
	}

	if _, first := h.Data(54322, 100, now); !first {
		t.Error("the second connection to the same site was passed over")
	}
}

func TestDataOnAnUnknownPort(t *testing.T) {
	h := New()

	if _, first := h.Data(9999, 100, time.Now()); first {
		t.Error("a port we never saw a hello on was reported as answering")
	}
}

func TestWentQuiet(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Data(54321, 100, now)
	h.Data(54321, 100, now.Add(time.Second))

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
	h.Data(54321, 100, now)

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

func TestResetNamesTheHostAndDropsTheLink(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)

	host, known := h.Reset(54321)
	if !known || host != "gateway.discord.gg" {
		t.Fatalf("Reset(54321) = %q %v, want the host back", host, known)
	}

	if _, first := h.Data(54321, 100, now); first {
		t.Error("a link dropped on reset still answers")
	}

	if _, known := h.Reset(54321); known {
		t.Error("the same link was dropped twice")
	}
}

func TestResetOnAPortWeNeverSaw(t *testing.T) {
	if _, known := New().Reset(9999); known {
		t.Error("a port we never touched was claimed as ours")
	}
}

// The false alarm this guards: a link the other side reset used to stay in place
// and get reported a second time as one that quietly died.
func TestResetLinkIsNotReportedQuietLater(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Data(54321, 100, now)
	h.Reset(54321)

	if got := h.WentQuiet(now.Add(time.Minute), 5*time.Second); len(got) != 0 {
		t.Errorf("a reset link was also called killed: %+v", got)
	}
}

// The bug this guards: the host on a port used to be kept a second time in the
// retry tracker, which forgets after 20s, so long connections lost their name.
func TestPortStaysKnownLongAfterTheHello(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("gateway.discord.gg", 54321, now)
	h.Data(54321, 100, now.Add(time.Hour))

	if host, known := h.Reset(54321); !known || host != "gateway.discord.gg" {
		t.Errorf("an hour later the port gave %q %v, want the host", host, known)
	}
}

// The false alarm this guards: a finished request goes quiet exactly like a
// killed one, and calling both killed made the report worthless.
func TestPolitelyClosedIsNotCalledKilled(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("updates.discord.com", 54321, now)
	h.Data(54321, 100, now)
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
	h.Data(54321, 100, now)

	if got := h.WentQuiet(now.Add(time.Minute), 5*time.Second); len(got) != 1 {
		t.Errorf("a connection that went quiet with no fin was not reported: %+v", got)
	}
}

// What the bytes are for: packets before silence say nothing about how far a
// stream got, and a stream cut at a fixed size is what a throttled link looks like.
func TestBytesAddUpAcrossPackets(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("discord.com", 54321, now)
	h.Data(54321, 1460, now)
	h.Data(54321, 1460, now)
	h.Data(54321, 700, now)

	got := h.WentQuiet(now.Add(time.Minute), time.Second)
	if len(got) != 1 {
		t.Fatalf("reported %d links, want 1", len(got))
	}

	if got[0].Bytes != 3620 {
		t.Errorf("Bytes = %d, want 3620", got[0].Bytes)
	}

	if got[0].Packets != 3 {
		t.Errorf("Packets = %d, want 3", got[0].Packets)
	}
}

func TestBytesStayApartPerPort(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("discord.com", 1111, now)
	h.Hello("discord.com", 2222, now)
	h.Data(1111, 500, now)
	h.Data(2222, 9000, now)

	for _, r := range h.WentQuiet(now.Add(time.Minute), time.Second) {
		want := map[uint16]int{1111: 500, 2222: 9000}[r.Port]
		if r.Bytes != want {
			t.Errorf("port %d carried %d bytes, want %d", r.Port, r.Bytes, want)
		}
	}
}

// The leak this guards: a link was marked as told about and kept forever, while
// the loop walks this map on every packet that goes by.
func TestALinkIsForgottenOnceToldAbout(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("discord.com", 54321, now)
	h.Data(54321, 1460, now)

	if got := h.WentQuiet(now.Add(time.Minute), time.Second); len(got) != 1 {
		t.Fatalf("reported %d links, want 1", len(got))
	}

	if _, known := h.Reset(54321); known {
		t.Error("the link is still held after it was reported")
	}
}

func TestAPolitelyClosedLinkIsForgotten(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("discord.com", 54321, now)
	h.Data(54321, 1460, now)
	h.Closed(54321)

	if _, known := h.Reset(54321); known {
		t.Error("the link is still held after a fin")
	}

	if got := h.WentQuiet(now.Add(time.Minute), time.Second); len(got) != 0 {
		t.Errorf("a closed link was called quiet: %+v", got)
	}
}

// Live links stay: only terminal ones go, or the run would stop watching what it
// is watching.
func TestALiveLinkIsKept(t *testing.T) {
	h := New()
	now := time.Now()

	h.Hello("discord.com", 54321, now)
	h.Data(54321, 1460, now)

	if got := h.WentQuiet(now.Add(time.Second), time.Minute); len(got) != 0 {
		t.Fatalf("a link quiet for a second was reported: %+v", got)
	}

	if _, known := h.Reset(54321); !known {
		t.Error("a live link was dropped")
	}
}

// Nothing accumulates over a long run: every link ends one of three ways.
func TestNothingIsHeldAfterEveryLinkEnds(t *testing.T) {
	h := New()
	now := time.Now()

	for port := uint16(1000); port < 1100; port++ {
		h.Hello("discord.com", port, now)
		h.Data(port, 500, now)
	}

	h.Closed(1000)
	h.Reset(1001)
	h.WentQuiet(now.Add(time.Minute), time.Second)

	for port := uint16(1000); port < 1100; port++ {
		if _, known := h.Reset(port); known {
			t.Fatalf("port %d is still held", port)
		}
	}
}
