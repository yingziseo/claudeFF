// code-helper keeps the existing, platform-native official Claude Code installer
// and PTY implementation available to the Tauri client. It exposes no TCP API.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/yingziseo/claudeFF/internal/code"
	"github.com/yingziseo/claudeFF/internal/model"
	"github.com/yingziseo/claudeFF/internal/proxy"
)

type request struct {
	ID      uint64 `json:"id"`
	Action  string `json:"action"`
	Relay   string `json:"relay,omitempty"`
	Project string `json:"project,omitempty"`
	Data    string `json:"data,omitempty"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
}
type response struct {
	ID    uint64 `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// The frontend cannot redirect installation/CLI traffic to an arbitrary URL:
// only the already-running application-owned loopback relay is accepted.
func validateRelay(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("应用代理入口无效")
	}
	ip := net.ParseIP(u.Hostname())
	port, err := strconv.Atoi(u.Port())
	if ip == nil || !ip.IsLoopback() || err != nil || port < 1 || port > 65535 {
		return errors.New("应用代理入口必须是本机转发")
	}
	return nil
}

func serve(ctx context.Context, root string, input io.Reader, output io.Writer) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var outputMu, operationMu, cancelMu sync.Mutex
	var operationCancel context.CancelFunc
	var jobs sync.WaitGroup
	send := func(r response) { outputMu.Lock(); defer outputMu.Unlock(); _ = json.NewEncoder(output).Encode(r) }
	manager := code.New(root, func(event, data string) {
		// Preserve UTF-8 sequences split across PTY reads. The frontend passes the
		// decoded bytes to xterm rather than interpreting each chunk as a string.
		if event == "terminal:data" {
			send(response{Event: event, Data: base64.StdEncoding.EncodeToString([]byte(data))})
		} else {
			send(response{Event: event, Data: data})
		}
	})
	snapshot := func() any {
		return struct {
			Status    model.Status `json:"status"`
			Installed bool         `json:"installed"`
			Project   string       `json:"project"`
		}{manager.State(), manager.Installed(), manager.Project()}
	}
	stop := func() {
		cancelMu.Lock()
		if operationCancel != nil {
			operationCancel()
		}
		cancelMu.Unlock()
		manager.Stop()
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var q request
		if err := json.Unmarshal(scanner.Bytes(), &q); err != nil || q.ID == 0 {
			send(response{ID: q.ID, Error: "请求格式无效"})
			continue
		}
		jobs.Add(1)
		go func(q request) {
			defer jobs.Done()
			var err error
			switch q.Action {
			case "state":
			case "stop":
				stop()
			case "write":
				if len(q.Data) > 65536 {
					err = errors.New("终端输入过长")
				} else {
					err = manager.Write(q.Data)
				}
			case "resize":
				err = manager.Resize(q.Cols, q.Rows)
			case "install", "start", "project":
				if !operationMu.TryLock() {
					send(response{ID: q.ID, Error: "上一项操作尚未完成"})
					return
				}
				defer operationMu.Unlock()
				operation, done := context.WithCancel(ctx)
				cancelMu.Lock()
				operationCancel = done
				cancelMu.Unlock()
				defer func() { done(); cancelMu.Lock(); operationCancel = nil; cancelMu.Unlock() }()
				if q.Action == "project" {
					err = manager.ChooseProject(q.Project)
					break
				}
				if err = validateRelay(q.Relay); err != nil {
					break
				}
				if q.Action == "install" {
					client := proxy.ClientThrough(q.Relay)
					client.Timeout = 10 * time.Minute
					err = manager.Install(operation, client, q.Relay)
				} else {
					// The manager independently verifies this relay; no invented exit IP is
					// passed to make a strict equality test succeed.
					err = manager.Start(ctx, q.Relay, &model.Observation{Revision: 1})
				}
			default:
				err = errors.New("不支持的终端操作")
			}
			if err != nil {
				send(response{ID: q.ID, Error: err.Error()})
			} else {
				send(response{ID: q.ID, Data: snapshot()})
			}
		}(q)
	}
	cancel()
	stop()
	jobs.Wait()
	return scanner.Err()
}
func main() {
	root := flag.String("data-dir", "", "application-owned Code state directory")
	flag.Parse()
	if *root == "" {
		os.Exit(2)
	}
	if serve(context.Background(), *root, os.Stdin, os.Stdout) != nil {
		os.Exit(1)
	}
}
