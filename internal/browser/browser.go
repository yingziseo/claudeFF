package browser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/yingziseo/claudeFF/internal/model"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Manager struct {
	mu              sync.Mutex
	sessionMu       sync.Mutex
	cancel          context.CancelFunc
	ctx             context.Context
	allocatorCancel context.CancelFunc
	server          *http.Server
	status          model.Status
	observed        map[string]any
	homepage        string
	root            string
}

func (m *Manager) State() (model.Status, map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := map[string]any{}
	for k, v := range m.observed {
		data[k] = v
	}
	return m.status, data
}
func (m *Manager) set(s model.Status) { m.mu.Lock(); m.status = s; m.mu.Unlock() }
func (m *Manager) Start(parent context.Context, exe, root, relayURL string, p model.Profile, expected *model.Observation) error {
	m.mu.Lock()
	if m.cancel != nil {
		ctx := m.ctx
		old := m.status.Observation
		m.mu.Unlock()
		if old != nil && expected != nil && old.IP != expected.IP {
			m.Stop()
			return errors.New("出口已变化，请重新启动浏览器")
		}
		return chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error { return page.BringToFront().Do(c) }))
	}
	if exe == "" {
		m.mu.Unlock()
		return errors.New("发布包缺少内置指纹内核，请重新解压完整 ZIP")
	}
	if expected == nil {
		m.mu.Unlock()
		return errors.New("请先检测全局代理")
	}
	m.status = model.Status{Phase: "busy", Message: "正在启动并核验浏览器出口"}
	m.mu.Unlock()
	dataDir := filepath.Join(root, "browser-profile")
	if e := os.MkdirAll(dataDir, 0700); e != nil {
		return e
	}
	platform := "windows"
	if runtime.GOOS == "darwin" {
		platform = "macos"
	}
	opts := []chromedp.ExecAllocatorOption{chromedp.ExecPath(exe), chromedp.UserDataDir(dataDir), chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck, chromedp.Flag("restore-last-session", true), chromedp.Flag("headless", false), chromedp.Flag("proxy-server", relayURL), chromedp.Flag("proxy-bypass-list", "127.0.0.1"), chromedp.Flag("fingerprint", p.Seed), chromedp.Flag("fingerprint-platform", platform), chromedp.Flag("lang", p.Language), chromedp.Flag("timezone", p.Timezone), chromedp.Flag("disable-non-proxied-udp", true), chromedp.Flag("disable-quic", true), chromedp.Flag("force-webrtc-ip-handling-policy", "disable_non_proxied_udp"), chromedp.Flag("disable-background-networking", true)}
	alloc, allocCancel := chromedp.NewExecAllocator(parent, opts...)
	ctx, cancel := chromedp.NewContext(alloc)
	m.mu.Lock()
	m.ctx = ctx
	m.cancel = cancel
	m.allocatorCancel = allocCancel
	m.root = root
	m.mu.Unlock()
	fail := func(e error) error { m.Stop(); m.set(model.Status{Phase: "failed", Message: e.Error()}); return e }
	if e := chromedp.Run(ctx); e != nil {
		return fail(errors.New("内核进程启动失败"))
	}
	// A separate probe tab must never overwrite a restored conversation.
	probeTab, closeProbe := chromedp.NewContext(ctx)
	probe, cancelProbe := context.WithTimeout(probeTab, 40*time.Second)
	defer cancelProbe()
	defer closeProbe()
	ip, endpoint, latency, e := browserExit(probe)
	if e != nil {
		return fail(errors.New("实际浏览器出口检测失败，已关闭浏览器"))
	}
	if ip != expected.IP {
		return fail(errors.New("浏览器出口与全局检测不同，已阻止使用"))
	}
	observed := map[string]any{}
	expr := `({canvas:(()=>{try{let c=document.createElement('canvas');c.width=240;c.height=60;let x=c.getContext('2d');x.font='16px Arial';x.fillStyle='#d97757';x.fillText('claudeFF stable profile',8,35);let data=c.toDataURL();let h=2166136261;for(let i=0;i<data.length;i++)h=Math.imul(h^data.charCodeAt(i),16777619);return (h>>>0).toString(16)}catch{return '不可用'}})(),audioSampleRate:(()=>{try{let c=new AudioContext();let rate=c.sampleRate;c.close();return rate}catch{return '不可用'}})(),userAgentData:navigator.userAgentData?JSON.stringify(navigator.userAgentData.toJSON()):'内核未提供',userAgent:navigator.userAgent,platform:navigator.platform,language:navigator.language,languages:navigator.languages,timezone:Intl.DateTimeFormat().resolvedOptions().timeZone,cores:navigator.hardwareConcurrency,memory:navigator.deviceMemory,screen:screen.width+' × '+screen.height,pixelRatio:devicePixelRatio,webgl:(()=>{try{let c=document.createElement('canvas').getContext('webgl');let e=c.getExtension('WEBGL_debug_renderer_info');return e?c.getParameter(e.UNMASKED_RENDERER_WEBGL):'不可用'}catch{return '不可用'}})()})`
	if e := chromedp.Run(probe, chromedp.Evaluate(expr, &observed)); e != nil {
		return fail(errors.New("浏览器参数观测失败"))
	}
	var tokenBytes [24]byte
	rand.Read(tokenBytes[:])
	token := hex.EncodeToString(tokenBytes[:])
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return fail(e)
	}
	url := "http://" + listener.Addr().String() + "/" + token
	data := struct{ IP, Country, Language, Timezone, ID, Version, At, Token string }{expected.IP, expected.Country, p.Language, p.Timezone, p.ID, KernelVersion, expected.At.Local().Format("2006-01-02 15:04:05"), token}
	tpl := template.Must(template.New("start").Parse(startHTML))
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/"+token {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-"+token+"'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.URL.Query().Get("refresh") == "1" {
			if e := m.verify(ctx, expected.IP); e != nil {
				m.Stop()
				m.set(model.Status{Phase: "failed", Message: e.Error()})
				http.Error(w, "浏览器出口核验失败，已停止", 503)
				return
			}
		}
		state, _ := m.State()
		display := data
		if state.Observation != nil {
			display.IP = state.Observation.IP
			display.At = state.Observation.At.Local().Format("2006-01-02 15:04:05")
		}
		tpl.Execute(w, display)
	})}
	m.mu.Lock()
	m.server = server
	m.observed = observed
	m.homepage = url
	m.mu.Unlock()
	go server.Serve(listener)
	closeProbe()
	if e = m.restoreTabs(ctx, url); e != nil {
		return fail(fmt.Errorf("浏览器标签页恢复失败（数据已保留）：%w", e))
	}
	o := *expected
	o.At = time.Now().UTC()
	o.Endpoint, o.Latency = endpoint, latency
	m.set(model.Status{Phase: "running", Message: "浏览器出口已核验", Observation: &o})
	go func() {
		<-ctx.Done()
		m.mu.Lock()
		if m.ctx == ctx {
			m.cancel = nil
			m.allocatorCancel = nil
			m.ctx = nil
			m.status = model.Status{Phase: "idle", Message: "浏览器已关闭"}
			if m.server != nil {
				m.server.Close()
				m.server = nil
			}
		}
		m.mu.Unlock()
	}()
	go m.monitor(ctx, expected.IP)
	go m.trackSession(ctx)
	return nil
}
func (m *Manager) Stop() {
	m.saveSession()
	m.mu.Lock()
	cancel, alloc, server, ctx := m.cancel, m.allocatorCancel, m.server, m.ctx
	m.cancel = nil
	m.ctx = nil
	m.allocatorCancel = nil
	m.server = nil
	m.observed = nil
	m.mu.Unlock()
	if ctx != nil {
		graceful, done := context.WithTimeout(ctx, 5*time.Second)
		chromedp.Cancel(graceful)
		done()
	}
	if cancel != nil {
		cancel()
	}
	if alloc != nil {
		alloc()
	}
	if server != nil {
		server.Close()
	}
	m.set(model.Status{Phase: "idle", Message: "浏览器已关闭"})
}

const startHTML = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>claudeFF 环境信息</title><meta name="viewport" content="width=device-width,initial-scale=1"><style>body{background:#fafaf9;color:#292927;font:16px -apple-system,Segoe UI,Microsoft YaHei,sans-serif;margin:0}main{max-width:850px;margin:10vh auto;padding:35px}h1{font-size:34px;font-weight:500}p{color:#686965;line-height:1.8}.grid{display:grid;grid-template-columns:repeat(3,1fr);background:white;border:1px solid #e4e4df;border-radius:16px;margin:35px 0}.grid div{padding:24px}small{display:block;color:#686965;margin-bottom:12px}b{font-weight:500}a{display:inline-block;background:#292927;color:white;padding:14px 22px;border-radius:8px;text-decoration:none}button{font:inherit;background:white;border:1px solid #e4e4df;color:#292927;padding:14px 22px;border-radius:8px;cursor:pointer}footer{margin-top:25px;color:#686965;font-size:12px}@media(max-width:600px){.grid{grid-template-columns:1fr 1fr}}</style><main><p>CLAUDEFF · 环境信息</p><h1>你的环境，一目了然。</h1><p>此页由当前指纹浏览器打开，出口已通过实际浏览器路径核验。</p><section class="grid"><div><small>出口 IP</small><b>{{.IP}}</b></div><div><small>国家 / 地区</small><b>{{if .Country}}{{.Country}}{{else}}未知{{end}}</b></div><div><small>语言</small><b>{{.Language}}</b></div><div><small>时区</small><b>{{.Timezone}}</b></div><div><small>环境标识</small><b>{{.ID}}</b></div><div><small>内核</small><b>{{.Version}}</b></div><div><small>住宅属性</small><b>未验证</b></div></section><a href="https://claude.ai/">打开 Claude.ai →</a> <a href="?refresh=1">刷新检测</a> <button id="copy">复制 IP</button><script nonce="{{.Token}}">document.getElementById("copy").onclick=()=>navigator.clipboard.writeText("{{.IP}}");</script><footer>最近核验 {{.At}} · 详细指纹请在管理程序查看。第三方工具，非官方客户端。</footer></main></html>`

func (m *Manager) verify(ctx context.Context, ip string) error {
	probe, cancel := chromedp.NewContext(ctx)
	defer cancel()
	limited, limit := context.WithTimeout(probe, 25*time.Second)
	defer limit()
	got, endpoint, latency, e := browserExit(limited)
	if e != nil || got != ip {
		return errors.New("运行期间出口检测失败或变化，已关闭浏览器")
	}
	m.mu.Lock()
	if m.status.Observation != nil {
		o := *m.status.Observation
		o.At = time.Now().UTC()
		o.Endpoint, o.Latency = endpoint, latency
		m.status.Observation = &o
	}
	m.mu.Unlock()
	return nil
}
func (m *Manager) monitor(ctx context.Context, ip string) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e := m.verify(ctx, ip); e != nil {
				m.Stop()
				m.set(model.Status{Phase: "failed", Message: e.Error()})
				return
			}
		}
	}
}
