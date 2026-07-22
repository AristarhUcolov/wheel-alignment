package specs

import (
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
)

// ParamReport is one measured value judged against its specification.
type ParamReport struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Axle  string `json:"axle"` // "front" | "rear" | "vehicle"

	Measured   align.Angle `json:"measured"`
	MeasuredMM *float64    `json:"measured_mm,omitempty"`

	Spec   *align.Range `json:"spec,omitempty"`
	SpecMM *MMRange     `json:"spec_mm,omitempty"`

	Status    align.Status `json:"status"`
	Deviation align.Angle  `json:"deviation"` // 0 when in spec

	Adjustable bool   `json:"adjustable"`
	Method     string `json:"method,omitempty"`
	Advice     string `json:"advice,omitempty"`
}

// Report is a complete comparison of a measurement against a specification.
type Report struct {
	SpecID    string `json:"spec_id,omitempty"`
	SpecTitle string `json:"spec_title,omitempty"`
	// SourceKind and Disclaimer travel with every report so that no display
	// path can show the numbers without showing how far to trust them.
	SourceKind    string        `json:"source_kind,omitempty"`
	SourceLabel   string        `json:"source_label,omitempty"`
	SourceRef     string        `json:"source_reference,omitempty"`
	Disclaimer    string        `json:"disclaimer,omitempty"`
	HasSpec       bool          `json:"has_spec"`
	ConditionsRU  string        `json:"conditions_ru,omitempty"`
	Params        []ParamReport `json:"params"`
	Steps         []Step        `json:"steps"`
	OverallStatus align.Status  `json:"overall_status"`
	OutOfSpec     int           `json:"out_of_spec"`
}

// Step is one instruction in the adjustment procedure.
type Step struct {
	Order  int    `json:"order"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Why    string `json:"why,omitempty"`
}

// Compare judges a measurement against a specification. Passing a nil spec
// produces a report with the measured values and no verdicts, which is what a
// person with an unlisted car gets — still useful, since cross values, thrust
// angle and included angle diagnose plenty on their own.
func Compare(res align.Result, spec *Spec) Report {
	r := Report{HasSpec: spec != nil}
	if spec != nil {
		r.SpecID = spec.ID
		r.SpecTitle = spec.Title()
		r.SourceKind = string(spec.Source.Kind)
		r.SourceLabel = spec.Source.Kind.RussianName()
		r.SourceRef = spec.Source.Reference
		r.Disclaimer = spec.Disclaimer()
		r.ConditionsRU = describeConditions(spec.Conditions)
	}

	add := func(p ParamReport) {
		if p.Spec != nil {
			p.Status = p.Spec.Grade(p.Measured)
			p.Deviation = p.Spec.Deviation(p.Measured)
		} else {
			p.Status = align.StatusNoSpec
		}
		if p.Status == align.StatusBad {
			r.OutOfSpec++
			p.Advice = adviceFor(p)
		}
		r.Params = append(r.Params, p)
	}

	for _, axle := range []struct {
		name  string
		l, rp align.Position
		spec  *AxleSpec
		sum   align.AxleResult
	}{
		{"front", align.FL, align.FR, axleOf(spec, true), res.Front},
		{"rear", align.RL, align.RR, axleOf(spec, false), res.Rear},
	} {
		for _, p := range []align.Position{axle.l, axle.rp} {
			w := res.Wheels[p.String()]
			add(ParamReport{
				Key: "camber_" + p.String(), Label: "Развал, " + p.RussianName(), Axle: axle.name,
				Measured: w.Camber, Spec: rangeOf(axle.spec, func(a *AxleSpec) *align.Range { return a.Camber }),
				Adjustable: adjOf(axle.spec, func(a *AxleSpec) bool { return a.Adjustable.Camber }),
				Method:     methodOf(axle.spec, func(a *AxleSpec) string { return a.Adjustable.CamberMethod }),
			})
		}
		add(ParamReport{
			Key: axle.name + "_cross_camber", Label: "Разница развала (лев − прав)", Axle: axle.name,
			Measured: axle.sum.CrossCamber, Spec: crossRange(axle.spec, true),
		})

		if res.Wheels[axle.l.String()].Caster != nil {
			for _, p := range []align.Position{axle.l, axle.rp} {
				w := res.Wheels[p.String()]
				if w.Caster == nil {
					continue
				}
				add(ParamReport{
					Key: "caster_" + p.String(), Label: "Кастер, " + p.RussianName(), Axle: axle.name,
					Measured: *w.Caster, Spec: rangeOf(axle.spec, func(a *AxleSpec) *align.Range { return a.Caster }),
					Adjustable: adjOf(axle.spec, func(a *AxleSpec) bool { return a.Adjustable.Caster }),
					Method:     methodOf(axle.spec, func(a *AxleSpec) string { return a.Adjustable.CasterMethod }),
				})
			}
			if axle.sum.CrossCaster != nil {
				add(ParamReport{
					Key: axle.name + "_cross_caster", Label: "Разница кастера (лев − прав)", Axle: axle.name,
					Measured: *axle.sum.CrossCaster, Spec: crossRange(axle.spec, false),
				})
			}
		}
		if res.Wheels[axle.l.String()].SAI != nil {
			for _, p := range []align.Position{axle.l, axle.rp} {
				w := res.Wheels[p.String()]
				if w.SAI == nil {
					continue
				}
				add(ParamReport{
					Key: "sai_" + p.String(), Label: "Поперечный наклон оси (SAI), " + p.RussianName(), Axle: axle.name,
					Measured: *w.SAI, Spec: rangeOf(axle.spec, func(a *AxleSpec) *align.Range { return a.SAI }),
				})
			}
		}

		rim := rimFor(spec, res, axle.l)
		for _, p := range []align.Position{axle.l, axle.rp} {
			w := res.Wheels[p.String()]
			mm := w.ToeThrust.ToeMM(rim)
			add(ParamReport{
				Key: "toe_" + p.String(), Label: "Схождение, " + p.RussianName(), Axle: axle.name,
				Measured: w.ToeThrust, MeasuredMM: &mm,
				Spec:       rangeOf(axle.spec, func(a *AxleSpec) *align.Range { return a.IndividualToe }),
				Adjustable: adjOf(axle.spec, func(a *AxleSpec) bool { return a.Adjustable.Toe }),
				Method:     methodOf(axle.spec, func(a *AxleSpec) string { return a.Adjustable.ToeMethod }),
			})
		}
		totalMM := axle.sum.TotalToe.ToeMM(rim)
		tp := ParamReport{
			Key: axle.name + "_total_toe", Label: "Суммарное схождение оси", Axle: axle.name,
			Measured: axle.sum.TotalToe, MeasuredMM: &totalMM,
			Spec:       rangeOf(axle.spec, func(a *AxleSpec) *align.Range { return a.TotalToe }),
			Adjustable: adjOf(axle.spec, func(a *AxleSpec) bool { return a.Adjustable.Toe }),
		}
		if spec != nil {
			if mmr := toeMMOf(spec, axle.name == "front"); mmr != nil {
				tp.SpecMM = mmr
			}
		}
		add(tp)
	}

	var thrustSpec *align.Range
	if spec != nil && spec.MaxThrustAngle != nil {
		t := align.RangeMinMax(-*spec.MaxThrustAngle, *spec.MaxThrustAngle)
		thrustSpec = &t
	}
	add(ParamReport{
		Key: "thrust_angle", Label: "Угол тяги", Axle: "vehicle",
		Measured: res.ThrustAngle, Spec: thrustSpec,
		Adjustable: adjOf(axleOf(spec, false), func(a *AxleSpec) bool { return a.Adjustable.Toe }),
		Method:     methodOf(axleOf(spec, false), func(a *AxleSpec) string { return a.Adjustable.ToeMethod }),
	})

	r.OverallStatus = align.StatusGood
	for _, p := range r.Params {
		if p.Status == align.StatusBad {
			r.OverallStatus = align.StatusBad
			break
		}
		if p.Status == align.StatusMarginal {
			r.OverallStatus = align.StatusMarginal
		}
	}
	r.Steps = buildProcedure(r, spec)
	return r
}

// buildProcedure orders the work. The order is not arbitrary and it is the part
// most home attempts get wrong: caster and camber both move toe when they
// change, so toe must be set last, and the rear axle defines the thrust line
// that front toe is referenced to, so the rear must be right before the front
// is touched at all.
func buildProcedure(r Report, spec *Spec) []Step {
	bad := map[string]ParamReport{}
	for _, p := range r.Params {
		if p.Status == align.StatusBad {
			bad[p.Key] = p
		}
	}
	if len(bad) == 0 {
		return []Step{{Order: 1, Title: "Регулировка не требуется",
			Detail: "Все измеренные углы в пределах допуска. Если автомобиль всё равно уводит — " +
				"проверьте давление в шинах, износ шин слева/справа, люфты в подвеске и рулевом, " +
				"а также тормозные механизмы."}}
	}

	var steps []Step
	n := 0
	step := func(title, detail, why string) {
		n++
		steps = append(steps, Step{Order: n, Title: title, Detail: detail, Why: why})
	}

	step("Подготовка",
		"Проверьте и выровняйте давление в шинах, устраните люфты в шаровых опорах, рулевых наконечниках, "+
			"сайлентблоках и ступичных подшипниках. Загрузите автомобиль так, как требует спецификация. "+
			"Прокатите машину 3–5 метров вперёд и дайте подвеске сесть.",
		"Изношенная подвеска даёт разные углы в статике и в движении — регулировать её бесполезно, "+
			"результат «уедет» на первой же кочке.")

	if hasAny(bad, "camber_RL", "camber_RR", "toe_RL", "toe_RR", "rear_total_toe", "thrust_angle") {
		step("Задняя ось: сначала развал, затем схождение",
			"Приведите в допуск развал задних колёс, затем схождение каждого колеса по отдельности. "+
				"Добивайтесь того, чтобы угол тяги был близок к нулю.",
			"Угол тяги задаётся задней осью. Пока он не нулевой, переднее схождение приходится «кривить» "+
				"под него, и руль не встанет ровно.")
	}
	if hasAny(bad, "caster_FL", "caster_FR", "front_cross_caster") {
		step("Передняя ось: кастер",
			"Отрегулируйте продольный наклон оси поворота. Разница между левым и правым бортом важнее, "+
				"чем абсолютное значение: именно она заставляет машину тянуть в сторону.",
			"Изменение кастера меняет и развал, и схождение, поэтому он идёт первым.")
	}
	if hasAny(bad, "camber_FL", "camber_FR", "front_cross_camber") {
		step("Передняя ось: развал",
			"Выставьте развал передних колёс, добиваясь минимальной разницы между бортами.",
			"Развал меняет схождение, поэтому схождение регулируется после него, а не до.")
	}
	if hasAny(bad, "toe_FL", "toe_FR", "front_total_toe") {
		step("Передняя ось: схождение — в последнюю очередь",
			"Зафиксируйте руль строго в положении «прямо» (по спицам и по метке), затем выставьте схождение "+
				"КАЖДОГО колеса по отдельности, а не только суммарное. Обе тяги крутите на одинаковую величину "+
				"в противоположные стороны, чтобы руль остался ровным.",
			"Суммарное схождение можно получить бесконечным числом способов, и только один из них оставляет "+
				"руль ровным. Именно поэтому «сделали схождение, а руль кривой» — самая частая жалоба.")
	}
	step("Проверка",
		"Прокатите автомобиль вперёд-назад, дайте подвеске сесть и перемерьте всё заново. "+
			"Затяните контргайки рулевых тяг и повторите замер: затяжка часто сдвигает схождение.",
		"Регулировка без контрольного замера — это не регулировка, а надежда.")

	if spec == nil || !spec.Verified() {
		step("Перед выездом",
			"Сверьте полученные значения с руководством по ремонту вашего автомобиля. "+
				"Данные в программе для этой модели не подтверждены заводским документом.",
			"")
	}
	return steps
}

func hasAny(m map[string]ParamReport, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func adviceFor(p ParamReport) string {
	if p.Spec == nil {
		return ""
	}
	dev := p.Deviation
	dir := "уменьшить"
	if dev < 0 {
		dir = "увеличить"
	}
	s := fmt.Sprintf("Нужно %s на %s (до диапазона %s … %s).",
		dir, dev.FormatMagnitude(),
		p.Spec.Min.FormatDegMin(), p.Spec.Max.FormatDegMin())
	switch {
	case p.Method != "":
		s += " Регулировка: " + p.Method
	case !p.Adjustable:
		s += " Штатной регулировки этого угла нет — потребуется ремонт, замена деформированной детали " +
			"или установка регулировочного комплекта."
	}
	return s
}

func describeConditions(c Conditions) string {
	var parts []string
	for _, v := range []string{c.Load, c.TyrePressure, c.FuelState, c.RideHeightNote, c.SettleProcedure, c.AdditionalChecks} {
		if v != "" {
			parts = append(parts, v)
		}
	}
	if c.RideHeightMM > 0 {
		parts = append(parts, fmt.Sprintf("контрольная высота кузова %.0f мм", c.RideHeightMM))
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "; "
		}
		out += p
	}
	return out
}

func axleOf(s *Spec, front bool) *AxleSpec {
	if s == nil {
		return nil
	}
	if front {
		return &s.Front
	}
	return &s.Rear
}

func rangeOf(a *AxleSpec, f func(*AxleSpec) *align.Range) *align.Range {
	if a == nil {
		return nil
	}
	return f(a)
}

func adjOf(a *AxleSpec, f func(*AxleSpec) bool) bool {
	if a == nil {
		return false
	}
	return f(a)
}

func methodOf(a *AxleSpec, f func(*AxleSpec) string) string {
	if a == nil {
		return ""
	}
	return f(a)
}

func crossRange(a *AxleSpec, camber bool) *align.Range {
	if a == nil {
		return nil
	}
	var lim *align.Angle
	if camber {
		lim = a.MaxCrossCamber
	} else {
		lim = a.MaxCrossCaster
	}
	if lim == nil {
		return nil
	}
	r := align.RangeMinMax(-*lim, *lim)
	return &r
}

func toeMMOf(s *Spec, front bool) *MMRange {
	if front {
		return s.FrontTotalToeMM
	}
	return s.RearTotalToeMM
}

// rimFor picks the reference diameter for millimetre toe: the specification's,
// falling back to what the measurement itself recorded.
func rimFor(s *Spec, res align.Result, p align.Position) float64 {
	if s != nil && s.RimDiameterMM() > 0 {
		return s.RimDiameterMM()
	}
	if w, ok := res.Wheels[p.String()]; ok && w.ToeThrust != 0 && w.ToeMM != 0 {
		return w.ToeMM / math.Tan(w.ToeThrust.Rad())
	}
	return align.Inches(15)
}
