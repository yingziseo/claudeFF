//go:build !windows

package sandbox

import (
	"errors"
	"os/exec"
)

func hide(c *exec.Cmd) {}
func Enable() error    { return errors.New("需要在 Windows 上启用 WSL2") }
