package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

// runCheckSpec validates a contributed vehicle data file before it is proposed.
//
// It runs exactly the checks the program runs when it loads its database, so a
// file that passes here will load; and it adds the plausibility warnings, which
// catch the mistakes that structure alone cannot — a dropped sign, minutes typed
// as decimal degrees, front and rear swapped.
func runCheckSpec(args []string) error {
	fs := flag.NewFlagSet("check-spec", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("укажите файл с данными: wheelalign check-spec <файл.json>")
	}
	path := fs.Arg(0)

	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	var file struct {
		Specs []specs.Spec `json:"specs"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		return fmt.Errorf("файл не разобран как JSON: %w", err)
	}
	if len(file.Specs) == 0 {
		return errors.New(`в файле нет записей — ожидается объект вида {"specs": [ ... ]}`)
	}

	fmt.Printf("\n  Проверка %s: записей %d\n\n", path, len(file.Specs))

	bad, warned := 0, 0
	for i, s := range file.Specs {
		title := s.Title()
		if s.ID == "" {
			title = fmt.Sprintf("запись %d (без id)", i+1)
		}

		if err := s.Validate(); err != nil {
			bad++
			fmt.Printf("  ✗ %s\n", title)
			fmt.Printf("      %s\n", wrap(err.Error(), 70, "      "))
			continue
		}
		s.Resolve()

		warnings := specs.Sanity(s)
		mark := "✓"
		if len(warnings) > 0 {
			mark = "!"
			warned++
		}
		fmt.Printf("  %s %s\n", mark, title)
		fmt.Printf("      источник: %s", s.Source.Kind.RussianName())
		if !s.Verified() {
			fmt.Printf("  — будет показано с предупреждением")
		}
		fmt.Println()

		for _, w := range warnings {
			fmt.Printf("      · %s\n", wrap(w, 70, "        "))
		}
	}

	fmt.Println()
	switch {
	case bad > 0:
		fmt.Printf("  Не пройдено: %d из %d. Такие записи не загрузятся.\n\n", bad, len(file.Specs))
		return fmt.Errorf("в файле есть ошибки")
	case warned > 0:
		fmt.Printf("  Все %d записей загрузятся. У %d есть замечания — проверьте их выше:\n"+
			"  это возможные, но необычные значения, и чаще всего так выглядит опечатка.\n\n",
			len(file.Specs), warned)
	default:
		fmt.Printf("  Все %d записей в порядке.\n\n", len(file.Specs))
	}
	return nil
}
