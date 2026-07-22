// Package vision turns camera observations of wheel-mounted targets into the
// 6-DOF poses that the measurement layer consumes.
//
// The split with the rest of the program is deliberate and strict: this package
// knows about pixels, lenses and targets, and knows nothing about camber, toe
// or cars. It hands package measure a list of geom.Pose values, and everything
// downstream is identical to the manual path. That is what makes the optical
// mode an addition rather than a second, parallel implementation of the same
// physics.
//
// Camera frame convention: +X right, +Y down, +Z forward along the optical
// axis — the standard computer-vision frame, which is NOT the vehicle frame
// used in package geom. The conversion happens once, at the boundary.
package vision

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// Point2 is a point on the image sensor, in pixels.
type Point2 struct {
	X, Y float64
}

func (p Point2) Sub(q Point2) Point2     { return Point2{p.X - q.X, p.Y - q.Y} }
func (p Point2) Len() float64            { return math.Hypot(p.X, p.Y) }
func (p Point2) DistTo(q Point2) float64 { return p.Sub(q).Len() }

// Camera is a pinhole camera with Brown–Conrady lens distortion.
//
// The distortion model matters more here than in most applications. Wheel
// targets sit near the edges of the frame — that is where the wheels are — and
// radial distortion grows with the fourth and sixth power of image radius. An
// uncorrected 3 % barrel distortion at the frame edge moves a target corner by
// tens of pixels, which is degrees of camber.
type Camera struct {
	Width  int `json:"width"`
	Height int `json:"height"`

	// Focal lengths and principal point, in pixels.
	Fx float64 `json:"fx"`
	Fy float64 `json:"fy"`
	Cx float64 `json:"cx"`
	Cy float64 `json:"cy"`

	// Brown–Conrady coefficients: K1..K3 radial, P1/P2 tangential.
	K1 float64 `json:"k1"`
	K2 float64 `json:"k2"`
	P1 float64 `json:"p1"`
	P2 float64 `json:"p2"`
	K3 float64 `json:"k3"`

	// Calibrated records whether these numbers came from an actual calibration
	// or were guessed from the field of view. Guessed intrinsics produce poses
	// that look plausible and are wrong, so the flag travels with the data and
	// every result derived from an uncalibrated camera is marked.
	Calibrated bool `json:"calibrated"`

	// CalibrationRMSPx is the reprojection error achieved during calibration —
	// the single most useful number for judging whether a calibration is any
	// good. Below about 0.3 px is healthy; above 1 px means something was
	// wrong with the procedure.
	CalibrationRMSPx float64 `json:"calibration_rms_px,omitempty"`
	CalibrationNote  string  `json:"calibration_note,omitempty"`
}

var ErrUncalibrated = errors.New("vision: камера не откалибрована")

// GuessFromFOV builds an approximate camera from the image size and a
// horizontal field of view in degrees, assuming a centred principal point and
// no distortion.
//
// This exists so that someone can try the optical mode before calibrating, and
// for nothing else. It is marked uncalibrated, and the errors it produces are
// systematic rather than random: they will not average away over more frames.
func GuessFromFOV(width, height int, fovDeg float64) Camera {
	f := float64(width) / (2 * math.Tan(geom.Rad(fovDeg)/2))
	return Camera{
		Width: width, Height: height,
		Fx: f, Fy: f,
		Cx: float64(width) / 2, Cy: float64(height) / 2,
		Calibrated: false,
		CalibrationNote: "Параметры камеры оценены по углу обзора, а не измерены. " +
			"Ошибка будет систематической: усреднение по кадрам её не уберёт. Откалибруйте камеру.",
	}
}

// LoadCamera reads a calibration file.
func LoadCamera(path string) (Camera, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Camera{}, fmt.Errorf("не удалось прочитать калибровку камеры: %w", err)
	}
	var c Camera
	if err := json.Unmarshal(b, &c); err != nil {
		return Camera{}, fmt.Errorf("файл калибровки повреждён: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Camera{}, err
	}
	return c, nil
}

// Save writes the calibration.
func (c Camera) Save(path string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Validate rejects intrinsics that cannot describe a real camera.
func (c Camera) Validate() error {
	switch {
	case c.Width <= 0 || c.Height <= 0:
		return errors.New("vision: не задан размер кадра")
	case c.Fx <= 0 || c.Fy <= 0:
		return errors.New("vision: фокусное расстояние должно быть положительным")
	case math.Abs(c.Fx/c.Fy-1) > 0.5:
		return errors.New("vision: fx и fy отличаются более чем в полтора раза — почти наверняка ошибка калибровки")
	case c.Cx < 0 || c.Cx > float64(c.Width) || c.Cy < 0 || c.Cy > float64(c.Height):
		return errors.New("vision: главная точка вне кадра")
	}
	return nil
}

// Warnings lists reasons to distrust results from this camera.
func (c Camera) Warnings() []string {
	var out []string
	if !c.Calibrated {
		out = append(out, "Камера не откалибрована — углы будут иметь систематическую ошибку. "+
			"Проведите калибровку по шахматной доске.")
	}
	if c.Calibrated && c.CalibrationRMSPx > 1.0 {
		out = append(out, fmt.Sprintf(
			"Ошибка калибровки %.2f пикс — это много. Пересниммите калибровочную серию: "+
				"доска должна занимать кадр целиком и попадать в углы кадра.", c.CalibrationRMSPx))
	}
	return out
}

// distort applies lens distortion to a normalised image point (the point where
// the ray pierces the plane Z = 1 in camera coordinates).
func (c Camera) distort(x, y float64) (float64, float64) {
	r2 := x*x + y*y
	radial := 1 + r2*(c.K1+r2*(c.K2+r2*c.K3))
	dx := 2*c.P1*x*y + c.P2*(r2+2*x*x)
	dy := c.P1*(r2+2*y*y) + 2*c.P2*x*y
	return x*radial + dx, y*radial + dy
}

// undistort inverts distort by fixed-point iteration.
//
// The forward model is a polynomial with no closed-form inverse. The iteration
// converges quickly for the modest distortion of any usable lens; it is capped
// so that a wildly wrong calibration fails visibly rather than spinning.
func (c Camera) undistort(xd, yd float64) (float64, float64) {
	x, y := xd, yd
	for i := 0; i < 20; i++ {
		r2 := x*x + y*y
		radial := 1 + r2*(c.K1+r2*(c.K2+r2*c.K3))
		if radial < 1e-6 {
			break
		}
		dx := 2*c.P1*x*y + c.P2*(r2+2*x*x)
		dy := c.P1*(r2+2*y*y) + 2*c.P2*x*y
		nx, ny := (xd-dx)/radial, (yd-dy)/radial
		if math.Abs(nx-x) < 1e-12 && math.Abs(ny-y) < 1e-12 {
			return nx, ny
		}
		x, y = nx, ny
	}
	return x, y
}

// Project maps a point in camera coordinates to pixels. ok is false when the
// point is behind the camera, where the projection is meaningless.
func (c Camera) Project(p geom.Vec3) (Point2, bool) {
	if p.Z <= 1e-9 {
		return Point2{}, false
	}
	xd, yd := c.distort(p.X/p.Z, p.Y/p.Z)
	return Point2{X: c.Fx*xd + c.Cx, Y: c.Fy*yd + c.Cy}, true
}

// Normalize maps a pixel to the undistorted normalised image plane: the
// direction of the ray through that pixel, scaled so its Z component is 1.
func (c Camera) Normalize(p Point2) (x, y float64) {
	return c.undistort((p.X-c.Cx)/c.Fx, (p.Y-c.Cy)/c.Fy)
}

// Ray returns the unit direction, in camera coordinates, of the ray through a
// pixel.
func (c Camera) Ray(p Point2) geom.Vec3 {
	x, y := c.Normalize(p)
	return geom.V(x, y, 1).Unit()
}

// ProjectPose projects model points through a pose into pixels. The pose maps
// target coordinates into camera coordinates.
func (c Camera) ProjectPose(pose geom.Pose, model []geom.Vec3) ([]Point2, bool) {
	out := make([]Point2, len(model))
	for i, m := range model {
		p, ok := c.Project(pose.Apply(m))
		if !ok {
			return out, false
		}
		out[i] = p
	}
	return out, true
}
