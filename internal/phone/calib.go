// Package phone turns a smartphone into a wireless camber and caster sensor.
//
// The phone opens a page served by this program over the local network, reads
// its accelerometer and gyroscope through the browser, and sends averaged
// batches back. Everything that matters — calibration, the camber maths, the
// guided caster sweep — happens here, in Go, where it is tested; the page on the
// phone only collects numbers and shows what it is told.
//
// # How a phone measures camber
//
// The phone is held against a straight bar laid across the rim flanges, screen
// facing outboard, upright. The rim plane is then the phone's back plane, so
// the phone's z axis (out of the screen) is the wheel's outboard spin axis, and
// the accelerometer, at rest, reports the reaction to gravity — a vector
// pointing straight up. Camber tips the spin axis below horizontal by exactly
// the camber angle, so the up-vector's z component is −sin γ:
//
//	γ = atan2(−u_z, √(u_x² + u_y²))
//
// The in-plane part enters only through its length, so it does not matter how
// the phone is rotated within the rim plane.
//
// # Why calibration is not optional
//
// A phone accelerometer's zero offset is typically 10–30 mg, and on the z axis
// that is 0.6–1.7° of camber — more than the whole factory tolerance. Two short
// procedures remove it:
//
//   - Flat: the phone lies screen up, then screen down. The true z reading
//     changes sign and the offset does not, so their mean is the offset. Tilt
//     of the table does not matter — it enters only as cos(tilt), second order.
//     The same step detects platforms that report gravity rather than its
//     reaction (Safari on iOS), by the sign of the screen-up reading.
//
//   - Mount: on the bar, upright, then upside down at the same spot. The camera
//     bump and the case tip the phone against the bar by some angle δ; turning
//     the phone over reverses it while camber stays put, so the half-difference
//     is δ. It also absorbs the accelerometer's cross-axis sensitivity from the
//     y axis into z, which reverses with the phone for the same reason and
//     would otherwise be worth another half degree.
package phone

import (
	"math"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

// G is standard gravity, m/s².
const G = 9.80665

// Sample is one batch from the phone: sensor readings averaged over DT seconds.
type Sample struct {
	DT float64 `json:"dt"`
	N  int     `json:"n"`
	// Mean accelerationIncludingGravity, m/s², in the device frame
	// (x right, y up the screen, z out of the screen).
	AX float64 `json:"ax"`
	AY float64 `json:"ay"`
	AZ float64 `json:"az"`
	// Mean rotation rate, deg/s, about device x, y and z (W3C beta, gamma,
	// alpha). HasGyro is false on phones without a gyroscope.
	GX      float64 `json:"gx"`
	GY      float64 `json:"gy"`
	GZ      float64 `json:"gz"`
	HasGyro bool    `json:"gyro"`
}

// Calibration is what the two procedures establish for one phone.
type Calibration struct {
	// Sign is +1 when the phone reports the reaction to gravity (Android:
	// +9.8 on z lying screen up) and −1 when it reports gravity itself.
	Sign  float64 `json:"sign"`
	BiasZ float64 `json:"bias_z"` // m/s², in the Sign-corrected frame
	Flat  bool    `json:"flat"`

	Mount   float64 `json:"mount"` // degrees, for the phone upright
	Mounted bool    `json:"mounted"`

	At time.Time `json:"at"`
}

// Ready reports whether the phone can be trusted for camber.
func (c Calibration) Ready() bool { return c.Flat && c.Mounted }

// up returns the Sign- and bias-corrected reaction vector.
func (c Calibration) up(s Sample) (x, y, z float64) {
	sign := c.Sign
	if sign == 0 {
		sign = 1
	}
	return sign * s.AX, sign * s.AY, sign*s.AZ - c.BiasZ
}

// tilt is the measured angle of the phone's z axis below horizontal, in
// degrees, before the mount correction; roll is the phone's rotation within the
// rim plane (0 upright, ±180 upside down).
func (c Calibration) tilt(s Sample) (tilt, roll float64) {
	x, y, z := c.up(s)
	return deg(math.Atan2(-z, math.Hypot(x, y))), deg(math.Atan2(x, y))
}

// Camber converts a sample into camber, degrees. ok is false when the phone is
// held too far from upright or upside down for the mount correction to apply.
func (c Calibration) Camber(s Sample) (camber float64, ok bool) {
	t, roll := c.tilt(s)
	cr := math.Cos(rad(roll))
	// The mount offset was measured upright and upside down; in between it is
	// not known, so readings more than about 30° off either are refused.
	if math.Abs(cr) < 0.85 {
		return t, false
	}
	return t - c.Mount*cr, true
}

// Errors from the calibration procedures, phrased for the phone's screen.
var (
	ErrNotFlat    = i18n.Err("телефон лежит не ровно: положите его на ровный стол")
	ErrNotFlipped = i18n.Err("похоже, телефон не перевернули: второе положение должно быть экраном вниз")
	ErrBadGravity = i18n.Err("акселерометр показывает неправдоподобную силу тяжести — телефон двигался или датчик неисправен")
	ErrNotUpright = i18n.Err("держите телефон вертикально, как в первый раз")
	ErrBadMount   = i18n.Err("разница больше 5° — телефон прижат к планке неровно; прижмите плотнее и повторите")
)

// SolveFlat computes Sign and BiasZ from the screen-up and screen-down means.
func SolveFlat(up, down Sample) (sign, bias float64, err error) {
	for _, s := range []Sample{up, down} {
		if math.Hypot(s.AX, s.AY) > 1.5 { // more than ~9° off flat
			return 0, 0, ErrNotFlat
		}
	}
	sign = 1
	if up.AZ < 0 {
		sign = -1
	}
	zu, zd := sign*up.AZ, sign*down.AZ
	if zd > 0 {
		return 0, 0, ErrNotFlipped
	}
	if g := (zu - zd) / 2; g < G*0.93 || g > G*1.07 {
		return 0, 0, ErrBadGravity
	}
	return sign, (zu + zd) / 2, nil
}

// SolveMount computes the mount offset from the upright and upside-down means
// taken at the same spot on the bar. It also returns the camber at that spot,
// which both readings agree on once the offset is removed.
func (c Calibration) SolveMount(upright, flipped Sample) (mount, camber float64, err error) {
	t1, r1 := c.tilt(upright)
	t2, r2 := c.tilt(flipped)
	if math.Cos(rad(r1)) < 0.85 || math.Cos(rad(r2)) > -0.85 {
		return 0, 0, ErrNotUpright
	}
	mount = (t1 - t2) / 2
	if math.Abs(mount) > 5 {
		return 0, 0, ErrBadMount
	}
	return mount, (t1 + t2) / 2, nil
}

// VerticalRate is the rotation rate about the vertical, deg/s: the steering
// rate when the phone rides on a steered wheel. The rate vector is projected
// onto the measured up direction, so the phone need not be exactly upright.
func (c Calibration) VerticalRate(s Sample) float64 {
	x, y, z := c.up(s)
	n := math.Sqrt(x*x + y*y + z*z)
	if n < 1 {
		return 0
	}
	return (s.GX*x + s.GY*y + s.GZ*z) / n
}

func deg(r float64) float64 { return r * 180 / math.Pi }
func rad(d float64) float64 { return d * math.Pi / 180 }
