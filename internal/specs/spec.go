// Package specs holds vehicle alignment specifications and compares
// measurements against them.
//
// # On the completeness of this database
//
// It is not complete and it never claims to be. Comprehensive factory
// alignment data is a licensed commercial product (Autodata, Mitchell,
// ALLDATA, Hunter, TecAlliance), and inventing the numbers would be worse than
// having none: a person who trusts a fabricated camber figure and adjusts to it
// ends up with a car that wears tyres out in a season, or that behaves badly in
// an emergency swerve.
//
// So every specification carries a Source, and every Source states where the
// figures came from and whether anyone has checked them. Anything not verified
// against a factory document is flagged as such everywhere it is displayed —
// see (Spec).Trust and (Spec).Disclaimer. The database is designed to be filled
// in by people with real service manuals in front of them, and the program is
// designed to be useful even when it has nothing for your car, by falling back
// to class-based engineering guidance that is clearly labelled as guidance.
package specs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
)

// SourceKind says where a set of figures came from. The ordering is meaningful:
// higher is more trustworthy.
type SourceKind string

const (
	// SourceFactory: transcribed from the manufacturer's own service
	// documentation, with the document identified in Reference.
	SourceFactory SourceKind = "factory"
	// SourceLicensed: from a commercial alignment data provider.
	SourceLicensed SourceKind = "licensed"
	// SourceCommunity: contributed by an owner or workshop and cross-checked
	// against at least one other independent source.
	SourceCommunity SourceKind = "community"
	// SourceUnverified: entered from a single uncorroborated source. Usable as
	// a starting point, never as an authority.
	SourceUnverified SourceKind = "unverified"
	// SourceClassGuidance: not vehicle-specific at all — typical ranges for a
	// class of vehicle, derived from engineering practice. Present so that the
	// program can still help someone whose car is not in the database, and
	// always presented as guidance rather than specification.
	SourceClassGuidance SourceKind = "class_guidance"
)

// Trust ranks a source for display and for choosing between competing entries.
func (k SourceKind) Trust() int {
	switch k {
	case SourceFactory:
		return 4
	case SourceLicensed:
		return 3
	case SourceCommunity:
		return 2
	case SourceUnverified:
		return 1
	default:
		return 0
	}
}

// RussianName is the label shown next to every figure from this source.
func (k SourceKind) RussianName() string {
	switch k {
	case SourceFactory:
		return "Заводское руководство"
	case SourceLicensed:
		return "Лицензионная база данных"
	case SourceCommunity:
		return "Сообщество (перепроверено)"
	case SourceUnverified:
		return "Не проверено"
	case SourceClassGuidance:
		return "Ориентировочные значения для класса"
	}
	return "Источник неизвестен"
}

// Source documents the provenance of one specification.
type Source struct {
	Kind SourceKind `json:"kind"`

	// Reference identifies the document: title, publisher, edition, page. For
	// community entries, how it was cross-checked.
	Reference string `json:"reference"`

	Contributor string `json:"contributor,omitempty"`
	Added       string `json:"added,omitempty"` // ISO date
	URL         string `json:"url,omitempty"`
}

// Validate rejects a source that does not actually document anything.
func (s Source) Validate() error {
	if s.Kind == "" {
		return errors.New("источник не указан")
	}
	if s.Kind != SourceClassGuidance && strings.TrimSpace(s.Reference) == "" {
		return fmt.Errorf("для источника %q обязательна ссылка на документ", s.Kind)
	}
	return nil
}

// AxleSpec is the factory tolerance for one axle.
//
// Ranges are in degrees on the wire (align.Angle marshals as degrees). Nil
// means the manufacturer does not specify or does not make it adjustable —
// which is itself important information and quite different from zero.
type AxleSpec struct {
	Camber *align.Range `json:"camber,omitempty"`
	Caster *align.Range `json:"caster,omitempty"`
	SAI    *align.Range `json:"sai,omitempty"`

	// TotalToe is the sum over the axle; IndividualToe is per wheel. Most
	// manuals give one or the other. Where only total is given, individual is
	// taken as half of it, which is correct for a symmetric car.
	TotalToe      *align.Range `json:"total_toe,omitempty"`
	IndividualToe *align.Range `json:"individual_toe,omitempty"`

	// MaxCrossCamber and MaxCrossCaster are the permitted side-to-side
	// differences. These matter more than the absolute figures for whether the
	// car pulls, and manuals often specify them separately.
	MaxCrossCamber *align.Angle `json:"max_cross_camber,omitempty"`
	MaxCrossCaster *align.Angle `json:"max_cross_caster,omitempty"`

	// Adjustable records what can actually be changed on this axle, so the
	// program does not tell someone to correct an angle their car has no
	// provision for.
	Adjustable Adjustability `json:"adjustable"`
}

// Adjustability describes what a person can change, and how.
type Adjustability struct {
	Camber bool `json:"camber"`
	Caster bool `json:"caster"`
	Toe    bool `json:"toe"`

	// CamberMethod and friends describe the mechanism in plain Russian:
	// "эксцентриковый болт нижнего рычага", "регулировочные шайбы между
	// верхним рычагом и кронштейном", "не регулируется штатно".
	CamberMethod string `json:"camber_method,omitempty"`
	CasterMethod string `json:"caster_method,omitempty"`
	ToeMethod    string `json:"toe_method,omitempty"`
}

// Conditions are the states the car must be in for the figures to apply.
// Getting these wrong invalidates the whole measurement, and they are the most
// commonly skipped step in a home alignment.
type Conditions struct {
	Load             string  `json:"load,omitempty"`     // "снаряжённая масса", "+ 2 человека на передних сиденьях"
	TyrePressure     string  `json:"pressure,omitempty"` // "по табличке на стойке двери"
	FuelState        string  `json:"fuel,omitempty"`     // "полный бак"
	RideHeightMM     float64 `json:"ride_height_mm,omitempty"`
	RideHeightNote   string  `json:"ride_height_note,omitempty"`
	SettleProcedure  string  `json:"settle,omitempty"` // "раскачать кузов и прокатить 3–5 м вперёд"
	AdditionalChecks string  `json:"checks,omitempty"`
}

// Spec is a complete alignment specification for one vehicle variant.
type Spec struct {
	ID    string   `json:"id"`
	Make  string   `json:"make"`
	Model string   `json:"model"`
	Trim  string   `json:"trim,omitempty"`
	Notes string   `json:"notes,omitempty"`
	Tags  []string `json:"tags,omitempty"`

	YearFrom int `json:"year_from"`
	YearTo   int `json:"year_to"` // 0 = still current

	// RimDiameterIn is the reference diameter for any figure quoted in
	// millimetres. A millimetre toe spec without its rim diameter is
	// meaningless, and manuals frequently omit it, so it is required here.
	RimDiameterIn float64 `json:"rim_diameter_in,omitempty"`

	// TotalToeMM, where the original document quoted toe linearly. Kept
	// alongside the angular form so a user can check the program against the
	// manual in the manual's own units.
	FrontTotalToeMM *MMRange `json:"front_total_toe_mm,omitempty"`
	RearTotalToeMM  *MMRange `json:"rear_total_toe_mm,omitempty"`

	Front AxleSpec `json:"front"`
	Rear  AxleSpec `json:"rear"`

	MaxThrustAngle *align.Angle `json:"max_thrust_angle,omitempty"`

	Conditions Conditions `json:"conditions"`
	Source     Source     `json:"source"`
}

// MMRange is a tolerance quoted in millimetres of toe.
type MMRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// ToAngle converts a millimetre range to angles at the given rim diameter.
func (m MMRange) ToAngle(rimDiaMM float64) align.Range {
	return align.RangeMinMax(
		align.ToeAngleFromMM(m.Min, rimDiaMM),
		align.ToeAngleFromMM(m.Max, rimDiaMM),
	)
}

// Title is the human-readable name of the vehicle.
func (s Spec) Title() string {
	years := fmt.Sprintf("%d–", s.YearFrom)
	if s.YearTo > 0 {
		years = fmt.Sprintf("%d–%d", s.YearFrom, s.YearTo)
	}
	parts := []string{s.Make, s.Model}
	if s.Trim != "" {
		parts = append(parts, s.Trim)
	}
	return fmt.Sprintf("%s (%s)", strings.Join(parts, " "), years)
}

// RimDiameterMM is the reference diameter for millimetre toe figures.
func (s Spec) RimDiameterMM() float64 {
	if s.RimDiameterIn <= 0 {
		return 0
	}
	return align.Inches(s.RimDiameterIn)
}

// Verified reports whether these figures have been checked against a
// manufacturer or licensed source.
func (s Spec) Verified() bool {
	return s.Source.Kind == SourceFactory || s.Source.Kind == SourceLicensed
}

// Disclaimer is the warning that must accompany any unverified figure. It is
// returned as a value rather than left to the UI so that no display path can
// forget it.
func (s Spec) Disclaimer() string {
	switch s.Source.Kind {
	case SourceFactory, SourceLicensed:
		return ""
	case SourceCommunity:
		return "Данные внесены сообществом и перепроверены по независимому источнику, но это не заводской документ. " +
			"Перед регулировкой сверьтесь с руководством по ремонту вашего автомобиля."
	case SourceClassGuidance:
		return "ЭТО НЕ ЗАВОДСКИЕ ДАННЫЕ вашего автомобиля. Это типичные значения для автомобилей такого класса — " +
			"они помогут понять, насколько сильно ваши углы отличаются от разумных, но регулировать «в них» нельзя. " +
			"Найдите заводские данные для вашей модели."
	default:
		return "ВНИМАНИЕ: данные не проверены. Обязательно сверьтесь с руководством по ремонту вашего автомобиля " +
			"перед тем, как что-либо регулировать."
	}
}

// Validate checks a specification for the mistakes that make it dangerous:
// missing provenance, inverted tolerance bands, millimetre toe with no rim
// diameter to interpret it against.
func (s Spec) Validate() error {
	var errs []string
	if strings.TrimSpace(s.ID) == "" {
		errs = append(errs, "не задан id")
	}
	if strings.TrimSpace(s.Make) == "" || strings.TrimSpace(s.Model) == "" {
		errs = append(errs, "не заданы марка и модель")
	}
	if s.YearFrom == 0 {
		errs = append(errs, "не задан год начала выпуска")
	}
	if s.YearTo != 0 && s.YearTo < s.YearFrom {
		errs = append(errs, "год окончания выпуска раньше года начала")
	}
	if err := s.Source.Validate(); err != nil {
		errs = append(errs, err.Error())
	}
	if (s.FrontTotalToeMM != nil || s.RearTotalToeMM != nil) && s.RimDiameterIn <= 0 {
		errs = append(errs, "схождение задано в миллиметрах, но не указан диаметр обода — "+
			"такие данные невозможно интерпретировать")
	}
	for name, ax := range map[string]AxleSpec{"передняя ось": s.Front, "задняя ось": s.Rear} {
		for pname, r := range map[string]*align.Range{
			"развал": ax.Camber, "кастер": ax.Caster, "SAI": ax.SAI,
			"суммарное схождение": ax.TotalToe, "схождение колеса": ax.IndividualToe,
		} {
			if r == nil {
				continue
			}
			if r.Max < r.Min {
				errs = append(errs, fmt.Sprintf("%s, %s: верхняя граница меньше нижней", name, pname))
			}
			if r.Nominal < r.Min || r.Nominal > r.Max {
				errs = append(errs, fmt.Sprintf("%s, %s: номинал вне допуска", name, pname))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s: %s", s.ID, strings.Join(errs, "; "))
	}
	return nil
}

// Resolve fills in derived figures: individual toe from total (and vice
// versa), and angular toe from millimetre toe. Called once when the database
// loads so the rest of the program never has to care which form a manual used.
func (s *Spec) Resolve() {
	rim := s.RimDiameterMM()
	resolveAxle(&s.Front, s.FrontTotalToeMM, rim)
	resolveAxle(&s.Rear, s.RearTotalToeMM, rim)
}

func resolveAxle(ax *AxleSpec, mm *MMRange, rimMM float64) {
	if ax.TotalToe == nil && mm != nil && rimMM > 0 {
		r := mm.ToAngle(rimMM)
		ax.TotalToe = &r
	}
	// A symmetric car splits total toe evenly between its two wheels.
	if ax.IndividualToe == nil && ax.TotalToe != nil {
		r := align.Range{Min: ax.TotalToe.Min / 2, Nominal: ax.TotalToe.Nominal / 2, Max: ax.TotalToe.Max / 2}
		ax.IndividualToe = &r
	}
	if ax.TotalToe == nil && ax.IndividualToe != nil {
		r := align.Range{Min: ax.IndividualToe.Min * 2, Nominal: ax.IndividualToe.Nominal * 2, Max: ax.IndividualToe.Max * 2}
		ax.TotalToe = &r
	}
}
