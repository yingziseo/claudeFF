//go:build darwin

package main

import "os/exec"

func desktopPlatform() string             { return "macOS" }
func openDataDirectory(path string) error { return exec.Command("open", path).Start() }
