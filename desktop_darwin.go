//go:build darwin

package main

import (
	"os/exec"
	"runtime"
)

func desktopPlatform() string {
	if runtime.GOARCH == "arm64" {
		return "macOS · Apple Silicon"
	}
	return "macOS · Intel"
}
func openDataDirectory(path string) error { return exec.Command("open", path).Start() }
