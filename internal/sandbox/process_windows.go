//go:build windows

package sandbox

import (
	"os/exec"
	"syscall"
)

func hide(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func Enable() error {
	return exec.Command("powershell.exe", "-NoProfile", "-Command", `Start-Process wsl.exe -Verb RunAs -ArgumentList '--install','--no-distribution'`).Start()
}
