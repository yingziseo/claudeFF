//go:build windows

package main

import "os/exec"

func desktopPlatform() string             { return "Windows" }
func openDataDirectory(path string) error { return exec.Command("explorer.exe", path).Start() }
