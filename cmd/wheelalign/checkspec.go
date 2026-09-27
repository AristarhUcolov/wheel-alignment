package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
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
		return errors.New(i18n.T("укажите файл с данными: wheelalign check-spec <файл.json>"))
	}
	path := fs.Arg(0)

	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf(i18n.T("не удалось прочитать файл: %w"), err)
	}
	var file struct {
		Specs []specs.Spec `json:"specs"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		return fmt.Errorf(i18n.T("файл не разобран как JSON: %w"), err)
	}
	if len(file.Specs) == 0 {
		return errors.New(i18n.T(`в файле нет записей — ожидается объект вида {"specs": [ ... ]}`))
	}

	fmt.Print("\n  ", i18n.F("Проверка %s, записей в файле: %d", path, len(file.Specs)), "\n\n")

	bad, warned := 0, 0
	for i, s := range file.Specs {
		title := s.Title()
		if s.ID == "" {
			title = i18n.F("запись %d (без id)", i+1)
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
		fmt.Print("      ", i18n.F("источник: %s", s.Source.Kind.Label()))
		if !s.Verified() {
			fmt.Print("  ", i18n.T("— будет показано с предупреждением"))
		}
		fmt.Println()

		for _, w := range warnings {
			fmt.Printf("      · %s\n", wrap(w, 70, "        "))
		}
	}

	fmt.Println()
	switch {
	case bad > 0:
		fmt.Print("  ", i18n.F("Не пройдено: %d из %d. Такие записи не загрузятся.", bad, len(file.Specs)), "\n\n")
		return errors.New(i18n.T("в файле есть ошибки"))
	case warned > 0:
		fmt.Print("  ", i18n.F("Загрузятся все записи (%d). С замечаниями: %d — проверьте их выше:\n"+
			"  это возможные, но необычные значения, и чаще всего так выглядит опечатка.", len(file.Specs), warned), "\n\n")
	default:
		fmt.Print("  ", i18n.F("Проверено записей: %d — все в порядке.", len(file.Specs)), "\n\n")
	}
	return nil
}
