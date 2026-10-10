//go:build !windows

package code

import (
	"context"
	"github.com/creack/pty"
	"os"
	"os/exec"
	"syscall"
)

type unixConsole struct {
	*os.File
	cmd *exec.Cmd
}

func (t *unixConsole) Resize(cols, rows int) error {
	return pty.Setsize(t.File, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}
func (t *unixConsole) Wait(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		if t.cmd.Process != nil {
			syscall.Kill(-t.cmd.Process.Pid, syscall.SIGTERM)
		}
	}()
	return t.cmd.Wait()
}
func (t *unixConsole) Close() error {
	if t.cmd.Process != nil {
		syscall.Kill(-t.cmd.Process.Pid, syscall.SIGTERM)
	}
	return t.File.Close()
}
func startConsole(exe, dir string, env []string) (console, error) {
	c := exec.Command(exe)
	c.Dir = dir
	c.Env = env
	f, e := pty.StartWithSize(c, &pty.Winsize{Cols: 100, Rows: 30})
	if e != nil {
		return nil, e
	}
	return &unixConsole{f, c}, nil
}
func hide(c *exec.Cmd) {}
