//go:build windows

package browser

import (
	"context"
	"encoding/base64"
	"github.com/yingziseo/claudeFF/internal/model"
	"github.com/yingziseo/claudeFF/internal/proxy"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsFingerprintKernelThroughAuthenticatedGateway(t *testing.T) {
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
	client := proxy.ClientThrough(relay.URL())
	client.Timeout = 6 * time.Minute
	exe, e := Install(ctx, client, filepath.Join(root, "kernel"))
	if e != nil {
		t.Fatal(e)
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
	t.Log("Verified Windows fingerprint kernel, authenticated gateway, actual exit IP, default pages, language/timezone and browser observations. This is a test gateway, not a residential-IP claim.")
}
