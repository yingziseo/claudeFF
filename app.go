//go:build windows || darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yingziseo/claudeFF/internal/browser"
	"github.com/yingziseo/claudeFF/internal/code"
	"github.com/yingziseo/claudeFF/internal/licensing"
	"github.com/yingziseo/claudeFF/internal/model"
	"github.com/yingziseo/claudeFF/internal/proxy"
	"github.com/yingziseo/claudeFF/internal/secret"
	"github.com/yingziseo/claudeFF/internal/store"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var releaseMode = "development"
var licenseURL = ""

type App struct {
	authMu         sync.Mutex
	businessCtx    context.Context
	businessCancel context.CancelFunc
	licensed       bool
	license        *licensing.Client
	ctx            context.Context
	root, bundle   string
	db             *store.Store
	browser        browser.Manager
	code           *code.Manager
	relay          *proxy.Relay
	network        model.Status
	mu             sync.Mutex
	operation      sync.Mutex
	initErr        error
	cancelInstall  context.CancelFunc
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	base, e := os.UserConfigDir()
	if e != nil {
		a.initErr = e
		return
	}
	a.root = filepath.Join(base, "claudeFF")
	exe, _ := os.Executable()
	a.bundle = filepath.Dir(exe)
	a.db, a.initErr = store.Open(a.root, secret.Vault{})
	if a.initErr != nil {
		return
	}
	a.network = model.Status{Phase: "idle", Message: "请检测全局代理"}
	a.code = code.New(a.root, func(event, data string) { runtime.EventsEmit(a.ctx, event, data) })
	if a.code.Installed() {
		a.code.RestoreReady()
	}
	if releaseMode == "production" {
		token, _ := a.db.LicenseToken()
		a.license, _ = licensing.NewClient(licenseURL, token)
		if a.license != nil && token != "" {
			a.licensed = a.license.Verify(a.ctx, false, false) == nil
		}
		go a.licenseMonitor()
	}

}
func (a *App) shutdown(ctx context.Context) {
	a.authMu.Lock()
	if a.businessCancel != nil {
		a.businessCancel()
	}
	a.authMu.Unlock()
	a.browser.Stop()
	if a.code != nil {
		a.code.Stop()
	}
	if a.relay != nil {
		a.relay.Close()
	}
	if a.db != nil {
		a.db.Close()
	}
}
func (a *App) beforeClose(ctx context.Context) bool {
	if a.code == nil {
		return false
	}
	bs, _ := a.browser.State()
	cs := a.code.State()
	locked := a.operation.TryLock()
	if locked {
		a.operation.Unlock()
	}
	if bs.Phase == "running" || cs.Phase == "running" || !locked {
		choice, _ := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.QuestionDialog, Title: "退出工作空间？", Message: "正在使用的浏览器或终端将关闭。请先保存工作。", Buttons: []string{"继续工作", "退出并关闭"}, DefaultButton: "继续工作", CancelButton: "继续工作"})
		return choice != "退出并关闭"
	}
	return false
}
func (a *App) snapshot() (string, error) {
	if a.initErr != nil {
		return "", errors.New("本地数据初始化失败，请检查目录权限")
	}
	a.authMu.Lock()
	authorized := releaseMode != "production" || a.licensed
	a.authMu.Unlock()
	if !authorized {
		raw, _ := json.Marshal(model.Snapshot{AuthRequired: true, Authorized: false, Native: true})
		return string(raw), nil
	}
	p, revision, e := a.db.Proxy()
	secretUnavailable := errors.Is(e, store.ErrSecretUnavailable)
	if e != nil && !secretUnavailable {
		return "", e
	}
	profile, e := a.db.Profile()
	if e != nil {
		return "", e
	}
	bs, observed := a.browser.State()
	a.mu.Lock()
	network := a.network
	a.mu.Unlock()
	if secretUnavailable {
		network = model.Status{Phase: "failed", Message: store.ErrSecretUnavailable.Error()}
	}
	cs := a.code.State()
	if network.Observation != nil && time.Since(network.Observation.At) > 5*time.Minute {
		var active *model.Observation
		if bs.Phase == "running" {
			active = bs.Observation
		}
		if cs.Phase == "running" && cs.Observation != nil && (active == nil || cs.Observation.At.After(active.At)) {
			active = cs.Observation
		}
		if active != nil && active.IP == network.Observation.IP && active.Revision == network.Observation.Revision && time.Since(active.At) < 2*time.Minute {
			network.Observation = active
			network.Message = "受管出口最近已核验"
		} else {
			network.Phase = "idle"
			network.Message = "检测已过期，请重新检测"
			network.Observation = nil
		}
	}

	previous, _ := a.db.PreviousObservation()
	s := model.Snapshot{BrowserSupported: browser.Supported(), Previous: previous, AuthRequired: releaseMode == "production", Authorized: true, Proxy: p.Public(), HasPassword: p.Password != "", Revision: revision, Profile: profile, Browser: bs, Code: a.code.State(), Network: network, Project: a.code.Project(), Platform: desktopPlatform(), Kernel: browser.Find(filepath.Join(a.bundle, "browser")) != "", Native: true, DataDir: a.root}
	raw, e := json.Marshal(struct {
		model.Snapshot
		Fingerprint map[string]any `json:"fingerprint"`
	}{s, observed})
	return string(raw), e
}
func (a *App) check(parent context.Context) error {
	p, rev, e := a.db.Proxy()
	if e != nil {
		return e
	}
	a.mu.Lock()
	a.network = model.Status{Phase: "busy", Message: "正在通过全局代理检测出口"}
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 35*time.Second)
	defer cancel()
	o, e := proxy.Detect(ctx, p, rev)
	a.mu.Lock()
	defer a.mu.Unlock()
	if e != nil {
		a.network = model.Status{Phase: "failed", Message: e.Error()}
		return e
	}
	a.network = model.Status{Phase: "ready", Message: "全局代理出口已检测", Observation: o}
	a.db.Record("network", o)
	return nil
}
func (a *App) relayReady() error {
	if a.relay != nil {
		return nil
	}
	p, _, e := a.db.Proxy()
	if e != nil {
		return e
	}
	a.relay, e = proxy.Start(p, "")
	return e
}
func (a *App) currentObservation() (*model.Observation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.network.Phase != "ready" || a.network.Observation == nil {
		return nil, errors.New("请先检测全局代理")
	}
	if time.Since(a.network.Observation.At) > 5*time.Minute {
		return nil, errors.New("连接验证已过期，请重新检测")
	}
	o := *a.network.Observation
	return &o, nil
}
func (a *App) stopped() error {
	bs, _ := a.browser.State()
	cs := a.code.State()
	if bs.Phase == "running" || cs.Phase == "running" || cs.Phase == "busy" {
		return errors.New("请先停止浏览器和终端，再修改共享设置")
	}
	return nil
}
func (a *App) Invoke(action, payload string) (string, error) {
	if a.initErr != nil {
		return "", a.initErr
	}
	if len(payload) > 2<<20 {
		return "", errors.New("请求过大")
	}
	if action == "snapshot" {
		return a.snapshot()
	}
	if action == "auth.login" || action == "auth.register" {
		a.authMu.Lock()
		defer a.authMu.Unlock()
		if a.license == nil {
			return "", errors.New("正式授权服务尚未配置，请联系提供者")
		}
		var input struct{ Username, Password, Card string }
		if e := json.Unmarshal([]byte(payload), &input); e != nil {
			return "", e
		}
		card := ""
		if action == "auth.register" {
			card = input.Card
			if card == "" {
				return "", errors.New("请输入卡密")
			}
		}
		if e := a.license.Login(a.ctx, input.Username, input.Password, card); e != nil {
			return "", e
		}
		if e := a.db.SaveLicenseToken(a.license.Token); e != nil {
			return "", e
		}
		a.licensed = true
		raw, _ := json.Marshal(map[string]bool{"authorized": true})
		return string(raw), nil
	}
	if action == "auth.logout" {
		a.authMu.Lock()
		a.licensed = false
		if a.businessCancel != nil {
			a.businessCancel()
		}
		if a.license != nil {
			var result map[string]any
			a.license.Request(a.ctx, "/v1/logout", struct{}{}, &result)
			a.license.Token = ""
		}
		a.db.SaveLicenseToken("")
		a.authMu.Unlock()
		a.browser.Stop()
		a.code.Stop()
		return a.snapshot()
	}
	a.authMu.Lock()
	allowed := releaseMode != "production" || a.licensed
	a.authMu.Unlock()
	if !allowed {
		return "", errors.New("请先登录并激活产品授权")
	}
	a.authMu.Lock()
	if a.businessCtx == nil || a.businessCtx.Err() != nil {
		a.businessCtx, a.businessCancel = context.WithCancel(a.ctx)
	}
	businessCtx := a.businessCtx
	if releaseMode == "production" && !a.licensed {
		a.businessCancel()
	}
	a.authMu.Unlock()

	if action == "proxy.parse" || action == "proxy.clipboard" {
		var input struct{ Value, Protocol string }
		if e := json.Unmarshal([]byte(payload), &input); e != nil {
			return "", errors.New("识别请求格式无效")
		}
		if action == "proxy.clipboard" {
			var e error
			input.Value, e = runtime.ClipboardGetText(a.ctx)
			if e != nil {
				return "", errors.New("剪贴板读取失败，请粘贴到识别框")
			}
		}
		p, e := model.ParseProxyText(input.Value, input.Protocol)
		if e != nil {
			return "", e
		}
		raw, e := json.Marshal(p)
		return string(raw), e
	}
	if action == "provider.open" {
		runtime.BrowserOpenURL(a.ctx, "https://www.ipdeep.cn/?extendid=30d6N2VjYjI3MzQ4NDUyNTY3OTQxMTg5")
		return "{}", nil
	}
	if action == "terminal.write" {
		var p struct {
			Data string `json:"data"`
		}
		if e := json.Unmarshal([]byte(payload), &p); e != nil {
			return "", e
		}
		return "{}", a.code.Write(p.Data)
	}
	if action == "terminal.resize" {
		var p struct{ Cols, Rows int }
		json.Unmarshal([]byte(payload), &p)
		return "{}", a.code.Resize(p.Cols, p.Rows)
	}
	// Stop/cancel can interrupt installation without acquiring its operation lock.
	if action == "code.stop" {
		a.mu.Lock()
		cancel := a.cancelInstall
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		a.code.Stop()
		return a.snapshot()
	}
	if !a.operation.TryLock() {
		return "", errors.New("另一项操作正在执行，请等待或取消安装")
	}
	defer a.operation.Unlock()
	var e error
	switch action {
	case "proxy.save", "proxy.saveAndCheck":
		if e = a.stopped(); e != nil {
			break
		}
		var p struct {
			model.Proxy
			KeepPassword bool `json:"keepPassword"`
		}
		if e = json.Unmarshal([]byte(payload), &p); e != nil {
			break
		}
		if strings.Contains(p.Host, ":") && net.ParseIP(p.Host) == nil {
			var parsed model.Proxy
			parsed, e = model.ParseProxyText(p.Host, p.Protocol)
			if e != nil {
				break
			}
			p.Host, p.Port, p.Protocol = parsed.Host, parsed.Port, parsed.Protocol
			if parsed.Username != "" || parsed.Password != "" {
				p.Username, p.Password = parsed.Username, parsed.Password
				p.KeepPassword = false
			}
		}
		if p.KeepPassword && p.Password == "" {
			old, _, err := a.db.Proxy()
			if err != nil {
				e = err
				break
			}
			p.Password = old.Password
		}
		if _, e = a.db.SaveProxy(p.Proxy); e != nil {
			break
		}
		if a.relay != nil {
			a.relay.Close()
			a.relay = nil
		}
		a.mu.Lock()
		a.network = model.Status{Phase: "idle", Message: "配置已更新，请重新检测"}
		a.mu.Unlock()
		if action == "proxy.saveAndCheck" {
			e = a.check(businessCtx)
		}
	case "proxy.check":
		e = a.check(businessCtx)
	case "kernel.install":
		if browser.Find(filepath.Join(a.bundle, "browser")) == "" {
			e = errors.New("发布包缺少内置指纹内核，请重新解压完整 ZIP")
		}
	case "browser.start":
		if !browser.Supported() {
			e = errors.New("当前 Mac 内核仅支持 Apple Silicon，Claude Code 可正常使用")
			break
		}
		if e = a.check(businessCtx); e != nil {
			break
		}
		var o *model.Observation
		o, e = a.currentObservation()
		if e != nil {
			break
		}
		if e = a.relayReady(); e != nil {
			break
		}
		var p *model.Profile
		p, e = a.ensureProfile("browser", o)
		if e != nil {
			break
		}
		e = a.browser.Start(businessCtx, browser.Find(filepath.Join(a.bundle, "browser")), a.root, a.relay.URL(), *p, o)
	case "browser.stop":
		a.browser.Stop()
	case "profile.save":
		if state, _ := a.browser.State(); state.Phase == "running" {
			e = errors.New("请先关闭浏览器")
			break
		}
		var p *model.Profile
		p, e = a.db.Profile()
		if e != nil {
			break
		}
		if p == nil {
			e = errors.New("请先创建环境")
			break
		}
		var fields struct{ Language, Timezone string }
		if e = json.Unmarshal([]byte(payload), &fields); e != nil {
			break
		}
		p.Language = fields.Language
		p.Timezone = fields.Timezone
		p.Revision++
		e = a.db.SaveProfile(*p)
	case "code.install":
		installCtx, cancel := context.WithTimeout(businessCtx, 25*time.Minute)
		defer cancel()
		a.mu.Lock()
		a.cancelInstall = cancel
		a.mu.Unlock()
		defer func() { a.mu.Lock(); a.cancelInstall = nil; a.mu.Unlock() }()
		if e = a.check(businessCtx); e != nil {
			break
		}
		if e = a.relayReady(); e != nil {
			break
		}
		client := proxy.ClientThrough(a.relay.URL())
		client.Timeout = 20 * time.Minute
		e = a.code.Install(installCtx, client, a.relay.URL())
	case "code.start":
		if !a.code.Installed() {
			e = errors.New("请先安装官方 Code")
			break
		}
		if e = a.check(businessCtx); e != nil {
			break
		}
		var o *model.Observation
		o, e = a.currentObservation()
		if e != nil {
			break
		}
		if e = a.relayReady(); e != nil {
			break
		}
		e = a.code.Start(businessCtx, a.relay.URL(), o)
	case "project.choose":
		var path string
		path, e = runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择本机项目目录"})
		if e == nil && path != "" {
			e = a.code.ChooseProject(path)
		}
	case "data.open":
		e = openDataDirectory(a.root)
	case "cache.clear":
		if state, _ := a.browser.State(); state.Phase == "running" {
			e = errors.New("请先关闭浏览器")
			break
		}
		for _, name := range []string{"Cache", "Code Cache", "GPUCache"} {
			if err := os.RemoveAll(filepath.Join(a.root, "browser-profile", "Default", name)); err != nil {
				e = err
				break
			}
		}
	default:
		e = errors.New("不支持的操作")
	}
	if e != nil {
		return "", e
	}
	return a.snapshot()
}

func (a *App) checkLicense() {
	a.authMu.Lock()
	if a.license == nil || a.license.Token == "" {
		a.authMu.Unlock()
		return
	}
	client := *a.license
	a.authMu.Unlock()
	bs, _ := a.browser.State()
	cs := a.code.State()
	e := client.Verify(a.ctx, bs.Phase == "running", cs.Phase == "running")
	a.authMu.Lock()
	same := a.license != nil && a.license.Token == client.Token
	if same {
		a.licensed = e == nil
		if e != nil && a.businessCancel != nil {
			a.businessCancel()
		}
	}
	a.authMu.Unlock()
	if same && e != nil {
		a.mu.Lock()
		cancel := a.cancelInstall
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		a.browser.Stop()
		a.code.Stop()
	}
}
func (a *App) licenseMonitor() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.checkLicense()
		}
	}
}

func (a *App) ensureProfile(source string, o *model.Observation) (*model.Profile, error) {
	existing, e := a.db.Profile()
	if e != nil {
		return nil, e
	}
	if existing != nil {
		return existing, nil
	}
	p, e := a.db.EnsureProfile(source)
	if e != nil {
		return nil, e
	}
	if o != nil {
		if o.Timezone != "" {
			p.Timezone = o.Timezone
		}
		switch o.CountryCode {
		case "GB":
			p.Language = "en-GB"
		case "JP":
			p.Language = "ja-JP"
		case "CN":
			p.Language = "zh-CN"
		}
	}
	if e = a.db.SaveProfile(*p); e != nil {
		return nil, e
	}
	return p, nil
}
