package code

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const OfficialBase = "https://downloads.claude.ai/claude-code-releases"

var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$`)

func platformOS() string { return runtime.GOOS }
func fetchOfficial(ctx context.Context, client *http.Client, root string) (string, error) {
	return fetchFrom(ctx, client, root, OfficialBase, runtime.GOOS, runtime.GOARCH)
}
func fetchFrom(ctx context.Context, client *http.Client, root, base, system, arch string) (string, error) {
	platform := system + "-" + arch
	if arch == "amd64" {
		platform = system + "-x64"
	}
	name := "claude"
	if system == "windows" {
		platform = strings.Replace(platform, "windows-", "win32-", 1)
		name += ".exe"
	}
	if system != "windows" && system != "darwin" && system != "linux" {
		return "", errors.New("此系统暂不支持官方原生 Code")
	}
	read := func(url string) ([]byte, error) {
		q, e := http.NewRequestWithContext(ctx, "GET", url, nil)
		if e != nil {
			return nil, e
		}
		r, e := client.Do(q)
		if e != nil {
			return nil, errors.New("官方 Code 下载连接失败，请检查全局代理")
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			return nil, fmt.Errorf("官方下载服务返回 HTTP %d", r.StatusCode)
		}
		b, e := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		return b, e
	}
	b, e := read(base + "/latest")
	if e != nil {
		return "", e
	}
	version := strings.TrimSpace(string(b))
	if !versionPattern.MatchString(version) {
		return "", errors.New("官方版本信息无效")
	}
	b, e = read(base + "/" + version + "/manifest.json")
	if e != nil {
		return "", e
	}
	var manifest struct {
		Platforms map[string]struct {
			Checksum string `json:"checksum"`
		} `json:"platforms"`
	}
	if json.Unmarshal(b, &manifest) != nil {
		return "", errors.New("官方清单格式无效")
	}
	digest := manifest.Platforms[platform].Checksum
	if len(digest) != 64 {
		return "", errors.New("官方清单缺少当前平台校验值")
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return "", e
	}
	dir, e := os.MkdirTemp(root, "code-install-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(dir)
	q, _ := http.NewRequestWithContext(ctx, "GET", base+"/"+version+"/"+platform+"/"+name, nil)
	r, e := client.Do(q)
	if e != nil {
		return "", errors.New("官方二进制下载失败")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return "", fmt.Errorf("官方二进制返回 HTTP %d", r.StatusCode)
	}
	f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if e != nil {
		return "", e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(r.Body, 512<<20))
	f.Close()
	if e != nil || n >= 512<<20 {
		return "", errors.New("官方二进制下载中断或大小超限")
	}
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(digest) {
		return "", errors.New("官方 Code 校验失败，未执行下载文件")
	}
	// Move the staged binary out of the automatically removed download directory.
	dest := filepath.Join(root, "code-installer-"+version+"-"+name)
	if e = os.Rename(filepath.Join(dir, name), dest); e != nil {
		return "", e
	}
	return dest, nil
}
