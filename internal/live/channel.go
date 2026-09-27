// Package live is the real-time side of the program: the screen a person
// watches while turning a tie rod.
//
// Sensors — a phone clamped to the rim, a DIY sensor head, a camera, or a
// person typing in a fresh string-line reading — push per-wheel readings into a
// Hub. The Hub smooths them, decides when they have settled, derives everything
// that depends on more than one wheel (thrust line, totals, cross values),
// grades it against the vehicle's tolerances and publishes a frame several
// times a second.
//
// Nothing here is a second implementation of the alignment maths. Per-wheel
// angles come in already measured; axle and vehicle values are formed exactly
// as package align forms them, and tolerances come from specs.Lookup, the same
// function the printed report uses.
package live

import (
	"math"
	"time"
)

const (
	// filterTau is the smoothing time constant. Long enough to steady a
	// phone's accelerometer, short enough that the needle follows a hand on a
	// spanner without a lag anyone notices.
	filterTau = 400 * time.Millisecond

	// stabilityWindow is how far back "settled" looks.
	stabilityWindow = 1500 * time.Millisecond

	// staleAfter is how long a streaming sensor may fall silent before its
	// reading is shown as out of date rather than as current.
	staleAfter = 3 * time.Second

	minStableSamples = 5
)

// Thresholds for "settled", in degrees. Below the resolution of any gauge a
// person could use at home, above the jitter of a phone accelerometer after
// the phone's own averaging.
const (
	stableCamber = 0.03
	stableToe    = 0.03
)

type reading struct {
	t time.Time
	v float64
}

// channel is one live quantity on one wheel: camber or toe.
type channel struct {
	has    bool
	value  float64 // smoothed, degrees
	lastT  time.Time
	source string

	// instant marks a typed-in reading. It is exact as far as the program can
	// know, so it is neither smoothed nor ever considered unsettled or stale.
	instant bool

	window []reading
}

func (c *channel) push(v float64, t time.Time, instant bool, source string) {
	restart := !c.has || instant || c.instant || source != c.source || t.Sub(c.lastT) > staleAfter
	if restart {
		c.value = v
		c.window = c.window[:0]
	} else {
		dt := t.Sub(c.lastT).Seconds()
		if dt < 0 {
			dt = 0
		}
		alpha := 1 - math.Exp(-dt/filterTau.Seconds())
		c.value += alpha * (v - c.value)
	}
	c.has, c.lastT, c.instant, c.source = true, t, instant, source

	c.window = append(c.window, reading{t, v})
	cut := 0
	for cut < len(c.window) && t.Sub(c.window[cut].t) > stabilityWindow {
		cut++
	}
	c.window = c.window[cut:]
}

func (c *channel) stale(now time.Time) bool {
	return c.has && !c.instant && now.Sub(c.lastT) > staleAfter
}

// stable reports whether the reading has settled: little scatter and no trend
// across the window. Both matter — a slowly turning adjuster has low scatter
// but a clear trend, and must not be reported as settled.
func (c *channel) stable(now time.Time, thr float64) bool {
	if !c.has {
		return false
	}
	if c.instant {
		return true
	}
	if c.stale(now) || len(c.window) < minStableSamples {
		return false
	}
	n := len(c.window)
	mean := 0.0
	for _, r := range c.window {
		mean += r.v
	}
	mean /= float64(n)
	ss := 0.0
	for _, r := range c.window {
		ss += (r.v - mean) * (r.v - mean)
	}
	if math.Sqrt(ss/float64(n)) > thr {
		return false
	}
	half := n / 2
	a, b := 0.0, 0.0
	for i, r := range c.window {
		if i < half {
			a += r.v
		} else {
			b += r.v
		}
	}
	return math.Abs(a/float64(half)-b/float64(n-half)) <= thr
}

func (c *channel) clear() { *c = channel{window: c.window[:0]} }
