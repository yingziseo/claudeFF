//go:build linux

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/creack/pty"
	"github.com/yingziseo/claudeFF/internal/model"
	"github.com/yingziseo/claudeFF/internal/proxy"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type config struct {
	Proxy    model.Proxy   `json:"proxy"`
	Profile  model.Profile `json:"profile"`
	Mode     string        `json:"mode"`
	Revision int           `json:"revision"`
}
type command struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

var writeMu sync.Mutex

func emit(kind string, data any) {
	writeMu.Lock()
	defer writeMu.Unlock()
	json.NewEncoder(os.Stdout).Encode(map[string]any{"type": kind, "data": data})
}
func main() {
	if e := run(); e != nil {
		emit("error", e.Error())
		os.Exit(1)
	}
}
func run() error {
	if os.Geteuid() != 0 {
		return errors.New("隔离代理必须由专用发行版受管 root 启动")
	}
	input := bufio.NewScanner(os.Stdin)
	input.Buffer(make([]byte, 8192), 1<<20)
	if !input.Scan() {
		return errors.New("缺少安全配置")
	}
	var cfg config
	if e := json.Unmarshal(input.Bytes(), &cfg); e != nil {
		return errors.New("配置无效")
	}
	if e := cfg.Proxy.Validate(); e != nil {
		return e
	}
	if e := cfg.Profile.Validate(); e != nil {
		return e
	}
	if cfg.Mode != "install" && cfg.Mode != "run" && cfg.Mode != "check" {
		return errors.New("不支持的隔离操作")
	}
	relay, e := proxy.Start(cfg.Proxy, "")
	if e != nil {
		return e
	}
	defer relay.Close()
	// Root relay only listens on loopback. All Code UID non-proxy traffic is denied by the firewall.
	env := append(os.Environ(), "CLAUDEFF_PROXY="+relay.URL(), "CLAUDEFF_LANG="+strings.ReplaceAll(cfg.Profile.Language, "-", "_")+".UTF-8", "CLAUDEFF_TZ="+cfg.Profile.Timezone)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var pt *os.File
	var ptMu sync.Mutex
	go func() {
		for input.Scan() {
			var c command
			if json.Unmarshal(input.Bytes(), &c) != nil {
				continue
			}
			ptMu.Lock()
			if pt != nil {
				switch c.Type {
				case "input":
					pt.Write([]byte(c.Data))
				case "resize":
					if c.Cols >= 20 && c.Cols <= 500 && c.Rows >= 5 && c.Rows <= 200 {
						pty.Setsize(pt, &pty.Winsize{Cols: c.Cols, Rows: c.Rows})
					}
				}
			}
			ptMu.Unlock()
			if c.Type == "stop" {
				cancel()
				return
			}
		}
		cancel()
	}()
	execute := func(name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		output, e := cmd.CombinedOutput()
		if e != nil {
			_ = output
			return fmt.Errorf("隔离步骤失败：%s；请检查 WSL2 条件与全局代理", name)
		}
		return nil
	}
	if cfg.Mode == "install" {
		emit("progress", "准备独立 WSL2 系统与隔离组件")
		if e = execute("/bin/bash", "/opt/claudeff/provision.sh"); e != nil {
			return e
		}
	}
	if e = execute("/bin/bash", "/opt/claudeff/firewall.sh", fmt.Sprint(relay.Port())); e != nil {
		return e
	}
	// Detect from Code's restricted UID, rather than trusting the root service's request.
	emit("progress", "正在同步语言、时区，并核验隔离环境出口")
	check := exec.CommandContext(ctx, "runuser", "-u", "claudeff", "--", "/bin/bash", "/opt/claudeff/launch.sh", "/usr/bin/curl", "--fail", "--silent", "--show-error", "--max-time", "20", proxy.DetectionURL)
	check.Env = env
	output, e := check.Output()
	if e != nil {
		return errors.New("隔离环境无法通过全局代理访问网络，已禁止启动")
	}
	var result struct {
		IP string `json:"ip"`
	}
	if json.Unmarshal(output, &result) != nil {
		return errors.New("隔离出口返回无效数据")
	}
	emit("observation", model.Observation{IP: result.IP, At: time.Now().UTC(), Revision: cfg.Revision})
	// Escape probes must fail: no Windows disk, interop binary, or direct TCP internet.
	probe := exec.CommandContext(ctx, "runuser", "-u", "claudeff", "--", "/bin/bash", "/opt/claudeff/launch.sh", "/bin/bash", "-c", `test ! -e /mnt/c && test ! -e /run/WSL && ! command -v powershell.exe && ! curl --noproxy '*' --connect-timeout 3 --max-time 5 -s http://1.1.1.1 >/dev/null`)
	probe.Env = env
	if e = probe.Run(); e != nil {
		return errors.New("隔离核验未通过，禁止降级运行")
	}
	if cfg.Mode == "check" {
		emit("ready", "隔离与出口检测完成")
		return nil
	}
	if cfg.Mode == "install" {
		emit("progress", "正在从官方渠道下载并安装 Claude Code")
		// A single permitted script is fetched over HTTPS; upstream credentials stay in root relay.
		installer := `set -e; curl --location --fail --silent --show-error --max-time 120 https://claude.ai/install.sh -o /tmp/claude-installer.sh; bash /tmp/claude-installer.sh stable; test -x "$HOME/.local/bin/claude"; "$HOME/.local/bin/claude" --version`
		if e = execute("runuser", "-u", "claudeff", "--", "/bin/bash", "/opt/claudeff/launch.sh", "/bin/bash", "-c", installer); e != nil {
			return e
		}
		emit("ready", "Claude Code 已安装；需重启专用发行版使宿主访问限制生效")
		return nil
	}
	if _, e = os.Stat("/home/claudeff/.local/bin/claude"); e != nil {
		return errors.New("Claude Code 尚未安装")
	}
	cmd := exec.CommandContext(ctx, "runuser", "-u", "claudeff", "--", "/bin/bash", "/opt/claudeff/launch.sh", "/home/claudeff/.local/bin/claude")
	cmd.Env = env
	f, e := pty.StartWithSize(cmd, &pty.Winsize{Cols: 100, Rows: 30})
	if e != nil {
		return errors.New("隔离终端启动失败")
	}
	defer f.Close()
	ptMu.Lock()
	pt = f
	ptMu.Unlock()
	emit("ready", "隔离终端已启动，请在终端中完成官方登录")
	go func() {
		buffer := make([]byte, 8192)
		for {
			n, e := f.Read(buffer)
			if n > 0 {
				emit("terminal", string(buffer[:n]))
			}
			if e != nil {
				return
			}
		}
	}()
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				q := exec.CommandContext(ctx, "runuser", "-u", "claudeff", "--", "/bin/bash", "/opt/claudeff/launch.sh", "/usr/bin/curl", "--fail", "--silent", "--max-time", "20", proxy.DetectionURL)
				q.Env = env
				data, err := q.Output()
				var observed struct {
					IP string `json:"ip"`
				}
				if err == nil {
					err = json.Unmarshal(data, &observed)
				}
				if err != nil || observed.IP != result.IP {
					emit("error", "运行中出口检测失败或发生变化，已停止隔离终端")
					cancel()
					return
				}
			}
		}
	}()
	e = cmd.Wait()
	ptMu.Lock()
	pt = nil
	ptMu.Unlock()
	if e != nil && ctx.Err() == nil {
		return errors.New("Claude Code 会话结束，退出状态异常")
	}
	emit("exit", "终端已结束")
	return nil
}
