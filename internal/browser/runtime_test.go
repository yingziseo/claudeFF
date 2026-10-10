//go:build windows || darwin

package browser

import (
	"context"
	"encoding/base64"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/yingziseo/claudeFF/internal/model"
	"github.com/yingziseo/claudeFF/internal/proxy"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestFingerprintKernelThroughAuthenticatedGateway(t *testing.T) {
	if os.Getenv("CLAUDEFF_KERNEL_SMOKE") != "1" {
		t.Skip("kernel runtime check is cloud-only")
	}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("ci:test")) {
			w.WriteHeader(407)
			return
		}
		if r.Method != "CONNECT" {
			w.WriteHeader(405)
			return
		}
		up, e := net.DialTimeout("tcp", r.Host, 20*time.Second)
		if e != nil {
			w.WriteHeader(502)
			return
		}
		defer up.Close()
		c, rw, e := w.(http.Hijacker).Hijack()
		if e != nil {
			return
		}
		defer c.Close()
		rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		rw.Flush()
		go func() { io.Copy(up, rw); up.Close() }()
		io.Copy(c, up)
	}))
	defer gateway.Close()
	addr := gateway.Listener.Addr().(*net.TCPAddr)
	p := model.Proxy{Protocol: "http", Host: "127.0.0.1", Port: addr.Port, Username: "ci", Password: "test"}
	relay, e := proxy.Start(p, "")
	if e != nil {
		t.Fatal(e)
	}
	defer relay.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	root := t.TempDir()
	exe := Find(os.Getenv("CLAUDEFF_KERNEL_DIR"))
	if exe == "" {
		t.Fatal("cloud package is missing bundled kernel")
	}
	observed, e := proxy.Detect(ctx, p, 1)
	if e != nil {
		t.Fatal(e)
	}
	profile := model.NewProfile("browser")
	profile.Timezone = "America/Los_Angeles"
	var manager Manager
	defer manager.Stop()
	if e = manager.Start(ctx, exe, root, relay.URL(), profile, observed); e != nil {
		t.Fatal(e)
	}
	state, fingerprint := manager.State()
	if state.Phase != "running" || state.Observation.IP != observed.IP {
		t.Fatal("actual browser path not validated")
	}
	if fingerprint["timezone"] != profile.Timezone || fingerprint["language"] != profile.Language {
		t.Fatal("actual fingerprint differs from configuration", fingerprint)
	}
	if fingerprint["userAgent"] == nil || fingerprint["webgl"] == nil {
		t.Fatal("fingerprint observations missing")
	}
	hash := fingerprint["canvas"]
	if hash == nil || hash == "不可用" {
		t.Fatal("canvas observation missing")
	}
	probe, closeProbe := chromedp.NewContext(manager.ctx)
	if e = chromedp.Run(probe, chromedp.Navigate("https://example.com"), chromedp.Evaluate(`localStorage.setItem('claudeff-ci-retain','verified')`, nil)); e != nil {
		t.Fatal(e)
	}
	// Keep this tab open so normal browser restart must restore its conversation URL.
	if e = chromedp.Run(probe, chromedp.Navigate("https://example.com/#retained-conversation")); e != nil {
		t.Fatal(e)
	}
	manager.Stop()
	closeProbe()
	if e = manager.Start(ctx, exe, root, relay.URL(), profile, observed); e != nil {
		t.Fatal(e)
	}
	var tabsFound bool
	if e = chromedp.Run(manager.ctx, chromedp.ActionFunc(func(c context.Context) error {
		ts, e := target.GetTargets().Do(c)
		for _, v := range ts {
			if v.URL == "https://example.com/#retained-conversation" {
				tabsFound = true
			}
		}
		return e
	})); e != nil || !tabsFound {
		t.Fatal("conversation tab was not restored", e)
	}
	_, second := manager.State()
	if second["canvas"] != hash {
		t.Fatal("same stable seed changed canvas observation")
	}
	probe, closeProbe = chromedp.NewContext(manager.ctx)
	defer closeProbe()
	var retained string
	if e = chromedp.Run(probe, chromedp.Navigate("https://example.com"), chromedp.Evaluate(`localStorage.getItem('claudeff-ci-retain')`, &retained)); e != nil || retained != "verified" {
		t.Fatal("browser data not retained", retained, e)
	}
	t.Log("Verified stable canvas and persistent browser local storage across restart.")
	t.Log("Verified Windows fingerprint kernel, authenticated gateway, actual exit IP, default pages, language/timezone and browser observations. This is a test gateway, not a residential-IP claim.")
}
