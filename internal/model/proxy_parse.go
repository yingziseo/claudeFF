package model

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseProxyText accepts provider clipboard formats without ever including the input in errors.
func ParseProxyText(value, protocol string) (Proxy, error) {
	value = strings.TrimSpace(value)
	bad := errors.New("代理格式无法识别，请粘贴 地址:端口:用户名:密码 或完整代理 URL")
	if value == "" || len(value) > 2048 || strings.ContainsAny(value, "\r\n\t") {
		return Proxy{}, bad
	}
	if protocol == "" {
		protocol = "http"
	}
	p := Proxy{Protocol: strings.ToLower(protocol)}
	if strings.Contains(value, "://") || strings.Contains(value, "@") {
		if !strings.Contains(value, "://") {
			value = p.Protocol + "://" + value
		}
		u, err := url.Parse(value)
		if err != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return Proxy{}, bad
		}
		p.Protocol = strings.ToLower(u.Scheme)
		if p.Protocol == "socks5h" {
			p.Protocol = "socks5"
		}
		p.Host = u.Hostname()
		p.Port, err = strconv.Atoi(u.Port())
		if err != nil {
			return Proxy{}, bad
		}
		if u.User != nil {
			p.Username = u.User.Username()
			p.Password, _ = u.User.Password()
		}
	} else {
		parts := strings.SplitN(value, ":", 4)
		if strings.HasPrefix(value, "[") {
			end := strings.Index(value, "]:")
			if end < 0 {
				return Proxy{}, bad
			}
			parts = append([]string{value[1:end]}, strings.SplitN(value[end+2:], ":", 3)...)
		}
		if len(parts) != 2 && len(parts) != 4 {
			return Proxy{}, bad
		}
		p.Host = strings.TrimSpace(parts[0])
		var err error
		p.Port, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return Proxy{}, bad
		}
		if len(parts) == 4 {
			p.Username, p.Password = parts[2], parts[3]
		}
	}
	if strings.Contains(p.Host, ":") && net.ParseIP(p.Host) == nil {
		return Proxy{}, bad
	}
	if err := p.Validate(); err != nil {
		return Proxy{}, err
	}
	return p, nil
}
