package server

import (
	"mime/multipart"
	"strconv"
	"strings"
)

// Small helpers for reading multipart form fields, tolerant of the field simply
// being absent — the optical endpoints have several optional inputs and a
// missing one should mean "use the default", never an error.

func formValue(f *multipart.Form, key string) string {
	if f == nil {
		return ""
	}
	if vs := f.Value[key]; len(vs) > 0 {
		return strings.TrimSpace(vs[0])
	}
	return ""
}

func formInt(f *multipart.Form, key string) int {
	n, _ := strconv.Atoi(formValue(f, key))
	return n
}

func formFloat(f *multipart.Form, key string) float64 {
	v, _ := strconv.ParseFloat(formValue(f, key), 64)
	return v
}

// formFloatOK distinguishes "absent" from "present and zero", which matters for
// a gravity component that legitimately can be zero.
func formFloatOK(f *multipart.Form, key string) (float64, bool) {
	s := formValue(f, key)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func firstFile(f *multipart.Form, key string) *multipart.FileHeader {
	if f == nil {
		return nil
	}
	if fhs := f.File[key]; len(fhs) > 0 {
		return fhs[0]
	}
	return nil
}
