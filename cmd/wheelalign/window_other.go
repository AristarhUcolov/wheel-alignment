//go:build !windows

package main

import "errors"

var errNoWindow = errors.New("отдельное окно есть только в сборке для Windows")

// runWindow has no native implementation outside Windows yet; the caller opens
// the browser instead, which works everywhere.
func runWindow(url, data string) error { return errNoWindow }

func notify(msg string) {}
