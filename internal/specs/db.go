package specs

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

//go:embed data/*.json
var builtin embed.FS

// DB is a loaded set of vehicle specifications.
type DB struct {
	mu    sync.RWMutex
	specs []Spec
	byID  map[string]int

	// userDir is where the owner's own entries live — figures typed in from
	// their own manual. Empty when the program runs without a writable
	// profile directory, in which case saving is refused rather than lost.
	userDir string
	local   map[string]bool

	// LoadErrors records entries that were rejected during loading. They are
	// surfaced in the UI rather than swallowed: a spec file that fails
	// validation is a spec somebody expected to be able to use.
	LoadErrors []string
}

// Load reads the built-in database, plus any additional directories of JSON
// files. Extra directories let a workshop or a club keep its own verified data
// without waiting for it to be merged upstream.
func Load(extraDirs ...fs.FS) (*DB, error) {
	db := &DB{byID: map[string]int{}, local: map[string]bool{}}
	if err := db.addFS(builtin, i18n.T("встроенная база"), false); err != nil {
		return nil, err
	}
	for i, f := range extraDirs {
		if err := db.addFS(f, i18n.F("дополнительный каталог %d", i+1), false); err != nil {
			return nil, err
		}
	}
	db.finish()
	return db, nil
}

// LoadWithUser is Load plus the owner's own entries from userDir, which is
// created if missing. Entries saved there with Save take precedence over a
// built-in entry of the same id: they are what this owner found in their own
// manual.
func LoadWithUser(userDir string, extraDirs ...fs.FS) (*DB, error) {
	db, err := Load(extraDirs...)
	if err != nil {
		return nil, err
	}
	if userDir == "" {
		return db, nil
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, fmt.Errorf(i18n.T("не удалось создать каталог пользовательских данных: %w"), err)
	}
	db.userDir = userDir
	if err := db.addFS(os.DirFS(userDir), i18n.T("ваши данные"), true); err != nil {
		return nil, err
	}
	db.finish()
	return db, nil
}

// finish sorts, reindexes and checks the references between entries.
func (d *DB) finish() {
	d.sort()
	for i := range d.specs {
		s := &d.specs[i]
		if s.GuidanceID == "" {
			continue
		}
		g, ok := d.byID[s.GuidanceID]
		if !ok || d.specs[g].Source.Kind != SourceClassGuidance {
			d.LoadErrors = append(d.LoadErrors, i18n.F(
				"%s: guidance_id %q не указывает на ориентир по классу", s.ID, s.GuidanceID))
			s.GuidanceID = ""
		}
	}
}

func (d *DB) addFS(f fs.FS, label string, local bool) error {
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
				// Keep the better-sourced of two entries with the same id —
				// except that the owner's own entry always wins: it is what
				// they read in their own manual.
				if local || s.Source.Kind.Trust() > d.specs[i].Source.Kind.Trust() {
					d.specs[i] = s
					d.local[s.ID] = local
				}
				continue
			}
			d.byID[s.ID] = len(d.specs)
			d.specs = append(d.specs, s)
			if local {
				d.local[s.ID] = true
			}
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

// IsSourceURL reports whether u is the source address of an entry that ships
// with the program (or comes from an added directory) — never of the owner's
// own entries. Those addresses, and only those, the interface may open in the
// browser besides its fixed list, so the person can check a figure where it
// came from.
func (d *DB) IsSourceURL(u string) bool {
	if u == "" {
		return false
	}
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "https" && p.Scheme != "http") || p.User != nil || p.Host == "" {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, s := range d.specs {
		if s.Source.URL == u && !d.local[s.ID] {
			return true
		}
	}
	return false
}

// IsLocal reports whether an entry is the owner's own.
func (d *DB) IsLocal(id string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.local[id]
}

// Limits returns the entry whose tolerances apply to s: s itself when it has
// figures, otherwise the class guidance it names. ok is false when there is
// nothing to compare against at all.
func (d *DB) Limits(s Spec) (Spec, bool) {
	if s.HasFigures() {
		return s, true
	}
	if s.GuidanceID == "" {
		return Spec{}, false
	}
	return d.Get(s.GuidanceID)
}

// ErrNoUserDir is returned by Save when the program has nowhere to keep the
// owner's entries.
var ErrNoUserDir = i18n.Err("не задан каталог для пользовательских данных")

// Save validates an entry, writes it to the owner's directory and makes it
// available immediately. The file has the same format as the built-in data,
// so it can be sent to the project unchanged.
func (d *DB) Save(s Spec) error {
	if d.userDir == "" {
		return ErrNoUserDir
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Source.Kind == SourceClassGuidance {
		return errors.New(i18n.T("ориентир по классу нельзя сохранить как данные автомобиля"))
	}
	s.Resolve()

	b, err := json.MarshalIndent(specFile{Specs: []Spec{s}}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	name := filepath.Join(d.userDir, safeFileName(s.ID)+".json")
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, name); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if i, ok := d.byID[s.ID]; ok {
		d.specs[i] = s
	} else {
		d.specs = append(d.specs, s)
	}
	d.local[s.ID] = true
	d.sort()
	return nil
}

// Delete removes one of the owner's own entries. Built-in entries cannot be
// deleted; an own entry that shadowed a built-in one uncovers it again on the
// next start.
func (d *DB) Delete(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.local[id] {
		return errors.New(i18n.T("удалить можно только свои записи"))
	}
	err := os.Remove(filepath.Join(d.userDir, safeFileName(id)+".json"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	i := d.byID[id]
	d.specs = append(d.specs[:i], d.specs[i+1:]...)
	delete(d.local, id)
	d.sort()
	return nil
}

// safeFileName keeps an id usable as a file name on every OS.
func safeFileName(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_',
			r >= 'а' && r <= 'я', r >= 'А' && r <= 'Я', r == 'ё', r == 'Ё':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "entry"
	}
	return b.String()
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
	return d.specs[i].Localized(), true
}

// All returns every specification, ordered by make, model and year.
func (d *DB) All() []Spec {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Spec, len(d.specs))
	for i, s := range d.specs {
		out[i] = s.Localized()
	}
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
	Text  string
	Make  string
	Year  int
	Class Class
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
		if q.Class != "" && s.Class != "" && s.Class != q.Class {
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
			fields := append([]string{s.Make, s.Model, s.Trim, s.Notes, s.ID}, s.Tags...)
			if s.EN != nil {
				fields = append(fields, s.EN.Make, s.EN.Model, s.EN.Trim)
			}
			if s.Class != "" {
				fields = append(fields, s.Class.Label())
			}
			hay := tokenize(strings.Join(fields, " "))
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
		// Prefer better-sourced data when scores are otherwise equal, and the
		// owner's own entries above everything: they are what this person
		// found in their own manual.
		score += s.Source.Kind.Trust()
		if d.local[s.ID] {
			score += 5
		}
		out = append(out, Match{Spec: s.Localized(), Score: score})
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
	s = translitReplacer.Replace(s)
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r >= 'а' && r <= 'я' || r == 'ё')
	})
}

// translitReplacer applies translit longest key first. The order matters as
// soon as one key contains another ("газель" and "газ"): ranging over the map
// directly picks an order at random, and the query and the index could then be
// transliterated differently and stop matching each other.
var translitReplacer = func() *strings.Replacer {
	keys := make([]string, 0, len(translit))
	for k := range translit {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	pairs := make([]string, 0, 2*len(keys))
	for _, k := range keys {
		pairs = append(pairs, k, translit[k])
	}
	return strings.NewReplacer(pairs...)
}()

// translit covers the marques and model names Russian speakers routinely type
// in either alphabet, so that "Ваз", "VAZ", "Лада", "Lada", "Самара" and
// "Samara" all find the same cars. Both the query and the indexed text are
// passed through it, so a single entry here makes the pair interchangeable in
// both directions.
var translit = map[string]string{
	"ваз": "vaz", "лада": "lada", "газ": "gaz", "уаз": "uaz", "заз": "zaz",
	"москвич": "moskvich", "иж": "izh", "камаз": "kamaz", "зил": "zil", "паз": "paz",
	"газель": "gazel", "буханка": "bukhanka",

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
