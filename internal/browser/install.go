package browser

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const KernelVersion = "148.0.7778.215"
const KernelURL = "https://github.com/adryfish/fingerprint-chromium/releases/download/148.0.7778.215/ungoogled-chromium_148.0.7778.215-1.1_windows_x64.zip"
const KernelSHA256 = "9ef3f471b7a6641b4224532522b29141ce3746e27d55788d88e2fd951f362579"

func Install(ctx context.Context, client *http.Client, root string) (string, error) {
	if p := Find(root); p != "" {
		return p, nil
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return "", e
	}
	staging, e := os.MkdirTemp(root, ".download-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(staging)
	req, e := http.NewRequestWithContext(ctx, "GET", KernelURL, nil)
	if e != nil {
		return "", e
	}
	r, e := client.Do(req)
	if e != nil {
		return "", errors.New("内核下载失败，请检查全局代理")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return "", fmt.Errorf("内核下载服务返回 HTTP %d", r.StatusCode)
	}
	f, e := os.Create(filepath.Join(staging, "kernel.zip"))
	if e != nil {
		return "", e
	}
	hash := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r.Body, 600<<20))
	f.Close()
	if e != nil || n >= 600<<20 {
		return "", errors.New("内核下载不完整或超出大小限制")
	}
	if hex.EncodeToString(hash.Sum(nil)) != KernelSHA256 {
		return "", errors.New("内核校验失败，未执行下载文件")
	}
	dest := filepath.Join(staging, "unpacked")
	if e = Extract(filepath.Join(staging, "kernel.zip"), dest); e != nil {
		return "", e
	}
	if Find(dest) == "" {
		return "", errors.New("下载包缺少 chrome.exe")
	}
	target := filepath.Join(root, KernelVersion)
	if e = os.Rename(dest, target); e != nil {
		return "", e
	}
	return Find(target), nil
}
func Find(root string) string {
	var result string
	filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e == nil && d.IsDir() && strings.HasPrefix(d.Name(), ".download-") {
			return filepath.SkipDir
		}
		if e == nil && !d.IsDir() && strings.EqualFold(d.Name(), "chrome.exe") {
			result = p
			return filepath.SkipAll
		}
		return nil
	})
	return result
}
func Extract(source, dest string) error {
	z, e := zip.OpenReader(source)
	if e != nil {
		return e
	}
	defer z.Close()
	var total uint64
	for _, f := range z.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		clean := filepath.Clean(name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(name, ":") || f.Mode()&os.ModeSymlink != 0 {
			return errors.New("内核压缩包包含不安全路径")
		}
		total += f.UncompressedSize64
		if total > 2<<30 {
			return errors.New("解压大小超限")
		}
		p := filepath.Join(dest, clean)
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(p, 0700); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			return e
		}
		in, e := f.Open()
		if e != nil {
			return e
		}
		out, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return e
		}
		_, e = io.Copy(out, io.LimitReader(in, int64(f.UncompressedSize64)+1))
		out.Close()
		in.Close()
		if e != nil {
			return e
		}
	}
	return nil
}
