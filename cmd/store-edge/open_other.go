//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openLocalUI() error {
	url := "http://127.0.0.1:17624/"
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	if err := exec.Command("xdg-open", url).Start(); err != nil {
		return fmt.Errorf("open %s in your browser: %w", url, err)
	}
	return nil
}
