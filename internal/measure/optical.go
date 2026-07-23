package measure

import (
	"fmt"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// OpticalSession is a full four-wheel optical measurement: every wheel already
// registered into the one reference frame the floor board defines.
//
// This is where the optical path finally reaches the same place the string path
// reaches — an align.WheelSet — after which the reporting, the comparison
// against the vehicle's spec and the adjustment advice are shared, because they
// were never allowed to depend on how the angles were obtained.
type OpticalSession struct {
	Wheels map[align.Position]vision.RegisteredWheel

	// RimDiameterMM is the reference diameter for toe quoted in millimetres.
	RimDiameterMM float64
}

// Result assembles the four registered wheels into a complete alignment.
//
// Two facts make the reference frame directly usable as the vehicle's vertical
// reference. The floor board lies flat on the ground, so the frame's +Z is the
// road normal — a better vertical than fitting a plane through four wheel
// centres, because it is measured against the surface the tyres actually stand
// on rather than inferred. And a wheel centre's height in that frame is its
// loaded rolling radius, so that too is measured rather than assumed.
func (s OpticalSession) Result() (align.Result, error) {
	for _, p := range align.AllPositions {
		w, ok := s.Wheels[p]
		if !ok {
			return align.Result{}, fmt.Errorf("%w: %s (%s)", align.ErrMissingWheel, p, p.RussianName())
		}
		if w.Axis.Len() < 0.5 {
			return align.Result{}, fmt.Errorf("%s: ось вращения не восстановлена", p)
		}
	}

	// Which way is outboard is a fact about the vehicle, not about any one
	// wheel, so it is settled only now that all four centres are known: outboard
	// is away from the centroid of the four. The vector from the centroid to a
	// wheel is dominated by its longitudinal part, but the spin axis is very
	// nearly perpendicular to that (toe is under a degree), so the sign is
	// decided by the lateral part by a wide margin.
	var centroid geom.Vec3
	for _, p := range align.AllPositions {
		centroid = centroid.Add(s.Wheels[p].Center)
	}
	centroid = centroid.Scale(0.25)

	obs := make([]align.WheelObs, 0, 4)
	var warnings []string
	for _, p := range align.AllPositions {
		w := s.Wheels[p]
		axis := w.Axis.Unit()
		outward := w.Center.Sub(centroid)
		if axis.Dot(outward) < 0 {
			axis = axis.Neg()
		}

		q := align.Quality{
			RunoutCompensated: true,
			RunoutMagnitude:   align.Deg(w.RunoutDeg),
			Frames:            w.Used,
			Warnings:          w.Warnings,
		}
		for _, f := range w.Frames {
			if f.OK && f.RMSPx > q.PoseRMSPx {
				q.PoseRMSPx = f.RMSPx
			}
		}

		obs = append(obs, align.WheelObs{
			Pos:      p,
			SpinAxis: axis,
			Center:   w.Center,
			// Height above the floor board's plane is the loaded radius.
			RollingRadiusMM: w.Center.Z,
			RimDiameterMM:   s.RimDiameterMM,
			Quality:         q,
		})

		if w.Center.Z < 100 || w.Center.Z > 500 {
			warnings = append(warnings, fmt.Sprintf(
				"%s: центр колеса на высоте %.0f мм над плоскостью напольной мишени — это не похоже на радиус качения. "+
					"Убедитесь, что напольная мишень лежит на полу лицом вверх и не приподнята.",
				p.RussianName(), w.Center.Z))
		}
	}

	ws, err := align.NewWheelSet(obs...)
	if err != nil {
		return align.Result{}, err
	}

	// Up is the floor board's own normal, which in the reference frame is +Z,
	// so gravity points along −Z.
	res, err := align.Compute(ws, align.FrameOptions{
		Mode:    align.RefGravity,
		Gravity: geom.V(0, 0, -1),
	})
	if err != nil {
		return align.Result{}, err
	}
	res.Reference.Mode = "optical_floor_reference"
	res.Warnings = append(warnings, res.Warnings...)
	return res, nil
}
