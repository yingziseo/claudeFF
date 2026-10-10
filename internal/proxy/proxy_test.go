package proxy

import (
	"bufio"
	"context"
	"github.com/yingziseo/claudeFF/internal/model"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticatedConnect(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	done := make(chan string, 1)
	go func() {
		c, e := listener.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		q, e := http.ReadRequest(bufio.NewReader(c))
		if e != nil {
			return
		}
		done <- q.Header.Get("Proxy-Authorization")
		c.Write([]byte("HTTP/1.1 200 OK\r\n\r\npayload"))
	}()
	p := model.Proxy{Protocol: "http", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Username: "user", Password: "pass"}
	c, e := Dial(context.Background(), p, "example.com:443")
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	data, e := io.ReadAll(c)
	if e != nil || string(data) != "payload" {
		t.Fatal(string(data), e)
	}
	if <-done != "Basic dXNlcjpwYXNz" {
		t.Fatal("authentication missing")
	}
}
func TestRelayFailClosedAndBlockedTargets(t *testing.T) {
	for _, dest := range []string{"127.0.0.1:443", "[::1]:443", "192.168.1.1:80", "localhost:443", "example.com:22"} {
		if SafeTarget(dest) == nil {
			t.Fatal(dest)
		}
	}
	r, e := Start(model.Proxy{Protocol: "http", Host: "127.0.0.1", Port: 1}, "secret")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for _, tc := range []struct {
		auth, url string
		want      int
	}{{"", "http://example.com", 407}, {"Bearer secret", "http://localhost", 403}, {"Bearer secret", "http://example.com", 502}} {
		q := httptest.NewRequest("GET", tc.url, nil)
		q.Header.Set("Proxy-Authorization", tc.auth)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d", w.Code, tc.want)
		}
	}
}
func TestDetectValidatesIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ip":"203.0.113.8"}`)) }))
	defer server.Close()
	o, e := DetectWith(context.Background(), &http.Client{Timeout: time.Second}, server.URL, 3)
	if e != nil || o.IP != "203.0.113.8" || o.Revision != 3 {
		t.Fatal(o, e)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ip":"fake"}`)) }))
	defer bad.Close()
	if _, e = DetectWith(context.Background(), http.DefaultClient, bad.URL, 1); e == nil {
		t.Fatal("invalid result accepted")
	}
}
