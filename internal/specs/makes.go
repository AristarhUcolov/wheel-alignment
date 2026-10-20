package specs

import (
	"sort"
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

// Region groups makes for browsing the database: somebody who does not know
// how their car's name is spelt can still find it by where it was built.
type Region string

const (
	RegionUSSR   Region = "ussr"
	RegionUSA    Region = "usa"
	RegionEurope Region = "europe"
	RegionAsia   Region = "asia"
	RegionOther  Region = "other"
)

// Regions in the order they are shown.
var Regions = []Region{RegionUSSR, RegionEurope, RegionUSA, RegionAsia, RegionOther}

// Label is the region's name in the current language.
func (r Region) Label() string {
	switch r {
	case RegionUSSR:
		return i18n.T("СССР, Россия и СНГ")
	case RegionUSA:
		return i18n.T("США")
	case RegionEurope:
		return i18n.T("Европа")
	case RegionAsia:
		return i18n.T("Азия")
	}
	return i18n.T("Другие")
}

// makeRegion is keyed by the make as written in the data files. A make that
// is missing here (an owner's own entry, say) is shown under "other"; the
// built-in ones are all listed — a test makes sure of it.
var makeRegion = map[string]Region{
	"ВАЗ (Lada)": RegionUSSR, "ГАЗ": RegionUSSR, "ЗИЛ": RegionUSSR, "КамАЗ": RegionUSSR,
	"ЛиАЗ": RegionUSSR, "МАЗ": RegionUSSR, "Москвич (АЗЛК)": RegionUSSR, "ПАЗ": RegionUSSR,
	"УАЗ": RegionUSSR, "Урал": RegionUSSR,

	"Chevrolet": RegionUSA, "Dodge": RegionUSA, "Ford": RegionUSA, "Jeep": RegionUSA, "Tesla": RegionUSA,

	"Audi": RegionEurope, "BMW": RegionEurope, "Citroën": RegionEurope, "DAF": RegionEurope,
	"Fiat": RegionEurope, "Ikarus": RegionEurope, "Land Rover": RegionEurope, "MAN": RegionEurope,
	"Mercedes-Benz": RegionEurope, "Opel": RegionEurope, "Peugeot": RegionEurope, "Renault": RegionEurope,
	"SEAT": RegionEurope, "Saab": RegionEurope, "Scania": RegionEurope, "Volkswagen": RegionEurope,
	"Volvo": RegionEurope, "Škoda": RegionEurope,

	"Daewoo": RegionAsia, "Honda": RegionAsia, "Hyundai": RegionAsia, "Kia": RegionAsia,
	"Mazda": RegionAsia, "Mitsubishi": RegionAsia, "Nissan": RegionAsia, "Subaru": RegionAsia,
	"Suzuki": RegionAsia, "Toyota": RegionAsia,
}

// RegionOf says where a make belongs.
func RegionOf(make string) Region {
	if r, ok := makeRegion[make]; ok {
		return r
	}
	return RegionOther
}

// MakeCount is one make with the number of entries it has.
type MakeCount struct {
	// Key is the make as stored — what Query.Make expects.
	Key string `json:"key"`
	// Name is the make in the current language.
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Region Region `json:"region"`
}

// MakeCounts lists every make with its entry count, by region, then by name.
func (d *DB) MakeCounts() []MakeCount {
	d.mu.RLock()
	idx := map[string]int{}
	var out []MakeCount
	for _, s := range d.specs {
		if s.Source.Kind == SourceClassGuidance {
			continue
		}
		i, ok := idx[s.Make]
		if !ok {
			i = len(out)
			idx[s.Make] = i
			out = append(out, MakeCount{Key: s.Make, Name: s.Localized().Make, Region: RegionOf(s.Make)})
		}
		out[i].Count++
	}
	d.mu.RUnlock()

	order := map[Region]int{}
	for i, r := range Regions {
		order[r] = i
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Region != out[j].Region {
			return order[out[i].Region] < order[out[j].Region]
		}
		return sortName(out[i].Name) < sortName(out[j].Name)
	})
	return out
}

// sortName puts Škoda among the S and Citroën among the C rather than after Z.
var sortName = strings.NewReplacer("Š", "S", "š", "s", "ë", "e", "Ё", "Е", "ё", "е").Replace
