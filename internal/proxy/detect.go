package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/yingziseo/claudeFF/internal/model"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

func bufReader(c net.Conn) *bufio.Reader { return bufio.NewReader(c) }

const DetectionURL = "https://api.ipify.org?format=json"

func Detect(ctx context.Context, p model.Proxy, rev int) (*model.Observation, error) {
	transport := &http.Transport{DialContext: func(ctx context.Context, network, target string) (net.Conn, error) { return Dial(ctx, p, target) }, TLSHandshakeTimeout: 10 * time.Second, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	o, e := DetectWith(ctx, client, DetectionURL, rev)
	if e != nil {
		return nil, e
	}
	GeoWith(ctx, client, "https://ipwho.is/"+o.IP+"?fields=success,ip,country,country_code,timezone.id", o)
	return o, nil
}
func DetectWith(ctx context.Context, c *http.Client, endpoint string, rev int) (*model.Observation, error) {
	started := time.Now()
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if e != nil {
		return nil, e
	}
	resp, e := c.Do(req)
	if e != nil {
		return nil, errors.New("无法通过代理检测出口，请检查连接后重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("出口检测服务暂不可用")
	}
	var result struct {
		IP string `json:"ip"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); e != nil || net.ParseIP(result.IP) == nil {
		return nil, errors.New("出口检测返回无效 IP")
	}
	return &model.Observation{IP: net.ParseIP(result.IP).String(), Latency: time.Since(started).Milliseconds(), At: time.Now().UTC(), Revision: rev}, nil
}
func ClientThrough(localURL string) *http.Client {
	u, _ := url.Parse(localURL)
	return &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(u), TLSHandshakeTimeout: 10 * time.Second}}
}

// Optional geography lookup never turns an unknown residential attribute into a positive claim.
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
		Success     bool `json:"success"`
		IP, Country string
		CountryCode string `json:"country_code"`
		Timezone    struct {
			ID string `json:"id"`
		}
	}
	if json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&data) != nil || !data.Success || net.ParseIP(data.IP) == nil || net.ParseIP(data.IP).String() != o.IP {
		return
	}
	o.Country = data.Country
	o.CountryCode = data.CountryCode
	if _, e = time.LoadLocation(data.Timezone.ID); e == nil && data.Timezone.ID != "Local" {
		o.Timezone = data.Timezone.ID
	}
}
