package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yingziseo/claudeFF/internal/model"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func bufReader(c net.Conn) *bufio.Reader { return bufio.NewReader(c) }

const DetectionURL = "https://api.ipify.org?format=json"

func DetectionEndpoints() []string {
	return []string{DetectionURL, "https://checkip.amazonaws.com", "https://icanhazip.com", "https://ipinfo.io/ip", "https://ifconfig.co/ip"}
}
func DecodeIP(data []byte) (string, error) {
	if ip := net.ParseIP(strings.TrimSpace(string(data))); ip != nil {
		return ip.String(), nil
	}
	var result struct {
		IP string `json:"ip"`
	}
	if json.Unmarshal(data, &result) != nil || net.ParseIP(result.IP) == nil {
		return "", errors.New("出口检测返回无效 IP")
	}
	return net.ParseIP(result.IP).String(), nil
}
func Detect(ctx context.Context, p model.Proxy, rev int) (*model.Observation, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, target string) (net.Conn, error) { return Dial(ctx, p, target) }, TLSHandshakeTimeout: 6 * time.Second, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	o, err := DetectAcross(ctx, client, DetectionEndpoints(), rev)
	if err != nil {
		return nil, err
	}
	GeoWith(ctx, client, "https://ipwho.is/"+o.IP+"?fields=success,ip,country,country_code,city,region,connection.isp,timezone.id", o)
	if o.CountryCode == "" {
		limited, cancel := context.WithTimeout(ctx, 3*time.Second)
		if fallback, e := DetectWith(limited, client, "https://ipinfo.io/json", rev); e == nil && fallback.IP == o.IP {
			o.CountryCode = fallback.CountryCode
			o.City = fallback.City
			o.Region = fallback.Region
			o.Timezone = fallback.Timezone
			o.ISP = fallback.ISP
		}
		cancel()
	}
	return o, nil
}
func DetectAcross(ctx context.Context, c *http.Client, endpoints []string, rev int) (*model.Observation, error) {
	ctx, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	var last error
	for _, endpoint := range endpoints {
		if ctx.Err() != nil {
			break
		}
		limited, cancel := context.WithTimeout(ctx, 8*time.Second)
		o, err := DetectWith(limited, c, endpoint, rev)
		cancel()
		if err == nil {
			return o, nil
		}
		last = err
	}
	if last == nil {
		last = errors.New("检测已取消或没有可用检测地址")
	}
	return nil, fmt.Errorf("代理检测失败：%w", last)
}
func DetectWith(ctx context.Context, c *http.Client, endpoint string, rev int) (*model.Observation, error) {
	started := time.Now()
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if e != nil {
		return nil, errors.New("检测地址无效")
	}
	resp, e := c.Do(req)
	if e != nil {
		var connection *ConnectionError
		if errors.As(e, &connection) {
			return nil, connection
		}
		var timeout net.Error
		if errors.Is(e, context.DeadlineExceeded) || (errors.As(e, &timeout) && timeout.Timeout()) {
			return nil, errors.New("连接超时：请确认代理网关可达，并具备访问海外网络的条件")
		}
		return nil, errors.New("检测请求或目标 TLS 连接失败；请确认代理协议、端口及海外网络连接")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusProxyAuthRequired {
		return nil, connectionError("代理认证失败（407），请检查用户名和密码")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("出口检测站返回 HTTP %d，已尝试代理内备用地址", resp.StatusCode)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 32768))
	if e != nil {
		return nil, errors.New("出口检测响应读取中断")
	}
	ip, e := DecodeIP(raw)
	if e != nil {
		return nil, e
	}
	o := &model.Observation{IP: ip, Latency: time.Since(started).Milliseconds(), At: time.Now().UTC(), Revision: rev, Endpoint: endpoint}
	// ipinfo's country is a country code; optional fields are never inferred from the gateway address.
	var info struct{ Country, City, Region, Timezone, Org string }
	if json.Unmarshal(raw, &info) == nil {
		if len(info.Country) == 2 {
			o.CountryCode = info.Country
		}
		o.City = info.City
		o.Region = info.Region
		o.ISP = info.Org
		if info.Timezone != "" {
			if _, e := time.LoadLocation(info.Timezone); e == nil && info.Timezone != "Local" {
				o.Timezone = info.Timezone
			}
		}
	}
	return o, nil
}
func ClientThrough(localURL string) *http.Client {
	u, _ := url.Parse(localURL)
	return &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(u), TLSHandshakeTimeout: 10 * time.Second}}
}
func GeoWith(ctx context.Context, c *http.Client, endpoint string, o *model.Observation) {
	limited, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	q, e := http.NewRequestWithContext(limited, "GET", endpoint, nil)
	if e != nil {
		return
	}
	r, e := c.Do(q)
	if e != nil {
		return
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return
	}
	var data struct {
		Success                   bool `json:"success"`
		IP, Country, City, Region string
		CountryCode               string `json:"country_code"`
		Connection                struct {
			ISP string `json:"isp"`
		}
		Timezone struct {
			ID string `json:"id"`
		}
	}
	if json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&data) != nil || !data.Success || net.ParseIP(data.IP) == nil || net.ParseIP(data.IP).String() != o.IP {
		return
	}
	o.Country = data.Country
	o.CountryCode = data.CountryCode
	o.City = data.City
	o.Region = data.Region
	o.ISP = data.Connection.ISP
	if _, e = time.LoadLocation(data.Timezone.ID); e == nil && data.Timezone.ID != "Local" {
		o.Timezone = data.Timezone.ID
	}
}
