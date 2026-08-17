//go:build windows

package main

import "os/exec"

func openLocalUI() error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://127.0.0.1:17624/").Start()
}
