// Package suspension describes suspension designs: how each one is built, how to
// recognise it on your own car, which alignment angles it lets you adjust and
// with what, and what has to be checked before adjusting is worth doing.
//
// # Why this exists alongside the vehicle database
//
// Factory tolerances are model-specific and cannot be invented (see package
// specs). Knowing how the car is built is different: there are only a dozen or
// so suspension designs in the world, and the practical consequences of each
// are standard engineering knowledge. A beam axle on kingpins has no camber
// adjustment whatever the badge on the bonnet says; a twist beam has no rear
// adjustment; a kingpin that rocks in its bushes makes every reading
// meaningless. So even for a car with no data in the database — which today is
// most cars — the program can say what can be adjusted, with what, in which
// order, and what to fix first.
//
// Everything here is phrased as what is typical for the design, never as a
// statement about a particular model: where a mechanism varies between models
// the text says so and sends the reader to the manual.
package suspension

import "fmt"

// Type identifies a suspension design. The string values are stored in vehicle
// data files, so they must never change once published.
type Type string

const (
	Unknown Type = ""

	// Front, or either axle.
	MacPherson         Type = "macpherson"
	DoubleWishboneBall Type = "double_wishbone_ball"
	DoubleWishboneKing Type = "double_wishbone_kingpin"
	BeamKingpin        Type = "beam_kingpin"
	BeamBallJoint      Type = "beam_ball_joint"
	Multilink          Type = "multilink"

	// Rear.
	LiveAxleLeaf    Type = "live_axle_leaf"
	LiveAxleLinks   Type = "live_axle_links"
	TwistBeam       Type = "twist_beam"
	TrailingArm     Type = "trailing_arm"
	SemiTrailingArm Type = "semi_trailing_arm"
)

// Axle says where a design is found.
type Axle string

const (
	AxleFront Axle = "front"
	AxleRear  Axle = "rear"
	AxleBoth  Axle = "both"
)

// Adjust describes how one angle is set on a design.
type Adjust struct {
	// Usual reports whether the angle is normally adjustable on this design as
	// built at the factory. False does not mean nothing can be done — often a
	// repair kit exists — and How says so where it does.
	Usual bool   `json:"usual"`
	How   string `json:"how"`
	// Tip is the practical hint that saves an hour: which way to turn, what
	// else moves when this one does.
	Tip string `json:"tip,omitempty"`
}

// Info is everything the program knows about one design.
type Info struct {
	ID       Type     `json:"id"`
	Name     string   `json:"name"`
	Axle     Axle     `json:"axle"`
	Summary  string   `json:"summary"`
	Identify string   `json:"identify"`
	Examples []string `json:"examples,omitempty"`

	Camber Adjust `json:"camber"`
	// Caster applies to steered axles only; it is empty for rear-only designs.
	Caster Adjust `json:"caster"`
	Toe    Adjust `json:"toe"`

	// PreChecks must be done before any adjustment. Worn joints make the static
	// angles differ from the running ones, and adjusting a worn suspension only
	// moves the error somewhere the gauge cannot see it.
	PreChecks []string `json:"pre_checks"`
	Notes     []string `json:"notes,omitempty"`
}

// Get returns the description of a design. The zero Type yields a generic
// description suitable for "I do not know what my car has".
func Get(t Type) (Info, bool) {
	i, ok := catalogue[t]
	return i, ok
}

// MustGet is Get for types known to be valid, such as those that passed Valid.
func MustGet(t Type) Info {
	i, ok := catalogue[t]
	if !ok {
		return catalogue[Unknown]
	}
	return i
}

// Valid reports whether t is a known design (the empty type counts as known:
// it means "not specified").
func Valid(t Type) bool {
	_, ok := catalogue[t]
	return ok
}

// Validate is Valid as an error, for data-file loading.
func Validate(t Type, axle Axle) error {
	i, ok := catalogue[t]
	if !ok {
		return fmt.Errorf("неизвестный тип подвески %q", string(t))
	}
	if t != Unknown && i.Axle != AxleBoth && i.Axle != axle {
		return fmt.Errorf("тип подвески %q не бывает на %s оси", i.Name, axleGenitive(axle))
	}
	return nil
}

// For lists the designs found on the given axle, in display order.
func For(axle Axle) []Info {
	var out []Info
	for _, t := range order {
		i := catalogue[t]
		if i.Axle == AxleBoth || i.Axle == axle {
			out = append(out, i)
		}
	}
	return out
}

// All lists every design, in display order, the generic one last.
func All() []Info {
	out := make([]Info, 0, len(order)+1)
	for _, t := range order {
		out = append(out, catalogue[t])
	}
	return append(out, catalogue[Unknown])
}

func axleGenitive(a Axle) string {
	if a == AxleRear {
		return "задней"
	}
	return "передней"
}

// order is the display order: commonest designs first within each axle.
var order = []Type{
	MacPherson, DoubleWishboneBall, DoubleWishboneKing, BeamKingpin, BeamBallJoint, Multilink,
	TwistBeam, LiveAxleLeaf, LiveAxleLinks, TrailingArm, SemiTrailingArm,
}

func init() {
	// Every design in the catalogue must be in the display order and vice
	// versa; a design missing from one of them would silently vanish from the
	// interface.
	seen := map[Type]bool{}
	for _, t := range order {
		if _, ok := catalogue[t]; !ok {
			panic("suspension: order lists unknown type " + string(t))
		}
		seen[t] = true
	}
	for t := range catalogue {
		if t != Unknown && !seen[t] {
			panic("suspension: type missing from display order: " + string(t))
		}
	}
}
