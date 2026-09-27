// Package i18n translates the program's messages.
//
// The source language is Russian, and the Russian text itself is the key: the
// code reads as it always did, i18n.T("Развал") instead of a symbolic key that
// sends every reader to a table. English lives in tables next to this file,
// one per package; a test parses the source tree and fails if any string passed
// to T or F has no translation, or if the two versions disagree on their
// formatting verbs.
//
// The language is one setting for the whole program. It serves a single person
// at a single computer, and a process-wide setting is what keeps a warning
// produced deep inside the maths in the same language as the screen showing
// it, without threading a language argument through every function.
package i18n

import (
	"fmt"
	"sync/atomic"
)

// Lang is an interface language.
type Lang string

const (
	RU Lang = "ru"
	EN Lang = "en"
)

// Valid reports whether l is a supported language.
func Valid(l Lang) bool { return l == RU || l == EN }

var current atomic.Value

func init() { current.Store(RU) }

// Set changes the language of everything produced from now on.
func Set(l Lang) {
	if Valid(l) {
		current.Store(l)
	}
}

// Current returns the language in use.
func Current() Lang { return current.Load().(Lang) }

// T translates a Russian string into the current language. A string with no
// translation is returned as it is — Russian is always better than nothing.
func T(ru string) string {
	if Current() == EN {
		if s, ok := en[ru]; ok {
			return s
		}
	}
	return ru
}

// F is fmt.Sprintf with a translated format.
func F(format string, args ...any) string { return fmt.Sprintf(T(format), args...) }

// N marks a string for translation without translating it: the string is
// stored, and translated with T when shown. A source name kept by the live hub
// is the example — it must follow the language when the language changes.
func N(ru string) string { return ru }

// Err returns an error whose message is translated each time it is read, so a
// package-level error variable speaks the current language.
func Err(ru string) error { return msgErr(ru) }

type msgErr string

func (e msgErr) Error() string { return T(string(e)) }

// Has reports whether an English translation exists, for tests.
func Has(ru string) bool {
	_, ok := en[ru]
	return ok
}

// en is the English table, assembled from the per-package parts.
var en = map[string]string{}

func add(m map[string]string) {
	for k, v := range m {
		if _, dup := en[k]; dup && en[k] != v {
			panic("i18n: two different translations for " + k)
		}
		en[k] = v
	}
}
