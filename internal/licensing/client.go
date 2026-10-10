package licensing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	Base, Token string
	HTTP        *http.Client
}

func NewClient(base, token string) (*Client, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("授权服务器地址无效")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")) {
		return nil, errors.New("授权服务必须使用 HTTPS")
	}
	return &Client{Base: strings.TrimRight(base, "/"), Token: token, HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Request(ctx context.Context, path string, body any, result any) error {
	raw, e := json.Marshal(body)
	if e != nil {
		return e
	}
	q, e := http.NewRequestWithContext(ctx, "POST", c.Base+path, bytes.NewReader(raw))
	if e != nil {
		return e
	}
	q.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		q.Header.Set("Authorization", "Bearer "+c.Token)
	}
	r, e := c.HTTP.Do(q)
	if e != nil {
		return errors.New("授权服务无法连接，请稍后重试")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		var data struct{ Error string }
		json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&data)
		if data.Error == "" {
			data.Error = "授权检查失败"
		}
		return errors.New(data.Error)
	}
	return json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(result)
}
func (c *Client) Login(ctx context.Context, name, password, card string) error {
	endpoint := "/v1/login"
	body := map[string]string{"Username": name, "Password": password}
	if card != "" {
		endpoint = "/v1/register"
		body["Card"] = card
	}
	var data struct{ Token string }
	if e := c.Request(ctx, endpoint, body, &data); e != nil {
		return e
	}
	if data.Token == "" {
		return errors.New("服务器未返回有效会话")
	}
	c.Token = data.Token
	return nil
}
func (c *Client) Verify(ctx context.Context, browser, code bool) error {
	var data struct{ Authorized bool }
	if e := c.Request(ctx, "/v1/heartbeat", map[string]any{"version": "0.2.0", "browser": browser, "code": code}, &data); e != nil {
		return e
	}
	if !data.Authorized {
		return errors.New("授权不可用")
	}
	return nil
}
