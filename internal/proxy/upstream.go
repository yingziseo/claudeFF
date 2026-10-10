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

type ConnectionError struct{ Message string }

func (e *ConnectionError) Error() string   { return e.Message }
func connectionError(message string) error { return &ConnectionError{Message: message} }

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
			return nil, connectionError("SOCKS5 连接或认证失败，请确认用户名、密码、端口及网络可达")
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
		return nil, connectionError("代理网关连接失败，请确认协议、端口及海外网络连接；HTTPS 选项仅用于网关本身提供 TLS 的代理")
	}
	deadline := time.Now().Add(12 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	c.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	req := &http.Request{Method: "CONNECT", URL: &url.URL{Opaque: target}, Host: target, Header: make(http.Header)}
	if p.Username != "" {
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(p.Username+":"+p.Password)))
	}
	if e = req.Write(c); e != nil {
		c.Close()
		return nil, connectionError("代理握手发送失败，连接已中断")
	}
	r := bufio.NewReader(c)
	resp, e := http.ReadResponse(r, req)
	if e != nil {
		c.Close()
		return nil, connectionError("代理未返回有效 HTTP 握手，请确认是否应选择 SOCKS5 协议")
	}
	if resp.StatusCode != 200 {
		c.Close()
		if resp.StatusCode == 407 {
			return nil, connectionError("代理认证失败（HTTP 407），请检查用户名和密码")
		}
		return nil, connectionError(fmt.Sprintf("代理网关拒绝连接（HTTP %d），请检查代理权限和目标限制", resp.StatusCode))
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
