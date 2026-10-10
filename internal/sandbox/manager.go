package sandbox

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yingziseo/claudeFF/internal/model"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const Distro = "ClaudeFF"
const RootURL = "https://cloud-images.ubuntu.com/noble/20260926/noble-server-cloudimg-amd64-root.tar.xz"
const RootSHA = "820311e238ea34d76a8ca2643d6c01065212e9417109e7397d70472a236df261"

type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}
type Manager struct {
	mu           sync.Mutex
	status       model.Status
	cmd          *exec.Cmd
	input        io.WriteCloser
	cancel       context.CancelFunc
	root, bundle string
	emit         func(string, string)
}

func New(root, bundle string, emit func(string, string)) *Manager {
	return &Manager{root: root, bundle: bundle, emit: emit, status: model.Status{Phase: "idle", Message: "尚未安装"}}
}
func (m *Manager) State() model.Status { m.mu.Lock(); defer m.mu.Unlock(); return m.status }
func (m *Manager) set(phase, message string) {
	m.mu.Lock()
	m.status.Phase = phase
	m.status.Message = message
	m.mu.Unlock()
}
func wsl(ctx context.Context, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, "wsl.exe", args...)
	hide(c)
	return c
}
func (m *Manager) Prepare(ctx context.Context, client *http.Client) error {
	if runtime.GOOS != "windows" {
		return errors.New("隔离环境安装需要 Windows 11 和 WSL2")
	}
	if e := wsl(ctx, "--status").Run(); e != nil {
		return errors.New("请先启用 WSL2：点击启用按钮，完成系统确认和必要重启后重试")
	}
	listing, e := wsl(ctx, "--list", "--quiet").Output()
	if e != nil {
		return errors.New("无法读取 WSL 发行版状态")
	}
	list := strings.ReplaceAll(string(listing), "\x00", "")
	exists := false
	for _, line := range strings.Split(list, "\n") {
		if strings.TrimSpace(line) == Distro {
			exists = true
		}
	}
	if exists {
		// Do not adopt a user distribution that happens to have the same name.
		if _, e = os.Stat(filepath.Join(m.root, "wsl", ".owned")); e != nil {
			return errors.New("已存在同名 WSL 发行版，未将其作为本产品环境，请保留并处理命名冲突")
		}
	} else {
		m.set("busy", "正在下载已校验的专用 Linux 系统")
		if e = os.MkdirAll(filepath.Join(m.root, "wsl"), 0700); e != nil {
			return e
		}
		image := filepath.Join(m.root, "wsl", "rootfs.tar.xz")
		if e = download(ctx, client, RootURL, RootSHA, image); e != nil {
			return e
		}
		m.set("busy", "正在导入独立 WSL2 系统")
		if e = wsl(ctx, "--import", Distro, filepath.Join(m.root, "wsl", "system"), image, "--version", "2").Run(); e != nil {
			return errors.New("WSL2 导入失败，请检查虚拟化、磁盘和系统组件")
		}
		if e = os.WriteFile(filepath.Join(m.root, "wsl", ".owned"), []byte("claudeff-owned-v1"), 0600); e != nil {
			return e
		}
		os.Remove(image)
	}
	if e = wsl(ctx, "-d", Distro, "-u", "root", "--exec", "mkdir", "-p", "/opt/claudeff").Run(); e != nil {
		return errors.New("无法准备隔离系统目录")
	}
	for _, name := range []string{"wsl-agent", "provision.sh", "firewall.sh", "launch.sh"} {
		path := filepath.Join(m.bundle, "sandbox", name)
		data, e := os.ReadFile(path)
		if e != nil {
			return fmt.Errorf("发布包缺少隔离资源 %s", name)
		}
		cmd := wsl(ctx, "-d", Distro, "-u", "root", "--exec", "tee", "/opt/claudeff/"+name)
		cmd.Stdin = bytes.NewReader(data)
		cmd.Stdout = io.Discard
		if e = cmd.Run(); e != nil {
			return errors.New("写入隔离资源失败")
		}
	}
	if e = wsl(ctx, "-d", Distro, "-u", "root", "--exec", "chmod", "700", "/opt/claudeff/wsl-agent", "/opt/claudeff/provision.sh", "/opt/claudeff/firewall.sh").Run(); e != nil {
		return e
	}
	return wsl(ctx, "-d", Distro, "-u", "root", "--exec", "chmod", "755", "/opt/claudeff/launch.sh").Run()
}
func download(ctx context.Context, c *http.Client, url, digest, path string) error {
	q, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return e
	}
	r, e := c.Do(q)
	if e != nil {
		return errors.New("隔离系统下载失败，未回退直连")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("隔离下载返回 HTTP %d", r.StatusCode)
	}
	f, e := os.OpenFile(path+".partial", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer os.Remove(path + ".partial")
	hash := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r.Body, 1<<30))
	f.Close()
	if e != nil || n >= 1<<30 {
		return errors.New("隔离系统下载中断或大小超限")
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return errors.New("隔离系统完整性校验失败")
	}
	return os.Rename(path+".partial", path)
}
func (m *Manager) Launch(parent context.Context, mode string, p model.Proxy, profile model.Profile, expected *model.Observation) error {
	if expected == nil {
		return errors.New("请先检测全局代理")
	}
	m.mu.Lock()
	if m.cmd != nil {
		m.mu.Unlock()
		return errors.New("隔离操作正在运行")
	}
	m.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	cmd := wsl(ctx, "-d", Distro, "-u", "root", "--exec", "/opt/claudeff/wsl-agent")
	in, e := cmd.StdinPipe()
	if e != nil {
		cancel()
		return e
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		cancel()
		return e
	}
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		cancel()
		return errors.New("隔离代理启动失败")
	}
	config := map[string]any{"mode": mode, "proxy": p, "profile": profile, "revision": expected.Revision}
	encoded, _ := json.Marshal(config)
	m.mu.Lock()
	m.cmd = cmd
	m.input = in
	m.cancel = cancel
	m.status = model.Status{Phase: "busy", Message: "正在核验隔离环境"}
	m.mu.Unlock()
	if _, e = in.Write(append(encoded, '\n')); e != nil {
		m.Stop()
		return e
	}
	ready := make(chan error, 1)
	finished := make(chan error, 1)
	signal := func(e error) {
		select {
		case ready <- e:
		default:
		}
	}
	go func() {
		scanner := bufio.NewScanner(out)
		scanner.Buffer(make([]byte, 8192), 1<<20)
		verified := false
		for scanner.Scan() {
			var event Event
			if json.Unmarshal(scanner.Bytes(), &event) != nil {
				continue
			}
			var text string
			json.Unmarshal(event.Data, &text)
			switch event.Type {
			case "progress":
				m.set("busy", text)
			case "observation":
				var o model.Observation
				if json.Unmarshal(event.Data, &o) == nil && o.IP == expected.IP && o.Revision == expected.Revision {
					verified = true
					m.mu.Lock()
					m.status.Observation = &o
					m.mu.Unlock()
				} else {
					signal(errors.New("Code 与浏览器配置的出口不同，已阻止启动"))
					cancel()
				}
			case "ready":
				if !verified {
					signal(errors.New("缺少隔离出口验证"))
					cancel()
					continue
				}
				if mode == "run" {
					m.set("running", text)
				} else {
					m.set("ready", text)
				}
				signal(nil)
			case "version":
				m.mu.Lock()
				m.status.Version = text
				m.mu.Unlock()
			case "terminal":
				if m.emit != nil {
					m.emit("terminal:data", text)
				}
			case "error":
				m.set("failed", text)
				signal(errors.New(text))
			case "exit":
				if m.State().Phase != "failed" {
					m.set("ready", text)
				}
			}
		}
		e := cmd.Wait()
		m.mu.Lock()
		m.cmd = nil
		m.input = nil
		m.cancel = nil
		m.mu.Unlock()
		cancel()
		finished <- e
		if e != nil {
			if m.State().Phase != "failed" {
				m.set("failed", "隔离进程结束，启动已阻止")
			}
			signal(errors.New("隔离进程未正常完成"))
		} else {
			signal(nil)
		}
	}()
	select {
	case e = <-ready:
		if e != nil {
			return e
		}
		if mode != "run" {
			select {
			case endErr := <-finished:
				if endErr != nil {
					return errors.New("隔离安装未完整结束")
				}
			case <-parent.Done():
				m.Stop()
				return parent.Err()
			}
		}
		return nil
	case <-parent.Done():
		m.Stop()
		return parent.Err()
	case <-time.After(20 * time.Minute):
		m.Stop()
		return errors.New("隔离操作超时，请检查连接后重试")
	}
}
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel, in := m.cancel, m.input
	m.mu.Unlock()
	if in != nil {
		json.NewEncoder(in).Encode(map[string]string{"type": "stop"})
		in.Close()
	}
	if cancel != nil {
		cancel()
	}
	m.set("idle", "隔离操作已停止")
}
func (m *Manager) Write(data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.input == nil {
		return errors.New("终端未启动")
	}
	return json.NewEncoder(m.input).Encode(map[string]string{"type": "input", "data": data})
}
func (m *Manager) Resize(cols, rows int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.input == nil {
		return nil
	}
	return json.NewEncoder(m.input).Encode(map[string]any{"type": "resize", "cols": cols, "rows": rows})
}
func (m *Manager) Restart(ctx context.Context) error { return wsl(ctx, "--terminate", Distro).Run() }
func (m *Manager) Installed() bool {
	_, e := os.Stat(filepath.Join(m.root, "code-installed"))
	return e == nil
}
func (m *Manager) MarkInstalled() error {
	return os.WriteFile(filepath.Join(m.root, "code-installed"), []byte(m.State().Version), 0600)
}
func (m *Manager) ChooseProject(ctx context.Context, path string) error {
	info, e := os.Stat(path)
	if e != nil || !info.IsDir() {
		return errors.New("请选择项目目录")
	}
	folder := "project-" + fmt.Sprint(time.Now().UnixNano())
	dest := "/workspace/" + folder
	if e = wsl(ctx, "-d", Distro, "-u", "root", "--exec", "mkdir", "-p", dest).Run(); e != nil {
		return e
	}
	cmd := wsl(ctx, "-d", Distro, "-u", "root", "--exec", "tar", "-xf", "-", "-C", dest, "--no-same-owner", "--no-same-permissions")
	pipe, e := cmd.StdinPipe()
	if e != nil {
		return e
	}
	if e = cmd.Start(); e != nil {
		return e
	}
	writer := tar.NewWriter(pipe)
	var size int64
	walkErr := filepath.WalkDir(path, func(file string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, file)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("项目包含符号链接，请先复制为独立目录再导入")
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return errors.New("项目包含不支持的特殊文件")
		}
		size += info.Size()
		if size > 2<<30 {
			return errors.New("项目超过 2GB，请精简构建缓存后导入")
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if err = writer.WriteHeader(header); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(writer, f)
		return err
	})
	writer.Close()
	pipe.Close()
	runErr := cmd.Wait()
	if walkErr != nil {
		return walkErr
	}
	if runErr != nil {
		return errors.New("项目导入失败，原目录未修改")
	}
	if e = wsl(ctx, "-d", Distro, "-u", "root", "--exec", "chown", "-R", "claudeff:claudeff", dest).Run(); e != nil {
		return e
	}
	marker := wsl(ctx, "-d", Distro, "-u", "root", "--exec", "tee", "/opt/claudeff/project-path")
	marker.Stdin = strings.NewReader(dest)
	marker.Stdout = io.Discard
	if e = marker.Run(); e != nil {
		return e
	}
	return nil
}

func (m *Manager) RestoreReady() {
	m.set("ready", "隔离安装已保存；启动前重新核验")
	if version, e := os.ReadFile(filepath.Join(m.root, "code-installed")); e == nil && len(version) < 120 {
		m.mu.Lock()
		m.status.Version = string(version)
		m.mu.Unlock()
	}
}
func (m *Manager) Fail(e error) { m.set("failed", e.Error()) }
