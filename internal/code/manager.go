package code

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yingziseo/claudeFF/internal/model"
	"github.com/yingziseo/claudeFF/internal/proxy"
)

type console interface {
	io.ReadWriteCloser
	Resize(int, int) error
	Wait(context.Context) error
}
type Manager struct {
	mu            sync.Mutex
	status        model.Status
	terminal      console
	cancel        context.CancelFunc
	root, project string
	emit          func(string, string)
}

func New(root string, emit func(string, string)) *Manager {
	home, _ := os.UserHomeDir()
	m := &Manager{root: root, project: home, emit: emit, status: model.Status{Phase: "idle", Message: "尚未安装"}}
	if data, e := os.ReadFile(filepath.Join(root, "code-project")); e == nil {
		if v, e := os.Stat(string(data)); e == nil && v.IsDir() {
			m.project = string(data)
		}
	}
	if m.Executable() != "" {
		m.status = model.Status{Phase: "ready", Message: "已检测到官方 Claude Code"}
	}
	return m
}
func (m *Manager) State() model.Status { m.mu.Lock(); defer m.mu.Unlock(); return m.status }
func (m *Manager) set(phase, message string) {
	m.mu.Lock()
	m.status.Phase = phase
	m.status.Message = message
	m.mu.Unlock()
}
func (m *Manager) Fail(e error)    { m.set("failed", e.Error()) }
func (m *Manager) Installed() bool { return m.Executable() != "" }
func (m *Manager) Executable() string {
	home, _ := os.UserHomeDir()
	name := "claude"
	if platformOS() == "windows" {
		name += ".exe"
	}
	own := filepath.Join(home, ".local", "bin", name)
	if s, e := os.Stat(own); e == nil && !s.IsDir() {
		return own
	}
	if p, e := exec.LookPath(name); e == nil {
		return p
	}
	return ""
}
func managedEnv(relay string) []string {
	env := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		switch strings.ToUpper(k) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY":
			continue
		}
		env = append(env, v)
	}
	return append(env, "HTTP_PROXY="+relay, "HTTPS_PROXY="+relay, "http_proxy="+relay, "https_proxy="+relay, "NO_PROXY=localhost,127.0.0.1,::1", "no_proxy=localhost,127.0.0.1,::1", "TERM=xterm-256color")
}
func (m *Manager) Install(ctx context.Context, client *http.Client, relay string) error {
	m.set("busy", "正在下载官方 Claude Code")
	defer func() {
		if ctx.Err() != nil {
			if m.Installed() {
				m.RestoreReady()
			} else {
				m.set("idle", "安装已取消，可重试")
			}
		}
	}()
	staged, err := fetchOfficial(ctx, client, m.root)
	if err != nil {
		m.Fail(err)
		return err
	}
	defer os.Remove(staged)
	m.set("busy", "正在运行官方安装器")
	c := exec.CommandContext(ctx, staged, "install", "stable")
	c.Env = managedEnv(relay)
	hide(c)
	// The official binary installs into the user's normal .local/bin and manages its own updates.
	if _, err = c.CombinedOutput(); err != nil {
		e := errors.New("官方安装未完成，请检查网络后重试")
		m.Fail(e)
		return e
	}
	exe := m.Executable()
	if exe == "" {
		e := errors.New("官方安装结束但未找到 claude，请重试安装")
		m.Fail(e)
		return e
	}
	q := exec.CommandContext(ctx, exe, "--version")
	q.Env = managedEnv(relay)
	hide(q)
	out, err := q.Output()
	if err != nil {
		e := errors.New("官方 Code 版本验证失败")
		m.Fail(e)
		return e
	}
	v := strings.TrimSpace(string(out))
	if len(v) > 120 || !strings.Contains(v, "Claude Code") {
		return errors.New("官方 Code 版本输出异常")
	}
	m.mu.Lock()
	m.status = model.Status{Phase: "ready", Message: "已安装，启动后完成官方登录", Version: v}
	m.mu.Unlock()
	return nil
}
func (m *Manager) Start(parent context.Context, relay string, expected *model.Observation) error {
	if expected == nil {
		return errors.New("请先检测全局代理")
	}
	exe := m.Executable()
	if exe == "" {
		return errors.New("请先安装官方 Claude Code")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.terminal != nil {
		return errors.New("终端已在运行")
	}
	// Verify the actual relay being passed to the native process, independently of saved settings.
	check, done := context.WithTimeout(parent, 30*time.Second)
	defer done()
	o, e := proxy.DetectAcross(check, proxy.ClientThrough(relay), proxy.DetectionEndpoints(), expected.Revision)
	if e != nil {
		return errors.New("Code 代理出口暂时无法确认，请重试检测")
	}
	// A gateway may rotate its exit between requests. Keep the actual observation
	// instead of rejecting a working route because an earlier probe used another IP.
	ctx, cancel := context.WithCancel(parent)
	term, e := startConsole(exe, m.project, managedEnv(relay))
	if e != nil {
		cancel()
		return errors.New("本机终端启动失败，请检查系统权限")
	}
	m.terminal, m.cancel = term, cancel
	m.status = model.Status{Phase: "running", Message: "官方 Code · 共用代理", Version: m.status.Version, Observation: o}
	go func() {
		b := make([]byte, 8192)
		for {
			n, e := term.Read(b)
			if n > 0 && m.emit != nil {
				m.emit("terminal:data", string(b[:n]))
			}
			if e != nil {
				return
			}
		}
	}()
	go func() {
		e := term.Wait(ctx)
		m.mu.Lock()
		if m.terminal == term {
			m.terminal = nil
			m.cancel = nil
			m.status.Phase = "ready"
			m.status.Message = "终端已结束"
			if e != nil && ctx.Err() == nil {
				m.status.Message = "终端异常结束，可重新启动"
			}
		}
		m.mu.Unlock()
		cancel()
		term.Close()
	}()
	return nil
}
func (m *Manager) Stop() {
	m.mu.Lock()
	t, c := m.terminal, m.cancel
	m.terminal = nil
	m.cancel = nil
	if m.Installed() {
		m.status.Phase = "ready"
		m.status.Message = "终端已停止"
	} else {
		m.status.Phase = "idle"
		m.status.Message = "尚未安装"
	}
	m.mu.Unlock()
	if c != nil {
		c()
	}
	if t != nil {
		t.Close()
	}
}
func (m *Manager) Write(s string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.terminal == nil {
		return errors.New("终端未启动")
	}
	_, e := m.terminal.Write([]byte(s))
	return e
}
func (m *Manager) Resize(cols, rows int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cols < 20 || cols > 500 || rows < 5 || rows > 200 {
		return nil
	}
	if m.terminal != nil {
		return m.terminal.Resize(cols, rows)
	}
	return nil
}
func (m *Manager) ChooseProject(path string) error {
	v, e := os.Stat(path)
	if e != nil || !v.IsDir() {
		return errors.New("请选择本机项目目录")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.terminal != nil {
		return errors.New("请先停止终端再选择项目")
	}
	if e = os.WriteFile(filepath.Join(m.root, "code-project"), []byte(path), 0600); e != nil {
		return e
	}
	m.project = path
	return nil
}
func (m *Manager) Project() string { m.mu.Lock(); defer m.mu.Unlock(); return m.project }
func (m *Manager) RestoreReady() {
	if m.Installed() {
		m.set("ready", "官方 Code 已就绪")
	}
}
