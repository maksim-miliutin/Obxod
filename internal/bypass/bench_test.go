package bypass

import (
	"fmt"
	"testing"
	"time"

	"obxod/internal/divert"
	"obxod/internal/rules"
)

type sink struct{}

func (sink) Send(packet []byte, addr *divert.Addr) error { return nil }

func benchRules(text string) rules.Set {
	set, err := rules.ParseAll([]string{text})
	if err != nil {
		panic(err)
	}

	return set
}

// Almost everything on the wire is not a hello, so this is the cost that is paid
// millions of times.
func BenchmarkForwardOther(b *testing.B) {
	e := New(Settings{Rules: benchRules("discord.com=hostfake:mail.ru,ts"), Wet: true})
	packet := packet443([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	addr := &divert.Addr{}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		e.forward(sink{}, packet, addr)
	}
}

// A hello is paid once per connection, so it may cost more.
func BenchmarkForwardHello(b *testing.B) {
	e := New(Settings{Rules: benchRules("discord.com=hostfake:mail.ru,ts"), Wet: true})
	packet := packet443(clientHello("updates.discord.com"))
	addr := &divert.Addr{}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		e.forward(sink{}, packet, addr)
	}
}

// The bookkeeping walks every live link, and the loop asks for it once a packet.
// Run this against the two above before believing the loop is cheap.
func BenchmarkStep(b *testing.B) {
	for _, links := range []int{0, 5, 40, 200} {
		b.Run(fmt.Sprint(links), func(b *testing.B) {
			e := New(Settings{Rules: benchRules("discord.com=hostfake:mail.ru,ts"), Wet: true})

			for i := range links {
				e.health.Hello("discord.com", uint16(50000+i))
			}

			when := time.Now()

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				e.step(when)
			}
		})
	}
}
