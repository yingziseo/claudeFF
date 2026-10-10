package proxy

import (
	"context"
	"crypto/subtle"
	"github.com/yingziseo/claudeFF/internal/model"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Relay struct {
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	p        model.Proxy
	token    string
	conns    map[net.Conn]bool
	closed   bool
}

func Start(p model.Proxy, token string) (*Relay, error) {
	if e := p.Validate(); e != nil {
		return nil, e
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	r := &Relay{p: p, token: token, listener: l, conns: map[net.Conn]bool{}}
	r.server = &http.Server{Handler: r, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 * 1024}
	go r.server.Serve(l)
	return r, nil
}
func (r *Relay) Port() int   { return r.listener.Addr().(*net.TCPAddr).Port }
func (r *Relay) URL() string { return "http://" + r.listener.Addr().String() }
func (r *Relay) Close() {
	r.mu.Lock()
	r.closed = true
	for c := range r.conns {
		c.Close()
	}
	r.mu.Unlock()
	r.server.Close()
}
func (r *Relay) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	if r.token != "" && subtle.ConstantTimeCompare([]byte(q.Header.Get("Proxy-Authorization")), []byte("Bearer "+r.token)) != 1 {
		w.Header().Set("Proxy-Authenticate", "Bearer")
		http.Error(w, "proxy authentication required", 407)
		return
	}
	target := q.Host
	if q.Method != "CONNECT" {
		if !q.URL.IsAbs() || q.URL.Scheme != "http" {
			http.Error(w, "invalid proxy request", 400)
			return
		}
		target = q.URL.Host
		if !strings.Contains(target, ":") {
			target = net.JoinHostPort(target, "80")
		}
	}
	if e := SafeTarget(target); e != nil {
		http.Error(w, "destination blocked", 403)
		return
	}
	ctx, cancel := context.WithTimeout(q.Context(), 20*time.Second)
	defer cancel()
	up, e := Dial(ctx, r.p, target)
	if e != nil {
		http.Error(w, "upstream unavailable", 502)
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		up.Close()
		http.Error(w, "closed", 503)
		return
	}
	r.conns[up] = true
	r.mu.Unlock()
	defer func() { up.Close(); r.mu.Lock(); delete(r.conns, up); r.mu.Unlock() }()
	if q.Method == "CONNECT" {
		downstream, rw, e := w.(http.Hijacker).Hijack()
		if e != nil {
			return
		}
		r.mu.Lock()
		r.conns[downstream] = true
		r.mu.Unlock()
		defer func() { downstream.Close(); r.mu.Lock(); delete(r.conns, downstream); r.mu.Unlock() }()
		rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		rw.Flush()
		done := make(chan struct{})
		go func() { io.Copy(up, rw); up.Close(); close(done) }()
		io.Copy(downstream, up)
		downstream.Close()
		<-done
		return
	}
	q = q.Clone(q.Context())
	q.RequestURI = ""
	q.Header.Del("Proxy-Authorization")
	q.Header.Del("Proxy-Connection")
	q.Header.Del("Connection")
	q.Close = true
	if e = q.Write(up); e != nil {
		http.Error(w, "upstream unavailable", 502)
		return
	}
	resp, e := http.ReadResponse(bufReader(up), q)
	if e != nil {
		http.Error(w, "invalid upstream", 502)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		if strings.EqualFold(k, "Connection") {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, io.LimitReader(resp.Body, 256<<20))
}
