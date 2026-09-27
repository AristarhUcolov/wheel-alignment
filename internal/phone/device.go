package phone

import (
	"fmt"
	"math"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
)

// Step is where a phone is in its procedures.
type Step string

const (
	StepLive       Step = "live"
	StepFlatUp     Step = "flat_up"
	StepFlatDown   Step = "flat_down"
	StepMountUp    Step = "mount_up"
	StepMountDown  Step = "mount_down"
	StepRunout1    Step = "runout_1"
	StepRunout2    Step = "runout_2"
	StepCasterZero Step = "caster_zero"
	StepCasterOut  Step = "caster_out"
	StepCasterIn   Step = "caster_in"
	StepCasterBack Step = "caster_back"
)

// Timing and thresholds of the hold-still detector. The phone must be still
// for the step's hold time before a reading is taken, and the reading is the
// mean over that time — around a hundred raw accelerometer samples, which is
// what brings a phone's noise down to hundredths of a degree. Calibration is
// done once and everything after rests on it, so it is averaged twice as long.
const (
	holdTime     = 1500 * time.Millisecond
	calHoldTime  = 3000 * time.Millisecond
	stillAccel   = 0.06 // m/s², standard deviation of each axis
	stillRate    = 1.5  // deg/s, mean rotation about the vertical
	minTurn      = 15.0 // degrees of steer before a caster reading is taken
	maxTurn      = 32.0
	maxRunoutDeg = 1.0
)

type timed struct {
	t time.Time
	s Sample
}

// Device is one phone and its progress through the procedures.
type Device struct {
	ID       string
	Name     string
	Wheel    align.Position
	HasWheel bool
	Cal      Calibration

	Step     Step
	Prompt   string
	Err      string
	Progress float64
	LastSeen time.Time

	win []timed

	camber   float64
	camberOK bool
	stable   bool

	// live holds recent per-batch camber readings for the value sent to the
	// screen: all of them while the phone is still, the last few while it is
	// moving. See liveCamber.
	live []float64

	runout    float64
	hasRunout bool

	// Procedure scratch.
	hold    Sample
	holdC   float64
	moved   bool
	yaw     float64
	gbias   [3]float64 // gyroscope zero offset, deg/s, measured with the wheel still
	hasBias bool
	outSign float64
	c0      float64
	cOut    float64
	tOut    float64
	hasGyro bool

	Caster *float64
	SAI    *float64
	// sweepDone is set when a caster sweep has produced a result to publish.
	sweepDone *measure.SweepSolution
	sweepWarn []string
}

func newDevice(id string, cal Calibration) *Device {
	d := &Device{ID: id, Cal: cal, Step: StepLive}
	d.setPrompt()
	return d
}

// Start begins a procedure. It returns an error the phone can display when the
// procedure cannot start yet.
func (d *Device) Start(cmd string) error {
	d.Err = ""
	d.win = d.win[:0]
	d.Progress = 0
	d.moved = false
	switch cmd {
	case "flat":
		d.Step = StepFlatUp
	case "mount":
		if !d.Cal.Flat {
			d.Step = StepFlatUp // the mount offset is measured on top of the flat one
		} else {
			d.Step = StepMountUp
		}
	case "runout":
		if !d.Cal.Ready() {
			return fmt.Errorf("сначала откалибруйте телефон")
		}
		d.Step = StepRunout1
	case "caster":
		if !d.Cal.Ready() {
			return fmt.Errorf("сначала откалибруйте телефон")
		}
		if !d.HasWheel || !d.Wheel.IsFront() {
			return fmt.Errorf("кастер меряется на передних колёсах — выберите переднее колесо")
		}
		if !d.hasGyro {
			return fmt.Errorf("в этом телефоне нет гироскопа — кастер меряйте угломером по шкале поворотных кругов")
		}
		d.Step = StepCasterZero
		d.hasBias = false
	case "cancel":
		d.Step = StepLive
	case "reset_runout":
		d.hasRunout, d.runout = false, 0
		d.Step = StepLive
	default:
		return fmt.Errorf("неизвестная команда %q", cmd)
	}
	d.setPrompt()
	return nil
}

// SetWheel assigns the phone to a wheel. Runout compensation belongs to a
// wheel, so it is dropped.
func (d *Device) SetWheel(p align.Position) {
	d.Wheel, d.HasWheel = p, true
	d.hasRunout, d.runout = false, 0
	d.live = d.live[:0]
	d.Caster, d.SAI = nil, nil
	if d.Step != StepFlatUp && d.Step != StepFlatDown {
		d.Step = StepLive
	}
	d.setPrompt()
}

// Feed processes one batch received at time t. It returns true when the
// calibration changed and should be saved.
func (d *Device) Feed(s Sample, t time.Time) (calChanged bool) {
	d.LastSeen = t
	if s.HasGyro {
		d.hasGyro = true
	}
	if s.N == 0 {
		return false
	}
	// Steering angle integrates continuously through the caster steps.
	if d.Step == StepCasterOut || d.Step == StepCasterIn || d.Step == StepCasterBack {
		d.yaw += d.steerRate(s) * s.DT
	}

	d.win = append(d.win, timed{t, s})
	cut := 0
	for cut < len(d.win) && t.Sub(d.win[cut].t) > calHoldTime {
		cut++
	}
	d.win = d.win[cut:]
	need := d.holdFor()
	mean, still, span := d.stats(need)
	d.stable = still && span >= need*9/10
	if !still {
		d.moved = true
	}
	d.Progress = 0
	if still {
		d.Progress = math.Min(1, float64(span)/float64(need*9/10))
	}

	d.camber, d.camberOK = 0, false
	if d.Cal.Ready() {
		if c, ok := d.Cal.Camber(s); ok {
			if d.hasRunout {
				c += d.runout
			}
			d.camber, d.camberOK = d.liveCamber(c, still), true
		} else {
			d.live = d.live[:0]
		}
	}

	if d.stable {
		calChanged = d.onStill(mean)
	}
	d.setPrompt()
	return calChanged
}

// Averaging of the value sent to the screen. A single 100 ms batch from a
// phone carries a few hundredths of a degree of noise — about the size of the
// thing being adjusted. While the phone is still, the reading is averaged over
// everything since it came to rest, up to liveStill; that is what makes a phone
// good to a few hundredths. The moment it moves, only the last liveMoving
// batches count, so the needle follows a spanner without lag.
const (
	liveStill  = 30 // batches, 3 s
	liveMoving = 3
)

func (d *Device) liveCamber(c float64, still bool) float64 {
	if !still {
		if len(d.live) > liveMoving {
			d.live = append(d.live[:0], d.live[len(d.live)-liveMoving+1:]...)
		}
	}
	d.live = append(d.live, c)
	if len(d.live) > liveStill {
		d.live = append(d.live[:0], d.live[len(d.live)-liveStill:]...)
	}
	sum := 0.0
	for _, v := range d.live {
		sum += v
	}
	return sum / float64(len(d.live))
}

// holdFor is how long the current step needs the phone still.
func (d *Device) holdFor() time.Duration {
	switch d.Step {
	case StepFlatUp, StepFlatDown, StepMountUp, StepMountDown:
		return calHoldTime
	}
	return holdTime
}

// steerRate is the steering rate, deg/s, from the gyroscope.
//
// A wheel on turntables with the brake held turns about its steering axis and
// nothing else, so the steering rate is the whole length of the rotation-rate
// vector, once the gyroscope's own offset is removed. Projecting onto the
// vertical instead would be wrong by the cosine of the steering axis's tilt —
// 2.4% for a typical 12° — and caster would carry the same error. The vertical
// component is used only for the sign.
func (d *Device) steerRate(s Sample) float64 {
	wx, wy, wz := s.GX-d.gbias[0], s.GY-d.gbias[1], s.GZ-d.gbias[2]
	mag := math.Sqrt(wx*wx + wy*wy + wz*wz)
	x, y, z := d.Cal.up(s)
	if wx*x+wy*y+wz*z < 0 {
		mag = -mag
	}
	return mag
}

// stats returns the mean over the last `need` of the window, whether the
// phone was still throughout it, and the time it covers.
func (d *Device) stats(need time.Duration) (Sample, bool, time.Duration) {
	var m Sample
	first := 0
	if len(d.win) > 0 {
		last := d.win[len(d.win)-1].t
		for first < len(d.win) && last.Sub(d.win[first].t) > need {
			first++
		}
	}
	win := d.win[first:]
	n := float64(len(win))
	if n == 0 {
		return m, false, 0
	}
	for _, w := range win {
		m.AX += w.s.AX / n
		m.AY += w.s.AY / n
		m.AZ += w.s.AZ / n
		m.GX += w.s.GX / n
		m.GY += w.s.GY / n
		m.GZ += w.s.GZ / n
		m.N += w.s.N
		m.DT += w.s.DT
	}
	m.HasGyro = d.hasGyro
	var vx, vy, vz float64
	for _, w := range win {
		vx += (w.s.AX - m.AX) * (w.s.AX - m.AX) / n
		vy += (w.s.AY - m.AY) * (w.s.AY - m.AY) / n
		vz += (w.s.AZ - m.AZ) * (w.s.AZ - m.AZ) / n
	}
	still := math.Sqrt(vx) < stillAccel && math.Sqrt(vy) < stillAccel && math.Sqrt(vz) < stillAccel
	if d.hasGyro {
		// With the offset known, "still" means turning slower than
		// stillRate; before it is known, a gyroscope's offset of a degree
		// or two a second must not count as movement.
		lim, rate := 5*stillRate, math.Sqrt(m.GX*m.GX+m.GY*m.GY+m.GZ*m.GZ)
		if d.hasBias {
			lim, rate = stillRate, math.Abs(d.steerRate(m))
		}
		if rate > lim {
			still = false
		}
	}
	span := win[len(win)-1].t.Sub(win[0].t)
	return m, still, span
}

// restart forgets the window, so the next reading is taken only after a fresh
// period of stillness — the phone has to be moved to the next position first.
func (d *Device) restart() {
	d.win = d.win[:0]
	d.moved = false
	d.stable = false
	d.Progress = 0
}

func (d *Device) onStill(m Sample) (calChanged bool) {
	switch d.Step {
	case StepFlatUp:
		if math.Hypot(m.AX, m.AY) > 1.5 {
			d.Err = ErrNotFlat.Error()
			return false
		}
		d.hold, d.Err = m, ""
		d.Step = StepFlatDown
		d.restart()

	case StepFlatDown:
		// Still screen up: the person has not turned it over yet.
		if (m.AZ > 0) == (d.hold.AZ > 0) {
			return false
		}
		sign, bias, err := SolveFlat(d.hold, m)
		if err != nil {
			d.Err = err.Error()
			d.Step = StepFlatUp
			d.restart()
			return false
		}
		d.Cal.Sign, d.Cal.BiasZ, d.Cal.Flat, d.Err = sign, bias, true, ""
		d.Cal.At = d.LastSeen
		d.Step = StepMountUp
		if d.Cal.Mounted {
			d.Step = StepLive
		}
		d.restart()
		return true

	case StepMountUp:
		if _, roll := d.Cal.tilt(m); math.Cos(rad(roll)) < 0.85 {
			d.Err = ErrNotUpright.Error()
			return false
		}
		d.hold, d.Err = m, ""
		d.Step = StepMountDown
		d.restart()

	case StepMountDown:
		if _, roll := d.Cal.tilt(m); math.Cos(rad(roll)) > -0.85 {
			return false // not turned over yet
		}
		mount, _, err := d.Cal.SolveMount(d.hold, m)
		if err != nil {
			d.Err = err.Error()
			d.Step = StepMountUp
			d.restart()
			return false
		}
		d.Cal.Mount, d.Cal.Mounted, d.Err = mount, true, ""
		d.Cal.At = d.LastSeen
		d.Step = StepLive
		d.restart()
		return true

	case StepRunout1:
		c, ok := d.Cal.Camber(m)
		if !ok {
			d.Err = ErrNotUpright.Error()
			return false
		}
		d.holdC, d.Err = c, ""
		d.Step = StepRunout2
		d.restart()

	case StepRunout2:
		// The car must have been rolled and the phone put back: require that
		// it moved in between, or a phone left on the rim would "measure" zero
		// runout.
		if !d.moved {
			return false
		}
		c, ok := d.Cal.Camber(m)
		if !ok {
			d.Err = ErrNotUpright.Error()
			return false
		}
		d.runout, d.hasRunout = (d.holdC-c)/2, true
		d.Err = ""
		if math.Abs(d.runout) > maxRunoutDeg {
			d.Err = fmt.Sprintf("биение диска %.1f° — диск погнут или на нём грязь; замеры этого колеса ненадёжны", math.Abs(d.runout))
		}
		d.Step = StepLive
		d.restart()

	case StepCasterZero:
		c, ok := d.Cal.Camber(m)
		if !ok {
			d.Err = ErrNotUpright.Error()
			return false
		}
		d.c0, d.yaw, d.Err = c, 0, ""
		d.gbias, d.hasBias = [3]float64{m.GX, m.GY, m.GZ}, true
		d.Step = StepCasterOut
		d.restart()

	case StepCasterOut:
		if math.Abs(d.yaw) < minTurn || math.Abs(d.yaw) > maxTurn {
			return false
		}
		c, ok := d.Cal.Camber(m)
		if !ok {
			return false
		}
		d.cOut, d.tOut, d.outSign = c, math.Abs(d.yaw), math.Copysign(1, d.yaw)
		d.Step = StepCasterIn
		d.restart()

	case StepCasterIn:
		if math.Copysign(1, d.yaw) == d.outSign || math.Abs(d.yaw) < minTurn || math.Abs(d.yaw) > maxTurn {
			return false
		}
		c, ok := d.Cal.Camber(m)
		if !ok {
			return false
		}
		sol, err := measure.SweepReading{
			CamberOut: align.Deg(d.cOut), CamberIn: align.Deg(c),
			CamberStraight: align.Deg(d.c0), HasStraight: true,
			SweepOut: align.Deg(d.tOut), SweepIn: align.Deg(math.Abs(d.yaw)),
		}.Solve(d.Wheel)
		if err != nil {
			d.Err = "не удалось посчитать кастер: " + err.Error()
			d.Step = StepLive
			d.restart()
			return false
		}
		cs := sol.Caster.Deg()
		d.Caster = &cs
		if sol.SAI != nil {
			v := sol.SAI.Deg()
			d.SAI = &v
		}
		d.sweepDone = &sol
		d.Step = StepCasterBack
		d.restart()

	case StepCasterBack:
		if math.Abs(d.yaw) < 2 {
			d.Step = StepLive
			d.restart()
		}
	}
	return false
}

func (d *Device) setPrompt() {
	switch d.Step {
	case StepFlatUp:
		d.Prompt = "Калибровка телефона, шаг 1 из 2. Положите телефон на ровный стол ЭКРАНОМ ВВЕРХ и не трогайте."
	case StepFlatDown:
		d.Prompt = "Шаг 2 из 2. Переверните телефон ЭКРАНОМ ВНИЗ на то же место и не трогайте."
	case StepMountUp:
		d.Prompt = "Калибровка крепления, шаг 1 из 2. Прижмите телефон к планке на ободе экраном наружу, вертикально, и держите неподвижно."
	case StepMountDown:
		d.Prompt = "Шаг 2 из 2. Переверните телефон вверх ногами и прижмите к тому же месту планки."
	case StepRunout1:
		d.Prompt = "Биение диска, шаг 1 из 2. Отметьте мелом место на шине сверху. Прижмите телефон к планке и держите."
	case StepRunout2:
		d.Prompt = "Шаг 2 из 2. Прокатите машину на пол-оборота колеса (метка внизу), приложите планку и телефон к тому же месту обода."
	case StepCasterZero:
		d.Prompt = "Кастер. Колёса прямо на поворотных кругах, педаль тормоза зафиксирована упором (колесо не должно проворачиваться), телефон на планке. Не трогайте руль — измеряю."
	case StepCasterOut:
		d.Prompt = fmt.Sprintf("Поверните колесо НАРУЖУ (от машины) примерно на 20° и остановитесь. Сейчас: %.1f°", math.Abs(d.yaw))
	case StepCasterIn:
		d.Prompt = fmt.Sprintf("Теперь поверните колесо ВНУТРЬ (к машине) примерно на 20° и остановитесь. Сейчас: %.1f°", d.yaw*-d.outSign)
	case StepCasterBack:
		d.Prompt = "Кастер измерен. Верните колёса прямо."
	default:
		switch {
		case !d.HasWheel:
			d.Prompt = "Выберите колесо, на котором стоит телефон."
		case !d.Cal.Flat:
			d.Prompt = "Нужна калибровка телефона (один раз, около минуты): нажмите «Калибровка телефона»."
		case !d.Cal.Mounted:
			d.Prompt = "Нужна калибровка крепления (один раз для телефона и планки): нажмите «Калибровка крепления»."
		case !d.camberOK:
			d.Prompt = "Держите телефон вертикально, экраном наружу, прижатым к планке."
		case !d.stable:
			d.Prompt = "Показания меняются…"
		default:
			d.Prompt = "Показания стабильны."
		}
	}
}
