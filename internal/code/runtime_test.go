package code

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/yingziseo/claudeFF/internal/model"
	"github.com/yingziseo/claudeFF/internal/proxy"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOfficialNativeInstallAndTerminal(t *testing.T) {
	if os.Getenv("CLAUDEFF_CODE_SMOKE") != "1" {
		t.Skip("actual official installation is cloud-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var requests atomic.Int64
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
		requests.Add(1)
		rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		rw.Flush()
		go func() { io.Copy(up, rw); up.Close() }()
		io.Copy(c, up)
	}))
	defer gateway.Close()
	port := gateway.Listener.Addr().(*net.TCPAddr).Port
	p := model.Proxy{Protocol: "http", Host: "127.0.0.1", Port: port, Username: "ci", Password: "test"}
	relay, e := proxy.Start(p, "")
	if e != nil {
		t.Fatal(e)
	}
	defer relay.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 13*time.Minute)
	defer cancel()
	root := t.TempDir()
	var mu sync.Mutex
	var terminal strings.Builder
	m := New(root, func(_, s string) { mu.Lock(); defer mu.Unlock(); terminal.WriteString(s) })
	defer m.Stop()
	client := proxy.ClientThrough(relay.URL())
	client.Timeout = 10 * time.Minute
	if e = m.Install(ctx, client, relay.URL()); e != nil {
		t.Fatal(e)
	}
	if !m.Installed() || m.State().Version == "" || requests.Load() == 0 {
		t.Fatal("actual installer/verified version or proxy traffic missing")
	}
	obs, e := proxy.Detect(ctx, p, 1)
	if e != nil {
		t.Fatal(e)
	}
	project := filepath.Join(home, "project")
	os.MkdirAll(project, 0700)
	if e = m.ChooseProject(project); e != nil {
		t.Fatal(e)
	}
	if e = m.Start(ctx, relay.URL(), obs); e != nil {
		t.Fatal(e)
	}
	if e = m.Resize(90, 25); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(25 * time.Second)
	seen := false
	for time.Now().Before(deadline) {
		mu.Lock()
		seen = terminal.Len() > 0
		mu.Unlock()
		if seen {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !seen {
		t.Fatal("official native terminal produced no output")
	}
	if e = m.Write("\x03"); e != nil {
		t.Fatal(e)
	}
	m.Stop()
	if m.State().Phase != "ready" {
		t.Fatal("installed Code lost after stop")
	}
	t.Log(fmt.Sprintf("Verified native official %s, authenticated managed proxy, terminal output/input/resize, local project and retained installation. No Claude account login or model request was performed.", m.State().Version))
}
