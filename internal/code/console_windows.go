//go:build windows

package code

import (
	"context"
	"errors"
	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
	"os/exec"
	"sync"
	"syscall"
)

type winConsole struct {
	*conpty.ConPty
	once sync.Once
}

func (w *winConsole) Wait(ctx context.Context) error {
	n, e := w.ConPty.Wait(ctx)
	if e == nil && n != 0 {
		return errors.New("Code exited")
	}
	return e
}
func (w *winConsole) Close() error {
	w.once.Do(func() {
		p, e := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(w.Pid()))
		if e == nil {
			windows.TerminateProcess(p, 0)
			windows.CloseHandle(p)
		}
		w.ConPty.Close()
	})
	return nil
}
func startConsole(exe, dir string, env []string) (console, error) {
	p, e := conpty.Start(syscall.EscapeArg(exe), conpty.ConPtyDimensions(100, 30), conpty.ConPtyWorkDir(dir), conpty.ConPtyEnv(env))
	if e != nil {
		return nil, e
	}
	return &winConsole{ConPty: p}, nil
}
func hide(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
