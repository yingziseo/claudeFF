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
	once    sync.Once
	process windows.Handle
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
		if w.process != 0 {
			windows.TerminateProcess(w.process, 0)
			windows.CloseHandle(w.process)
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
	handle, e := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(p.Pid()))
	if e != nil {
		p.Close()
		return nil, e
	}
	return &winConsole{ConPty: p, process: handle}, nil
}
func hide(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
