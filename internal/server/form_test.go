package server

import (
	"mime/multipart"
	"testing"
)

// Numbers arrive as people type them: a decimal comma, a typographic minus.
func TestFormFloatTolerant(t *testing.T) {
	for in, want := range map[string]float64{
		"0.5": 0.5, "0,5": 0.5, "-0,5": -0.5, "−0.5": -0.5, "–1,25": -1.25, " 22,5 ": 22.5, "1 400": 1400,
	} {
		f := &multipart.Form{Value: map[string][]string{"x": {in}}}
		if got := formFloat(f, "x"); got != want {
			t.Errorf("%q: %v, want %v", in, got, want)
		}
		if got, ok := formFloatOK(f, "x"); !ok || got != want {
			t.Errorf("%q: formFloatOK %v %v", in, got, ok)
		}
	}
	if _, ok := formFloatOK(&multipart.Form{Value: map[string][]string{"x": {"abc"}}}, "x"); ok {
		t.Error("abc parsed as a number")
	}
	if got := formFloatQ("−3,5"); got != -3.5 {
		t.Errorf("formFloatQ: %v", got)
	}
}
