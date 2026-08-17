//go:build !windows

package main

import (
	"context"
	"errors"
)

func runWindowsService(name string, run func(context.Context, bool) error) error {
	return errors.New("Windows service mode is unavailable on this operating system")
}
