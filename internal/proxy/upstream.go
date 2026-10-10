package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/yingziseo/claudeFF/internal/model"
	xproxy "golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func Dial(ctx context.Context, p model.Proxy, target string) (net.Conn, error) {
	if e := p.Validate(); e != nil {
		return nil, e
	}
	d := &net.Dialer{Timeout: 15 * time.Second}
	if p.Protocol == "socks5" {
		var auth *xproxy.Auth
		if p.Username != "" {
			auth = &xproxy.Auth{User: p.Username, Password: p.Password}
		}
		s, e := xproxy.SOCKS5("tcp", p.Address(), auth, d)
		if e != nil {
			return nil, errors.New("SOCKS5 配置失败")
		}
		c, e := s.(xproxy.ContextDialer).DialContext(ctx, "tcp", target)
		if e != nil {
			return nil, errors.New("SOCKS5 代理连接失败")
		}
		return c, nil
	}
	var c net.Conn
	var e error
	if p.Protocol == "https" {
		c, e = (&tls.Dialer{NetDialer: d, Config: &tls.Config{ServerName: p.Host, MinVersion: tls.VersionTLS12}}).DialContext(ctx, "tcp", p.Address())
	} else {
		c, e = d.DialContext(ctx, "tcp", p.Address())
	}
	if e != nil {
		return nil, errors.New("代理网关不可达")
	}
	c.SetDeadline(time.Now().Add(20 * time.Second))
	req := &http.Request{Method: "CONNECT", URL: &url.URL{Opaque: target}, Host: target, Header: make(http.Header)}
	if p.Username != "" {
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(p.Username+":"+p.Password)))
	}
	if e = req.Write(c); e != nil {
		c.Close()
		return nil, errors.New("代理握手失败")
	}
	r := bufio.NewReader(c)
	resp, e := http.ReadResponse(r, req)
	if e != nil {
		c.Close()
		return nil, errors.New("代理响应无效")
	}
	if resp.StatusCode != 200 {
		c.Close()
		return nil, fmt.Errorf("代理连接被拒绝（HTTP %d），请检查认证和网关权限", resp.StatusCode)
	}
	c.SetDeadline(time.Time{})
	return &bufferedConn{Conn: c, reader: r}, nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) { return c.reader.Read(b) }
func SafeTarget(address string) error {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return errors.New("无效目标")
	}
	if port != "80" && port != "443" {
		return errors.New("仅允许 HTTP/HTTPS 目标")
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return errors.New("不允许本地目标")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return errors.New("不允许私网目标")
	}
	return nil
}
