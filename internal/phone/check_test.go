package phone

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
)

// runCheck walks the phone through the accuracy check: on the table screen up
// and down, then on the bar upright and upside down — the same positions as the
// calibration, with the turn-overs in between.
func (r *rig) runCheck(p *simPhone, a geom.Vec3) PhoneState {
	r.t.Helper()
	r.cmd(phoneRequest{Cmd: "check"})
	if r.last.Step != StepCheckFlatUp {
		r.t.Fatalf("the check did not start: %+v", r.last)
	}
	r.hold(p, flatUp(), 3500*time.Millisecond)
	if r.last.Step != StepCheckFlatDown {
		r.t.Fatalf("check, screen up: still at %s (%s)", r.last.Step, r.last.Error)
	}
	r.turnOver(p)
	r.hold(p, flatDown(), 3500*time.Millisecond)
	if r.last.Step != StepCheckMountUp {
		r.t.Fatalf("check, screen down: still at %s (%s)", r.last.Step, r.last.Error)
	}
	r.hold(p, p.onRim(a, true), 3500*time.Millisecond)
	if r.last.Step != StepCheckMountDown {
		r.t.Fatalf("check, on the bar: still at %s (%s)", r.last.Step, r.last.Error)
	}
	r.shake(p, p.onRim(a, false))
	return r.hold(p, p.onRim(a, false), 3500*time.Millisecond)
}

// TestAccuracyCheckPasses: a phone checked right after its calibration — on
// the table and on the bar, turned over each time — shows both errors well
// inside a tenth of a degree, and the check leaves the calibration alone.
func TestAccuracyCheckPasses(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		r := newRig(t)
		p := typicalPhone(31, sign)
		a := spinAxis(align.FR, -0.6, 0.05)
		r.cmd(phoneRequest{Wheel: "FR"})
		r.calibrate(p, a)
		cal := r.l.devices[r.id].Cal

		st := r.runCheck(p, a)
		if st.Step != StepLive || st.Check == nil || !st.Check.HasZero || !st.Check.HasMount {
			t.Fatalf("sign %+v: check did not finish: %+v", sign, st)
		}
		if !st.CheckOK || math.Abs(st.Check.Zero) > 0.05 || math.Abs(st.Check.Mount) > 0.05 {
			t.Errorf("sign %+v: a freshly calibrated phone failed its check: zero %.3f°, mount %.3f°", sign, st.Check.Zero, st.Check.Mount)
		}
		got := r.l.devices[r.id].Cal
		if got.BiasZ != cal.BiasZ || got.Mount != cal.Mount {
			t.Error("the check changed the calibration — it may only measure it")
		}
		if inf := r.l.Info(); len(inf.Devices) != 1 || inf.Devices[0].Check != "ok" {
			t.Errorf("the desktop list does not show the passed check: %+v", inf.Devices)
		}
		t.Logf("sign %+v: zero %.4f°, mount %.4f°", sign, st.Check.Zero, st.Check.Mount)
	}
}

// TestAccuracyCheckCatchesChanges: what really happens to a calibrated phone —
// a thicker case tips it on the bar by another 0.6°, the accelerometer's zero
// creeps in the cold by 0.05 m/s² — must fail the check, each caught by the
// half of the check that sees it: the bar for the case, the table for the zero.
func TestAccuracyCheckCatchesChanges(t *testing.T) {
	for _, c := range []struct {
		name           string
		change         func(p *simPhone)
		zeroBad, mntBd bool
	}{
		{"new case", func(p *simPhone) { p.mount += 0.6 }, false, true},
		{"zero drift", func(p *simPhone) { p.bias.Z += 0.05 }, true, false},
	} {
		r := newRig(t)
		p := typicalPhone(41, 1)
		a := spinAxis(align.FL, 0.2, 0)
		r.cmd(phoneRequest{Wheel: "FL"})
		r.calibrate(p, a)
		c.change(p)
		st := r.runCheck(p, a)
		if st.Check == nil || st.CheckOK {
			t.Errorf("%s: the check passed a phone whose calibration no longer holds: %+v", c.name, st.Check)
			continue
		}
		if bad := math.Abs(st.Check.Zero) > checkLimit; bad != c.zeroBad {
			t.Errorf("%s: zero %.3f° judged bad=%v", c.name, st.Check.Zero, bad)
		}
		if bad := math.Abs(st.Check.Mount) > checkLimit; bad != c.mntBd {
			t.Errorf("%s: mount %.3f° judged bad=%v", c.name, st.Check.Mount, bad)
		}
		t.Logf("%s: zero %.3f°, mount %.3f°", c.name, st.Check.Zero, st.Check.Mount)
	}
	// 0.05 m/s² on the z axis is asin(0.05/9.81) = 0.29° of camber, and a case
	// 0.6° thicker on one side puts 0.6° into every upright reading: the
	// check must report about those sizes, not just "bad".
}

// TestAccuracyCheckNeedsCalibration: there is nothing to check before the
// phone is calibrated.
func TestAccuracyCheckNeedsCalibration(t *testing.T) {
	r := newRig(t)
	if _, err := r.l.handle(r.id, phoneRequest{Cmd: "check"}); err == nil {
		t.Error("the check started on an uncalibrated phone")
	}
}

// TestGyroUnitsFound: Chrome on Android before version 66 reported rotation in
// radians a second, against the specification. Turning the phone over during
// the flat calibration — at least half a turn — tells which, and the caster
// sweep must come out right either way.
func TestGyroUnitsFound(t *testing.T) {
	for _, c := range []struct {
		name  string
		unit  float64
		scale float64
	}{
		{"degrees a second", 1, 1},
		{"radians a second (old Chrome)", math.Pi / 180, 180 / math.Pi},
	} {
		r := newRig(t)
		p := typicalPhone(51, 1)
		p.gyroUnit = c.unit
		pos := align.FL
		w := simulate.WheelSpec{Camber: align.Deg(-0.4), Toe: align.Deg(0.05), Caster: align.Deg(4.2), SAI: align.Deg(11)}
		r.cmd(phoneRequest{Wheel: pos.String()})
		r.calibrate(p, w.SpinAxis(pos))
		if got := r.l.devices[r.id].Cal.GyroScale; math.Abs(got-c.scale) > 1e-9 {
			t.Fatalf("%s: gyroscope scale %.3f, want %.3f", c.name, got, c.scale)
		}
		caster := r.sweep(p, pos, w, 19.5, -20.5)
		if math.Abs(caster-4.2) > 0.1 {
			t.Errorf("%s: caster %.3f°, true 4.20°", c.name, caster)
		}
	}
}

// TestGyroScaleUnclear: a turn-over the gyroscope saw as something in between
// — neither a half turn in degrees nor in radians — must not change the scale.
func TestGyroScaleUnclear(t *testing.T) {
	for turn, want := range map[float64]float64{0: 0, 1: 0, 40: 0, 3.4: 180 / math.Pi, 9: 180 / math.Pi, 185: 1, 420: 1} {
		if got := gyroScaleFrom(turn); got != want {
			t.Errorf("turn %.1f: scale %.3f, want %.3f", turn, got, want)
		}
	}
}

// sweep runs the guided caster sweep on a calibrated phone: straight, then the
// first turn (outboard steer in degrees, positive = out), then the second, then
// back. It returns the caster the hub shows.
func (r *rig) sweep(p *simPhone, pos align.Position, w simulate.WheelSpec, first, second float64) float64 {
	r.t.Helper()
	a0 := w.SpinAxis(pos)
	f0 := p.onRim(a0, true)
	r.cmd(phoneRequest{Cmd: "caster"})
	r.hold(p, f0, 2500*time.Millisecond)
	if r.last.Step != StepCasterOut {
		r.t.Fatalf("%s: straight-ahead reading not taken: %+v", pos, r.last)
	}
	k := w.SteeringAxis(pos)
	s := pos.SideSign()
	steer := func(from, to, secs float64) frame {
		steps := int(secs / 0.1)
		var f frame
		for i := 1; i <= steps; i++ {
			th := from + (to-from)*float64(i)/float64(steps)
			f = rotate(f0, geom.Rodrigues(k, s*th*math.Pi/180))
			r.now = r.now.Add(100 * time.Millisecond)
			r.last, _ = r.l.handle(r.id, phoneRequest{Samples: []Sample{p.read(f, k.Scale(s*(to-from)/secs), 0.1)}})
		}
		return f
	}
	r.hold(p, steer(0, first, 2), 2500*time.Millisecond)
	if r.last.Step != StepCasterIn {
		r.t.Fatalf("%s: first turn not taken at %.1f°: %+v", pos, first, r.last)
	}
	r.hold(p, steer(first, second, 4), 2500*time.Millisecond)
	if r.last.Step != StepCasterBack {
		r.t.Fatalf("%s: second turn not taken at %.1f°: %+v", pos, second, r.last)
	}
	steer(second, 0, 2)
	r.hold(p, f0, 2500*time.Millisecond)
	return r.hub.Frame().Params["caster_"+pos.String()].Value
}

// TestCasterWrongWayWarns: steering the wheel in first and out second gives
// the same caster with the opposite sign. The phone cannot tell left from
// right on every browser, but a caster of −3.5° on a road car is that
// mistake almost every time — the phone says so and how to redo it.
func TestCasterWrongWayWarns(t *testing.T) {
	for _, pos := range []align.Position{align.FL, align.FR} {
		r := newRig(t)
		p := typicalPhone(61, 1)
		w := simulate.WheelSpec{Camber: align.Deg(-0.5), Toe: align.Deg(0.08), Caster: align.Deg(3.5), SAI: align.Deg(12)}
		r.cmd(phoneRequest{Wheel: pos.String()})
		r.calibrate(p, w.SpinAxis(pos))
		caster := r.sweep(p, pos, w, -19.6, 20.4) // in first: the wrong way round
		if caster > -3 {
			t.Fatalf("%s: wrong-way sweep gave %.2f°, expected about −3.5° (the sign swaps)", pos, caster)
		}
		if r.last.Warn == "" {
			t.Errorf("%s: no warning for a negative caster", pos)
		}
		want := map[align.Position]string{align.FL: "ВЛЕВО, потом ВПРАВО", align.FR: "ВПРАВО, потом ВЛЕВО"}[pos]
		if !strings.Contains(r.last.Warn, want) {
			t.Errorf("%s: the warning does not say which way to steer first: %q", pos, r.last.Warn)
		}
		if ws := r.hub.Frame().Warnings; len(ws) == 0 {
			t.Errorf("%s: the warning did not reach the computer", pos)
		}

		// Done the right way round, the warning goes.
		caster = r.sweep(p, pos, w, 19.6, -20.4)
		if math.Abs(caster-3.5) > 0.1 || r.last.Warn != "" {
			t.Errorf("%s: right-way sweep: caster %.2f°, warning %q", pos, caster, r.last.Warn)
		}
	}
}

// TestSteerDirectionShown: during the sweep the phone shows which way to turn
// the steering wheel — out first: left for the left wheel, right for the right
// one — and the window the wheel has to reach.
func TestSteerDirectionShown(t *testing.T) {
	for pos, dirs := range map[align.Position][2]string{align.FL: {"left", "right"}, align.FR: {"right", "left"}} {
		r := newRig(t)
		p := typicalPhone(71, 1)
		w := simulate.WheelSpec{Camber: align.Deg(-0.3), Caster: align.Deg(3), SAI: align.Deg(12)}
		r.cmd(phoneRequest{Wheel: pos.String()})
		r.calibrate(p, w.SpinAxis(pos))
		r.cmd(phoneRequest{Cmd: "caster"})
		st := r.hold(p, p.onRim(w.SpinAxis(pos), true), 2500*time.Millisecond)
		if st.Step != StepCasterOut || st.SteerDir != dirs[0] || st.SteerLo != minTurn || st.SteerHi != maxTurn {
			t.Errorf("%s, first turn: direction %q window %.0f…%.0f, want %q %.0f…%.0f", pos, st.SteerDir, st.SteerLo, st.SteerHi, dirs[0], minTurn, maxTurn)
		}
		r.last.Step = StepCasterIn
		r.l.devices[r.id].Step = StepCasterIn
		st = r.cmd(phoneRequest{})
		if st.SteerDir != dirs[1] {
			t.Errorf("%s, second turn: direction %q, want %q", pos, st.SteerDir, dirs[1])
		}
	}
}

// TestSidewaysPhoneRefused: the mount correction is known only for the phone
// upright and upside down. Held sideways on the bar it must not send camber.
func TestSidewaysPhoneRefused(t *testing.T) {
	r := newRig(t)
	p := typicalPhone(81, 1)
	a := spinAxis(align.RR, -1, 0)
	r.cmd(phoneRequest{Wheel: "RR"})
	r.calibrate(p, a)
	r.l.hub.Clear("")
	up := p.onRim(a, true)
	sideways := rotate(up, geom.Rodrigues(up.z, math.Pi/2))
	st := r.hold(p, sideways, 3*time.Second)
	if r.hub.Frame().Params["camber_RR"].Has || st.Camber != nil {
		t.Error("a phone held sideways sent camber")
	}
}
