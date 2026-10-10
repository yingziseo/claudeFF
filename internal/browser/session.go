package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/yingziseo/claudeFF/internal/proxy"
)

// Chromium's own Sessions store keeps navigation history, tab groups and form state.
// The private URL checkpoint additionally handles browser crashes and the changing
// local information-page port. It stores URLs privately, never extracts page contents.
type session struct {
	Tabs []string `json:"tabs"`
	Home string   `json:"home"`
}

func externalTab(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.User == nil
}
func detectionTab(raw string) bool {
	for _, p := range proxy.DetectionEndpoints() {
		if raw == p {
			return true
		}
	}
	return false
}
func (m *Manager) restoreTabs(ctx context.Context, home string) error {
	var previous session
	b, _ := os.ReadFile(filepath.Join(m.root, "browser-session.json"))
	_ = json.Unmarshal(b, &previous)
	var infos []*target.Info
	if e := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error { var e error; infos, e = target.GetTargets().Do(c); return e })); e != nil {
		return e
	}
	controlBlank := false
	for _, t := range infos {
		if t.TargetID == chromedp.FromContext(ctx).Target.TargetID && (t.URL == "about:blank" || t.URL == "") {
			controlBlank = true
		}
	}
	restored, hasHome := 0, false
	for _, t := range infos {
		if t.Type != "page" {
			continue
		}
		if previous.Home != "" && t.URL == previous.Home {
			if t.TargetID == chromedp.FromContext(ctx).Target.TargetID {
				if e := chromedp.Run(ctx, chromedp.Navigate(home)); e != nil {
					return e
				}
			} else if controlBlank {
				if e := chromedp.Run(ctx, chromedp.Navigate(home), chromedp.ActionFunc(func(c context.Context) error { return target.CloseTarget(t.TargetID).Do(c) })); e != nil {
					return e
				}
			} else {
				if e := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
					_, e := target.CreateTarget(home).Do(c)
					if e == nil {
						e = target.CloseTarget(t.TargetID).Do(c)
					}
					return e
				})); e != nil {
					return e
				}
			}
			hasHome = true
		} else if externalTab(t.URL) && !detectionTab(t.URL) {
			restored++
		}
	}
	if !hasHome {
		blank := false
		for _, t := range infos {
			if t.TargetID == chromedp.FromContext(ctx).Target.TargetID && (t.URL == "about:blank" || t.URL == "") {
				blank = true
			}
		}
		if blank {
			if e := chromedp.Run(ctx, chromedp.Navigate(home)); e != nil {
				return e
			}
		} else {
			if e := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error { _, e := target.CreateTarget(home).Do(c); return e })); e != nil {
				return e
			}
		}
	}
	if restored == 0 {
		urls := previous.Tabs
		if len(urls) == 0 {
			urls = []string{"https://claude.ai/"}
		}
		for _, raw := range urls {
			if !externalTab(raw) || detectionTab(raw) {
				continue
			}
			if e := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error { _, e := target.CreateTarget(raw).Do(c); return e })); e != nil {
				return e
			}
		}
	}
	// Leave the last conversation selected without changing its URL.
	return chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
		tabs, e := target.GetTargets().Do(c)
		if e != nil {
			return e
		}
		for _, t := range tabs {
			if t.Type == "page" && externalTab(t.URL) && !detectionTab(t.URL) {
				return target.ActivateTarget(t.TargetID).Do(c)
			}
		}
		return page.BringToFront().Do(c)
	}))
}
func (m *Manager) saveSession() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	m.mu.Lock()
	ctx, root, home := m.ctx, m.root, m.homepage
	m.mu.Unlock()
	if ctx == nil || ctx.Err() != nil || root == "" {
		return
	}
	limited, done := context.WithTimeout(ctx, 3*time.Second)
	defer done()
	saved := session{Home: home}
	if e := chromedp.Run(limited, chromedp.ActionFunc(func(c context.Context) error {
		ts, e := target.GetTargets().Do(c)
		if e != nil {
			return e
		}
		for _, t := range ts {
			if t.Type == "page" && externalTab(t.URL) && !detectionTab(t.URL) {
				saved.Tabs = append(saved.Tabs, t.URL)
			}
		}
		return nil
	})); e != nil {
		return
	}
	raw, _ := json.Marshal(saved)
	path := filepath.Join(root, "browser-session.json")
	if os.WriteFile(path+".tmp", raw, 0600) == nil {
		_ = os.Rename(path+".tmp", path)
	}
}
func (m *Manager) trackSession(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.saveSession()
		}
	}
}
func browserExit(ctx context.Context) (string, string, int64, error) {
	for _, endpoint := range proxy.DetectionEndpoints() {
		limited, done := context.WithTimeout(ctx, 8*time.Second)
		start := time.Now()
		var body string
		e := chromedp.Run(limited, chromedp.Navigate(endpoint), chromedp.Text("body", &body, chromedp.ByQuery))
		done()
		if e == nil {
			if ip, e := proxy.DecodeIP([]byte(body)); e == nil {
				return ip, endpoint, time.Since(start).Milliseconds(), nil
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return "", "", 0, errors.New("浏览器代理检测站均无法访问")
}
