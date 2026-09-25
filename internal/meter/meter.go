package meter

import (
	"fmt"
	"time"
)

type Rate struct {
	last    int
	at      time.Time
	started bool
}

func (r *Rate) Sample(total int, now time.Time) float64 {
	if !r.started {
		r.last = total
		r.at = now
		r.started = true

		return 0
	}

	seconds := now.Sub(r.at).Seconds()
	grew := total - r.last

	r.last = total
	r.at = now

	if seconds <= 0 {
		return 0
	}

	return float64(grew) / seconds
}

func Human(perSecond float64) string {
	switch {
	case perSecond >= 1<<20:
		return fmt.Sprintf("%.1f МБ/с", perSecond/(1<<20))
	case perSecond >= 1<<10:
		return fmt.Sprintf("%.0f КБ/с", perSecond/(1<<10))
	default:
		return fmt.Sprintf("%.0f Б/с", perSecond)
	}
}
