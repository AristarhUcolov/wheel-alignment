package specs

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

//go:embed data/*.json
var builtin embed.FS

// DB is a loaded set of vehicle specifications.
type DB struct {
	mu    sync.RWMutex
	specs []Spec
	byID  map[string]int

	// LoadErrors records entries that were rejected during loading. They are
	// surfaced in the UI rather than swallowed: a spec file that fails
	// validation is a spec somebody expected to be able to use.
	LoadErrors []string
}

// Load reads the built-in database, plus any additional directories of JSON
// files. Extra directories let a workshop or a club keep its own verified data
// without waiting for it to be merged upstream.
func Load(extraDirs ...fs.FS) (*DB, error) {
	db := &DB{byID: map[string]int{}}
	if err := db.addFS(builtin, "встроенная база"); err != nil {
		return nil, err
	}
	for i, f := range extraDirs {
		if err := db.addFS(f, fmt.Sprintf("дополнительный каталог %d", i+1)); err != nil {
			return nil, err
		}
	}
	db.sort()
	return db, nil
}

func (d *DB) addFS(f fs.FS, label string) error {
	return fs.WalkDir(f, ".", func(path string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		b, err := fs.ReadFile(f, path)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", label, path, err)
		}
		var file specFile
		if err := json.Unmarshal(b, &file); err != nil {
			return fmt.Errorf("%s: %s: %w", label, path, err)
		}
		for _, s := range file.Specs {
			if err := s.Validate(); err != nil {
				d.LoadErrors = append(d.LoadErrors, fmt.Sprintf("%s: %s: %v", label, path, err))
				continue
			}
			s.Resolve()
			if i, dup := d.byID[s.ID]; dup {
				// Keep the better-sourced of two entries with the same id.
				if s.Source.Kind.Trust() > d.specs[i].Source.Kind.Trust() {
					d.specs[i] = s
				}
				continue
			}
			d.byID[s.ID] = len(d.specs)
			d.specs = append(d.specs, s)
		}
		return nil
	})
}

type specFile struct {
	Specs []Spec `json:"specs"`
}

func (d *DB) sort() {
	sort.SliceStable(d.specs, func(i, j int) bool {
		a, b := d.specs[i], d.specs[j]
		if a.Make != b.Make {
			return a.Make < b.Make
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		return a.YearFrom < b.YearFrom
	})
	d.byID = make(map[string]int, len(d.specs))
	for i, s := range d.specs {
		d.byID[s.ID] = i
	}
}

// Count is the number of specifications loaded.
func (d *DB) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.specs)
}

// Get returns a specification by id.
func (d *DB) Get(id string) (Spec, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	i, ok := d.byID[id]
	if !ok {
		return Spec{}, false
	}
	return d.specs[i], true
}

// All returns every specification, ordered by make, model and year.
func (d *DB) All() []Spec {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Spec, len(d.specs))
	copy(out, d.specs)
	return out
}

// Makes lists the distinct manufacturers present.
func (d *DB) Makes() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, s := range d.specs {
		if s.Source.Kind == SourceClassGuidance {
			continue
		}
		if !seen[s.Make] {
			seen[s.Make] = true
			out = append(out, s.Make)
		}
	}
	sort.Strings(out)
	return out
}

// Query is a search over the database.
type Query struct {
	Text string
	Make string
	Year int
	// IncludeGuidance allows the generic class-based entries into the results.
	// Off by default so a search for a real car does not surface them as if
	// they were that car's data.
	IncludeGuidance bool
}

// Match is a search hit with its relevance.
type Match struct {
	Spec  Spec
	Score int
}

// Search finds specifications matching a query, best first.
//
// Matching is deliberately forgiving about how people actually type car names:
// Latin and Cyrillic, with and without hyphens, model before make. Somebody
// looking for their car should not have to guess the database's spelling.
func (d *DB) Search(q Query) []Match {
	d.mu.RLock()
	defer d.mu.RUnlock()

	terms := tokenize(q.Text)
	var out []Match

	for _, s := range d.specs {
		if s.Source.Kind == SourceClassGuidance && !q.IncludeGuidance {
			continue
		}
		if q.Make != "" && !strings.EqualFold(q.Make, s.Make) {
			continue
		}
		if q.Year != 0 {
			if q.Year < s.YearFrom {
				continue
			}
			if s.YearTo != 0 && q.Year > s.YearTo {
				continue
			}
		}
		score := 0
		if len(terms) > 0 {
			hay := tokenize(strings.Join(append([]string{
				s.Make, s.Model, s.Trim, s.Notes, s.ID,
			}, s.Tags...), " "))
			matched := 0
			for _, t := range terms {
				for _, h := range hay {
					if h == t {
						score += 10
						matched++
						break
					}
					if strings.HasPrefix(h, t) {
						score += 5
						matched++
						break
					}
				}
			}
			if matched < len(terms) {
				continue // every term must land somewhere
			}
		}
		// Prefer better-sourced data when scores are otherwise equal.
		score += s.Source.Kind.Trust()
		out = append(out, Match{Spec: s, Score: score})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Spec.Title() < out[j].Spec.Title()
	})
	return out
}

// tokenize lowercases, transliterates the Cyrillic forms of common marques,
// and splits on anything that is not a letter or a digit.
func tokenize(s string) []string {
	s = strings.ToLower(strings.TrimSpace(s))
	for cyr, lat := range translit {
		s = strings.ReplaceAll(s, cyr, lat)
	}
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r >= 'а' && r <= 'я' || r == 'ё')
	})
}

// translit covers the marques and model names Russian speakers routinely type
// in either alphabet, so that "Ваз", "VAZ", "Лада", "Lada", "Самара" and
// "Samara" all find the same cars. Both the query and the indexed text are
// passed through it, so a single entry here makes the pair interchangeable in
// both directions.
var translit = map[string]string{
	"ваз": "vaz", "лада": "lada", "газ": "gaz", "уаз": "uaz", "заз": "zaz",
	"москвич": "moskvich", "иж": "izh", "камаз": "kamaz",

	// Model names, which people type in Latin about as often as in Cyrillic.
	"самара": "samara", "спутник": "sputnik", "нива": "niva", "волга": "volga",
	"ока": "oka", "приора": "priora", "калина": "kalina", "гранта": "granta",
	"веста": "vesta", "ларгус": "largus", "патриот": "patriot", "хантер": "hunter",

	"тойота": "toyota", "мерседес": "mercedes", "бмв": "bmw", "ауди": "audi",
	"фольксваген": "volkswagen", "опель": "opel", "форд": "ford", "рено": "renault",
	"пежо": "peugeot", "ситроен": "citroen", "ниссан": "nissan", "хонда": "honda",
	"мазда": "mazda", "митсубиси": "mitsubishi", "мицубиси": "mitsubishi",
	"субару": "subaru", "хёндай": "hyundai", "хендай": "hyundai", "киа": "kia",
	"шкода": "skoda", "фиат": "fiat", "вольво": "volvo", "шевроле": "chevrolet",
	"дэу": "daewoo", "сузуки": "suzuki", "лексус": "lexus", "инфинити": "infiniti",
}
