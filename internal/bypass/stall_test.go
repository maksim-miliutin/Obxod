package bypass

import (
	"strings"
	"testing"
	"time"
)

func TestAResetBurstSuggestsAReload(t *testing.T) {
	out := &lines{}
	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Report: out.say})
	now := time.Now()

	// Opens the window with the count at zero, the way a live run does before a reset.
	_ = e.check(now)

	for port := uint16(50000); port < 50000+stallHint; port++ {
		e.health.Hello("gateway.discord.gg", port)
		e.reset(port)
	}

	_ = e.check(now.Add(time.Second))

	if !strings.Contains(strings.Join(out.all(), "\n"), "Ctrl+R") {
		t.Errorf("a burst of resets said nothing about reloading:\n%s", strings.Join(out.all(), "\n"))
	}
}

func TestAFewResetsStayQuiet(t *testing.T) {
	out := &lines{}
	e := New(Settings{Rules: setOf(t, "discord.com=hostfake:mail.ru"), Report: out.say})
	now := time.Now()

	_ = e.check(now)

	for port := uint16(50000); port < 50000+stallHint-1; port++ {
		e.health.Hello("gateway.discord.gg", port)
		e.reset(port)
	}

	_ = e.check(now.Add(time.Second))

	if strings.Contains(strings.Join(out.all(), "\n"), "Ctrl+R") {
		t.Errorf("a few resets should not suggest a reload:\n%s", strings.Join(out.all(), "\n"))
	}
}
