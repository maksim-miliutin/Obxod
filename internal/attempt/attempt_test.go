package attempt

import (
	"testing"
	"time"
)

func TestFirstThenAgain(t *testing.T) {
	tracker := New(time.Minute)
	now := time.Now()

	if got := tracker.Saw("gateway.discord.gg", 54321, 1000, now); got != First {
		t.Errorf("first hello read as %v, want First", got)
	}

	if got := tracker.Saw("gateway.discord.gg", 54321, 1000, now.Add(time.Second)); got != Again {
		t.Errorf("retransmission read as %v, want Again", got)
	}

	if got := tracker.Saw("gateway.discord.gg", 54321, 1000, now.Add(3*time.Second)); got != Again {
		t.Errorf("third try read as %v, want Again", got)
	}
}

func TestDifferentConnectionsAreNotRepeats(t *testing.T) {
	tracker := New(time.Minute)
	now := time.Now()

	tracker.Saw("gateway.discord.gg", 54321, 1000, now)

	cases := []struct {
		name string
		port uint16
		seq  uint32
		host string
	}{
		{"another source port", 54322, 1000, "gateway.discord.gg"},
		{"another sequence", 54321, 2000, "gateway.discord.gg"},
		{"another host", 54321, 1000, "discord.com"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tracker.Saw(c.host, c.port, c.seq, now.Add(time.Second)); got != First {
				t.Errorf("read as %v, want First", got)
			}
		})
	}
}

func TestOldAttemptsAreForgotten(t *testing.T) {
	tracker := New(10 * time.Second)
	now := time.Now()

	tracker.Saw("gateway.discord.gg", 54321, 1000, now)

	// A connection reusing the same port and sequence a minute later is a new try,
	// not a retransmission: retransmissions come within seconds.
	if got := tracker.Saw("gateway.discord.gg", 54321, 1000, now.Add(time.Minute)); got != First {
		t.Errorf("read as %v, want First", got)
	}
}

func TestSweepKeepsTheMapSmall(t *testing.T) {
	tracker := New(10 * time.Second)
	now := time.Now()

	for i := 0; i < 500; i++ {
		tracker.Saw("example.com", uint16(40000+i), uint32(i), now)
	}

	if tracker.Watching() != 500 {
		t.Fatalf("watching %d, want 500", tracker.Watching())
	}

	tracker.Saw("example.com", 1, 1, now.Add(time.Minute))

	if tracker.Watching() != 1 {
		t.Errorf("watching %d after everything went stale, want 1", tracker.Watching())
	}
}

func TestHostOnPort(t *testing.T) {
	tracker := New(time.Minute)
	now := time.Now()

	tracker.Saw("gateway.discord.gg", 54321, 1000, now)
	tracker.Saw("discord.com", 54322, 2000, now)

	if host, ok := tracker.HostOn(54321); !ok || host != "gateway.discord.gg" {
		t.Errorf("HostOn(54321) = %q %v, want gateway.discord.gg", host, ok)
	}

	if host, ok := tracker.HostOn(54322); !ok || host != "discord.com" {
		t.Errorf("HostOn(54322) = %q %v, want discord.com", host, ok)
	}

	if _, ok := tracker.HostOn(9999); ok {
		t.Error("a port we never used was claimed as known")
	}
}

func TestHostOnForgetsWithTheRest(t *testing.T) {
	tracker := New(10 * time.Second)
	now := time.Now()

	tracker.Saw("gateway.discord.gg", 54321, 1000, now)
	tracker.Saw("discord.com", 54322, 2000, now.Add(time.Minute))

	if _, ok := tracker.HostOn(54321); ok {
		t.Error("a stale port is still claimed as known")
	}
}
