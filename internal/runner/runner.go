package runner

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"obxod/internal/bypass"
	"obxod/internal/divert"
	"obxod/internal/filter"
	"obxod/internal/rules"
	"obxod/internal/sweep"
)

type Config struct {
	Rules    rules.Set
	Hunt     *sweep.Sweep
	Ports    []uint16
	Voice    []filter.PortRange
	Pattern  []byte
	Recorded []byte
	Voiced   []byte
	Silence  time.Duration
	DropQUIC bool
	Wet      bool
	Seen     bool
	Report   func(string)
}

type Session struct {
	engine  *bypass.Engine
	wire    bypass.Wire
	eyes    bypass.Eyes
	handles []io.Closer
	once    sync.Once
	stopped atomic.Bool
}

func DefaultPorts() []uint16 {
	return []uint16{443, 2053, 2083, 2087, 2096, 8443}
}

func Open(cfg Config) (*Session, error) {
	outbound, err := filter.Outbound(filter.Ports{TCP: cfg.Ports, Voice: cfg.Voice, QUIC: cfg.DropQUIC})
	if err != nil {
		return nil, err
	}

	watching, err := filter.Replies(cfg.Ports)
	if err != nil {
		return nil, err
	}

	wire, err := divert.Open(outbound, divert.Modify)
	if err != nil {
		return nil, err
	}

	eyes, err := divert.Open(watching, divert.Sniff)
	if err != nil {
		wire.Close()

		return nil, fmt.Errorf("cannot watch replies: %w", err)
	}

	engine := bypass.New(bypass.Settings{
		Rules:    cfg.Rules,
		Hunt:     cfg.Hunt,
		Pattern:  cfg.Pattern,
		Recorded: cfg.Recorded,
		Voiced:   cfg.Voiced,
		Silence:  cfg.Silence,
		DropQUIC: cfg.DropQUIC,
		Wet:      cfg.Wet,
		Seen:     cfg.Seen,
		Report:   cfg.Report,
	})

	return &Session{
		engine:  engine,
		wire:    wire,
		eyes:    eyes,
		handles: []io.Closer{wire, eyes},
	}, nil
}

func (s *Session) Run() error {
	err := s.engine.Run(s.wire, s.eyes)

	// A read that fails after Stop is the stop being carried out, not a fault.
	if err != nil && !s.stopped.Load() {
		return err
	}

	return nil
}

func (s *Session) Stop() {
	s.stopped.Store(true)

	// Closed once: Stop arrives from a Ctrl+C handler and from a defer both.
	s.once.Do(func() {
		for _, h := range s.handles {
			h.Close()
		}
	})
}

func (s *Session) Downloaded() int {
	return s.engine.Downloaded()
}
